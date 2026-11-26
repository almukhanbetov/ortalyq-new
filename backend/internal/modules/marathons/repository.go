package marathons

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Create inserts the classroom, the owner's teacher membership and the settings row atomically.
func (r *Repository) Create(ctx context.Context, ownerID uuid.UUID, req SaveRequest, timezone string) (uuid.UUID, error) {
	var id uuid.UUID
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO classrooms (name, lesson_type, owner_id, cover_color)
			VALUES ($1, 'marathon', $2, $3) RETURNING id`, req.Name, ownerID, req.CoverColor).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO classroom_members (classroom_id, user_id, role) VALUES ($1, $2, 'teacher')`, id, ownerID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO marathon_settings (classroom_id, direction, target_language, timezone)
			VALUES ($1, $2, $3, $4)`, id, req.Direction, req.TargetLanguage, timezone)
		return err
	})
	return id, err
}

// Update rewrites the basic fields. The settings row is locked so concurrent saves apply one after another.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, req SaveRequest) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM marathon_settings WHERE classroom_id = $1 FOR UPDATE`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE classrooms SET name = $2, cover_color = $3, updated_at = now() WHERE id = $1`,
			id, req.Name, req.CoverColor); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE marathon_settings SET direction = $2, target_language = $3, updated_at = now()
			WHERE classroom_id = $1`, id, req.Direction, req.TargetLanguage)
		return err
	})
}

// Archive hides the marathon; repeated calls keep the first archive time.
func (r *Repository) Archive(ctx context.Context, id, actorID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE classrooms SET archived_at = now(), archived_by = $2, updated_at = now()
		WHERE id = $1 AND lesson_type = 'marathon' AND archived_at IS NULL`, id, actorID)
	return err
}

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// errRollback aborts a transaction without reporting an error to the caller.
var errRollback = errors.New("rollback")

// State reads the access-relevant settings with now() of the DB. Returns pgx.ErrNoRows for non-marathons.
func (r *Repository) State(ctx context.Context, q database.Querier, classroomID uuid.UUID) (*state, error) {
	var s state
	err := q.QueryRow(ctx, `SELECT end_at, (end_at IS NOT NULL AND end_at <= now()), now(), start_at, auto_deactivate_days, timezone,
			mode, progress_epoch, interval_days, weekdays, extract(hour FROM open_time)::int, teacher_check, unlock_after,
			stats_enabled, rating_enabled, rating_limit, rating_show_place
		FROM marathon_settings WHERE classroom_id = $1`, classroomID).Scan(&s.EndAt, &s.Finished, &s.Now, &s.StartAt, &s.AutoDays, &s.Timezone,
		&s.Mode, &s.Epoch, &s.IntervalDays, &s.Weekdays, &s.OpenHour, &s.TeacherCheck, &s.UnlockAfter,
		&s.StatsEnabled, &s.RatingEnabled, &s.RatingLimit, &s.RatingShowPlace)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// SetPassage stores the passage settings under the settings row lock.
func (r *Repository) SetPassage(ctx context.Context, id uuid.UUID, req PassageRequest) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM marathon_settings WHERE classroom_id = $1 FOR UPDATE`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE marathon_settings SET start_at = $2, end_at = $3, auto_deactivate_days = $4, updated_at = now()
			WHERE classroom_id = $1`, id, req.StartAt, req.EndAt, req.AutoDeactivateDays)
		return err
	})
}

// lockSettings serialises lesson changes of one marathon (positions, the own course, removals).
func lockSettings(ctx context.Context, tx pgx.Tx, classroomID uuid.UUID) error {
	var one int
	return tx.QueryRow(ctx, `SELECT 1 FROM marathon_settings WHERE classroom_id = $1 FOR UPDATE`, classroomID).Scan(&one)
}

// appendLesson puts a lesson at the end of the marathon (idempotent for an active lesson; a removed one is
// restored with its progress) and opens it in the classroom so the shared lesson checks apply.
func appendLesson(ctx context.Context, tx pgx.Tx, classroomID, lessonID, courseID uuid.UUID, cover *string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO marathon_lessons (classroom_id, lesson_id, position, cover_color)
		VALUES ($1, $2, (SELECT COALESCE(max(position), 0) + 1 FROM marathon_lessons WHERE classroom_id = $1), $3)
		ON CONFLICT (classroom_id, lesson_id) DO UPDATE
		SET removed_at = NULL, removed_by = NULL, position = EXCLUDED.position
		WHERE marathon_lessons.removed_at IS NOT NULL`, classroomID, lessonID, cover); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO classroom_courses (classroom_id, course_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, classroomID, courseID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO classroom_lessons (classroom_id, lesson_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, classroomID, lessonID)
	return err
}

// AddLesson appends a lesson from the teacher's materials.
func (r *Repository) AddLesson(ctx context.Context, classroomID, lessonID, courseID uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		return appendLesson(ctx, tx, classroomID, lessonID, courseID, nil)
	})
}

// ownCourse returns the marathon's own course, creating it on first use with the marathon name and colour.
// The caller holds the settings lock, so a double click never creates two courses.
func ownCourse(ctx context.Context, tx pgx.Tx, classroomID, ownerID uuid.UUID) (uuid.UUID, error) {
	var id *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT ms.course_id FROM marathon_settings ms
		JOIN courses co ON co.id = ms.course_id WHERE ms.classroom_id = $1`, classroomID).Scan(&id); err != nil && !database.IsNoRows(err) {
		return uuid.Nil, err
	}
	if id != nil {
		return *id, nil
	}
	var cid uuid.UUID
	if err := tx.QueryRow(ctx, `INSERT INTO courses (owner_id, title, cover_color, position)
		SELECT $2, c.name, c.cover_color, (SELECT COALESCE(max(position), -1) + 1 FROM courses WHERE owner_id = $2)
		FROM classrooms c WHERE c.id = $1 RETURNING id`, classroomID, ownerID).Scan(&cid); err != nil {
		return uuid.Nil, err
	}
	_, err := tx.Exec(ctx, `UPDATE marathon_settings SET course_id = $2, updated_at = now() WHERE classroom_id = $1`, classroomID, cid)
	return cid, err
}

// CreateLesson creates a new lesson in the marathon's own course and appends it to the marathon, atomically.
func (r *Repository) CreateLesson(ctx context.Context, classroomID, ownerID uuid.UUID, title, description, cover string) (uuid.UUID, error) {
	var id uuid.UUID
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		courseID, err := ownCourse(ctx, tx, classroomID, ownerID)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO lessons (course_id, title, description, position)
			VALUES ($1, $2, $3, (SELECT COALESCE(max(position), -1) + 1 FROM lessons WHERE course_id = $1)) RETURNING id`,
			courseID, title, description).Scan(&id); err != nil {
			return err
		}
		return appendLesson(ctx, tx, classroomID, id, courseID, &cover)
	})
	return id, err
}

type srcExercise struct {
	id, versionID uuid.UUID
	scope, kind   string
	sectionID     *uuid.UUID
	position      int
	hidden        bool
}

