package signup

import "time"

type RequestEmail struct {
	Email       string `json:"email"`
	Locale      string `json:"locale"`
	AcceptTerms bool   `json:"accept_terms"`
}

// RequestResponse is identical for a new and an existing address.
type RequestResponse struct {
	ResendAfter int `json:"resend_after"` // seconds until the next letter may be requested
}

type TokenRequest struct {
	Token string `json:"token"`
}

type LinkInfo struct {
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

type CompleteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}
