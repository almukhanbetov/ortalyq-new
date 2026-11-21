// Package uploads stores chat attachments outside any public directory and serves
// them only to classroom members.
package uploads

import (
	"time"

	"github.com/google/uuid"
)

type Attachment struct {
	ID           uuid.UUID  `json:"id"`
	ClassroomID  uuid.UUID  `json:"classroom_id"`
	UploaderID   *uuid.UUID `json:"uploader_id"`
	OriginalName string     `json:"name"`
	StoredName   string     `json:"-"`
	MimeType     string     `json:"mime_type"`
	SizeBytes    int64      `json:"size_bytes"`
	// Purpose: chat (default) or appearance — a cover or background picture (stage 10 of marathons).
	Purpose   string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// allowed maps detected content types to the extension used for storage.
const (
	PurposeChat       = "chat"
	PurposeAppearance = "appearance"
)

var allowed = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
	"text/plain":      ".txt",
	"audio/mpeg":      ".mp3",
	"video/mp4":       ".mp4",
}

// officeTypes are zip containers recognised by their declared extension.
var officeTypes = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}