// CopyLesson copies a marathon lesson (sections and the current version of every exercise, with questions,
// options and the answer key) into the marathon's own course and appends the copy to the marathon.
// The copy is a new template: attempts, progress and results of the original are not copied.
func (r *Repository) CopyLesson(ctx context.Context, classroomID, ownerID, srcID uuid.UUID, titleSuffix string) (uuid.UUID, error) {
	var newID uuid.UUID
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		var cover *string
		if err := tx.QueryRow(ctx, `SELECT cover_color FROM marathon_lessons
			WHERE classroom_id = $1 AND lesson_id = $2 AND removed_at IS NULL`, classroomID, srcID).Scan(&cover); err != nil {
			return err // pgx.ErrNoRows: not a lesson of this marathon
		}
		courseID, err := ownCourse(ctx, tx, classroomID, ownerID)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO lessons (course_id, title, description, position, language, age, level,
				lesson_type, duration_min, skills, grammar, vocabulary, functions, tags)
			SELECT $2, left(title, 200 - char_length($3)) || $3, description,
				(SELECT COALESCE(max(position), -1) + 1 FROM lessons WHERE course_id = $2),
				language, age, level, lesson_type, duration_min, skills, grammar, vocabulary, functions, tags
			FROM lessons WHERE id = $1 RETURNING id`, srcID, courseID, titleSuffix).Scan(&newID); err != nil {
			return err
		}

		// Sections, keeping their order; the map translates old section ids for the exercises.
		rows, err := tx.Query(ctx, `SELECT id, title, position FROM sections WHERE lesson_id = $1 ORDER BY position, created_at`, srcID)
		if err != nil {
			return err
		}
		type sec struct {
			id    uuid.UUID
			title string
			pos   int
		}
		secs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sec, error) {
			var x sec
			return x, row.Scan(&x.id, &x.title, &x.pos)
		})
		if err != nil {
			return err
		}
		secMap := map[uuid.UUID]uuid.UUID{}
		for _, x := range secs {
			var id uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO sections (lesson_id, title, position) VALUES ($1, $2, $3) RETURNING id`,
				newID, x.title, x.pos).Scan(&id); err != nil {
				return err
			}
			secMap[x.id] = id
		}

		// Exercises: only the current version of every exercise that is not deleted.
		rows, err = tx.Query(ctx, `SELECT id, current_version_id, scope, kind, section_id, position, hidden FROM exercises
			WHERE lesson_id = $1 AND deleted_at IS NULL AND current_version_id IS NOT NULL
			ORDER BY scope, position, created_at`, srcID)
		if err != nil {
			return err
		}
		exs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (srcExercise, error) {
			var e srcExercise
			return e, row.Scan(&e.id, &e.versionID, &e.scope, &e.kind, &e.sectionID, &e.position, &e.hidden)
		})
		if err != nil {
			return err
		}
		for _, e := range exs {
			var section *uuid.UUID
			if e.sectionID != nil {
				m, ok := secMap[*e.sectionID]
				if !ok {
					continue // the section is gone; such an exercise is not shown in the lesson either
				}
				section = &m
			}
			var exID, verID uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO exercises (lesson_id, scope, section_id, kind, position, hidden, copied_from_id, created_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
				newID, e.scope, section, e.kind, e.position, e.hidden, e.id, ownerID).Scan(&exID); err != nil {
				return err
			}
			// file_id: a PDF block of the copy shows the same stored file; it is served through the copy's own
			// exercise, so the copy's access rules apply to it.
			if err := tx.QueryRow(ctx, `INSERT INTO exercise_versions (exercise_id, version, title, check_mode, timer_enabled,
					timer_seconds, scoring_enabled, points_per_correct, category_id, created_by, file_id)
				SELECT $2, 1, title, check_mode, timer_enabled, timer_seconds, scoring_enabled, points_per_correct, category_id, $3, file_id
				FROM exercise_versions WHERE id = $1 RETURNING id`, e.versionID, exID, ownerID).Scan(&verID); err != nil {
				return err
			}
			qrows, err := tx.Query(ctx, `SELECT id FROM exercise_questions WHERE version_id = $1 ORDER BY position`, e.versionID)
			if err != nil {
				return err
			}
			qids, err := pgx.CollectRows(qrows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			for _, qid := range qids {
				var nq uuid.UUID
				if err := tx.QueryRow(ctx, `INSERT INTO exercise_questions (version_id, position, prompt, multiple)
					SELECT $2, position, prompt, multiple FROM exercise_questions WHERE id = $1 RETURNING id`, qid, verID).Scan(&nq); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO exercise_options (question_id, position, label, is_correct)
					SELECT $2, position, label, is_correct FROM exercise_options WHERE question_id = $1`, qid, nq); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE exercises SET current_version_id = $2 WHERE id = $1`, exID, verID); err != nil {
				return err
			}
		}
		return appendLesson(ctx, tx, classroomID, newID, courseID, cover)
	})
	return newID, err
}

// SetCover stores the marathon-level cover colour of a lesson (nil restores the course colour).
func (r *Repository) SetCover(ctx context.Context, classroomID, lessonID uuid.UUID, cover *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE marathon_lessons SET cover_color = $3
		WHERE classroom_id = $1 AND lesson_id = $2 AND removed_at IS NULL`, classroomID, lessonID, cover)
	return err
}

// ReorderLessons stores a new order. ok=false when ids is not exactly the set of active lessons.
func (r *Repository) ReorderLessons(ctx context.Context, classroomID uuid.UUID, ids []uuid.UUID) (bool, error) {
	ok := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM marathon_lessons WHERE classroom_id = $1 AND removed_at IS NULL`, classroomID).Scan(&n); err != nil {
			return err
		}
		if n != len(ids) {
			return nil
		}
		tag, err := tx.Exec(ctx, `UPDATE marathon_lessons ml SET position = o.ord
			FROM unnest($2::uuid[]) WITH ORDINALITY AS o(lesson_id, ord)
			WHERE ml.classroom_id = $1 AND ml.lesson_id = o.lesson_id AND ml.removed_at IS NULL`, classroomID, ids)
		if err != nil {
			return err
		}
		ok = int(tag.RowsAffected()) == n
		if !ok {
			return errRollback
		}
		return nil
	})
	if err == errRollback {
		return false, nil
	}
	return ok, err
}

// RemoveLesson takes a lesson out of the marathon: students lose access to it, the template, attempts and
// progress are kept. found=false when the lesson was never in the marathon. Repeated calls are harmless.
func (r *Repository) RemoveLesson(ctx context.Context, classroomID, lessonID, actorID uuid.UUID) (bool, error) {
	found := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM marathon_lessons WHERE classroom_id = $1 AND lesson_id = $2)`,
			classroomID, lessonID).Scan(&found); err != nil || !found {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE marathon_lessons SET removed_at = now(), removed_by = $3
			WHERE classroom_id = $1 AND lesson_id = $2 AND removed_at IS NULL`, classroomID, lessonID, actorID); err != nil {
			return err
		}
		// The classroom_lessons row is only the "lesson is open in this class" fact used by RequireLesson;
		// marathons keep their own progress in marathon_lesson_progress.
		_, err := tx.Exec(ctx, `DELETE FROM classroom_lessons WHERE classroom_id = $1 AND lesson_id = $2`, classroomID, lessonID)
		return err
	})
	return found, err
}

