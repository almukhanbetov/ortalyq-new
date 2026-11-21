package marathons

import (
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/modules/attempts"
	"ortalyq/backend/internal/modules/materials"
)

// SaveRequest carries the basic marathon fields; used both to create and to update.
type SaveRequest struct {
	Name           string  `json:"name"`
	CoverColor     string  `json:"cover_color"`
	Direction      string  `json:"direction"`
	TargetLanguage *string `json:"target_language"`
}

// PassageRequest saves all passage settings at once; null switches a setting off.
// StartAt: delayed start; EndAt: end date; AutoDeactivateDays: individual access period in days.
type PassageRequest struct {
	StartAt            *time.Time `json:"start_at"`
	EndAt              *time.Time `json:"end_at"`
	AutoDeactivateDays *int       `json:"auto_deactivate_days"`
}

// MeLesson is a lesson in the student's own view with its access status (open, not_started, deactivated).
type MeLesson struct {
	Lesson
	Access string `json:"access"` // open, not_started, deactivated, locked
	// Lock: why the lesson is closed by the passage mode (reason, opens_at), for Access = locked.
	Lock *LessonAccess `json:"lock,omitempty"`
}

// MeResponse is GET /marathons/:cid/me: the student's access decision and their lessons. A student deactivated
// by the teacher gets no lessons (only the status), as the server refuses everything else for them.
type MeResponse struct {
	Access           Access     `json:"access"`
	Mode             string     `json:"mode"`
	ServerNow        time.Time  `json:"server_now"`
	Items            []MeLesson `json:"items"`
	CompletedLessons int        `json:"completed_lessons"`
	TotalLessons     int        `json:"total_lessons"`
}

// AddLessonRequest adds a lesson from the teacher's materials (lesson_id) or creates a new one (new).
type AddLessonRequest struct {
	LessonID *uuid.UUID    `json:"lesson_id"`
	New      *LessonFields `json:"new"`
}

// LessonFields are the "Настройки урока" fields. Title and description belong to the lesson template;
// the cover colour belongs to the lesson in this marathon.
type LessonFields struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	CoverColor  *string `json:"cover_color"`
}

type ReorderLessonsRequest struct {
	LessonIDs []uuid.UUID `json:"lesson_ids"`
}

// LessonCopied is the answer to "Копировать": the new lesson id and the updated list.
type LessonCopied struct {
	LessonID uuid.UUID `json:"lesson_id"`
	LessonsResponse
}

type AddStudentsRequest struct {
	StudentIDs []uuid.UUID `json:"student_ids"`
}

// SetActiveRequest activates (active=true) or deactivates (active=false) the listed participants.
type SetActiveRequest struct {
	StudentIDs []uuid.UUID `json:"student_ids"`
	Active     *bool       `json:"active"`
}

type LessonsResponse struct {
	Items            []Lesson `json:"items"`
	CompletedLessons int      `json:"completed_lessons"` // the caller's own count (students)
	TotalLessons     int      `json:"total_lessons"`
}

// StudentProgress is the "Прогресс ученика" window: the student and their status per lesson.
type StudentProgress struct {
	Student Student `json:"student"`
	LessonsResponse
}

// LessonReview is the teacher's view of one lesson as the student left it, with the review state (stage 6).
type LessonReview struct {
	Student     Student                 `json:"student"`
	LessonID    uuid.UUID               `json:"lesson_id"`
	LessonTitle string                  `json:"lesson_title"`
	CourseTitle string                  `json:"course_title"`
	Completed   bool                    `json:"completed"`
	CompletedAt *time.Time              `json:"completed_at"`
	Sections    []materials.Section     `json:"sections"`
	Exercises   []attempts.ExerciseView `json:"exercises"`
	Points      *attempts.Points        `json:"points"`
	// Review: the lesson status of the student and what the teacher decided before.
	Lesson           Lesson `json:"lesson"`
	TeacherCheck     bool   `json:"teacher_check"`
	SubmissionsCount int    `json:"submissions_count"`
	// Unchanged (answer to a decision only): the lesson already had the requested status, so nothing was saved —
	// in particular a second «Завершить урок» keeps the first rating and comment (idempotent). The interface says so.
	Unchanged bool `json:"unchanged,omitempty"`
}

