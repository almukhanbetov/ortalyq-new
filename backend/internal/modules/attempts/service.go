package attempts

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/classrooms"
	"ortalyq/backend/internal/modules/exercises"
	"ortalyq/backend/internal/modules/materials"
	"ortalyq/backend/internal/realtime"
)

type Exercises interface {
	ListForLesson(ctx context.Context, lessonID uuid.UUID, scope string, includeHidden bool) ([]exercises.Exercise, error)
	Exercise(ctx context.Context, id uuid.UUID) (*exercises.Exercise, error)
	Versions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*exercises.Version, error)
}

type Classrooms interface {
	MemberRole(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (string, error)
	RequireLesson(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID) (string, error)
	RequireTeacher(ctx context.Context, p auth.Principal, classroomID uuid.UUID) error
	ResolveStudent(ctx context.Context, p auth.Principal, role string, classroomID uuid.UUID, requested *uuid.UUID) (uuid.UUID, error)
	// StudentAccess rejects work in a closed marathon; checked inside the transaction with now() of the DB.
	StudentAccess(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) error
}

// HomeworkState reports a homework status; implemented by the homework repository.
type HomeworkState interface {
	Status(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) (string, error)
}

// LessonReviewState reports whether the lesson answers of a student are locked by the teacher check of a marathon:
// the lesson is sent for review or accepted by the teacher. Implemented by the marathons repository; false for
// regular classes.
type LessonReviewState interface {
	LessonLocked(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) (bool, error)
}

// LessonOwner is satisfied by materials.Service: the teacher preview is allowed only to the lesson owner.
type LessonOwner interface {
	OwnedLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) (*materials.LessonRef, error)
}

type Service struct {
	owner  LessonOwner
	repo   *Repository
	ex     Exercises
	cls    Classrooms
	hw     HomeworkState
	review LessonReviewState
	pub    realtime.Publisher
}

func NewService(repo *Repository, ex Exercises, cls Classrooms, hw HomeworkState, review LessonReviewState, owner LessonOwner, pub realtime.Publisher) *Service {
	return &Service{repo: repo, ex: ex, cls: cls, hw: hw, review: review, owner: owner, pub: pub}
}

// ErrLessonLocked: the marathon lesson is on the teacher's review (or accepted), answers cannot change.
func ErrLessonLocked() error {
	return apperr.Conflict("lesson_review_locked", "Урок на проверке у преподавателя — ответы заблокированы")
}

// scopeLocked tells whether answers of this exercise scope are locked for the student right now:
// homework after it is sent (pending/done), a marathon lesson while it is on review or accepted.
func (s *Service) scopeLocked(ctx context.Context, q database.Querier, scope string, classroomID, lessonID, studentID uuid.UUID) (bool, error) {
	switch scope {
	case exercises.ScopeHomework:
		st, err := s.hw.Status(ctx, q, classroomID, lessonID, studentID)
		return homeworkLocked(st), err
	case exercises.ScopeLesson:
		if s.review == nil {
			return false, nil
		}
		return s.review.LessonLocked(ctx, q, classroomID, lessonID, studentID)
	}
	return false, nil
}

func lockedErr(scope string) error {
	if scope == exercises.ScopeHomework {
		return apperr.Conflict("homework_locked", "Домашняя работа отправлена — ответы заблокированы")
	}
	return ErrLessonLocked()
}

func homeworkLocked(status string) bool { return status == "pending" || status == "done" }

func strp(s string) *string { return &s }

// ---------- view building ----------

type viewer struct {
	teacher bool
}

func (s *Service) buildView(v viewer, ex exercises.Exercise, ver *exercises.Version, att *Attempt, answers map[uuid.UUID]Answer, locked bool, resets int, now time.Time) ExerciseView {
	out := ExerciseView{ExerciseID: ex.ID, Scope: ex.Scope, SectionID: ex.SectionID, Kind: ex.Kind, Position: ex.Position,
		Locked: locked, ResetCount: resets}
	if v.teacher {
		out.Hidden = ex.Hidden
	}
	finished := att != nil && att.Status == StatusFinished
	revealAll := v.teacher || finished
	if revealAll {
		out.Version = ver
	} else {
		out.Version = ver.Sanitized()
	}
	if att == nil {
		return out
	}
	key := ver.CorrectSet()
	av := &AttemptView{ID: att.ID, Status: att.Status, StartedAt: att.StartedAt, DeadlineAt: att.DeadlineAt, FinishedAt: att.FinishedAt,
		FinishReason: att.FinishReason, ServerNow: now, Answers: []AnswerView{}, Revealed: revealAll}
	instant := ver.Settings.CheckMode == exercises.CheckInstant
	for _, q := range ver.Questions {
		a, has := answers[q.ID]
		if !has && !revealAll {
			continue
		}
		view := AnswerView{QuestionID: q.ID, Selected: a.Selected, Text: a.Text, Checked: a.Checked}
		if view.Selected == nil {
			view.Selected = []uuid.UUID{}
		}
		if revealAll || (instant && a.Checked) {
			switch ver.Grade(q, a.Response()) {
			case exercises.Correct:
				view.Result = strp("correct")
			case exercises.Wrong:
				view.Result = strp("wrong")
			default:
				view.Result = strp("unanswered")
			}
			if ver.Kind == exercises.KindGaps {
				view.CorrectAnswers = exercises.CorrectTexts(q)
			} else {
				for _, o := range q.Options {
					if key[q.ID][o.ID] {
						view.CorrectOptionIDs = append(view.CorrectOptionIDs, o.ID)
					}
				}
			}
		}
		av.Answers = append(av.Answers, view)
	}
	if finished {
		sum := Summary{PointsMax: ver.MaxPoints()}
		if att.CorrectCount != nil {
			sum.Correct, sum.Wrong, sum.Unanswered = *att.CorrectCount, *att.WrongCount, *att.UnansweredCount
			sum.PointsEarned, sum.PointsMax = *att.PointsEarned, *att.PointsMax
		}
		if att.FinishedAt != nil {
			sum.ElapsedSeconds = int(att.FinishedAt.Sub(att.StartedAt).Seconds())
		}
		av.Summary = &sum
	}
	out.Attempt = av
	return out
}

