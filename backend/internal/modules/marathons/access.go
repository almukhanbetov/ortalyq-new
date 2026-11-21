package marathons

import (
	"time"

	"ortalyq/backend/internal/apperr"
)

// Access statuses of a student in a marathon (plan, section 3.2). Lesson-level rules of the other modes
// (interval, weekdays, progressive, closed, lesson opening dates) belong to stage 5 and are not here yet:
// in the "open" mode every lesson of the marathon follows the student's access.
const (
	AccessOpen        = "open"
	AccessNotStarted  = "not_started"
	AccessDeactivated = "deactivated"
	// AccessLocked: the student may work, but this lesson is closed by the passage mode (lesson level only).
	AccessLocked = "locked"

	ReasonAutoDeactivation = "auto_deactivation"
)

// AccessSettings is the part of marathon_settings that decides access.
type AccessSettings struct {
	StartAt            *time.Time
	EndAt              *time.Time
	AutoDeactivateDays *int
	Timezone           string
}

// AccessEnrollment is the part of a student's enrollment that decides access.
type AccessEnrollment struct {
	ActivatedAt   time.Time
	DeactivatedAt *time.Time // set by the teacher (manual deactivation)
}

// Access is the decision for one student at one moment.
type Access struct {
	Status string `json:"status"`
	// Reason of a deactivation: manual, end_date or auto_deactivation.
	Reason *string `json:"reason"`
	// OpensAt: when lessons open (not_started).
	OpensAt *time.Time `json:"opens_at"`
	// DeactivatesAt: when access ends or ended (with DeactivationReason); for a manual deactivation, its moment.
	DeactivatesAt      *time.Time `json:"deactivates_at"`
	DeactivationReason *string    `json:"deactivation_reason"`
}

// Deadline is the student's own end of access: min(end_at, activated_at + N days), with its reason.
// N days are counted in the marathon time zone, so a daylight-saving change keeps the wall-clock time.
// Returns nil when neither applies.
func Deadline(s AccessSettings, e AccessEnrollment) (*time.Time, string) {
	var at *time.Time
	reason := ""
	if s.EndAt != nil {
		t := *s.EndAt
		at, reason = &t, ReasonEndDate
	}
	if s.AutoDeactivateDays != nil {
		loc, err := time.LoadLocation(s.Timezone)
		if err != nil {
			loc = time.UTC
		}
		t := e.ActivatedAt.In(loc).AddDate(0, 0, *s.AutoDeactivateDays)
		if at == nil || t.Before(*at) {
			at, reason = &t, ReasonAutoDeactivation
		}
	}
	return at, reason
}

// StudentAccessAt decides access by the order of the plan (section 3.2):
//  1. deactivated by the teacher → deactivated (manual);
//  2. now ≥ min(end_at, activated_at + N days) → deactivated (end_date / auto_deactivation);
//  3. start_at in the future → not_started, opens at start_at;
//  4. otherwise open (mode "open").
//
// now is the time of the database (now()), never the client clock. Boundaries: access ends exactly at the
// deadline (now == deadline is closed) and opens exactly at start_at (now == start_at is open).
func StudentAccessAt(s AccessSettings, e AccessEnrollment, now time.Time) Access {
	deadline, reason := Deadline(s, e)
	if e.DeactivatedAt != nil {
		r := ReasonManual
		at := *e.DeactivatedAt
		return Access{Status: AccessDeactivated, Reason: &r, DeactivatesAt: &at, DeactivationReason: &r}
	}
	a := Access{Status: AccessOpen}
	if deadline != nil {
		r := reason
		a.DeactivatesAt, a.DeactivationReason = deadline, &r
		if !now.Before(*deadline) {
			a.Status, a.Reason = AccessDeactivated, &r
			return a
		}
	}
	if s.StartAt != nil && now.Before(*s.StartAt) {
		at := *s.StartAt
		a.Status, a.OpensAt = AccessNotStarted, &at
	}
	return a
}

// Err turns a refusal into the API error; nil when access is open.
func (a Access) Err() error {
	switch a.Status {
	case AccessNotStarted:
		return ErrNotStarted(*a.OpensAt)
	case AccessDeactivated:
		switch *a.Reason {
		case ReasonManual:
			return ErrDeactivated()
		case ReasonAutoDeactivation:
			return ErrAutoDeactivated()
		default:
			return ErrClosed()
		}
	}
	return nil
}

