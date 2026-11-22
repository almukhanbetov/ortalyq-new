package marathons

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/mail"
	"ortalyq/backend/internal/modules/attempts"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/classrooms"
	"ortalyq/backend/internal/modules/exercises"
	"ortalyq/backend/internal/modules/materials"
	"ortalyq/backend/internal/modules/uploads"
	"ortalyq/backend/internal/realtime"
)

// Classrooms is satisfied by classrooms.Service.
type Classrooms interface {
	Get(ctx context.Context, p auth.Principal, id uuid.UUID) (*classrooms.Classroom, error)
	MemberRole(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (string, error)
	RequireTeacher(ctx context.Context, p auth.Principal, classroomID uuid.UUID) error
}

// LessonOwnership is satisfied by materials.Service.
type LessonOwnership interface {
	OwnedLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) (*materials.LessonRef, error)
}

// Materials is satisfied by materials.Service.
type Materials interface {
	LessonRef(ctx context.Context, lessonID uuid.UUID) (*materials.LessonRef, error)
	Sections(ctx context.Context, lessonID uuid.UUID) ([]materials.Section, error)
	UpdateLesson(ctx context.Context, p auth.Principal, id uuid.UUID, req materials.LessonRequest) (*materials.Lesson, error)
}

// Results is satisfied by attempts.Service. ReviewLesson and Points only read; FinalizeScope and ReopenScope run
// inside the caller's transaction for the teacher check (stage 6), as for homework.
type Results interface {
	ReviewLesson(ctx context.Context, classroomID, lessonID, studentID uuid.UUID) ([]attempts.ExerciseView, error)
	Points(ctx context.Context, classroomID, lessonID, studentID uuid.UUID, scope string) (*attempts.Points, error)
	FinalizeScope(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, scope string) ([]attempts.SnapshotExercise, error)
	ReopenScope(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, scope string) error
	// PointsFor recomputes points of many students and lessons at once (statistics and rating, stage 7).
	PointsFor(ctx context.Context, classroomID uuid.UUID, lessonIDs, studentIDs []uuid.UUID, scope string) (map[uuid.UUID]map[uuid.UUID]*attempts.Points, error)
}

// AssignmentChecker is satisfied by users.Service: a teacher adds only students assigned to them.
type AssignmentChecker interface {
	IsAssigned(ctx context.Context, teacherID, studentID uuid.UUID) (bool, error)
}

type Service struct {
	repo    *Repository
	cls     Classrooms
	lessons LessonOwnership
	mat     Materials
	results Results
	assign  AssignmentChecker
	pub     realtime.Publisher
	// Invitations by e-mail (stage 9), wired with SetInvitations.
	mailer   Mailer
	invitees Invitees
	appURL   string
	// Appearance pictures (stage 10), wired with SetImages.
	images Images
}

func NewService(repo *Repository, cls Classrooms, lessons LessonOwnership, mat Materials, results Results, assign AssignmentChecker, pub realtime.Publisher) *Service {
	return &Service{repo: repo, cls: cls, lessons: lessons, mat: mat, results: results, assign: assign, pub: pub}
}

// ErrDeactivated is returned to a student the teacher has deactivated in this marathon. It is checked on the
// server for every way in: the lesson list, a direct lesson URL and the attempts API (start, answer, finish).
func ErrDeactivated() *apperr.Error {
	return apperr.New(403, "marathon_deactivated", "Преподаватель закрыл вам доступ к этому марафону")
}

// ErrClosed is returned to a student once the marathon end date has passed (also for direct API calls).
func ErrClosed() *apperr.Error {
	return apperr.New(403, "marathon_closed", "Марафон завершён. Доступ к марафону закрыт")
}

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// normalize trims and validates the basic fields (shared by create and update).
func normalize(req *SaveRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.CoverColor == "" {
		req.CoverColor = "#1AC6FF"
	}
	if req.Direction == "" {
		req.Direction = DirectionLanguages
	}
	if req.TargetLanguage != nil {
		l := strings.ToLower(strings.TrimSpace(*req.TargetLanguage))
		req.TargetLanguage = &l
		if l == "" {
			req.TargetLanguage = nil
		}
	}
	v := apperr.V{}
	v.Check(req.Name != "" && len([]rune(req.Name)) <= 120, "name", "Укажите название марафона")
	v.Check(colorRe.MatchString(req.CoverColor), "cover_color", "Некорректный цвет обложки")
	switch req.Direction {
	case DirectionLanguages:
		v.Check(req.TargetLanguage != nil && knownLanguage(*req.TargetLanguage), "target_language", "Выберите изучаемый язык")
	case DirectionOther:
		req.TargetLanguage = nil // the language field is hidden for "Другое"
	default:
		v.Add("direction", "Неизвестное направление")
	}
	return v.Err()
}

// owned returns the marathon when the caller is its teacher; anything else is 404/403 as for classes.
func (s *Service) owned(ctx context.Context, p auth.Principal, id uuid.UUID) (*classrooms.Classroom, error) {
	if err := s.cls.RequireTeacher(ctx, p, id); err != nil {
		return nil, err
	}
	c, err := s.cls.Get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if c.LessonType != classrooms.TypeMarathon {
		return nil, apperr.NotFound("Марафон")
	}
	return c, nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, req SaveRequest) (*classrooms.Classroom, error) {
	if !p.CanTeach() {
		return nil, apperr.Forbidden()
	}
	if err := normalize(&req); err != nil {
		return nil, err
	}
	tz := p.Timezone
	if tz == "" {
		tz = "Asia/Almaty"
	}
	id, err := s.repo.Create(ctx, p.UserID, req, tz)
	if err != nil {
		return nil, err
	}
	return s.cls.Get(ctx, p, id)
}

func (s *Service) Update(ctx context.Context, p auth.Principal, id uuid.UUID, req SaveRequest) (*classrooms.Classroom, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	if err := normalize(&req); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, req); err != nil {
		return nil, err
	}
	return s.cls.Get(ctx, p, id)
}

func (s *Service) Archive(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	if _, err := s.owned(ctx, p, id); err != nil {
		return err
	}
	return s.repo.Archive(ctx, id, p.UserID)
}

// ---- Access gate (classrooms.MarathonGate) ----

// StudentAccess implements classrooms.MarathonGate: the single server-side decision for every lesson action of a
// student (lesson list aside): RequireLesson (direct lesson URL, exercises, progress, starting attempts, homework),
// answering and finishing an attempt, completing a lesson. Rules are in access.go (StudentAccessAt) and use
// now() of the DB. It runs inside the caller's transaction, so an answer racing with a deactivation sees the
// decision. Nothing is deleted by any refusal.
func (s *Service) StudentAccess(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) error {
	st, err := s.repo.State(ctx, q, classroomID)
	if err != nil {
		if database.IsNoRows(err) {
			return nil // not a marathon
		}
		return err
	}
	enr, err := s.repo.Participation(ctx, q, classroomID, studentID)
	if err != nil {
		return err
	}
	if enr == nil {
		return apperr.NotFound("Марафон")
	}
	if err := StudentAccessAt(st.access(), *enr, st.Now).Err(); err != nil {
		return err
	}
	if lessonID == uuid.Nil {
		return nil
	}
	// Lesson-level rule of the passage mode (stage 5), only after the student-level access allowed the student.
	ids, states, err := s.repo.LessonStates(ctx, q, classroomID, studentID)
	if err != nil {
		return err
	}
	for i, id := range ids {
		if id == lessonID {
			return LessonAccessAt(st.rules(enr.ActivatedAt), states, i, st.Now).Err()
		}
	}
	return apperr.NotFound("Урок")
}

func (s *Service) changed(id uuid.UUID, what string) {
	s.pub.Publish(id, realtime.EventMarathonChanged, map[string]any{"what": what})
}

// ---- Passage settings ----

