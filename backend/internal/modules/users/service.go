package users

import (
	"context"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
)

// SessionRevoker ends all sessions of a user (e.g. after disabling).
type SessionRevoker interface {
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
}

type Service struct {
	repo     *Repository
	sessions SessionRevoker
}

func NewService(repo *Repository, sessions SessionRevoker) *Service {
	return &Service{repo: repo, sessions: sessions}
}

var palette = []string{"#1AC6FF", "#FF7A45", "#9254DE", "#16C784", "#F759AB", "#FFB020", "#2F80ED"}

// AvatarColor picks the default avatar colour of a new account from its address.
func AvatarColor(email string) string {
	if email == "" {
		return palette[0]
	}
	return palette[int(email[0])%len(palette)]
}

func validRole(r string) bool {
	return r == auth.RoleAdmin || r == auth.RoleTeacher || r == auth.RoleStudent || r == auth.RolePrivateTeacher
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]User, int, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	v := apperr.V{}
	_, mailErr := mail.ParseAddress(req.Email)
	v.Check(req.Email != "" && mailErr == nil, "email", "Некорректный email")
	v.Check(req.FullName != "" && len(req.FullName) <= 120, "full_name", "Укажите имя")
	v.Check(validRole(req.Role), "role", "Неизвестная роль")
	if msg := auth.ValidatePassword(req.Password); msg != "" {
		v.Add("password", msg)
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	color := AvatarColor(req.Email)
	u, err := s.repo.Create(ctx, req.Email, req.FullName, req.Role, hash, color)
	if database.IsUniqueViolation(err, "") {
		return nil, apperr.Validation(map[string]string{"email": "Пользователь с таким email уже существует"})
	}
	return u, err
}

func (s *Service) Update(ctx context.Context, actor auth.Principal, id uuid.UUID, req UpdateUserRequest) (*User, error) {
	cur, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Пользователь")
		}
		return nil, err
	}
	v := apperr.V{}
	if req.FullName != nil {
		n := strings.TrimSpace(*req.FullName)
		req.FullName = &n
		v.Check(n != "" && len(n) <= 120, "full_name", "Укажите имя")
	}
	if req.Role != nil {
		v.Check(validRole(*req.Role), "role", "Неизвестная роль")
		v.Check(id != actor.UserID || *req.Role == auth.RoleAdmin, "role", "Нельзя снять роль администратора с себя")
		// A private teacher never has school students: assignments are removed by the administrator first.
		if *req.Role == auth.RolePrivateTeacher && cur.Role != auth.RolePrivateTeacher {
			has, err := s.repo.HasAssignments(ctx, id)
			if err != nil {
				return nil, err
			}
			v.Check(!has, "role", "Сначала снимите назначенных преподавателю учеников")
		}
	}
	if req.Disabled != nil && *req.Disabled {
		v.Check(id != actor.UserID, "disabled", "Нельзя заблокировать себя")
	}
	var hash *string
	if req.Password != nil {
		if msg := auth.ValidatePassword(*req.Password); msg != "" {
			v.Add("password", msg)
		} else {
			h, err := auth.HashPassword(*req.Password)
			if err != nil {
				return nil, err
			}
			hash = &h
		}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, req.FullName, req.Role, req.Disabled, hash); err != nil {
		return nil, err
	}
	if (req.Disabled != nil && *req.Disabled) || hash != nil || req.Role != nil {
		if err := s.sessions.DeleteUserSessions(ctx, id); err != nil {
			return nil, err
		}
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) Assign(ctx context.Context, actor auth.Principal, req AssignmentRequest) error {
	t, err := s.repo.Get(ctx, req.TeacherID)
	if err != nil {
		return apperr.Validation(map[string]string{"teacher_id": "Преподаватель не найден"})
	}
	st, err := s.repo.Get(ctx, req.StudentID)
	if err != nil {
		return apperr.Validation(map[string]string{"student_id": "Ученик не найден"})
	}
	v := apperr.V{}
	// School students are assigned to school teachers only; a private teacher (self sign-up) gets none.
	v.Check(t.Role == auth.RoleTeacher, "teacher_id", "Пользователь не является преподавателем школы")
	v.Check(st.Role == auth.RoleStudent, "student_id", "Пользователь не является учеником")
	if err := v.Err(); err != nil {
		return err
	}
	return s.repo.Assign(ctx, req.TeacherID, req.StudentID, actor.UserID)
}

func (s *Service) Unassign(ctx context.Context, req AssignmentRequest) error {
	return s.repo.Unassign(ctx, req.TeacherID, req.StudentID)
}

func (s *Service) Assignments(ctx context.Context, limit, offset int) ([]Assignment, int, error) {
	return s.repo.Assignments(ctx, limit, offset)
}

func (s *Service) MyStudents(ctx context.Context, p auth.Principal) ([]User, error) {
	if !p.CanTeach() {
		return nil, apperr.Forbidden()
	}
	return s.repo.StudentsOf(ctx, p.UserID)
}

// IsAssigned is used by the classrooms module when a teacher creates a class.
func (s *Service) IsAssigned(ctx context.Context, teacherID, studentID uuid.UUID) (bool, error) {
	return s.repo.IsAssigned(ctx, teacherID, studentID)
}

// AssignedStudentByEmail finds an active student assigned to the teacher by e-mail (case-insensitive).
// ok=false for any other address: unknown, not a student, disabled or not assigned — the caller cannot tell
// these apart, so a teacher cannot probe which accounts exist.
func (s *Service) AssignedStudentByEmail(ctx context.Context, teacherID uuid.UUID, email string) (uuid.UUID, string, bool, error) {
	list, err := s.repo.StudentsOf(ctx, teacherID)
	if err != nil {
		return uuid.Nil, "", false, err
	}
	for _, u := range list {
		if strings.EqualFold(u.Email, email) {
			return u.ID, u.FullName, true, nil
		}
	}
	return uuid.Nil, "", false, nil
}
