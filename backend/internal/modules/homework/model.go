// Package homework manages homework status, submissions and review transitions.
package homework

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	StatusNotSubmitted = "not_submitted"
	StatusPending      = "pending"
	StatusRevision     = "revision"
	StatusDone         = "done"
)

// teacherTransitions lists allowed teacher-initiated status changes.
var teacherTransitions = map[string]map[string]bool{
	StatusNotSubmitted: {StatusDone: true},
	StatusPending:      {StatusRevision: true, StatusDone: true},
	StatusRevision:     {StatusDone: true},
	StatusDone:         {StatusNotSubmitted: true, StatusRevision: true},
}

type Homework struct {
	ID              uuid.UUID
	ClassroomID     uuid.UUID
	LessonID        uuid.UUID
	StudentID       uuid.UUID
	Status          string
	StatusChangedAt time.Time
	StatusChangedBy *uuid.UUID
}

type StatusEvent struct {
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ActorID    *uuid.UUID `json:"actor_id"`
	ActorName  string    `json:"actor_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Submission struct {
	ID          uuid.UUID       `json:"id"`
	Number      int             `json:"number"`
	SubmittedAt time.Time       `json:"submitted_at"`
	Snapshot    json.RawMessage `json:"snapshot"`
}
