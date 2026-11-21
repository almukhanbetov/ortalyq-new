package marathons

import (
	"errors"
	"testing"
	"time"

	"ortalyq/backend/internal/apperr"
)

func ptr[T any](v T) *T { return &v }

// Table tests of the access rules (plan, section 3.2). now is passed in, as the server passes now() of the DB.
func TestStudentAccessAt(t *testing.T) {
	base := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	activated := base.Add(-10 * 24 * time.Hour) // 10 days ago
	enr := AccessEnrollment{ActivatedAt: activated}
	manual := AccessEnrollment{ActivatedAt: activated, DeactivatedAt: ptr(base.Add(-time.Hour))}
	tz := "Asia/Almaty"

	cases := []struct {
		name       string
		s          AccessSettings
		e          AccessEnrollment
		now        time.Time
		status     string
		reason     string // "" = no reason
		opensAt    *time.Time
		deadline   *time.Time
		deadReason string
	}{
		{name: "no dates", s: AccessSettings{Timezone: tz}, e: enr, now: base, status: AccessOpen},
		{name: "before start", s: AccessSettings{Timezone: tz, StartAt: ptr(base.Add(time.Minute))}, e: enr, now: base,
			status: AccessNotStarted, opensAt: ptr(base.Add(time.Minute))},
		{name: "exactly at start: open", s: AccessSettings{Timezone: tz, StartAt: ptr(base)}, e: enr, now: base, status: AccessOpen},
		{name: "one nanosecond before start", s: AccessSettings{Timezone: tz, StartAt: ptr(base)}, e: enr, now: base.Add(-1),
			status: AccessNotStarted, opensAt: ptr(base)},
		{name: "before end", s: AccessSettings{Timezone: tz, EndAt: ptr(base.Add(time.Nanosecond))}, e: enr, now: base,
			status: AccessOpen, deadline: ptr(base.Add(time.Nanosecond)), deadReason: ReasonEndDate},
		{name: "exactly at end: closed", s: AccessSettings{Timezone: tz, EndAt: ptr(base)}, e: enr, now: base,
			status: AccessDeactivated, reason: ReasonEndDate, deadline: ptr(base), deadReason: ReasonEndDate},
		{name: "auto deactivation reached exactly (10 days)", s: AccessSettings{Timezone: tz, AutoDeactivateDays: ptr(10)}, e: enr, now: base,
			status: AccessDeactivated, reason: ReasonAutoDeactivation, deadline: ptr(base), deadReason: ReasonAutoDeactivation},
		{name: "auto deactivation not yet (11 days)", s: AccessSettings{Timezone: tz, AutoDeactivateDays: ptr(11)}, e: enr, now: base,
			status: AccessOpen, deadline: ptr(base.Add(24 * time.Hour)), deadReason: ReasonAutoDeactivation},
		{name: "end date earlier than auto: reason end_date",
			s: AccessSettings{Timezone: tz, AutoDeactivateDays: ptr(30), EndAt: ptr(base.Add(time.Hour))}, e: enr, now: base,
			status: AccessOpen, deadline: ptr(base.Add(time.Hour)), deadReason: ReasonEndDate},
		{name: "auto earlier than end date: reason auto",
			s: AccessSettings{Timezone: tz, AutoDeactivateDays: ptr(5), EndAt: ptr(base.Add(time.Hour))}, e: enr, now: base,
			status: AccessDeactivated, reason: ReasonAutoDeactivation, deadline: ptr(activated.AddDate(0, 0, 5)), deadReason: ReasonAutoDeactivation},
		{name: "deadline wins over a future start",
			s: AccessSettings{Timezone: tz, StartAt: ptr(base.Add(time.Hour)), AutoDeactivateDays: ptr(3)}, e: enr, now: base,
			status: AccessDeactivated, reason: ReasonAutoDeactivation, deadline: ptr(activated.AddDate(0, 0, 3)), deadReason: ReasonAutoDeactivation},
		{name: "manual wins over open dates", s: AccessSettings{Timezone: tz, EndAt: ptr(base.Add(48 * time.Hour))}, e: manual, now: base,
			status: AccessDeactivated, reason: ReasonManual, deadline: manual.DeactivatedAt, deadReason: ReasonManual},
		{name: "manual wins over a passed end date", s: AccessSettings{Timezone: tz, EndAt: ptr(base.Add(-48 * time.Hour))}, e: manual, now: base,
			status: AccessDeactivated, reason: ReasonManual, deadline: manual.DeactivatedAt, deadReason: ReasonManual},
		{name: "manual wins over not started", s: AccessSettings{Timezone: tz, StartAt: ptr(base.Add(time.Hour))}, e: manual, now: base,
			status: AccessDeactivated, reason: ReasonManual, deadline: manual.DeactivatedAt, deadReason: ReasonManual},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := StudentAccessAt(c.s, c.e, c.now)
			if a.Status != c.status {
				t.Fatalf("status %s, want %s (%+v)", a.Status, c.status, a)
			}
			if got := deref(a.Reason); got != c.reason {
				t.Fatalf("reason %q, want %q", got, c.reason)
			}
			if !sameTime(a.OpensAt, c.opensAt) {
				t.Fatalf("opens_at %v, want %v", a.OpensAt, c.opensAt)
			}
			if !sameTime(a.DeactivatesAt, c.deadline) || deref(a.DeactivationReason) != c.deadReason {
				t.Fatalf("deadline %v %q, want %v %q", a.DeactivatesAt, deref(a.DeactivationReason), c.deadline, c.deadReason)
			}
			if (a.Err() == nil) != (c.status == AccessOpen) {
				t.Fatalf("err %v for status %s", a.Err(), a.Status)
			}
		})
	}
}

