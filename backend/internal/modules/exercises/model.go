// Package exercises manages exercise templates and their immutable versions.
package exercises

import (
	"time"

	"github.com/google/uuid"
)

const (
	ScopeLesson   = "lesson"
	ScopeHomework = "homework"
	KindTest      = "test"

	CheckInstant = "instant" // checked while answering
	CheckAfter   = "after"   // checked after the attempt is finished
)

type Option struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	IsCorrect *bool     `json:"is_correct,omitempty"` // omitted when the key must stay hidden
}

type Question struct {
	ID       uuid.UUID `json:"id"`
	Prompt   string    `json:"prompt"`
	Multiple bool      `json:"multiple"`
	Options  []Option  `json:"options"`
}

type Settings struct {
	CheckMode        string     `json:"check_mode"`
	TimerEnabled     bool       `json:"timer_enabled"`
	TimerSeconds     *int       `json:"timer_seconds"`
	ScoringEnabled   bool       `json:"scoring_enabled"`
	PointsPerCorrect int        `json:"points_per_correct"`
	CategoryID       *uuid.UUID `json:"category_id"`
	CategoryName     string     `json:"category_name,omitempty"`
}

// Version is an immutable snapshot of an exercise's content and settings.
type Version struct {
	ID         uuid.UUID  `json:"id"`
	ExerciseID uuid.UUID  `json:"exercise_id"`
	Number     int        `json:"number"`
	Title      string     `json:"title"`
	Settings   Settings   `json:"settings"`
	Questions  []Question `json:"questions"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CorrectSet returns the correct option ids per question.
func (v *Version) CorrectSet() map[uuid.UUID]map[uuid.UUID]bool {
	out := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, q := range v.Questions {
		set := map[uuid.UUID]bool{}
		for _, o := range q.Options {
			if o.IsCorrect != nil && *o.IsCorrect {
				set[o.ID] = true
			}
		}
		out[q.ID] = set
	}
	return out
}

// MaxPoints is the maximum score obtainable for this version.
func (v *Version) MaxPoints() int {
	if !v.Settings.ScoringEnabled {
		return 0
	}
	return len(v.Questions) * v.Settings.PointsPerCorrect
}

// Sanitized returns a deep copy with the answer key removed.
func (v *Version) Sanitized() *Version {
	c := *v
	c.Questions = make([]Question, len(v.Questions))
	for i, q := range v.Questions {
		nq := q
		nq.Options = make([]Option, len(q.Options))
		for j, o := range q.Options {
			nq.Options[j] = Option{ID: o.ID, Label: o.Label}
		}
		c.Questions[i] = nq
	}
	return &c
}

type Exercise struct {
	ID           uuid.UUID  `json:"id"`
	LessonID     uuid.UUID  `json:"lesson_id"`
	Scope        string     `json:"scope"`
	SectionID    *uuid.UUID `json:"section_id"`
	Kind         string     `json:"kind"`
	Position     int        `json:"position"`
	Hidden       bool       `json:"hidden"`
	CopiedFromID *uuid.UUID `json:"copied_from_id"`
	VersionID    uuid.UUID  `json:"version_id"`
	Version      *Version   `json:"version"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Category struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Color  string    `json:"color"`
	System bool      `json:"system"`
}

// CatalogEntry describes an exercise type shown in the "add exercise" catalog.
type CatalogEntry struct {
	Kind      string `json:"kind"`
	Group     string `json:"group"`
	Available bool   `json:"available"`
}

type QuestionStat struct {
	QuestionID uuid.UUID `json:"question_id"`
	Prompt     string    `json:"prompt"`
	Correct    int       `json:"correct"`
	Wrong      int       `json:"wrong"`
	Unanswered int       `json:"unanswered"`
}

type Stats struct {
	VersionNumber int            `json:"version_number"`
	Attempts      int            `json:"attempts"`
	Finished      int            `json:"finished"`
	Questions     []QuestionStat `json:"questions"`
}
