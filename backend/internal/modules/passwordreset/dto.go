package passwordreset

import "time"

type RequestReset struct {
	Email  string `json:"email"`
	Locale string `json:"locale"`
}

// RequestResponse is identical for an existing and an unknown address.
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
