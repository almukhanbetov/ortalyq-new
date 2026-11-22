package router_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/mail"
)

// Password reset by e-mail: request → one-time link (letter through the outbox to a local SMTP test receiver) →
// new password → sign in again. Checked: the same answer and the same limits for existing, unknown and disabled
// addresses (a letter only to the active account), the new password works and the old one does not, every old
// session ends, failed sign-in attempts are cleared, the account keeps its id, role and rights, the link works once,
// expires and is closed by a newer letter, and only the hash of the token is stored.

var resetLinkRe = regexp.MustCompile(`http://web\.test/reset-password#([A-Za-z0-9_-]{43})`)

func (e *signupEnv) lastResetToken(to string) string {
	e.t.Helper()
	l := e.letters(to)
	for i := len(l) - 1; i >= 0; i-- {
		if m := resetLinkRe.FindStringSubmatch(l[i]); m != nil {
			return m[1]
		}
	}
	e.t.Fatalf("no reset letter to %s", to)
	return ""
}

func TestPasswordResetFlow(t *testing.T) {
	e := setupSignup(t, nil) // default limits: 60 s between letters to one address
	anon := e.anonymous()
	ctx := t.Context()

	// Server-side validation of the address.
	for _, bad := range []string{"", "no-at", "a@b", "Иван <a@b.kz>"} {
		out := errCode(t, anon, "POST", "/api/password-reset/request", obj{"email": bad}, 422, "validation_failed")
		if str(out, "error", "fields", "email") == "" {
			t.Fatalf("%q: %v", bad, out)
		}
	}

	// Before the reset: an open session, the role and id of the account, and a sign-in lock after failed attempts.
	old := e.login("teacher")
	var before obj
	old.must("GET", "/api/auth/me", nil, 200, &before)
	if _, err := e.pool.Exec(ctx, `UPDATE users SET disabled_at = now() WHERE email = 'teacher2@test.local'`); err != nil {
		t.Fatal(err)
	}
	guess := e.anonymous()
	for i := 0; i < 5; i++ {
		guess.must("POST", "/api/auth/login", obj{"email": "teacher@test.local", "password": "wrong-guess"}, 401, nil)
	}
	guess.must("POST", "/api/auth/login", obj{"email": "teacher@test.local", "password": "password-123"}, 429, nil)

	// Existing, unknown and disabled addresses: identical status and body; again at once: the same 429.
	var bodies [][]byte
	for _, to := range []string{" Teacher@Test.Local ", "nobody@test.local", "teacher2@test.local"} {
		st, b := anon.do("POST", "/api/password-reset/request", obj{"email": to, "locale": "kk"})
		if st != 202 {
			t.Fatalf("%s: %d %s", to, st, b)
		}
		bodies = append(bodies, b)
	}
	if !bytes.Equal(bodies[0], bodies[1]) || !bytes.Equal(bodies[0], bodies[2]) {
		t.Fatalf("answers differ: %s / %s / %s", bodies[0], bodies[1], bodies[2])
	}
	st1, b1 := anon.do("POST", "/api/password-reset/request", obj{"email": "teacher@test.local"})
	st2, b2 := anon.do("POST", "/api/password-reset/request", obj{"email": "nobody@test.local"})
	if st1 != 429 || st1 != st2 || !bytes.Equal(b1, b2) {
		t.Fatalf("limit answers differ: %d %s / %d %s", st1, b1, st2, b2)
	}

	// A letter only to the active account; the body leaves the outbox once sent; only the hash is stored.
	e.dispatch()
	if l := e.letters("teacher@test.local"); len(l) != 1 || !resetLinkRe.MatchString(l[0]) || !strings.Contains(l[0], "Сәлеметсіз") {
		t.Fatalf("letter to the account: %v", l)
	}
	if n := len(e.letters("nobody@test.local")) + len(e.letters("teacher2@test.local")); n != 0 {
		t.Fatalf("letters to unknown/disabled addresses: %d", n)
	}
	token := e.lastResetToken("teacher@test.local")
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE kind = 'password_reset' AND body <> $1`, mail.Redacted).Scan(&n); err != nil || n != 0 {
		t.Fatalf("outbox bodies kept: %d %v", n, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM password_resets WHERE token_hash = convert_to($1, 'UTF8')
		OR encode(token_hash, 'escape') LIKE '%' || $1 || '%'`, token).Scan(&n); err != nil || n != 0 {
		t.Fatalf("plain token stored: %d %v", n, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM password_resets WHERE token_hash IS NOT NULL AND expires_at > now() + interval '59 minutes'
		AND expires_at <= now() + interval '60 minutes'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("link must expire in an hour: %d %v", n, err)
	}

	// Link checks: malformed and unknown tokens are invalid; the real one shows the address.
	errCode(t, anon, "POST", "/api/password-reset/verify", obj{"token": "short"}, 404, "reset_link_invalid")
	errCode(t, anon, "POST", "/api/password-reset/verify", obj{"token": strings.Repeat("A", 43)}, 404, "reset_link_invalid")
	var info obj
	anon.must("POST", "/api/password-reset/verify", obj{"token": token}, 200, &info)
	if str(info, "email") != "teacher@test.local" {
		t.Fatalf("verify: %v", info)
	}

	// Password policy is the same as everywhere.
	errCode(t, anon, "POST", "/api/password-reset/complete", obj{"token": token, "password": "short"}, 422, "validation_failed")
	errCode(t, anon, "POST", "/api/password-reset/complete", obj{"token": token, "password": "Teacher@test.local"}, 422, "validation_failed")
	errCode(t, anon, "POST", "/api/password-reset/complete", obj{"token": token, "password": strings.Repeat("x", 73)}, 422, "validation_failed")

	// Success: no session is started; the old session ends; the old password fails; the new one works.
	anon.must("POST", "/api/password-reset/complete", obj{"token": token, "password": "new-pass-456"}, 204, nil)
	anon.must("GET", "/api/auth/me", nil, 401, nil)
	old.must("GET", "/api/auth/me", nil, 401, nil)
	c := e.anonymous()
	c.must("POST", "/api/auth/login", obj{"email": "teacher@test.local", "password": "password-123"}, 401, nil)
	var after obj
	c.must("POST", "/api/auth/login", obj{"email": "teacher@test.local", "password": "new-pass-456"}, 200, &after)
	if str(after, "id") != str(before, "id") || str(after, "role") != "teacher" || str(after, "role") != str(before, "role") {
		t.Fatalf("account changed: %v → %v", before, after)
	}
	// Rights stay: the school teacher still reaches teacher-only routes and keeps the assigned-students route.
	c.must("GET", "/api/courses", nil, 200, nil)
	c.must("GET", "/api/teacher/students", nil, 200, nil)

	// The link works once.
	errCode(t, anon, "POST", "/api/password-reset/verify", obj{"token": token}, 410, "reset_link_used")
	errCode(t, anon, "POST", "/api/password-reset/complete", obj{"token": token, "password": "again-pass-789"}, 410, "reset_link_used")
	e.login("admin").must("GET", "/api/admin/users", nil, 200, nil)
}

