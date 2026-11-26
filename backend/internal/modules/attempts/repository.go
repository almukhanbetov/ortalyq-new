package attempts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/exercises"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

const attCols = `id, classroom_id, exercise_id, version_id, student_id, status, started_at, deadline_at, finished_at,
	finish_reason, correct_count, wrong_count, unanswered_count, points_earned, points_max`

func scanAtt(row pgx.Row, a *Attempt) error {
	return row.Scan(&a.ID, &a.ClassroomID, &a.ExerciseID, &a.VersionID, &a.StudentID, &a.Status, &a.StartedAt, &a.DeadlineAt,
		&a.FinishedAt, &a.FinishReason, &a.CorrectCount, &a.WrongCount, &a.UnansweredCount, &a.PointsEarned, &a.PointsMax)
}

func (r *Repository) Now(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := r.pool.QueryRow(ctx, `SELECT now()`).Scan(&t)
	return t, err
}

// Start inserts a live attempt if none exists, and returns the live one (idempotent).
func (r *Repository) Start(ctx context.Context, classroomID, exerciseID, versionID, studentID uuid.UUID, timerSeconds *int) (*Attempt, error) {
	_, err := r.pool.Exec(ctx, `INSERT INTO attempts (classroom_id, exercise_id, version_id, student_id, status, deadline_at)
		VALUES ($1, $2, $3, $4, 'in_progress', CASE WHEN $5::int IS NULL THEN NULL ELSE now() + make_interval(secs => $5::int) END)
		ON CONFLICT (classroom_id, exercise_id, student_id) WHERE status <> 'reset' DO NOTHING`,
		classroomID, exerciseID, versionID, studentID, timerSeconds)
	if err != nil {
		return nil, err
	}
	return r.Live(ctx, r.pool, classroomID, exerciseID, studentID)
}

