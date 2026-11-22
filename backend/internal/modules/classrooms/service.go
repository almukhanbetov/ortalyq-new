package classrooms

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/materials"
	"ortalyq/backend/internal/realtime"
)

type AssignmentChecker interface {
	IsAssigned(ctx context.Context, teacherID, studentID uuid.UUID) (bool, error)
}

// LessonOwnership is satisfied by materials.Service.
type LessonOwnership interface {
	OwnedLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) (*materials.LessonRef, error)
}

// MarathonGate decides whether a marathon student may work on lessons right now (implemented by marathons.Service).
// It returns nil for classrooms that are not marathons. q lets callers run the check inside their transaction.
type MarathonGate interface {
	// lessonID = uuid.Nil checks the marathon only; otherwise the lesson rule of the passage mode applies too.
	StudentAccess(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) error
}

type Service struct {
	repo      *Repository
	assign    AssignmentChecker
	lessons   LessonOwnership
	gate      MarathonGate
	publisher realtime.Publisher
}

func NewService(repo *Repository, assign AssignmentChecker, publisher realtime.Publisher) *Service {
	return &Service{repo: repo, assign: assign, publisher: publisher}
}

// SetLessonOwnership breaks the construction cycle with materials (which notifies us).
func (s *Service) SetLessonOwnership(l LessonOwnership) { s.lessons = l }

// SetMarathonGate breaks the construction cycle with marathons (which depends on classrooms).
func (s *Service) SetMarathonGate(g MarathonGate) { s.gate = g }

// ---- Access checks used by other modules ----

// StudentAccess rejects lesson work of a marathon student after the marathon is closed.
// Teachers and regular classes are never restricted here.
func (s *Service) StudentAccess(ctx context.Context, q database.Querier, classroomID, lessonID, studentID uuid.UUID) error {
	if s.gate == nil {
		return nil
	}
	if q == nil {
		q = s.repo.db
	}
	return s.gate.StudentAccess(ctx, q, classroomID, lessonID, studentID)
}

// MemberRole returns the caller's role in the classroom or 404 when not a member.
func (s *Service) MemberRole(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (string, error) {
	role, err := s.repo.MemberRole(ctx, classroomID, p.UserID)
	if err != nil {
		if database.IsNoRows(err) {
			return "", apperr.NotFound("Класс")
		}
		return "", err
	}
	return role, nil
}

func (s *Service) RequireTeacher(ctx context.Context, p auth.Principal, classroomID uuid.UUID) error {
	role, err := s.MemberRole(ctx, p, classroomID)
	if err != nil {
		return err
	}
	if role != MemberTeacher {
		return apperr.Forbidden()
	}
	return nil
}

// RequireLesson checks membership and that the lesson is opened in this classroom.
func (s *Service) RequireLesson(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID) (string, error) {
	role, err := s.MemberRole(ctx, p, classroomID)
	if err != nil {
		return "", err
	}
	ok, err := s.repo.LessonOpened(ctx, classroomID, lessonID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", apperr.NotFound("Урок")
	}
	if role == MemberStudent {
		if err := s.StudentAccess(ctx, nil, classroomID, lessonID, p.UserID); err != nil {
			return "", err
		}
	}
	return role, nil
}

// ResolveStudent picks whose progress is being viewed. Students always see their own;
// teachers may pass a student id that must belong to the classroom.
func (s *Service) ResolveStudent(ctx context.Context, p auth.Principal, role string, classroomID uuid.UUID, requested *uuid.UUID) (uuid.UUID, error) {
	if role == MemberStudent {
		if requested != nil && *requested != p.UserID {
			return uuid.Nil, apperr.Forbidden()
		}
		return p.UserID, nil
	}
	ids, err := s.repo.StudentIDs(ctx, classroomID)
	if err != nil {
		return uuid.Nil, err
	}
	if requested == nil {
		if len(ids) == 0 {
			return uuid.Nil, apperr.NotFound("Ученик")
		}
		return ids[0], nil
	}
	for _, id := range ids {
		if id == *requested {
			return id, nil
		}
	}
	return uuid.Nil, apperr.NotFound("Ученик")
}

func (s *Service) StudentIDs(ctx context.Context, classroomID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.StudentIDs(ctx, classroomID)
}

// LessonChanged implements materials.ChangeNotifier.
func (s *Service) LessonChanged(ctx context.Context, lessonID uuid.UUID) {
	ids, err := s.repo.ClassroomsUsingLesson(ctx, lessonID)
	if err != nil {
		slog.Error("lesson changed lookup", "err", err)
		return
	}
	for _, id := range ids {
		s.publisher.Publish(id, realtime.EventMaterialsChanged, map[string]any{"lesson_id": lessonID})
	}
}

// ---- Use cases ----

func (s *Service) decorate(ctx context.Context, userID uuid.UUID, list []Classroom) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(list))
	for i := range list {
		ids[i] = list[i].ID
	}
	members, err := s.repo.Members(ctx, ids)
	if err != nil {
		return err
	}
	badges, err := s.repo.HomeworkBadges(ctx, userID, ids)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Members = members[list[i].ID]
		if list[i].Members == nil {
			list[i].Members = []Member{}
		}
		list[i].HomeworkBadge = badges[list[i].ID]
	}
	return nil
}

