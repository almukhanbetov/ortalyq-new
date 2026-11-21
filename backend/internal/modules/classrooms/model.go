// Package classrooms manages classes, their members and which lesson is open.
// It also exposes access checks used by other modules.
package classrooms

import (
	"time"

	"github.com/google/uuid"
)

const (
	MemberTeacher = "teacher"
	MemberStudent = "student"
)

type Member struct {
	UserID      uuid.UUID `json:"user_id"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	AvatarColor string    `json:"avatar_color"`
}

type CurrentLesson struct {
	LessonID    uuid.UUID `json:"lesson_id"`
	LessonTitle string    `json:"lesson_title"`
	CourseID    uuid.UUID `json:"course_id"`
	CourseTitle string    `json:"course_title"`
}

type Classroom struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	LessonType string    `json:"lesson_type"`
	OwnerID    uuid.UUID `json:"owner_id"`
	CoverColor string    `json:"cover_color"`
	// CoverImageID: the cover picture of a marathon (stage 10), an attachment of this class; null = generated cover.
	CoverImageID  *uuid.UUID     `json:"cover_image_id"`
	MyRole        string         `json:"my_role"`
	CurrentLesson *CurrentLesson `json:"current_lesson"`
	Members       []Member       `json:"members"`
	HomeworkBadge int            `json:"homework_badge"`
	LessonsCount  int            `json:"lessons_count"`
	StudentsCount int            `json:"students_count"`
	Marathon      *MarathonInfo  `json:"marathon"` // set only for lesson_type = marathon
	CreatedAt     time.Time      `json:"created_at"`
}

const TypeMarathon = "marathon"

// MarathonInfo is the part of marathon settings shown on cards and the marathon header.
type MarathonInfo struct {
	Direction      string  `json:"direction"`
	TargetLanguage *string `json:"target_language"`
	Mode           string  `json:"mode"`
	Timezone       string  `json:"timezone"`
	// EndAt closes the marathon for students; Finished is computed with now() of the DB.
	EndAt     *time.Time `json:"end_at"`
	Finished  bool       `json:"finished"`
	ServerNow time.Time  `json:"server_now"`
	// StartAt: delayed start (lessons closed for students before it); Started is computed with now() of the DB.
	StartAt *time.Time `json:"start_at"`
	Started bool       `json:"started"`
	// AutoDeactivateDays: each student loses access N days after their activation date (null = off).
	AutoDeactivateDays *int `json:"auto_deactivate_days"`
	// ProgressEpoch: lesson statuses count in this epoch; a mode change starts a new one (sent back as expected_epoch).
	ProgressEpoch int `json:"progress_epoch"`
	// Schedule of "По интервалам" / "По дням недели": interval in days, weekdays (1 = Monday … 7 = Sunday)
	// and the opening hour "HH:00" in the marathon time zone; empty for the other modes.
	IntervalDays *int    `json:"interval_days"`
	Weekdays     []int   `json:"weekdays"`
	OpenTime     *string `json:"open_time"`
	// TeacherCheck: lessons are sent to the teacher for review (stage 6). UnlockAfter: "Открытие уроков" of the
	// progressive mode with the check on — teacher_check (after acceptance) or submission (after sending).
	TeacherCheck bool   `json:"teacher_check"`
	UnlockAfter  string `json:"unlock_after"`
	// Statistics and rating (stage 7): "Статистика", "Рейтинг учеников", "Отображать" (3 / 5 / 10, null = all),
	// "Место ученика" (show a student their own place outside the shown ones).
	StatsEnabled    bool `json:"stats_enabled"`
	RatingEnabled   bool `json:"rating_enabled"`
	RatingLimit     *int `json:"rating_limit"`
	RatingShowPlace bool `json:"rating_show_place"`
}
