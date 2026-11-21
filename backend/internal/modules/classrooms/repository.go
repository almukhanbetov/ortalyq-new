package classrooms

import (
	"context"
	"strings"
	"time"

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

const classCols = `c.id, c.name, c.lesson_type, c.owner_id, c.cover_color, c.cover_attachment_id, m.role, c.created_at,
	l.id, l.title, co.id, co.title,
	ms.direction, ms.target_language, ms.mode, ms.timezone, ms.end_at, (ms.end_at IS NOT NULL AND ms.end_at <= now()), now(),
	ms.start_at, (ms.start_at IS NULL OR ms.start_at <= now()), ms.auto_deactivate_days, ms.progress_epoch,
	ms.interval_days, ms.weekdays, to_char(ms.open_time, 'HH24:MI'), ms.teacher_check, ms.unlock_after,
	ms.stats_enabled, ms.rating_enabled, ms.rating_limit, ms.rating_show_place,
	(SELECT count(*) FROM classroom_lessons cl WHERE cl.classroom_id = c.id),
	(SELECT count(*) FROM classroom_members sm WHERE sm.classroom_id = c.id AND sm.role = 'student')`

func scanClass(row pgx.Row, c *Classroom) error {
	var lid, cid *uuid.UUID
	var lt, ct, dir, mode, tz *string
	var lang *string
	var endAt, startAt *time.Time
	var finished, started *bool
	var autoDays, epoch, intervalDays, weekdays *int
	var openTime, unlockAfter *string
	var teacherCheck, statsOn, ratingOn, showPlace *bool
	var ratingLimit *int
	var now time.Time
	if err := row.Scan(&c.ID, &c.Name, &c.LessonType, &c.OwnerID, &c.CoverColor, &c.CoverImageID, &c.MyRole, &c.CreatedAt, &lid, &lt, &cid, &ct,
		&dir, &lang, &mode, &tz, &endAt, &finished, &now, &startAt, &started, &autoDays, &epoch, &intervalDays, &weekdays, &openTime, &teacherCheck, &unlockAfter, &statsOn, &ratingOn, &ratingLimit, &showPlace, &c.LessonsCount, &c.StudentsCount); err != nil {
		return err
	}
	if lid != nil {
		c.CurrentLesson = &CurrentLesson{LessonID: *lid, LessonTitle: *lt, CourseID: *cid, CourseTitle: *ct}
	}
	if dir != nil && mode != nil {
		c.Marathon = &MarathonInfo{Direction: *dir, TargetLanguage: lang, Mode: *mode, Timezone: *tz, EndAt: endAt,
			Finished: finished != nil && *finished, ServerNow: now,
			StartAt: startAt, Started: started == nil || *started, AutoDeactivateDays: autoDays}
		if epoch != nil {
			c.Marathon.ProgressEpoch = *epoch
		}
		c.Marathon.IntervalDays, c.Marathon.OpenTime = intervalDays, openTime
		c.Marathon.TeacherCheck = teacherCheck != nil && *teacherCheck
		if unlockAfter != nil {
			c.Marathon.UnlockAfter = *unlockAfter
		}
		c.Marathon.StatsEnabled = statsOn != nil && *statsOn
		c.Marathon.RatingEnabled = ratingOn != nil && *ratingOn
		c.Marathon.RatingShowPlace = showPlace != nil && *showPlace
		c.Marathon.RatingLimit = ratingLimit
		c.Marathon.Weekdays = []int{}
		if weekdays != nil {
			for d := 1; d <= 7; d++ {
				if *weekdays&(1<<(d-1)) != 0 {
					c.Marathon.Weekdays = append(c.Marathon.Weekdays, d)
				}
			}
		}
	}
	return nil
}

// Archived classes (a deleted marathon) are invisible to everyone.
const classFrom = ` FROM classrooms c
	JOIN classroom_members m ON m.classroom_id = c.id AND m.user_id = $1 AND c.archived_at IS NULL
	LEFT JOIN marathon_settings ms ON ms.classroom_id = c.id
	LEFT JOIN lessons l ON l.id = c.current_lesson_id
	LEFT JOIN courses co ON co.id = l.course_id`

// sortColumns whitelists ORDER BY expressions (never interpolate user input).
var sortColumns = map[string]string{"created": "c.created_at", "name": "lower(c.name)"}

func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID, f ListFilter) ([]Classroom, int, error) {
	where := ` WHERE ($2 = '' OR c.name ILIKE '%' || $2 || '%' ESCAPE '\') AND ($3 = '' OR c.lesson_type = $3)`
	q := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.Query)
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+classFrom+where, userID, q, f.Type).Scan(&total); err != nil {
		return nil, 0, err
	}
	col, ok := sortColumns[f.Sort]
	if !ok {
		col = sortColumns["created"]
	}
	dir := "ASC"
	if f.Desc {
		dir = "DESC"
	}
	rows, err := r.db.Query(ctx, `SELECT `+classCols+classFrom+where+` ORDER BY `+col+` `+dir+`, c.id `+dir+` LIMIT $4 OFFSET $5`,
		userID, q, f.Type, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Classroom{}
	for rows.Next() {
		var c Classroom
		if err := scanClass(rows, &c); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetForUser(ctx context.Context, id, userID uuid.UUID) (*Classroom, error) {
	var c Classroom
	if err := scanClass(r.db.QueryRow(ctx, `SELECT `+classCols+classFrom+` WHERE c.id = $2`, userID, id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) Members(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]Member, error) {
	rows, err := r.db.Query(ctx, `SELECT m.classroom_id, u.id, u.full_name, m.role, u.avatar_color
		FROM classroom_members m JOIN users u ON u.id = m.user_id
		WHERE m.classroom_id = ANY($1) ORDER BY m.role DESC, u.full_name`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]Member{}
	for rows.Next() {
		var cid uuid.UUID
		var m Member
		if err := rows.Scan(&cid, &m.UserID, &m.FullName, &m.Role, &m.AvatarColor); err != nil {
			return nil, err
		}
		out[cid] = append(out[cid], m)
	}
	return out, rows.Err()
}

// HomeworkBadges: teacher -> submissions awaiting review; student -> homework still to do.
// Marathons: teacher -> lessons sent for the teacher check and not reviewed (current epoch, current participants
// and lessons); student -> 0 (the marathon page shows the student's own statuses).
func (r *Repository) HomeworkBadges(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.db.Query(ctx, `SELECT m.classroom_id,
		CASE WHEN ms.classroom_id IS NOT NULL AND m.role = 'teacher' THEN
			(SELECT count(*) FROM marathon_lesson_progress p
			   JOIN marathon_lessons ml ON ml.classroom_id = p.classroom_id AND ml.lesson_id = p.lesson_id AND ml.removed_at IS NULL
			   JOIN marathon_enrollments e ON e.classroom_id = p.classroom_id AND e.student_id = p.student_id AND e.removed_at IS NULL
			  WHERE p.classroom_id = m.classroom_id AND p.epoch = ms.progress_epoch AND p.status = 'submitted')
		WHEN ms.classroom_id IS NOT NULL THEN 0
		WHEN m.role = 'teacher' THEN
			(SELECT count(*) FROM homeworks h WHERE h.classroom_id = m.classroom_id AND h.status = 'pending')
		ELSE
			(SELECT count(*) FROM classroom_lessons cl
			 WHERE cl.classroom_id = m.classroom_id
			   AND EXISTS (SELECT 1 FROM exercises e WHERE e.lesson_id = cl.lesson_id AND e.scope = 'homework' AND e.deleted_at IS NULL AND NOT e.hidden)
			   AND NOT EXISTS (SELECT 1 FROM homeworks h WHERE h.classroom_id = cl.classroom_id AND h.lesson_id = cl.lesson_id
			                   AND h.student_id = m.user_id AND h.status IN ('pending', 'done')))
		END
		FROM classroom_members m LEFT JOIN marathon_settings ms ON ms.classroom_id = m.classroom_id
		WHERE m.user_id = $1 AND m.classroom_id = ANY($2)`, userID, ids)
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

func (r *Repository) MemberRole(ctx context.Context, classroomID, userID uuid.UUID) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT m.role FROM classroom_members m JOIN classrooms c ON c.id = m.classroom_id
		WHERE m.classroom_id = $1 AND m.user_id = $2 AND c.archived_at IS NULL`, classroomID, userID).Scan(&role)
	return role, err
}

func (r *Repository) StudentIDs(ctx context.Context, classroomID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id FROM classroom_members WHERE classroom_id = $1 AND role = 'student' ORDER BY joined_at`, classroomID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *Repository) LessonOpened(ctx context.Context, classroomID, lessonID uuid.UUID) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM classroom_lessons WHERE classroom_id = $1 AND lesson_id = $2)`, classroomID, lessonID).Scan(&ok)
	return ok, err
}

func (r *Repository) Create(ctx context.Context, teacherID, studentID uuid.UUID, name, lessonType, color string) (uuid.UUID, error) {
	var id uuid.UUID
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO classrooms (name, lesson_type, owner_id, cover_color) VALUES ($1, $2, $3, $4) RETURNING id`,
			name, lessonType, teacherID, color).Scan(&id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO classroom_members (classroom_id, user_id, role) VALUES ($1, $2, 'teacher'), ($1, $3, 'student')`, id, teacherID, studentID)
		return err
	})
	return id, err
}

func (r *Repository) Rename(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.db.Exec(ctx, `UPDATE classrooms SET name = $2, updated_at = now() WHERE id = $1`, id, name)
	return err
}

// SetCurrentLesson opens a lesson in the classroom (idempotent) and grants its course.
func (r *Repository) SetCurrentLesson(ctx context.Context, classroomID, lessonID, courseID uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO classroom_courses (classroom_id, course_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, classroomID, courseID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO classroom_lessons (classroom_id, lesson_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, classroomID, lessonID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE classrooms SET current_lesson_id = $2, updated_at = now() WHERE id = $1`, classroomID, lessonID)
		return err
	})
}

func (r *Repository) ClassroomsUsingLesson(ctx context.Context, lessonID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT classroom_id FROM classroom_lessons WHERE lesson_id = $1`, lessonID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}