var classTypes = map[string]bool{"": true, "individual": true, "group": true, "marathon": true}

func (s *Service) List(ctx context.Context, p auth.Principal, f ListFilter) ([]Classroom, int, error) {
	f.Query = strings.TrimSpace(f.Query)
	v := apperr.V{}
	v.Check(classTypes[f.Type], "type", "Неизвестный тип класса")
	v.Check(f.Sort == "" || f.Sort == "created" || f.Sort == "name", "sort", "Неизвестная сортировка")
	v.Check(len([]rune(f.Query)) <= 120, "q", "Слишком длинный запрос")
	if err := v.Err(); err != nil {
		return nil, 0, err
	}
	list, total, err := s.repo.ListForUser(ctx, p.UserID, f)
	if err != nil {
		return nil, 0, err
	}
	return list, total, s.decorate(ctx, p.UserID, list)
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (*Classroom, error) {
	c, err := s.repo.GetForUser(ctx, id, p.UserID)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Класс")
		}
		return nil, err
	}
	list := []Classroom{*c}
	if err := s.decorate(ctx, p.UserID, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, req CreateRequest) (*Classroom, error) {
	if !p.CanTeach() {
		return nil, apperr.Forbidden()
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.LessonType == "" {
		req.LessonType = "individual"
	}
	if req.CoverColor == "" {
		req.CoverColor = "#1AC6FF"
	}
	v := apperr.V{}
	v.Check(req.Name != "" && len([]rune(req.Name)) <= 120, "name", "Укажите название класса")
	v.Check(req.LessonType == "individual", "lesson_type", "На этом этапе доступен только индивидуальный класс")
	ok, err := s.assign.IsAssigned(ctx, p.UserID, req.StudentID)
	if err != nil {
		return nil, err
	}
	// The same rule for both kinds of teacher: only a student linked to the caller (teacher_students). School teachers
	// get students from the administrator, a private teacher from accepted invitations.
	if p.Role == auth.RolePrivateTeacher {
		v.Check(ok, "student_id", "Выберите ученика, принявшего ваше приглашение")
	} else {
		v.Check(ok, "student_id", "Выберите ученика, назначенного вам администратором")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	id, err := s.repo.Create(ctx, p.UserID, req.StudentID, req.Name, req.LessonType, req.CoverColor)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, p, id)
}

func (s *Service) Rename(ctx context.Context, p auth.Principal, id uuid.UUID, req UpdateRequest) (*Classroom, error) {
	if err := s.RequireTeacher(ctx, p, id); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 120 {
		return nil, apperr.Validation(map[string]string{"name": "Укажите название класса"})
	}
	if err := s.repo.Rename(ctx, id, name); err != nil {
		return nil, err
	}
	return s.Get(ctx, p, id)
}

func (s *Service) SetCurrentLesson(ctx context.Context, p auth.Principal, id uuid.UUID, lessonID uuid.UUID) (*Classroom, error) {
	if err := s.RequireTeacher(ctx, p, id); err != nil {
		return nil, err
	}
	if c, err := s.Get(ctx, p, id); err != nil {
		return nil, err
	} else if c.LessonType == TypeMarathon {
		// Students pass marathon lessons on their own; there is no teacher-led current lesson.
		return nil, apperr.Conflict("marathon_no_current_lesson", "В марафоне нет текущего урока")
	}
	ref, err := s.lessons.OwnedLesson(ctx, p, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetCurrentLesson(ctx, id, lessonID, ref.CourseID); err != nil {
		return nil, err
	}
	s.publisher.Publish(id, realtime.EventCurrentLesson, map[string]any{"lesson_id": lessonID})
	return s.Get(ctx, p, id)
}
