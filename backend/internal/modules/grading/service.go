package grading

import (
	"context"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/modules/attempts"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/classrooms"
	"ortalyq/backend/internal/modules/exercises"
	"ortalyq/backend/internal/modules/materials"
	"ortalyq/backend/internal/realtime"
)

type Classrooms interface {
	RequireLesson(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID) (string, error)
	ResolveStudent(ctx context.Context, p auth.Principal, role string, classroomID uuid.UUID, requested *uuid.UUID) (uuid.UUID, error)
}

type Materials interface {
	LessonRef(ctx context.Context, lessonID uuid.UUID) (*materials.LessonRef, error)
	Sections(ctx context.Context, lessonID uuid.UUID) ([]materials.Section, error)
	SectionLesson(ctx context.Context, sectionID uuid.UUID) (uuid.UUID, error)
}

type Points interface {
	Points(ctx context.Context, classroomID, lessonID, studentID uuid.UUID, scope string) (*attempts.Points, error)
}

type Service struct {
	repo *Repository
	cls  Classrooms
	mat  Materials
	pts  Points
	pub  realtime.Publisher
}

func NewService(repo *Repository, cls Classrooms, mat Materials, pts Points, pub realtime.Publisher) *Service {
	return &Service{repo: repo, cls: cls, mat: mat, pts: pts, pub: pub}
}

func (s *Service) Progress(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, student *uuid.UUID) (*Progress, error) {
	role, err := s.cls.RequireLesson(ctx, p, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	sid, err := s.cls.ResolveStudent(ctx, p, role, classroomID, student)
	if err != nil {
		return nil, err
	}
	ref, err := s.mat.LessonRef(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	secs, err := s.mat.Sections(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	done, err := s.repo.SectionCompletions(ctx, classroomID, lessonID)
	if err != nil {
		return nil, err
	}
	pr := &Progress{LessonID: lessonID, LessonTitle: ref.LessonTitle, CourseTitle: ref.CourseTitle, StudentID: sid, Sections: []SectionProgress{}}
	for _, sec := range secs {
		sp := SectionProgress{ID: sec.ID, Title: sec.Title, Position: sec.Position}
		if d, ok := done[sec.ID]; ok {
			sp.Completed, sp.CompletedAt = true, d.CompletedAt
		}
		pr.Sections = append(pr.Sections, sp)
	}
	if pr.Completion, err = s.repo.Completion(ctx, classroomID, lessonID); err != nil {
		return nil, err
	}
	if pr.Lesson, err = s.pts.Points(ctx, classroomID, lessonID, sid, exercises.ScopeLesson); err != nil {
		return nil, err
	}
	if pr.Homework, err = s.pts.Points(ctx, classroomID, lessonID, sid, exercises.ScopeHomework); err != nil {
		return nil, err
	}
	return pr, nil
}

func (s *Service) requireTeacherLesson(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID) error {
	role, err := s.cls.RequireLesson(ctx, p, classroomID, lessonID)
	if err != nil {
		return err
	}
	if role != classrooms.MemberTeacher {
		return apperr.Forbidden()
	}
	return nil
}

func (s *Service) SetSection(ctx context.Context, p auth.Principal, classroomID, sectionID uuid.UUID, completed bool) error {
	lessonID, err := s.mat.SectionLesson(ctx, sectionID)
	if err != nil {
		return err
	}
	if err := s.requireTeacherLesson(ctx, p, classroomID, lessonID); err != nil {
		return err
	}
	changed, err := s.repo.SetSection(ctx, classroomID, sectionID, p.UserID, completed)
	if err != nil {
		return err
	}
	if changed {
		s.pub.Publish(classroomID, realtime.EventSectionCompletion, map[string]any{"lesson_id": lessonID, "section_id": sectionID, "completed": completed})
	}
	return nil
}

func (s *Service) SetCompletion(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, req LessonCompletionRequest) error {
	if err := s.requireTeacherLesson(ctx, p, classroomID, lessonID); err != nil {
		return err
	}
	if req.Completed && (req.Grade == nil || !validGrades[*req.Grade]) {
		return apperr.Validation(map[string]string{"grade": "Выберите оценку урока"})
	}
	if !req.Completed {
		req.Grade = nil
	}
	changed, err := s.repo.SetCompletion(ctx, classroomID, lessonID, p.UserID, req.Completed, req.Grade)
	if err != nil {
		return err
	}
	if changed {
		s.pub.Publish(classroomID, realtime.EventLessonCompletion, map[string]any{"lesson_id": lessonID, "completed": req.Completed, "grade": req.Grade})
	}
	return nil
}

func (s *Service) SetGrade(ctx context.Context, p auth.Principal, classroomID, lessonID uuid.UUID, grade string) error {
	if err := s.requireTeacherLesson(ctx, p, classroomID, lessonID); err != nil {
		return err
	}
	if !validGrades[grade] {
		return apperr.Validation(map[string]string{"grade": "Выберите оценку урока"})
	}
	c, err := s.repo.Completion(ctx, classroomID, lessonID)
	if err != nil {
		return err
	}
	if !c.Completed {
		return apperr.Conflict("lesson_not_completed", "Сначала завершите урок")
	}
	return s.SetCompletion(ctx, p, classroomID, lessonID, LessonCompletionRequest{Completed: true, Grade: &grade})
}