// ReviewSettingsRequest: "Проверка учителем" and "Открытие уроков" (progressive mode only).
type ReviewSettingsRequest struct {
	TeacherCheck *bool  `json:"teacher_check"`
	UnlockAfter  string `json:"unlock_after"`
}

// ReviewRequest is the teacher's decision on a sent lesson:
//   - to_status "revision" — «Отправить на доработку»;
//   - to_status "completed" — «Завершить урок» with a rating (not_bad / nice / amazing) and an optional comment;
//   - to_status "submitted" — «Отменить завершение» of an accepted lesson.
//
// from_status (optional) is the status the teacher saw: a different current status answers 409 status_changed.
type ReviewRequest struct {
	StudentID  uuid.UUID `json:"student_id"`
	ToStatus   string    `json:"to_status"`
	FromStatus string    `json:"from_status"`
	Rating     *string   `json:"rating"`
	Comment    *string   `json:"comment"`
}

// OpenAtRequest sets or removes (null) "Дата открытия" of a lesson.
type OpenAtRequest struct {
	OpenAt *time.Time `json:"open_at"`
}

// UnlockRequest opens a lesson in the closed mode: for everyone without student_id, or for one student.
type UnlockRequest struct {
	StudentID *uuid.UUID `json:"student_id"`
}

// ModeRequest changes the passage mode. ExpectedEpoch is the progress epoch the teacher saw;
// ConfirmReset is the accepted warning that lesson statuses start over.
type ModeRequest struct {
	Mode          string `json:"mode"`
	ExpectedEpoch int    `json:"expected_epoch"`
	ConfirmReset  bool   `json:"confirm_reset"`
	// ConfirmResetDates: the teacher accepted that lesson opening dates are reset (modes without dates).
	ConfirmResetDates bool `json:"confirm_reset_dates"`
	// Schedule: interval_days (1..7) for "interval"; weekdays (1 = Monday … 7 = Sunday) for "weekdays";
	// open_time "HH:00" (whole hour, marathon time zone) for both.
	IntervalDays *int    `json:"interval_days"`
	Weekdays     []int   `json:"weekdays"`
	OpenTime     *string `json:"open_time"`
}

// StatsSettingsRequest: "Дополнительно → Статистика учеников". Without the statistics the rating is switched off;
// rating_limit is 3, 5, 10 or null (all students).
type StatsSettingsRequest struct {
	StatsEnabled    *bool `json:"stats_enabled"`
	RatingEnabled   bool  `json:"rating_enabled"`
	RatingLimit     *int  `json:"rating_limit"`
	RatingShowPlace bool  `json:"rating_show_place"`
}

// CommentRequest sets «Комментарий к статистике» of a lesson; an empty body removes it.
type CommentRequest struct {
	Body string `json:"body"`
}

// Summary is a student's result over several lessons, recomputed from answers (attempts.PointsFor).
type Summary struct {
	Earned            int                       `json:"earned"`
	Max               int                       `json:"max"`
	Exercises         int                       `json:"exercises"`
	FinishedExercises int                       `json:"finished_exercises"`
	Categories        []attempts.CategoryPoints `json:"categories"`
}

// StatsCategory is a column of the statistics: a score category met in the marathon lessons.
type StatsCategory struct {
	CategoryID   *uuid.UUID `json:"category_id"`
	CategoryName string     `json:"category_name"`
}

// StatsRow is one participant in the "Статистика" tab and the export.
type StatsRow struct {
	StudentID        uuid.UUID `json:"student_id"`
	FullName         string    `json:"full_name"`
	Email            string    `json:"email"`
	AvatarColor      string    `json:"avatar_color"`
	Status           string    `json:"status"`
	InactiveReason   *string   `json:"inactive_reason"`
	CompletedLessons int       `json:"completed_lessons"`
	TotalLessons     int       `json:"total_lessons"`
	// Completed: lesson results only — the student completed this lesson in the current epoch.
	Completed bool    `json:"completed"`
	Points    Summary `json:"points"`
	// Place in the rating; 0 for a student who is not rated (deactivated by the teacher or by the individual period).
	Place int `json:"place"`
}