// Lessons lists the marathon lessons in order. With studentID set, the completion and the review state of the
// current epoch reflect that student. CompletedCount / PendingCount count current participants only.
func (r *Repository) Lessons(ctx context.Context, classroomID uuid.UUID, studentID *uuid.UUID) ([]Lesson, error) {
	rows, err := r.pool.Query(ctx, `SELECT l.id, l.title, l.description, co.title, COALESCE(ml.cover_color, co.cover_color), ml.cover_attachment_id, ml.position, ml.open_at,
			(SELECT count(*) FROM sections s WHERE s.lesson_id = l.id),
			(SELECT ARRAY[count(*) FILTER (WHERE p.status = 'completed'), count(*) FILTER (WHERE p.status = 'submitted')]
			   FROM marathon_lesson_progress p
			   JOIN marathon_enrollments e ON e.classroom_id = p.classroom_id AND e.student_id = p.student_id AND e.removed_at IS NULL
			  WHERE p.classroom_id = ml.classroom_id AND p.lesson_id = ml.lesson_id AND p.epoch = ms.progress_epoch),
			p.completed_at, COALESCE(p.status, 'none'), p.submitted_at, COALESCE(p.accepted, false), p.rating, p.teacher_comment
		FROM marathon_lessons ml
		JOIN marathon_settings ms ON ms.classroom_id = ml.classroom_id
		JOIN lessons l ON l.id = ml.lesson_id
		JOIN courses co ON co.id = l.course_id
		LEFT JOIN marathon_lesson_progress p ON p.classroom_id = ml.classroom_id AND p.lesson_id = ml.lesson_id
			AND p.student_id = $2 AND p.epoch = ms.progress_epoch
		WHERE ml.classroom_id = $1 AND ml.removed_at IS NULL
		ORDER BY ml.position`, classroomID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lesson{}
	for rows.Next() {
		var l Lesson
		var counts []int
		if err := rows.Scan(&l.LessonID, &l.Title, &l.Description, &l.CourseTitle, &l.CoverColor, &l.CoverImageID, &l.Position, &l.OpenAt, &l.SectionCount,
			&counts, &l.CompletedAt, &l.Status, &l.SubmittedAt, &l.Accepted, &l.Rating, &l.TeacherComment); err != nil {
			return nil, err
		}
		if len(counts) == 2 {
			l.CompletedCount, l.PendingCount = counts[0], counts[1]
		}
		l.Completed = l.Status == LessonCompleted
		out = append(out, l)
	}
	return out, rows.Err()
}

// HasLesson reports whether the lesson belongs to the marathon.
func (r *Repository) HasLesson(ctx context.Context, q database.Querier, classroomID, lessonID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM marathon_lessons WHERE classroom_id = $1 AND lesson_id = $2 AND removed_at IS NULL)`, classroomID, lessonID).Scan(&ok)
	return ok, err
}

// AddStudents enrolls students (idempotent): enrollment and classroom membership in one transaction.
// A removed student comes back active, with the answers and progress kept from before.
func (r *Repository) AddStudents(ctx context.Context, classroomID, actorID uuid.UUID, ids []uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		return addStudentsTx(ctx, tx, classroomID, actorID, ids)
	})
}

// addStudentsTx is AddStudents inside the caller's transaction (also used when an invitation is accepted).
func addStudentsTx(ctx context.Context, tx pgx.Tx, classroomID, actorID uuid.UUID, ids []uuid.UUID) error {
	rows, err := tx.Query(ctx, `INSERT INTO marathon_enrollments (classroom_id, student_id, added_by)
		SELECT $1, unnest($2::uuid[]), $3
		ON CONFLICT (classroom_id, student_id) DO UPDATE
		SET removed_at = NULL, removed_by = NULL, deactivated_at = NULL, deactivated_by = NULL,
			activated_at = now(), added_by = EXCLUDED.added_by
		WHERE marathon_enrollments.removed_at IS NOT NULL
		RETURNING student_id`, classroomID, ids, actorID)
	if err != nil {
		return err
	}
	added, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	if err := logEvents(ctx, tx, classroomID, added, "added", actorID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO classroom_members (classroom_id, user_id, role)
		SELECT $1, unnest($2::uuid[]), 'student' ON CONFLICT DO NOTHING`, classroomID, ids)
	return err
}

func logEvents(ctx context.Context, tx pgx.Tx, classroomID uuid.UUID, ids []uuid.UUID, event string, actorID uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO marathon_enrollment_events (classroom_id, student_id, event, actor_id)
		SELECT $1, unnest($2::uuid[]), $3, $4`, classroomID, ids, event, actorID)
	return err
}

// SetActive activates or deactivates current participants. ok=false (nothing changed) when any id is not a
// participant. Rows are locked, so a concurrent answer of the student waits for the decision.
// Repeating the same action keeps the first date and author.
func (r *Repository) SetActive(ctx context.Context, classroomID, actorID uuid.UUID, ids []uuid.UUID, active bool) (bool, error) {
	ok := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT student_id FROM marathon_enrollments
			WHERE classroom_id = $1 AND student_id = ANY($2::uuid[]) AND removed_at IS NULL FOR UPDATE`, classroomID, ids)
		if err != nil {
			return err
		}
		found, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		if len(found) != len(ids) {
			return nil
		}
		ok = true
		q := `UPDATE marathon_enrollments SET deactivated_at = now(), deactivated_by = $3
			WHERE classroom_id = $1 AND student_id = ANY($2::uuid[]) AND deactivated_at IS NULL RETURNING student_id`
		event := "deactivated"
		if active {
			q = `UPDATE marathon_enrollments SET deactivated_at = NULL, deactivated_by = NULL, reactivated_at = now(), reactivated_by = $3
				WHERE classroom_id = $1 AND student_id = ANY($2::uuid[]) AND deactivated_at IS NOT NULL RETURNING student_id`
			event = "activated"
		}
		rows, err = tx.Query(ctx, q, classroomID, ids, actorID)
		if err != nil {
			return err
		}
		changed, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		return logEvents(ctx, tx, classroomID, changed, event, actorID)
	})
	return ok, err
}