// finalize grades and finishes an in-progress attempt (caller holds the row lock).
func (s *Service) finalize(ctx context.Context, q database.Querier, a *Attempt, reason string, at time.Time) error {
	vs, err := s.ex.Versions(ctx, []uuid.UUID{a.VersionID})
	if err != nil {
		return err
	}
	ans, err := s.repo.Answers(ctx, q, []uuid.UUID{a.ID})
	if err != nil {
		return err
	}
	sum := Grade(vs[a.VersionID], ans[a.ID], false)
	return s.repo.Finish(ctx, q, a.ID, reason, at, sum)
}

// expireDue finishes attempts whose server-side deadline has passed.
func (s *Service) expireDue(ctx context.Context, list map[uuid.UUID]*Attempt, now time.Time) (bool, error) {
	changed := false
	for _, a := range list {
		if a.Status != StatusInProgress || a.DeadlineAt == nil || now.Before(*a.DeadlineAt) {
			continue
		}
		err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
			locked, err := s.repo.Get(ctx, tx, a.ID, true)
			if err != nil {
				return err
			}
			if locked.Status != StatusInProgress {
				return nil
			}
			return s.finalize(ctx, tx, locked, ReasonTimeout, *locked.DeadlineAt)
		})
		if err != nil {
			return false, err
		}
		changed = true
		s.publish(a, StatusFinished)
	}
	return changed, nil
}

func (s *Service) publish(a *Attempt, status string) {
	s.pub.Publish(a.ClassroomID, realtime.EventAttemptUpdated, map[string]any{
		"attempt_id": a.ID, "exercise_id": a.ExerciseID, "student_id": a.StudentID, "status": status,
	})
}

func (s *Service) views(ctx context.Context, role string, classroomID, studentID uuid.UUID, list []exercises.Exercise) ([]ExerciseView, error) {
	return s.buildViews(ctx, role, classroomID, studentID, list, true)
}

// buildViews renders exercises for a student. With write = false nothing is stored: attempts past their
// deadline are shown as saved instead of being finalized (used for the teacher's read-only review).
func (s *Service) buildViews(ctx context.Context, role string, classroomID, studentID uuid.UUID, list []exercises.Exercise, write bool) ([]ExerciseView, error) {
	out := []ExerciseView{}
	if len(list) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	now, err := s.repo.Now(ctx)
	if err != nil {
		return nil, err
	}
	atts, err := s.repo.LiveForStudent(ctx, s.repo.Pool(), classroomID, studentID, ids)
	if err != nil {
		return nil, err
	}
	if write { // read-only reviews keep the stored state
		if changed, err := s.expireDue(ctx, atts, now); err != nil {
			return nil, err
		} else if changed {
			if atts, err = s.repo.LiveForStudent(ctx, s.repo.Pool(), classroomID, studentID, ids); err != nil {
				return nil, err
			}
		}
	}
	resets, err := s.repo.ResetCounts(ctx, classroomID, studentID, ids)
	if err != nil {
		return nil, err
	}
	var attIDs, verIDs []uuid.UUID
	for _, a := range atts {
		attIDs = append(attIDs, a.ID)
		verIDs = append(verIDs, a.VersionID)
	}
	answers, err := s.repo.Answers(ctx, s.repo.Pool(), attIDs)
	if err != nil {
		return nil, err
	}
	versions, err := s.ex.Versions(ctx, verIDs)
	if err != nil {
		return nil, err
	}
	type scopeKey struct {
		lesson uuid.UUID
		scope  string
	}
	lockBy := map[scopeKey]bool{}
	v := viewer{teacher: role == classrooms.MemberTeacher}
	for _, e := range list {
		k := scopeKey{e.LessonID, e.Scope}
		locked, ok := lockBy[k]
		if !ok {
			l, err := s.scopeLocked(ctx, s.repo.Pool(), e.Scope, classroomID, e.LessonID, studentID)
			if err != nil {
				return nil, err
			}
			locked, lockBy[k] = l, l
		}
		ver := e.Version
		att := atts[e.ID]
		if att != nil {
			ver = versions[att.VersionID]
		}
		var ans map[uuid.UUID]Answer
		if att != nil {
			ans = answers[att.ID]
		}
		out = append(out, s.buildView(v, e, ver, att, ans, locked, resets[e.ID], now))
	}
	return out, nil
}

// ---------- use cases ----------