// StatsResponse is GET /marathons/:cid/stats (owner).
type StatsResponse struct {
	Items        []StatsRow      `json:"items"`
	Categories   []StatsCategory `json:"categories"`
	TotalLessons int             `json:"total_lessons"`
}

// RatingResponse: the places a viewer may see and, for a student outside them, their own place when allowed.
type RatingResponse struct {
	Items     []RatingEntry `json:"items"`
	Me        *RatingEntry  `json:"me"`
	Limit     *int          `json:"limit"`
	ShowPlace bool          `json:"show_place"`
	Total     int           `json:"total"`
}

// LessonResults is «Результаты урока». Student: own points, the comment and (with the rating on) the lesson
// rating. Teacher: the comment, every participant's points for this lesson and the full lesson rating.
type LessonResults struct {
	LessonID uuid.UUID        `json:"lesson_id"`
	Title    string           `json:"title"`
	Points   *attempts.Points `json:"points,omitempty"`
	Comment  *LessonComment   `json:"comment"`
	Rating   *RatingResponse  `json:"rating"`
	Students []StatsRow       `json:"students"` // teacher only (null for a student)
}

// MyResults is «Успехи ученика в марафоне»: progress always, points and place only with the statistics on.
type MyResults struct {
	CompletedLessons  int             `json:"completed_lessons"`
	TotalLessons      int             `json:"total_lessons"`
	FinishedExercises int             `json:"finished_exercises"`
	Exercises         int             `json:"exercises"`
	StatsEnabled      bool            `json:"stats_enabled"`
	Points            *Summary        `json:"points"`
	Rating            *RatingResponse `json:"rating"`
}

// InviteRequest is «Добавить ученика → Вручную»: e-mails (up to 20) and «Язык писем».
type InviteRequest struct {
	Emails []string `json:"emails"`
	Locale string   `json:"locale"`
}

// InviteResult is the outcome for one address: invited, renewed (an expired invitation got a new link),
// already_invited (an open invitation exists; no second letter), already_participant, unavailable (no student
// with this address is assigned to the teacher — Н6, see the plan) or invalid (not an e-mail).
type InviteResult struct {
	Email        string     `json:"email"`
	Result       string     `json:"result"`
	InvitationID *uuid.UUID `json:"invitation_id,omitempty"`
}

// InvitationInfo is the public page of an invitation link: what it is about, without personal data beyond a
// masked address.
type InvitationInfo struct {
	MarathonName string    `json:"marathon_name"`
	TeacherName  string    `json:"teacher_name"`
	EmailMasked  string    `json:"email_masked"`
	Status       string    `json:"status"` // pending, expired, accepted, revoked
	ExpiresAt    time.Time `json:"expires_at"`
}

// Accepted is the answer to accepting an invitation.
type Accepted struct {
	MarathonID uuid.UUID `json:"marathon_id"`
}

// ---- Appearance (stage 10) ----

// MarathonAppearance is the «Оформление» tab of the marathon settings.
type MarathonAppearance struct {
	LessonAppearance bool       `json:"lesson_appearance"`
	Appearance       Appearance `json:"appearance"`
}

// AppearanceRequest saves the marathon appearance (LessonAppearance) or the own appearance of a lesson (Enabled).
type AppearanceRequest struct {
	LessonAppearance  *bool     `json:"lesson_appearance"`
	Enabled           *bool     `json:"enabled"`
	ReactionsEnabled  bool      `json:"reactions_enabled"`
	Reactions         Reactions `json:"reactions"`
	BackgroundEnabled bool      `json:"background_enabled"`
	Background        string    `json:"background"`
}

// ImageResult answers an upload or a removal of a cover or background picture.
type ImageResult struct {
	ImageID *uuid.UUID `json:"image_id"`
}
