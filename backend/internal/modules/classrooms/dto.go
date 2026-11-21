package classrooms

import "github.com/google/uuid"

type CreateRequest struct {
	Name       string    `json:"name"`
	LessonType string    `json:"lesson_type"`
	StudentID  uuid.UUID `json:"student_id"`
	CoverColor string    `json:"cover_color"`
}

type UpdateRequest struct {
	Name string `json:"name"`
}

type SetLessonRequest struct {
	LessonID uuid.UUID `json:"lesson_id"`
}

// ListFilter: search + type filter + sorting for the classes page.
type ListFilter struct {
	Query  string
	Type   string // "", individual, group, marathon
	Sort   string // created | name
	Desc   bool
	Limit  int
	Offset int
}
