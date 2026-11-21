package homework

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/attempts"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/classrooms"
	"ortalyq/backend/internal/modules/exercises"
	"ortalyq/backend/internal/realtime"
)

type Classrooms interface {
	RequireLesson(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID) (string, error)
	ResolveStudent(ctx context.Context, p auth.Principal, role string, classroomID uuid.UUID, requested *uuid.UUID) (uuid.UUID, error)
}

type Attempts interface {
	FinalizeHomework(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID) ([]attempts.SnapshotExercise, error)
	ReopenHomework(ctx context.Context, tx pgx.Tx, classroomID, lessonID, studentID uuid.UUID) error
	LessonExercises(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, scope string, student *uuid.UUID) ([]attempts.ExerciseView, error)
}

type Service struct {
	repo *Repository
	cls  Classrooms
	att  Attempts
	pub  realtime.Publisher
}

func NewService(repo *Repository, cls Classrooms, att Attempts, pub realtime.Publisher) *Service {
	return &Service{repo: repo, cls: cls, att: att, pub: pub}
}

func (s *Service) Get(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, student *uuid.UUID) (*View, error) {
	role, err := s.cls.RequireLesson(ctx, p, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	sid, err := s.cls.ResolveStudent(ctx, p, role, classroomID, student)
	if err != nil {
		return nil, err
	}
	ex, err := s.att.LessonExercises(ctx, p, classroomID, lessonID, exercises.ScopeHomework, &sid)
	if err != nil {
		return nil, err
	}
	hasVisible := false
	for _, e := range ex {
		if !e.Hidden {
			hasVisible = true
		}
	}
	v := &View{StudentID: sid, Status: StatusNotSubmitted, HasExercises: hasVisible, Events: []StatusEvent{}}
	h, err := s.repo.Find(ctx, classroomID, lessonID, sid)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return v, nil
	}
	v.Status, v.StatusChangedAt = h.Status, &h.StatusChangedAt
	if v.Events, err = s.repo.Events(ctx, h.ID); err != nil {
		return nil, err
	}
	sub, count, err := s.repo.LatestSubmission(ctx, h.ID)
	if err != nil {
		return nil, err
	}
	v.SubmissionsCount = count
	if role == classrooms.MemberTeacher {
		v.LatestSubmission = sub
	}
	return v, nil
}

func (s *Service) publish(classroomID, lessonID, studentID uuid.UUID, status string) {
	s.pub.Publish(classroomID, realtime.EventHomeworkStatus, map[string]any{"lesson_id": lessonID, "student_id": studentID, "status": status})
}

// Submit sends the homework for review, storing an immutable snapshot of answers.
// Re-submitting while already pending is a no-op (idempotent).
func (s *Service) Submit(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID) (*View, error) {
	role, err := s.cls.RequireLesson(ctx, p, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.Forbidden()
	}
	current, err := s.Get(ctx, p, classroomID, lessonID, nil)
	if err != nil {
		return nil, err
	}
	if !current.HasExercises {
		return nil, apperr.Conflict("homework_empty", "В домашней работе нет заданий")
	}
	changed := false
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		h, err := s.repo.Lock(ctx, tx, classroomID, lessonID, p.UserID)
		if err != nil {
			return err
		}
		switch h.Status {
		case StatusPending:
			return nil
		case StatusDone:
			return apperr.Conflict("homework_done", "Работа уже принята преподавателем")
		}
		snap, err := s.att.FinalizeHomework(ctx, tx, classroomID, lessonID, p.UserID)
		if err != nil {
			return err
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if err := s.repo.AddSubmission(ctx, tx, h.ID, p.UserID, b); err != nil {
			return err
		}
		changed = true
		return s.repo.SetStatus(ctx, tx, h, StatusPending, p.UserID)
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.publish(classroomID, lessonID, p.UserID, StatusPending)
	}
	return s.Get(ctx, p, classroomID, lessonID, nil)
}

// SetStatus applies a teacher review transition.
func (s *Service) SetStatus(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, req StatusRequest) (*View, error) {
	role, err := s.cls.RequireLesson(ctx, p, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberTeacher {
		return nil, apperr.Forbidden()
	}
	sid, err := s.cls.ResolveStudent(ctx, p, role, classroomID, &req.StudentID)
	if err != nil {
		return nil, err
	}
	if _, ok := teacherTransitions[req.ToStatus]; !ok && req.ToStatus != StatusPending {
		return nil, apperr.Validation(map[string]string{"to_status": "Неизвестный статус"})
	}
	changed := false
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		h, err := s.repo.Lock(ctx, tx, classroomID, lessonID, sid)
		if err != nil {
			return err
		}
		if h.Status == req.ToStatus {
			return nil // idempotent
		}
		if req.FromStatus != "" && req.FromStatus != h.Status {
			return apperr.Conflict("status_changed", "Статус уже изменился — обновите страницу")
		}
		if !teacherTransitions[h.Status][req.ToStatus] {
			return apperr.Conflict("transition_not_allowed", "Такой переход статуса недоступен")
		}
		switch req.ToStatus {
		case StatusRevision, StatusNotSubmitted:
			if err := s.att.ReopenHomework(ctx, tx, classroomID, lessonID, sid); err != nil {
				return err
			}
		case StatusDone:
			// Freeze any unfinished attempts so the accepted state is graded.
			if _, err := s.att.FinalizeHomework(ctx, tx, classroomID, lessonID, sid); err != nil {
				return err
			}
		}
		changed = true
		return s.repo.SetStatus(ctx, tx, h, req.ToStatus, p.UserID)
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.publish(classroomID, lessonID, sid, req.ToStatus)
	}
	return s.Get(ctx, p, classroomID, lessonID, &sid)
}
