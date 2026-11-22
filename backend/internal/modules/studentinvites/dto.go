package studentinvites

import "time"

// InviteRequest is «Пригласить ученика»: the address and the language of the letter.
type InviteRequest struct {
	Email  string `json:"email"`
	Locale string `json:"locale"`
}

type InviteResponse struct {
	Email       string    `json:"email"`
	ExpiresAt   time.Time `json:"expires_at"`
	ResendAfter int       `json:"resend_after"` // seconds until the next letter to this address may be sent
}

type TokenRequest struct {
	Token string `json:"token"`
}

// LinkInfo is what the invitation page shows. HasAccount tells the holder of the link (the owner of the address)
// whether to sign in or to create an account.
type LinkInfo struct {
	TeacherName string    `json:"teacher_name"`
	Email       string    `json:"email"`
	ExpiresAt   time.Time `json:"expires_at"`
	HasAccount  bool      `json:"has_account"`
}

// RegisterRequest creates a student account from the link.
type RegisterRequest struct {
	Token       string `json:"token"`
	FullName    string `json:"full_name"`
	Password    string `json:"password"`
	AcceptTerms bool   `json:"accept_terms"`
}

type Accepted struct {
	TeacherName string `json:"teacher_name"`
}
