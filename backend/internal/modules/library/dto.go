package library

import "github.com/google/uuid"

// CatalogLesson is one lesson of a catalog course, as shown before adding it.
type CatalogLesson struct {
	Title       string   `json:"title"`
	Level       string   `json:"level"`
	DurationMin int      `json:"duration_min"`
	Sections    []string `json:"sections"`
	Exercises   int      `json:"exercises"` // answered exercises of the class part (tests, gaps)
	TextBlocks  int      `json:"text_blocks"`
	Homework    int      `json:"homework"`
}

// CatalogCourse is a course of the catalog with the teacher's own copy, if any.
type CatalogCourse struct {
	Key         string          `json:"key"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	CoverColor  string          `json:"cover_color"`
	Level       string          `json:"level"`
	Language    string          `json:"language"`
	LessonCount int             `json:"lesson_count"`
	Lessons     []CatalogLesson `json:"lessons"`
	// OwnedCourseID is the teacher's copy in «Личные материалы»; null until the course is added.
	OwnedCourseID *uuid.UUID `json:"owned_course_id"`
}

type ListResponse struct {
	Items []CatalogCourse `json:"items"`
	Total int             `json:"total"`
}

// AddResponse: the teacher's copy; Created is false when the course was already there.
type AddResponse struct {
	CourseID uuid.UUID `json:"course_id"`
	Created  bool      `json:"created"`
}
