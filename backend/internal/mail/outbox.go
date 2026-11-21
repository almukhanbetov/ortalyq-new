package mail

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// Redacted replaces the body of a message once it is sent or has failed for good: the outbox keeps who, what and
// when, but not one-time links.
const Redacted = "[текст письма удалён после отправки]"

// Dispatcher owns email_outbox: Enqueue inside the caller's transaction, DispatchOnce / Run to send after commit.
type Dispatcher struct {
	pool        *pgxpool.Pool
	sender      Sender
	MaxAttempts int
	kick        chan struct{}
}

func NewDispatcher(pool *pgxpool.Pool, sender Sender) *Dispatcher {
	return &Dispatcher{pool: pool, sender: sender, MaxAttempts: 5, kick: make(chan struct{}, 1)}
}

// Enqueue records a message to send; kind and refID say what it is about (e.g. a marathon invitation).
func (d *Dispatcher) Enqueue(ctx context.Context, tx pgx.Tx, kind string, refID uuid.UUID, locale string, m Message) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `INSERT INTO email_outbox (kind, ref_id, to_email, locale, subject, body) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		kind, refID, m.To, locale, m.Subject, m.Text).Scan(&id)
	return id, err
}

// Kick asks a running dispatcher to send now (call it after the commit). Never blocks.
func (d *Dispatcher) Kick() {
	select {
	case d.kick <- struct{}{}:
	default:
	}
}

// Run sends queued messages on every kick and every interval until ctx ends.
func (d *Dispatcher) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, err := d.DispatchOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("mail dispatch", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-d.kick:
		}
	}
}

type queued struct {
	id       uuid.UUID
	attempts int
	msg      Message
}

// DispatchOnce sends the messages that are due, one transaction per message (rows locked with SKIP LOCKED, so two
// dispatchers never send the same message twice). A failure is retried with a growing delay; after MaxAttempts
// the message is "failed". Returns how many were sent.
func (d *Dispatcher) DispatchOnce(ctx context.Context) (int, error) {
	sent := 0
	for i := 0; i < 50; i++ {
		done := false
		err := database.WithTx(ctx, d.pool, func(tx pgx.Tx) error {
			var q queued
			err := tx.QueryRow(ctx, `SELECT id, attempts, to_email, subject, body FROM email_outbox
				WHERE status = 'queued' AND next_attempt_at <= now() ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`).
				Scan(&q.id, &q.attempts, &q.msg.To, &q.msg.Subject, &q.msg.Text)
			if database.IsNoRows(err) {
				done = true
				return nil
			}
			if err != nil {
				return err
			}
			pid, sendErr := d.sender.Send(ctx, q.msg)
			if sendErr == nil {
				sent++
				_, err = tx.Exec(ctx, `UPDATE email_outbox SET status = 'sent', attempts = attempts + 1, provider_id = $2, sent_at = now(),
					last_error = NULL, body = $3, updated_at = now() WHERE id = $1`, q.id, pid, Redacted)
				return err
			}
			attempts := q.attempts + 1
			if attempts >= d.MaxAttempts {
				_, err = tx.Exec(ctx, `UPDATE email_outbox SET status = 'failed', attempts = $2, last_error = $3, body = $4, updated_at = now()
					WHERE id = $1`, q.id, attempts, truncate(sendErr.Error()), Redacted)
				return err
			}
			delay := time.Duration(attempts*attempts) * 30 * time.Second
			_, err = tx.Exec(ctx, `UPDATE email_outbox SET attempts = $2, last_error = $3, next_attempt_at = now() + make_interval(secs => $4),
				updated_at = now() WHERE id = $1`, q.id, attempts, truncate(sendErr.Error()), delay.Seconds())
			return err
		})
		if err != nil {
			return sent, err
		}
		if done {
			break
		}
	}
	return sent, nil
}

func truncate(s string) string {
	if len(s) > 500 {
		return s[:500]
	}
	return s
}
