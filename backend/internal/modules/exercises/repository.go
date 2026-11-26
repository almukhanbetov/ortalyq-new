package exercises

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// insertVersion writes a new immutable version and makes it current.
func insertVersion(ctx context.Context, tx pgx.Tx, exerciseID uuid.UUID, by uuid.UUID, req SaveRequest) (uuid.UUID, error) {
	s := req.Settings
	var vid uuid.UUID
	err := tx.QueryRow(ctx, `INSERT INTO exercise_versions (exercise_id, version, title, check_mode, timer_enabled, timer_seconds,
		scoring_enabled, points_per_correct, category_id, created_by)
		VALUES ($1, (SELECT COALESCE(max(version), 0) + 1 FROM exercise_versions WHERE exercise_id = $1),
		$2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		exerciseID, strings.TrimSpace(req.Title), s.CheckMode, s.TimerEnabled, s.TimerSeconds, s.ScoringEnabled, s.PointsPerCorrect, s.CategoryID, by).Scan(&vid)
	if err != nil {
		return uuid.Nil, err
	}
	for qi, q := range req.Questions {
		var qid uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO exercise_questions (version_id, position, prompt, multiple) VALUES ($1, $2, $3, $4) RETURNING id`,
			vid, qi, strings.TrimSpace(q.Prompt), q.Multiple).Scan(&qid); err != nil {
			return uuid.Nil, err
		}
		for oi, o := range q.Options {
			if _, err := tx.Exec(ctx, `INSERT INTO exercise_options (question_id, position, label, is_correct) VALUES ($1, $2, $3, $4)`,
				qid, oi, strings.TrimSpace(o.Label), o.IsCorrect); err != nil {
				return uuid.Nil, err
			}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE exercises SET current_version_id = $2, updated_at = now() WHERE id = $1`, exerciseID, vid)
	return vid, err
}

func (r *Repository) Create(ctx context.Context, lessonID uuid.UUID, scope string, sectionID *uuid.UUID, kind string, by uuid.UUID, copiedFrom *uuid.UUID, req SaveRequest) (uuid.UUID, error) {
	var id uuid.UUID
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) (err error) {
		id, err = CreateTx(ctx, tx, lessonID, scope, sectionID, kind, by, copiedFrom, req)
		return err
	})
	return id, err
}

// CreateTx inserts an exercise with its first version inside the caller's transaction
// (the editor via Create, the course loader in internal/content). The request must be validated already.
func CreateTx(ctx context.Context, tx pgx.Tx, lessonID uuid.UUID, scope string, sectionID *uuid.UUID, kind string, by uuid.UUID, copiedFrom *uuid.UUID, req SaveRequest) (uuid.UUID, error) {
	var id uuid.UUID
	hidden := req.Hidden != nil && *req.Hidden
	if err := tx.QueryRow(ctx, `INSERT INTO exercises (lesson_id, scope, section_id, kind, hidden, created_by, copied_from_id, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT COALESCE(max(position), -1) + 1 FROM exercises
		  WHERE lesson_id = $1 AND scope = $2 AND section_id IS NOT DISTINCT FROM $3 AND deleted_at IS NULL)) RETURNING id`,
		lessonID, scope, sectionID, kind, hidden, by, copiedFrom).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	_, err := insertVersion(ctx, tx, id, by, req)
	return id, err
}

func (r *Repository) SaveVersion(ctx context.Context, id, by uuid.UUID, req SaveRequest) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		// Lock the exercise row so concurrent saves get distinct version numbers.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM exercises WHERE id = $1 FOR UPDATE`, id); err != nil {
			return err
		}
		if _, err := insertVersion(ctx, tx, id, by, req); err != nil {
			return err
		}
		if req.Hidden != nil {
			_, err := tx.Exec(ctx, `UPDATE exercises SET hidden = $2 WHERE id = $1`, id, *req.Hidden)
			return err
		}
		return nil
	})
}

func (r *Repository) SetHidden(ctx context.Context, id uuid.UUID, hidden bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE exercises SET hidden = $2, updated_at = now() WHERE id = $1`, id, hidden)
	return err
}

func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE exercises SET deleted_at = now(), updated_at = now() WHERE id = $1`, id)
	return err
}

const exCols = `e.id, e.lesson_id, e.scope, e.section_id, e.kind, e.position, e.hidden, e.copied_from_id, e.current_version_id, e.updated_at`