// LessonExercises returns exercises of a lesson (or its homework) as seen in a classroom.
func (s *Service) LessonExercises(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, scope string, student *uuid.UUID) ([]ExerciseView, error) {
	role, err := s.cls.RequireLesson(ctx, p, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	sid, err := s.cls.ResolveStudent(ctx, p, role, classroomID, student)
	if err != nil {
		return nil, err
	}
	if scope != exercises.ScopeLesson && scope != exercises.ScopeHomework {
		return nil, apperr.BadRequest("invalid_scope", "Неизвестная область")
	}
	list, err := s.ex.ListForLesson(ctx, lessonID, scope, role == classrooms.MemberTeacher)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, role, classroomID, sid, list)
}

// ReviewLesson returns a student's saved lesson exercises for the teacher's read-only review.
// The caller checks access (marathons: owner only). It never starts, finishes or changes attempts.
func (s *Service) ReviewLesson(ctx context.Context, classroomID, lessonID, studentID uuid.UUID) ([]ExerciseView, error) {
	list, err := s.ex.ListForLesson(ctx, lessonID, exercises.ScopeLesson, true)
	if err != nil {
		return nil, err
	}
	return s.buildViews(ctx, classrooms.MemberTeacher, classroomID, studentID, list, false)
}

// accessibleExercise checks the viewer may see this exercise in this classroom.
func (s *Service) accessibleExercise(ctx context.Context, p auth.Principal, classroomID, exerciseID uuid.UUID) (*exercises.Exercise, string, error) {
	e, err := s.ex.Exercise(ctx, exerciseID)
	if err != nil {
		return nil, "", err
	}
	role, err := s.cls.RequireLesson(ctx, p, classroomID, e.LessonID)
	if err != nil {
		return nil, "", err
	}
	if e.Hidden && role != classrooms.MemberTeacher {
		return nil, "", apperr.NotFound("Упражнение")
	}
	return e, role, nil
}

func (s *Service) single(ctx context.Context, role string, classroomID, studentID uuid.UUID, e *exercises.Exercise) (*ExerciseView, error) {
	vs, err := s.ex.Versions(ctx, []uuid.UUID{e.VersionID})
	if err != nil {
		return nil, err
	}
	e.Version = vs[e.VersionID]
	list, err := s.views(ctx, role, classroomID, studentID, []exercises.Exercise{*e})
	if err != nil {
		return nil, err
	}
	return &list[0], nil
}

func (s *Service) View(ctx context.Context, p auth.Principal, classroomID, exerciseID uuid.UUID, student *uuid.UUID) (*ExerciseView, error) {
	e, role, err := s.accessibleExercise(ctx, p, classroomID, exerciseID)
	if err != nil {
		return nil, err
	}
	sid, err := s.cls.ResolveStudent(ctx, p, role, classroomID, student)
	if err != nil {
		return nil, err
	}
	return s.single(ctx, role, classroomID, sid, e)
}

// Start begins (or returns the existing) attempt. Only students take attempts.
func (s *Service) Start(ctx context.Context, p auth.Principal, classroomID, exerciseID uuid.UUID) (*ExerciseView, error) {
	e, role, err := s.accessibleExercise(ctx, p, classroomID, exerciseID)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.New(403, "teacher_cannot_answer", "Преподаватель не выполняет упражнения за ученика")
	}
	existing, err := s.repo.Live(ctx, s.repo.Pool(), classroomID, exerciseID, p.UserID)
	if err != nil && !database.IsNoRows(err) {
		return nil, err
	}
	if existing == nil {
		locked, err := s.scopeLocked(ctx, s.repo.Pool(), e.Scope, classroomID, e.LessonID, p.UserID)
		if err != nil {
			return nil, err
		}
		if locked {
			return nil, lockedErr(e.Scope)
		}
		vs, err := s.ex.Versions(ctx, []uuid.UUID{e.VersionID})
		if err != nil {
			return nil, err
		}
		v := vs[e.VersionID]
		var timer *int
		if v.Settings.TimerEnabled {
			timer = v.Settings.TimerSeconds
		}
		a, err := s.repo.Start(ctx, classroomID, exerciseID, e.VersionID, p.UserID, timer)
		if err != nil {
			return nil, err
		}
		s.publish(a, a.Status)
	}
	return s.single(ctx, role, classroomID, p.UserID, e)
}

// ownAttempt loads an attempt owned by the student principal.
func (s *Service) ownAttempt(ctx context.Context, q database.Querier, p auth.Principal, id uuid.UUID, lock bool) (*Attempt, error) {
	a, err := s.repo.Get(ctx, q, id, lock)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Попытка")
		}
		return nil, err
	}
	if a.StudentID != p.UserID {
		return nil, apperr.NotFound("Попытка")
	}
	return a, nil
}

// lessonStillOpen rejects work on an attempt whose lesson is no longer open in the class
// (a lesson removed from a marathon). Regular classes never close a lesson, so nothing changes for them.
func (s *Service) lessonStillOpen(ctx context.Context, q database.Querier, a *Attempt) error {
	ok, err := s.repo.LessonOpen(ctx, q, a.ClassroomID, a.ExerciseID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.NotFound("Урок")
	}
	return nil
}

