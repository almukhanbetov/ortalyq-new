package passwordreset

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

// LockEmail serialises the requests of one address until the transaction ends (the limits are race-free).
func (r *Repository) LockEmail(ctx context.Context, tx pgx.Tx, email string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('password_resets:' || $1, 0))`, email)
	return err
}

type usage struct {
	ByEmail int
	ByIP    int
	// Seconds until a request is allowed again (<= 0: now): the resend pause of the address and the moments the
	// oldest request of the address / of the IP leaves the hourly window.
	WaitResend, WaitEmailWindow, WaitIPWindow float64
}

func (r *Repository) Usage(ctx context.Context, tx pgx.Tx, email, ip string, window, resendAfter time.Duration) (usage, error) {
	var u usage
	err := tx.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE email = $1),
		count(*) FILTER (WHERE ip = $2),
		COALESCE(EXTRACT(EPOCH FROM max(created_at) FILTER (WHERE email = $1) + make_interval(secs => $4) - now()), 0)::float8,
		COALESCE(EXTRACT(EPOCH FROM min(created_at) FILTER (WHERE email = $1) + make_interval(secs => $3) - now()), 0)::float8,
		COALESCE(EXTRACT(EPOCH FROM min(created_at) FILTER (WHERE ip = $2) + make_interval(secs => $3) - now()), 0)::float8
		FROM password_resets WHERE created_at > now() - make_interval(secs => $3) AND (email = $1 OR ip = $2)`,
		email, ip, window.Seconds(), resendAfter.Seconds()).Scan(&u.ByEmail, &u.ByIP, &u.WaitResend, &u.WaitEmailWindow, &u.WaitIPWindow)
	return u, err
}

// Supersede closes the open links of the account: only the newest letter works.
func (r *Repository) Supersede(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE password_resets SET superseded_at = now()
		WHERE user_id = $1 AND used_at IS NULL AND superseded_at IS NULL`, userID)
	return err
}

// Insert records a request; userID and hash are nil when the address has no active account (no letter).
func (r *Repository) Insert(ctx context.Context, tx pgx.Tx, email, locale string, userID *uuid.UUID, hash []byte, ip string, ttl time.Duration) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `INSERT INTO password_resets (email, locale, user_id, token_hash, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, now() + make_interval(secs => $6)) RETURNING id`,
		email, locale, userID, hash, ip, ttl.Seconds()).Scan(&id)
	return id, err
}

func (r *Repository) SetOutbox(ctx context.Context, tx pgx.Tx, id, outboxID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE password_resets SET outbox_id = $2 WHERE id = $1`, id, outboxID)
	return err
}

const linkCols = `id, email, user_id, expires_at, superseded_at, used_at, expires_at <= now()`

func scanLink(row pgx.Row) (*link, error) {
	var l link
	err := row.Scan(&l.ID, &l.Email, &l.UserID, &l.ExpiresAt, &l.SupersededAt, &l.UsedAt, &l.Expired)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// LinkByHash returns the link of a token (nil when unknown).
func (r *Repository) LinkByHash(ctx context.Context, hash []byte) (*link, error) {
	return scanLink(r.pool.QueryRow(ctx, `SELECT `+linkCols+` FROM password_resets WHERE token_hash = $1`, hash))
}

// LinkForUpdate locks the link row until the transaction ends.
func (r *Repository) LinkForUpdate(ctx context.Context, tx pgx.Tx, hash []byte) (*link, error) {
	return scanLink(tx.QueryRow(ctx, `SELECT `+linkCols+` FROM password_resets WHERE token_hash = $1 FOR UPDATE`, hash))
}

// MarkUsed closes the link and every other open link of the same account.
func (r *Repository) MarkUsed(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE password_resets SET used_at = now() WHERE id = $1`, id); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE password_resets SET superseded_at = now()
		WHERE user_id = $1 AND id <> $2 AND used_at IS NULL AND superseded_at IS NULL`, userID, id)
	return err
}

func (r *Repository) DeleteOld(ctx context.Context, keep time.Duration) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM password_resets WHERE created_at < now() - make_interval(secs => $1)`, keep.Seconds())
	return err
}