func TestPasswordResetExpiryAndSupersede(t *testing.T) {
	e := setupSignup(t, func(c *config.Config) { c.ResetResendAfter = time.Millisecond })
	anon := e.anonymous()
	ctx := t.Context()

	// A newer letter closes the older link.
	anon.must("POST", "/api/password-reset/request", obj{"email": "student@test.local"}, 202, nil)
	e.dispatch()
	first := e.lastResetToken("student@test.local")
	time.Sleep(5 * time.Millisecond)
	anon.must("POST", "/api/password-reset/request", obj{"email": "student@test.local"}, 202, nil)
	e.dispatch()
	second := e.lastResetToken("student@test.local")
	if first == second {
		t.Fatal("each letter must carry its own link")
	}
	errCode(t, anon, "POST", "/api/password-reset/verify", obj{"token": first}, 410, "reset_link_expired")
	errCode(t, anon, "POST", "/api/password-reset/complete", obj{"token": first, "password": "new-pass-456"}, 410, "reset_link_expired")

	// An expired link is refused and changes nothing.
	if _, err := e.pool.Exec(ctx, `UPDATE password_resets SET expires_at = now() - interval '1 second' WHERE email = 'student@test.local'`); err != nil {
		t.Fatal(err)
	}
	errCode(t, anon, "POST", "/api/password-reset/verify", obj{"token": second}, 410, "reset_link_expired")
	errCode(t, anon, "POST", "/api/password-reset/complete", obj{"token": second, "password": "new-pass-456"}, 410, "reset_link_expired")
	e.login("student") // the old password still works

	// Hourly limit per address: 5 requests, then 429 (also for an unknown address).
	for _, to := range []string{"student2@test.local", "ghost@test.local"} {
		for i := 0; i < 5; i++ {
			time.Sleep(2 * time.Millisecond)
			anon.must("POST", "/api/password-reset/request", obj{"email": to}, 202, nil)
		}
		time.Sleep(2 * time.Millisecond)
		out := errCode(t, anon, "POST", "/api/password-reset/request", obj{"email": to}, 429, "rate_limited")
		if str(out, "error", "fields", "retry_after") == "" {
			t.Fatalf("%s: no retry_after: %v", to, out)
		}
	}
}