func (s *Service) Answer(ctx context.Context, p auth.Principal, attemptID, questionID uuid.UUID, req AnswerRequest) (*ExerciseView, error) {
	var att *Attempt
	var ex *exercises.Exercise
	var expired bool
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		a, err := s.ownAttempt(ctx, tx, p, attemptID, true)
		if err != nil {
			return err
		}
		att = a
		if _, err := s.cls.MemberRole(ctx, p, a.ClassroomID); err != nil {
			return err
		}
		// A closed marathon is reported first, whatever the attempt state.
		lessonID, err := s.repo.ExerciseLesson(ctx, tx, a.ExerciseID)
		if err != nil {
			return err
		}
		if err := s.cls.StudentAccess(ctx, tx, a.ClassroomID, lessonID, a.StudentID); err != nil {
			return err
		}
		if err := s.lessonStillOpen(ctx, tx, a); err != nil {
			return err
		}
		if ex, err = s.ex.Exercise(ctx, a.ExerciseID); err != nil {
			return err
		}
		// A sent homework or a marathon lesson on review is reported before the attempt state: the student
		// sees why the answers cannot change.
		locked, err := s.scopeLocked(ctx, tx, ex.Scope, a.ClassroomID, ex.LessonID, a.StudentID)
		if err != nil {
			return err
		}
		if locked {
			return lockedErr(ex.Scope)
		}
		if a.Status != StatusInProgress {
			return apperr.Conflict("attempt_finished", "Попытка уже завершена")
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		if a.DeadlineAt != nil && !now.Before(*a.DeadlineAt) {
			expired = true
			return s.finalize(ctx, tx, a, ReasonTimeout, *a.DeadlineAt)
		}
		vs, err := s.ex.Versions(ctx, []uuid.UUID{a.VersionID})
		if err != nil {
			return err
		}
		sel, err := selection(vs[a.VersionID], questionID, req)
		if err != nil {
			return err
		}
		ans, err := s.repo.Answers(ctx, tx, []uuid.UUID{a.ID})
		if err != nil {
			return err
		}
		if prev, ok := ans[a.ID][questionID]; ok && prev.Checked {
			if sameResponse(prev.Response(), sel) {
				return nil // idempotent retry of an already checked answer
			}
			return apperr.Conflict("answer_locked", "Ответ уже проверен и зафиксирован")
		}
		return s.repo.UpsertAnswer(ctx, tx, a.ID, questionID, sel, req.Check)
	})
	if err != nil {
		return nil, err
	}
	if expired {
		s.publish(att, StatusFinished)
		return nil, apperr.Conflict("attempt_expired", "Время вышло — попытка завершена")
	}
	s.publish(att, att.Status)
	return s.View(ctx, p, att.ClassroomID, att.ExerciseID, nil)
}

// selection validates an answer against the version: the question exists; for a test the options belong to it and
// a single-choice question has one option; for a gap exercise only a typed word (spaces around it are dropped);
// "check" only in the instant mode and with something given. Returns the answer without repeated options.
// Shared by student attempts and the teacher preview.
func selection(v *exercises.Version, questionID uuid.UUID, req AnswerRequest) (exercises.Response, error) {
	var none exercises.Response
	q, ok := v.Question(questionID)
	if !ok {
		return none, apperr.NotFound("Вопрос")
	}
	instant := v.Settings.CheckMode == exercises.CheckInstant
	if req.Check && !instant {
		return none, apperr.BadRequest("check_not_allowed", "Проверка выполняется после завершения")
	}
	if v.Kind == exercises.KindGaps {
		if len(req.OptionIDs) > 0 {
			return none, apperr.Validation(map[string]string{"option_ids": "В задании с пропуском ответ вписывается словом"})
		}
		text := strings.TrimSpace(req.Text)
		if utf8.RuneCountInString(text) > 100 {
			return none, apperr.Validation(map[string]string{"text": "Ответ не длиннее 100 символов"})
		}
		if req.Check && text == "" {
			return none, apperr.Validation(map[string]string{"text": "Впишите слово"})
		}
		return exercises.Response{Selected: []uuid.UUID{}, Text: text}, nil
	}
	valid := map[uuid.UUID]bool{}
	for _, o := range q.Options {
		valid[o.ID] = true
	}
	sel := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for _, id := range req.OptionIDs {
		if !valid[id] {
			return none, apperr.Validation(map[string]string{"option_ids": "Неизвестный вариант ответа"})
		}
		if !seen[id] {
			seen[id] = true
			sel = append(sel, id)
		}
	}
	if !q.Multiple && len(sel) > 1 {
		return none, apperr.Validation(map[string]string{"option_ids": "Можно выбрать только один вариант"})
	}
	if req.Check && len(sel) == 0 {
		return none, apperr.Validation(map[string]string{"option_ids": "Выберите ответ"})
	}
	return exercises.Response{Selected: sel}, nil
}

// sameResponse tells whether a repeated answer is the same as the stored one (idempotent retry).
func sameResponse(a, b exercises.Response) bool {
	return sameSet(a.Selected, b.Selected) && a.Text == b.Text
}

func sameSet(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[uuid.UUID]bool{}
	for _, x := range a {
		m[x] = true
	}
	for _, x := range b {
		if !m[x] {
			return false
		}
	}
	return true
}