// Error codes follow the reason.
func TestAccessErrors(t *testing.T) {
	for reason, code := range map[string]string{ReasonManual: "marathon_deactivated", ReasonEndDate: "marathon_closed", ReasonAutoDeactivation: "marathon_auto_deactivated"} {
		r := reason
		if got := (Access{Status: AccessDeactivated, Reason: &r}).Err(); got == nil || errCode(got) != code {
			t.Fatalf("%s: %v", reason, got)
		}
	}
	at := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)
	err := Access{Status: AccessNotStarted, OpensAt: &at}.Err()
	e := ErrNotStarted(at)
	if errCode(err) != "marathon_not_started" || e.Fields["opens_at"] != "2026-12-01T05:00:00Z" {
		t.Fatalf("not started: %v %v", err, e.Fields)
	}
}

// Days are counted in the marathon time zone: across a daylight-saving change the wall-clock time stays.
func TestDeadlineDaylightSaving(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	activated := time.Date(2026, 3, 28, 10, 0, 0, 0, berlin) // Saturday before the switch to summer time
	at, reason := Deadline(AccessSettings{Timezone: "Europe/Berlin", AutoDeactivateDays: ptr(1)}, AccessEnrollment{ActivatedAt: activated.UTC()})
	if reason != ReasonAutoDeactivation || !at.Equal(time.Date(2026, 3, 29, 10, 0, 0, 0, berlin)) {
		t.Fatalf("deadline %v %s", at, reason)
	}
	if d := at.Sub(activated); d != 23*time.Hour { // the night is one hour shorter
		t.Fatalf("duration %v", d)
	}
	// An unknown zone falls back to UTC instead of failing the request.
	at, _ = Deadline(AccessSettings{Timezone: "Nowhere/City", AutoDeactivateDays: ptr(1)}, AccessEnrollment{ActivatedAt: activated})
	if !at.Equal(activated.Add(24 * time.Hour)) {
		t.Fatalf("fallback: %v", at)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func errCode(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// Table tests of the lesson rule per mode (plan, section 3.2, step 4).
func TestLessonAccessAt(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	done := LessonState{Completed: true}
	todo := LessonState{}
	cases := []struct {
		name    string
		mode    string
		lessons []LessonState
		idx     int
		open    bool
		reason  string
	}{
		{"open, no date", ModeOpen, []LessonState{todo}, 0, true, ""},
		{"open, date in the future", ModeOpen, []LessonState{{OpenAt: ptr(now.Add(time.Nanosecond))}}, 0, false, LockOpensAt},
		{"open, exactly at the date", ModeOpen, []LessonState{{OpenAt: ptr(now)}}, 0, true, ""},
		{"open, date passed", ModeOpen, []LessonState{{OpenAt: ptr(now.Add(-time.Hour))}}, 0, true, ""},
		{"open ignores order and completion", ModeOpen, []LessonState{todo, todo}, 1, true, ""},
		{"progressive, first lesson", ModeProgressive, []LessonState{todo, todo}, 0, true, ""},
		{"progressive, previous not completed", ModeProgressive, []LessonState{todo, todo}, 1, false, LockPreviousLesson},
		{"progressive, previous completed", ModeProgressive, []LessonState{done, todo}, 1, true, ""},
		{"progressive, only the direct predecessor counts", ModeProgressive, []LessonState{todo, done, todo}, 2, true, ""},
		{"progressive, a gap closes the next one", ModeProgressive, []LessonState{done, todo, todo}, 2, false, LockPreviousLesson},
		{"closed, not opened", ModeClosed, []LessonState{todo}, 0, false, LockNotUnlocked},
		{"closed, opened", ModeClosed, []LessonState{{Unlocked: true}}, 0, true, ""},
		{"closed, completion does not open", ModeClosed, []LessonState{done, todo}, 1, false, LockNotUnlocked},
		{"progressive: previous completed but the date is in the future (Н3 = A, both conditions)", ModeProgressive,
			[]LessonState{done, {OpenAt: ptr(now.Add(time.Minute))}}, 1, false, LockOpensAt},
		{"progressive: date passed but the previous lesson is not completed", ModeProgressive,
			[]LessonState{todo, {OpenAt: ptr(now.Add(-time.Minute))}}, 1, false, LockPreviousLesson},
		{"progressive: both conditions met", ModeProgressive, []LessonState{done, {OpenAt: ptr(now)}}, 1, true, ""},
		{"progressive: a date also holds the first lesson", ModeProgressive, []LessonState{{OpenAt: ptr(now.Add(time.Minute))}}, 0, false, LockOpensAt},
		{"closed ignores dates", ModeClosed, []LessonState{{Unlocked: true, OpenAt: ptr(now.Add(time.Hour))}}, 0, true, ""},
		{"unknown mode", "random", []LessonState{todo}, 0, false, LockUnsupportedMode},
		{"index out of range", ModeOpen, []LessonState{todo}, 3, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := LessonAccessAt(LessonRules{Mode: c.mode}, c.lessons, c.idx, now)
			if a.Open != c.open || a.Reason != c.reason {
				t.Fatalf("got %+v, want open=%v reason=%q", a, c.open, c.reason)
			}
			if (a.Err() == nil) != c.open {
				t.Fatalf("err %v", a.Err())
			}
		})
	}
	e := LessonAccess{Reason: LockOpensAt, OpensAt: ptr(now)}.Err()
	var ae *apperr.Error
	if !errors.As(e, &ae) || ae.Code != "marathon_lesson_locked" || ae.Fields["reason"] != LockOpensAt || ae.Fields["opens_at"] != "2026-11-20T12:00:00Z" {
		t.Fatalf("lock error: %v", e)
	}
}

// Schedules of "interval" and "weekdays" (Н1 = A): lesson 0 at the anchor = max(start_at, activated_at),
// the next lessons by the schedule in the marathon time zone.
func TestScheduleOpenAt(t *testing.T) {
	almaty, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, almaty) }
	// Friday 20 November 2026, 15:30 Almaty.
	anchor := at(2026, 11, 20, 15, 30)
	mon, wed, fri, sun := 1, 4, 16, 64
	cases := []struct {
		name string
		r    LessonRules
		k    int
		want time.Time
	}{
		{"interval: lesson 0 at the anchor", LessonRules{Mode: ModeInterval, Anchor: anchor, IntervalDays: 2, OpenHour: 9, Location: almaty}, 0, anchor},
		{"interval: lesson 1 = anchor date + 2 days at 09:00", LessonRules{Mode: ModeInterval, Anchor: anchor, IntervalDays: 2, OpenHour: 9, Location: almaty}, 1, at(2026, 11, 22, 9, 0)},
		{"interval: lesson 3 = + 6 days", LessonRules{Mode: ModeInterval, Anchor: anchor, IntervalDays: 2, OpenHour: 9, Location: almaty}, 3, at(2026, 11, 26, 9, 0)},
		{"interval: month boundary", LessonRules{Mode: ModeInterval, Anchor: anchor, IntervalDays: 7, OpenHour: 0, Location: almaty}, 2, at(2026, 12, 4, 0, 0)},
		{"weekdays: the anchor day counts when the hour is later (Fri 18:00 > Fri 15:30)",
			LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: fri, OpenHour: 18, Location: almaty}, 1, at(2026, 11, 20, 18, 0)},
		{"weekdays: the anchor day does not count when the hour has passed (Fri 09:00 < 15:30) → next Friday",
			LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: fri, OpenHour: 9, Location: almaty}, 1, at(2026, 11, 27, 9, 0)},
		{"weekdays: exactly at the anchor moment is not after it → next week",
			LessonRules{Mode: ModeWeekdays, Anchor: at(2026, 11, 20, 9, 0), Weekdays: fri, OpenHour: 9, Location: almaty}, 1, at(2026, 11, 27, 9, 0)},
		{"weekdays: Mon+Wed, lesson 1 = Monday", LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: mon | wed, OpenHour: 10, Location: almaty}, 1, at(2026, 11, 23, 10, 0)},
		{"weekdays: Mon+Wed, lesson 2 = Wednesday", LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: mon | wed, OpenHour: 10, Location: almaty}, 2, at(2026, 11, 25, 10, 0)},
		{"weekdays: Mon+Wed, lesson 3 = next Monday", LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: mon | wed, OpenHour: 10, Location: almaty}, 3, at(2026, 11, 30, 10, 0)},
		{"weekdays: Sunday bit", LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: sun, OpenHour: 0, Location: almaty}, 1, at(2026, 11, 22, 0, 0)},
		{"weekdays: every day", LessonRules{Mode: ModeWeekdays, Anchor: anchor, Weekdays: 127, OpenHour: 8, Location: almaty}, 1, at(2026, 11, 21, 8, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ScheduleOpenAt(c.r, c.k)
			if !ok || !got.Equal(c.want) {
				t.Fatalf("got %v %v, want %v", got.In(almaty), ok, c.want)
			}
		})
	}
	if _, ok := ScheduleOpenAt(LessonRules{Mode: ModeWeekdays, Anchor: anchor, Location: almaty}, 1); ok {
		t.Fatal("an empty weekday mask has no schedule")
	}
	if _, ok := ScheduleOpenAt(LessonRules{Mode: ModeOpen, Anchor: anchor}, 1); ok {
		t.Fatal("open mode has no schedule")
	}

	// Daylight saving: the hour on the wall clock stays (Berlin switches on 29 March 2026).
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	r := LessonRules{Mode: ModeInterval, Anchor: time.Date(2026, 3, 27, 12, 0, 0, 0, berlin), IntervalDays: 3, OpenHour: 9, Location: berlin}
	if got, _ := ScheduleOpenAt(r, 1); !got.Equal(time.Date(2026, 3, 30, 9, 0, 0, 0, berlin)) || got.UTC().Hour() != 7 {
		t.Fatalf("summer time: %v (UTC %v)", got, got.UTC())
	}
}