// ErrNotStarted is returned before start_at; fields.opens_at carries the moment (RFC 3339).
func ErrNotStarted(opensAt time.Time) *apperr.Error {
	e := apperr.New(403, "marathon_not_started", "Марафон ещё не начался")
	e.Fields = map[string]string{"opens_at": opensAt.UTC().Format(time.RFC3339Nano)}
	return e
}

// ErrAutoDeactivated is returned once the student's individual access period is over.
func ErrAutoDeactivated() *apperr.Error {
	return apperr.New(403, "marathon_auto_deactivated", "Срок вашего доступа к марафону истёк")
}

// Passage modes (all five of the original are accepted since Н1 and Н3 were decided).
const (
	ModeProgressive = "progressive"
	ModeClosed      = "closed"
	ModeInterval    = "interval"
	ModeWeekdays    = "weekdays"
)

// AvailableModes are the modes a teacher can choose.
var AvailableModes = []string{ModeOpen, ModeInterval, ModeProgressive, ModeClosed, ModeWeekdays}

func modeAvailable(m string) bool {
	for _, x := range AvailableModes {
		if x == m {
			return true
		}
	}
	return false
}

// modeUsesOpenDates: "Дата открытия" of a lesson applies in these modes (Н3 = A). In the others the dates are
// reset when switching, only after an explicit confirmation.
func modeUsesOpenDates(m string) bool { return m == ModeOpen || m == ModeProgressive }

// Lesson lock reasons (lesson-level, after the student-level access of StudentAccessAt).
const (
	LockOpensAt        = "opens_at"               // open/progressive: the lesson opening date is in the future
	LockPreviousLesson = "previous_not_completed" // progressive: the previous lesson is not completed in this epoch
	// progressive with the teacher check (stage 6): the previous lesson is not accepted by the teacher yet
	// ("После проверки учителем") or not sent for review yet ("После отправки на проверку").
	LockPreviousReview     = "previous_not_reviewed"
	LockPreviousSubmission = "previous_not_submitted"
	LockNotUnlocked        = "not_unlocked"     // closed: the teacher has not opened the lesson
	LockScheduled          = "scheduled"        // interval/weekdays: the lesson's moment in the schedule has not come
	LockUnsupportedMode    = "mode_unavailable" // unknown mode (should not happen)
)

// LessonState is one active lesson of the marathon, in order, as seen for one student in the current epoch.
type LessonState struct {
	OpenAt    *time.Time
	Completed bool // completed in the current epoch (by the student, or accepted by the teacher)
	// Submitted: sent for the teacher check in the current epoch and not completed (on review or returned for
	// revision). A lesson returned for revision has been sent once, so "После отправки на проверку" keeps the
	// next lesson open.
	Submitted bool
	Unlocked  bool // closed mode: opened for everyone or for this student in the current epoch
}

// LessonAccess is the lesson-level decision.
type LessonAccess struct {
	Open    bool       `json:"open"`
	Reason  string     `json:"reason,omitempty"`
	OpensAt *time.Time `json:"opens_at,omitempty"`
}

// Weekday bits of the "weekdays" mask: Monday = 1, Tuesday = 2, … Sunday = 64.
func weekdayBit(d time.Weekday) int {
	return 1 << ((int(d) + 6) % 7) // time.Monday (1) → bit 0, … time.Sunday (0) → bit 6
}

// LessonRules is what decides lessons for one student: the mode and, for the schedules, the anchor and parameters.
type LessonRules struct {
	Mode string
	// Anchor = max(start_at, activated_at) of the student (Н1 = A): lesson 0 opens at this moment.
	Anchor       time.Time
	IntervalDays int // interval: "Раз в N дней"
	Weekdays     int // weekdays: bit mask, Monday = 1 … Sunday = 64
	OpenHour     int // "в Время": whole hour in the marathon time zone
	Location     *time.Location
	// Unlock: what opens the next lesson in the progressive mode. "" — the previous lesson is completed (no teacher
	// check); UnlockTeacherCheck — accepted by the teacher; UnlockSubmission — sent for review (or completed).
	Unlock string
}

// "Открытие уроков" of the progressive mode while the teacher check is on.
const (
	UnlockTeacherCheck = "teacher_check"
	UnlockSubmission   = "submission"
)