func (r *Repository) Live(ctx context.Context, q database.Querier, classroomID, exerciseID, studentID uuid.UUID) (*Attempt, error) {
	var a Attempt
	err := scanAtt(q.QueryRow(ctx, `SELECT `+attCols+` FROM attempts WHERE classroom_id = $1 AND exercise_id = $2 AND student_id = $3 AND status <> 'reset'`,
		classroomID, exerciseID, studentID), &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) Get(ctx context.Context, q database.Querier, id uuid.UUID, forUpdate bool) (*Attempt, error) {
	sql := `SELECT ` + attCols + ` FROM attempts WHERE id = $1`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	var a Attempt
	if err := scanAtt(q.QueryRow(ctx, sql, id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// LiveForStudent returns live attempts keyed by exercise for one student in one classroom.
func (r *Repository) LiveForStudent(ctx context.Context, q database.Querier, classroomID, studentID uuid.UUID, exerciseIDs []uuid.UUID) (map[uuid.UUID]*Attempt, error) {
	rows, err := q.Query(ctx, `SELECT `+attCols+` FROM attempts WHERE classroom_id = $1 AND student_id = $2 AND exercise_id = ANY($3) AND status <> 'reset'`,
		classroomID, studentID, exerciseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]*Attempt{}
	for rows.Next() {
		var a Attempt
		if err := scanAtt(rows, &a); err != nil {
			return nil, err
		}
		out[a.ExerciseID] = &a
	}
	return out, rows.Err()
}

// LiveForStudents returns live attempts of several students keyed by student and exercise.
func (r *Repository) LiveForStudents(ctx context.Context, classroomID uuid.UUID, studentIDs, exerciseIDs []uuid.UUID) (map[uuid.UUID]map[uuid.UUID]*Attempt, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+attCols+` FROM attempts WHERE classroom_id = $1 AND student_id = ANY($2) AND exercise_id = ANY($3) AND status <> 'reset'`,
		classroomID, studentIDs, exerciseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]map[uuid.UUID]*Attempt{}
	for rows.Next() {
		var a Attempt
		if err := scanAtt(rows, &a); err != nil {
			return nil, err
		}
		if out[a.StudentID] == nil {
			out[a.StudentID] = map[uuid.UUID]*Attempt{}
		}
		out[a.StudentID][a.ExerciseID] = &a
	}
	return out, rows.Err()
}

func (r *Repository) ResetCounts(ctx context.Context, classroomID, studentID uuid.UUID, exerciseIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT exercise_id, count(*) FROM attempts WHERE classroom_id = $1 AND student_id = $2
		AND exercise_id = ANY($3) AND status = 'reset' GROUP BY exercise_id`, classroomID, studentID, exerciseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func (r *Repository) Answers(ctx context.Context, q database.Querier, attemptIDs []uuid.UUID) (map[uuid.UUID]map[uuid.UUID]Answer, error) {
	out := map[uuid.UUID]map[uuid.UUID]Answer{}
	if len(attemptIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT attempt_id, question_id, selected_option_ids, answer_text, checked, updated_at FROM attempt_answers WHERE attempt_id = ANY($1)`, attemptIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var aid uuid.UUID
		var a Answer
		if err := rows.Scan(&aid, &a.QuestionID, &a.Selected, &a.Text, &a.Checked, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if out[aid] == nil {
			out[aid] = map[uuid.UUID]Answer{}
		}
		out[aid][a.QuestionID] = a
	}
	return out, rows.Err()
}

func (r *Repository) UpsertAnswer(ctx context.Context, q database.Querier, attemptID, questionID uuid.UUID, resp exercises.Response, checked bool) error {
	_, err := q.Exec(ctx, `INSERT INTO attempt_answers (attempt_id, question_id, selected_option_ids, answer_text, checked)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (attempt_id, question_id) DO UPDATE SET selected_option_ids = EXCLUDED.selected_option_ids,
		answer_text = EXCLUDED.answer_text, checked = EXCLUDED.checked, updated_at = now()`, attemptID, questionID, resp.Selected, resp.Text, checked)
	if err == nil {
		_, err = q.Exec(ctx, `UPDATE attempts SET updated_at = now() WHERE id = $1`, attemptID)
	}
	return err
}

func (r *Repository) Finish(ctx context.Context, q database.Querier, id uuid.UUID, reason string, at time.Time, s Summary) error {
	_, err := q.Exec(ctx, `UPDATE attempts SET status = 'finished', finished_at = $3, finish_reason = $2,
		correct_count = $4, wrong_count = $5, unanswered_count = $6, points_earned = $7, points_max = $8, updated_at = now()
		WHERE id = $1 AND status = 'in_progress'`, id, reason, at, s.Correct, s.Wrong, s.Unanswered, s.PointsEarned, s.PointsMax)
	return err
}

// Reopen returns a finished attempt to progress (homework revision). The timer is not restarted.
func (r *Repository) Reopen(ctx context.Context, q database.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE attempts SET status = 'in_progress', finished_at = NULL, finish_reason = NULL, deadline_at = NULL,
		correct_count = NULL, wrong_count = NULL, unanswered_count = NULL, points_earned = NULL, points_max = NULL, updated_at = now()
		WHERE id = $1 AND status = 'finished'`, id)
	return err
}

func (r *Repository) UncheckAnswers(ctx context.Context, q database.Querier, attemptID uuid.UUID, questionIDs []uuid.UUID) error {
	if len(questionIDs) == 0 {
		return nil
	}
	_, err := q.Exec(ctx, `UPDATE attempt_answers SET checked = false, updated_at = now() WHERE attempt_id = $1 AND question_id = ANY($2)`, attemptID, questionIDs)
	return err
}

// Reset archives the live attempt (kept for audit) so a fresh one can start.
func (r *Repository) Reset(ctx context.Context, classroomID, exerciseID, studentID, by uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE attempts SET status = 'reset', reset_at = now(), reset_by = $4, updated_at = now()
		WHERE classroom_id = $1 AND exercise_id = $2 AND student_id = $3 AND status <> 'reset'`, classroomID, exerciseID, studentID, by)
	return tag.RowsAffected() > 0, err
}

// LessonOpen reports whether the exercise's lesson is still open in the classroom.
func (r *Repository) LessonOpen(ctx context.Context, q database.Querier, classroomID, exerciseID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM exercises e
		JOIN classroom_lessons cl ON cl.lesson_id = e.lesson_id AND cl.classroom_id = $1 WHERE e.id = $2)`, classroomID, exerciseID).Scan(&ok)
	return ok, err
}

// ExerciseLesson returns the lesson of an exercise (for the lesson-level access check of a marathon).
func (r *Repository) ExerciseLesson(ctx context.Context, q database.Querier, exerciseID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT lesson_id FROM exercises WHERE id = $1`, exerciseID).Scan(&id)
	return id, err
}

// ---------- teacher preview (stage 8): separate tables, never mixed with student attempts ----------

// previewCols reads a preview attempt into Attempt: no classroom (uuid.Nil), the teacher in StudentID.
const previewCols = `id, '00000000-0000-0000-0000-000000000000'::uuid, exercise_id, version_id, teacher_id, status, started_at, deadline_at,
	finished_at, finish_reason, correct_count, wrong_count, unanswered_count, points_earned, points_max`

