package exercises

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/materials"
)

type Lessons interface {
	OwnedLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) (*materials.LessonRef, error)
	SectionLesson(ctx context.Context, sectionID uuid.UUID) (uuid.UUID, error)
}

type Notifier interface {
	LessonChanged(ctx context.Context, lessonID uuid.UUID)
}

type Service struct {
	repo    *Repository
	lessons Lessons
	notify  Notifier
}

func NewService(repo *Repository, lessons Lessons, notify Notifier) *Service {
	return &Service{repo: repo, lessons: lessons, notify: notify}
}

func (s *Service) Catalog() []CatalogEntry { return Catalog }

func (s *Service) validate(ctx context.Context, p auth.Principal, kind string, req *SaveRequest) error {
	h, ok := handlerFor(kind)
	if !ok {
		return apperr.Validation(map[string]string{"kind": "Этот тип упражнения пока недоступен"})
	}
	errs := h.Validate(*req)
	v := apperr.V(errs)
	req.Title = strings.TrimSpace(req.Title)
	v.Check(req.Title != "" && len([]rune(req.Title)) <= 200, "title", "Введите заголовок")
	st := &req.Settings
	if st.CheckMode == "" {
		st.CheckMode = CheckInstant
	}
	v.Check(st.CheckMode == CheckInstant || st.CheckMode == CheckAfter, "settings.check_mode", "Неизвестный режим проверки")
	if st.TimerEnabled {
		v.Check(st.TimerSeconds != nil && *st.TimerSeconds >= 10 && *st.TimerSeconds <= 36000, "settings.timer_seconds", "Длительность таймера от 10 секунд до 10 часов")
	} else {
		st.TimerSeconds = nil
	}
	if st.ScoringEnabled {
		v.Check(st.PointsPerCorrect >= 1 && st.PointsPerCorrect <= 100, "settings.points_per_correct", "Баллы за правильный ответ: от 1 до 100")
		if st.CategoryID == nil {
			def := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
			st.CategoryID = &def
		}
		ok, err := s.repo.CategoryVisible(ctx, *st.CategoryID, p.UserID)
		if err != nil {
			return err
		}
		v.Check(ok, "settings.category_id", "Категория не найдена")
	} else {
		if st.PointsPerCorrect < 0 || st.PointsPerCorrect > 100 {
			st.PointsPerCorrect = 1
		}
		if st.PointsPerCorrect == 0 {
			st.PointsPerCorrect = 1
		}
		if st.CategoryID != nil {
			if ok, err := s.repo.CategoryVisible(ctx, *st.CategoryID, p.UserID); err != nil || !ok {
				st.CategoryID = nil
			}
		}
	}
	return v.Err()
}

// owned loads an exercise owned (via its lesson) by the teacher.
func (s *Service) owned(ctx context.Context, p auth.Principal, id uuid.UUID) (*Exercise, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Упражнение")
		}
		return nil, err
	}
	if _, err := s.lessons.OwnedLesson(ctx, p, e.LessonID); err != nil {
		return nil, apperr.NotFound("Упражнение")
	}
	return e, nil
}

