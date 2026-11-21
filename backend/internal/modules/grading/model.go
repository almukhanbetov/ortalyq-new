// Package grading handles section/lesson completion, lesson grades and point summaries.
// All state is per classroom, never on the lesson template.
package grading

import (
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/modules/attempts"
)

var validGrades = map[string]bool{"fine": true, "good": true, "amazing": true}

type SectionProgress struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Position    int        `json:"position"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at"`
}

type Completion struct {
	Completed       bool       `json:"completed"`
	CompletedAt     *time.Time `json:"completed_at"`
	CompletedByName string     `json:"completed_by_name"`
	Grade           *string    `json:"grade"`
	GradedAt        *time.Time `json:"graded_at"`
}

type Progress struct {
	LessonID    uuid.UUID         `json:"lesson_id"`
	LessonTitle string            `json:"lesson_title"`
	CourseTitle string            `json:"course_title"`
	StudentID   uuid.UUID         `json:"student_id"`
	Sections    []SectionProgress `json:"sections"`
	Completion  Completion        `json:"completion"`
	Lesson      *attempts.Points  `json:"lesson_points"`
	Homework    *attempts.Points  `json:"homework_points"`
}