// Boundaries of the schedule rule and the anchor of a student added late.
func TestScheduleLessonAccess(t *testing.T) {
	almaty, _ := time.LoadLocation("Asia/Almaty")
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, almaty)
	st := &state{Mode: ModeInterval, StartAt: &start, Timezone: "Asia/Almaty", IntervalDays: ptr(1), OpenHour: ptr(9)}
	lessons := []LessonState{{}, {}, {}}

	// Added before the start: the schedule counts from start_at.
	early := st.rules(start.Add(-48 * time.Hour))
	if !early.Anchor.Equal(start) {
		t.Fatalf("anchor of an early student: %v", early.Anchor)
	}
	lesson1 := time.Date(2026, 11, 21, 9, 0, 0, 0, almaty)
	if a := LessonAccessAt(early, lessons, 1, lesson1.Add(-time.Nanosecond)); a.Open || a.Reason != LockScheduled || !a.OpensAt.Equal(lesson1) {
		t.Fatalf("one nanosecond before: %+v", a)
	}
	if a := LessonAccessAt(early, lessons, 1, lesson1); !a.Open {
		t.Fatalf("exactly at the moment: %+v", a)
	}
	if a := LessonAccessAt(early, lessons, 0, start); !a.Open {
		t.Fatalf("lesson 0 at the anchor: %+v", a)
	}

	// Added three days after the start: their own schedule from the adding moment, lesson 0 right away.
	added := time.Date(2026, 11, 23, 20, 0, 0, 0, almaty)
	late := st.rules(added)
	if !late.Anchor.Equal(added) {
		t.Fatalf("anchor of a late student: %v", late.Anchor)
	}
	if a := LessonAccessAt(late, lessons, 0, added); !a.Open {
		t.Fatalf("late student, lesson 0: %+v", a)
	}
	if a := LessonAccessAt(late, lessons, 1, added.Add(time.Hour)); a.Open || !a.OpensAt.Equal(time.Date(2026, 11, 24, 9, 0, 0, 0, almaty)) {
		t.Fatalf("late student, lesson 1: %+v", a)
	}
	// The early student already has lesson 2 open at that moment: schedules are per student.
	if a := LessonAccessAt(early, lessons, 2, added); !a.Open {
		t.Fatalf("early student, lesson 2: %+v", a)
	}
}