// RemoveStudent takes a student out of the marathon: the membership goes (so every classroom check answers 404),
// answers, attempts and progress stay. found=false when the student was never enrolled. Repeats are harmless.
func (r *Repository) RemoveStudent(ctx context.Context, classroomID, studentID, actorID uuid.UUID) (bool, error) {
	found := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var removed bool
		err := tx.QueryRow(ctx, `SELECT removed_at IS NOT NULL FROM marathon_enrollments
			WHERE classroom_id = $1 AND student_id = $2 FOR UPDATE`, classroomID, studentID).Scan(&removed)
		if database.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if removed {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE marathon_enrollments SET removed_at = now(), removed_by = $3
			WHERE classroom_id = $1 AND student_id = $2`, classroomID, studentID, actorID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM classroom_members WHERE classroom_id = $1 AND user_id = $2 AND role = 'student'`,
			classroomID, studentID); err != nil {
			return err
		}
		return logEvents(ctx, tx, classroomID, []uuid.UUID{studentID}, "removed", actorID)
	})
	return found, err
}

// Participation reads the student's enrollment (nil for removed or unknown students). The row is locked for
// share, so an answer that races with a deactivation waits and then sees the new state.
func (r *Repository) Participation(ctx context.Context, q database.Querier, classroomID, studentID uuid.UUID) (*AccessEnrollment, error) {
	var e AccessEnrollment
	err := q.QueryRow(ctx, `SELECT activated_at, deactivated_at FROM marathon_enrollments
		WHERE classroom_id = $1 AND student_id = $2 AND removed_at IS NULL FOR SHARE`, classroomID, studentID).Scan(&e.ActivatedAt, &e.DeactivatedAt)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Students lists enrolled students with the number of completed lessons. Status is filled by the service.
func (r *Repository) Students(ctx context.Context, classroomID uuid.UUID) ([]Student, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id, u.full_name, u.email, u.avatar_color, e.activated_at,
			e.deactivated_at, d.full_name, e.reactivated_at,
			(SELECT ARRAY[count(*) FILTER (WHERE p.status = 'completed'), count(*) FILTER (WHERE p.status = 'submitted')]
			   FROM marathon_lesson_progress p
			   JOIN marathon_lessons ml ON ml.classroom_id = p.classroom_id AND ml.lesson_id = p.lesson_id AND ml.removed_at IS NULL
			  WHERE p.classroom_id = e.classroom_id AND p.student_id = e.student_id
				AND p.epoch = (SELECT progress_epoch FROM marathon_settings WHERE classroom_id = e.classroom_id)),
			(SELECT count(*) FROM marathon_lessons ml WHERE ml.classroom_id = e.classroom_id AND ml.removed_at IS NULL)
		FROM marathon_enrollments e JOIN users u ON u.id = e.student_id
		LEFT JOIN users d ON d.id = e.deactivated_by
		WHERE e.classroom_id = $1 AND e.removed_at IS NULL
		ORDER BY e.activated_at, u.full_name`, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Student{}
	for rows.Next() {
		var s Student
		var counts []int
		if err := rows.Scan(&s.ID, &s.FullName, &s.Email, &s.AvatarColor, &s.ActivatedAt,
			&s.DeactivatedAt, &s.DeactivatedByName, &s.ReactivatedAt, &counts, &s.TotalLessons); err != nil {
			return nil, err
		}
		if len(counts) == 2 {
			s.CompletedLessons, s.PendingLessons = counts[0], counts[1]
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Enrolled reports whether the user is a student of the marathon.
func (r *Repository) Enrolled(ctx context.Context, q database.Querier, classroomID, studentID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM marathon_enrollments WHERE classroom_id = $1 AND student_id = $2 AND removed_at IS NULL)`, classroomID, studentID).Scan(&ok)
	return ok, err
}

// LessonStates returns the active lessons in order with what decides their access for one student in the
// current epoch: opening date, completion, and whether the teacher opened it (for everyone or this student).
func (r *Repository) LessonStates(ctx context.Context, q database.Querier, classroomID, studentID uuid.UUID) ([]uuid.UUID, []LessonState, error) {
	rows, err := q.Query(ctx, `SELECT ml.lesson_id, ml.open_at,
			COALESCE((SELECT p.status FROM marathon_lesson_progress p WHERE p.classroom_id = ml.classroom_id AND p.lesson_id = ml.lesson_id
				AND p.student_id = $2 AND p.epoch = ms.progress_epoch), 'none'),
			EXISTS (SELECT 1 FROM marathon_lesson_unlocks u WHERE u.classroom_id = ml.classroom_id AND u.lesson_id = ml.lesson_id
				AND (u.student_id IS NULL OR u.student_id = $2) AND u.epoch = ms.progress_epoch)
		FROM marathon_lessons ml JOIN marathon_settings ms ON ms.classroom_id = ml.classroom_id
		WHERE ml.classroom_id = $1 AND ml.removed_at IS NULL
		ORDER BY ml.position`, classroomID, studentID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	var states []LessonState
	for rows.Next() {
		var id uuid.UUID
		var st LessonState
		var status string
		if err := rows.Scan(&id, &st.OpenAt, &status, &st.Unlocked); err != nil {
			return nil, nil, err
		}
		st.Completed = status == LessonCompleted
		st.Submitted = status == LessonSubmitted || status == LessonRevision
		ids, states = append(ids, id), append(states, st)
	}
	return ids, states, rows.Err()
}

// SetOpenAt stores the lesson opening date (nil removes it). found=false for a lesson not in the marathon.
func (r *Repository) SetOpenAt(ctx context.Context, classroomID, lessonID uuid.UUID, at *time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE marathon_lessons SET open_at = $3
		WHERE classroom_id = $1 AND lesson_id = $2 AND removed_at IS NULL`, classroomID, lessonID, at)
	return tag.RowsAffected() == 1, err
}

// Unlock opens a lesson in the closed mode for everyone (studentID nil) or for one student, in the current
// epoch. Idempotent (unique index); the settings row lock keeps it consistent with a concurrent mode change.
func (r *Repository) Unlock(ctx context.Context, classroomID, lessonID uuid.UUID, studentID *uuid.UUID, actorID uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var epoch int
		if err := tx.QueryRow(ctx, `SELECT progress_epoch FROM marathon_settings WHERE classroom_id = $1 FOR UPDATE`, classroomID).Scan(&epoch); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO marathon_lesson_unlocks (classroom_id, lesson_id, student_id, epoch, unlocked_by)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (classroom_id, lesson_id, COALESCE(student_id, '00000000-0000-0000-0000-000000000000'::uuid), epoch) DO NOTHING`,
			classroomID, lessonID, studentID, epoch, actorID)
		return err
	})
}