// Finish completes an attempt. Repeated calls return the same result.
func (s *Service) Finish(ctx context.Context, p auth.Principal, attemptID uuid.UUID) (*ExerciseView, error) {
	var att *Attempt
	changed := false
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		a, err := s.ownAttempt(ctx, tx, p, attemptID, true)
		if err != nil {
			return err
		}
		att = a
		if a.Status != StatusInProgress {
			return nil
		}
		lessonID, err := s.repo.ExerciseLesson(ctx, tx, a.ExerciseID)
		if err != nil {
			return err
		}
		if err := s.cls.StudentAccess(ctx, tx, a.ClassroomID, lessonID, a.StudentID); err != nil {
			return err
		}
		if err := s.lessonStillOpen(ctx, tx, a); err != nil {
			return err
		}
		ex, err := s.ex.Exercise(ctx, a.ExerciseID)
		if err != nil {
			return err
		}
		// Sending a lesson for review finishes its attempts; one left open (started at the same moment) must
		// not change the result while the teacher reviews it.
		if ex.Scope == exercises.ScopeLesson {
			locked, err := s.scopeLocked(ctx, tx, ex.Scope, a.ClassroomID, lessonID, a.StudentID)
			if err != nil {
				return err
			}
			if locked {
				return ErrLessonLocked()
			}
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		reason, at := ReasonManual, now
		if a.DeadlineAt != nil && !now.Before(*a.DeadlineAt) {
			reason, at = ReasonTimeout, *a.DeadlineAt
		}
		changed = true
		return s.finalize(ctx, tx, a, reason, at)
	})
	if err != nil {
		return nil, err
	}
	if att.Status == StatusReset {
		return nil, apperr.Conflict("attempt_reset", "Попытка была сброшена преподавателем")
	}
	if changed {
		s.publish(att, StatusFinished)
	}
	return s.View(ctx, p, att.ClassroomID, att.ExerciseID, nil)
}

// Reset archives a student's attempt (teacher only). History stays for audit.
func (s *Service) Reset(ctx context.Context, p auth.Principal, classroomID, exerciseID, studentID uuid.UUID) (*ExerciseView, error) {
	e, role, err := s.accessibleExercise(ctx, p, classroomID, exerciseID)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberTeacher {
		return nil, apperr.Forbidden()
	}
	sid, err := s.cls.ResolveStudent(ctx, p, role, classroomID, &studentID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.Reset(ctx, classroomID, exerciseID, sid, p.UserID); err != nil {
		return nil, err
	}
	s.pub.Publish(classroomID, realtime.EventAttemptUpdated, map[string]any{"exercise_id": exerciseID, "student_id": sid, "status": StatusReset})
	return s.single(ctx, role, classroomID, sid, e)
}

// ---------- review integration: homework and marathon lessons (run inside the caller's transaction) ----------

func (s *Service) scopeAttempts(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID, scope string) ([]exercises.Exercise, map[uuid.UUID]*Attempt, error) {
	list, err := s.ex.ListForLesson(ctx, lessonID, scope, false)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]uuid.UUID, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	atts, err := s.repo.LiveForStudent(ctx, q, classroomID, studentID, ids)
	return list, atts, err
}

// FinalizeHomework finishes all in-progress homework attempts and returns the snapshot.
func (s *Service) FinalizeHomework(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID) ([]SnapshotExercise, error) {
	return s.FinalizeScope(ctx, tx, classroomID, lessonID, studentID, exercises.ScopeHomework)
}

// FinalizeScope finishes all in-progress attempts of the visible exercises of a lesson scope (homework, or the
// lesson itself for the marathon teacher check) and returns the snapshot of answers. Points are recomputed
// from the answers, so finalizing again never changes them.
func (s *Service) FinalizeScope(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, scope string) ([]SnapshotExercise, error) {
	list, atts, err := s.scopeAttempts(ctx, tx, classroomID, lessonID, studentID, scope)
	if err != nil {
		return nil, err
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		return nil, err
	}
	for _, a := range atts {
		locked, err := s.repo.Get(ctx, tx, a.ID, true)
		if err != nil {
			return nil, err
		}
		if locked.Status == StatusInProgress {
			reason, at := ReasonHomeworkSubmit, now
			if locked.DeadlineAt != nil && now.After(*locked.DeadlineAt) {
				reason, at = ReasonTimeout, *locked.DeadlineAt
			}
			if err := s.finalize(ctx, tx, locked, reason, at); err != nil {
				return nil, err
			}
		}
	}
	return s.snapshot(ctx, tx, classroomID, studentID, list)
}

