package materials

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

type Repository struct {
	pool *pgxpool.Pool
	db   database.Querier
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool, db: pool} }

// lessonMatch is a reusable WHERE fragment over alias l, parameters $2..$12.
const lessonMatch = `($2 = '' OR l.title ILIKE '%' || $2 || '%' OR c.title ILIKE '%' || $2 || '%')
	AND ($3 = '' OR l.language = $3) AND ($4 = '' OR l.age = $4) AND ($5 = '' OR l.level = $5)
	AND ($6 = '' OR l.lesson_type = $6) AND ($7 = '' OR $7 = ANY(l.skills))
	AND ($8 = 0 OR l.duration_min <= $8) AND ($9 = '' OR $9 = ANY(l.grammar))
	AND ($10 = '' OR $10 = ANY(l.vocabulary)) AND ($11 = '' OR $11 = ANY(l.functions))
	AND ($12 = '' OR $12 = ANY(l.tags))`

func filterArgs(f LessonFilter) []any {
	return []any{f.Query, f.Language, f.Age, f.Level, f.LessonType, f.Skill, f.MaxDuration, f.Grammar, f.Vocabulary, f.Function, f.Tag}
}

func (r *Repository) ListCourses(ctx context.Context, owner uuid.UUID, f LessonFilter, limit, offset int) ([]Course, int, error) {
	// A course matches when its title matches the query (and no params are set) or any lesson matches.
	where := `c.owner_id = $1 AND (
		(NOT $13 AND ($2 = '' OR c.title ILIKE '%' || $2 || '%'))
		OR EXISTS (SELECT 1 FROM lessons l WHERE l.course_id = c.id AND ` + lessonMatch + `))`
	args := append([]any{owner}, filterArgs(f)...)
	args = append(args, f.Active())
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM courses c WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, `SELECT `+courseCols+`
		FROM courses c WHERE `+where+` ORDER BY c.position, c.created_at LIMIT $14 OFFSET $15`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Course{}
	for rows.Next() {
		var c Course
		if err := scanCourse(rows, &c); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

const courseCols = `c.id, c.owner_id, c.title, c.description, c.cover_color, c.cover_image_id, c.language, c.sections_enabled,
	c.tags, c.position, (SELECT count(*) FROM lessons l WHERE l.course_id = c.id), c.created_at, c.updated_at`

func scanCourse(row pgx.Row, c *Course) error {
	return row.Scan(&c.ID, &c.OwnerID, &c.Title, &c.Description, &c.CoverColor, &c.CoverImageID, &c.Language, &c.SectionsEnabled,
		&c.Tags, &c.Position, &c.LessonCount, &c.CreatedAt, &c.UpdatedAt)
}

func (r *Repository) GetCourse(ctx context.Context, id uuid.UUID) (*Course, error) {
	var c Course
	if err := scanCourse(r.db.QueryRow(ctx, `SELECT `+courseCols+` FROM courses c WHERE c.id = $1`, id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

const lessonCols = `l.id, l.course_id, l.title, l.description, l.position,
	(SELECT count(*) FROM sections s WHERE s.lesson_id = l.id),
	(SELECT c2.sections_enabled FROM courses c2 WHERE c2.id = l.course_id),
	l.language, l.age, l.level, l.lesson_type, l.duration_min, l.skills, l.grammar, l.vocabulary, l.functions, l.tags, l.updated_at`

func scanLesson(row pgx.Row, l *Lesson) error {
	return row.Scan(&l.ID, &l.CourseID, &l.Title, &l.Description, &l.Position, &l.SectionCount, &l.SectionsEnabled,
		&l.Language, &l.Age, &l.Level, &l.LessonType, &l.DurationMin, &l.Skills, &l.Grammar, &l.Vocabulary, &l.Functions, &l.Tags, &l.UpdatedAt)
}

func (r *Repository) ListLessons(ctx context.Context, courseID uuid.UUID, f LessonFilter) ([]Lesson, error) {
	args := append([]any{courseID}, filterArgs(f)...)
	// When searching inside a course, a course-title match shows all its lessons.
	rows, err := r.db.Query(ctx, `SELECT `+lessonCols+` FROM lessons l JOIN courses c ON c.id = l.course_id
		WHERE l.course_id = $1 AND `+lessonMatch+` ORDER BY l.position, l.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lesson{}
	for rows.Next() {
		var l Lesson
		if err := scanLesson(rows, &l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repository) GetLesson(ctx context.Context, id uuid.UUID) (*Lesson, error) {
	var l Lesson
	if err := scanLesson(r.db.QueryRow(ctx, `SELECT `+lessonCols+` FROM lessons l WHERE l.id = $1`, id), &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *Repository) Sections(ctx context.Context, lessonID uuid.UUID) ([]Section, error) {
	rows, err := r.db.Query(ctx, `SELECT id, lesson_id, title, position FROM sections WHERE lesson_id = $1 ORDER BY position, created_at`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Section{}
	for rows.Next() {
		var s Section
		if err := rows.Scan(&s.ID, &s.LessonID, &s.Title, &s.Position); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) LessonRef(ctx context.Context, lessonID uuid.UUID) (*LessonRef, error) {
	var ref LessonRef
	err := r.db.QueryRow(ctx, `SELECT l.id, l.title, c.id, c.title, c.owner_id FROM lessons l JOIN courses c ON c.id = l.course_id WHERE l.id = $1`, lessonID).
		Scan(&ref.LessonID, &ref.LessonTitle, &ref.CourseID, &ref.CourseTitle, &ref.OwnerID)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

func (r *Repository) SectionLesson(ctx context.Context, sectionID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT lesson_id FROM sections WHERE id = $1`, sectionID).Scan(&id)
	return id, err
}

// NewCourse holds the checked fields of a new course.
type NewCourse struct {
	Title, Description, Color, Language string
	CoverImageID                        *uuid.UUID
	SectionsEnabled                     bool
	Tags                                []string
}

func (r *Repository) CreateCourse(ctx context.Context, owner uuid.UUID, n NewCourse) (*Course, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `INSERT INTO courses (owner_id, title, description, cover_color, cover_image_id, language, sections_enabled, tags, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT COALESCE(max(position), -1) + 1 FROM courses WHERE owner_id = $1)) RETURNING id`,
		owner, n.Title, n.Description, n.Color, n.CoverImageID, n.Language, n.SectionsEnabled, n.Tags).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetCourse(ctx, id)
}

// CourseChanges are the checked fields of an update; nil leaves a field as it is. ClearCover removes the picture.
type CourseChanges struct {
	Title, Description, Color, Language *string
	CoverImageID                        *uuid.UUID
	ClearCover                          bool
	SectionsEnabled                     *bool
	Tags                                []string // nil: unchanged
}

func (r *Repository) UpdateCourse(ctx context.Context, id uuid.UUID, ch CourseChanges) error {
	_, err := r.db.Exec(ctx, `UPDATE courses SET title = COALESCE($2, title), description = COALESCE($3, description),
		cover_color = COALESCE($4, cover_color), language = COALESCE($5, language),
		cover_image_id = CASE WHEN $7 THEN NULL ELSE COALESCE($6, cover_image_id) END,
		sections_enabled = COALESCE($8, sections_enabled), tags = COALESCE($9, tags), updated_at = now() WHERE id = $1`,
		id, ch.Title, ch.Description, ch.Color, ch.Language, ch.CoverImageID, ch.ClearCover, ch.SectionsEnabled, ch.Tags)
	return err
}

func (r *Repository) CreateCover(ctx context.Context, c *Cover) (*Cover, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO course_covers (owner_id, stored_name, mime_type, size_bytes, width, height)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`, c.OwnerID, c.StoredName, c.MimeType, c.SizeBytes, c.Width, c.Height).
		Scan(&c.ID, &c.CreatedAt)
	return c, err
}

// Cover returns a cover picture or a no-rows error.
func (r *Repository) Cover(ctx context.Context, id uuid.UUID) (*Cover, error) {
	var c Cover
	err := r.db.QueryRow(ctx, `SELECT id, owner_id, stored_name, mime_type, size_bytes, width, height, created_at FROM course_covers WHERE id = $1`, id).
		Scan(&c.ID, &c.OwnerID, &c.StoredName, &c.MimeType, &c.SizeBytes, &c.Width, &c.Height, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateLesson adds a lesson; in a course with «Разделы» off it gets one section in the same statement.
func (r *Repository) CreateLesson(ctx context.Context, courseID uuid.UUID, title string) (*Lesson, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `WITH l AS (
			INSERT INTO lessons (course_id, title, position)
			VALUES ($1, $2, (SELECT COALESCE(max(position), -1) + 1 FROM lessons WHERE course_id = $1)) RETURNING id
		), s AS (
			INSERT INTO sections (lesson_id, title, position)
			SELECT l.id, $3, 0 FROM l WHERE NOT (SELECT sections_enabled FROM courses WHERE id = $1)
		)
		SELECT id FROM l`, courseID, title, DefaultSectionTitle).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetLesson(ctx, id)
}

func (r *Repository) UpdateLesson(ctx context.Context, id uuid.UUID, title, desc *string, a *LessonAttrs) error {
	if a == nil {
		_, err := r.db.Exec(ctx, `UPDATE lessons SET title = COALESCE($2, title), description = COALESCE($3, description), updated_at = now() WHERE id = $1`, id, title, desc)
		return err
	}
	_, err := r.db.Exec(ctx, `UPDATE lessons SET title = COALESCE($2, title), description = COALESCE($3, description),
		language = $4, age = $5, level = $6, lesson_type = $7, duration_min = $8, skills = $9, grammar = $10,
		vocabulary = $11, functions = $12, tags = $13, updated_at = now() WHERE id = $1`,
		id, title, desc, a.Language, a.Age, a.Level, a.LessonType, a.DurationMin, a.Skills, a.Grammar, a.Vocabulary, a.Functions, a.Tags)
	return err
}

func (r *Repository) CreateSection(ctx context.Context, lessonID uuid.UUID, title string) (*Section, error) {
	var s Section
	err := r.db.QueryRow(ctx, `INSERT INTO sections (lesson_id, title, position)
		VALUES ($1, $2, (SELECT COALESCE(max(position), -1) + 1 FROM sections WHERE lesson_id = $1))
		RETURNING id, lesson_id, title, position`, lessonID, title).Scan(&s.ID, &s.LessonID, &s.Title, &s.Position)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) UpdateSection(ctx context.Context, id uuid.UUID, title string) error {
	_, err := r.db.Exec(ctx, `UPDATE sections SET title = $2, updated_at = now() WHERE id = $1`, id, title)
	return err
}

var reorderTables = map[string]string{"courses": "owner_id", "lessons": "course_id", "sections": "lesson_id"}

// Reorder sets positions; ids must be exactly the children of parent.
func (r *Repository) Reorder(ctx context.Context, table string, parent uuid.UUID, ids []uuid.UUID) (bool, error) {
	col, ok := reorderTables[table]
	if !ok {
		return false, fmt.Errorf("bad table %s", table)
	}
	valid := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s = $1`, table, col), parent).Scan(&n); err != nil {
			return err
		}
		var matched int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(DISTINCT id) FROM %s WHERE %s = $1 AND id = ANY($2)`, table, col), parent, ids).Scan(&matched); err != nil {
			return err
		}
		if n != len(ids) || matched != n {
			return nil
		}
		valid = true
		_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s t SET position = o.ord - 1, updated_at = now()
			FROM unnest($2::uuid[]) WITH ORDINALITY AS o(id, ord) WHERE t.id = o.id AND t.%s = $1`, table, col), parent, ids)
		return err
	})
	return valid, err
}

