package grading

type SectionCompletionRequest struct {
	Completed bool `json:"completed"`
}

type LessonCompletionRequest struct {
	Completed bool    `json:"completed"`
	Grade     *string `json:"grade"`
}

type GradeRequest struct {
	Grade string `json:"grade"`
}
