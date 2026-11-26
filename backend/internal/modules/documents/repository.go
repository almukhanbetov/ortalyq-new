package documents

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const cols = `id, owner_id, title, original_name, stored_name, mime_type, size_bytes, created_at`

func scan(row pgx.Row) (*Document, error) {
	var d Document
	err := row.Scan(&d.ID, &d.OwnerID, &d.Title, &d.OriginalName, &d.StoredName, &d.MimeType, &d.SizeBytes, &d.CreatedAt)
	return &d, err
}

func (r *Repository) Create(ctx context.Context, d *Document) (*Document, error) {
	return scan(r.pool.QueryRow(ctx, `
		INSERT INTO material_documents (owner_id, title, original_name, stored_name, mime_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+cols,
		d.OwnerID, d.Title, d.OriginalName, d.StoredName, d.MimeType, d.SizeBytes))
}

// List returns the owner's documents, newest first, matching q in the title (as the search of courses does).
func (r *Repository) List(ctx context.Context, owner uuid.UUID, q string, limit, offset int) ([]Document, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+`, count(*) OVER () FROM material_documents
		WHERE owner_id = $1 AND ($2 = '' OR title ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC, id LIMIT $3 OFFSET $4`, owner, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out, total := []Document{}, 0
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Title, &d.OriginalName, &d.StoredName, &d.MimeType, &d.SizeBytes, &d.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && offset > 0 {
		err = r.pool.QueryRow(ctx, `SELECT count(*) FROM material_documents WHERE owner_id = $1 AND ($2 = '' OR title ILIKE '%' || $2 || '%')`, owner, q).Scan(&total)
	}
	return out, total, err
}

// Get returns a document or nil.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Document, error) {
	d, err := scan(r.pool.QueryRow(ctx, `SELECT `+cols+` FROM material_documents WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return d, err
}
