package homework

import (
	"time"

	"github.com/google/uuid"
)

type StatusRequest struct {
	StudentID  uuid.UUID `json:"student_id"`
	ToStatus   string    `json:"to_status"`
	FromStatus string    `json:"from_status"` // optional optimistic check
}

type View struct {
	StudentID        uuid.UUID     `json:"student_id"`
	Status           string        `json:"status"`
	StatusChangedAt  *time.Time    `json:"status_changed_at"`
	HasExercises     bool          `json:"has_exercises"`
	SubmissionsCount int           `json:"submissions_count"`
	Events           []StatusEvent `json:"events"`
	LatestSubmission *Submission   `json:"latest_submission"` // teacher only
}
