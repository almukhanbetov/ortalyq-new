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
	// CoverImageID is the uploaded cover picture (served to the owner by /course-covers/:id); nil shows the colour.
	CoverImageID *uuid.UUID `json:"cover_image_id"`
	// Language is a code of the project list (internal/languages) or "".
	Language string `json:"language"`
	// SectionsEnabled («Разделы»): lessons are divided into named sections; off — one section shown as a list.
	SectionsEnabled bool      `json:"sections_enabled"`
	Tags            []string  `json:"tags"`
	Position        int       `json:"position"`
	LessonCount     int       `json:"lesson_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Lessons         []Lesson  `json:"lessons,omitempty"`
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
	// SectionsEnabled is the «Разделы» switch of the course.
	SectionsEnabled bool `json:"sections_enabled"`
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

// Cover is a cover picture uploaded from the material form; only its uploader attaches and reads it.
type Cover struct {
	ID         uuid.UUID `json:"id"`
	OwnerID    uuid.UUID `json:"-"`
	StoredName string    `json:"-"`
	MimeType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	CreatedAt  time.Time `json:"created_at"`
}

// DefaultSectionTitle is the one section a lesson gets when the course has «Разделы» off.
const DefaultSectionTitle = "Основное"

// MaxTags is the most tags a material keeps; each tag is at most TagMax characters.
const (
	MaxTags = 20
	TagMax  = 40
)
