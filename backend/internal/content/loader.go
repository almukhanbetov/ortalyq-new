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
	BlocksAdded   int      // text blocks added now (to new lessons and to lessons loaded before)
	BlocksKept    int      // text blocks loaded before: left as the teacher has them, or deleted by the teacher
	BlocksSkipped []string // blocks whose section the teacher renamed or deleted: not added
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
		blockKeys := map[string]bool{}
		for _, b := range l.Blocks() {
			where := l.Title + " / " + b.Section + " / " + b.Title
			if blockKeys[b.Key] {
				add(where, "block key must be unique")
			}
			blockKeys[b.Key] = true
			found := false
			for _, s := range l.Sections {
				found = found || s.Title == b.Section
			}
			if !found {
				add(where, "no such section")
			}
			for f, m := range exercises.CheckContent(exercises.KindText, b.Request()) {
				add(where, f+" "+m)
			}
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
	return load(ctx, pool, ownerID, c, false)
}

// AddFromCatalog is «Добавить в мои материалы»: the same load, except that a course the teacher once received and
// later deleted is given again as a fresh copy (the teacher asks for it explicitly). While the copy exists, adding
// again creates nothing.
func AddFromCatalog(ctx context.Context, pool *pgxpool.Pool, ownerID uuid.UUID, c Course) (*Result, error) {
	return load(ctx, pool, ownerID, c, true)
}

func load(ctx context.Context, pool *pgxpool.Pool, ownerID uuid.UUID, c Course, restore bool) (*Result, error) {
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
			if !exists && !restore {
				res.CourseDeleted = true
				return nil
			}
			if !exists {
				// Forget the deleted copy (the course and its lessons/blocks) and load a new one.
				if _, err := tx.Exec(ctx, `DELETE FROM content_imports WHERE owner_id = $1 AND (key = $2 OR left(key, length($2) + 1) = $2 || '/')`, ownerID, c.Key); err != nil {
					return err
				}
				seen = false
			}
		}
		if !seen {
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
				if !exists {
					res.Deleted = append(res.Deleted, l.Title)
					continue
				}
				res.Kept = append(res.Kept, l.Title)
			} else {
				if lessonID, err = createLesson(ctx, tx, ownerID, courseID, l); err != nil {
					return fmt.Errorf("%s: %w", l.Title, err)
				}
				if err := remember(ctx, tx, ownerID, key, lessonID); err != nil {
					return err
				}
				res.Created = append(res.Created, l.Title)
			}
			if err := addBlocks(ctx, tx, ownerID, lessonID, key, l, res); err != nil {
				return fmt.Errorf("%s: %w", l.Title, err)
			}
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

// addBlocks adds the lesson's text blocks that were never loaded for this teacher. A loaded block is never touched
// again (the teacher may edit or delete it). Blocks «at the top» go before the section's exercises: the items of
// the section move down by the number of new blocks, nothing else of the teacher's changes.
func addBlocks(ctx context.Context, tx pgx.Tx, owner, lessonID uuid.UUID, lessonKey string, l Lesson, res *Result) error {
	rows, err := tx.Query(ctx, `SELECT id, title FROM sections WHERE lesson_id = $1 ORDER BY position, created_at`, lessonID)
	if err != nil {
		return err
	}
	sections := map[string]uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			rows.Close()
			return err
		}
		if _, dup := sections[title]; !dup {
			sections[title] = id
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	type missing struct {
		b   Block
		key string
	}
	top := map[uuid.UUID][]missing{}
	var order []uuid.UUID
	var bottom []missing
	bottomSection := map[string]uuid.UUID{}
	for _, b := range l.Blocks() {
		key := lessonKey + "/block/" + b.Key
		if _, seen, err := imported(ctx, tx, owner, key); err != nil {
			return err
		} else if seen {
			res.BlocksKept++
			continue
		}
		sid, ok := sections[b.Section]
		if !ok {
			res.BlocksSkipped = append(res.BlocksSkipped, l.Title+" / "+b.Section+" / "+b.Title)
			continue
		}
		if b.AtTop {
			if _, ok := top[sid]; !ok {
				order = append(order, sid)
			}
			top[sid] = append(top[sid], missing{b, key})
		} else {
			bottom = append(bottom, missing{b, key})
			bottomSection[key] = sid
		}
	}
	create := func(sid uuid.UUID, m missing) (uuid.UUID, error) {
		id, err := exercises.CreateTx(ctx, tx, lessonID, exercises.ScopeLesson, &sid, exercises.KindText, owner, nil, m.b.Request())
		if err != nil {
			return uuid.Nil, err
		}
		if err := remember(ctx, tx, owner, m.key, id); err != nil {
			return uuid.Nil, err
		}
		res.BlocksAdded++
		return id, nil
	}
	for _, sid := range order {
		list := top[sid]
		if _, err := tx.Exec(ctx, `UPDATE exercises SET position = position + $2 WHERE section_id = $1 AND deleted_at IS NULL`, sid, len(list)); err != nil {
			return err
		}
		for i, m := range list {
			id, err := create(sid, m)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE exercises SET position = $2 WHERE id = $1`, id, i); err != nil {
				return err
			}
		}
	}
	for _, m := range bottom {
		if _, err := create(bottomSection[m.key], m); err != nil {
			return err
		}
	}
	return nil
}
