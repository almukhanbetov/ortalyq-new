package router_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Plan, section 6.1: the whole marathon chain in one scenario — creation (progressive mode, teacher check, dates),
// lessons, students, access, the attempt, sending for review, revision, acceptance, the next lesson, statistics and
// rating, the teacher preview kept apart, cancelling and completing again, access blocking, a mode change that keeps
// attempts and points, and the end date. Every step checks the server's answer and, where it matters, the database.
func TestMarathonFullScenario(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2") // not assigned to the teacher, not enrolled
	sid := e.assign(admin)
	rows := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	code := func(b []byte) string {
		var x obj
		_ = json.Unmarshal(b, &x)
		return str(x, "error", "code")
	}

	// ---- 1. Marathon: progressive, teacher check (teacher_check), start now-1m, end now+1d, statistics on ----
	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Сквозной марафон", "direction": "other"}, 201, &m)
	id := str(m, "id")
	base := "/api/marathons/" + id
	teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": 1, "confirm_reset": true}, 200, nil)
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true, "unlock_after": "teacher_check"}, 200, nil)
	now := time.Now()
	teacher.must("PATCH", base+"/settings", obj{"start_at": now.Add(-time.Minute), "end_at": now.Add(24 * time.Hour)}, 200, &m)
	if get(m, "marathon", "started") != true || get(m, "marathon", "finished") != false || str(m, "marathon", "mode") != "progressive" {
		t.Fatalf("settings: %v", m["marathon"])
	}
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": true, "rating_enabled": true}, 200, nil)

	// ---- 2. Lesson 1: a test of 2 questions, instant check, 60 s timer, 1 point per answer; lesson 2 ----
	var course obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	var lids, exids []string
	for i, title := range []string{"Урок 1", "Урок 2"} {
		var l, sec, ex obj
		teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": title}, 201, &l)
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/sections", obj{"title": "Раздел"}, 201, &sec)
		settings := obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1}
		if i == 0 {
			settings["timer_enabled"], settings["timer_seconds"] = true, 60
		}
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test",
			"title": "Тест", "settings": settings, "questions": []obj{testQuestion("She ___ a teacher.", "is", "are"), testQuestion("They ___ friends.", "are", "is")}}, 201, &ex)
		teacher.must("POST", base+"/lessons", obj{"lesson_id": str(l, "id")}, 200, nil)
		lids, exids = append(lids, str(l, "id")), append(exids, str(ex, "id"))
	}
	teacher2.must("POST", base+"/lessons", obj{"lesson_id": lids[0]}, 404, nil)

	// ---- 3. Students: the assigned one, a repeat without a duplicate; an unassigned student — 422 ----
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid}}, 200, nil)
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid}}, 200, nil)
	if n := rows(`SELECT count(*) FROM marathon_enrollments WHERE classroom_id = $1`, id); n != 1 {
		t.Fatalf("enrollments: %d", n)
	}
	teacher.must("POST", base+"/students", obj{"student_ids": []string{e.ids["student2"]}}, 422, nil)

	// ---- 4. Access: lesson 1 open, lesson 2 locked (waits for the review); others do not see the marathon ----
	var me obj
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 0, "access") != "open" || str(me, "items", 1, "access") != "locked" || str(me, "items", 1, "lock", "reason") != "previous_not_reviewed" {
		t.Fatalf("access: %v", me["items"])
	}
	b := student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[1]+"/exercises?scope=lesson", nil, 403, nil)
	if code(b) != "marathon_lesson_locked" {
		t.Fatalf("lesson 2: %s", b)
	}
	teacher2.must("GET", "/api/classrooms/"+id, nil, 404, nil)
	student2.must("GET", "/api/classrooms/"+id, nil, 404, nil)
	student2.must("GET", base+"/me", nil, 404, nil)

	// ---- 5. The attempt: started twice — the same; right and wrong answers; a repeat is idempotent; a change — 409 ----
	var a1, a2 obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exids[0]+"/attempt", nil, 200, &a1)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exids[0]+"/attempt", nil, 200, &a2)
	aid := str(a1, "attempt", "id")
	if aid == "" || aid != str(a2, "attempt", "id") {
		t.Fatal("start twice gave two attempts")
	}
	q1 := get(a1, "version", "questions", 0).(map[string]any)
	q2 := get(a1, "version", "questions", 1).(map[string]any)
	opt := func(q map[string]any, label string) string {
		for _, o := range q["options"].([]any) {
			if o.(map[string]any)["label"] == label {
				return o.(map[string]any)["id"].(string)
			}
		}
		t.Fatalf("no option %s", label)
		return ""
	}
	answer := func(c *client, q map[string]any, label string, want int) []byte {
		t.Helper()
		return c.must("PUT", "/api/attempts/"+aid+"/answers/"+q["id"].(string), obj{"option_ids": []string{opt(q, label)}, "check": true}, want, nil)
	}
	answer(student, q1, "is", 200)
	answer(student, q2, "is", 200) // wrong
	answer(student, q1, "is", 200) // the same answer again
	if b := answer(student, q1, "are", 409); code(b) != "answer_locked" {
		t.Fatalf("change of a checked answer: %s", b)
	}

	// ---- 6. finish twice — one result; points 1 of 2 ----
	var f1, f2 obj
	student.must("POST", "/api/attempts/"+aid+"/finish", nil, 200, &f1)
	student.must("POST", "/api/attempts/"+aid+"/finish", nil, 200, &f2)
	if str(f1, "attempt", "finished_at") != str(f2, "attempt", "finished_at") || num(f1, "attempt", "summary", "points_earned") != 1 ||
		num(f1, "attempt", "summary", "points_max") != 2 || num(f2, "attempt", "summary", "points_earned") != 1 {
		t.Fatalf("finish: %v / %v", f1["attempt"], f2["attempt"])
	}

	// ---- 7. Sending for review: answers locked, a repeated send keeps one snapshot ----
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	if b := answer(student, q2, "are", 409); code(b) != "lesson_review_locked" {
		t.Fatalf("answer during review: %s", b)
	}
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	snapshots := func() int {
		return rows(`SELECT count(*) FROM marathon_lesson_submissions WHERE classroom_id = $1 AND student_id = $2`, id, sid)
	}
	if n := snapshots(); n != 1 {
		t.Fatalf("snapshots after a repeated send: %d", n)
	}

	// ---- 8. Revision: only the wrong answer opens; the student fixes it and sends again; the teacher accepts ----
	review := func(from, to string, extra obj, want int) obj {
		t.Helper()
		body := obj{"student_id": sid, "from_status": from, "to_status": to}
		for k, v := range extra {
			body[k] = v
		}
		var out obj
		teacher.must("POST", base+"/lessons/"+lids[0]+"/review", body, want, &out)
		return out
	}
	review("submitted", "revision", nil, 200)
	if b := answer(student, q1, "are", 409); code(b) != "answer_locked" {
		t.Fatalf("a right answer must stay locked after revision: %s", b)
	}
	answer(student, q2, "are", 200)
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	if n := snapshots(); n != 2 {
		t.Fatalf("snapshots after the second send: %d", n)
	}
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 1, "access") != "locked" {
		t.Fatal("lesson 2 opened before the acceptance")
	}
	out := review("submitted", "completed", obj{"rating": "nice", "comment": "Молодец"}, 200)
	if get(out, "unchanged") != nil {
		t.Fatalf("first acceptance reported as unchanged: %v", out["unchanged"])
	}

	// ---- 9. Lesson 2 opens; statistics and rating: 2 points for S, nobody else; the preview stays apart ----
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 1, "access") != "open" || str(me, "items", 0, "status") != "completed" || get(me, "items", 0, "accepted") != true {
		t.Fatalf("after acceptance: %v", me["items"])
	}
	var stats, rating obj
	teacher.must("GET", base+"/stats", nil, 200, &stats)
	if len(get(stats, "items").([]any)) != 1 || num(stats, "items", 0, "points", "earned") != 2 || num(stats, "items", 0, "completed_lessons") != 1 {
		t.Fatalf("stats: %v", stats["items"])
	}
	student.must("GET", base+"/rating", nil, 200, &rating)
	if len(get(rating, "items").([]any)) != 1 || num(rating, "items", 0, "points") != 2 || get(rating, "items", 0, "me") != true {
		t.Fatalf("rating: %v", rating["items"])
	}
	before := string(teacher.must("GET", base+"/stats", nil, 200, nil)) + string(student.must("GET", base+"/rating", nil, 200, nil)) +
		string(teacher.must("GET", "/api/exercises/"+exids[0]+"/stats", nil, 200, nil))
	var pv obj
	teacher.must("POST", "/api/preview/exercises/"+exids[0]+"/attempt", nil, 200, &pv)
	pq := get(pv, "version", "questions", 0).(map[string]any)
	teacher.must("PUT", "/api/preview/attempts/"+str(pv, "attempt", "id")+"/answers/"+pq["id"].(string), obj{"option_ids": []string{opt(pq, "are")}, "check": true}, 200, nil)
	teacher.must("POST", "/api/preview/attempts/"+str(pv, "attempt", "id")+"/finish", nil, 200, nil)
	after := string(teacher.must("GET", base+"/stats", nil, 200, nil)) + string(student.must("GET", base+"/rating", nil, 200, nil)) +
		string(teacher.must("GET", "/api/exercises/"+exids[0]+"/stats", nil, 200, nil))
	if stripNow(before) != stripNow(after) {
		t.Fatal("the preview changed statistics, rating or exercise statistics")
	}
	if rows(`SELECT count(*) FROM attempts WHERE student_id = $1`, e.ids["teacher"]) != 0 || rows(`SELECT count(*) FROM preview_attempts`) != 1 {
		t.Fatal("preview attempts must be stored apart")
	}
	student.must("GET", "/api/preview/lessons/"+lids[0]+"/exercises", nil, 403, nil)

	// ---- Cancel the completion and complete again; repeated and stale decisions ----
	review("completed", "submitted", nil, 200)
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 0, "status") != "submitted" || str(me, "items", 1, "access") != "locked" {
		t.Fatalf("after cancel: %v", me["items"])
	}
	teacher.must("GET", base+"/stats", nil, 200, &stats)
	if num(stats, "items", 0, "completed_lessons") != 0 || num(stats, "items", 0, "points", "earned") != 2 {
		t.Fatalf("stats after cancel: %v", stats["items"])
	}
	review("submitted", "completed", obj{"rating": "amazing"}, 200)
	// A second acceptance (another tab) with another rating: idempotent, nothing saved, the answer says so.
	out = review("submitted", "completed", obj{"rating": "not_bad", "comment": "другое"}, 200)
	if get(out, "unchanged") != true || str(out, "lesson", "rating") != "amazing" {
		t.Fatalf("repeated acceptance: unchanged=%v rating=%v", out["unchanged"], get(out, "lesson", "rating"))
	}
	if b := teacher.must("POST", base+"/lessons/"+lids[0]+"/review", obj{"student_id": sid, "from_status": "submitted", "to_status": "revision"}, 409, nil); code(b) != "status_changed" {
		t.Fatalf("stale decision: %s", b)
	}
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 1, "access") != "open" {
		t.Fatal("lesson 2 must be open after the second completion")
	}

	// ---- Access blocking: manual deactivation, delayed start ----
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, nil)
	for _, p := range []string{"/api/classrooms/" + id + "/lessons/" + lids[1] + "/exercises?scope=lesson"} {
		if b := student.must("GET", p, nil, 403, nil); code(b) != "marathon_deactivated" {
			t.Fatalf("deactivated: %s", b)
		}
	}
	if b := student.must("POST", "/api/classrooms/"+id+"/exercises/"+exids[1]+"/attempt", nil, 403, nil); code(b) != "marathon_deactivated" {
		t.Fatalf("deactivated start: %s", b)
	}
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": true}, 200, nil)
	teacher.must("PATCH", base+"/settings", obj{"start_at": now.Add(time.Hour), "end_at": now.Add(24 * time.Hour)}, 200, nil)
	var ns obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises?scope=lesson", nil, 403, &ns)
	if str(ns, "error", "code") != "marathon_not_started" || str(ns, "error", "fields", "opens_at") == "" {
		t.Fatalf("not started: %v", ns)
	}
	teacher.must("PATCH", base+"/settings", obj{"start_at": now.Add(-time.Minute), "end_at": now.Add(24 * time.Hour)}, 200, nil)

	// ---- 10. Mode «Открытый» with confirmation: statuses reset, attempts and points kept; the same epoch again ----
	dataHash := func() string {
		t.Helper()
		var h string
		if err := e.pool.QueryRow(ctx, `SELECT md5(string_agg(x, ',' ORDER BY x)) FROM (
				SELECT a.id::text || a.status || COALESCE(a.finished_at::text, '') x FROM attempts a WHERE classroom_id = $1
				UNION ALL SELECT aa.attempt_id::text || aa.question_id::text || aa.selected_option_ids::text || aa.checked::text
				FROM attempt_answers aa JOIN attempts a ON a.id = aa.attempt_id WHERE a.classroom_id = $1) z`, id).Scan(&h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	hash := dataHash()
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	epoch := num(m, "marathon", "progress_epoch")
	teacher.must("POST", base+"/mode", obj{"mode": "open", "expected_epoch": epoch, "confirm_reset": true}, 200, nil)
	teacher.must("POST", base+"/mode", obj{"mode": "open", "expected_epoch": epoch, "confirm_reset": true}, 200, nil)
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if num(m, "marathon", "progress_epoch") != epoch+1 {
		t.Fatalf("epoch %d → %d: the repeat must not reset again", epoch, num(m, "marathon", "progress_epoch"))
	}
	if b := teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": epoch, "confirm_reset": true}, 409, nil); code(b) != "settings_changed" {
		t.Fatalf("stale epoch: %s", b)
	}
	student.must("GET", base+"/me", nil, 200, &me)
	for i := 0; i < 2; i++ {
		if str(me, "items", i, "status") != "none" || str(me, "items", i, "access") != "open" {
			t.Fatalf("after the mode change: %v", me["items"])
		}
	}
	teacher.must("GET", base+"/stats", nil, 200, &stats)
	if dataHash() != hash || num(stats, "items", 0, "points", "earned") != 2 {
		t.Fatal("the mode change touched attempts, answers or points")
	}

	// ---- 11. After end_at (by now() of the DB): start and answer refused, progress shows the reason, data kept ----
	var a3 obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exids[1]+"/attempt", nil, 200, &a3)
	hash = dataHash() // with the attempt just started; the end date must not change anything after this
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_settings SET end_at = now() - interval '1 minute' WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if b := student.must("POST", "/api/classrooms/"+id+"/exercises/"+exids[1]+"/attempt", nil, 403, nil); code(b) != "marathon_closed" {
		t.Fatalf("start after end: %s", b)
	}
	q3 := get(a3, "version", "questions", 0).(map[string]any)
	if b := student.must("PUT", "/api/attempts/"+str(a3, "attempt", "id")+"/answers/"+q3["id"].(string), obj{"option_ids": []string{opt(q3, "is")}, "check": true}, 403, nil); code(b) != "marathon_closed" {
		t.Fatalf("answer after end: %s", b)
	}
	var prog obj
	teacher.must("GET", base+"/students/"+sid+"/progress", nil, 200, &prog)
	if str(prog, "student", "status") != "inactive" || str(prog, "student", "inactive_reason") != "end_date" {
		t.Fatalf("progress after end: %v", prog["student"])
	}
	teacher.must("GET", base+"/stats", nil, 200, &stats)
	if dataHash() != hash || num(stats, "items", 0, "points", "earned") != 2 {
		t.Fatal("data lost after the end date")
	}
	teacher.must("GET", base+"/students/"+sid+"/lessons/"+lids[0], nil, 200, nil)
}

// stripNow drops server_now values so two reads of the same data compare equal.
func stripNow(s string) string {
	var v any
	if err := json.Unmarshal([]byte("["+joinJSON(s)+"]"), &v); err != nil {
		return s
	}
	var walk func(any) any
	walk = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			delete(t, "server_now")
			for k, y := range t {
				t[k] = walk(y)
			}
		case []any:
			for i, y := range t {
				t[i] = walk(y)
			}
		}
		return x
	}
	b, _ := json.Marshal(walk(v))
	return string(b)
}

// joinJSON turns concatenated JSON documents into a comma-separated list.
func joinJSON(s string) string {
	dec := json.NewDecoder(strings.NewReader(s))
	var parts []string
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return s
		}
		parts = append(parts, string(raw))
	}
	return strings.Join(parts, ",")
}