// UnlockedForAll lists lessons opened for everyone in the current epoch (closed mode, teacher view).
func (r *Repository) UnlockedForAll(ctx context.Context, classroomID uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.lesson_id FROM marathon_lesson_unlocks u
		JOIN marathon_settings ms ON ms.classroom_id = u.classroom_id AND u.epoch = ms.progress_epoch
		WHERE u.classroom_id = $1 AND u.student_id IS NULL`, classroomID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}

// ModeChange is the result of ChangeMode.
type ModeChange struct {
	Changed       bool // the mode changed: a new epoch started
	ParamsUpdated bool // same mode, its schedule parameters changed (no new epoch: statuses are kept)
	Conflict      bool // the epoch moved on and the mode differs: the caller saw stale settings
	DatesBlock    bool // the new mode does not use lesson opening dates and the reset was not confirmed
	DatesReset    int  // how many opening dates were reset (confirmed)
}

// Schedule holds the parameters of "interval" / "weekdays" (all nil for the other modes).
type Schedule struct {
	IntervalDays *int
	Weekdays     *int
	OpenHour     *int
}

func eqInt(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

// ChangeMode switches the passage mode and starts a new progress epoch, under the settings row lock.
// expectedEpoch makes a repeated request harmless: it does not reset progress a second time. With the same mode
// only the schedule parameters are updated, without a new epoch. Lesson opening dates are reset only when the new
// mode does not use them AND resetDates confirms it. Answers, attempts and points are never touched.
func (r *Repository) ChangeMode(ctx context.Context, classroomID, actorID uuid.UUID, mode string, sch Schedule, expectedEpoch int, resetDates bool) (ModeChange, error) {
	var out ModeChange
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var cur string
		var epoch int
		var old Schedule
		if err := tx.QueryRow(ctx, `SELECT mode, progress_epoch, interval_days, weekdays, extract(hour FROM open_time)::int
			FROM marathon_settings WHERE classroom_id = $1 FOR UPDATE`, classroomID).Scan(&cur, &epoch, &old.IntervalDays, &old.Weekdays, &old.OpenHour); err != nil {
			return err
		}
		if cur == mode {
			if eqInt(old.IntervalDays, sch.IntervalDays) && eqInt(old.Weekdays, sch.Weekdays) && eqInt(old.OpenHour, sch.OpenHour) {
				return nil
			}
			out.ParamsUpdated = true
			_, err := tx.Exec(ctx, `UPDATE marathon_settings SET interval_days = $2, weekdays = $3,
				open_time = CASE WHEN $4::int IS NULL THEN NULL ELSE make_time($4::int, 0, 0) END, updated_at = now()
				WHERE classroom_id = $1`, classroomID, sch.IntervalDays, sch.Weekdays, sch.OpenHour)
			return err
		}
		if epoch != expectedEpoch {
			out.Conflict = true
			return nil
		}
		if !modeUsesOpenDates(mode) {
			var dated int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM marathon_lessons
				WHERE classroom_id = $1 AND removed_at IS NULL AND open_at IS NOT NULL`, classroomID).Scan(&dated); err != nil {
				return err
			}
			if dated > 0 && !resetDates {
				out.DatesBlock = true
				return nil
			}
			if dated > 0 {
				if _, err := tx.Exec(ctx, `UPDATE marathon_lessons SET open_at = NULL WHERE classroom_id = $1 AND open_at IS NOT NULL`, classroomID); err != nil {
					return err
				}
				out.DatesReset = dated
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE marathon_settings SET mode = $2, progress_epoch = progress_epoch + 1,
				interval_days = $3, weekdays = $4, open_time = CASE WHEN $5::int IS NULL THEN NULL ELSE make_time($5::int, 0, 0) END,
				updated_at = now()
			WHERE classroom_id = $1`, classroomID, mode, sch.IntervalDays, sch.Weekdays, sch.OpenHour); err != nil {
			return err
		}
		out.Changed = true
		_, err := tx.Exec(ctx, `INSERT INTO marathon_mode_changes (classroom_id, from_mode, to_mode, from_epoch, to_epoch, changed_by, dates_reset)
			VALUES ($1, $2, $3, $4, $4 + 1, $5, $6)`, classroomID, cur, mode, epoch, actorID, out.DatesReset)
		return err
	})
	return out, err
}

// ---- Teacher check (stage 6) ----

// reviewState is what a status change of one student's lesson is decided on, read under LockReview.
type reviewState struct {
	Epoch        int
	TeacherCheck bool
	Status       string // LessonNone when the epoch has no progress row
	Accepted     bool
}

// LockReview serialises every status change of one student's lesson (submit, «Завершить урок», the teacher's
// review): a transaction advisory lock on the triple (two first submissions cannot both insert), the settings row
// FOR SHARE (a concurrent mode change waits) and the progress row of the current epoch FOR UPDATE.
func (r *Repository) LockReview(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID) (*reviewState, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('marathon-review/' || $1::text || '/' || $2::text || '/' || $3::text, 0))`,
		classroomID, lessonID, studentID); err != nil {
		return nil, err
	}
	st := reviewState{Status: LessonNone}
	if err := tx.QueryRow(ctx, `SELECT progress_epoch, teacher_check FROM marathon_settings WHERE classroom_id = $1 FOR SHARE`,
		classroomID).Scan(&st.Epoch, &st.TeacherCheck); err != nil {
		return nil, err
	}
	err := tx.QueryRow(ctx, `SELECT status, accepted FROM marathon_lesson_progress
		WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3 AND epoch = $4 FOR UPDATE`,
		classroomID, lessonID, studentID, st.Epoch).Scan(&st.Status, &st.Accepted)
	if err != nil && !database.IsNoRows(err) {
		return nil, err
	}
	return &st, nil
}

func lessonEvent(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, epoch int, from, to string, rating *string, actorID uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO marathon_lesson_events (classroom_id, lesson_id, student_id, epoch, from_status, to_status, rating, actor_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, classroomID, lessonID, studentID, epoch, from, to, rating, actorID)
	return err
}

// SelfComplete stores «Завершить урок» of the student (teacher check off): completed, not accepted.
// The caller holds LockReview and has checked the status is not completed.
func (r *Repository) SelfComplete(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, st *reviewState) error {
	if _, err := tx.Exec(ctx, `INSERT INTO marathon_lesson_progress (classroom_id, lesson_id, student_id, epoch, status, completed_at, completed_by)
		VALUES ($1, $2, $3, $4, 'completed', now(), $3)
		ON CONFLICT (classroom_id, lesson_id, student_id, epoch) DO UPDATE
		SET status = 'completed', completed_at = now(), completed_by = $3, accepted = false, updated_at = now()`,
		classroomID, lessonID, studentID, st.Epoch); err != nil {
		return err
	}
	return lessonEvent(ctx, tx, classroomID, lessonID, studentID, st.Epoch, st.Status, LessonCompleted, nil, studentID)
}