func (s *Service) snapshot(ctx context.Context, tx pgx.Tx, classroomID, studentID uuid.UUID, list []exercises.Exercise) ([]SnapshotExercise, error) {
	ids := make([]uuid.UUID, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	atts, err := s.repo.LiveForStudent(ctx, tx, classroomID, studentID, ids)
	if err != nil {
		return nil, err
	}
	var attIDs []uuid.UUID
	verIDs := []uuid.UUID{}
	for _, e := range list {
		verIDs = append(verIDs, e.VersionID)
		if a := atts[e.ID]; a != nil {
			attIDs = append(attIDs, a.ID)
			verIDs = append(verIDs, a.VersionID)
		}
	}
	answers, err := s.repo.Answers(ctx, tx, attIDs)
	if err != nil {
		return nil, err
	}
	versions, err := s.ex.Versions(ctx, verIDs)
	if err != nil {
		return nil, err
	}
	out := []SnapshotExercise{}
	for _, e := range list {
		a := atts[e.ID]
		ver := versions[e.VersionID]
		var ans map[uuid.UUID]Answer
		se := SnapshotExercise{ExerciseID: e.ID, VersionID: e.VersionID, Title: ver.Title, Answers: []AnswerView{}}
		if a != nil {
			ver = versions[a.VersionID]
			ans = answers[a.ID]
			id := a.ID
			se.AttemptID, se.VersionID, se.Title = &id, a.VersionID, ver.Title
		}
		se.Version = ver
		for _, q := range ver.Questions {
			x := ans[q.ID]
			view := AnswerView{QuestionID: q.ID, Selected: x.Selected, Text: x.Text, Checked: x.Checked}
			if view.Selected == nil {
				view.Selected = []uuid.UUID{}
			}
			if ver.Kind == exercises.KindGaps {
				view.CorrectAnswers = exercises.CorrectTexts(q)
			}
			switch ver.Grade(q, x.Response()) {
			case exercises.Correct:
				view.Result = strp("correct")
			case exercises.Wrong:
				view.Result = strp("wrong")
			default:
				view.Result = strp("unanswered")
			}
			se.Answers = append(se.Answers, view)
		}
		se.Summary = Grade(ver, ans, false)
		if a != nil && a.FinishedAt != nil {
			se.Summary.ElapsedSeconds = int(a.FinishedAt.Sub(a.StartedAt).Seconds())
		}
		out = append(out, se)
	}
	return out, nil
}

// ReopenHomework unlocks homework attempts for revision. Instant-mode answers that
// were checked wrong become editable again; correct ones stay fixed.
func (s *Service) ReopenHomework(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID) error {
	return s.ReopenScope(ctx, tx, classroomID, lessonID, studentID, exercises.ScopeHomework)
}

// ReopenScope unlocks the attempts of a lesson scope for revision (homework, or a marathon lesson returned by the
// teacher), with the same rule as homework: wrong instantly-checked answers become editable, correct ones stay.
func (s *Service) ReopenScope(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID, scope string) error {
	_, atts, err := s.scopeAttempts(ctx, tx, classroomID, lessonID, studentID, scope)
	if err != nil {
		return err
	}
	var attIDs, verIDs []uuid.UUID
	for _, a := range atts {
		attIDs = append(attIDs, a.ID)
		verIDs = append(verIDs, a.VersionID)
	}
	answers, err := s.repo.Answers(ctx, tx, attIDs)
	if err != nil {
		return err
	}
	versions, err := s.ex.Versions(ctx, verIDs)
	if err != nil {
		return err
	}
	for _, a := range atts {
		if err := s.repo.Reopen(ctx, tx, a.ID); err != nil {
			return err
		}
		ver := versions[a.VersionID]
		var unlock []uuid.UUID
		for qid, ans := range answers[a.ID] {
			q, ok := ver.Question(qid)
			if ans.Checked && (!ok || ver.Grade(q, ans.Response()) != exercises.Correct) {
				unlock = append(unlock, qid)
			}
		}
		if err := s.repo.UncheckAnswers(ctx, tx, a.ID, unlock); err != nil {
			return err
		}
	}
	return nil
}

// ---------- points for grading ----------

// Points sums earned/max points for a student in a lesson scope, recomputed from answers.
func (s *Service) Points(ctx context.Context, classroomID, lessonID, studentID uuid.UUID, scope string) (*Points, error) {
	all, err := s.PointsFor(ctx, classroomID, []uuid.UUID{lessonID}, []uuid.UUID{studentID}, scope)
	if err != nil {
		return nil, err
	}
	return all[studentID][lessonID], nil
}

// PointsFor recomputes points of several students in several lessons of one classroom at once (marathon
// statistics and rating). The rule is the one of Points (which calls it): a finished attempt counts all its
// answers, an unfinished instant one only checked answers, an unfinished "after" one nothing; hidden exercises do
// not count. Every pair (student, lesson) is present in the result, also without attempts.
func (s *Service) PointsFor(ctx context.Context, classroomID uuid.UUID, lessonIDs, studentIDs []uuid.UUID, scope string) (map[uuid.UUID]map[uuid.UUID]*Points, error) {
	out := map[uuid.UUID]map[uuid.UUID]*Points{}
	for _, sid := range studentIDs {
		out[sid] = map[uuid.UUID]*Points{}
		for _, lid := range lessonIDs {
			out[sid][lid] = &Points{Categories: []CategoryPoints{}}
		}
	}
	var list []exercises.Exercise
	for _, lid := range lessonIDs {
		l, err := s.ex.ListForLesson(ctx, lid, scope, false)
		if err != nil {
			return nil, err
		}
		list = append(list, l...)
	}
	if len(list) == 0 || len(studentIDs) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	atts, err := s.repo.LiveForStudents(ctx, classroomID, studentIDs, ids)
	if err != nil {
		return nil, err
	}
	var attIDs, verIDs []uuid.UUID
	for _, byEx := range atts {
		for _, a := range byEx {
			attIDs = append(attIDs, a.ID)
			verIDs = append(verIDs, a.VersionID)
		}
	}
	answers, err := s.repo.Answers(ctx, s.repo.Pool(), attIDs)
	if err != nil {
		return nil, err
	}
	versions, err := s.ex.Versions(ctx, verIDs)
	if err != nil {
		return nil, err
	}
	for _, sid := range studentIDs {
		for _, lid := range lessonIDs {
			pts := out[sid][lid]
			byCat := map[string]*CategoryPoints{}
			var order []string
			for _, e := range list {
				if e.LessonID != lid {
					continue
				}
				ver := e.Version
				a := atts[sid][e.ID]
				if a != nil {
					ver = versions[a.VersionID]
				}
				pts.Exercises++
				earned := 0
				if a != nil {
					switch {
					case a.Status == StatusFinished:
						pts.Finished++
						earned = Grade(ver, answers[a.ID], false).PointsEarned
					case ver.Settings.CheckMode == exercises.CheckInstant:
						earned = Grade(ver, answers[a.ID], true).PointsEarned
					}
				}
				if !ver.Settings.ScoringEnabled {
					continue
				}
				key := ""
				if ver.Settings.CategoryID != nil {
					key = ver.Settings.CategoryID.String()
				}
				c, ok := byCat[key]
				if !ok {
					c = &CategoryPoints{CategoryID: ver.Settings.CategoryID, CategoryName: ver.Settings.CategoryName}
					byCat[key] = c
					order = append(order, key)
				}
				c.Earned += earned
				c.Max += ver.MaxPoints()
				pts.Earned += earned
				pts.Max += ver.MaxPoints()
			}
			for _, k := range order {
				pts.Categories = append(pts.Categories, *byCat[k])
			}
		}
	}
	return out, nil
}

// ---------- teacher preview (stage 8) ----------
//
// «Режим предпросмотра»: the owner of a lesson takes its tests as a student would (the same view, timer, instant
// check, results), but the attempts live in preview_attempts / preview_answers. Nothing here touches attempts,
// classrooms, statistics, ratings, reviews or progress, and no realtime event is sent: students never see it.

// previewExercise returns a visible exercise of a lesson owned by the caller; anything else is 404.
func (s *Service) previewExercise(ctx context.Context, p auth.Principal, exerciseID uuid.UUID) (*exercises.Exercise, error) {
	e, err := s.ex.Exercise(ctx, exerciseID)
	if err != nil {
		return nil, err
	}
	if _, err := s.owner.OwnedLesson(ctx, p, e.LessonID); err != nil {
		return nil, err
	}
	if e.Hidden {
		return nil, apperr.NotFound("Упражнение") // the preview shows what a student sees
	}
	return e, nil
}

// previewFinalize grades and finishes a preview attempt (the caller holds the row lock).
func (s *Service) previewFinalize(ctx context.Context, q database.Querier, a *Attempt, reason string, at time.Time) error {
	vs, err := s.ex.Versions(ctx, []uuid.UUID{a.VersionID})
	if err != nil {
		return err
	}
	ans, err := s.repo.PreviewAnswers(ctx, q, []uuid.UUID{a.ID})
	if err != nil {
		return err
	}
	return s.repo.PreviewFinish(ctx, q, a.ID, reason, at, Grade(vs[a.VersionID], ans[a.ID], false))
}

// previewViews renders exercises for the teacher's preview exactly as for a student (key hidden until revealed);
// preview attempts past their deadline are finished first, with the deadline as the finish time.
func (s *Service) previewViews(ctx context.Context, teacherID uuid.UUID, list []exercises.Exercise) ([]ExerciseView, error) {
	out := []ExerciseView{}
	if len(list) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	now, err := s.repo.Now(ctx)
	if err != nil {
		return nil, err
	}
	atts, err := s.repo.PreviewLive(ctx, teacherID, ids)
	if err != nil {
		return nil, err
	}
	expired := false
	for _, a := range atts {
		if a.Status != StatusInProgress || a.DeadlineAt == nil || now.Before(*a.DeadlineAt) {
			continue
		}
		if err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
			locked, err := s.repo.PreviewGet(ctx, tx, a.ID, true)
			if err != nil || locked.Status != StatusInProgress {
				return err
			}
			return s.previewFinalize(ctx, tx, locked, ReasonTimeout, *locked.DeadlineAt)
		}); err != nil {
			return nil, err
		}
		expired = true
	}
	if expired {
		if atts, err = s.repo.PreviewLive(ctx, teacherID, ids); err != nil {
			return nil, err
		}
	}
	resets, err := s.repo.PreviewResetCounts(ctx, teacherID, ids)
	if err != nil {
		return nil, err
	}
	var attIDs, verIDs []uuid.UUID
	for _, a := range atts {
		attIDs = append(attIDs, a.ID)
		verIDs = append(verIDs, a.VersionID)
	}
	answers, err := s.repo.PreviewAnswers(ctx, s.repo.Pool(), attIDs)
	if err != nil {
		return nil, err
	}
	versions, err := s.ex.Versions(ctx, verIDs)
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		ver := e.Version
		att := atts[e.ID]
		var ans map[uuid.UUID]Answer
		if att != nil {
			ver = versions[att.VersionID]
			ans = answers[att.ID]
		}
		out = append(out, s.buildView(viewer{teacher: false}, e, ver, att, ans, false, resets[e.ID], now))
	}
	return out, nil
}

