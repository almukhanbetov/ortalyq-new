package grading

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) SectionCompletions(ctx context.Context, classroomID, lessonID uuid.UUID) (map[uuid.UUID]SectionProgress, error) {
	rows, err := r.pool.Query(ctx, `SELECT sc.section_id, sc.completed_at FROM section_completions sc
		JOIN sections s ON s.id = sc.section_id WHERE sc.classroom_id = $1 AND s.lesson_id = $2`, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]SectionProgress{}
	for rows.Next() {
		var sp SectionProgress
		if err := rows.Scan(&sp.ID, &sp.CompletedAt); err != nil {
			return nil, err
		}
		sp.Completed = true
		out[sp.ID] = sp
	}
	return out, rows.Err()
}

func (r *Repository) SetSection(ctx context.Context, classroomID, sectionID, actor uuid.UUID, completed bool) (bool, error) {
	if completed {
		tag, err := r.pool.Exec(ctx, `INSERT INTO section_completions (classroom_id, section_id, completed_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			classroomID, sectionID, actor)
		return tag.RowsAffected() > 0, err
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM section_completions WHERE classroom_id = $1 AND section_id = $2`, classroomID, sectionID)
	return tag.RowsAffected() > 0, err
}

func (r *Repository) Completion(ctx context.Context, classroomID, lessonID uuid.UUID) (Completion, error) {
	var c Completion
	err := r.pool.QueryRow(ctx, `SELECT cl.completed_at, COALESCE(u.full_name, ''), cl.grade, cl.graded_at
		FROM classroom_lessons cl LEFT JOIN users u ON u.id = cl.completed_by WHERE cl.classroom_id = $1 AND cl.lesson_id = $2`,
		classroomID, lessonID).Scan(&c.CompletedAt, &c.CompletedByName, &c.Grade, &c.GradedAt)
	c.Completed = c.CompletedAt != nil
	return c, err
}

// SetCompletion updates completion+grade atomically and writes an audit event.
// Returns false when nothing changed (idempotent repeat).
func (r *Repository) SetCompletion(ctx context.Context, classroomID, lessonID, actor uuid.UUID, completed bool, grade *string) (bool, error) {
	changed := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var cur *string
		var at any
		if err := tx.QueryRow(ctx, `SELECT grade, completed_at FROM classroom_lessons WHERE classroom_id = $1 AND lesson_id = $2 FOR UPDATE`,
			classroomID, lessonID).Scan(&cur, &at); err != nil {
			return err
		}
		isDone := cur != nil
		var event string
		switch {
		case completed && !isDone:
			event = "completed"
			_, err := tx.Exec(ctx, `UPDATE classroom_lessons SET completed_at = now(), completed_by = $3, grade = $4, graded_at = now(), graded_by = $3
				WHERE classroom_id = $1 AND lesson_id = $2`, classroomID, lessonID, actor, *grade)
			if err != nil {
				return err
			}
		case completed && isDone && grade != nil && *grade != *cur:
			event = "grade_changed"
			if _, err := tx.Exec(ctx, `UPDATE classroom_lessons SET grade = $3, graded_at = now(), graded_by = $4 WHERE classroom_id = $1 AND lesson_id = $2`,
				classroomID, lessonID, *grade, actor); err != nil {
				return err
			}
		case !completed && isDone:
			event = "uncompleted"
			if _, err := tx.Exec(ctx, `UPDATE classroom_lessons SET completed_at = NULL, completed_by = NULL, grade = NULL, graded_at = NULL, graded_by = NULL
				WHERE classroom_id = $1 AND lesson_id = $2`, classroomID, lessonID); err != nil {
				return err
			}
		default:
			return nil
		}
		changed = true
		_, err := tx.Exec(ctx, `INSERT INTO classroom_lesson_events (classroom_id, lesson_id, actor_id, event, grade) VALUES ($1, $2, $3, $4, $5)`,
			classroomID, lessonID, actor, event, grade)
		return err
	})
	return changed, err
}