func (s *Service) SetPassage(ctx context.Context, p auth.Principal, id uuid.UUID, req PassageRequest) (*classrooms.Classroom, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	v := apperr.V{}
	v.Check(req.EndAt == nil || !req.EndAt.IsZero(), "end_at", "Укажите дату завершения")
	v.Check(req.StartAt == nil || !req.StartAt.IsZero(), "start_at", "Укажите дату старта")
	if req.StartAt != nil && req.EndAt != nil && !req.EndAt.After(*req.StartAt) {
		v.Add("end_at", "Дата завершения должна быть позже даты старта")
	}
	if d := req.AutoDeactivateDays; d != nil {
		v.Check(*d >= 1 && *d <= 3650, "auto_deactivate_days", "Срок — от 1 до 3650 дней")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if err := s.repo.SetPassage(ctx, id, req); err != nil {
		return nil, err
	}
	s.changed(id, "settings")
	return s.cls.Get(ctx, p, id)
}

// ---- Lessons ----

// AddLesson adds a lesson from the teacher's own materials, or creates a new lesson in the marathon's own course.
func (s *Service) AddLesson(ctx context.Context, p auth.Principal, id uuid.UUID, req AddLessonRequest) (*LessonsResponse, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	switch {
	case req.New != nil && req.LessonID == nil:
		f := *req.New
		if f.Title == nil {
			f.Title = new(string)
		}
		if err := checkLessonFields(&f, true); err != nil {
			return nil, err
		}
		cover := ""
		if f.CoverColor != nil {
			cover = *f.CoverColor
		}
		if cover == "" {
			cover = "#1AC6FF"
		}
		desc := ""
		if f.Description != nil {
			desc = *f.Description
		}
		if _, err := s.repo.CreateLesson(ctx, id, p.UserID, *f.Title, desc, cover); err != nil {
			return nil, err
		}
	case req.LessonID != nil && req.New == nil:
		ref, err := s.lessons.OwnedLesson(ctx, p, *req.LessonID)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddLesson(ctx, id, ref.LessonID, ref.CourseID); err != nil {
			return nil, err
		}
	default:
		return nil, apperr.Validation(map[string]string{"lesson_id": "Выберите урок или создайте новый"})
	}
	s.changed(id, "lessons")
	return s.Lessons(ctx, p, id)
}

// checkLessonFields trims and validates the lesson settings. requireTitle is set when creating.
func checkLessonFields(f *LessonFields, requireTitle bool) error {
	v := apperr.V{}
	if f.Title != nil {
		t := strings.TrimSpace(*f.Title)
		f.Title = &t
		v.Check(t != "" && len([]rune(t)) <= 200, "title", "Укажите название урока (до 200 символов)")
	} else if requireTitle {
		v.Add("title", "Укажите название урока (до 200 символов)")
	}
	if f.Description != nil {
		d := strings.TrimSpace(*f.Description)
		f.Description = &d
		v.Check(len([]rune(d)) <= 2000, "description", "Описание — не длиннее 2000 символов")
	}
	if f.CoverColor != nil && *f.CoverColor != "" {
		v.Check(colorRe.MatchString(*f.CoverColor), "cover_color", "Некорректный цвет обложки")
	}
	return v.Err()
}

// marathonLesson checks that the caller owns the marathon and the lesson is in it (removed lessons are 404).
func (s *Service) marathonLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) error {
	if _, err := s.owned(ctx, p, id); err != nil {
		return err
	}
	ok, err := s.repo.HasLesson(ctx, s.repo.Pool(), id, lessonID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.NotFound("Урок")
	}
	return nil
}

// UpdateLesson saves "Настройки урока". Title and description change the lesson template (the same lesson in
// other classes shows the new text); the cover colour is stored for this marathon only.
func (s *Service) UpdateLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, f LessonFields) (*LessonsResponse, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	if err := checkLessonFields(&f, false); err != nil {
		return nil, err
	}
	if f.Title != nil || f.Description != nil {
		if _, err := s.mat.UpdateLesson(ctx, p, lessonID, materials.LessonRequest{Title: f.Title, Description: f.Description}); err != nil {
			return nil, err
		}
	}
	if f.CoverColor != nil {
		var cover *string
		if *f.CoverColor != "" {
			cover = f.CoverColor
		}
		if err := s.repo.SetCover(ctx, id, lessonID, cover); err != nil {
			return nil, err
		}
	}
	s.changed(id, "lessons")
	return s.Lessons(ctx, p, id)
}

// ReorderLessons stores the order shown in the "Урок" tab; the list must contain every lesson exactly once.
func (s *Service) ReorderLessons(ctx context.Context, p auth.Principal, id uuid.UUID, req ReorderLessonsRequest) (*LessonsResponse, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	ok, err := s.repo.ReorderLessons(ctx, id, req.LessonIDs)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.Validation(map[string]string{"lesson_ids": "Список должен содержать все уроки марафона ровно один раз"})
	}
	s.changed(id, "lessons")
	return s.Lessons(ctx, p, id)
}

// RemoveLesson takes a lesson out of the marathon. The lesson stays in the teacher's materials; students'
// answers and progress are kept and come back if the lesson is added again. Repeated calls are harmless.
func (s *Service) RemoveLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*LessonsResponse, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	found, err := s.repo.RemoveLesson(ctx, id, lessonID, p.UserID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperr.NotFound("Урок")
	}
	s.changed(id, "lessons")
	return s.Lessons(ctx, p, id)
}

// CopyLesson copies a lesson of the marathon into the marathon's own course and adds the copy at the end.
// The original and its results are not changed.
func (s *Service) CopyLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*LessonCopied, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	if _, err := s.lessons.OwnedLesson(ctx, p, lessonID); err != nil {
		return nil, err
	}
	newID, err := s.repo.CopyLesson(ctx, id, p.UserID, lessonID, " (копия)")
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Урок")
		}
		return nil, err
	}
	s.changed(id, "lessons")
	list, err := s.Lessons(ctx, p, id)
	if err != nil {
		return nil, err
	}
	return &LessonCopied{LessonID: newID, LessonsResponse: *list}, nil
}

// Lessons returns the marathon lessons: with per-lesson completion counts for the teacher,
// with the caller's own progress for a student.
func (s *Service) Lessons(ctx context.Context, p auth.Principal, id uuid.UUID) (*LessonsResponse, error) {
	role, err := s.marathonRole(ctx, p, id)
	if err != nil {
		return nil, err
	}
	var sid *uuid.UUID
	if role == classrooms.MemberStudent {
		sid = &p.UserID
		enr, err := s.repo.Participation(ctx, s.repo.Pool(), id, p.UserID)
		if err != nil {
			return nil, err
		}
		if enr == nil {
			return nil, apperr.NotFound("Марафон")
		}
		if enr.DeactivatedAt != nil {
			return nil, ErrDeactivated()
		}
	}
	list, err := s.repo.Lessons(ctx, id, sid)
	if err != nil {
		return nil, err
	}
	out := &LessonsResponse{Items: list, TotalLessons: len(list)}
	var forAll map[uuid.UUID]bool
	if role == classrooms.MemberTeacher {
		if forAll, err = s.repo.UnlockedForAll(ctx, id); err != nil {
			return nil, err
		}
	}
	for i := range list {
		list[i].UnlockedForAll = forAll[list[i].LessonID]
		if role != classrooms.MemberTeacher {
			forStudent(&list[i]) // students do not see how many others have finished
		}
		if list[i].Completed {
			out.CompletedLessons++
		}
	}
	return out, nil
}

