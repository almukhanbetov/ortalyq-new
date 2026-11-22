// Package signup is the self sign-up of a private teacher: e-mail → one-time confirmation link → password →
// signed in. The server assigns the role (auth.RolePrivateTeacher); an address that already has an account is
// never changed by this flow, and the answer to the e-mail step is the same for a new and an existing address.
package signup

import (
	"time"

	"github.com/google/uuid"
)

// Defaults of the flow (Options overrides them in tests).
const (
	DefaultLinkTTL     = 24 * time.Hour
	DefaultResendAfter = 60 * time.Second
	DefaultMaxPerEmail = 5  // letters per address per hour
	DefaultMaxPerIP    = 20 // letters per client IP per hour
	limitWindow        = time.Hour
	// Rows are needed only for the limits and for answering "already used" on an old link.
	keepRows = 30 * 24 * time.Hour
)

// Link states as error codes of the API.
const (
	CodeLinkInvalid = "signup_link_invalid"
	CodeLinkExpired = "signup_link_expired"
	CodeLinkUsed    = "signup_link_used"
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

// link is one confirmation link (a row of email_signups with a token).
type link struct {
	ID           uuid.UUID
	Email        string
	Locale       string
	TermsAt      time.Time
	ExpiresAt    time.Time
	SupersededAt *time.Time
	UsedAt       *time.Time
	Expired      bool // computed by the database clock
}
