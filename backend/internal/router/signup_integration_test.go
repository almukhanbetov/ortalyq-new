package router_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/mail"
	"ortalyq/backend/internal/mail/mailtest"
)

// Private teacher sign-up: e-mail → one-time link (letter through the outbox to a local SMTP test receiver) →
// password → signed in as private_teacher. Checked: validation, the same answer for new and existing addresses
// (the existing account is not touched), resend limits, one use, expiry, a newer letter closing the older link,
// an account created in the meantime, the rights of the new role and that old logins and /admin still work.

var signupLinkRe = regexp.MustCompile(`http://web\.test/signup/confirm#([A-Za-z0-9_-]{43})`)

type signupEnv struct {
	*env
	sink *mailtest.Server
}

func setupSignup(t *testing.T, o func(*config.Config)) *signupEnv {
	sink, err := mailtest.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sink.Close() })
	e := setup(t, func(c *config.Config) {
		c.MailDriver, c.MailSMTPAddr, c.MailFrom, c.AppURL = "smtp", sink.Addr(), "noreply@bilimway.test", "http://web.test"
		if o != nil {
			o(c)
		}
	})
	return &signupEnv{env: e, sink: sink}
}

func (e *signupEnv) dispatch() {
	e.t.Helper()
	if _, err := e.app.Mail.DispatchOnce(e.t.Context()); err != nil {
		e.t.Fatal(err)
	}
}

// letters returns the bodies of all letters to the address, oldest first.
func (e *signupEnv) letters(to string) []string {
	var out []string
	for _, m := range e.sink.Messages() {
		if m.To == to {
			out = append(out, m.Body())
		}
	}
	return out
}

func (e *signupEnv) lastToken(to string) string {
	e.t.Helper()
	l := e.letters(to)
	for i := len(l) - 1; i >= 0; i-- {
		if m := signupLinkRe.FindStringSubmatch(l[i]); m != nil {
			return m[1]
		}
	}
	e.t.Fatalf("no confirmation letter to %s", to)
	return ""
}

func errCode(t *testing.T, c *client, method, path string, body any, want int, code string) obj {
	t.Helper()
	var out obj
	c.must(method, path, body, want, &out)
	if str(out, "error", "code") != code {
		t.Fatalf("%s %s: want code %s, got %v", method, path, code, out)
	}
	return out
}

func TestSignupResponsesAndLimits(t *testing.T) {
	e := setupSignup(t, nil) // default limits: 60 s between letters to one address
	anon := e.anonymous()

	// Server-side validation.
	for _, bad := range []string{"", "no-at", "a@b", "Иван <a@b.kz>", "a@b.kz, c@d.kz", "a@.kz", strings.Repeat("a", 250) + "@b.kz"} {
		out := errCode(t, anon, "POST", "/api/signup/email", obj{"email": bad, "accept_terms": true}, 422, "validation_failed")
		if str(out, "error", "fields", "email") == "" {
			t.Fatalf("%q: %v", bad, out)
		}
	}
	errCode(t, anon, "POST", "/api/signup/email", obj{"email": "new@test.local"}, 422, "validation_failed")

	// New and existing address: identical status and body.
	st1, b1 := anon.do("POST", "/api/signup/email", obj{"email": "  New@Test.Local ", "accept_terms": true, "locale": "kk"})
	st2, b2 := anon.do("POST", "/api/signup/email", obj{"email": "TEACHER@test.local", "accept_terms": true, "locale": "ru"})
	if st1 != 202 || st1 != st2 || !bytes.Equal(b1, b2) {
		t.Fatalf("answers differ: %d %s / %d %s", st1, b1, st2, b2)
	}
	// Repeated at once: the same 429 for both.
	st1, b1 = anon.do("POST", "/api/signup/email", obj{"email": "new@test.local", "accept_terms": true})
	st2, b2 = anon.do("POST", "/api/signup/email", obj{"email": "teacher@test.local", "accept_terms": true})
	if st1 != 429 || st1 != st2 || !bytes.Equal(b1, b2) {
		t.Fatalf("limit answers differ: %d %s / %d %s", st1, b1, st2, b2)
	}

	e.dispatch()
	newL, oldL := e.letters("new@test.local"), e.letters("teacher@test.local")
	if len(newL) != 1 || !signupLinkRe.MatchString(newL[0]) || !strings.Contains(newL[0], "Сәлеметсіз") {
		t.Fatalf("letter to the new address: %v", newL)
	}
	if len(oldL) != 1 || strings.Contains(oldL[0], "/signup/confirm") || !strings.Contains(oldL[0], "http://web.test/login") {
		t.Fatalf("letter to the existing address must not carry a link: %v", oldL)
	}
	// Bodies are removed from the outbox after sending; only the hash of the token is stored.
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_outbox WHERE kind LIKE 'signup_%' AND body <> $1`, mail.Redacted).Scan(&n); err != nil || n != 0 {
		t.Fatalf("outbox bodies kept: %d %v", n, err)
	}
	tok := e.lastToken("new@test.local")
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_signups WHERE token_hash IS NOT NULL AND position($1 in encode(token_hash, 'escape')) > 0`, tok).Scan(&n); err != nil || n != 0 {
		t.Fatalf("plain token stored: %d %v", n, err)
	}

	// The existing account is unchanged: same role, the old password still works.
	teacher := e.login("teacher")
	var me obj
	teacher.must("GET", "/api/auth/me", nil, 200, &me)
	if str(me, "role") != "teacher" {
		t.Fatalf("role changed: %v", me)
	}
}

