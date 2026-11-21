package uploads

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const cols = `id, classroom_id, uploader_id, original_name, stored_name, mime_type, size_bytes, purpose, created_at`

func (r *Repository) Insert(ctx context.Context, a *Attachment) error {
	if a.Purpose == "" {
		a.Purpose = PurposeChat
	}
	return r.pool.QueryRow(ctx, `INSERT INTO attachments (classroom_id, uploader_id, original_name, stored_name, mime_type, size_bytes, purpose)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`, a.ClassroomID, a.UploaderID, a.OriginalName, a.StoredName, a.MimeType, a.SizeBytes, a.Purpose).
		Scan(&a.ID, &a.CreatedAt)
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Attachment, error) {
	var a Attachment
	err := r.pool.QueryRow(ctx, `SELECT `+cols+` FROM attachments WHERE id = $1`, id).
		Scan(&a.ID, &a.ClassroomID, &a.UploaderID, &a.OriginalName, &a.StoredName, &a.MimeType, &a.SizeBytes, &a.Purpose, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1`, id)
	return err
}
