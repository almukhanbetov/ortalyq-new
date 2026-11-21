// Package materials manages lesson templates: course -> lesson -> section.
package materials

import (
	"time"

	"github.com/google/uuid"
)

type Course struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CoverColor  string    `json:"cover_color"`
	Position    int       `json:"position"`
	LessonCount int       `json:"lesson_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Lessons     []Lesson  `json:"lessons,omitempty"`
}

// LessonAttrs are material parameters used by the lesson filter.
type LessonAttrs struct {
	Language    string   `json:"language"`
	Age         string   `json:"age"`
	Level       string   `json:"level"`
	LessonType  string   `json:"lesson_type"`
	DurationMin *int     `json:"duration_min"`
	Skills      []string `json:"skills"`
	Grammar     []string `json:"grammar"`
	Vocabulary  []string `json:"vocabulary"`
	Functions   []string `json:"functions"`
	Tags        []string `json:"tags"`
}

type Lesson struct {
	ID           uuid.UUID `json:"id"`
	CourseID     uuid.UUID `json:"course_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Position     int       `json:"position"`
	SectionCount int       `json:"section_count"`
	LessonAttrs
	Sections  []Section `json:"sections,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Section struct {
	ID       uuid.UUID `json:"id"`
	LessonID uuid.UUID `json:"lesson_id"`
	Title    string    `json:"title"`
	Position int       `json:"position"`
}

// LessonRef identifies a lesson with its course for display and access checks.
type LessonRef struct {
	LessonID    uuid.UUID `json:"lesson_id"`
	LessonTitle string    `json:"lesson_title"`
	CourseID    uuid.UUID `json:"course_id"`
	CourseTitle string    `json:"course_title"`
	OwnerID     uuid.UUID `json:"owner_id"`
}

type FilterOptions struct {
	Language   []string `json:"language"`
	Age        []string `json:"age"`
	Level      []string `json:"level"`
	LessonType []string `json:"lesson_type"`
	Skills     []string `json:"skills"`
	Duration   []int    `json:"duration"`
	Grammar    []string `json:"grammar"`
	Vocabulary []string `json:"vocabulary"`
	Functions  []string `json:"functions"`
	Tags       []string `json:"tags"`
}