func TestSignupFlow(t *testing.T) {
	e := setupSignup(t, func(c *config.Config) {
		c.SignupResendAfter, c.SignupMaxPerEmail, c.SignupMaxPerIP = time.Millisecond, 3, 6
	})
	ctx := t.Context()
	anon := e.anonymous()
	send := func(c *client, email string, want int) {
		t.Helper()
		time.Sleep(5 * time.Millisecond)
		c.must("POST", "/api/signup/email", obj{"email": email, "accept_terms": true, "locale": "ru"}, want, nil)
	}

	// A newer letter closes the older link.
	send(anon, "tutor@test.local", 202)
	e.dispatch()
	first := e.lastToken("tutor@test.local")
	send(anon, "tutor@test.local", 202)
	e.dispatch()
	tok := e.lastToken("tutor@test.local")
	if tok == first {
		t.Fatal("token not rotated")
	}
	errCode(t, anon, "POST", "/api/signup/verify", obj{"token": first}, 410, "signup_link_expired")
	errCode(t, anon, "POST", "/api/signup/verify", obj{"token": strings.Repeat("A", 43)}, 404, "signup_link_invalid")
	errCode(t, anon, "POST", "/api/signup/verify", obj{"token": "short"}, 404, "signup_link_invalid")

	// Hourly limit per address.
	send(anon, "tutor@test.local", 202)
	e.dispatch()
	tok = e.lastToken("tutor@test.local")
	out := errCode(t, anon, "POST", "/api/signup/email", obj{"email": "tutor@test.local", "accept_terms": true}, 429, "rate_limited")
	if str(out, "error", "fields", "retry_after") == "" {
		t.Fatalf("no retry hint: %v", out)
	}

	// The link opens the password form (repeatable until the password is set).
	var info obj
	anon.must("POST", "/api/signup/verify", obj{"token": tok}, 200, &info)
	anon.must("POST", "/api/signup/verify", obj{"token": tok}, 200, &info)
	if str(info, "email") != "tutor@test.local" {
		t.Fatalf("verify: %v", info)
	}
	// Password checked on the server; a rejected password keeps the link usable.
	errCode(t, anon, "POST", "/api/signup/complete", obj{"token": tok, "password": "short"}, 422, "validation_failed")
	errCode(t, anon, "POST", "/api/signup/complete", obj{"token": tok, "password": "tutor@test.local"}, 422, "validation_failed")
	errCode(t, anon, "POST", "/api/signup/complete", obj{"token": tok, "password": strings.Repeat("x", 73)}, 422, "validation_failed")

	var me obj
	anon.must("POST", "/api/signup/complete", obj{"token": tok, "password": "tutor-pass-1"}, 201, &me)
	if str(me, "role") != "private_teacher" || str(me, "email") != "tutor@test.local" {
		t.Fatalf("complete: %v", me)
	}
	anon.must("GET", "/api/auth/me", nil, 200, &me) // signed in by the cookie
	if str(me, "role") != "private_teacher" {
		t.Fatalf("me: %v", me)
	}
	var terms int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = 'tutor@test.local' AND terms_accepted_at IS NOT NULL`).Scan(&terms); err != nil || terms != 1 {
		t.Fatalf("terms_accepted_at: %d %v", terms, err)
	}

	// The link works once.
	other := e.anonymous()
	errCode(t, other, "POST", "/api/signup/verify", obj{"token": tok}, 410, "signup_link_used")
	errCode(t, other, "POST", "/api/signup/complete", obj{"token": tok, "password": "another-pass-1"}, 410, "signup_link_used")
	// Signing in with the new password works; the other password was not set.
	tutor := e.anonymous()
	tutor.must("POST", "/api/auth/login", obj{"email": "tutor@test.local", "password": "another-pass-1"}, 401, nil)
	tutor.must("POST", "/api/auth/login", obj{"email": "TUTOR@test.local", "password": "tutor-pass-1"}, 200, nil)

	// Rights of the private teacher: authoring yes, school part and admin no.
	var course, m obj
	tutor.must("POST", "/api/courses", obj{"title": "Мой курс"}, 201, &course)
	tutor.must("POST", "/api/marathons", obj{"name": "Мой марафон", "direction": "other"}, 201, &m)
	var students obj
	tutor.must("GET", "/api/teacher/students", nil, 200, &students)
	if len(students["items"].([]any)) != 0 {
		t.Fatalf("students: %v", students)
	}
	tutor.must("GET", "/api/admin/users", nil, 403, nil)
	tutor.must("POST", "/api/classrooms", obj{"name": "Класс", "student_id": e.ids["student"]}, 422, nil)
	admin := e.login("admin")
	errCode(t, admin, "POST", "/api/admin/assignments", obj{"teacher_id": str(me, "id"), "student_id": e.ids["student"]}, 422, "validation_failed")
	// An administrator cannot turn a school teacher with students into a private teacher.
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	errCode(t, admin, "PATCH", "/api/admin/users/"+e.ids["teacher"], obj{"role": "private_teacher"}, 422, "validation_failed")
	var list obj
	admin.must("GET", "/api/admin/users?role=private_teacher", nil, 200, &list)
	if num(list, "total") != 1 {
		t.Fatalf("admin list: %v", list)
	}

	// Expired link.
	send(e.anonymous(), "late@test.local", 202)
	e.dispatch()
	late := e.lastToken("late@test.local")
	if _, err := e.pool.Exec(ctx, `UPDATE email_signups SET expires_at = now() - interval '1 second' WHERE email = 'late@test.local'`); err != nil {
		t.Fatal(err)
	}
	errCode(t, anon, "POST", "/api/signup/verify", obj{"token": late}, 410, "signup_link_expired")
	errCode(t, anon, "POST", "/api/signup/complete", obj{"token": late, "password": "late-pass-123"}, 410, "signup_link_expired")

	// An account created for the address after the letter: the link is closed, that account is not changed.
	send(e.anonymous(), "race@test.local", 202)
	e.dispatch()
	race := e.lastToken("race@test.local")
	admin.must("POST", "/api/admin/users", obj{"email": "race@test.local", "full_name": "Race", "role": "student", "password": "admin-set-pass"}, 201, nil)
	errCode(t, anon, "POST", "/api/signup/complete", obj{"token": race, "password": "attacker-pass-1"}, 410, "signup_link_used")
	r := e.anonymous()
	r.must("POST", "/api/auth/login", obj{"email": "race@test.local", "password": "admin-set-pass"}, 200, &me)
	if str(me, "role") != "student" {
		t.Fatalf("race account changed: %v", me)
	}

	// Hourly limit per IP (all test requests come from 127.0.0.1): 5 letters so far, the 6th passes, the 7th not.
	send(e.anonymous(), "ip1@test.local", 202)
	send(e.anonymous(), "ip2@test.local", 429)

	// Old flows still work: existing logins and /admin.
	e.login("teacher").must("GET", "/api/auth/me", nil, 200, nil)
	e.login("student").must("GET", "/api/admin/users", nil, 403, nil)
	admin.must("GET", "/api/admin/users", nil, 200, nil)
}
