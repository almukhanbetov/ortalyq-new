package router_test

import (
	"testing"
)

// Stage 8: the teacher's lesson preview. The owner takes a lesson with separate preview attempts (timer, instant
// check, finish, re-entry, reset of one exercise and of the lesson). Rights through direct API calls. Isolation:
// every student-facing number and every student table is the same before and after the preview.
func TestMarathonPreview(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	sid := e.assign(admin)
	ctx := t.Context()

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон с предпросмотром", "direction": "other"}, 201, &m)
	id := str(m, "id")
	base := "/api/marathons/" + id
	var course obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	lids := []string{}
	var exA, exT, exH obj
	for i, title := range []string{"Урок 1", "Урок 2"} {
		var l, sec obj
		teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": title}, 201, &l)
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/sections", obj{"title": "Раздел"}, 201, &sec)
		var ex obj
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест",
			"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 5},
			"questions": []obj{testQuestion("She ___ a teacher.", "is", "are"), testQuestion("They ___ friends.", "are", "is")}}, 201, &ex)
		if i == 0 {
			exA = ex
			teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "На время",
				"settings":  obj{"check_mode": "after", "scoring_enabled": true, "points_per_correct": 1, "timer_enabled": true, "timer_seconds": 60},
				"questions": []obj{testQuestion("I ___ here.", "am", "is")}}, 201, &exT)
			teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Скрытое",
				"settings":  obj{"check_mode": "instant"},
				"questions": []obj{testQuestion("We ___ here.", "are", "is")}}, 201, &exH)
			teacher.must("PATCH", "/api/exercises/"+str(exH, "id")+"/hidden", obj{"hidden": true}, 204, nil)
		}
		teacher.must("POST", base+"/lessons", obj{"lesson_id": str(l, "id")}, 200, nil)
		lids = append(lids, str(l, "id"))
	}
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid}}, 200, nil)
	teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": 1, "confirm_reset": true}, 200, nil)
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true}, 200, nil)
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": true, "rating_enabled": true}, 200, nil)

	// The student's real work: lesson 1, one right and one wrong, sent for review.
	var sv obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(exA, "id")+"/attempt", nil, 200, &sv)
	opt := func(v obj, qi int, label string) (string, string) {
		q := get(v, "version", "questions", qi).(map[string]any)
		for _, o := range q["options"].([]any) {
			if o.(map[string]any)["label"] == label {
				return q["id"].(string), o.(map[string]any)["id"].(string)
			}
		}
		t.Fatalf("no option %s", label)
		return "", ""
	}
	for qi, label := range []string{"is", "is"} {
		q, o := opt(sv, qi, label)
		student.must("PUT", "/api/attempts/"+str(sv, "attempt", "id")+"/answers/"+q, obj{"option_ids": []string{o}, "check": true}, 200, nil)
	}
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)

	// snapshot of everything a preview must not change: API answers and the student tables.
	snapshot := func() string {
		t.Helper()
		out := ""
		for _, r := range []struct {
			c    *client
			path string
		}{
			{teacher, base + "/stats"}, {teacher, base + "/rating"}, {student, base + "/rating"}, {student, base + "/me"},
			{student, base + "/me/results"}, {teacher, base + "/lessons"}, {teacher, base + "/students"},
			{teacher, "/api/classrooms?type=marathon"}, {teacher, base + "/lessons/" + lids[0] + "/results"},
			{teacher, "/api/exercises/" + str(exA, "id") + "/stats"}, {teacher, "/api/exercises/" + str(exT, "id") + "/stats"},
			{teacher, base + "/students/" + sid + "/lessons/" + lids[0]},
			{student, "/api/classrooms/" + id + "/lessons/" + lids[0] + "/exercises?scope=lesson"},
		} {
			_, b := r.c.do("GET", r.path, nil)
			out += r.path + "=" + string(b) + "\n"
		}
		for _, table := range []string{"attempts", "attempt_answers", "marathon_lesson_progress", "marathon_lesson_submissions", "marathon_lesson_events", "homeworks"} {
			var h string
			if err := e.pool.QueryRow(ctx, `SELECT COALESCE(md5(string_agg(x::text, ',' ORDER BY x::text)), '') FROM `+table+` x`).Scan(&h); err != nil {
				t.Fatal(err)
			}
			out += table + "=" + h + "\n"
		}
		return out
	}
	// server_now fields differ between calls; drop them before comparing.
	clean := func(s string) string {
		for {
			i := indexOf(s, `"server_now":"`)
			if i < 0 {
				return s
			}
			j := indexOf(s[i+14:], `"`)
			s = s[:i] + s[i+14+j+1:]
		}
	}
	before := clean(snapshot())

	pv := "/api/preview"
	// ---- Rights ----
	student.must("GET", pv+"/lessons/"+lids[0]+"/exercises", nil, 403, nil)
	admin.must("GET", pv+"/lessons/"+lids[0]+"/exercises", nil, 403, nil)
	teacher2.must("GET", pv+"/lessons/"+lids[0]+"/exercises", nil, 404, nil)
	teacher2.must("POST", pv+"/exercises/"+str(exA, "id")+"/attempt", nil, 404, nil)
	student.must("POST", pv+"/exercises/"+str(exA, "id")+"/attempt", nil, 403, nil)
	teacher.must("POST", pv+"/exercises/"+str(exH, "id")+"/attempt", nil, 404, nil) // hidden: not what a student sees
	// The teacher still cannot use the students' attempts.
	teacher.must("POST", "/api/classrooms/"+id+"/exercises/"+str(exA, "id")+"/attempt", nil, 403, nil)

	// ---- The preview list: visible exercises, the key hidden as for a student ----
	var list obj
	teacher.must("GET", pv+"/lessons/"+lids[0]+"/exercises", nil, 200, &list)
	items := list["items"].([]any)
	if len(items) != 2 || get(items[0], "attempt") != nil {
		t.Fatalf("preview list: %v", items)
	}
	if _, has := get(items[0], "version", "questions", 0, "options", 0).(map[string]any)["is_correct"]; has {
		t.Fatalf("the key is visible before answering: %v", get(items[0], "version", "questions", 0))
	}

	// ---- Taking the lesson: idempotent start, instant check, fixed answer, finish ----
	var v obj
	teacher.must("POST", pv+"/exercises/"+str(exA, "id")+"/attempt", nil, 200, &v)
	var v2 obj
	teacher.must("POST", pv+"/exercises/"+str(exA, "id")+"/attempt", nil, 200, &v2)
	aid := str(v, "attempt", "id")
	if aid == "" || str(v2, "attempt", "id") != aid {
		t.Fatalf("start twice: %v / %v", v["attempt"], v2["attempt"])
	}
	q0, right := opt(v, 0, "is")
	_, wrong := opt(v, 0, "are")
	teacher.must("PUT", pv+"/attempts/"+aid+"/answers/"+q0, obj{"option_ids": []string{right}, "check": true}, 200, &v)
	if str(v, "attempt", "answers", 0, "result") != "correct" {
		t.Fatalf("instant check: %v", v["attempt"])
	}
	teacher.must("PUT", pv+"/attempts/"+aid+"/answers/"+q0, obj{"option_ids": []string{right}, "check": true}, 200, nil) // same answer again
	var lockedErr obj
	teacher.must("PUT", pv+"/attempts/"+aid+"/answers/"+q0, obj{"option_ids": []string{wrong}, "check": true}, 409, &lockedErr)
	if str(lockedErr, "error", "code") != "answer_locked" {
		t.Fatalf("checked answer must stay: %v", lockedErr)
	}
	teacher2.must("PUT", pv+"/attempts/"+aid+"/answers/"+q0, obj{"option_ids": []string{right}}, 404, nil)
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+q0, obj{"option_ids": []string{right}}, 404, nil) // not a student attempt
	teacher.must("POST", pv+"/attempts/"+aid+"/finish", nil, 200, &v)
	teacher.must("POST", pv+"/attempts/"+aid+"/finish", nil, 200, &v2)
	if str(v, "attempt", "status") != "finished" || num(v, "attempt", "summary", "points_earned") != 5 || clean(mustJSONs(v["attempt"])) != clean(mustJSONs(v2["attempt"])) {
		t.Fatalf("finish: %v", v["attempt"])
	}

	// ---- Timer: a deadline in the past finishes the attempt with the deadline as its time ----
	var tv obj
	teacher.must("POST", pv+"/exercises/"+str(exT, "id")+"/attempt", nil, 200, &tv)
	if str(tv, "attempt", "deadline_at") == "" {
		t.Fatalf("timed preview without a deadline: %v", tv["attempt"])
	}
	if _, err := e.pool.Exec(ctx, `UPDATE preview_attempts SET started_at = now() - interval '61 seconds', deadline_at = now() - interval '1 second' WHERE id = $1`, str(tv, "attempt", "id")); err != nil {
		t.Fatal(err)
	}
	qt, ot := opt(tv, 0, "am")
	var exp obj
	teacher.must("PUT", pv+"/attempts/"+str(tv, "attempt", "id")+"/answers/"+qt, obj{"option_ids": []string{ot}}, 409, &exp)
	if str(exp, "error", "code") != "attempt_expired" {
		t.Fatalf("expired: %v", exp)
	}
	teacher.must("GET", pv+"/exercises/"+str(exT, "id"), nil, 200, &tv)
	if str(tv, "attempt", "status") != "finished" || str(tv, "attempt", "finish_reason") != "timeout" || num(tv, "attempt", "summary", "elapsed_seconds") != 60 {
		t.Fatalf("timeout: %v", tv["attempt"])
	}

	// ---- Re-entry: the same attempts and answers come back ----
	teacher.must("GET", pv+"/lessons/"+lids[0]+"/exercises", nil, 200, &list)
	if str(list, "items", 0, "attempt", "id") != aid || str(list, "items", 0, "attempt", "status") != "finished" || str(list, "items", 1, "attempt", "finish_reason") != "timeout" {
		t.Fatalf("re-entry: %v", list["items"])
	}
	// The preview of a lesson closed for the student (lesson 2 in the progressive mode) works for the owner.
	teacher.must("GET", pv+"/lessons/"+lids[1]+"/exercises", nil, 200, nil)

	// ---- Isolation: nothing a student, the statistics, the rating, the review or the progress shows has changed ----
	if after := clean(snapshot()); after != before {
		t.Fatalf("the preview changed student data:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	var me obj
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 1, "access") != "locked" || num(me, "completed_lessons") != 0 {
		t.Fatalf("the preview opened the next lesson or completed one: %v", me)
	}
	var cnt int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM attempts WHERE student_id = $1`, e.ids["teacher"]).Scan(&cnt); err != nil || cnt != 0 {
		t.Fatalf("teacher rows in attempts: %d %v", cnt, err)
	}

	// ---- Reset of one exercise: a new attempt can start, the old one is archived ----
	teacher.must("POST", pv+"/attempts/"+aid+"/reset", nil, 200, &v)
	teacher.must("POST", pv+"/attempts/"+aid+"/reset", nil, 200, nil) // repeat
	if get(v, "attempt") != nil || num(v, "reset_count") != 1 {
		t.Fatalf("reset: %v", v)
	}
	teacher2.must("POST", pv+"/attempts/"+str(tv, "attempt", "id")+"/reset", nil, 404, nil)
	teacher.must("POST", pv+"/exercises/"+str(exA, "id")+"/attempt", nil, 200, &v)
	if str(v, "attempt", "id") == aid || len(get(v, "attempt", "answers").([]any)) != 0 {
		t.Fatalf("new attempt after reset: %v", v["attempt"])
	}
	teacher.must("POST", pv+"/attempts/"+aid+"/finish", nil, 409, nil) // the archived attempt is gone

	// ---- «Сбросить предпросмотр» for the whole lesson ----
	teacher2.must("POST", pv+"/lessons/"+lids[0]+"/reset", nil, 404, nil)
	student.must("POST", pv+"/lessons/"+lids[0]+"/reset", nil, 403, nil)
	teacher.must("POST", pv+"/lessons/"+lids[0]+"/reset", nil, 200, &list)
	for _, it := range list["items"].([]any) {
		if get(it, "attempt") != nil {
			t.Fatalf("lesson reset left an attempt: %v", it)
		}
	}
	if after := clean(snapshot()); after != before {
		t.Fatalf("resetting the preview changed student data")
	}

	// The student keeps working as before: the review decision still applies to their real answers.
	teacher.must("POST", base+"/lessons/"+lids[0]+"/review", obj{"student_id": sid, "to_status": "completed", "rating": "nice"}, 200, nil)
	student.must("GET", base+"/me", nil, 200, &me)
	if num(me, "completed_lessons") != 1 || str(me, "items", 1, "access") != "open" {
		t.Fatalf("after the real review: %v", me)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
