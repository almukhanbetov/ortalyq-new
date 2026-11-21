// Package attempts runs exercise attempts: start, answer, check, finish, reset.
package attempts

import (
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/modules/exercises"
)

const (
	StatusInProgress = "in_progress"
	StatusFinished   = "finished"
	StatusReset      = "reset"

	ReasonManual         = "manual"
	ReasonTimeout        = "timeout"
	ReasonHomeworkSubmit = "homework_submit"
)

type Attempt struct {
	ID              uuid.UUID
	ClassroomID     uuid.UUID
	ExerciseID      uuid.UUID
	VersionID       uuid.UUID
	StudentID       uuid.UUID
	Status          string
	StartedAt       time.Time
	DeadlineAt      *time.Time
	FinishedAt      *time.Time
	FinishReason    *string
	CorrectCount    *int
	WrongCount      *int
	UnansweredCount *int
	PointsEarned    *int
	PointsMax       *int
}

type Answer struct {
	QuestionID uuid.UUID
	Selected   []uuid.UUID
	Checked    bool
	UpdatedAt  time.Time
}

// Summary is the result of grading an attempt.
type Summary struct {
	ElapsedSeconds int `json:"elapsed_seconds"`
	Correct        int `json:"correct"`
	Wrong          int `json:"wrong"`
	Unanswered     int `json:"unanswered"`
	PointsEarned   int `json:"points_earned"`
	PointsMax      int `json:"points_max"`
}

// Grade computes the summary of answers against a version. Pure and deterministic,
// so recomputation never double-counts points.
func Grade(v *exercises.Version, answers map[uuid.UUID]Answer, onlyChecked bool) Summary {
	key := v.CorrectSet()
	var s Summary
	for _, q := range v.Questions {
		a, ok := answers[q.ID]
		if !ok || (onlyChecked && !a.Checked) {
			s.Unanswered++
			continue
		}
		switch exercises.GradeQuestion(key[q.ID], a.Selected) {
		case exercises.Correct:
			s.Correct++
		case exercises.Wrong:
			s.Wrong++
		default:
			s.Unanswered++
		}
	}
	s.PointsMax = v.MaxPoints()
	if v.Settings.ScoringEnabled {
		s.PointsEarned = s.Correct * v.Settings.PointsPerCorrect
	}
	return s
}
