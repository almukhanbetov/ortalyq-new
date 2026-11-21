package chat

import (
	"time"

	"github.com/google/uuid"
)

type SendRequest struct {
	ClientID     uuid.UUID  `json:"client_id"`
	Kind         string     `json:"kind"`
	Body         string     `json:"body"`
	GifID        string     `json:"gif_id"`
	AttachmentID *uuid.UUID `json:"attachment_id"`
}

type EditRequest struct {
	Body string `json:"body"`
}

type Cursor struct {
	Before   *time.Time
	BeforeID *uuid.UUID
	Limit    int
}

type Page struct {
	Items   []Message `json:"items"`
	HasMore bool      `json:"has_more"`
}
