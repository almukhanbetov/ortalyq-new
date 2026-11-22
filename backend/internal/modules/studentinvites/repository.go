package studentinvites

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// LockAddress serialises the letters of one teacher to one address, and the use of the links sent to one address,
// until the transaction ends.
func (r *Repository) LockAddress(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('student_invitations:' || $1, 0))`, key)
	return err
}

type usage struct {
	ByTeacher int
	// Seconds until a letter is allowed again (<= 0: now): the pause for this address and the moment the oldest
	// letter of the teacher leaves the hourly window.
	WaitResend, WaitWindow float64
}

func (r *Repository) Usage(ctx context.Context, tx pgx.Tx, teacherID uuid.UUID, email string, window, resendAfter time.Duration) (usage, error) {
	var u usage
	err := tx.QueryRow(ctx, `SELECT
		count(*),
		COALESCE(EXTRACT(EPOCH FROM max(created_at) FILTER (WHERE email = $2) + make_interval(secs => $4) - now()), 0)::float8,
		COALESCE(EXTRACT(EPOCH FROM min(created_at) + make_interval(secs => $3) - now()), 0)::float8
		FROM student_invitations WHERE teacher_id = $1 AND created_at > now() - make_interval(secs => $3)`,
		teacherID, email, window.Seconds(), resendAfter.Seconds()).Scan(&u.ByTeacher, &u.WaitResend, &u.WaitWindow)
	return u, err
}

// Supersede closes the open links of the teacher to the address: only the newest letter works.
func (r *Repository) Supersede(ctx context.Context, tx pgx.Tx, teacherID uuid.UUID, email string) error {
	_, err := tx.Exec(ctx, `UPDATE student_invitations SET superseded_at = now()
		WHERE teacher_id = $1 AND email = $2 AND accepted_at IS NULL AND superseded_at IS NULL`, teacherID, email)
	return err
}

func (r *Repository) Insert(ctx context.Context, tx pgx.Tx, teacherID uuid.UUID, email, locale string, hash []byte, ttl time.Duration) (uuid.UUID, time.Time, error) {
	var id uuid.UUID
	var exp time.Time
	err := tx.QueryRow(ctx, `INSERT INTO student_invitations (teacher_id, email, locale, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5)) RETURNING id, expires_at`,
		teacherID, email, locale, hash, ttl.Seconds()).Scan(&id, &exp)
	return id, exp, err
}

func (r *Repository) SetOutbox(ctx context.Context, tx pgx.Tx, id, outboxID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE student_invitations SET outbox_id = $2 WHERE id = $1`, id, outboxID)
	return err
}

const linkCols = `id, teacher_id, email, expires_at, superseded_at, accepted_at, accepted_by, expires_at <= now()`

func scanLink(row pgx.Row) (*link, error) {
	var l link
	err := row.Scan(&l.ID, &l.TeacherID, &l.Email, &l.ExpiresAt, &l.SupersededAt, &l.AcceptedAt, &l.AcceptedBy, &l.Expired)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// LinkByHash returns the invitation of a token (nil when unknown).
func (r *Repository) LinkByHash(ctx context.Context, hash []byte) (*link, error) {
	return scanLink(r.pool.QueryRow(ctx, `SELECT `+linkCols+` FROM student_invitations WHERE token_hash = $1`, hash))
}

// LinkForUpdate locks the invitation row until the transaction ends.
func (r *Repository) LinkForUpdate(ctx context.Context, tx pgx.Tx, hash []byte) (*link, error) {
	return scanLink(tx.QueryRow(ctx, `SELECT `+linkCols+` FROM student_invitations WHERE token_hash = $1 FOR UPDATE`, hash))
}

// MarkAccepted closes the invitation and the other open invitations of the same teacher to the same address.
func (r *Repository) MarkAccepted(ctx context.Context, tx pgx.Tx, l *link, studentID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE student_invitations SET accepted_at = now(), accepted_by = $2 WHERE id = $1`, l.ID, studentID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE student_invitations SET superseded_at = now()
		WHERE teacher_id = $1 AND email = $2 AND id <> $3 AND accepted_at IS NULL AND superseded_at IS NULL`, l.TeacherID, l.Email, l.ID)
	return err
}
