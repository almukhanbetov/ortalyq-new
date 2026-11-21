package exercises

import "github.com/google/uuid"

type OptionInput struct {
	Label     string `json:"label"`
	IsCorrect bool   `json:"is_correct"`
}

type QuestionInput struct {
	Prompt   string        `json:"prompt"`
	Multiple bool          `json:"multiple"`
	Options  []OptionInput `json:"options"`
}

type SaveRequest struct {
	Title     string          `json:"title"`
	Settings  Settings        `json:"settings"`
	Questions []QuestionInput `json:"questions"`
	Hidden    *bool           `json:"hidden"`
}

type CreateRequest struct {
	Scope     string     `json:"scope"`
	SectionID *uuid.UUID `json:"section_id"`
	Kind      string     `json:"kind"`
	SaveRequest
}

type HiddenRequest struct {
	Hidden bool `json:"hidden"`
}

type ReorderRequest struct {
	Scope     string      `json:"scope"`
	SectionID *uuid.UUID  `json:"section_id"`
	IDs       []uuid.UUID `json:"ids"`
}

type CategoryRequest struct {
	Name string `json:"name"`
}