// Submit stores the student's «Отправить на проверку»: the immutable snapshot (numbered within the epoch) and the
// status submitted. The caller holds LockReview and has checked the status is none or revision.
func (r *Repository) Submit(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, st *reviewState, snapshot []byte) error {
	if _, err := tx.Exec(ctx, `INSERT INTO marathon_lesson_submissions (classroom_id, lesson_id, student_id, epoch, number, snapshot)
		VALUES ($1, $2, $3, $4, (SELECT COALESCE(max(number), 0) + 1 FROM marathon_lesson_submissions
			WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3 AND epoch = $4), $5)`,
		classroomID, lessonID, studentID, st.Epoch, snapshot); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO marathon_lesson_progress (classroom_id, lesson_id, student_id, epoch, status, submitted_at)
		VALUES ($1, $2, $3, $4, 'submitted', now())
		ON CONFLICT (classroom_id, lesson_id, student_id, epoch) DO UPDATE
		SET status = 'submitted', submitted_at = now(), completed_at = NULL, accepted = false, updated_at = now()`,
		classroomID, lessonID, studentID, st.Epoch); err != nil {
		return err
	}
	return lessonEvent(ctx, tx, classroomID, lessonID, studentID, st.Epoch, st.Status, LessonSubmitted, nil, studentID)
}

// Review applies the teacher's decision to an existing progress row (the caller holds LockReview and has checked
// the transition): revision; completed (accepted, with rating and comment); submitted («Отменить завершение»: the
// completion is removed, rating and comment stay as the previous review).
func (r *Repository) Review(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, st *reviewState, to string, rating, comment *string, actorID uuid.UUID) error {
	var q string
	args := []any{classroomID, lessonID, studentID, st.Epoch, actorID}
	switch to {
	case LessonRevision:
		q = `UPDATE marathon_lesson_progress SET status = 'revision', completed_at = NULL, accepted = false,
			reviewed_at = now(), reviewed_by = $5, updated_at = now()`
	case LessonCompleted:
		q = `UPDATE marathon_lesson_progress SET status = 'completed', completed_at = now(), completed_by = $5, accepted = true,
			reviewed_at = now(), reviewed_by = $5, rating = $6, teacher_comment = $7, updated_at = now()`
		args = append(args, rating, comment)
	case LessonSubmitted:
		q = `UPDATE marathon_lesson_progress SET status = 'submitted', completed_at = NULL, accepted = false,
			reviewed_at = now(), reviewed_by = $5, updated_at = now()`
	default:
		return errors.New("unknown review status")
	}
	tag, err := tx.Exec(ctx, q+` WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3 AND epoch = $4`, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("review: progress row is missing")
	}
	return lessonEvent(ctx, tx, classroomID, lessonID, studentID, st.Epoch, st.Status, to, rating, actorID)
}

// LessonLocked implements attempts.LessonReviewState: in the current epoch the lesson is sent for review or
// accepted by the teacher. Lessons completed by the student themselves (check off) stay editable, as before.
// Always false for regular classes (no marathon rows).
func (r *Repository) LessonLocked(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM marathon_lesson_progress p
		JOIN marathon_settings ms ON ms.classroom_id = p.classroom_id AND p.epoch = ms.progress_epoch
		WHERE p.classroom_id = $1 AND p.lesson_id = $2 AND p.student_id = $3 AND (p.status = 'submitted' OR p.accepted))`,
		classroomID, lessonID, studentID).Scan(&ok)
	return ok, err
}

// SetReviewSettings stores "Проверка учителем" and "Открытие уроков" under the settings row lock.
// changed=false when nothing differs (a repeated save).
func (r *Repository) SetReviewSettings(ctx context.Context, id uuid.UUID, teacherCheck bool, unlockAfter string) (bool, error) {
	changed := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var cur bool
		var curUnlock string
		if err := tx.QueryRow(ctx, `SELECT teacher_check, unlock_after FROM marathon_settings WHERE classroom_id = $1 FOR UPDATE`, id).Scan(&cur, &curUnlock); err != nil {
			return err
		}
		if cur == teacherCheck && curUnlock == unlockAfter {
			return nil
		}
		changed = true
		_, err := tx.Exec(ctx, `UPDATE marathon_settings SET teacher_check = $2, unlock_after = $3, updated_at = now() WHERE classroom_id = $1`,
			id, teacherCheck, unlockAfter)
		return err
	})
	return changed, err
}

// SubmissionsCount counts the snapshots of a student's lesson in the current epoch.
func (r *Repository) SubmissionsCount(ctx context.Context, classroomID, lessonID, studentID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_lesson_submissions s
		JOIN marathon_settings ms ON ms.classroom_id = s.classroom_id AND s.epoch = ms.progress_epoch
		WHERE s.classroom_id = $1 AND s.lesson_id = $2 AND s.student_id = $3`, classroomID, lessonID, studentID).Scan(&n)
	return n, err
}

// ---- Statistics and rating (stage 7) ----

// SetStatsSettings stores "Статистика", "Рейтинг учеников", "Отображать" and "Место ученика" under the settings
// row lock. changed=false for a repeated save.
func (r *Repository) SetStatsSettings(ctx context.Context, id uuid.UUID, stats, rating bool, limit *int, showPlace bool) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE marathon_settings SET stats_enabled = $2, rating_enabled = $3, rating_limit = $4,
			rating_show_place = $5, updated_at = now()
		WHERE classroom_id = $1 AND (stats_enabled, rating_enabled, rating_limit, rating_show_place)
			IS DISTINCT FROM ($2, $3, $4::smallint, $5)`, id, stats, rating, limit, showPlace)
	return tag.RowsAffected() == 1, err
}

// LessonComment is «Комментарий к статистике» of a lesson in this marathon.
type LessonComment struct {
	Body          string    `json:"body"`
	UpdatedAt     time.Time `json:"updated_at"`
	UpdatedByName *string   `json:"updated_by_name"`
}