// Me is the student's own view: their access decision (status, reason, opening and deactivation moments) and the
// lessons with their access. Teachers get 403: they are not participants. A student deactivated by the teacher
// gets only the status, no lessons (stage 3 decision, Н8).
func (s *Service) Me(ctx context.Context, p auth.Principal, id uuid.UUID) (*MeResponse, error) {
	role, err := s.marathonRole(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.New(403, "not_participant", "Этот раздел — для участников марафона")
	}
	st, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	enr, err := s.repo.Participation(ctx, s.repo.Pool(), id, p.UserID)
	if err != nil {
		return nil, err
	}
	if enr == nil {
		return nil, apperr.NotFound("Марафон")
	}
	acc := StudentAccessAt(st.access(), *enr, st.Now)
	out := &MeResponse{Access: acc, ServerNow: st.Now, Items: []MeLesson{}}
	if enr.DeactivatedAt != nil {
		return out, nil
	}
	list, err := s.repo.Lessons(ctx, id, &p.UserID)
	if err != nil {
		return nil, err
	}
	locks, err := s.lessonLocks(ctx, id, p.UserID, st)
	if err != nil {
		return nil, err
	}
	out.Mode = st.Mode
	out.TotalLessons = len(list)
	for _, l := range list {
		forStudent(&l)
		if l.Completed {
			out.CompletedLessons++
		}
		item := MeLesson{Lesson: l, Access: acc.Status}
		// The lesson rule matters only while the student-level access is open.
		if acc.Status == AccessOpen {
			if la, ok := locks[l.LessonID]; ok && !la.Open {
				item.Access, item.Lock = AccessLocked, &la
			}
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// lessonLocks computes the lesson rule of the mode for every active lesson of one student.
func (s *Service) lessonLocks(ctx context.Context, id, studentID uuid.UUID, st *state) (map[uuid.UUID]LessonAccess, error) {
	enr, err := s.repo.Participation(ctx, s.repo.Pool(), id, studentID)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]LessonAccess{}
	if enr == nil {
		return out, nil
	}
	ids, states, err := s.repo.LessonStates(ctx, s.repo.Pool(), id, studentID)
	if err != nil {
		return nil, err
	}
	rules := st.rules(enr.ActivatedAt)
	for i, lid := range ids {
		out[lid] = LessonAccessAt(rules, states, i, st.Now)
	}
	return out, nil
}

// SetLessonOpenAt stores "Дата открытия" of a lesson (owner only; null removes it). It works only in the open
// and progressive modes (Н3 = A); the schedule and closed modes do not use it.
func (s *Service) SetLessonOpenAt(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, req OpenAtRequest) (*LessonsResponse, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	if req.OpenAt != nil {
		if req.OpenAt.IsZero() {
			return nil, apperr.Validation(map[string]string{"open_at": "Укажите дату открытия"})
		}
		st, err := s.repo.State(ctx, s.repo.Pool(), id)
		if err != nil {
			return nil, err
		}
		if !modeUsesOpenDates(st.Mode) {
			return nil, apperr.Validation(map[string]string{"open_at": "В этом способе прохождения дата открытия урока не применяется"})
		}
	}
	if _, err := s.repo.SetOpenAt(ctx, id, lessonID, req.OpenAt); err != nil {
		return nil, err
	}
	s.changed(id, "lessons")
	return s.Lessons(ctx, p, id)
}

// UnlockLesson opens a lesson in the closed mode for everyone (no student) or for one participant (owner only).
// It never overrides dates or a deactivation: those are checked before the lesson rule. Idempotent.
func (s *Service) UnlockLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, req UnlockRequest) (*LessonsResponse, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	st, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	if st.Mode != ModeClosed {
		return nil, apperr.Validation(map[string]string{"mode": "Открывать уроки вручную можно в закрытом режиме"})
	}
	if req.StudentID != nil {
		enr, err := s.repo.Participation(ctx, s.repo.Pool(), id, *req.StudentID)
		if err != nil {
			return nil, err
		}
		if enr == nil {
			return nil, apperr.Validation(map[string]string{"student_id": "Ученик не участвует в этом марафоне"})
		}
	}
	if err := s.repo.Unlock(ctx, id, lessonID, req.StudentID, p.UserID); err != nil {
		return nil, err
	}
	s.changed(id, "lessons")
	return s.Lessons(ctx, p, id)
}

// schedule validates the parameters of "interval" / "weekdays"; other modes get none.
func schedule(req ModeRequest) (Schedule, error) {
	var sch Schedule
	if req.Mode != ModeInterval && req.Mode != ModeWeekdays {
		return sch, nil
	}
	v := apperr.V{}
	if req.OpenTime == nil {
		v.Add("open_time", "Выберите время открытия уроков")
	} else if t, err := time.Parse("15:04", *req.OpenTime); err != nil || t.Minute() != 0 {
		v.Add("open_time", "Время — целый час, например 09:00")
	} else {
		h := t.Hour()
		sch.OpenHour = &h
	}
	if req.Mode == ModeInterval {
		if req.IntervalDays == nil || *req.IntervalDays < 1 || *req.IntervalDays > 7 {
			v.Add("interval_days", "Интервал — от 1 до 7 дней")
		} else {
			d := *req.IntervalDays
			sch.IntervalDays = &d
		}
	} else {
		mask := 0
		for _, d := range req.Weekdays {
			if d < 1 || d > 7 {
				v.Add("weekdays", "Неизвестный день недели")
				break
			}
			mask |= 1 << (d - 1)
		}
		if mask == 0 {
			v.Add("weekdays", "Выберите дни недели")
		}
		sch.Weekdays = &mask
	}
	return sch, v.Err()
}

// ChangeMode switches the passage mode with the original's warning accepted (confirm_reset): answers, attempts
// and points stay, lesson statuses start over in a new epoch. expected_epoch makes a repeat harmless. With the
// same mode only the schedule is updated (no reset). Switching to a mode without lesson dates resets the dates
// only with confirm_reset_dates; otherwise 409 lesson_dates_set.
func (s *Service) ChangeMode(ctx context.Context, p auth.Principal, id uuid.UUID, req ModeRequest) (*classrooms.Classroom, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	if !modeAvailable(req.Mode) {
		return nil, apperr.Validation(map[string]string{"mode": "Неизвестный способ прохождения"})
	}
	sch, err := schedule(req)
	if err != nil {
		return nil, err
	}
	st, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	if st.Mode != req.Mode && !req.ConfirmReset {
		return nil, apperr.Validation(map[string]string{"confirm_reset": "Подтвердите сброс прогресса по урокам"})
	}
	res, err := s.repo.ChangeMode(ctx, id, p.UserID, req.Mode, sch, req.ExpectedEpoch, req.ConfirmResetDates)
	if err != nil {
		return nil, err
	}
	switch {
	case res.Conflict:
		return nil, apperr.Conflict("settings_changed", "Настройки марафона уже изменились. Обновите страницу")
	case res.DatesBlock:
		return nil, apperr.Conflict("lesson_dates_set", "Выбранный способ прохождения не поддерживает даты открытия уроков. Подтвердите их сброс")
	}
	if res.Changed || res.ParamsUpdated {
		s.changed(id, "settings")
	}
	return s.cls.Get(ctx, p, id)
}

// CompleteLesson is «Завершить урок» of the student while the teacher check is off. Idempotent: a completed lesson
// stays as it is. With the check on the lesson goes to the teacher (SubmitLesson) and this answers 409. Every
// student-level and lesson-level access rule applies (deactivation, dates, the passage mode).
func (s *Service) CompleteLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*LessonsResponse, error) {
	role, err := s.marathonRole(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.New(403, "teacher_cannot_answer", "Преподаватель не проходит уроки за ученика")
	}
	changed := false
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		st, err := s.repo.LockReview(ctx, tx, id, lessonID, p.UserID)
		if err != nil {
			return err
		}
		if err := s.StudentAccess(ctx, tx, id, lessonID, p.UserID); err != nil {
			return err
		}
		if st.Status == LessonCompleted {
			return nil
		}
		if st.TeacherCheck {
			return apperr.Conflict("teacher_check_required", "В этом марафоне урок отправляется на проверку учителю")
		}
		changed = true
		return s.repo.SelfComplete(ctx, tx, id, lessonID, p.UserID, st)
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.changed(id, "progress")
	}
	return s.Lessons(ctx, p, id)
}

// SubmitLesson is «Отправить на проверку» (teacher check on): the lesson attempts are finished, an immutable
// snapshot of the answers is stored and the status becomes submitted; answers stay locked until the teacher's
// decision. Repeating while submitted changes nothing and adds no snapshot. An accepted lesson answers 409.
// The same access rules as for answering apply (deactivation, dates, the passage mode).
func (s *Service) SubmitLesson(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*LessonsResponse, error) {
	role, err := s.marathonRole(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.New(403, "teacher_cannot_answer", "Преподаватель не проходит уроки за ученика")
	}
	changed := false
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		st, err := s.repo.LockReview(ctx, tx, id, lessonID, p.UserID)
		if err != nil {
			return err
		}
		if err := s.StudentAccess(ctx, tx, id, lessonID, p.UserID); err != nil {
			return err
		}
		switch st.Status {
		case LessonSubmitted:
			return nil
		case LessonCompleted:
			return apperr.Conflict("lesson_completed", "Урок уже завершён")
		}
		if !st.TeacherCheck {
			return apperr.Conflict("teacher_check_off", "Проверка учителем выключена: завершите урок кнопкой «Завершить урок»")
		}
		snap, err := s.results.FinalizeScope(ctx, tx, id, lessonID, p.UserID, exercises.ScopeLesson)
		if err != nil {
			return err
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		changed = true
		return s.repo.Submit(ctx, tx, id, lessonID, p.UserID, st, b)
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.changed(id, "review")
	}
	return s.Lessons(ctx, p, id)
}

// reviewTransitions are the teacher's decisions: «Отправить на доработку» and «Завершить урок» for a sent lesson
// (also from revision, as homework allows), «Отменить завершение» for an accepted one.
var reviewTransitions = map[string]map[string]bool{
	LessonSubmitted: {LessonRevision: true, LessonCompleted: true},
	LessonRevision:  {LessonCompleted: true},
	LessonCompleted: {LessonSubmitted: true}, // only an accepted lesson (checked below)
}

