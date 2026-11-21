package router_test

// End-to-end API tests against a real PostgreSQL database.
// Run with TEST_DATABASE_URL set (see README); skipped otherwise.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/router"
)

const origin = "http://localhost:3448"

type env struct {
	t    *testing.T
	app  *router.App
	srv  *httptest.Server
	pool *pgxpool.Pool
	ids  map[string]string
}

type client struct {
	e    *env
	http *http.Client
	name string
}

func setup(t *testing.T, overrides ...func(*config.Config)) *env {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(url, "up"); err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.HashPassword("password-123")
	ids := map[string]string{}
	for _, u := range [][3]string{{"admin", "admin"}, {"teacher", "teacher"}, {"student", "student"}, {"student2", "student"}, {"teacher2", "teacher"}} {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO users (email, full_name, role, password_hash) VALUES ($1, $2, $3, $4) RETURNING id::text`,
			u[0]+"@test.local", strings.ToUpper(u[0][:1])+u[0][1:], u[1], hash).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[u[0]] = id
	}
	cfg := config.Config{Env: "local", AllowedOrigins: []string{origin}, CookieName: "s", SessionTTL: time.Hour,
		UploadDir: t.TempDir(), UploadMaxBytes: 1 << 20, GiphyAPIBase: "http://127.0.0.1:1",
		LoginMaxPerEmail: 5, LoginMaxPerIP: 100, LoginWindow: time.Minute}
	for _, o := range overrides {
		o(&cfg)
	}
	app, err := router.New(cfg, pool)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Engine)
	t.Cleanup(srv.Close)
	return &env{t: t, app: app, srv: srv, pool: pool, ids: ids}
}

func (e *env) login(name string) *client {
	jar, _ := cookiejar.New(nil)
	c := &client{e: e, http: &http.Client{Jar: jar}, name: name}
	c.must("POST", "/api/auth/login", map[string]string{"email": name + "@test.local", "password": "password-123"}, 200, nil)
	return c
}

// do performs a JSON request and returns status and raw body.
func (c *client) do(method, path string, body any) (int, []byte) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (c *client) must(method, path string, body any, want int, out any) []byte {
	c.e.t.Helper()
	st, b := c.do(method, path, body)
	if st != want {
		c.e.t.Fatalf("%s %s %s: want %d got %d: %s", c.name, method, path, want, st, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			c.e.t.Fatalf("decode %s: %v", b, err)
		}
	}
	return b
}

type obj = map[string]any

func get(m any, path ...any) any {
	cur := m
	for _, p := range path {
		switch k := p.(type) {
		case string:
			cur = cur.(map[string]any)[k]
		case int:
			cur = cur.([]any)[k]
		}
	}
	return cur
}

func str(m any, path ...any) string { v, _ := get(m, path...).(string); return v }
func num(m any, path ...any) int    { v, _ := get(m, path...).(float64); return int(v) }

func testQuestion(prompt, correct, wrong string) obj {
	return obj{"prompt": prompt, "multiple": false, "options": []obj{{"label": correct, "is_correct": true}, {"label": wrong, "is_correct": false}}}
}

func TestFullScenario(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	student := e.login("student")
	student2 := e.login("student2")
	teacher2 := e.login("teacher2")

	// Admin assigns students to teachers.
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	teacher.must("GET", "/api/admin/users", nil, 403, nil)
	// A teacher cannot create a class with an unassigned student.
	teacher2.must("POST", "/api/classrooms", obj{"name": "X", "student_id": e.ids["student"]}, 422, nil)

	var cls, cls2 obj
	teacher.must("POST", "/api/classrooms", obj{"name": "Class A", "student_id": e.ids["student"]}, 201, &cls)
	teacher.must("POST", "/api/classrooms", obj{"name": "Class B", "student_id": e.ids["student2"]}, 201, &cls2)
	cid, cid2 := str(cls, "id"), str(cls2, "id")

	// Materials: course -> lesson -> sections, rename and reorder.
	var course, lesson, sec1, sec2 obj
	teacher.must("POST", "/api/courses", obj{"title": "English"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Lesson 1", "attrs": obj{"level": "A1", "skills": []string{"Grammar"}}}, 201, &lesson)
	lid := str(lesson, "id")
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Warm-up"}, 201, &sec1)
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Grammar"}, 201, &sec2)
	teacher.must("PATCH", "/api/sections/"+str(sec1, "id"), obj{"title": "Warm up!"}, 204, nil)
	teacher.must("PUT", "/api/lessons/"+lid+"/sections/order", obj{"ids": []string{str(sec2, "id"), str(sec1, "id")}}, 204, nil)
	teacher.must("PUT", "/api/lessons/"+lid+"/sections/order", obj{"ids": []string{str(sec2, "id")}}, 422, nil)
	teacher2.must("GET", "/api/lessons/"+lid, nil, 404, nil)
	var filtered obj
	teacher.must("GET", "/api/courses?skill=Grammar&level=A1", nil, 200, &filtered)
	if num(filtered, "total") != 1 {
		t.Fatalf("filter: %v", filtered)
	}
	teacher.must("GET", "/api/courses?skill=Reading", nil, 200, &filtered)
	if num(filtered, "total") != 0 {
		t.Fatalf("filter should exclude: %v", filtered)
	}

	// Validation: question without correct answer is rejected.
	bad := obj{"scope": "lesson", "section_id": str(sec1, "id"), "kind": "test", "title": "T",
		"questions": []obj{{"prompt": "Q", "options": []obj{{"label": "a"}, {"label": "b"}}}}}
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", bad, 422, nil)
	bad["kind"] = "matching"
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", bad, 422, nil)

	// Exercise 1: instant check, scoring 1 point, category "Универсальная".
	var ex1 obj
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "lesson", "section_id": str(sec1, "id"), "kind": "test",
		"title": "Проверка английского", "settings": obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1},
		"questions": []obj{testQuestion("Choose the correct answer: She ___ a teacher.", "is", "are")}}, 201, &ex1)
	ex1ID := str(ex1, "id")
	// Exercise 2: check after finishing, with a timer.
	var ex2 obj
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "lesson", "section_id": str(sec2, "id"), "kind": "test",
		"title": "Timed", "settings": obj{"check_mode": "after", "timer_enabled": true, "timer_seconds": 60},
		"questions": []obj{testQuestion("I ___ fine.", "am", "is"), testQuestion("We ___ here.", "are", "is")}}, 201, &ex2)
	ex2ID := str(ex2, "id")

	// Student cannot see the lesson before it is opened in the classroom.
	student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises", nil, 404, nil)
	teacher.must("PUT", "/api/classrooms/"+cid+"/current-lesson", obj{"lesson_id": lid}, 200, nil)
	teacher2.must("PUT", "/api/classrooms/"+cid+"/current-lesson", obj{"lesson_id": lid}, 404, nil)

	// Answer key must not leak to the student.
	raw := student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises", nil, 200, nil)
	if strings.Contains(string(raw), "is_correct") || strings.Contains(string(raw), "correct_option_ids\":[") {
		t.Fatalf("key leaked: %s", raw)
	}
	// Teacher cannot answer for the student.
	teacher.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex1ID+"/attempt", nil, 403, nil)

	// Instant check: start is idempotent; correct answer is graded and locked.
	var v1, v1b obj
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex1ID+"/attempt", nil, 200, &v1)
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex1ID+"/attempt", nil, 200, &v1b)
	if str(v1, "attempt", "id") != str(v1b, "attempt", "id") {
		t.Fatal("start not idempotent")
	}
	aid := str(v1, "attempt", "id")
	q := get(v1, "version", "questions", 0).(map[string]any)
	qid := q["id"].(string)
	optIs, optAre := str(q, "options", 0, "id"), str(q, "options", 1, "id")
	var ans obj
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"option_ids": []string{optIs}, "check": true}, 200, &ans)
	if str(ans, "attempt", "answers", 0, "result") != "correct" {
		t.Fatalf("expected correct: %v", ans)
	}
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"option_ids": []string{optIs}, "check": true}, 200, nil) // retry
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"option_ids": []string{optAre}, "check": true}, 409, nil)
	student2.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"option_ids": []string{optAre}}, 404, nil)
	var fin obj
	student.must("POST", "/api/attempts/"+aid+"/finish", nil, 200, &fin)
	student.must("POST", "/api/attempts/"+aid+"/finish", nil, 200, nil) // repeat
	if num(fin, "attempt", "summary", "correct") != 1 || num(fin, "attempt", "summary", "points_earned") != 1 {
		t.Fatalf("summary: %v", fin["attempt"])
	}

	// After-check + timer: key hidden until finish; refresh keeps the deadline.
	var t1, t2 obj
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex2ID+"/attempt", nil, 200, &t1)
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex2ID+"/attempt", nil, 200, &t2)
	if str(t1, "attempt", "deadline_at") == "" || str(t1, "attempt", "deadline_at") != str(t2, "attempt", "deadline_at") {
		t.Fatalf("deadline changed: %v vs %v", t1["attempt"], t2["attempt"])
	}
	aid2 := str(t1, "attempt", "id")
	q2 := str(t1, "version", "questions", 0, "id")
	var a2 obj
	student.must("PUT", "/api/attempts/"+aid2+"/answers/"+q2, obj{"option_ids": []string{str(t1, "version", "questions", 0, "options", 1, "id")}}, 200, &a2)
	if get(a2, "attempt", "answers", 0, "result") != nil || strings.Contains(fmt.Sprint(a2), "is_correct") {
		t.Fatalf("after-mode revealed early: %v", a2)
	}
	student.must("PUT", "/api/attempts/"+aid2+"/answers/"+q2, obj{"option_ids": []string{str(t1, "version", "questions", 0, "options", 0, "id")}, "check": true}, 400, nil)
	// Simulate time passing on the server: deadline in the past.
	if _, err := e.pool.Exec(context.Background(), `UPDATE attempts SET deadline_at = now() - interval '1 second', started_at = now() - interval '61 seconds' WHERE id = $1`, aid2); err != nil {
		t.Fatal(err)
	}
	student.must("PUT", "/api/attempts/"+aid2+"/answers/"+q2, obj{"option_ids": []string{str(t1, "version", "questions", 0, "options", 0, "id")}}, 409, nil)
	var expired obj
	student.must("GET", "/api/classrooms/"+cid+"/exercises/"+ex2ID, nil, 200, &expired)
	if str(expired, "attempt", "status") != "finished" || str(expired, "attempt", "finish_reason") != "timeout" ||
		num(expired, "attempt", "summary", "wrong") != 1 || num(expired, "attempt", "summary", "unanswered") != 1 {
		t.Fatalf("timeout: %v", expired["attempt"])
	}

	// Editing the exercise creates a new version; the finished attempt keeps its snapshot.
	teacher.must("PUT", "/api/exercises/"+ex1ID, obj{"title": "Проверка английского v2", "settings": obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1},
		"questions": []obj{testQuestion("Changed question?", "yes", "no")}}, 200, nil)
	var after obj
	student.must("GET", "/api/classrooms/"+cid+"/exercises/"+ex1ID, nil, 200, &after)
	if str(after, "version", "questions", 0, "prompt") != "Choose the correct answer: She ___ a teacher." {
		t.Fatalf("finished attempt lost its version: %v", after["version"])
	}

	// Points are recomputed, never accumulated: still 1/1 after repeats.
	var prog obj
	student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/progress", nil, 200, &prog)
	if num(prog, "lesson_points", "earned") != 1 || num(prog, "lesson_points", "max") != 1 || str(prog, "lesson_points", "categories", 0, "category_name") != "Универсальная" {
		t.Fatalf("points: %v", prog["lesson_points"])
	}

	// Data isolation: the same lesson in class B has no answers from class A.
	teacher.must("PUT", "/api/classrooms/"+cid2+"/current-lesson", obj{"lesson_id": lid}, 200, nil)
	var other obj
	student2.must("GET", "/api/classrooms/"+cid2+"/lessons/"+lid+"/exercises", nil, 200, &other)
	for _, it := range other["items"].([]any) {
		if get(it, "attempt") != nil {
			t.Fatal("attempt leaked across classrooms")
		}
	}
	student2.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises", nil, 404, nil)
	student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises?student_id="+e.ids["student2"], nil, 403, nil)
	teacher2.must("GET", "/api/classrooms/"+cid, nil, 404, nil)

	// Reset: only the chosen student, history kept.
	teacher.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex1ID+"/reset", obj{"student_id": e.ids["student"]}, 200, nil)
	var resetCount int
	e.pool.QueryRow(context.Background(), `SELECT count(*) FROM attempts WHERE exercise_id = $1 AND status = 'reset'`, ex1ID).Scan(&resetCount)
	if resetCount != 1 {
		t.Fatalf("reset audit rows: %d", resetCount)
	}
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+ex1ID+"/attempt", nil, 200, nil)

	// ---------- Homework ----------
	var hw1, hwCopy obj
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "homework", "kind": "test", "title": "Домашняя работа — to be",
		"settings": obj{"check_mode": "after"}, "questions": []obj{testQuestion("They ___ students.", "are", "is")}}, 201, &hw1)
	teacher.must("POST", "/api/exercises/"+ex1ID+"/copy-to-homework", nil, 201, &hwCopy)
	if str(hwCopy, "id") == ex1ID || str(hwCopy, "scope") != "homework" {
		t.Fatal("copy not independent")
	}
	var hwv obj
	student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework", nil, 200, &hwv)
	if str(hwv, "status") != "not_submitted" || hwv["latest_submission"] != nil {
		t.Fatalf("hw initial: %v", hwv)
	}
	var hs obj
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+str(hw1, "id")+"/attempt", nil, 200, &hs)
	hwAid := str(hs, "attempt", "id")
	hwQ := str(hs, "version", "questions", 0, "id")
	hwIs := str(hs, "version", "questions", 0, "options", 1, "id") // wrong: "is"
	hwAre := str(hs, "version", "questions", 0, "options", 0, "id")
	student.must("PUT", "/api/attempts/"+hwAid+"/answers/"+hwQ, obj{"option_ids": []string{hwIs}}, 200, nil)
	student.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/submit", nil, 200, &hwv)
	student.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/submit", nil, 200, nil) // idempotent
	if str(hwv, "status") != "pending" || num(hwv, "submissions_count") != 1 {
		t.Fatalf("submit: %v", hwv)
	}
	student.must("PUT", "/api/attempts/"+hwAid+"/answers/"+hwQ, obj{"option_ids": []string{hwAre}}, 409, nil)
	student.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/status", obj{"student_id": e.ids["student"], "to_status": "done"}, 403, nil)
	teacher.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/status", obj{"student_id": e.ids["student"], "to_status": "revision", "from_status": "pending"}, 200, &hwv)
	if str(hwv, "status") != "revision" {
		t.Fatal("revision")
	}
	// Student fixes and resubmits; the first snapshot must remain unchanged.
	student.must("PUT", "/api/attempts/"+hwAid+"/answers/"+hwQ, obj{"option_ids": []string{hwAre}}, 200, nil)
	var tv obj
	teacher.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework?student_id="+e.ids["student"], nil, 200, &tv)
	if num(tv, "latest_submission", "number") != 1 || !strings.Contains(string(mustJSON(tv["latest_submission"])), hwIs) {
		t.Fatal("draft changed submitted snapshot")
	}
	student.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/submit", nil, 200, &hwv)
	teacher.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework?student_id="+e.ids["student"], nil, 200, &tv)
	if num(tv, "latest_submission", "number") != 2 || str(tv, "status") != "pending" {
		t.Fatalf("resubmit: %v", tv)
	}
	// Stale optimistic check is rejected.
	teacher.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/status", obj{"student_id": e.ids["student"], "to_status": "done", "from_status": "revision"}, 409, nil)
	teacher.must("POST", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework/status", obj{"student_id": e.ids["student"], "to_status": "done"}, 200, &tv)
	if str(tv, "status") != "done" || len(tv["events"].([]any)) != 4 {
		t.Fatalf("done/events: %v", tv)
	}
	if str(tv, "events", 3, "actor_name") != "Teacher" {
		t.Fatalf("event actor: %v", tv["events"])
	}
	// Teacher may complete student2's homework before submission (explicit action).
	teacher.must("POST", "/api/classrooms/"+cid2+"/lessons/"+lid+"/homework/status", obj{"student_id": e.ids["student2"], "to_status": "done"}, 200, nil)

	// ---------- Section & lesson completion ----------
	student.must("PUT", "/api/classrooms/"+cid+"/sections/"+str(sec1, "id")+"/completion", obj{"completed": true}, 403, nil)
	teacher.must("PUT", "/api/classrooms/"+cid+"/sections/"+str(sec1, "id")+"/completion", obj{"completed": true}, 204, nil)
	teacher.must("PUT", "/api/classrooms/"+cid+"/lessons/"+lid+"/completion", obj{"completed": true}, 422, nil)
	student.must("PUT", "/api/classrooms/"+cid+"/lessons/"+lid+"/completion", obj{"completed": true, "grade": "good"}, 403, nil)
	teacher.must("PUT", "/api/classrooms/"+cid+"/lessons/"+lid+"/completion", obj{"completed": true, "grade": "good"}, 204, nil)
	teacher.must("PUT", "/api/classrooms/"+cid+"/lessons/"+lid+"/grade", obj{"grade": "amazing"}, 204, nil)
	student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/progress", nil, 200, &prog)
	if get(prog, "completion", "completed") != true || str(prog, "completion", "grade") != "amazing" || get(prog, "sections", 1, "completed") != true {
		t.Fatalf("completion: %v", prog)
	}
	// Completion of class A does not leak into class B.
	var prog2 obj
	student2.must("GET", "/api/classrooms/"+cid2+"/lessons/"+lid+"/progress", nil, 200, &prog2)
	if get(prog2, "completion", "completed") != false {
		t.Fatal("completion leaked across classrooms")
	}
	// Lesson and homework states are independent.
	teacher.must("PUT", "/api/classrooms/"+cid+"/lessons/"+lid+"/completion", obj{"completed": false}, 204, nil)
	student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/homework", nil, 200, &hwv)
	if str(hwv, "status") != "done" {
		t.Fatal("homework status changed by lesson uncomplete")
	}

	// ---------- Chat ----------
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/classrooms/" + cid + "/ws"
	if _, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: teacher2.http, HTTPHeader: http.Header{"Origin": {origin}}}); err == nil || resp == nil || resp.StatusCode != 404 {
		t.Fatalf("ws must reject non-members: %v", err)
	}
	if _, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {origin}}}); err == nil || resp == nil || resp.StatusCode != 401 {
		t.Fatalf("ws must reject anonymous: %v", err)
	}
	ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: student.http, HTTPHeader: http.Header{"Origin": {origin}}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	readEvent := func(typ string) obj {
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("ws read waiting for %s: %v", typ, err)
			}
			var ev obj
			json.Unmarshal(b, &ev)
			if ev["type"] == typ {
				return ev
			}
		}
	}
	readEvent("hello")

	clientID := "7a0c3d5e-1111-4222-8333-944455556666"
	var m1, m1b obj
	teacher.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": clientID, "kind": "text", "body": "Hello!"}, 201, &m1)
	teacher.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": clientID, "kind": "text", "body": "Hello!"}, 201, &m1b)
	if str(m1, "id") != str(m1b, "id") {
		t.Fatal("duplicate message created")
	}
	ev := readEvent("chat.message.created")
	if str(ev, "payload", "body") != "Hello!" {
		t.Fatalf("event: %v", ev)
	}
	student.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": "7a0c3d5e-1111-4222-8333-944455550000", "kind": "text", "body": "Hi teacher"}, 201, nil)
	student.must("PATCH", "/api/messages/"+str(m1, "id"), obj{"body": "hacked"}, 403, nil)
	student.must("DELETE", "/api/messages/"+str(m1, "id"), nil, 403, nil)
	teacher.must("PATCH", "/api/messages/"+str(m1, "id"), obj{"body": "Hello, edited"}, 200, nil)
	if str(readEvent("chat.message.updated"), "payload", "body") != "Hello, edited" {
		t.Fatal("edit not synced")
	}
	teacher.must("DELETE", "/api/messages/"+str(m1, "id"), nil, 204, nil)
	teacher.must("DELETE", "/api/messages/"+str(m1, "id"), nil, 204, nil)
	readEvent("chat.message.deleted")
	var page obj
	student.must("GET", "/api/classrooms/"+cid+"/messages", nil, 200, &page)
	if len(page["items"].([]any)) != 1 || str(page, "items", 0, "body") != "Hi teacher" {
		t.Fatalf("history after delete: %v", page)
	}
	teacher2.must("GET", "/api/classrooms/"+cid+"/messages", nil, 404, nil)
	// GIF without a configured key: explicit unavailability, nothing stored.
	st, _ := teacher.do("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": "7a0c3d5e-1111-4222-8333-944455551111", "kind": "gif", "gif_id": "abc"})
	if st != 503 {
		t.Fatalf("gif without key: %d", st)
	}

	// Attachments: type check, member-only download.
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	var att obj
	upload(t, teacher, "/api/classrooms/"+cid+"/attachments", "pic.png", png, 201, &att)
	upload(t, teacher, "/api/classrooms/"+cid+"/attachments", "evil.html", []byte("<html><script>alert(1)</script></html>"), 422, nil)
	upload(t, teacher2, "/api/classrooms/"+cid+"/attachments", "pic.png", png, 404, nil)
	var fm obj
	teacher.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": "7a0c3d5e-1111-4222-8333-944455552222", "kind": "file", "attachment_id": str(att, "id"), "body": "see"}, 201, &fm)
	if st, b := student.do("GET", "/api/attachments/"+str(att, "id"), nil); st != 200 || !bytes.Equal(b, png) {
		t.Fatalf("student download: %d", st)
	}
	if st, _ := teacher2.do("GET", "/api/attachments/"+str(att, "id"), nil); st != 404 {
		t.Fatalf("foreign download must be denied: %d", st)
	}
	teacher.must("DELETE", "/api/messages/"+str(fm, "id"), nil, 204, nil)
	if st, _ := student.do("GET", "/api/attachments/"+str(att, "id"), nil); st != 404 {
		t.Fatalf("attachment of deleted message still served: %d", st)
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func upload(t *testing.T, c *client, path, name string, data []byte, want int, out any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", name)
	fw.Write(data)
	w.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", origin)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("upload %s: want %d got %d: %s", name, want, resp.StatusCode, b)
	}
	if out != nil {
		json.Unmarshal(b, out)
	}
}

func TestLoginRateLimitAndSession(t *testing.T) {
	e := setup(t)
	jar, _ := cookiejar.New(nil)
	c := &client{e: e, http: &http.Client{Jar: jar}, name: "anon"}
	for i := 0; i < 5; i++ {
		c.must("POST", "/api/auth/login", obj{"email": "teacher@test.local", "password": "wrong"}, 401, nil)
	}
	c.must("POST", "/api/auth/login", obj{"email": "teacher@test.local", "password": "password-123"}, 429, nil)

	s := e.login("student")
	s.must("GET", "/api/auth/me", nil, 200, nil)
	// Cookie flags.
	st, _ := s.do("POST", "/api/auth/logout", nil)
	if st != 204 {
		t.Fatal("logout")
	}
	s.must("GET", "/api/auth/me", nil, 401, nil)
	// Foreign origin cannot perform state-changing requests.
	s2 := e.login("student2")
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/auth/logout", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, _ := s2.http.Do(req)
	if resp.StatusCode != 403 {
		t.Fatalf("origin guard: %d", resp.StatusCode)
	}
	// Session cookie is HttpOnly and SameSite.
	jar2, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar2}
	resp, _ = hc.Post(e.srv.URL+"/api/auth/login", "application/json", strings.NewReader(`{"email":"admin@test.local","password":"password-123"}`))
	sc := resp.Header.Get("Set-Cookie")
	if !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Lax") {
		t.Fatalf("cookie flags: %s", sc)
	}
}

// TestGifWithStubbedGiphy exercises the GIF flow against a local GIPHY stub:
// search, send with caption, caption edit, and rejection of unknown ids.
func TestGifWithStubbedGiphy(t *testing.T) {
	giphy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		img := `{"fixed_height":{"url":"https://media.giphy.com/media/abc123/200.gif","width":"200","height":"200"},"fixed_height_small":{"url":"https://media.giphy.com/media/abc123/100.gif","width":"100","height":"100"}}`
		switch r.URL.Path {
		case "/v1/gifs/search", "/v1/gifs/trending":
			fmt.Fprintf(w, `{"data":[{"id":"abc123","title":"Hello GIF","images":%s}]}`, img)
		case "/v1/gifs/abc123":
			fmt.Fprintf(w, `{"data":{"id":"abc123","title":"Hello GIF","images":%s}}`, img)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer giphy.Close()
	e := setup(t, func(c *config.Config) { c.GiphyAPIKey = "test-key"; c.GiphyAPIBase = giphy.URL })
	admin, teacher, student := e.login("admin"), e.login("teacher"), e.login("student")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	var cls obj
	teacher.must("POST", "/api/classrooms", obj{"name": "G", "student_id": e.ids["student"]}, 201, &cls)
	cid := str(cls, "id")

	var status obj
	teacher.must("GET", "/api/gifs/status", nil, 200, &status)
	if status["enabled"] != true {
		t.Fatal("gif status should be enabled")
	}
	var found obj
	teacher.must("GET", "/api/gifs/search?q=hello", nil, 200, &found)
	if str(found, "items", 0, "id") != "abc123" {
		t.Fatalf("search: %v", found)
	}
	var m obj
	teacher.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": "11111111-2222-4333-8444-555555555555", "kind": "gif", "gif_id": "abc123", "body": "Well done"}, 201, &m)
	if str(m, "gif", "url") != "https://media.giphy.com/media/abc123/200.gif" || str(m, "body") != "Well done" {
		t.Fatalf("gif message: %v", m)
	}
	teacher.must("PATCH", "/api/messages/"+str(m, "id"), obj{"body": "Excellent"}, 200, nil)
	teacher.must("PATCH", "/api/messages/"+str(m, "id"), obj{"body": ""}, 200, nil) // caption is optional
	var page obj
	student.must("GET", "/api/classrooms/"+cid+"/messages", nil, 200, &page)
	if str(page, "items", 0, "kind") != "gif" || str(page, "items", 0, "body") != "" {
		t.Fatalf("student sees: %v", page)
	}
	// Unknown or malformed ids are not stored; arbitrary URLs cannot be injected.
	teacher.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": "11111111-2222-4333-8444-555555555556", "kind": "gif", "gif_id": "nope"}, 404, nil)
	teacher.must("POST", "/api/classrooms/"+cid+"/messages", obj{"client_id": "11111111-2222-4333-8444-555555555557", "kind": "gif", "gif_id": "../../x"}, 422, nil)
}

// TestClassroomListFilters: search + type filter + sorting work together; wildcards are literal.
func TestClassroomListFilters(t *testing.T) {
	e := setup(t)
	admin, teacher := e.login("admin"), e.login("teacher")
	for _, s := range []string{"student", "student2"} {
		admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids[s]}, 204, nil)
	}
	for _, c := range [][2]string{{"Beta English", "student"}, {"Alpha English", "student2"}, {"Gamma 100%", "student"}} {
		teacher.must("POST", "/api/classrooms", obj{"name": c[0], "student_id": e.ids[c[1]]}, 201, nil)
	}
	names := func(path string) []string {
		var l obj
		teacher.must("GET", path, nil, 200, &l)
		out := []string{}
		for _, it := range l["items"].([]any) {
			out = append(out, str(it, "name"))
		}
		if num(l, "total") != len(out) {
			t.Fatalf("total mismatch for %s", path)
		}
		return out
	}
	if got := fmt.Sprint(names("/api/classrooms?sort=name&order=asc")); got != "[Alpha English Beta English Gamma 100%]" {
		t.Fatalf("name asc: %s", got)
	}
	if got := fmt.Sprint(names("/api/classrooms?sort=name&order=desc&q=english")); got != "[Beta English Alpha English]" {
		t.Fatalf("search + desc: %s", got)
	}
	if got := fmt.Sprint(names("/api/classrooms?q=english&type=individual&sort=name&order=asc")); got != "[Alpha English Beta English]" {
		t.Fatalf("search + type: %s", got)
	}
	if got := names("/api/classrooms?q=english&type=group"); len(got) != 0 {
		t.Fatalf("group should be empty: %v", got)
	}
	if got := names("/api/classrooms?type=marathon"); len(got) != 0 {
		t.Fatalf("marathon should be empty: %v", got)
	}
	if got := fmt.Sprint(names("/api/classrooms?q=0%25")); got != "[Gamma 100%]" {
		t.Fatalf("literal percent: %s", got)
	}
	if got := names("/api/classrooms?q=%25"); len(got) != 1 {
		t.Fatalf("%% must not match everything: %v", got)
	}
	teacher.must("GET", "/api/classrooms?type=webinar", nil, 422, nil)
	teacher.must("GET", "/api/classrooms?sort=id", nil, 422, nil)
}
