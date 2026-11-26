package documents

import "time"

type DocumentDTO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	FileName  string    `json:"file_name"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type ListResponse struct {
	Items []DocumentDTO `json:"items"`
	Total int           `json:"total"`
}

func toDTO(d *Document) DocumentDTO {
	return DocumentDTO{ID: d.ID.String(), Title: d.Title, FileName: d.OriginalName, MimeType: d.MimeType, SizeBytes: d.SizeBytes, CreatedAt: d.CreatedAt}
}