// Review applies the owner's decision on a student's lesson (POST …/lessons/:lid/review). Rights: owner only;
// the student must be a participant and the lesson part of the marathon. The teacher is not limited by the
// student's access (dates, deactivation). Revision reopens the attempts as for homework (wrong instantly-checked
// answers become editable, correct ones stay); acceptance finishes open attempts and stores the rating and the
// comment; cancellation returns the lesson to "submitted" and keeps answers and points. Repeating the current
// status changes nothing; from_status protects against a decision made on a stale page.
func (s *Service) Review(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, req ReviewRequest) (*LessonReview, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	enr, err := s.repo.Participation(ctx, s.repo.Pool(), id, req.StudentID)
	if err != nil {
		return nil, err
	}
	if enr == nil {
		return nil, apperr.NotFound("Ученик")
	}
	var rating, comment *string
	v := apperr.V{}
	switch req.ToStatus {
	case LessonRevision, LessonSubmitted:
	case LessonCompleted:
		if req.Rating == nil || !knownRating(*req.Rating) {
			v.Add("rating", "Выберите оценку за урок")
		} else {
			rating = req.Rating
		}
		if req.Comment != nil {
			c := strings.TrimSpace(*req.Comment)
			v.Check(len([]rune(c)) <= 2000, "comment", "Комментарий — не длиннее 2000 символов")
			if c != "" {
				comment = &c
			}
		}
	default:
		v.Add("to_status", "Неизвестный статус")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	changed := false
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		st, err := s.repo.LockReview(ctx, tx, id, lessonID, req.StudentID)
		if err != nil {
			return err
		}
		if st.Status == req.ToStatus {
			return nil // a repeated decision
		}
		if req.FromStatus != "" && req.FromStatus != st.Status {
			return apperr.Conflict("status_changed", "Статус урока уже изменился — обновите страницу")
		}
		if !reviewTransitions[st.Status][req.ToStatus] || (st.Status == LessonCompleted && !st.Accepted) {
			return apperr.Conflict("transition_not_allowed", "Такой переход статуса урока недоступен")
		}
		switch req.ToStatus {
		case LessonRevision:
			if err := s.results.ReopenScope(ctx, tx, id, lessonID, req.StudentID, exercises.ScopeLesson); err != nil {
				return err
			}
		case LessonCompleted:
			// Freeze attempts left open (revision) so the accepted state is graded.
			if _, err := s.results.FinalizeScope(ctx, tx, id, lessonID, req.StudentID, exercises.ScopeLesson); err != nil {
				return err
			}
		}
		changed = true
		return s.repo.Review(ctx, tx, id, lessonID, req.StudentID, st, req.ToStatus, rating, comment, p.UserID)
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.changed(id, "review")
	}
	out, err := s.ReviewLesson(ctx, p, id, req.StudentID, lessonID)
	if out != nil {
		out.Unchanged = !changed
	}
	return out, err
}

// SetReviewSettings saves "Проверка учителем" and "Открытие уроков" (owner only). Switching the check does not
// start a new epoch: statuses are kept. A sent lesson stays on review; with the check off the student may
// complete it themselves, and the teacher can still decide on it.
func (s *Service) SetReviewSettings(ctx context.Context, p auth.Principal, id uuid.UUID, req ReviewSettingsRequest) (*classrooms.Classroom, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	v := apperr.V{}
	v.Check(req.TeacherCheck != nil, "teacher_check", "Укажите, включена ли проверка учителем")
	if req.UnlockAfter == "" {
		req.UnlockAfter = UnlockTeacherCheck
	}
	v.Check(req.UnlockAfter == UnlockTeacherCheck || req.UnlockAfter == UnlockSubmission, "unlock_after", "Неизвестный способ открытия уроков")
	if err := v.Err(); err != nil {
		return nil, err
	}
	changed, err := s.repo.SetReviewSettings(ctx, id, *req.TeacherCheck, req.UnlockAfter)
	if err != nil {
		return nil, err
	}
	if changed {
		s.changed(id, "settings")
	}
	return s.cls.Get(ctx, p, id)
}

// forStudent hides what the student must not see: the review of a lesson that is not accepted now (after
// «Отменить завершение» it stays stored as the previous review).
func forStudent(l *Lesson) {
	l.CompletedCount, l.PendingCount = 0, 0
	l.PreviousRating, l.PreviousComment = nil, nil
	if !l.Accepted {
		l.Rating, l.TeacherComment = nil, nil
	}
}

// forTeacher moves a review that is not current (cancelled completion) to the "previous" fields.
func forTeacher(l *Lesson) {
	if !l.Accepted {
		l.PreviousRating, l.PreviousComment = l.Rating, l.TeacherComment
		l.Rating, l.TeacherComment = nil, nil
	}
}

// ---- Students ----

func (s *Service) AddStudents(ctx context.Context, p auth.Principal, id uuid.UUID, req AddStudentsRequest) ([]Student, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	ids, err := uniqueIDs(req.StudentIDs)
	if err != nil {
		return nil, err
	}
	for _, sid := range ids {
		ok, err := s.assign.IsAssigned(ctx, p.UserID, sid)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperr.Validation(map[string]string{"student_ids": "Можно добавить только учеников, назначенных вам администратором"})
		}
	}
	if err := s.repo.AddStudents(ctx, id, p.UserID, ids); err != nil {
		return nil, err
	}
	s.changed(id, "students")
	return s.Students(ctx, p, id)
}

// Students lists participants with their status and progress (teacher-owner only).
// Status is computed from end_at and now() of the DB; finishing never changes the stored rows.
func (s *Service) Students(ctx context.Context, p auth.Principal, id uuid.UUID) ([]Student, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	st, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	list, err := s.repo.Students(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range list {
		// The same decision the server applies to the student (access.go), shown to the teacher.
		acc := StudentAccessAt(st.access(), AccessEnrollment{ActivatedAt: list[i].ActivatedAt, DeactivatedAt: list[i].DeactivatedAt}, st.Now)
		list[i].AccessStatus = acc.Status
		list[i].Status = StudentActive
		list[i].DeactivatesAt, list[i].DeactivationReason = acc.DeactivatesAt, acc.DeactivationReason
		if acc.Status == AccessDeactivated {
			list[i].Status, list[i].InactiveReason = StudentInactive, acc.Reason
		}
	}
	return list, nil
}

// SetStudentsActive activates or deactivates several participants at once (owner only). All ids must be
// current participants, otherwise nothing changes. Deactivation keeps answers, attempts and progress; activation
// gives access back with all of them. Repeating the action is harmless and keeps the first date and author.
func (s *Service) SetStudentsActive(ctx context.Context, p auth.Principal, id uuid.UUID, req SetActiveRequest) ([]Student, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	ids, err := uniqueIDs(req.StudentIDs)
	if err != nil {
		return nil, err
	}
	if req.Active == nil {
		return nil, apperr.Validation(map[string]string{"active": "Укажите действие"})
	}
	ok, err := s.repo.SetActive(ctx, id, p.UserID, ids, *req.Active)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.Validation(map[string]string{"student_ids": "Ученик не участвует в этом марафоне"})
	}
	s.changed(id, "students")
	return s.Students(ctx, p, id)
}

// RemoveStudent takes a student out of the marathon (owner only). Answers and progress are kept and come
// back if the student is added again.
func (s *Service) RemoveStudent(ctx context.Context, p auth.Principal, id, studentID uuid.UUID) ([]Student, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	found, err := s.repo.RemoveStudent(ctx, id, studentID, p.UserID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperr.NotFound("Ученик")
	}
	s.changed(id, "students")
	return s.Students(ctx, p, id)
}

// uniqueIDs drops repeats and limits a bulk action to 100 students.
func uniqueIDs(in []uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]bool{}
	ids := []uuid.UUID{}
	for _, sid := range in {
		if !seen[sid] {
			seen[sid] = true
			ids = append(ids, sid)
		}
	}
	if len(ids) == 0 || len(ids) > 100 {
		return nil, apperr.Validation(map[string]string{"student_ids": "Выберите учеников"})
	}
	return ids, nil
}

// student returns one participant of the owner's marathon; anyone else is 404.
func (s *Service) student(ctx context.Context, p auth.Principal, id, studentID uuid.UUID) (*Student, error) {
	list, err := s.Students(ctx, p, id)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == studentID {
			return &list[i], nil
		}
	}
	return nil, apperr.NotFound("Ученик")
}