// Stage 6: "По мере прохождения" with the teacher check. Without the check (Unlock "") the stage 5 rule stays.
func TestLessonAccessTeacherCheck(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	done := LessonState{Completed: true}
	sent := LessonState{Submitted: true} // on review or returned for revision
	todo := LessonState{}
	cases := []struct {
		name    string
		unlock  string
		lessons []LessonState
		idx     int
		open    bool
		reason  string
	}{
		{"teacher_check: first lesson is open", UnlockTeacherCheck, []LessonState{todo, todo}, 0, true, ""},
		{"teacher_check: previous not sent", UnlockTeacherCheck, []LessonState{todo, todo}, 1, false, LockPreviousReview},
		{"teacher_check: previous sent, not accepted", UnlockTeacherCheck, []LessonState{sent, todo}, 1, false, LockPreviousReview},
		{"teacher_check: previous accepted", UnlockTeacherCheck, []LessonState{done, todo}, 1, true, ""},
		{"teacher_check: accepted, but the date is in the future", UnlockTeacherCheck, []LessonState{done, {OpenAt: ptr(now.Add(time.Second))}}, 1, false, LockOpensAt},
		{"submission: previous not sent", UnlockSubmission, []LessonState{todo, todo}, 1, false, LockPreviousSubmission},
		{"submission: previous sent", UnlockSubmission, []LessonState{sent, todo}, 1, true, ""},
		{"submission: previous completed", UnlockSubmission, []LessonState{done, todo}, 1, true, ""},
		{"submission: sent, date in the future", UnlockSubmission, []LessonState{sent, {OpenAt: ptr(now.Add(time.Hour))}}, 1, false, LockOpensAt},
		{"no check: a sent lesson does not open the next", "", []LessonState{sent, todo}, 1, false, LockPreviousLesson},
		{"no check: completed opens the next", "", []LessonState{done, todo}, 1, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := LessonAccessAt(LessonRules{Mode: ModeProgressive, Unlock: c.unlock}, c.lessons, c.idx, now)
			if a.Open != c.open || a.Reason != c.reason {
				t.Fatalf("got %+v, want open=%v reason=%q", a, c.open, c.reason)
			}
		})
	}
	// The unlock rule matters only in the progressive mode.
	for _, mode := range []string{ModeOpen, ModeClosed} {
		a := LessonAccessAt(LessonRules{Mode: mode, Unlock: UnlockTeacherCheck}, []LessonState{todo, {Unlocked: true}}, 1, now)
		if !a.Open {
			t.Fatalf("%s must ignore unlock_after: %+v", mode, a)
		}
	}
	// The rules of a marathon with the check off ignore unlock_after.
	st := state{Mode: ModeProgressive, Timezone: "Asia/Almaty", UnlockAfter: UnlockSubmission}
	if r := st.rules(now); r.Unlock != "" {
		t.Fatalf("check off: %+v", r)
	}
	st.TeacherCheck = true
	if r := st.rules(now); r.Unlock != UnlockSubmission {
		t.Fatalf("check on: %+v", r)
	}
}
