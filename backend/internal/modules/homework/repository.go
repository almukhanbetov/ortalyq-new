package homework

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// Status implements attempts.HomeworkState. Missing rows mean "not submitted".
func (r *Repository) Status(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) (string, error) {
	var st string
	err := q.QueryRow(ctx, `SELECT status FROM homeworks WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3`,
		classroomID, lessonID, studentID).Scan(&st)
	if database.IsNoRows(err) {
		return StatusNotSubmitted, nil
	}
	return st, err
}

func scanHW(row pgx.Row) (*Homework, error) {
	var h Homework
	err := row.Scan(&h.ID, &h.ClassroomID, &h.LessonID, &h.StudentID, &h.Status, &h.StatusChangedAt, &h.StatusChangedBy)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

const hwCols = `id, classroom_id, lesson_id, student_id, status, status_changed_at, status_changed_by`

// Lock ensures a homework row exists and locks it for the transaction.
func (r *Repository) Lock(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID) (*Homework, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO homeworks (classroom_id, lesson_id, student_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		classroomID, lessonID, studentID); err != nil {
		return nil, err
	}
	return scanHW(tx.QueryRow(ctx, `SELECT `+hwCols+` FROM homeworks WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3 FOR UPDATE`,
		classroomID, lessonID, studentID))
}

func (r *Repository) Find(ctx context.Context, classroomID, lessonID, studentID uuid.UUID) (*Homework, error) {
	h, err := scanHW(r.pool.QueryRow(ctx, `SELECT `+hwCols+` FROM homeworks WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3`,
		classroomID, lessonID, studentID))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return h, err
}

func (r *Repository) SetStatus(ctx context.Context, tx pgx.Tx, h *Homework, to string, actor uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE homeworks SET status = $2, status_changed_at = now(), status_changed_by = $3 WHERE id = $1`, h.ID, to, actor); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO homework_status_events (homework_id, from_status, to_status, actor_id) VALUES ($1, $2, $3, $4)`, h.ID, h.Status, to, actor)
	return err
}

func (r *Repository) AddSubmission(ctx context.Context, tx pgx.Tx, homeworkID, by uuid.UUID, snapshot []byte) error {
	_, err := tx.Exec(ctx, `INSERT INTO homework_submissions (homework_id, number, submitted_by, snapshot)
		VALUES ($1, (SELECT COALESCE(max(number), 0) + 1 FROM homework_submissions WHERE homework_id = $1), $2, $3)`, homeworkID, by, snapshot)
	return err
}

func (r *Repository) Events(ctx context.Context, homeworkID uuid.UUID) ([]StatusEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.from_status, e.to_status, e.actor_id, COALESCE(u.full_name, ''), e.created_at
		FROM homework_status_events e LEFT JOIN users u ON u.id = e.actor_id WHERE e.homework_id = $1 ORDER BY e.created_at, e.id`, homeworkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StatusEvent{}
	for rows.Next() {
		var e StatusEvent
		if err := rows.Scan(&e.FromStatus, &e.ToStatus, &e.ActorID, &e.ActorName, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) LatestSubmission(ctx context.Context, homeworkID uuid.UUID) (*Submission, int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM homework_submissions WHERE homework_id = $1`, homeworkID).Scan(&count); err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return nil, 0, nil
	}
	var s Submission
	err := r.pool.QueryRow(ctx, `SELECT id, number, submitted_at, snapshot FROM homework_submissions WHERE homework_id = $1 ORDER BY number DESC LIMIT 1`, homeworkID).
		Scan(&s.ID, &s.Number, &s.SubmittedAt, &s.Snapshot)
	return &s, count, err
}
