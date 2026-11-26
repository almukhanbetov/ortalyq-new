package content

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/exercises"
)

// Result tells what a load did.
type Result struct {
	CourseID      uuid.UUID
	CourseCreated bool
	// CourseDeleted: the course was loaded before and the teacher deleted it, so nothing is loaded again.
	CourseDeleted bool
	Created       []string // lesson titles added now
	Kept          []string // lessons loaded before and left as the teacher has them
	Deleted       []string // lessons loaded before and deleted by the teacher; not restored
}

// Validate checks the course the way the editor checks a saved exercise, plus the authoring rules of this
// content: one gap and one answer per «Слова и пропуски», one correct option per test question.
func Validate(c Course) error {
	var errs []string
	add := func(where, msg string) { errs = append(errs, where+": "+msg) }
	if c.Key == "" || c.Title == "" {
		add(c.Title, "course key and title are required")
	}
	keys := map[string]bool{}
	for _, l := range c.Lessons {
		if l.Key == "" || strings.Contains(l.Key, "/") || keys[l.Key] {
			add(l.Title, "lesson key must be unique and without /")
		}
		keys[l.Key] = true
		check := func(where string, e Exercise) {
			for f, m := range exercises.CheckContent(e.Kind, e.SaveRequest) {
				add(where, f+" "+m)
			}
			for i, q := range e.Questions {
				n := 0
				for _, o := range q.Options {
					if o.IsCorrect {
						n++
					}
				}
				if n != 1 || q.Multiple {
					add(where, fmt.Sprintf("question %d must have exactly one correct option", i+1))
				}
			}
		}
		for _, s := range l.Sections {
			for _, e := range s.Exercises {
				check(l.Title+" / "+s.Title+" / "+e.Title, e)
			}
		}
		for _, e := range l.Homework {
			check(l.Title+" / homework / "+e.Title, e)
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return errors.New("content: " + strings.Join(errs, "; "))
	}
	return nil
}

// Load gives the course to the teacher. The first load creates the course with all lessons. A repeated load
// adds only lessons whose keys were never loaded for this teacher: it never creates a duplicate, never
// overwrites a lesson the teacher has changed and does not bring back what the teacher deleted.
// Everything happens in one transaction under an advisory lock, so parallel loads cannot both create.
func Load(ctx context.Context, pool *pgxpool.Pool, ownerID uuid.UUID, c Course) (*Result, error) {
	if err := Validate(c); err != nil {
		return nil, err
	}
	res := &Result{}
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "content:"+ownerID.String()+":"+c.Key); err != nil {
			return err
		}
		courseID, seen, err := imported(ctx, tx, ownerID, c.Key)
		if err != nil {
			return err
		}
		if seen {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM courses WHERE id = $1 AND owner_id = $2)`, courseID, ownerID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				res.CourseDeleted = true
				return nil
			}
		} else {
			if err := tx.QueryRow(ctx, `INSERT INTO courses (owner_id, title, description, cover_color, position)
				VALUES ($1, $2, $3, $4, (SELECT COALESCE(max(position), -1) + 1 FROM courses WHERE owner_id = $1)) RETURNING id`,
				ownerID, c.Title, c.Description, c.CoverColor).Scan(&courseID); err != nil {
				return err
			}
			if err := remember(ctx, tx, ownerID, c.Key, courseID); err != nil {
				return err
			}
			res.CourseCreated = true
		}
		res.CourseID = courseID
		for _, l := range c.Lessons {
			key := c.Key + "/" + l.Key
			lessonID, seen, err := imported(ctx, tx, ownerID, key)
			if err != nil {
				return err
			}
			if seen {
				var exists bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lessons WHERE id = $1)`, lessonID).Scan(&exists); err != nil {
					return err
				}
				if exists {
					res.Kept = append(res.Kept, l.Title)
				} else {
					res.Deleted = append(res.Deleted, l.Title)
				}
				continue
			}
			if lessonID, err = createLesson(ctx, tx, ownerID, courseID, l); err != nil {
				return fmt.Errorf("%s: %w", l.Title, err)
			}
			if err := remember(ctx, tx, ownerID, key, lessonID); err != nil {
				return err
			}
			res.Created = append(res.Created, l.Title)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func imported(ctx context.Context, tx pgx.Tx, owner uuid.UUID, key string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT entity_id FROM content_imports WHERE owner_id = $1 AND key = $2`, owner, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func remember(ctx context.Context, tx pgx.Tx, owner uuid.UUID, key string, id uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO content_imports (owner_id, key, entity_id) VALUES ($1, $2, $3)`, owner, key, id)
	return err
}

func createLesson(ctx context.Context, tx pgx.Tx, owner, courseID uuid.UUID, l Lesson) (uuid.UUID, error) {
	a := l.Attrs
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `INSERT INTO lessons (course_id, title, description, position, language, age, level, lesson_type,
		duration_min, skills, grammar, vocabulary, functions, tags)
		VALUES ($1, $2, $3, (SELECT COALESCE(max(position), -1) + 1 FROM lessons WHERE course_id = $1), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`,
		courseID, l.Title, l.GuideText(), a.Language, a.Age, a.Level, a.LessonType, a.DurationMin,
		nonNil(a.Skills), nonNil(a.Grammar), nonNil(a.Vocabulary), nonNil(a.Functions), nonNil(a.Tags)).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	for i, s := range l.Sections {
		var sid uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO sections (lesson_id, title, position) VALUES ($1, $2, $3) RETURNING id`, id, s.Title, i).Scan(&sid); err != nil {
			return uuid.Nil, err
		}
		for _, e := range s.Exercises {
			if _, err := exercises.CreateTx(ctx, tx, id, exercises.ScopeLesson, &sid, e.Kind, owner, nil, e.SaveRequest); err != nil {
				return uuid.Nil, err
			}
		}
	}
	for _, e := range l.Homework {
		if _, err := exercises.CreateTx(ctx, tx, id, exercises.ScopeHomework, nil, e.Kind, owner, nil, e.SaveRequest); err != nil {
			return uuid.Nil, err
		}
	}
	return id, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