// Comment reads the statistics comment of a marathon lesson (nil when there is none).
func (r *Repository) Comment(ctx context.Context, classroomID, lessonID uuid.UUID) (*LessonComment, error) {
	var c LessonComment
	err := r.pool.QueryRow(ctx, `SELECT c.body, c.updated_at, u.full_name FROM marathon_lesson_comments c
		LEFT JOIN users u ON u.id = c.updated_by WHERE c.classroom_id = $1 AND c.lesson_id = $2`, classroomID, lessonID).
		Scan(&c.Body, &c.UpdatedAt, &c.UpdatedByName)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SetComment stores the statistics comment of a marathon lesson; an empty body removes it. Repeats are harmless.
func (r *Repository) SetComment(ctx context.Context, classroomID, lessonID uuid.UUID, body string, actorID uuid.UUID) error {
	if body == "" {
		_, err := r.pool.Exec(ctx, `DELETE FROM marathon_lesson_comments WHERE classroom_id = $1 AND lesson_id = $2`, classroomID, lessonID)
		return err
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO marathon_lesson_comments (classroom_id, lesson_id, body, updated_by) VALUES ($1, $2, $3, $4)
		ON CONFLICT (classroom_id, lesson_id) DO UPDATE SET body = EXCLUDED.body, updated_by = EXCLUDED.updated_by, updated_at = now()
		WHERE marathon_lesson_comments.body IS DISTINCT FROM EXCLUDED.body`, classroomID, lessonID, body, actorID)
	return err
}

// CompletedByLesson returns, for the current epoch, which participants completed which active lessons
// (lesson → student → true). Used for the lesson results and the statistics.
func (r *Repository) CompletedByLesson(ctx context.Context, classroomID uuid.UUID) (map[uuid.UUID]map[uuid.UUID]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.lesson_id, p.student_id FROM marathon_lesson_progress p
		JOIN marathon_settings ms ON ms.classroom_id = p.classroom_id AND p.epoch = ms.progress_epoch
		WHERE p.classroom_id = $1 AND p.status = 'completed'`, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]map[uuid.UUID]bool{}
	for rows.Next() {
		var lid, sid uuid.UUID
		if err := rows.Scan(&lid, &sid); err != nil {
			return nil, err
		}
		if out[lid] == nil {
			out[lid] = map[uuid.UUID]bool{}
		}
		out[lid][sid] = true
	}
	return out, rows.Err()
}

// ---- Invitations by e-mail (stage 9) ----

// InvitationRow is an invitation as stored (the token is never stored, only its SHA-256).
type InvitationRow struct {
	ID          uuid.UUID
	ClassroomID uuid.UUID
	Email       string
	StudentID   uuid.UUID
	Locale      string
	InvitedBy   *uuid.UUID
	ExpiresAt   time.Time
	Expired     bool // expires_at <= now() of the DB
	AcceptedAt  *time.Time
	AcceptedBy  *uuid.UUID
	RevokedAt   *time.Time
	// For the public page and the acceptance: the marathon, its owner and whether it is archived.
	MarathonName string
	OwnerID      uuid.UUID
	TeacherName  string
	Archived     bool
}

const invCols = `i.id, i.classroom_id, i.email, i.student_id, i.locale, i.invited_by, i.expires_at, i.expires_at <= now(),
	i.accepted_at, i.accepted_by, i.revoked_at, c.name, c.owner_id, o.full_name, c.archived_at IS NOT NULL`

const invFrom = ` FROM marathon_invitations i JOIN classrooms c ON c.id = i.classroom_id JOIN users o ON o.id = c.owner_id`

func scanInv(row pgx.Row) (*InvitationRow, error) {
	var x InvitationRow
	err := row.Scan(&x.ID, &x.ClassroomID, &x.Email, &x.StudentID, &x.Locale, &x.InvitedBy, &x.ExpiresAt, &x.Expired,
		&x.AcceptedAt, &x.AcceptedBy, &x.RevokedAt, &x.MarathonName, &x.OwnerID, &x.TeacherName, &x.Archived)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &x, nil
}

// OpenInvitation locks the open (not accepted, not revoked) invitation of an address in a marathon; nil if none.
func (r *Repository) OpenInvitation(ctx context.Context, tx pgx.Tx, classroomID uuid.UUID, email string) (*InvitationRow, error) {
	return scanInv(tx.QueryRow(ctx, `SELECT `+invCols+invFrom+` WHERE i.classroom_id = $1 AND i.email = $2
		AND i.accepted_at IS NULL AND i.revoked_at IS NULL FOR UPDATE OF i`, classroomID, email))
}

// InvitationByID locks an invitation of the marathon; nil if none.
func (r *Repository) InvitationByID(ctx context.Context, tx pgx.Tx, classroomID, id uuid.UUID) (*InvitationRow, error) {
	return scanInv(tx.QueryRow(ctx, `SELECT `+invCols+invFrom+` WHERE i.classroom_id = $1 AND i.id = $2 FOR UPDATE OF i`, classroomID, id))
}

// InvitationByToken finds an invitation by the hash of its token (locked when tx is a transaction); nil if none.
func (r *Repository) InvitationByToken(ctx context.Context, q database.Querier, hash []byte, lock bool) (*InvitationRow, error) {
	sql := `SELECT ` + invCols + invFrom + ` WHERE i.token_hash = $1`
	if lock {
		sql += ` FOR UPDATE OF i`
	}
	return scanInv(q.QueryRow(ctx, sql, hash))
}

// CreateInvitation inserts an open invitation; ok=false when a concurrent request created it first.
func (r *Repository) CreateInvitation(ctx context.Context, tx pgx.Tx, classroomID, studentID, actorID uuid.UUID, email, locale string, hash []byte, ttl time.Duration) (uuid.UUID, time.Time, bool, error) {
	var id uuid.UUID
	var exp time.Time
	err := tx.QueryRow(ctx, `INSERT INTO marathon_invitations (classroom_id, email, student_id, locale, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, now() + make_interval(secs => $7))
		ON CONFLICT (classroom_id, email) WHERE accepted_at IS NULL AND revoked_at IS NULL DO NOTHING
		RETURNING id, expires_at`, classroomID, email, studentID, locale, hash, actorID, ttl.Seconds()).Scan(&id, &exp)
	if database.IsNoRows(err) {
		return uuid.Nil, exp, false, nil
	}
	return id, exp, err == nil, err
}

// RenewInvitation gives an open invitation a new token and a new period (the old link stops working).
func (r *Repository) RenewInvitation(ctx context.Context, tx pgx.Tx, id uuid.UUID, locale string, hash []byte, ttl time.Duration) (time.Time, error) {
	var exp time.Time
	err := tx.QueryRow(ctx, `UPDATE marathon_invitations SET token_hash = $2, locale = $3, expires_at = now() + make_interval(secs => $4),
		sent_at = now(), sends = sends + 1 WHERE id = $1 RETURNING expires_at`, id, hash, locale, ttl.Seconds()).Scan(&exp)
	return exp, err
}

func (r *Repository) SetInvitationOutbox(ctx context.Context, tx pgx.Tx, id, outboxID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE marathon_invitations SET outbox_id = $2 WHERE id = $1`, id, outboxID)
	return err
}

// AcceptInvitation marks an invitation accepted by the user.
func (r *Repository) AcceptInvitation(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE marathon_invitations SET accepted_at = now(), accepted_by = $2 WHERE id = $1 AND accepted_at IS NULL`, id, userID)
	return err
}