// StudentProgress is the owner's "Прогресс ученика": status and every lesson with this student's completion.
// Read-only and available after the end date too.
func (s *Service) StudentProgress(ctx context.Context, p auth.Principal, id, studentID uuid.UUID) (*StudentProgress, error) {
	st, err := s.student(ctx, p, id, studentID)
	if err != nil {
		return nil, err
	}
	list, err := s.repo.Lessons(ctx, id, &studentID)
	if err != nil {
		return nil, err
	}
	state, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	locks, err := s.lessonLocks(ctx, id, studentID, state)
	if err != nil {
		return nil, err
	}
	forAll, err := s.repo.UnlockedForAll(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range list {
		la := locks[list[i].LessonID]
		list[i].StudentLesson = &la
		list[i].UnlockedForAll = forAll[list[i].LessonID]
		forTeacher(&list[i])
	}
	out := &StudentProgress{Student: *st, LessonsResponse: LessonsResponse{Items: list, TotalLessons: len(list)}}
	for _, l := range list {
		if l.Completed {
			out.CompletedLessons++
		}
	}
	return out, nil
}

// ReviewLesson shows the owner a lesson as the student left it: saved answers, results and points.
// Nothing is written: no attempt is started or finished and progress is not touched.
func (s *Service) ReviewLesson(ctx context.Context, p auth.Principal, id, studentID, lessonID uuid.UUID) (*LessonReview, error) {
	pr, err := s.StudentProgress(ctx, p, id, studentID)
	if err != nil {
		return nil, err
	}
	var lesson *Lesson
	for i := range pr.Items {
		if pr.Items[i].LessonID == lessonID {
			lesson = &pr.Items[i]
		}
	}
	if lesson == nil {
		return nil, apperr.NotFound("Урок")
	}
	ref, err := s.mat.LessonRef(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	secs, err := s.mat.Sections(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	views, err := s.results.ReviewLesson(ctx, id, lessonID, studentID)
	if err != nil {
		return nil, err
	}
	pts, err := s.results.Points(ctx, id, lessonID, studentID, "lesson")
	if err != nil {
		return nil, err
	}
	if secs == nil {
		secs = []materials.Section{}
	}
	subs, err := s.repo.SubmissionsCount(ctx, id, lessonID, studentID)
	if err != nil {
		return nil, err
	}
	st, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	return &LessonReview{Student: pr.Student, LessonID: lessonID, LessonTitle: ref.LessonTitle, CourseTitle: ref.CourseTitle,
		Completed: lesson.Completed, CompletedAt: lesson.CompletedAt, Sections: secs, Exercises: views, Points: pts,
		Lesson: *lesson, TeacherCheck: st.TeacherCheck, SubmissionsCount: subs}, nil
}

// marathonRole returns the caller's role in a marathon (404 for non-members and other class types).
func (s *Service) marathonRole(ctx context.Context, p auth.Principal, id uuid.UUID) (string, error) {
	role, err := s.cls.MemberRole(ctx, p, id)
	if err != nil {
		return "", err
	}
	if _, err := s.repo.State(ctx, s.repo.Pool(), id); err != nil {
		if database.IsNoRows(err) {
			return "", apperr.NotFound("Марафон")
		}
		return "", err
	}
	return role, nil
}

// ---- Statistics and rating (stage 7) ----

// ErrStatsDisabled: the statistics of the marathon is switched off (also for direct API calls).
func ErrStatsDisabled() *apperr.Error {
	return apperr.New(403, "marathon_stats_disabled", "Статистика в этом марафоне выключена")
}

// ErrRatingDisabled: the students' rating is switched off.
func ErrRatingDisabled() *apperr.Error {
	return apperr.New(403, "marathon_rating_disabled", "Рейтинг учеников в этом марафоне выключен")
}

// SetStatsSettings saves "Статистика", "Рейтинг учеников", "Отображать" and "Место ученика" (owner only).
// Switching the statistics off switches the rating off as well; nothing is deleted.
func (s *Service) SetStatsSettings(ctx context.Context, p auth.Principal, id uuid.UUID, req StatsSettingsRequest) (*classrooms.Classroom, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	v := apperr.V{}
	v.Check(req.StatsEnabled != nil, "stats_enabled", "Укажите, включена ли статистика")
	if l := req.RatingLimit; l != nil {
		v.Check(*l == 3 || *l == 5 || *l == 10, "rating_limit", "Можно показать первых 3, 5 или 10 учеников либо всех")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	rating := req.RatingEnabled && *req.StatsEnabled
	changed, err := s.repo.SetStatsSettings(ctx, id, *req.StatsEnabled, rating, req.RatingLimit, req.RatingShowPlace)
	if err != nil {
		return nil, err
	}
	if changed {
		s.changed(id, "settings")
	}
	return s.cls.Get(ctx, p, id)
}

// rated tells whether a participant takes part in the rating: everyone except students deactivated by the teacher
// or by their individual period. The marathon end date closes the marathon for all and does not remove anybody
// from the final standings.
func rated(a Access) bool {
	return a.Status != AccessDeactivated || (a.Reason != nil && *a.Reason == ReasonEndDate)
}

// statsData is what the statistics, the ratings and the results are computed from, read at one moment.
type statsData struct {
	st       *state
	students []Student            // current participants with their computed status
	access   map[uuid.UUID]Access // decision for every participant
	lessons  []Lesson             // active lessons in order
	points   map[uuid.UUID]map[uuid.UUID]*attempts.Points
	done     map[uuid.UUID]map[uuid.UUID]bool // lesson → student → completed in the current epoch
}

func (s *Service) loadStats(ctx context.Context, id uuid.UUID) (*statsData, error) {
	st, err := s.repo.State(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	students, err := s.repo.Students(ctx, id)
	if err != nil {
		return nil, err
	}
	lessons, err := s.repo.Lessons(ctx, id, nil)
	if err != nil {
		return nil, err
	}
	d := &statsData{st: st, students: students, access: map[uuid.UUID]Access{}, lessons: lessons}
	sids := make([]uuid.UUID, len(students))
	for i := range students {
		acc := StudentAccessAt(st.access(), AccessEnrollment{ActivatedAt: students[i].ActivatedAt, DeactivatedAt: students[i].DeactivatedAt}, st.Now)
		d.access[students[i].ID] = acc
		students[i].Status, students[i].AccessStatus = StudentActive, acc.Status
		if acc.Status == AccessDeactivated {
			students[i].Status, students[i].InactiveReason = StudentInactive, acc.Reason
		}
		sids[i] = students[i].ID
	}
	lids := make([]uuid.UUID, len(lessons))
	for i := range lessons {
		lids[i] = lessons[i].LessonID
	}
	if d.points, err = s.results.PointsFor(ctx, id, lids, sids, exercises.ScopeLesson); err != nil {
		return nil, err
	}
	if d.done, err = s.repo.CompletedByLesson(ctx, id); err != nil {
		return nil, err
	}
	return d, nil
}

// summary adds up one student's points over the given lessons (all active lessons when lessonIDs is nil),
// keeping the categories in the order they first appear.
func (d *statsData) summary(studentID uuid.UUID, lessonIDs []uuid.UUID) Summary {
	if lessonIDs == nil {
		for _, l := range d.lessons {
			lessonIDs = append(lessonIDs, l.LessonID)
		}
	}
	sum := Summary{Categories: []attempts.CategoryPoints{}}
	idx := map[string]int{}
	for _, lid := range lessonIDs {
		p := d.points[studentID][lid]
		if p == nil {
			continue
		}
		sum.Earned += p.Earned
		sum.Max += p.Max
		sum.Exercises += p.Exercises
		sum.FinishedExercises += p.Finished
		for _, c := range p.Categories {
			k := ""
			if c.CategoryID != nil {
				k = c.CategoryID.String()
			}
			if i, ok := idx[k]; ok {
				sum.Categories[i].Earned += c.Earned
				sum.Categories[i].Max += c.Max
				continue
			}
			idx[k] = len(sum.Categories)
			sum.Categories = append(sum.Categories, c)
		}
	}
	return sum
}

// ranking ranks the rated participants by their points over lessonIDs (nil = the whole marathon).
func (d *statsData) ranking(lessonIDs []uuid.UUID) []RatingEntry {
	entries := []RatingEntry{}
	for _, st := range d.students {
		if !rated(d.access[st.ID]) {
			continue
		}
		entries = append(entries, RatingEntry{StudentID: st.ID, FullName: st.FullName, AvatarColor: st.AvatarColor, Points: d.summary(st.ID, lessonIDs).Earned})
	}
	return Rank(entries)
}

// rows builds the statistics rows (all participants) with their places; rated students first, by place.
func (d *statsData) rows(lessonIDs []uuid.UUID) []StatsRow {
	places := map[uuid.UUID]int{}
	for _, e := range d.ranking(lessonIDs) {
		places[e.StudentID] = e.Place
	}
	out := []StatsRow{}
	for _, st := range d.students {
		row := StatsRow{StudentID: st.ID, FullName: st.FullName, Email: st.Email, AvatarColor: st.AvatarColor, Status: st.Status,
			InactiveReason: st.InactiveReason, CompletedLessons: st.CompletedLessons, TotalLessons: st.TotalLessons,
			Points: d.summary(st.ID, lessonIDs), Place: places[st.ID]}
		if len(lessonIDs) == 1 {
			row.Completed = d.done[lessonIDs[0]][st.ID]
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Place == 0) != (b.Place == 0) {
			return a.Place != 0
		}
		if a.Place != b.Place {
			return a.Place < b.Place
		}
		if a.Points.Earned != b.Points.Earned {
			return a.Points.Earned > b.Points.Earned
		}
		return strings.ToLower(a.FullName) < strings.ToLower(b.FullName)
	})
	return out
}

// viewerRating is the rating as the viewer may see it: everything for the owner, the settings' limit and the own
// place rule for a student.
func (d *statsData) viewerRating(lessonIDs []uuid.UUID, student uuid.UUID) *RatingResponse {
	ranked := d.ranking(lessonIDs)
	out := &RatingResponse{Limit: d.st.RatingLimit, ShowPlace: d.st.RatingShowPlace, Total: len(ranked)}
	if student == uuid.Nil {
		out.Items, _ = VisibleRating(ranked, nil, false, uuid.Nil)
		return out
	}
	out.Items, out.Me = VisibleRating(ranked, d.st.RatingLimit, d.st.RatingShowPlace, student)
	return out
}

// Stats is the "Статистика" tab of the owner: every participant with completed lessons, points and categories.
// The same data is exported to CSV by the page.
func (s *Service) Stats(ctx context.Context, p auth.Principal, id uuid.UUID) (*StatsResponse, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	d, err := s.loadStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if !d.st.StatsEnabled {
		return nil, ErrStatsDisabled()
	}
	out := &StatsResponse{Items: d.rows(nil), Categories: []StatsCategory{}, TotalLessons: len(d.lessons)}
	seen := map[string]bool{}
	for _, l := range d.lessons {
		for _, st := range d.students {
			for _, c := range d.points[st.ID][l.LessonID].Categories {
				k := ""
				if c.CategoryID != nil {
					k = c.CategoryID.String()
				}
				if !seen[k] {
					seen[k] = true
					out.Categories = append(out.Categories, StatsCategory{CategoryID: c.CategoryID, CategoryName: c.CategoryName})
				}
			}
		}
	}
	return out, nil
}

// statsViewer checks that the caller may see the statistics: the owner, or a participant whose access to the
// marathon is open (lessonID = uuid.Nil) or to this lesson. Returns the student id (uuid.Nil for the owner).
func (s *Service) statsViewer(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, needRating bool) (uuid.UUID, *statsData, error) {
	role, err := s.marathonRole(ctx, p, id)
	if err != nil {
		return uuid.Nil, nil, err
	}
	student := uuid.Nil
	switch role {
	case classrooms.MemberTeacher:
		if _, err := s.owned(ctx, p, id); err != nil {
			return uuid.Nil, nil, err
		}
		if lessonID != uuid.Nil {
			if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
				return uuid.Nil, nil, err
			}
		}
	case classrooms.MemberStudent:
		student = p.UserID
		if err := s.StudentAccess(ctx, s.repo.Pool(), id, lessonID, p.UserID); err != nil {
			return uuid.Nil, nil, err
		}
	default:
		return uuid.Nil, nil, apperr.Forbidden()
	}
	d, err := s.loadStats(ctx, id)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if !d.st.StatsEnabled {
		return uuid.Nil, nil, ErrStatsDisabled()
	}
	if needRating && student != uuid.Nil && !d.st.RatingEnabled {
		return uuid.Nil, nil, ErrRatingDisabled()
	}
	return student, d, nil
}

// Rating is the marathon rating: points over all active lessons. Students see it only with the rating on and
// their access open; deactivated students, removed students and the teacher are not in it.
func (s *Service) Rating(ctx context.Context, p auth.Principal, id uuid.UUID) (*RatingResponse, error) {
	student, d, err := s.statsViewer(ctx, p, id, uuid.Nil, true)
	if err != nil {
		return nil, err
	}
	return d.viewerRating(nil, student), nil
}

// LessonResults is «Результаты урока» (statistics on). A student needs access to the lesson (the same rules as
// for working on it) and sees their own points, the comment and, with the rating on, the lesson rating.
func (s *Service) LessonResults(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*LessonResults, error) {
	student, d, err := s.statsViewer(ctx, p, id, lessonID, false)
	if err != nil {
		return nil, err
	}
	out := &LessonResults{LessonID: lessonID}
	for _, l := range d.lessons {
		if l.LessonID == lessonID {
			out.Title = l.Title
		}
	}
	if out.Comment, err = s.repo.Comment(ctx, id, lessonID); err != nil {
		return nil, err
	}
	only := []uuid.UUID{lessonID}
	if student == uuid.Nil {
		out.Students = d.rows(only)
		out.Rating = d.viewerRating(only, uuid.Nil)
		return out, nil
	}
	out.Points = d.points[student][lessonID]
	if d.st.RatingEnabled {
		out.Rating = d.viewerRating(only, student)
	}
	return out, nil
}

// SetLessonComment saves «Комментарий к статистике» of a lesson (owner only). It is shown to the students of this
// marathon only (Н7), up to 2000 characters; empty removes it.
func (s *Service) SetLessonComment(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, req CommentRequest) (*LessonComment, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	body := strings.TrimSpace(req.Body)
	if len([]rune(body)) > 2000 {
		return nil, apperr.Validation(map[string]string{"body": "Комментарий — не длиннее 2000 символов"})
	}
	if err := s.repo.SetComment(ctx, id, lessonID, body, p.UserID); err != nil {
		return nil, err
	}
	s.changed(id, "stats")
	return s.repo.Comment(ctx, id, lessonID)
}

// MyResults is «Успехи ученика в марафоне» for the student: completed lessons and finished exercises always,
// points and the place only with the statistics (and the rating) on. Same access rules as the lessons.
func (s *Service) MyResults(ctx context.Context, p auth.Principal, id uuid.UUID) (*MyResults, error) {
	role, err := s.marathonRole(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.New(403, "not_participant", "Этот раздел — для участников марафона")
	}
	if err := s.StudentAccess(ctx, s.repo.Pool(), id, uuid.Nil, p.UserID); err != nil {
		return nil, err
	}
	d, err := s.loadStats(ctx, id)
	if err != nil {
		return nil, err
	}
	sum := d.summary(p.UserID, nil)
	out := &MyResults{TotalLessons: len(d.lessons), FinishedExercises: sum.FinishedExercises, Exercises: sum.Exercises, StatsEnabled: d.st.StatsEnabled}
	for _, l := range d.lessons {
		if d.done[l.LessonID][p.UserID] {
			out.CompletedLessons++
		}
	}
	if d.st.StatsEnabled {
		out.Points = &sum
		if d.st.RatingEnabled {
			out.Rating = d.viewerRating(nil, p.UserID)
		}
	}
	return out, nil
}

// ---- Invitations by e-mail (stage 9) ----

// Mailer is satisfied by mail.Dispatcher: letters are written to the outbox in the caller's transaction and sent
// after the commit.
type Mailer interface {
	Enqueue(ctx context.Context, tx pgx.Tx, kind string, refID uuid.UUID, locale string, m mail.Message) (uuid.UUID, error)
	Kick()
}

// Invitees is satisfied by users.Service: only students assigned to the teacher can be invited (Н6, default).
type Invitees interface {
	AssignedStudentByEmail(ctx context.Context, teacherID uuid.UUID, email string) (uuid.UUID, string, bool, error)
	IsAssigned(ctx context.Context, teacherID, studentID uuid.UUID) (bool, error)
}

// InviteTTL is how long an invitation link works.
const InviteTTL = 7 * 24 * time.Hour

// SetInvitations wires the mail outbox, the student lookup and the public URL of the web app (links in letters).
func (s *Service) SetInvitations(m Mailer, inv Invitees, appURL string) {
	s.mailer, s.invitees, s.appURL = m, inv, strings.TrimRight(appURL, "/")
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func newToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	t := base64.RawURLEncoding.EncodeToString(b)
	return t, tokenHash(t), nil
}

func tokenHash(t string) []byte {
	h := sha256.Sum256([]byte(t))
	return h[:]
}

// maskEmail shows the address the invitation was sent to without revealing it fully: "st***@ortalyq.local".
func maskEmail(e string) string {
	i := strings.Index(e, "@")
	if i <= 0 {
		return "***"
	}
	keep := 2
	if i < keep {
		keep = i
	}
	return e[:keep] + "***" + e[i:]
}

// letter writes the invitation letter to the outbox and links it to the invitation (inside the transaction).
func (s *Service) letter(ctx context.Context, tx pgx.Tx, invID uuid.UUID, email, student, teacher, marathon, tz, locale, token string, expires time.Time) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	exp := expires.In(loc)
	m := mail.Invitation(locale, mail.InvitationData{To: email, Student: student, Teacher: teacher, Marathon: marathon,
		Link: s.appURL + "/invite/" + token, Expires: exp.Format("02.01.2006 15:04") + " (UTC" + exp.Format("-07:00") + ")"})
	oid, err := s.mailer.Enqueue(ctx, tx, "marathon_invitation", invID, locale, m)
	if err != nil {
		return err
	}
	return s.repo.SetInvitationOutbox(ctx, tx, invID, oid)
}

// Invite is «Добавить ученика → Вручную» (owner only): one invitation letter per address. Only students assigned
// to the teacher can be invited (Н6 default); any other address answers "unavailable" without telling whether an
// account exists. An open invitation is not duplicated; an expired one gets a new link. Letters are sent after the
// commit through the outbox.
func (s *Service) Invite(ctx context.Context, p auth.Principal, id uuid.UUID, req InviteRequest) ([]InviteResult, error) {
	m, err := s.owned(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if s.mailer == nil || s.invitees == nil {
		return nil, apperr.Unavailable("mail_unavailable", "Отправка писем не настроена")
	}
	v := apperr.V{}
	locale := req.Locale
	if locale == "" {
		locale = "ru"
	}
	v.Check(locale == "ru" || locale == "kk" || locale == "en", "locale", "Выберите язык писем")
	seen := map[string]bool{}
	emails := []string{}
	for _, e := range req.Emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" && !seen[e] {
			seen[e] = true
			emails = append(emails, e)
		}
	}
	v.Check(len(emails) > 0 && len(emails) <= 20, "emails", "Укажите от 1 до 20 адресов")
	if err := v.Err(); err != nil {
		return nil, err
	}
	tz := "Asia/Almaty"
	if m.Marathon != nil {
		tz = m.Marathon.Timezone
	}
	out := []InviteResult{}
	queued := false
	for _, email := range emails {
		r := InviteResult{Email: email}
		if !emailRe.MatchString(email) || len(email) > 254 {
			r.Result = "invalid"
			out = append(out, r)
			continue
		}
		sid, name, ok, err := s.invitees.AssignedStudentByEmail(ctx, p.UserID, email)
		if err != nil {
			return nil, err
		}
		if !ok {
			r.Result = "unavailable"
			out = append(out, r)
			continue
		}
		if enr, err := s.repo.Participation(ctx, s.repo.Pool(), id, sid); err != nil {
			return nil, err
		} else if enr != nil {
			r.Result = "already_participant"
			out = append(out, r)
			continue
		}
		err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
			cur, err := s.repo.OpenInvitation(ctx, tx, id, email)
			if err != nil {
				return err
			}
			if cur != nil && !cur.Expired {
				r.Result, r.InvitationID = "already_invited", &cur.ID
				return nil
			}
			token, hash, err := newToken()
			if err != nil {
				return err
			}
			var invID uuid.UUID
			var exp time.Time
			if cur != nil {
				invID, r.Result = cur.ID, "renewed"
				if exp, err = s.repo.RenewInvitation(ctx, tx, cur.ID, locale, hash, InviteTTL); err != nil {
					return err
				}
			} else {
				var created bool
				if invID, exp, created, err = s.repo.CreateInvitation(ctx, tx, id, sid, p.UserID, email, locale, hash, InviteTTL); err != nil {
					return err
				}
				if !created { // a concurrent request created it: no second letter
					r.Result = "already_invited"
					return nil
				}
				r.Result = "invited"
			}
			r.InvitationID = &invID
			queued = true
			return s.letter(ctx, tx, invID, email, name, p.FullName, m.Name, tz, locale, token, exp)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if queued {
		s.mailer.Kick()
		s.changed(id, "invitations")
	}
	return out, nil
}

// Invitations lists the marathon invitations for the owner.
func (s *Service) Invitations(ctx context.Context, p auth.Principal, id uuid.UUID) ([]Invitation, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	return s.repo.Invitations(ctx, id)
}

// ResendInvitation is «Отправить приглашение» again (owner only): a new link and period; the old link stops
// working. Only for an invitation that is neither accepted nor revoked, to a student still assigned to the teacher.
func (s *Service) ResendInvitation(ctx context.Context, p auth.Principal, id, invID uuid.UUID) ([]Invitation, error) {
	m, err := s.owned(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if s.mailer == nil {
		return nil, apperr.Unavailable("mail_unavailable", "Отправка писем не настроена")
	}
	tz := "Asia/Almaty"
	if m.Marathon != nil {
		tz = m.Marathon.Timezone
	}
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		inv, err := s.repo.InvitationByID(ctx, tx, id, invID)
		if err != nil {
			return err
		}
		if inv == nil {
			return apperr.NotFound("Приглашение")
		}
		if inv.AcceptedAt != nil || inv.RevokedAt != nil {
			return apperr.Conflict("invitation_closed", "Приглашение уже принято или отменено")
		}
		if enr, err := s.repo.Participation(ctx, tx, id, inv.StudentID); err != nil {
			return err
		} else if enr != nil {
			return apperr.Conflict("already_participant", "Ученик уже участвует в марафоне")
		}
		ok, err := s.invitees.IsAssigned(ctx, p.UserID, inv.StudentID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.Conflict("invitation_unavailable", "Ученик больше не назначен вам")
		}
		token, hash, err := newToken()
		if err != nil {
			return err
		}
		exp, err := s.repo.RenewInvitation(ctx, tx, inv.ID, inv.Locale, hash, InviteTTL)
		if err != nil {
			return err
		}
		_, name, _, err := s.invitees.AssignedStudentByEmail(ctx, p.UserID, inv.Email)
		if err != nil {
			return err
		}
		return s.letter(ctx, tx, inv.ID, inv.Email, name, p.FullName, m.Name, tz, inv.Locale, token, exp)
	})
	if err != nil {
		return nil, err
	}
	s.mailer.Kick()
	s.changed(id, "invitations")
	return s.repo.Invitations(ctx, id)
}

// RevokeInvitation cancels an open invitation (owner only): its link stops working. Repeats are harmless; an
// accepted invitation cannot be revoked (remove the student instead).
func (s *Service) RevokeInvitation(ctx context.Context, p auth.Principal, id, invID uuid.UUID) ([]Invitation, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		inv, err := s.repo.InvitationByID(ctx, tx, id, invID)
		if err != nil {
			return err
		}
		if inv == nil {
			return apperr.NotFound("Приглашение")
		}
		if inv.AcceptedAt != nil {
			return apperr.Conflict("invitation_used", "Приглашение уже принято")
		}
		return s.repo.RevokeInvitation(ctx, tx, inv.ID, p.UserID)
	})
	if err != nil {
		return nil, err
	}
	s.changed(id, "invitations")
	return s.repo.Invitations(ctx, id)
}

