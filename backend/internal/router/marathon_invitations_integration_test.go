package router_test

import (
	"crypto/sha256"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"sync"
	"testing"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/mail/mailtest"
)

// Stage 9: invitations by e-mail. Letters go through the outbox to a local SMTP test receiver (nothing leaves the
// machine). Checked: rights and validation, who can be invited (Н6 default), one open invitation per address
// (also under concurrency), the token (only its hash is stored, one use, expiry, rotation on resend, revocation),
// the account check on acceptance, delivery failures with retries, and participants with their progress.
func TestMarathonInvitations(t *testing.T) {
	sink, err := mailtest.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	e := setup(t, func(c *config.Config) {
		c.MailDriver, c.MailSMTPAddr, c.MailFrom, c.AppURL = "smtp", sink.Addr(), "noreply@ortalyq.test", "http://web.test"
	})
	ctx := t.Context()
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2")
	sid := e.assign(admin)
	sid2 := e.ids["student2"]
	anon := e.anonymous()

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон по приглашению", "direction": "other"}, 201, &m)
	id := str(m, "id")
	base := "/api/marathons/" + id
	var course, l, sec obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок"}, 201, &l)
	teacher.must("POST", "/api/lessons/"+str(l, "id")+"/sections", obj{"title": "Раздел"}, 201, &sec)
	teacher.must("POST", base+"/lessons", obj{"lesson_id": str(l, "id")}, 200, nil)

	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	dispatch := func() int {
		t.Helper()
		n, err := e.app.Mail.DispatchOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	linkRe := regexp.MustCompile(`http://web\.test/invite/([A-Za-z0-9_-]{43})`)
	lastToken := func(to string) string {
		t.Helper()
		msgs := sink.Messages()
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].To == to {
				if m := linkRe.FindStringSubmatch(msgs[i].Body()); m != nil {
					return m[1]
				}
			}
		}
		t.Fatalf("no letter to %s", to)
		return ""
	}
	invite := func(emails ...string) []any {
		t.Helper()
		var out obj
		teacher.must("POST", base+"/invitations", obj{"emails": emails, "locale": "ru"}, 200, &out)
		return out["items"].([]any)
	}
	code := func(c *client, method, path string, want int, wantCode string) {
		t.Helper()
		var out obj
		c.must(method, path, nil, want, &out)
		if str(out, "error", "code") != wantCode {
			t.Fatalf("%s %s: want %s, got %v", method, path, wantCode, out)
		}
	}

	// ---- Rights and validation ----
	teacher2.must("POST", base+"/invitations", obj{"emails": []string{"student@test.local"}}, 404, nil)
	student.must("POST", base+"/invitations", obj{"emails": []string{"student@test.local"}}, 403, nil)
	admin.must("POST", base+"/invitations", obj{"emails": []string{"student@test.local"}}, 403, nil)
	teacher2.must("GET", base+"/invitations", nil, 404, nil)
	teacher.must("POST", base+"/invitations", obj{"emails": []string{}}, 422, nil)
	teacher.must("POST", base+"/invitations", obj{"emails": []string{"student@test.local"}, "locale": "de"}, 422, nil)
	many := make([]string, 21)
	for i := range many {
		many[i] = strings.Repeat("a", i+1) + "@x.kz"
	}
	teacher.must("POST", base+"/invitations", obj{"emails": many}, 422, nil)

	// ---- Who can be invited: only students assigned to the teacher (Н6 default) ----
	res := invite("  STUDENT@test.local ", "nobody@test.local", "teacher2@test.local", "student2@test.local", "not-an-email")
	want := []string{"invited", "unavailable", "unavailable", "unavailable", "invalid"}
	for i, w := range want {
		if str(res[i], "result") != w {
			t.Fatalf("invite results: %v", res)
		}
	}
	if str(res[0], "email") != "student@test.local" {
		t.Fatalf("address not normalised: %v", res[0])
	}
	if n := count(`SELECT count(*) FROM email_outbox WHERE status = 'queued'`); n != 1 {
		t.Fatalf("outbox before dispatch: %d", n)
	}
	if dispatch() != 1 || len(sink.Messages()) != 1 {
		t.Fatalf("letters: %v", sink.Messages())
	}
	token := lastToken("student@test.local")
	letter := sink.Messages()[0].Body()
	if !strings.Contains(letter, "Марафон по приглашению") || !strings.Contains(letter, "Student") || !strings.Contains(letter, "Teacher") {
		t.Fatalf("letter: %s", letter)
	}
	// Only the hash is stored; the outbox body is redacted after sending.
	h := sha256.Sum256([]byte(token))
	if count(`SELECT count(*) FROM marathon_invitations WHERE token_hash = $1`, h[:]) != 1 ||
		count(`SELECT count(*) FROM email_outbox WHERE body LIKE '%' || $1 || '%'`, token) != 0 ||
		count(`SELECT count(*) FROM email_outbox WHERE status = 'sent' AND provider_id IS NOT NULL`) != 1 {
		t.Fatal("token storage or outbox state is wrong")
	}

	// ---- Repeated invitation: no second row, no second letter (also two at once) ----
	if r := invite("student@test.local"); str(r[0], "result") != "already_invited" {
		t.Fatalf("repeat: %v", r)
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			teacher.do("POST", base+"/invitations", obj{"emails": []string{"student@test.local"}})
		}()
	}
	wg.Wait()
	if count(`SELECT count(*) FROM marathon_invitations`) != 1 || count(`SELECT count(*) FROM email_outbox`) != 1 {
		t.Fatal("repeated invitations created rows")
	}

	// ---- Public page of the link ----
	var info obj
	anon.must("GET", "/api/invitations/"+token, nil, 200, &info)
	if str(info, "marathon_name") != "Марафон по приглашению" || str(info, "email_masked") != "st***@test.local" || str(info, "status") != "pending" {
		t.Fatalf("info: %v", info)
	}
	anon.must("GET", "/api/invitations/short", nil, 404, nil)
	anon.must("GET", "/api/invitations/"+strings.Repeat("A", 43), nil, 404, nil)

	// ---- Acceptance: signed in, the invited student only ----
	anon.must("POST", "/api/invitations/"+token+"/accept", nil, 401, nil)
	code(teacher, "POST", "/api/invitations/"+token+"/accept", 403, "invitation_wrong_account")
	code(student2, "POST", "/api/invitations/"+token+"/accept", 403, "invitation_wrong_account")
	student.must("GET", base+"/me", nil, 404, nil) // not a participant yet
	var acc obj
	student.must("POST", "/api/invitations/"+token+"/accept", nil, 200, &acc)
	if str(acc, "marathon_id") != id {
		t.Fatalf("accepted: %v", acc)
	}
	student.must("POST", "/api/invitations/"+token+"/accept", nil, 200, nil) // the same student again: harmless
	code(student2, "POST", "/api/invitations/"+token+"/accept", 409, "invitation_used")
	student.must("GET", base+"/me", nil, 200, nil)
	var list obj
	teacher.must("GET", base+"/invitations", nil, 200, &list)
	if str(list, "items", 0, "status") != "accepted" || str(list, "items", 0, "delivery") != "sent" {
		t.Fatalf("invitation list: %v", list)
	}
	if r := invite("student@test.local"); str(r[0], "result") != "already_participant" {
		t.Fatalf("participant invited again: %v", r)
	}

	// ---- Progress survives removal and a new invitation ----
	student.must("POST", base+"/lessons/"+str(l, "id")+"/complete", nil, 200, nil)
	teacher.must("DELETE", base+"/students/"+sid, nil, 200, nil)
	if r := invite("student@test.local"); str(r[0], "result") != "invited" {
		t.Fatalf("invite a removed student: %v", r)
	}
	dispatch()
	token2 := lastToken("student@test.local")
	if token2 == token {
		t.Fatal("a new invitation reused the token")
	}
	student.must("POST", "/api/invitations/"+token+"/accept", nil, 200, nil) // the first, already accepted link stays harmless for its owner
	student.must("GET", base+"/me", nil, 404, nil)                           // …and did not re-enrol the removed student
	student.must("POST", "/api/invitations/"+token2+"/accept", nil, 200, nil)
	var me obj
	student.must("GET", base+"/me", nil, 200, &me)
	if num(me, "completed_lessons") != 1 {
		t.Fatalf("progress lost: %v", me)
	}

	// ---- Expiry, renewal and the old link ----
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": sid2}, 204, nil)
	invite("student2@test.local")
	dispatch()
	t2 := lastToken("student2@test.local")
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_invitations SET expires_at = now() - interval '1 second' WHERE email = 'student2@test.local'`); err != nil {
		t.Fatal(err)
	}
	code(student2, "POST", "/api/invitations/"+t2+"/accept", 410, "invitation_expired")
	anon.must("GET", "/api/invitations/"+t2, nil, 200, &info)
	if str(info, "status") != "expired" {
		t.Fatalf("expired info: %v", info)
	}
	teacher.must("GET", base+"/invitations", nil, 200, &list)
	if str(list, "items", 0, "status") != "expired" {
		t.Fatalf("list after expiry: %v", list["items"])
	}
	if r := invite("student2@test.local"); str(r[0], "result") != "renewed" {
		t.Fatalf("renew: %v", r)
	}
	dispatch()
	t3 := lastToken("student2@test.local")
	anon.must("GET", "/api/invitations/"+t2, nil, 404, nil) // the old link stopped working

	// ---- Resend rotates the token; revocation closes the link ----
	teacher.must("GET", base+"/invitations", nil, 200, &list)
	iid := str(list, "items", 0, "id")
	teacher2.must("POST", base+"/invitations/"+iid+"/resend", nil, 404, nil)
	teacher.must("POST", base+"/invitations/"+iid+"/resend", nil, 200, &list)
	if num(list, "items", 0, "sends") != 3 {
		t.Fatalf("sends: %v", list["items"])
	}
	dispatch()
	t4 := lastToken("student2@test.local")
	anon.must("GET", "/api/invitations/"+t3, nil, 404, nil)
	teacher2.must("DELETE", base+"/invitations/"+iid, nil, 404, nil)
	teacher.must("DELETE", base+"/invitations/"+iid, nil, 200, nil)
	teacher.must("DELETE", base+"/invitations/"+iid, nil, 200, nil) // repeat
	code(student2, "POST", "/api/invitations/"+t4+"/accept", 410, "invitation_revoked")
	code(teacher, "POST", base+"/invitations/"+iid+"/resend", 409, "invitation_closed")

	// ---- A student no longer assigned to the teacher cannot accept ----
	if r := invite("student2@test.local"); str(r[0], "result") != "invited" {
		t.Fatalf("invite after revocation: %v", r)
	}
	dispatch()
	t5 := lastToken("student2@test.local")
	admin.must("DELETE", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": sid2}, 204, nil)
	code(student2, "POST", "/api/invitations/"+t5+"/accept", 403, "invitation_unavailable")
	student2.must("GET", base+"/me", nil, 404, nil)

	// ---- Delivery failure: retries, then "failed"; resending after the receiver recovers ----
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": sid2}, 204, nil)
	teacher.must("GET", base+"/invitations", nil, 200, &list)
	iid = str(list, "items", 0, "id")
	before := len(sink.Messages())
	sink.Fail(true)
	teacher.must("POST", base+"/invitations/"+iid+"/resend", nil, 200, nil)
	for range 5 {
		dispatch()
		if _, err := e.pool.Exec(ctx, `UPDATE email_outbox SET next_attempt_at = now() WHERE status = 'queued'`); err != nil {
			t.Fatal(err)
		}
	}
	if count(`SELECT count(*) FROM email_outbox WHERE status = 'failed' AND attempts = 5 AND last_error LIKE '%550%' AND body NOT LIKE '%/invite/%'`) != 1 {
		t.Fatal("failed letter not recorded")
	}
	teacher.must("GET", base+"/invitations", nil, 200, &list)
	if str(list, "items", 0, "delivery") != "failed" || len(sink.Messages()) != before {
		t.Fatalf("delivery state: %v", list["items"])
	}
	sink.Fail(false)
	teacher.must("POST", base+"/invitations/"+iid+"/resend", nil, 200, nil)
	dispatch()
	t6 := lastToken("student2@test.local")
	student2.must("POST", "/api/invitations/"+t6+"/accept", nil, 200, nil)
	var sts obj
	teacher.must("GET", base+"/students", nil, 200, &sts)
	if len(sts["items"].([]any)) != 2 {
		t.Fatalf("participants: %v", sts)
	}

	// ---- An archived marathon closes its links ----
	if r := invite("student@test.local", "student2@test.local"); str(r[0], "result") != "already_participant" || str(r[1], "result") != "already_participant" {
		t.Fatalf("participants invited: %v", r)
	}
	teacher.must("POST", base+"/archive", nil, 204, nil)
	anon.must("GET", "/api/invitations/"+t6, nil, 404, nil)
}

// anonymous is a client without a session (its own cookie jar).
func (e *env) anonymous() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar}, name: "anon"}
}
