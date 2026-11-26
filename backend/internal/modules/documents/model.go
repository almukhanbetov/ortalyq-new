// Package documents: PDF documents in «Материалы» → «Личные материалы». A document is a personal material of the
// teacher who uploaded it; only the owner sees it in the list, opens it and reads its file.
package documents

import (
	"time"

	"github.com/google/uuid"
)

// TitleMax is the longest title of a document (the same limit as a course).
const TitleMax = 200

type Document struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	Title        string
	OriginalName string
	StoredName   string
	MimeType     string
	SizeBytes    int64
	CreatedAt    time.Time
}
