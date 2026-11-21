package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const msgSelect = `SELECT m.id, m.classroom_id, m.client_id, u.id, u.full_name, u.avatar_color, COALESCE(cm.role, ''),
	m.kind, m.body, m.gif, a.id, a.original_name, a.mime_type, a.size_bytes, m.created_at, m.edited_at, m.deleted_at
	FROM chat_messages m JOIN users u ON u.id = m.author_id
	LEFT JOIN classroom_members cm ON cm.classroom_id = m.classroom_id AND cm.user_id = m.author_id
	LEFT JOIN attachments a ON a.id = m.attachment_id`

func scanMsg(row pgx.Row) (*Message, error) {
	var m Message
	var gif []byte
	var aid *uuid.UUID
	var aname, amime *string
	var asize *int64
	if err := row.Scan(&m.ID, &m.ClassroomID, &m.ClientID, &m.Author.ID, &m.Author.FullName, &m.Author.AvatarColor, &m.Author.Role,
		&m.Kind, &m.Body, &gif, &aid, &aname, &amime, &asize, &m.CreatedAt, &m.EditedAt, &m.DeletedAt); err != nil {
		return nil, err
	}
	if len(gif) > 0 {
		var g Gif
		if err := json.Unmarshal(gif, &g); err == nil {
			m.Gif = &g
		}
	}
	if aid != nil {
		m.Attachment = &AttachmentRef{ID: *aid, Name: *aname, MimeType: *amime, SizeBytes: *asize}
	}
	return &m, nil
}

// List returns non-deleted messages newest-first before the cursor.
func (r *Repository) List(ctx context.Context, classroomID uuid.UUID, before *time.Time, beforeID *uuid.UUID, limit int) ([]Message, error) {
	rows, err := r.pool.Query(ctx, msgSelect+` WHERE m.classroom_id = $1 AND m.deleted_at IS NULL
		AND ($2::timestamptz IS NULL OR (m.created_at, m.id) < ($2, $3::uuid))
		ORDER BY m.created_at DESC, m.id DESC LIMIT $4`, classroomID, before, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		m, err := scanMsg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Message, error) {
	return scanMsg(r.pool.QueryRow(ctx, msgSelect+` WHERE m.id = $1`, id))
}

func (r *Repository) ByClientID(ctx context.Context, authorID, clientID uuid.UUID) (*Message, error) {
	return scanMsg(r.pool.QueryRow(ctx, msgSelect+` WHERE m.author_id = $1 AND m.client_id = $2`, authorID, clientID))
}

// Insert is idempotent on (author_id, client_id). Returns created=false on repeats.
func (r *Repository) Insert(ctx context.Context, classroomID, authorID, clientID uuid.UUID, kind, body string, gif *Gif, attachmentID *uuid.UUID) (uuid.UUID, bool, error) {
	var gifJSON []byte
	if gif != nil {
		gifJSON, _ = json.Marshal(gif)
	}
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `INSERT INTO chat_messages (classroom_id, author_id, client_id, kind, body, gif, attachment_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (author_id, client_id) DO NOTHING RETURNING id`,
		classroomID, authorID, clientID, kind, body, gifJSON, attachmentID).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func (r *Repository) AttachmentInUse(ctx context.Context, attachmentID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM chat_messages WHERE attachment_id = $1)`, attachmentID).Scan(&ok)
	return ok, err
}

func (r *Repository) UpdateBody(ctx context.Context, id uuid.UUID, body string) error {
	_, err := r.pool.Exec(ctx, `UPDATE chat_messages SET body = $2, edited_at = now() WHERE id = $1 AND deleted_at IS NULL`, id, body)
	return err
}

// SoftDelete wipes content; the row stays for audit/idempotency.
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE chat_messages SET deleted_at = now(), body = '', gif = NULL, attachment_id = NULL WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}