// RevokeInvitation cancels an open invitation; repeats are harmless.
func (r *Repository) RevokeInvitation(ctx context.Context, tx pgx.Tx, id, actorID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE marathon_invitations SET revoked_at = now(), revoked_by = $2 WHERE id = $1 AND revoked_at IS NULL AND accepted_at IS NULL`, id, actorID)
	return err
}

// Invitations lists the invitations of a marathon with the delivery state of their last letter.
func (r *Repository) Invitations(ctx context.Context, classroomID uuid.UUID) ([]Invitation, error) {
	rows, err := r.pool.Query(ctx, `SELECT i.id, i.email, i.student_id, u.full_name, u.avatar_color, i.locale, i.created_at, i.sent_at, i.sends,
			i.expires_at, i.expires_at <= now(), i.accepted_at, i.revoked_at, COALESCE(o.status, 'queued'), o.attempts, o.last_error,
			EXISTS (SELECT 1 FROM marathon_enrollments e WHERE e.classroom_id = i.classroom_id AND e.student_id = i.student_id AND e.removed_at IS NULL)
		FROM marathon_invitations i JOIN users u ON u.id = i.student_id LEFT JOIN email_outbox o ON o.id = i.outbox_id
		WHERE i.classroom_id = $1 ORDER BY i.created_at DESC`, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var x Invitation
		var expired, participant bool
		var attempts *int
		if err := rows.Scan(&x.ID, &x.Email, &x.StudentID, &x.StudentName, &x.AvatarColor, &x.Locale, &x.CreatedAt, &x.SentAt, &x.Sends,
			&x.ExpiresAt, &expired, &x.AcceptedAt, &x.RevokedAt, &x.Delivery, &attempts, &x.DeliveryError, &participant); err != nil {
			return nil, err
		}
		switch {
		case x.AcceptedAt != nil:
			x.Status = InviteAccepted
		case x.RevokedAt != nil:
			x.Status = InviteRevoked
		case participant:
			x.Status = InviteParticipant
		case expired:
			x.Status = InviteExpired
		default:
			x.Status = InvitePending
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// ---- Appearance (stage 10) ----

const appearanceCols = `enabled, reactions_enabled, reactions, background_enabled, background, background_attachment_id`

func scanAppearance(row pgx.Row) (*Appearance, error) {
	var a Appearance
	if err := row.Scan(&a.Enabled, &a.ReactionsEnabled, &a.Reactions, &a.BackgroundEnabled, &a.Background, &a.BackgroundImageID); err != nil {
		if database.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// Appearance reads the marathon appearance (lessonID nil) or the own appearance of a lesson; nil when never saved.
func (r *Repository) Appearance(ctx context.Context, q database.Querier, classroomID uuid.UUID, lessonID *uuid.UUID) (*Appearance, error) {
	return scanAppearance(q.QueryRow(ctx, `SELECT `+appearanceCols+` FROM marathon_appearances
		WHERE classroom_id = $1 AND lesson_id IS NOT DISTINCT FROM $2`, classroomID, lessonID))
}

// LessonAppearanceOn reads «Оформление урока» of the marathon.
func (r *Repository) LessonAppearanceOn(ctx context.Context, q database.Querier, classroomID uuid.UUID) (bool, error) {
	var on bool
	err := q.QueryRow(ctx, `SELECT lesson_appearance FROM marathon_settings WHERE classroom_id = $1`, classroomID).Scan(&on)
	return on, err
}

// ensureAppearance creates the row with the defaults when it does not exist yet and locks it.
func ensureAppearance(ctx context.Context, tx pgx.Tx, classroomID uuid.UUID, lessonID *uuid.UUID) (*Appearance, error) {
	def := DefaultAppearance(lessonID != nil)
	if _, err := tx.Exec(ctx, `INSERT INTO marathon_appearances (classroom_id, lesson_id, enabled, reactions)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (classroom_id, (COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid))) DO NOTHING`,
		classroomID, lessonID, def.Enabled, def.Reactions); err != nil {
		return nil, err
	}
	return scanAppearance(tx.QueryRow(ctx, `SELECT `+appearanceCols+` FROM marathon_appearances
		WHERE classroom_id = $1 AND lesson_id IS NOT DISTINCT FROM $2 FOR UPDATE`, classroomID, lessonID))
}

// SaveAppearance stores the appearance; check validates it against the locked current row (the custom background
// needs an uploaded picture). lessonAppearance, when set, is «Оформление урока» of the marathon.
// changed=false when nothing differs from the stored values.
func (r *Repository) SaveAppearance(ctx context.Context, classroomID uuid.UUID, lessonID *uuid.UUID, a Appearance, lessonAppearance *bool,
	actorID uuid.UUID, check func(cur *Appearance) error) (bool, error) {
	changed := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		if lessonID != nil {
			ok, err := r.HasLesson(ctx, tx, classroomID, *lessonID)
			if err != nil {
				return err
			}
			if !ok {
				return pgx.ErrNoRows
			}
		}
		cur, err := ensureAppearance(ctx, tx, classroomID, lessonID)
		if err != nil {
			return err
		}
		if err := check(cur); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE marathon_appearances SET enabled = $3, reactions_enabled = $4, reactions = $5,
				background_enabled = $6, background = $7, updated_by = $8, updated_at = now()
			WHERE classroom_id = $1 AND lesson_id IS NOT DISTINCT FROM $2
			  AND (enabled, reactions_enabled, reactions, background_enabled, background) IS DISTINCT FROM ($3, $4, $5::jsonb, $6, $7)`,
			classroomID, lessonID, a.Enabled, a.ReactionsEnabled, a.Reactions, a.BackgroundEnabled, a.Background, actorID)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected() > 0
		if lessonAppearance != nil {
			tag, err := tx.Exec(ctx, `UPDATE marathon_settings SET lesson_appearance = $2, updated_at = now()
				WHERE classroom_id = $1 AND lesson_appearance <> $2`, classroomID, *lessonAppearance)
			if err != nil {
				return err
			}
			changed = changed || tag.RowsAffected() > 0
		}
		return nil
	})
	return changed, err
}

// SetImage puts a picture into a slot (nil clears it) and returns the previous picture, which the caller removes
// after the commit. found=false: the lesson is not in the marathon. Clearing a custom background that is selected
// falls back to the plain one.
func (r *Repository) SetImage(ctx context.Context, classroomID uuid.UUID, lessonID *uuid.UUID, slot string, image *uuid.UUID) (old *uuid.UUID, found bool, err error) {
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockSettings(ctx, tx, classroomID); err != nil {
			return err
		}
		if lessonID != nil {
			ok, err := r.HasLesson(ctx, tx, classroomID, *lessonID)
			if err != nil || !ok {
				return err
			}
		}
		found = true
		switch {
		case slot == SlotCover && lessonID == nil:
			if err := tx.QueryRow(ctx, `SELECT cover_attachment_id FROM classrooms WHERE id = $1 FOR UPDATE`, classroomID).Scan(&old); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE classrooms SET cover_attachment_id = $2, updated_at = now() WHERE id = $1`, classroomID, image)
			return err
		case slot == SlotCover:
			if err := tx.QueryRow(ctx, `SELECT cover_attachment_id FROM marathon_lessons WHERE classroom_id = $1 AND lesson_id = $2 FOR UPDATE`,
				classroomID, *lessonID).Scan(&old); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE marathon_lessons SET cover_attachment_id = $3 WHERE classroom_id = $1 AND lesson_id = $2`,
				classroomID, *lessonID, image)
			return err
		default:
			cur, err := ensureAppearance(ctx, tx, classroomID, lessonID)
			if err != nil {
				return err
			}
			old = cur.BackgroundImageID
			_, err = tx.Exec(ctx, `UPDATE marathon_appearances SET background_attachment_id = $3,
					background = CASE WHEN $3::uuid IS NULL AND background = 'custom' THEN 'plain' ELSE background END, updated_at = now()
				WHERE classroom_id = $1 AND lesson_id IS NOT DISTINCT FROM $2`, classroomID, lessonID, image)
			return err
		}
	})
	return old, found, err
}
