// Package studentinvites lets a private teacher invite a student by e-mail. The letter carries a one-time link that
// works until it expires. Accepting links the student to the teacher (teacher_students), so the student can be
// chosen for an individual class. An existing account accepts after signing in with the invited address; a new
// address creates a student account from the link (the link proves the address). School teachers keep getting
// students from the administrator.
package studentinvites

import (
	"time"

	"github.com/google/uuid"
)

// Defaults of the flow (Options overrides them in tests).
const (
	DefaultLinkTTL       = 7 * 24 * time.Hour
	DefaultResendAfter   = 60 * time.Second
	DefaultMaxPerTeacher = 30 // letters per teacher per hour
	limitWindow          = time.Hour
)

// Link states and refusals as error codes of the API.
const (
	CodeLinkInvalid   = "student_invite_invalid"
	CodeLinkExpired   = "student_invite_expired"
	CodeLinkUsed      = "student_invite_used"
	CodeWrongAccount  = "student_invite_wrong_account"
	CodeNotStudent    = "student_invite_not_student"
	CodeAccountExists = "student_invite_account_exists"
	CodeAlreadyLinked = "student_already_linked"
)

type Options struct {
	LinkTTL       time.Duration
	ResendAfter   time.Duration
	MaxPerTeacher int
}

func (o Options) withDefaults() Options {
	if o.LinkTTL <= 0 {
		o.LinkTTL = DefaultLinkTTL
	}
	if o.ResendAfter <= 0 {
		o.ResendAfter = DefaultResendAfter
	}
	if o.MaxPerTeacher <= 0 {
		o.MaxPerTeacher = DefaultMaxPerTeacher
	}
	return o
}

// link is one invitation letter (a row of student_invitations).
type link struct {
	ID           uuid.UUID
	TeacherID    uuid.UUID
	Email        string
	ExpiresAt    time.Time
	SupersededAt *time.Time
	AcceptedAt   *time.Time
	AcceptedBy   *uuid.UUID
	Expired      bool // computed by the database clock
}