func (s *Service) previewSingle(ctx context.Context, teacherID uuid.UUID, e *exercises.Exercise) (*ExerciseView, error) {
	vs, err := s.ex.Versions(ctx, []uuid.UUID{e.VersionID})
	if err != nil {
		return nil, err
	}
	e.Version = vs[e.VersionID]
	list, err := s.previewViews(ctx, teacherID, []exercises.Exercise{*e})
	if err != nil {
		return nil, err
	}
	return &list[0], nil
}

// PreviewLesson lists the visible lesson exercises of an owned lesson with the teacher's own preview attempts.
func (s *Service) PreviewLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) ([]ExerciseView, error) {
	if _, err := s.owner.OwnedLesson(ctx, p, lessonID); err != nil {
		return nil, err
	}
	list, err := s.ex.ListForLesson(ctx, lessonID, exercises.ScopeLesson, false)
	if err != nil {
		return nil, err
	}
	return s.previewViews(ctx, p.UserID, list)
}

// PreviewView returns one exercise in the preview.
func (s *Service) PreviewView(ctx context.Context, p auth.Principal, exerciseID uuid.UUID) (*ExerciseView, error) {
	e, err := s.previewExercise(ctx, p, exerciseID)
	if err != nil {
		return nil, err
	}
	return s.previewSingle(ctx, p.UserID, e)
}

