package router_test

import (
	"regexp"
	"strings"
	"testing"
)

// A private teacher invites students by e-mail (letter through the outbox to a local SMTP test receiver). Checked:
// only a private teacher invites; validation and the resend pause; a new address creates a student account from the
// link and signs in; an existing student accepts after signing in with the invited address (no second account); the
// wrong account and a non-student account are refused; one use, expiry and a newer letter closing the older link;
// only the hash of the token is stored. Then the class: a private teacher may choose only students linked to them,
// the student sees the class, another private teacher cannot choose the student nor open the class, and the school
// assignment by the administrator works as before.

var inviteLinkRe = regexp.MustCompile(`http://web\.test/student-invite#([A-Za-z0-9_-]{43})`)

func (e *signupEnv) lastInviteToken(to string) string {
	e.t.Helper()
	l := e.letters(to)
	for i := len(l) - 1; i >= 0; i-- {
		if m := inviteLinkRe.FindStringSubmatch(l[i]); m != nil {
			return m[1]
		}
	}
	e.t.Fatalf("no invitation letter to %s", to)
	return ""
}

func TestStudentInvitations(t *testing.T) {
	e := setupSignup(t, nil)
	ctx := t.Context()
	for _, n := range []string{"tutor", "tutor2"} {
		var id string
		if err := e.pool.QueryRow(ctx, `INSERT INTO users (email, full_name, role, password_hash)
			SELECT $1, $2, 'private_teacher', password_hash FROM users WHERE email = 'teacher@test.local' RETURNING id::text`,
			n+"@test.local", "Tutor "+n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		e.ids[n] = id
	}
	tutor, tutor2 := e.login("tutor"), e.login("tutor2")
	anon := e.anonymous()
	invite := func(c *client, email string, want int) obj {
		t.Helper()
		var out obj
		c.must("POST", "/api/teacher/invitations", obj{"email": email, "locale": "ru"}, want, &out)
		return out
	}

	// Only a private teacher invites; the school order stays with the administrator.
	invite(e.login("teacher"), "fresh@test.local", 403)
	invite(e.login("student"), "fresh@test.local", 403)
	anon.must("POST", "/api/teacher/invitations", obj{"email": "fresh@test.local"}, 401, nil)
	for _, bad := range []string{"", "no-at", "Иван <a@b.kz>", "Tutor@Test.Local"} {
		if str(invite(tutor, bad, 422), "error", "fields", "email") == "" {
			t.Fatalf("%q accepted", bad)
		}
	}

	// Before any invitation the teacher has no students and cannot create a class with someone else's student.
	var list obj
	tutor.must("GET", "/api/teacher/students", nil, 200, &list)
	if n := len(get(list, "items").([]any)); n != 0 {
		t.Fatalf("students before invitations: %d", n)
	}
	out := errCode(t, tutor, "POST", "/api/classrooms", obj{"name": "X", "student_id": e.ids["student"]}, 422, "validation_failed")
	if !strings.Contains(str(out, "error", "fields", "student_id"), "приглашение") {
		t.Fatalf("private teacher hint: %v", out)
	}

	// A new address: letter, pause, only the hash stored.
	res := invite(tutor, " Fresh@Test.Local ", 202)
	if str(res, "email") != "fresh@test.local" || num(res, "resend_after") != 60 {
		t.Fatalf("invite: %v", res)
	}
	if out := errCode(t, tutor, "POST", "/api/teacher/invitations", obj{"email": "fresh@test.local"}, 429, "rate_limited"); str(out, "error", "fields", "retry_after") == "" {
		t.Fatalf("no retry hint: %v", out)
	}
	e.dispatch()
	if l := e.letters("fresh@test.local"); len(l) != 1 || !strings.Contains(l[0], "Tutor tutor") {
		t.Fatalf("letters: %v", l)
	}
	fresh := e.lastInviteToken("fresh@test.local")
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM student_invitations WHERE encode(token_hash, 'escape') LIKE '%' || $1 || '%'`, fresh).Scan(&n); err != nil || n != 0 {
		t.Fatalf("plain token stored: %d %v", n, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM student_invitations WHERE expires_at > now() + interval '6 days 23 hours'
		AND expires_at <= now() + interval '7 days'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("link must expire in 7 days: %d %v", n, err)
	}

	// The link page; the form is checked on the server; the account is created as a student and signed in.
	errCode(t, anon, "POST", "/api/student-invitations/verify", obj{"token": "short"}, 404, "student_invite_invalid")
	errCode(t, anon, "POST", "/api/student-invitations/verify", obj{"token": strings.Repeat("A", 43)}, 404, "student_invite_invalid")
	var info obj
	anon.must("POST", "/api/student-invitations/verify", obj{"token": fresh}, 200, &info)
	if str(info, "email") != "fresh@test.local" || str(info, "teacher_name") != "Tutor tutor" || get(info, "has_account") != false {
		t.Fatalf("verify: %v", info)
	}
	reg := obj{"token": fresh, "full_name": "  Асель   Ученица ", "password": "student-pass-1", "accept_terms": true}
	bad := func(k string, v any) obj { m := obj{}; for a, b := range reg { m[a] = b }; m[k] = v; return m }
	for k, v := range map[string]any{"full_name": " ", "password": "short", "accept_terms": false} {
		if out := errCode(t, anon, "POST", "/api/student-invitations/register", bad(k, v), 422, "validation_failed"); str(out, "error", "fields", k) == "" {
			t.Fatalf("%s: %v", k, out)
		}
	}
	newbie := e.anonymous()
	var me obj
	newbie.must("POST", "/api/student-invitations/register", reg, 201, &me)
	if str(me, "role") != "student" || str(me, "email") != "fresh@test.local" || str(me, "full_name") != "Асель Ученица" {
		t.Fatalf("register: %v", me)
	}
	newbie.must("GET", "/api/auth/me", nil, 200, nil)
	errCode(t, anon, "POST", "/api/student-invitations/verify", obj{"token": fresh}, 410, "student_invite_used")
	errCode(t, e.anonymous(), "POST", "/api/student-invitations/register", reg, 410, "student_invite_used")
	freshID := str(me, "id")

	// The student is in this teacher's list only; a class with them can be created by this teacher only.
	tutor.must("GET", "/api/teacher/students", nil, 200, &list)
	if items := get(list, "items").([]any); len(items) != 1 || str(items[0], "id") != freshID {
		t.Fatalf("tutor students: %v", list)
	}
	tutor2.must("GET", "/api/teacher/students", nil, 200, &list)
	if n := len(get(list, "items").([]any)); n != 0 {
		t.Fatalf("tutor2 students: %d", n)
	}
	errCode(t, tutor2, "POST", "/api/classrooms", obj{"name": "Чужой", "student_id": freshID}, 422, "validation_failed")
	var class obj
	tutor.must("POST", "/api/classrooms", obj{"name": "Английский", "student_id": freshID, "lesson_type": "individual"}, 201, &class)
	cid := str(class, "id")
	var mine obj
	newbie.must("GET", "/api/classrooms", nil, 200, &mine)
	if items := get(mine, "items").([]any); len(items) != 1 || str(items[0], "id") != cid {
		t.Fatalf("student classes: %v", mine)
	}
	newbie.must("GET", "/api/classrooms/"+cid, nil, 200, nil)
	tutor2.must("GET", "/api/classrooms/"+cid, nil, 404, nil)
	tutor2.must("PATCH", "/api/classrooms/"+cid, obj{"name": "Взлом"}, 404, nil)
	invite(tutor, "fresh@test.local", 409) // already in the list

	// An existing student: the page asks to sign in; creating a second account is refused; wrong account refused.
	invite(tutor, "student@test.local", 202)
	invite(tutor, "teacher2@test.local", 202) // the letter does not reveal that this is a teacher account
	e.dispatch()
	tok := e.lastInviteToken("student@test.local")
	anon.must("POST", "/api/student-invitations/verify", obj{"token": tok}, 200, &info)
	if get(info, "has_account") != true {
		t.Fatalf("verify existing: %v", info)
	}
	errCode(t, anon, "POST", "/api/student-invitations/register", obj{"token": tok, "full_name": "Дубль", "password": "dup-pass-123", "accept_terms": true},
		409, "student_invite_account_exists")
	anon.must("POST", "/api/student-invitations/accept", obj{"token": tok}, 401, nil)
	if out := errCode(t, e.login("student2"), "POST", "/api/student-invitations/accept", obj{"token": tok}, 403, "student_invite_wrong_account"); str(out, "error", "fields", "email") != "st***@test.local" {
		t.Fatalf("masked address: %v", out)
	}
	errCode(t, e.login("teacher2"), "POST", "/api/student-invitations/accept", obj{"token": e.lastInviteToken("teacher2@test.local")}, 403, "student_invite_not_student")
	st := e.login("student")
	st.must("POST", "/api/student-invitations/accept", obj{"token": tok}, 200, nil)
	st.must("POST", "/api/student-invitations/accept", obj{"token": tok}, 200, nil) // a repeat is harmless
	errCode(t, e.login("student2"), "POST", "/api/student-invitations/accept", obj{"token": tok}, 410, "student_invite_used")
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(email) = 'student@test.local'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("duplicate account: %d %v", n, err)
	}
	tutor.must("POST", "/api/classrooms", obj{"name": "Математика", "student_id": e.ids["student"]}, 201, nil)

	// Expiry and a newer letter closing the older link.
	invite(tutor, "late@test.local", 202)
	e.dispatch()
	first := e.lastInviteToken("late@test.local")
	if _, err := e.pool.Exec(ctx, `UPDATE student_invitations SET created_at = created_at - interval '2 minutes' WHERE email = 'late@test.local'`); err != nil {
		t.Fatal(err)
	}
	invite(tutor, "late@test.local", 202)
	e.dispatch()
	second := e.lastInviteToken("late@test.local")
	errCode(t, anon, "POST", "/api/student-invitations/verify", obj{"token": first}, 410, "student_invite_expired")
	anon.must("POST", "/api/student-invitations/verify", obj{"token": second}, 200, nil)
	if _, err := e.pool.Exec(ctx, `UPDATE student_invitations SET expires_at = now() - interval '1 second' WHERE email = 'late@test.local'`); err != nil {
		t.Fatal(err)
	}
	errCode(t, anon, "POST", "/api/student-invitations/verify", obj{"token": second}, 410, "student_invite_expired")
	errCode(t, anon, "POST", "/api/student-invitations/register", obj{"token": second, "full_name": "Поздний", "password": "late-pass-123", "accept_terms": true},
		410, "student_invite_expired")

	// The school order is unchanged: the administrator assigns to a school teacher, never to a private one.
	admin := e.login("admin")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	errCode(t, admin, "POST", "/api/admin/assignments", obj{"teacher_id": e.ids["tutor"], "student_id": e.ids["student2"]}, 422, "validation_failed")
	teacher := e.login("teacher")
	out = errCode(t, teacher, "POST", "/api/classrooms", obj{"name": "Y", "student_id": freshID}, 422, "validation_failed")
	if !strings.Contains(str(out, "error", "fields", "student_id"), "администратором") {
		t.Fatalf("school teacher hint: %v", out)
	}
	teacher.must("POST", "/api/classrooms", obj{"name": "Школа", "student_id": e.ids["student2"]}, 201, nil)
}
