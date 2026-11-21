package attempts

import (
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/modules/exercises"
)

type AnswerRequest struct {
	OptionIDs []uuid.UUID `json:"option_ids"`
	Check     bool        `json:"check"`
}

type ResetRequest struct {
	StudentID uuid.UUID `json:"student_id"`
}

type AnswerView struct {
	QuestionID       uuid.UUID   `json:"question_id"`
	Selected         []uuid.UUID `json:"selected"`
	Checked          bool        `json:"checked"`
	Result           *string     `json:"result"`             // correct | wrong | unanswered, only when revealed
	CorrectOptionIDs []uuid.UUID `json:"correct_option_ids"` // only when revealed
}

type AttemptView struct {
	ID           uuid.UUID    `json:"id"`
	Status       string       `json:"status"`
	StartedAt    time.Time    `json:"started_at"`
	DeadlineAt   *time.Time   `json:"deadline_at"`
	FinishedAt   *time.Time   `json:"finished_at"`
	FinishReason *string      `json:"finish_reason"`
	ServerNow    time.Time    `json:"server_now"`
	Answers      []AnswerView `json:"answers"`
	Summary      *Summary     `json:"summary"`
	Revealed     bool         `json:"revealed"`
}

// ExerciseView is an exercise as seen by one viewer in one classroom.
type ExerciseView struct {
	ExerciseID uuid.UUID          `json:"exercise_id"`
	Scope      string             `json:"scope"`
	SectionID  *uuid.UUID         `json:"section_id"`
	Kind       string             `json:"kind"`
	Position   int                `json:"position"`
	Hidden     bool               `json:"hidden"`
	Version    *exercises.Version `json:"version"`
	Attempt    *AttemptView       `json:"attempt"`
	Locked     bool               `json:"locked"` // homework submitted: answers frozen
	ResetCount int                `json:"reset_count"`
}

// SnapshotExercise is the per-exercise part of a homework submission snapshot.
type SnapshotExercise struct {
	ExerciseID uuid.UUID          `json:"exercise_id"`
	VersionID  uuid.UUID          `json:"version_id"`
	Title      string             `json:"title"`
	Version    *exercises.Version `json:"version"`
	AttemptID  *uuid.UUID         `json:"attempt_id"`
	Answers    []AnswerView       `json:"answers"`
	Summary    Summary            `json:"summary"`
}

// ScopePoints aggregates points of one scope (lesson or homework).
type CategoryPoints struct {
	CategoryID   *uuid.UUID `json:"category_id"`
	CategoryName string     `json:"category_name"`
	Earned       int        `json:"earned"`
	Max          int        `json:"max"`
}

type Points struct {
	Earned     int              `json:"earned"`
	Max        int              `json:"max"`
	Categories []CategoryPoints `json:"categories"`
	Exercises  int              `json:"exercises"`
	Finished   int              `json:"finished"`
}