func invitationStatus(inv *InvitationRow) string {
	switch {
	case inv.AcceptedAt != nil:
		return InviteAccepted
	case inv.RevokedAt != nil:
		return InviteRevoked
	case inv.Expired:
		return InviteExpired
	}
	return InvitePending
}

// InvitationInfo is the public page of a link (no sign-in needed): marathon, teacher, masked address, status.
// An unknown token, or an archived marathon, is 404.
func (s *Service) InvitationInfo(ctx context.Context, token string) (*InvitationInfo, error) {
	inv, err := s.repo.InvitationByToken(ctx, s.repo.Pool(), tokenHash(token), false)
	if err != nil {
		return nil, err
	}
	if inv == nil || inv.Archived {
		return nil, apperr.NotFound("Приглашение")
	}
	return &InvitationInfo{MarathonName: inv.MarathonName, TeacherName: inv.TeacherName, EmailMasked: maskEmail(inv.Email),
		Status: invitationStatus(inv), ExpiresAt: inv.ExpiresAt}, nil
}

// AcceptInvitation enrolls the signed-in student. The link works once, until it expires, only for the student
// account with the invited address, and only while the student is still assigned to the marathon owner. A removed
// participant comes back with their answers and progress (stage 3). Accepting again by the same student is harmless.
func (s *Service) AcceptInvitation(ctx context.Context, p auth.Principal, token string) (*Accepted, error) {
	var out *Accepted
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		inv, err := s.repo.InvitationByToken(ctx, tx, tokenHash(token), true)
		if err != nil {
			return err
		}
		if inv == nil || inv.Archived {
			return apperr.NotFound("Приглашение")
		}
		if inv.AcceptedAt != nil {
			if inv.AcceptedBy != nil && *inv.AcceptedBy == p.UserID {
				out = &Accepted{MarathonID: inv.ClassroomID}
				return nil
			}
			return apperr.Conflict("invitation_used", "Это приглашение уже использовано")
		}
		if inv.RevokedAt != nil {
			return apperr.New(410, "invitation_revoked", "Преподаватель отменил это приглашение")
		}
		if inv.Expired {
			return apperr.New(410, "invitation_expired", "Срок действия приглашения истёк. Попросите преподавателя отправить его снова")
		}
		if !p.IsStudent() || !strings.EqualFold(p.Email, inv.Email) || p.UserID != inv.StudentID {
			e := apperr.New(403, "invitation_wrong_account", "Приглашение отправлено на другой адрес. Войдите под учётной записью, на которую пришло письмо")
			e.Fields = map[string]string{"email": maskEmail(inv.Email)}
			return e
		}
		ok, err := s.invitees.IsAssigned(ctx, inv.OwnerID, p.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.New(403, "invitation_unavailable", "Приглашение больше недействительно. Обратитесь к преподавателю")
		}
		actor := inv.OwnerID
		if inv.InvitedBy != nil {
			actor = *inv.InvitedBy
		}
		if err := addStudentsTx(ctx, tx, inv.ClassroomID, actor, []uuid.UUID{p.UserID}); err != nil {
			return err
		}
		if err := s.repo.AcceptInvitation(ctx, tx, inv.ID, p.UserID); err != nil {
			return err
		}
		out = &Accepted{MarathonID: inv.ClassroomID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.changed(out.MarathonID, "students")
	return out, nil
}

// ---- Appearance (stage 10) ----

// Images is satisfied by uploads.Service: appearance pictures are attachments of the marathon class, opened only
// by its members.
type Images interface {
	SaveImage(ctx context.Context, classroomID, uploaderID uuid.UUID, original string, size int64, r io.Reader) (*uploads.Attachment, error)
	Remove(ctx context.Context, id uuid.UUID) error
}

// SetImages wires the picture storage.
func (s *Service) SetImages(img Images) { s.images = img }

// appearanceOwner checks the owner and, for a lesson, that it is an active lesson of the marathon.
func (s *Service) appearanceOwner(ctx context.Context, p auth.Principal, id uuid.UUID, lessonID *uuid.UUID) error {
	if lessonID != nil {
		return s.marathonLesson(ctx, p, id, *lessonID)
	}
	_, err := s.owned(ctx, p, id)
	return err
}

// UploadImage stores a cover or background picture of the marathon or of a lesson in it. It takes effect at once:
// the previous picture of the slot is deleted after the commit. A custom background is only stored here; it is
// selected and switched on by saving the appearance.
func (s *Service) UploadImage(ctx context.Context, p auth.Principal, id uuid.UUID, lessonID *uuid.UUID, slot, name string, size int64, r io.Reader) (*ImageResult, error) {
	if !knownSlot(slot) {
		return nil, apperr.NotFound("Картинка")
	}
	if err := s.appearanceOwner(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	a, err := s.images.SaveImage(ctx, id, p.UserID, name, size, r)
	if err != nil {
		return nil, err
	}
	old, found, err := s.repo.SetImage(ctx, id, lessonID, slot, &a.ID)
	if err != nil || !found {
		_ = s.images.Remove(ctx, a.ID)
		if err == nil {
			err = apperr.NotFound("Урок")
		}
		return nil, err
	}
	if old != nil {
		_ = s.images.Remove(ctx, *old)
	}
	s.imageChanged(id, lessonID, slot)
	return &ImageResult{ImageID: &a.ID}, nil
}

// RemoveImage deletes the picture of a slot (repeated calls are harmless). Without a cover picture the generated
// cover is shown; a removed custom background that was selected falls back to the plain one.
func (s *Service) RemoveImage(ctx context.Context, p auth.Principal, id uuid.UUID, lessonID *uuid.UUID, slot string) error {
	if !knownSlot(slot) {
		return apperr.NotFound("Картинка")
	}
	if err := s.appearanceOwner(ctx, p, id, lessonID); err != nil {
		return err
	}
	old, found, err := s.repo.SetImage(ctx, id, lessonID, slot, nil)
	if err != nil {
		return err
	}
	if !found {
		return apperr.NotFound("Урок")
	}
	if old != nil {
		_ = s.images.Remove(ctx, *old)
		s.imageChanged(id, lessonID, slot)
	}
	return nil
}

func (s *Service) imageChanged(id uuid.UUID, lessonID *uuid.UUID, slot string) {
	switch {
	case slot == SlotCover && lessonID != nil:
		s.changed(id, "lessons")
	case slot == SlotCover:
		s.changed(id, "settings")
	default:
		s.changed(id, "appearance")
	}
}

// AppearanceSettings returns the «Оформление» tab of the marathon (owner only).
func (s *Service) AppearanceSettings(ctx context.Context, p auth.Principal, id uuid.UUID) (*MarathonAppearance, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	on, err := s.repo.LessonAppearanceOn(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	a, err := s.repo.Appearance(ctx, s.repo.Pool(), id, nil)
	if err != nil {
		return nil, err
	}
	if a == nil {
		d := DefaultAppearance(false)
		a = &d
	}
	return &MarathonAppearance{LessonAppearance: on, Appearance: *a}, nil
}

// LessonAppearance returns the own appearance of a lesson in the marathon (owner only).
func (s *Service) LessonAppearance(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*Appearance, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	a, err := s.repo.Appearance(ctx, s.repo.Pool(), id, &lessonID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		d := DefaultAppearance(true)
		a = &d
	}
	return a, nil
}

func (s *Service) saveAppearance(ctx context.Context, p auth.Principal, id uuid.UUID, lessonID *uuid.UUID, req AppearanceRequest) error {
	a := Appearance{Enabled: lessonID == nil, ReactionsEnabled: req.ReactionsEnabled, Reactions: req.Reactions,
		BackgroundEnabled: req.BackgroundEnabled, Background: req.Background}
	var lessonAppearance *bool
	if lessonID == nil {
		if req.LessonAppearance == nil {
			return apperr.Validation(map[string]string{"lesson_appearance": "Укажите, включено ли оформление урока"})
		}
		lessonAppearance = req.LessonAppearance
	} else {
		if req.Enabled == nil {
			return apperr.Validation(map[string]string{"enabled": "Укажите, есть ли у урока своё оформление"})
		}
		a.Enabled = *req.Enabled
	}
	// Validate before the transaction too, so a wrong request never creates the row.
	if err := checkAppearance(&a, true); err != nil {
		return err
	}
	changed, err := s.repo.SaveAppearance(ctx, id, lessonID, a, lessonAppearance, p.UserID, func(cur *Appearance) error {
		return checkAppearance(&a, cur.BackgroundImageID != nil)
	})
	if database.IsNoRows(err) {
		return apperr.NotFound("Урок")
	}
	if err != nil {
		return err
	}
	if changed {
		s.changed(id, "appearance")
	}
	return nil
}

// SetAppearance saves the «Оформление» tab of the marathon.
func (s *Service) SetAppearance(ctx context.Context, p auth.Principal, id uuid.UUID, req AppearanceRequest) (*MarathonAppearance, error) {
	if _, err := s.owned(ctx, p, id); err != nil {
		return nil, err
	}
	if err := s.saveAppearance(ctx, p, id, nil, req); err != nil {
		return nil, err
	}
	return s.AppearanceSettings(ctx, p, id)
}

// SetLessonAppearance saves the own appearance of a lesson in the marathon.
func (s *Service) SetLessonAppearance(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID, req AppearanceRequest) (*Appearance, error) {
	if err := s.marathonLesson(ctx, p, id, lessonID); err != nil {
		return nil, err
	}
	if err := s.saveAppearance(ctx, p, id, &lessonID, req); err != nil {
		return nil, err
	}
	return s.LessonAppearance(ctx, p, id, lessonID)
}

// EffectiveAppearance is what the lesson page applies: for an enrolled student of the marathon and for its owner
// (lesson preview). It carries no answers or results, so it does not depend on the lesson being open.
func (s *Service) EffectiveAppearance(ctx context.Context, p auth.Principal, id, lessonID uuid.UUID) (*EffectiveAppearance, error) {
	if _, err := s.marathonRole(ctx, p, id); err != nil {
		return nil, err
	}
	q := s.repo.Pool()
	ok, err := s.repo.HasLesson(ctx, q, id, lessonID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.NotFound("Урок")
	}
	on, err := s.repo.LessonAppearanceOn(ctx, q, id)
	if err != nil {
		return nil, err
	}
	m, err := s.repo.Appearance(ctx, q, id, nil)
	if err != nil {
		return nil, err
	}
	var l *Appearance
	if on {
		if l, err = s.repo.Appearance(ctx, q, id, &lessonID); err != nil {
			return nil, err
		}
	}
	out := Resolve(on, m, l)
	return &out, nil
}
