package router_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// Stage 6 of marathons: "Проверка учителем". Sending a lesson (snapshot, locked answers), revision (only wrong
// instantly-checked answers become editable), acceptance with rating and comment, «Отменить завершение» and the
// second completion, "По мере прохождения" with "После проверки учителем" / "После отправки на проверку", rights,
// direct API calls, repeated and concurrent requests, answers and points kept, deactivation and end date.
func TestMarathonTeacherCheck(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2")
	ctx := t.Context()
	sid := e.assign(admin)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	sid2 := e.ids["student2"]

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон с проверкой", "direction": "other"}, 201, &m)
	id := str(m, "id")
	base := "/api/marathons/" + id
	if get(m, "marathon", "teacher_check") != false || str(m, "marathon", "unlock_after") != "teacher_check" {
		t.Fatalf("defaults: %v", m["marathon"])
	}
	var course obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	// Lesson 1: an instant test with two questions (5 points each) and a second test (after check).
	// Lessons 2 and 3: one instant question.
	lids := []string{}
	var exA, exB, ex2 obj
	for i, title := range []string{"Урок 1", "Урок 2", "Урок 3"} {
		var l, sec obj
		teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": title}, 201, &l)
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/sections", obj{"title": "Раздел"}, 201, &sec)
		qs := []obj{testQuestion("She ___ a teacher.", "is", "are")}
		if i == 0 {
			qs = append(qs, testQuestion("They ___ friends.", "are", "is"))
		}
		var ex obj
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест",
			"settings": obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 5}, "questions": qs}, 201, &ex)
		switch i {
		case 0:
			exA = ex
			teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест 2",
				"settings":  obj{"check_mode": "after", "scoring_enabled": true, "points_per_correct": 1},
				"questions": []obj{testQuestion("I ___ here.", "am", "is")}}, 201, &exB)
		case 1:
			ex2 = ex
		}
		teacher.must("POST", base+"/lessons", obj{"lesson_id": str(l, "id")}, 200, nil)
		lids = append(lids, str(l, "id"))
	}
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid, sid2}}, 200, nil)

	// option returns the id of the option with this label in the version of an exercise view.
	option := func(v obj, qi int, label string) (string, string) {
		t.Helper()
		q := get(v, "version", "questions", qi).(map[string]any)
		for _, o := range q["options"].([]any) {
			if o.(map[string]any)["label"] == label {
				return q["id"].(string), o.(map[string]any)["id"].(string)
			}
		}
		t.Fatalf("no option %q in %v", label, q)
		return "", ""
	}
	start := func(c *client, ex obj) obj {
		t.Helper()
		var v obj
		c.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex, "id")+"/attempt", nil, 200, &v)
		return v
	}
	answer := func(c *client, v obj, qi int, label string, want int) obj {
		t.Helper()
		qid, oid := option(v, qi, label)
		var out obj
		c.must("PUT", "/api/attempts/"+str(v, "attempt", "id")+"/answers/"+qid, obj{"option_ids": []string{oid}, "check": true}, want, &out)
		return out
	}
	code := func(c *client, method, path string, body any, want int, wantCode string) obj {
		t.Helper()
		var out obj
		c.must(method, path, body, want, &out)
		if str(out, "error", "code") != wantCode {
			t.Fatalf("%s %s: want %s, got %v", method, path, wantCode, out)
		}
		return out
	}
	me := func(c *client) obj {
		t.Helper()
		var out obj
		c.must("GET", base+"/me", nil, 200, &out)
		return out
	}
	review := func(sid, lid string) obj {
		t.Helper()
		var out obj
		teacher.must("GET", base+"/students/"+sid+"/lessons/"+lid, nil, 200, &out)
		return out
	}
	decide := func(c *client, lid string, body obj, want int) obj {
		t.Helper()
		var out obj
		c.must("POST", base+"/lessons/"+lid+"/review", body, want, &out)
		return out
	}
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	submissions := func(student, lid string) int {
		return count(`SELECT count(*) FROM marathon_lesson_submissions WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3`, id, lid, student)
	}
	events := func(student, lid string) int {
		return count(`SELECT count(*) FROM marathon_lesson_events WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3`, id, lid, student)
	}

	// ---- Settings: rights and validation ----
	teacher2.must("PUT", base+"/review-settings", obj{"teacher_check": true}, 404, nil)
	student.must("PUT", base+"/review-settings", obj{"teacher_check": true}, 403, nil)
	teacher.must("PUT", base+"/review-settings", obj{}, 422, nil)
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true, "unlock_after": "later"}, 422, nil)

	// ---- Check off (stage 5 behaviour): «Завершить урок», no submission ----
	code(student, "POST", base+"/lessons/"+lids[2]+"/submit", nil, 409, "teacher_check_off")
	student.must("POST", base+"/lessons/"+lids[2]+"/complete", nil, 200, nil)
	student.must("POST", base+"/lessons/"+lids[2]+"/complete", nil, 200, nil) // idempotent
	if str(me(student), "items", 2, "status") != "completed" || events(sid, lids[2]) != 1 {
		t.Fatalf("self completion: %v", me(student)["items"])
	}

	// Progressive mode (new epoch) with the check on, "После проверки учителем".
	teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": 1, "confirm_reset": true}, 200, nil)
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true, "unlock_after": "teacher_check"}, 200, &m)
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true, "unlock_after": "teacher_check"}, 200, &m) // repeat
	if get(m, "marathon", "teacher_check") != true || num(m, "marathon", "progress_epoch") != 2 {
		t.Fatalf("settings: %v", m["marathon"])
	}
	mm := me(student)
	if str(mm, "items", 0, "access") != "open" || str(mm, "items", 1, "lock", "reason") != "previous_not_reviewed" || str(mm, "items", 2, "status") != "none" {
		t.Fatalf("progressive + check: %v", mm["items"])
	}
	code(student, "POST", base+"/lessons/"+lids[0]+"/complete", nil, 409, "teacher_check_required")
	code(student, "POST", "/api/classrooms/"+id+"/exercises/"+str(ex2, "id")+"/attempt", nil, 403, "marathon_lesson_locked")
	code(student, "POST", base+"/lessons/"+lids[1]+"/submit", nil, 403, "marathon_lesson_locked")
	teacher.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 403, nil)

	// ---- Student works on lesson 1: one right, one wrong (both checked) ----
	va := start(student, exA)
	if str(start(student, exA), "attempt", "id") != str(va, "attempt", "id") {
		t.Fatal("second start returned another attempt")
	}
	answer(student, va, 0, "is", 200)
	answer(student, va, 1, "is", 200) // wrong
	pointsBefore := get(review(sid, lids[0]), "points")

	// ---- Two submits at once: one snapshot, one event, the status is "submitted" ----
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i], _ = student.do("POST", base+"/lessons/"+lids[0]+"/submit", nil)
		}(i)
	}
	wg.Wait()
	if statuses[0] != 200 || statuses[1] != 200 {
		t.Fatalf("concurrent submit: %v", statuses)
	}
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil) // repeat while submitted
	if n := submissions(sid, lids[0]); n != 1 {
		t.Fatalf("snapshots after repeated submit: %d", n)
	}
	if n := events(sid, lids[0]); n != 1 {
		t.Fatalf("events after repeated submit: %d", n)
	}
	var snap json.RawMessage
	if err := e.pool.QueryRow(ctx, `SELECT snapshot FROM marathon_lesson_submissions WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3`,
		id, lids[0], sid).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	var snapEx []obj
	_ = json.Unmarshal(snap, &snapEx)
	if len(snapEx) != 2 || num(snapEx[0], "summary", "correct") != 1 || num(snapEx[0], "summary", "wrong") != 1 {
		t.Fatalf("snapshot: %s", snap)
	}
	mm = me(student)
	if str(mm, "items", 0, "status") != "submitted" || num(mm, "completed_lessons") != 0 || str(mm, "items", 1, "lock", "reason") != "previous_not_reviewed" {
		t.Fatalf("after submit: %v", mm)
	}
	if get(mm, "items", 0, "rating") != nil || get(mm, "items", 0, "pending_count") != float64(0) {
		t.Fatalf("student sees review fields: %v", mm["items"])
	}

	// ---- Answers are locked while the lesson is on review (direct API) ----
	code(student, "PUT", "/api/attempts/"+str(va, "attempt", "id")+"/answers/"+get(va, "version", "questions", 1).(map[string]any)["id"].(string),
		obj{"option_ids": []string{}, "check": false}, 409, "lesson_review_locked")
	code(student, "POST", "/api/classrooms/"+id+"/exercises/"+str(exB, "id")+"/attempt", nil, 409, "lesson_review_locked")
	student.must("POST", "/api/attempts/"+str(va, "attempt", "id")+"/finish", nil, 200, nil) // finished by the submit: same result
	var lv obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 200, &lv)
	for _, it := range lv["items"].([]any) {
		if it.(map[string]any)["locked"] != true {
			t.Fatalf("exercise not shown as locked: %v", it)
		}
	}
	// Sending finishes the attempts (finished count grows); the earned points are recomputed to the same value.
	if pts := get(review(sid, lids[0]), "points"); num(pts, "earned") != num(pointsBefore, "earned") || num(pts, "max") != num(pointsBefore, "max") || num(pts, "earned") != 5 {
		t.Fatalf("points changed by submit: %v → %v", pointsBefore, pts)
	}

	// ---- Teacher side: counters, rights, validation ----
	var list obj
	teacher.must("GET", "/api/classrooms?type=marathon", nil, 200, &list)
	if num(list, "items", 0, "homework_badge") != 1 {
		t.Fatalf("card badge: %v", list["items"])
	}
	var tl obj
	teacher.must("GET", base+"/lessons", nil, 200, &tl)
	if num(tl, "items", 0, "pending_count") != 1 || num(tl, "items", 0, "completed_count") != 0 {
		t.Fatalf("lesson counters: %v", tl["items"])
	}
	var sts obj
	teacher.must("GET", base+"/students", nil, 200, &sts)
	for _, s := range sts["items"].([]any) {
		want := 0
		if s.(map[string]any)["id"] == sid {
			want = 1
		}
		if num(s, "pending_lessons") != want {
			t.Fatalf("pending_lessons: %v", s)
		}
	}
	rv := review(sid, lids[0])
	if str(rv, "lesson", "status") != "submitted" || num(rv, "submissions_count") != 1 || get(rv, "teacher_check") != true {
		t.Fatalf("review view: %v", rv["lesson"])
	}
	accept := obj{"student_id": sid, "to_status": "completed", "rating": "amazing", "comment": "  Молодец!  "}
	decide(teacher2, lids[0], accept, 404)
	decide(student, lids[0], accept, 403)
	decide(teacher, lids[0], obj{"student_id": e.ids["teacher2"], "to_status": "revision"}, 404)
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "done"}, 422)
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "completed"}, 422)
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "completed", "rating": "great"}, 422)
	if out := decide(teacher, lids[0], obj{"student_id": sid, "to_status": "revision", "from_status": "none"}, 409); str(out, "error", "code") != "status_changed" {
		t.Fatalf("stale page: %v", out)
	}
	// student2 has not sent lesson 1: nothing to accept, revise or cancel.
	for _, to := range []obj{{"to_status": "completed", "rating": "nice"}, {"to_status": "revision"}, {"to_status": "submitted"}} {
		to["student_id"] = sid2
		if out := decide(teacher, lids[0], to, 409); str(out, "error", "code") != "transition_not_allowed" {
			t.Fatalf("%v: %v", to, out)
		}
	}

	// ---- Revision: only the wrong instantly-checked answer becomes editable ----
	rv = decide(teacher, lids[0], obj{"student_id": sid, "to_status": "revision", "from_status": "submitted"}, 200)
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "revision"}, 200) // repeat
	if str(rv, "lesson", "status") != "revision" || events(sid, lids[0]) != 2 {
		t.Fatalf("revision: %v, events %d", rv["lesson"], events(sid, lids[0]))
	}
	mm = me(student)
	if str(mm, "items", 0, "status") != "revision" || str(mm, "items", 1, "lock", "reason") != "previous_not_reviewed" {
		t.Fatalf("student after revision: %v", mm["items"])
	}
	if out := answer(student, va, 0, "are", 409); str(out, "error", "code") != "answer_locked" {
		t.Fatalf("correct answer must stay fixed: %v", out)
	}
	answer(student, va, 1, "are", 200) // the wrong one is corrected
	vb := start(student, exB)          // the second exercise can be started now
	qid, oid := option(vb, 0, "am")
	student.must("PUT", "/api/attempts/"+str(vb, "attempt", "id")+"/answers/"+qid, obj{"option_ids": []string{oid}}, 200, nil)
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil) // again; open attempts are finished
	if n := submissions(sid, lids[0]); n != 2 {
		t.Fatalf("second submission: %d", n)
	}

	// ---- Two accepts at once: one transition ----
	before := events(sid, lids[0])
	var wg2 sync.WaitGroup
	acc := make([]int, 2)
	for i := range acc {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			acc[i], _ = teacher.do("POST", base+"/lessons/"+lids[0]+"/review", accept)
		}(i)
	}
	wg2.Wait()
	if acc[0] != 200 || acc[1] != 200 || events(sid, lids[0]) != before+1 {
		t.Fatalf("concurrent accept: %v, events %d → %d", acc, before, events(sid, lids[0]))
	}
	rv = review(sid, lids[0])
	if str(rv, "lesson", "status") != "completed" || get(rv, "lesson", "accepted") != true || str(rv, "lesson", "rating") != "amazing" ||
		str(rv, "lesson", "teacher_comment") != "Молодец!" || num(rv, "points", "earned") != 11 {
		t.Fatalf("accepted: %v points %v", rv["lesson"], rv["points"])
	}
	pointsAccepted := get(rv, "points")
	mm = me(student)
	if str(mm, "items", 0, "status") != "completed" || str(mm, "items", 0, "rating") != "amazing" || str(mm, "items", 0, "teacher_comment") != "Молодец!" ||
		num(mm, "completed_lessons") != 1 || str(mm, "items", 1, "access") != "open" {
		t.Fatalf("student after acceptance: %v", mm)
	}
	// An accepted lesson stays locked; it cannot be sent again.
	code(student, "PUT", "/api/attempts/"+str(va, "attempt", "id")+"/answers/"+get(va, "version", "questions", 1).(map[string]any)["id"].(string),
		obj{"option_ids": []string{}}, 409, "lesson_review_locked")
	code(student, "POST", base+"/lessons/"+lids[0]+"/submit", nil, 409, "lesson_completed")
	teacher.must("GET", "/api/classrooms?type=marathon", nil, 200, &list)
	if num(list, "items", 0, "homework_badge") != 0 {
		t.Fatalf("badge after acceptance: %v", list["items"])
	}

	// The student starts lesson 2 (opened by the acceptance).
	v2 := start(student, ex2)
	answer(student, v2, 0, "is", 200)

	// ---- «Отменить завершение»: back to submitted, lesson 2 closes again, answers and points stay ----
	rv = decide(teacher, lids[0], obj{"student_id": sid, "to_status": "submitted", "from_status": "completed"}, 200)
	if str(rv, "lesson", "status") != "submitted" || get(rv, "lesson", "rating") != nil || str(rv, "lesson", "previous_rating") != "amazing" ||
		mustJSONs(get(rv, "points")) != mustJSONs(pointsAccepted) {
		t.Fatalf("cancelled: %v points %v", rv["lesson"], rv["points"])
	}
	mm = me(student)
	if str(mm, "items", 0, "status") != "submitted" || get(mm, "items", 0, "rating") != nil || get(mm, "items", 0, "teacher_comment") != nil ||
		num(mm, "completed_lessons") != 0 || str(mm, "items", 1, "lock", "reason") != "previous_not_reviewed" {
		t.Fatalf("student after cancel: %v", mm)
	}
	code(student, "PUT", "/api/attempts/"+str(v2, "attempt", "id")+"/answers/"+get(v2, "version", "questions", 0).(map[string]any)["id"].(string),
		obj{"option_ids": []string{}}, 403, "marathon_lesson_locked")
	if str(review(sid, lids[1]), "exercises", 0, "attempt", "id") != str(v2, "attempt", "id") {
		t.Fatal("lesson 2 attempt lost after cancel")
	}

	// ---- Second completion: progress and lesson 2 come back with the same attempt ----
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "completed", "rating": "nice"}, 200)
	mm = me(student)
	if str(mm, "items", 0, "rating") != "nice" || get(mm, "items", 0, "teacher_comment") != nil || num(mm, "completed_lessons") != 1 || str(mm, "items", 1, "access") != "open" {
		t.Fatalf("second completion: %v", mm)
	}
	if mustJSONs(get(review(sid, lids[0]), "points")) != mustJSONs(pointsAccepted) {
		t.Fatal("points changed by cancel and second completion")
	}
	student.must("GET", "/api/classrooms/"+id+"/exercises/"+str(ex2, "id"), nil, 200, nil)

	// ---- "После отправки на проверку": the next lesson opens with the submission ----
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true, "unlock_after": "submission"}, 200, nil)
	if str(me(student2), "items", 1, "lock", "reason") != "previous_not_submitted" {
		t.Fatalf("student2 before submit: %v", me(student2)["items"])
	}
	student2.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	if str(me(student2), "items", 1, "access") != "open" {
		t.Fatalf("submission must open the next lesson: %v", me(student2)["items"])
	}
	decide(teacher, lids[0], obj{"student_id": sid2, "to_status": "revision"}, 200)
	if str(me(student2), "items", 1, "access") != "open" {
		t.Fatalf("revision of a sent lesson keeps the next lesson open: %v", me(student2)["items"])
	}

	// ---- Check switched off: statuses stay; the student completes the returned lesson themselves ----
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": false}, 200, &m)
	if get(m, "marathon", "teacher_check") != false || num(m, "marathon", "progress_epoch") != 2 {
		t.Fatalf("check off: %v", m["marathon"])
	}
	if str(me(student2), "items", 0, "status") != "revision" || str(me(student2), "items", 1, "lock", "reason") != "previous_not_completed" {
		t.Fatalf("student2 with check off: %v", me(student2)["items"])
	}
	code(student2, "POST", base+"/lessons/"+lids[0]+"/submit", nil, 409, "teacher_check_off") // a returned lesson cannot be sent with the check off
	student2.must("POST", base+"/lessons/"+lids[0]+"/complete", nil, 200, nil)
	if mm := me(student2); str(mm, "items", 0, "status") != "completed" || get(mm, "items", 0, "accepted") != false || str(mm, "items", 1, "access") != "open" {
		t.Fatalf("self completion after revision: %v", mm["items"])
	}
	// Cancelling a lesson the student completed themselves is not a review decision.
	if out := decide(teacher, lids[0], obj{"student_id": sid2, "to_status": "submitted"}, 409); str(out, "error", "code") != "transition_not_allowed" {
		t.Fatalf("cancel of a self-completed lesson: %v", out)
	}
	// The accepted lesson of student 1 stays accepted and locked.
	if str(me(student), "items", 0, "rating") != "nice" {
		t.Fatal("accepted lesson changed when the check was switched off")
	}

	// ---- Deactivation and end date: the student cannot submit, the teacher still decides ----
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true, "unlock_after": "teacher_check"}, 200, nil)
	student.must("POST", base+"/lessons/"+lids[1]+"/submit", nil, 200, nil)
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, nil)
	code(student, "POST", base+"/lessons/"+lids[1]+"/submit", nil, 403, "marathon_deactivated")
	code(student, "POST", base+"/lessons/"+lids[2]+"/submit", nil, 403, "marathon_deactivated")
	decide(teacher, lids[1], obj{"student_id": sid, "to_status": "completed", "rating": "not_bad"}, 200)
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": true}, 200, nil)
	if str(me(student), "items", 1, "rating") != "not_bad" || str(me(student), "items", 2, "access") != "open" {
		t.Fatalf("after reactivation: %v", me(student)["items"])
	}
	teacher.must("PATCH", base+"/settings", obj{"end_at": time.Now().Add(-time.Minute).UTC()}, 200, nil)
	code(student2, "POST", base+"/lessons/"+lids[1]+"/submit", nil, 403, "marathon_closed")
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "submitted"}, 200) // the teacher is not limited by the end date
	teacher.must("PATCH", base+"/settings", obj{"end_at": nil}, 200, nil)
	decide(teacher, lids[0], obj{"student_id": sid, "to_status": "completed", "rating": "nice"}, 200)

	// ---- Removed lesson and removed student: 404 for the decision ----
	teacher.must("DELETE", base+"/students/"+sid2, nil, 200, nil)
	decide(teacher, lids[0], obj{"student_id": sid2, "to_status": "completed", "rating": "nice"}, 404)
	code(student2, "POST", base+"/lessons/"+lids[0]+"/submit", nil, 404, "not_found")

	// ---- Mode change: a new epoch resets statuses; answers and points stay ----
	pts := mustJSONs(get(review(sid, lids[0]), "points"))
	var cls obj
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &cls)
	teacher.must("POST", base+"/mode", obj{"mode": "open", "expected_epoch": num(cls, "marathon", "progress_epoch"), "confirm_reset": true}, 200, nil)
	mm = me(student)
	for i := range lids {
		if str(mm, "items", i, "status") != "none" || get(mm, "items", i, "rating") != nil {
			t.Fatalf("new epoch: %v", mm["items"])
		}
	}
	if mustJSONs(get(review(sid, lids[0]), "points")) != pts {
		t.Fatal("points changed by the mode change")
	}
	// In the new epoch the old acceptance does not lock: the teacher check works from the start.
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	if str(me(student), "items", 0, "status") != "submitted" {
		t.Fatal("submit in the new epoch")
	}
	if n := count(`SELECT count(*) FROM marathon_lesson_submissions WHERE classroom_id = $1 AND lesson_id = $2 AND student_id = $3 AND epoch = 3`, id, lids[0], sid); n != 1 {
		t.Fatalf("snapshot in the new epoch: %d", n)
	}
}