func (r *Repository) Options(ctx context.Context, owner uuid.UUID) (*FilterOptions, error) {
	var o FilterOptions
	err := r.db.QueryRow(ctx, `SELECT
		COALESCE(array_agg(DISTINCT l.language) FILTER (WHERE l.language <> ''), '{}'),
		COALESCE(array_agg(DISTINCT l.age) FILTER (WHERE l.age <> ''), '{}'),
		COALESCE(array_agg(DISTINCT l.level) FILTER (WHERE l.level <> ''), '{}'),
		COALESCE(array_agg(DISTINCT l.lesson_type) FILTER (WHERE l.lesson_type <> ''), '{}'),
		COALESCE((SELECT array_agg(DISTINCT x) FROM lessons l2 JOIN courses c2 ON c2.id = l2.course_id, unnest(l2.skills) x WHERE c2.owner_id = $1), '{}'),
		COALESCE(array_agg(DISTINCT l.duration_min) FILTER (WHERE l.duration_min IS NOT NULL), '{}'),
		COALESCE((SELECT array_agg(DISTINCT x) FROM lessons l2 JOIN courses c2 ON c2.id = l2.course_id, unnest(l2.grammar) x WHERE c2.owner_id = $1), '{}'),
		COALESCE((SELECT array_agg(DISTINCT x) FROM lessons l2 JOIN courses c2 ON c2.id = l2.course_id, unnest(l2.vocabulary) x WHERE c2.owner_id = $1), '{}'),
		COALESCE((SELECT array_agg(DISTINCT x) FROM lessons l2 JOIN courses c2 ON c2.id = l2.course_id, unnest(l2.functions) x WHERE c2.owner_id = $1), '{}'),
		COALESCE((SELECT array_agg(DISTINCT x) FROM lessons l2 JOIN courses c2 ON c2.id = l2.course_id, unnest(l2.tags) x WHERE c2.owner_id = $1), '{}')
		FROM lessons l JOIN courses c ON c.id = l.course_id WHERE c.owner_id = $1`, owner).
		Scan(&o.Language, &o.Age, &o.Level, &o.LessonType, &o.Skills, &o.Duration, &o.Grammar, &o.Vocabulary, &o.Functions, &o.Tags)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
