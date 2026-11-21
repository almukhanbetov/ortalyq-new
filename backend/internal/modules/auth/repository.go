package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/database"
)

type Repository struct{ db database.Querier }

func NewRepository(db database.Querier) *Repository { return &Repository{db: db} }

const principalCols = `u.id, u.role, u.email, u.full_name, u.locale, u.timezone, u.avatar_color`

func (r *Repository) UserByEmail(ctx context.Context, email string) (*userRecord, error) {
	var u userRecord
	err := r.db.QueryRow(ctx, `SELECT `+principalCols+`, u.password_hash, u.disabled_at
		FROM users u WHERE lower(u.email) = lower($1)`, email).
		Scan(&u.UserID, &u.Role, &u.Email, &u.FullName, &u.Locale, &u.Timezone, &u.AvatarColor, &u.PasswordHash, &u.DisabledAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CountFailures(ctx context.Context, email, ip string, since time.Time) (byEmail, byIP int, err error) {
	err = r.db.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE lower(email) = lower($1)),
		count(*) FILTER (WHERE ip = $2)
		FROM login_attempts WHERE success = false AND attempted_at > $3 AND (lower(email) = lower($1) OR ip = $2)`,
		email, ip, since).Scan(&byEmail, &byIP)
	return
}

func (r *Repository) RecordAttempt(ctx context.Context, email, ip string, success bool) error {
	_, err := r.db.Exec(ctx, `INSERT INTO login_attempts (email, ip, success) VALUES ($1, $2, $3)`, email, ip, success)
	return err
}

func (r *Repository) ClearFailures(ctx context.Context, email string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM login_attempts WHERE lower(email) = lower($1) AND success = false`, email)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, hash []byte, userID uuid.UUID, expires time.Time, ua, ip string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at, user_agent, ip) VALUES ($1, $2, $3, $4, $5)`,
		hash, userID, expires, ua, ip)
	return err
}

type sessionRow struct {
	Principal  Principal
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

func (r *Repository) SessionPrincipal(ctx context.Context, hash []byte) (*sessionRow, error) {
	var s sessionRow
	p := &s.Principal
	err := r.db.QueryRow(ctx, `SELECT `+principalCols+`, s.last_seen_at, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.disabled_at IS NULL`, hash).
		Scan(&p.UserID, &p.Role, &p.Email, &p.FullName, &p.Locale, &p.Timezone, &p.AvatarColor, &s.LastSeenAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) TouchSession(ctx context.Context, hash []byte, expires time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET last_seen_at = now(), expires_at = $2 WHERE token_hash = $1`, hash, expires)
	return err
}

func (r *Repository) DeleteSession(ctx context.Context, hash []byte) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash)
	return err
}

func (r *Repository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

func (r *Repository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	if err == nil {
		_, err = r.db.Exec(ctx, `DELETE FROM login_attempts WHERE attempted_at < now() - interval '1 day'`)
	}
	return err
}

func (r *Repository) UpdatePrefs(ctx context.Context, userID uuid.UUID, locale, tz *string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET locale = COALESCE($2, locale), timezone = COALESCE($3, timezone), updated_at = now() WHERE id = $1`,
		userID, locale, tz)
	return err
}
