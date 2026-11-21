// Package chat implements the classroom chat. Messages are stored in PostgreSQL;
// WebSocket only delivers change notifications.
package chat

import (
	"time"

	"github.com/google/uuid"
)

type Author struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	AvatarColor string    `json:"avatar_color"`
	Role        string    `json:"role"`
}

type Gif struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	PreviewURL string `json:"preview_url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type AttachmentRef struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
}

type Message struct {
	ID          uuid.UUID      `json:"id"`
	ClassroomID uuid.UUID      `json:"classroom_id"`
	ClientID    uuid.UUID      `json:"client_id"`
	Author      Author         `json:"author"`
	Kind        string         `json:"kind"`
	Body        string         `json:"body"`
	Gif         *Gif           `json:"gif"`
	Attachment  *AttachmentRef `json:"attachment"`
	CreatedAt   time.Time      `json:"created_at"`
	EditedAt    *time.Time     `json:"edited_at"`
	DeletedAt   *time.Time     `json:"-"`
}