// PreviewStart begins (or returns the live) preview attempt on the current version, with the timer if set.
func (s *Service) PreviewStart(ctx context.Context, p auth.Principal, exerciseID uuid.UUID) (*ExerciseView, error) {
	e, err := s.previewExercise(ctx, p, exerciseID)
	if err != nil {
		return nil, err
	}
	vs, err := s.ex.Versions(ctx, []uuid.UUID{e.VersionID})
	if err != nil {
		return nil, err
	}
	var timer *int
	if v := vs[e.VersionID]; v.Settings.TimerEnabled {
		timer = v.Settings.TimerSeconds
	}
	if _, err := s.repo.PreviewStart(ctx, e.ID, e.VersionID, p.UserID, timer); err != nil {
		return nil, err
	}
	return s.previewSingle(ctx, p.UserID, e)
}

// ownPreview loads a preview attempt of the caller; another teacher's attempt is 404.
func (s *Service) ownPreview(ctx context.Context, q database.Querier, p auth.Principal, id uuid.UUID, lock bool) (*Attempt, error) {
	a, err := s.repo.PreviewGet(ctx, q, id, lock)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Попытка")
		}
		return nil, err
	}
	if a.StudentID != p.UserID {
		return nil, apperr.NotFound("Попытка")
	}
	return a, nil
}

// PreviewAnswer saves an answer of the preview with the student rules (checked answers are fixed, the deadline is
// enforced by now() of the DB).
func (s *Service) PreviewAnswer(ctx context.Context, p auth.Principal, attemptID, questionID uuid.UUID, req AnswerRequest) (*ExerciseView, error) {
	var att *Attempt
	expired := false
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		a, err := s.ownPreview(ctx, tx, p, attemptID, true)
		if err != nil {
			return err
		}
		att = a
		if _, err := s.previewExercise(ctx, p, a.ExerciseID); err != nil {
			return err
		}
		if a.Status != StatusInProgress {
			return apperr.Conflict("attempt_finished", "Попытка уже завершена")
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		if a.DeadlineAt != nil && !now.Before(*a.DeadlineAt) {
			expired = true
			return s.previewFinalize(ctx, tx, a, ReasonTimeout, *a.DeadlineAt)
		}
		vs, err := s.ex.Versions(ctx, []uuid.UUID{a.VersionID})
		if err != nil {
			return err
		}
		sel, err := selection(vs[a.VersionID], questionID, req)
		if err != nil {
			return err
		}
		ans, err := s.repo.PreviewAnswers(ctx, tx, []uuid.UUID{a.ID})
		if err != nil {
			return err
		}
		if prev, ok := ans[a.ID][questionID]; ok && prev.Checked {
			if sameResponse(prev.Response(), sel) {
				return nil
			}
			return apperr.Conflict("answer_locked", "Ответ уже проверен и зафиксирован")
		}
		return s.repo.PreviewUpsertAnswer(ctx, tx, a.ID, questionID, sel, req.Check)
	})
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, apperr.Conflict("attempt_expired", "Время вышло — попытка завершена")
	}
	return s.PreviewView(ctx, p, att.ExerciseID)
}

// PreviewFinish finishes a preview attempt; repeated calls return the same result.
func (s *Service) PreviewFinish(ctx context.Context, p auth.Principal, attemptID uuid.UUID) (*ExerciseView, error) {
	var att *Attempt
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		a, err := s.ownPreview(ctx, tx, p, attemptID, true)
		if err != nil {
			return err
		}
		att = a
		if a.Status != StatusInProgress {
			return nil
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		reason, at := ReasonManual, now
		if a.DeadlineAt != nil && !now.Before(*a.DeadlineAt) {
			reason, at = ReasonTimeout, *a.DeadlineAt
		}
		return s.previewFinalize(ctx, tx, a, reason, at)
	})
	if err != nil {
		return nil, err
	}
	if att.Status == StatusReset {
		return nil, apperr.Conflict("attempt_reset", "Попытка предпросмотра сброшена")
	}
	return s.PreviewView(ctx, p, att.ExerciseID)
}

// PreviewResetAttempt archives one preview attempt (the exercise can be taken again). Repeat-safe.
func (s *Service) PreviewResetAttempt(ctx context.Context, p auth.Principal, attemptID uuid.UUID) (*ExerciseView, error) {
	a, err := s.ownPreview(ctx, s.repo.Pool(), p, attemptID, false)
	if err != nil {
		return nil, err
	}
	e, err := s.previewExercise(ctx, p, a.ExerciseID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.PreviewReset(ctx, p.UserID, []uuid.UUID{e.ID}); err != nil {
		return nil, err
	}
	return s.previewSingle(ctx, p.UserID, e)
}

// PreviewResetLesson is «Сбросить предпросмотр»: every live preview attempt of the caller in the lesson is archived.
func (s *Service) PreviewResetLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) ([]ExerciseView, error) {
	if _, err := s.owner.OwnedLesson(ctx, p, lessonID); err != nil {
		return nil, err
	}
	list, err := s.ex.ListForLesson(ctx, lessonID, exercises.ScopeLesson, true)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	if len(ids) > 0 {
		if _, err := s.repo.PreviewReset(ctx, p.UserID, ids); err != nil {
			return nil, err
		}
	}
	return s.PreviewLesson(ctx, p, lessonID)
}
