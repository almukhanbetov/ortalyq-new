package signup

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

// LockEmail serialises the letters of one address until the transaction ends (the limits are race-free).
func (r *Repository) LockEmail(ctx context.Context, tx pgx.Tx, email string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('email_signups:' || $1, 0))`, email)
	return err
}

type usage struct {
	ByEmail int
	ByIP    int
	// Seconds until a letter is allowed again (<= 0: now): the resend pause of the address and the moments the
	// oldest letter of the address / of the IP leaves the hourly window.
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
		FROM email_signups WHERE created_at > now() - make_interval(secs => $3) AND (email = $1 OR ip = $2)`,
		email, ip, window.Seconds(), resendAfter.Seconds()).Scan(&u.ByEmail, &u.ByIP, &u.WaitResend, &u.WaitEmailWindow, &u.WaitIPWindow)
	return u, err
}

// Supersede closes the open links of the address: only the newest letter works.
func (r *Repository) Supersede(ctx context.Context, tx pgx.Tx, email string) error {
	_, err := tx.Exec(ctx, `UPDATE email_signups SET superseded_at = now()
		WHERE email = $1 AND token_hash IS NOT NULL AND used_at IS NULL AND superseded_at IS NULL`, email)
	return err
}

// Insert records a letter; hash is nil for an address that already has an account.
func (r *Repository) Insert(ctx context.Context, tx pgx.Tx, email, locale string, hash []byte, ip string, ttl time.Duration) (uuid.UUID, time.Time, error) {
	var id uuid.UUID
	var exp time.Time
	err := tx.QueryRow(ctx, `INSERT INTO email_signups (email, locale, token_hash, ip, terms_accepted_at, expires_at)
		VALUES ($1, $2, $3, $4, now(), now() + make_interval(secs => $5)) RETURNING id, expires_at`,
		email, locale, hash, ip, ttl.Seconds()).Scan(&id, &exp)
	return id, exp, err
}

func (r *Repository) SetOutbox(ctx context.Context, tx pgx.Tx, id, outboxID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE email_signups SET outbox_id = $2 WHERE id = $1`, id, outboxID)
	return err
}

const linkCols = `id, email, locale, terms_accepted_at, expires_at, superseded_at, used_at, expires_at <= now()`

func scanLink(row pgx.Row) (*link, error) {
	var l link
	err := row.Scan(&l.ID, &l.Email, &l.Locale, &l.TermsAt, &l.ExpiresAt, &l.SupersededAt, &l.UsedAt, &l.Expired)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// LinkByHash returns the link of a token (nil when unknown). q may be a transaction.
func (r *Repository) LinkByHash(ctx context.Context, q database.Querier, hash []byte) (*link, error) {
	return scanLink(q.QueryRow(ctx, `SELECT `+linkCols+` FROM email_signups WHERE token_hash = $1`, hash))
}

// LinkForUpdate locks the link row until the transaction ends.
func (r *Repository) LinkForUpdate(ctx context.Context, tx pgx.Tx, hash []byte) (*link, error) {
	return scanLink(tx.QueryRow(ctx, `SELECT `+linkCols+` FROM email_signups WHERE token_hash = $1 FOR UPDATE`, hash))
}

func (r *Repository) MarkVerified(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE email_signups SET verified_at = COALESCE(verified_at, now()) WHERE id = $1`, id)
	return err
}

// MarkUsed closes the link for good; userID is nil when no account was created by it.
func (r *Repository) MarkUsed(ctx context.Context, tx pgx.Tx, id uuid.UUID, userID *uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE email_signups SET used_at = now(), verified_at = COALESCE(verified_at, now()), user_id = $2
		WHERE id = $1`, id, userID)
	return err
}

func (r *Repository) DeleteOld(ctx context.Context, keep time.Duration) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM email_signups WHERE created_at < now() - make_interval(secs => $1)`, keep.Seconds())
	return err
}
