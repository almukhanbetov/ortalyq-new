package materials

import "github.com/google/uuid"

type CourseRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	CoverColor  *string `json:"cover_color"`
}

type LessonRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Attrs       *LessonAttrs `json:"attrs"`
}

type SectionRequest struct {
	Title string `json:"title"`
}

type ReorderRequest struct {
	IDs []uuid.UUID `json:"ids"`
}

// LessonFilter holds search + material parameters (not progress state).
type LessonFilter struct {
	Query       string
	Language    string
	Age         string
	Level       string
	LessonType  string
	Skill       string
	MaxDuration int
	Grammar     string
	Vocabulary  string
	Function    string
	Tag         string
}

func (f LessonFilter) Active() bool {
	return f.Language != "" || f.Age != "" || f.Level != "" || f.LessonType != "" || f.Skill != "" ||
		f.MaxDuration > 0 || f.Grammar != "" || f.Vocabulary != "" || f.Function != "" || f.Tag != ""
}