func scanEx(row pgx.Row, e *Exercise) error {
	return row.Scan(&e.ID, &e.LessonID, &e.Scope, &e.SectionID, &e.Kind, &e.Position, &e.Hidden, &e.CopiedFromID, &e.VersionID, &e.UpdatedAt)
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Exercise, error) {
	var e Exercise
	if err := scanEx(r.pool.QueryRow(ctx, `SELECT `+exCols+` FROM exercises e WHERE e.id = $1 AND e.deleted_at IS NULL`, id), &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ListForLesson returns exercises ordered by section position then exercise position.
func (r *Repository) ListForLesson(ctx context.Context, lessonID uuid.UUID, scope string, includeHidden bool) ([]Exercise, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+exCols+` FROM exercises e LEFT JOIN sections s ON s.id = e.section_id
		WHERE e.lesson_id = $1 AND e.scope = $2 AND e.deleted_at IS NULL AND ($3 OR NOT e.hidden)
		ORDER BY s.position NULLS FIRST, e.position, e.created_at`, lessonID, scope, includeHidden)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Exercise{}
	for rows.Next() {
		var e Exercise
		if err := scanEx(rows, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LoadVersions loads full versions (with key) by id.
func (r *Repository) LoadVersions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Version, error) {
	out := map[uuid.UUID]*Version{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT v.id, v.exercise_id, e.kind, v.version, v.title, v.check_mode, v.timer_enabled, v.timer_seconds,
		v.scoring_enabled, v.points_per_correct, v.category_id, COALESCE(c.name, ''), v.created_at
		FROM exercise_versions v JOIN exercises e ON e.id = v.exercise_id
		LEFT JOIN score_categories c ON c.id = v.category_id WHERE v.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Version
		s := &v.Settings
		if err := rows.Scan(&v.ID, &v.ExerciseID, &v.Kind, &v.Number, &v.Title, &s.CheckMode, &s.TimerEnabled, &s.TimerSeconds,
			&s.ScoringEnabled, &s.PointsPerCorrect, &s.CategoryID, &s.CategoryName, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		v.Questions = []Question{}
		out[v.ID] = &v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	qrows, err := r.pool.Query(ctx, `SELECT q.version_id, q.id, q.prompt, q.multiple, o.id, o.label, o.is_correct
		FROM exercise_questions q LEFT JOIN exercise_options o ON o.question_id = q.id
		WHERE q.version_id = ANY($1) ORDER BY q.version_id, q.position, o.position`, ids)
	if err != nil {
		return nil, err
	}
	defer qrows.Close()
	for qrows.Next() {
		// LEFT JOIN: a text block's question has no options (oid, label and correct come back NULL).
		var vid, qid uuid.UUID
		var oid *uuid.UUID
		var label *string
		var prompt string
		var multiple bool
		var correct *bool
		if err := qrows.Scan(&vid, &qid, &prompt, &multiple, &oid, &label, &correct); err != nil {
			return nil, err
		}
		v := out[vid]
		n := len(v.Questions)
		if n == 0 || v.Questions[n-1].ID != qid {
			v.Questions = append(v.Questions, Question{ID: qid, Prompt: prompt, Multiple: multiple, Options: []Option{}})
			n++
		}
		if oid != nil {
			c := *correct
			v.Questions[n-1].Options = append(v.Questions[n-1].Options, Option{ID: *oid, Label: *label, IsCorrect: &c})
		}
	}
	return out, qrows.Err()
}

func (r *Repository) Reorder(ctx context.Context, lessonID uuid.UUID, scope string, sectionID *uuid.UUID, ids []uuid.UUID) (bool, error) {
	valid := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var n, matched int
		if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE id = ANY($4)) FROM exercises
			WHERE lesson_id = $1 AND scope = $2 AND section_id IS NOT DISTINCT FROM $3 AND deleted_at IS NULL`,
			lessonID, scope, sectionID, ids).Scan(&n, &matched); err != nil {
			return err
		}
		if n != len(ids) || matched != n {
			return nil
		}
		valid = true
		_, err := tx.Exec(ctx, `UPDATE exercises e SET position = o.ord - 1, updated_at = now()
			FROM unnest($2::uuid[]) WITH ORDINALITY AS o(id, ord) WHERE e.id = o.id AND e.lesson_id = $1`, lessonID, ids)
		return err
	})
	return valid, err
}

func (r *Repository) Categories(ctx context.Context, owner uuid.UUID) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, color, owner_id IS NULL FROM score_categories
		WHERE owner_id IS NULL OR owner_id = $1 ORDER BY owner_id NULLS FIRST, name`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.System); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) CategoryVisible(ctx context.Context, id, owner uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM score_categories WHERE id = $1 AND (owner_id IS NULL OR owner_id = $2))`, id, owner).Scan(&ok)
	return ok, err
}

func (r *Repository) CreateCategory(ctx context.Context, owner uuid.UUID, name, color string) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx, `INSERT INTO score_categories (owner_id, name, color) VALUES ($1, $2, $3) RETURNING id, name, color, false`,
		owner, name, color).Scan(&c.ID, &c.Name, &c.Color, &c.System)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// QuestionStats aggregates live (non-reset) attempts of a given version.
func (r *Repository) QuestionStats(ctx context.Context, versionID uuid.UUID) (attempts, finished int, answers map[uuid.UUID][]Response, err error) {
	if err = r.pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE status = 'finished') FROM attempts WHERE version_id = $1 AND status <> 'reset'`,
		versionID).Scan(&attempts, &finished); err != nil {
		return
	}
	rows, err := r.pool.Query(ctx, `SELECT aa.question_id, aa.selected_option_ids, aa.answer_text FROM attempt_answers aa
		JOIN attempts a ON a.id = aa.attempt_id WHERE a.version_id = $1 AND a.status <> 'reset'`, versionID)
	if err != nil {
		return
	}
	defer rows.Close()
	answers = map[uuid.UUID][]Response{}
	for rows.Next() {
		var q uuid.UUID
		var resp Response
		if err = rows.Scan(&q, &resp.Selected, &resp.Text); err != nil {
			return
		}
		answers[q] = append(answers[q], resp)
	}
	err = rows.Err()
	return
}
