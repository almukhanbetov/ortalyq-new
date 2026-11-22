// Package passwordreset restores access by e-mail: request → one-time link → new password → sign in again.
// The answer to a request is the same for an existing and an unknown address; only an existing, active account gets
// a letter. A successful reset keeps the account (ID, role, rights) and ends all of its sessions.
package passwordreset

import (
	"time"

	"github.com/google/uuid"
)

// Defaults of the flow (Options overrides them in tests).
const (
	DefaultLinkTTL     = time.Hour
	DefaultResendAfter = 60 * time.Second
	DefaultMaxPerEmail = 5  // requests per address per hour
	DefaultMaxPerIP    = 20 // requests per client IP per hour
	limitWindow        = time.Hour
	// Rows are needed only for the limits and for answering "already used" on an old link.
	keepRows = 30 * 24 * time.Hour
)

// Link states as error codes of the API.
const (
	CodeLinkInvalid = "reset_link_invalid"
	CodeLinkExpired = "reset_link_expired"
	CodeLinkUsed    = "reset_link_used"
)

type Options struct {
	LinkTTL     time.Duration
	ResendAfter time.Duration
	MaxPerEmail int
	MaxPerIP    int
}

func (o Options) withDefaults() Options {
	if o.LinkTTL <= 0 {
		o.LinkTTL = DefaultLinkTTL
	}
	if o.ResendAfter <= 0 {
		o.ResendAfter = DefaultResendAfter
	}
	if o.MaxPerEmail <= 0 {
		o.MaxPerEmail = DefaultMaxPerEmail
	}
	if o.MaxPerIP <= 0 {
		o.MaxPerIP = DefaultMaxPerIP
	}
	return o
}

// link is one reset link (a row of password_resets with a token).
type link struct {
	ID           uuid.UUID
	Email        string
	UserID       uuid.UUID
	ExpiresAt    time.Time
	SupersededAt *time.Time
	UsedAt       *time.Time
	Expired      bool // computed by the database clock
}