// ScheduleOpenAt is the moment lesson k opens in the "interval" and "weekdays" modes (Н1 = A), in the marathon
// time zone:
//   - k = 0: the anchor itself (max(start_at, activated_at));
//   - interval: lesson k ≥ 1 opens on the calendar day (anchor's local date + k·N days) at OpenHour:00;
//   - weekdays: lesson k ≥ 1 opens at the k-th moment "selected weekday at OpenHour:00" strictly after the anchor.
//     The first suitable moment is found by walking local calendar days from the anchor's date: the anchor's own
//     day counts when it is a selected weekday and OpenHour:00 of that day is later than the anchor.
//
// Wall times are built with time.Date in the zone, so a daylight-saving change keeps the hour (a nonexistent hour
// is normalised forward by Go). ok=false for a mode without a schedule or an empty weekday mask.
func ScheduleOpenAt(r LessonRules, k int) (time.Time, bool) {
	if k == 0 {
		return r.Anchor, r.Mode == ModeInterval || r.Mode == ModeWeekdays
	}
	loc := r.Location
	if loc == nil {
		loc = time.UTC
	}
	a := r.Anchor.In(loc)
	switch r.Mode {
	case ModeInterval:
		if r.IntervalDays < 1 {
			return time.Time{}, false
		}
		return time.Date(a.Year(), a.Month(), a.Day()+k*r.IntervalDays, r.OpenHour, 0, 0, 0, loc), true
	case ModeWeekdays:
		if r.Weekdays&127 == 0 {
			return time.Time{}, false
		}
		found := 0
		for day := 0; day <= 7*(k+1); day++ {
			c := time.Date(a.Year(), a.Month(), a.Day()+day, r.OpenHour, 0, 0, 0, loc)
			if r.Weekdays&weekdayBit(c.Weekday()) != 0 && c.After(r.Anchor) {
				found++
				if found == k {
					return c, true
				}
			}
		}
	}
	return time.Time{}, false
}

// LessonAccessAt decides whether lesson idx is open (plan, section 3.2, step 4):
//   - open: open unless its opening date is in the future (now == open_at is open);
//   - progressive: lesson 0, or lesson k when lesson k−1 is completed in the current epoch ("completed" = the
//     student's «Завершить урок», Н2; with the teacher check — accepted by the teacher, or only sent for review
//     when unlock_after = submission), AND its opening date has come (Н3 = A: both conditions). The rule follows
//     the current status: a cancelled completion closes the next lesson again (answers are kept);
//   - closed: open when the teacher opened it for everyone or for this student in the current epoch;
//   - interval / weekdays: open when now ≥ ScheduleOpenAt(k) (lesson 0 at the anchor). Opening dates do not apply.
//
// It is applied only after StudentAccessAt allowed the student: a lesson rule never reopens a marathon
// closed by dates or by a deactivation.
func LessonAccessAt(r LessonRules, lessons []LessonState, idx int, now time.Time) LessonAccess {
	if idx < 0 || idx >= len(lessons) {
		return LessonAccess{}
	}
	l := lessons[idx]
	datePending := l.OpenAt != nil && now.Before(*l.OpenAt)
	switch r.Mode {
	case ModeOpen:
		if datePending {
			at := *l.OpenAt
			return LessonAccess{Reason: LockOpensAt, OpensAt: &at}
		}
		return LessonAccess{Open: true}
	case ModeProgressive:
		if idx > 0 {
			prev := lessons[idx-1]
			switch {
			case prev.Completed:
			case r.Unlock == UnlockSubmission && prev.Submitted:
			case r.Unlock == UnlockSubmission:
				return LessonAccess{Reason: LockPreviousSubmission}
			case r.Unlock == UnlockTeacherCheck:
				return LessonAccess{Reason: LockPreviousReview}
			default:
				return LessonAccess{Reason: LockPreviousLesson}
			}
		}
		if datePending {
			at := *l.OpenAt
			return LessonAccess{Reason: LockOpensAt, OpensAt: &at}
		}
		return LessonAccess{Open: true}
	case ModeClosed:
		if l.Unlocked {
			return LessonAccess{Open: true}
		}
		return LessonAccess{Reason: LockNotUnlocked}
	case ModeInterval, ModeWeekdays:
		at, ok := ScheduleOpenAt(r, idx)
		if !ok {
			return LessonAccess{Reason: LockUnsupportedMode}
		}
		if now.Before(at) {
			return LessonAccess{Reason: LockScheduled, OpensAt: &at}
		}
		return LessonAccess{Open: true}
	}
	return LessonAccess{Reason: LockUnsupportedMode}
}

// Err turns a closed lesson into the API error; nil when open.
func (a LessonAccess) Err() error {
	if a.Open {
		return nil
	}
	e := apperr.New(403, "marathon_lesson_locked", "Этот урок пока закрыт")
	e.Fields = map[string]string{"reason": a.Reason}
	if a.OpensAt != nil {
		e.Fields["opens_at"] = a.OpensAt.UTC().Format(time.RFC3339Nano)
	}
	return e
}