// PreviewStart inserts a live preview attempt if none exists and returns the live one (idempotent).
func (r *Repository) PreviewStart(ctx context.Context, exerciseID, versionID, teacherID uuid.UUID, timerSeconds *int) (*Attempt, error) {
	if _, err := r.pool.Exec(ctx, `INSERT INTO preview_attempts (exercise_id, version_id, teacher_id, deadline_at)
		VALUES ($1, $2, $3, CASE WHEN $4::int IS NULL THEN NULL ELSE now() + make_interval(secs => $4::int) END)
		ON CONFLICT (exercise_id, teacher_id) WHERE status <> 'reset' DO NOTHING`, exerciseID, versionID, teacherID, timerSeconds); err != nil {
		return nil, err
	}
	var a Attempt
	err := scanAtt(r.pool.QueryRow(ctx, `SELECT `+previewCols+` FROM preview_attempts
		WHERE exercise_id = $1 AND teacher_id = $2 AND status <> 'reset'`, exerciseID, teacherID), &a)
	return &a, err
}

// PreviewGet reads one preview attempt (optionally locked).
func (r *Repository) PreviewGet(ctx context.Context, q database.Querier, id uuid.UUID, forUpdate bool) (*Attempt, error) {
	sql := `SELECT ` + previewCols + ` FROM preview_attempts WHERE id = $1`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	var a Attempt
	if err := scanAtt(q.QueryRow(ctx, sql, id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// PreviewLive returns the teacher's live preview attempts keyed by exercise.
func (r *Repository) PreviewLive(ctx context.Context, teacherID uuid.UUID, exerciseIDs []uuid.UUID) (map[uuid.UUID]*Attempt, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+previewCols+` FROM preview_attempts
		WHERE teacher_id = $1 AND exercise_id = ANY($2) AND status <> 'reset'`, teacherID, exerciseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]*Attempt{}
	for rows.Next() {
		var a Attempt
		if err := scanAtt(rows, &a); err != nil {
			return nil, err
		}
		out[a.ExerciseID] = &a
	}
	return out, rows.Err()
}

// PreviewResetCounts counts archived preview attempts per exercise.
func (r *Repository) PreviewResetCounts(ctx context.Context, teacherID uuid.UUID, exerciseIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT exercise_id, count(*) FROM preview_attempts
		WHERE teacher_id = $1 AND exercise_id = ANY($2) AND status = 'reset' GROUP BY exercise_id`, teacherID, exerciseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// PreviewAnswers reads preview answers keyed by attempt and question.
func (r *Repository) PreviewAnswers(ctx context.Context, q database.Querier, attemptIDs []uuid.UUID) (map[uuid.UUID]map[uuid.UUID]Answer, error) {
	out := map[uuid.UUID]map[uuid.UUID]Answer{}
	if len(attemptIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT attempt_id, question_id, selected_option_ids, answer_text, checked, updated_at FROM preview_answers WHERE attempt_id = ANY($1)`, attemptIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var aid uuid.UUID
		var a Answer
		if err := rows.Scan(&aid, &a.QuestionID, &a.Selected, &a.Text, &a.Checked, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if out[aid] == nil {
			out[aid] = map[uuid.UUID]Answer{}
		}
		out[aid][a.QuestionID] = a
	}
	return out, rows.Err()
}

func (r *Repository) PreviewUpsertAnswer(ctx context.Context, q database.Querier, attemptID, questionID uuid.UUID, resp exercises.Response, checked bool) error {
	_, err := q.Exec(ctx, `INSERT INTO preview_answers (attempt_id, question_id, selected_option_ids, answer_text, checked) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (attempt_id, question_id) DO UPDATE SET selected_option_ids = EXCLUDED.selected_option_ids,
		answer_text = EXCLUDED.answer_text, checked = EXCLUDED.checked, updated_at = now()`, attemptID, questionID, resp.Selected, resp.Text, checked)
	if err == nil {
		_, err = q.Exec(ctx, `UPDATE preview_attempts SET updated_at = now() WHERE id = $1`, attemptID)
	}
	return err
}

func (r *Repository) PreviewFinish(ctx context.Context, q database.Querier, id uuid.UUID, reason string, at time.Time, s Summary) error {
	_, err := q.Exec(ctx, `UPDATE preview_attempts SET status = 'finished', finished_at = $3, finish_reason = $2,
		correct_count = $4, wrong_count = $5, unanswered_count = $6, points_earned = $7, points_max = $8, updated_at = now()
		WHERE id = $1 AND status = 'in_progress'`, id, reason, at, s.Correct, s.Wrong, s.Unanswered, s.PointsEarned, s.PointsMax)
	return err
}

// PreviewReset archives the teacher's live preview attempts of the given exercises (repeat-safe).
func (r *Repository) PreviewReset(ctx context.Context, teacherID uuid.UUID, exerciseIDs []uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE preview_attempts SET status = 'reset', reset_at = now(), updated_at = now()
		WHERE teacher_id = $1 AND exercise_id = ANY($2) AND status <> 'reset'`, teacherID, exerciseIDs)
	return tag.RowsAffected(), err
}