func (s *Service) withVersions(ctx context.Context, list []Exercise) error {
	ids := make([]uuid.UUID, 0, len(list))
	for _, e := range list {
		ids = append(ids, e.VersionID)
	}
	vs, err := s.repo.LoadVersions(ctx, ids)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Version = vs[list[i].VersionID]
	}
	return nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, lessonID uuid.UUID, req CreateRequest) (*Exercise, error) {
	if _, err := s.lessons.OwnedLesson(ctx, p, lessonID); err != nil {
		return nil, err
	}
	v := apperr.V{}
	switch req.Scope {
	case ScopeLesson:
		if req.SectionID == nil {
			v.Add("section_id", "Выберите раздел")
		} else if lid, err := s.lessons.SectionLesson(ctx, *req.SectionID); err != nil || lid != lessonID {
			v.Add("section_id", "Раздел не принадлежит уроку")
		}
	case ScopeHomework:
		req.SectionID = nil
	default:
		v.Add("scope", "Укажите, куда добавить упражнение")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if err := s.validate(ctx, p, req.Kind, &req.SaveRequest); err != nil {
		return nil, err
	}
	id, err := s.repo.Create(ctx, lessonID, req.Scope, req.SectionID, req.Kind, p.UserID, nil, req.SaveRequest)
	if err != nil {
		return nil, err
	}
	s.notify.LessonChanged(ctx, lessonID)
	return s.Get(ctx, p, id)
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (*Exercise, error) {
	e, err := s.owned(ctx, p, id)
	if err != nil {
		return nil, err
	}
	list := []Exercise{*e}
	if err := s.withVersions(ctx, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

// Update stores a new version; existing attempts keep pointing at their own version.
func (s *Service) Update(ctx context.Context, p auth.Principal, id uuid.UUID, req SaveRequest) (*Exercise, error) {
	e, err := s.owned(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if err := s.validate(ctx, p, e.Kind, &req); err != nil {
		return nil, err
	}
	if err := s.repo.SaveVersion(ctx, id, p.UserID, req); err != nil {
		return nil, err
	}
	s.notify.LessonChanged(ctx, e.LessonID)
	return s.Get(ctx, p, id)
}

func (s *Service) SetHidden(ctx context.Context, p auth.Principal, id uuid.UUID, hidden bool) error {
	e, err := s.owned(ctx, p, id)
	if err != nil {
		return err
	}
	if err := s.repo.SetHidden(ctx, id, hidden); err != nil {
		return err
	}
	s.notify.LessonChanged(ctx, e.LessonID)
	return nil
}

// Delete hides the exercise from the template; attempts and results stay for audit.
func (s *Service) Delete(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	e, err := s.owned(ctx, p, id)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	s.notify.LessonChanged(ctx, e.LessonID)
	return nil
}

func (s *Service) Reorder(ctx context.Context, p auth.Principal, lessonID uuid.UUID, req ReorderRequest) error {
	if _, err := s.lessons.OwnedLesson(ctx, p, lessonID); err != nil {
		return err
	}
	if req.Scope == ScopeHomework {
		req.SectionID = nil
	}
	ok, err := s.repo.Reorder(ctx, lessonID, req.Scope, req.SectionID, req.IDs)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Validation(map[string]string{"ids": "Список должен содержать все упражнения ровно один раз"})
	}
	s.notify.LessonChanged(ctx, lessonID)
	return nil
}

// CopyToHomework duplicates the current version into the lesson's homework as a new,
// independent exercise (own attempts and results).
func (s *Service) CopyToHomework(ctx context.Context, p auth.Principal, id uuid.UUID) (*Exercise, error) {
	src, err := s.Get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	v := src.Version
	req := SaveRequest{Title: v.Title, Settings: v.Settings, Hidden: &src.Hidden}
	for _, q := range v.Questions {
		qi := QuestionInput{Prompt: q.Prompt, Multiple: q.Multiple}
		for _, o := range q.Options {
			qi.Options = append(qi.Options, OptionInput{Label: o.Label, IsCorrect: o.IsCorrect != nil && *o.IsCorrect})
		}
		req.Questions = append(req.Questions, qi)
	}
	newID, err := s.repo.Create(ctx, src.LessonID, ScopeHomework, nil, src.Kind, p.UserID, &src.ID, req)
	if err != nil {
		return nil, err
	}
	s.notify.LessonChanged(ctx, src.LessonID)
	return s.Get(ctx, p, newID)
}

// ListForEditor returns all exercises (incl. hidden) with keys for the lesson owner.
func (s *Service) ListForEditor(ctx context.Context, p auth.Principal, lessonID uuid.UUID, scope string) ([]Exercise, error) {
	if _, err := s.lessons.OwnedLesson(ctx, p, lessonID); err != nil {
		return nil, err
	}
	return s.ListForLesson(ctx, lessonID, scope, true)
}

// ListForLesson is used by other modules after they have checked access.
// Versions include the key; callers must sanitize for students.
func (s *Service) ListForLesson(ctx context.Context, lessonID uuid.UUID, scope string, includeHidden bool) ([]Exercise, error) {
	list, err := s.repo.ListForLesson(ctx, lessonID, scope, includeHidden)
	if err != nil {
		return nil, err
	}
	return list, s.withVersions(ctx, list)
}

// Exercise returns an exercise by id (no access check; callers check).
func (s *Service) Exercise(ctx context.Context, id uuid.UUID) (*Exercise, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Упражнение")
		}
		return nil, err
	}
	return e, nil
}

func (s *Service) Versions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Version, error) {
	return s.repo.LoadVersions(ctx, ids)
}

func (s *Service) Stats(ctx context.Context, p auth.Principal, id uuid.UUID) (*Stats, error) {
	e, err := s.Get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	attempts, finished, answers, err := s.repo.QuestionStats(ctx, e.VersionID)
	if err != nil {
		return nil, err
	}
	st := &Stats{VersionNumber: e.Version.Number, Attempts: attempts, Finished: finished, Questions: []QuestionStat{}}
	for _, q := range e.Version.Questions {
		qs := QuestionStat{QuestionID: q.ID, Prompt: q.Prompt}
		for _, resp := range answers[q.ID] {
			switch e.Version.Grade(q, resp) {
			case Correct:
				qs.Correct++
			case Wrong:
				qs.Wrong++
			}
		}
		qs.Unanswered = attempts - qs.Correct - qs.Wrong
		if qs.Unanswered < 0 {
			qs.Unanswered = 0
		}
		st.Questions = append(st.Questions, qs)
	}
	return st, nil
}

func (s *Service) Categories(ctx context.Context, p auth.Principal) ([]Category, error) {
	return s.repo.Categories(ctx, p.UserID)
}

func (s *Service) CreateCategory(ctx context.Context, p auth.Principal, req CategoryRequest) (*Category, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 60 {
		return nil, apperr.Validation(map[string]string{"name": "Название категории: от 1 до 60 символов"})
	}
	c, err := s.repo.CreateCategory(ctx, p.UserID, name, "#9254DE")
	if database.IsUniqueViolation(err, "") {
		return nil, apperr.Validation(map[string]string{"name": "Такая категория уже есть"})
	}
	return c, err
}
