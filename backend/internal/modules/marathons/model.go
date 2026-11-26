// Package marathons manages marathons: self-paced classes where students take lessons on their own.
// A marathon is a classroom of type "marathon" plus a marathon_settings row.
package marathons

import (
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/languages"
)

const (
	DirectionLanguages = "languages"
	DirectionOther     = "other"

	ModeOpen = "open"
)

// Languages a marathon can teach: the project list (also used by materials). Display names live in the frontend locales.
var Languages = languages.Codes

func knownLanguage(code string) bool { return languages.Known(code) }

// Student statuses in a marathon. "inactive" is computed, never stored: after end_at every student is inactive.
const (
	StudentActive   = "active"
	StudentInactive = "inactive"

	ReasonEndDate = "end_date"
	// ReasonManual: the teacher deactivated the student. It wins over the end date: both close access.
	ReasonManual = "manual"
)

// Lesson is a lesson of the marathon in its order. CompletedCount is shown to the teacher,
// Completed to the student viewing their own list.
type Lesson struct {
	LessonID    uuid.UUID `json:"lesson_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CourseTitle string    `json:"course_title"`
	CoverColor  string    `json:"cover_color"`
	// CoverImageID: the lesson cover picture in this marathon (stage 10); null = generated cover.
	CoverImageID *uuid.UUID `json:"cover_image_id"`
	Position     int        `json:"position"`
	// OpenAt: "Дата открытия" of the lesson (open mode).
	OpenAt *time.Time `json:"open_at"`
	// UnlockedForAll: closed mode, the teacher opened the lesson for everyone (teacher view).
	UnlockedForAll bool `json:"unlocked_for_all"`
	// StudentLesson: the lesson rule of the mode for the student in "Прогресс ученика" (teacher view).
	StudentLesson  *LessonAccess `json:"student_lesson,omitempty"`
	SectionCount   int           `json:"section_count"`
	CompletedCount int           `json:"completed_count"`
	// PendingCount: teacher view, how many participants sent the lesson and wait for the review.
	PendingCount int        `json:"pending_count"`
	Completed    bool       `json:"completed"`
	CompletedAt  *time.Time `json:"completed_at"`
	// Review (stage 6) of the student this list is built for, in the current epoch:
	// Status none / submitted / revision / completed; Accepted — completed by the teacher's review.
	// Rating and TeacherComment are shown only while the lesson is accepted; PreviousRating/PreviousComment
	// (teacher only) keep the last review after «Отменить завершение» to prefill the rating window.
	Status          string     `json:"status"`
	SubmittedAt     *time.Time `json:"submitted_at"`
	Accepted        bool       `json:"accepted"`
	Rating          *string    `json:"rating"`
	TeacherComment  *string    `json:"teacher_comment"`
	PreviousRating  *string    `json:"previous_rating,omitempty"`
	PreviousComment *string    `json:"previous_comment,omitempty"`
}

// Lesson review statuses (stage 6). "none" is not stored: there is no progress row in the epoch.
const (
	LessonNone      = "none"
	LessonSubmitted = "submitted"
	LessonRevision  = "revision"
	LessonCompleted = "completed"
)

// Ratings of an accepted lesson («Поставьте оценку»).
var Ratings = []string{"not_bad", "nice", "amazing"}

func knownRating(r string) bool {
	for _, x := range Ratings {
		if x == r {
			return true
		}
	}
	return false
}

// Student is a marathon participant with their progress.
type Student struct {
	ID             uuid.UUID `json:"id"`
	FullName       string    `json:"full_name"`
	Email          string    `json:"email"`
	AvatarColor    string    `json:"avatar_color"`
	ActivatedAt    time.Time `json:"activated_at"`
	Status         string    `json:"status"`
	InactiveReason *string   `json:"inactive_reason"`
	// When and why the student loses access (today only the marathon end date); null without an end date.
	DeactivatesAt      *time.Time `json:"deactivates_at"`
	DeactivationReason *string    `json:"deactivation_reason"`
	CompletedLessons   int        `json:"completed_lessons"`
	TotalLessons       int        `json:"total_lessons"`
	// PendingLessons: lessons the student sent and the teacher has not reviewed yet (current epoch).
	PendingLessons int `json:"pending_lessons"`
	// Manual state, set by the teacher: when and by whom the student was deactivated (null while active),
	// and the last manual activation.
	// AccessStatus: open, not_started or deactivated — the same decision the server applies to this student.
	AccessStatus      string     `json:"access_status"`
	DeactivatedAt     *time.Time `json:"deactivated_at"`
	DeactivatedByName *string    `json:"deactivated_by_name"`
	ReactivatedAt     *time.Time `json:"reactivated_at"`
}

// state is the part of the settings that decides access, read with now() of the DB.
type state struct {
	EndAt    *time.Time
	Finished bool
	Now      time.Time
	StartAt  *time.Time
	AutoDays *int
	Timezone string
	Mode     string
	Epoch    int
	// Schedule of "interval" / "weekdays" (null in other modes).
	IntervalDays *int
	Weekdays     *int
	OpenHour     *int
	// Teacher check (stage 6) and "Открытие уроков" of the progressive mode.
	TeacherCheck bool
	UnlockAfter  string
	// Statistics and rating (stage 7).
	StatsEnabled    bool
	RatingEnabled   bool
	RatingLimit     *int
	RatingShowPlace bool
}

// rules builds the lesson rules of one student: the anchor of the schedule is max(start_at, activated_at) (Н1 = A).
func (st *state) rules(activatedAt time.Time) LessonRules {
	r := LessonRules{Mode: st.Mode, Anchor: activatedAt}
	if st.StartAt != nil && st.StartAt.After(activatedAt) {
		r.Anchor = *st.StartAt
	}
	if st.IntervalDays != nil {
		r.IntervalDays = *st.IntervalDays
	}
	if st.Weekdays != nil {
		r.Weekdays = *st.Weekdays
	}
	if st.OpenHour != nil {
		r.OpenHour = *st.OpenHour
	}
	if loc, err := time.LoadLocation(st.Timezone); err == nil {
		r.Location = loc
	}
	if st.TeacherCheck {
		r.Unlock = st.UnlockAfter
	}
	return r
}

func (st *state) access() AccessSettings {
	return AccessSettings{StartAt: st.StartAt, EndAt: st.EndAt, AutoDeactivateDays: st.AutoDays, Timezone: st.Timezone}
}

// Invitation statuses (computed): pending — waiting for the student; expired — the link is out of date (can be
// sent again); accepted; revoked by the teacher; participant — the student joined another way meanwhile.
const (
	InvitePending     = "pending"
	InviteExpired     = "expired"
	InviteAccepted    = "accepted"
	InviteRevoked     = "revoked"
	InviteParticipant = "participant"
)

// Invitation is a row of the teacher's invitation list. Delivery is the state of the last letter in the outbox.
type Invitation struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	StudentID     uuid.UUID  `json:"student_id"`
	StudentName   string     `json:"student_name"`
	AvatarColor   string     `json:"avatar_color"`
	Locale        string     `json:"locale"`
	Status        string     `json:"status"`
	Delivery      string     `json:"delivery"` // queued, sent, failed
	DeliveryError *string    `json:"-"`        // kept in the outbox for the operator; not shown in the UI
	Sends         int        `json:"sends"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        time.Time  `json:"sent_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	AcceptedAt    *time.Time `json:"accepted_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
}
