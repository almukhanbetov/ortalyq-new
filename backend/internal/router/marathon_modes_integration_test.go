package router_test

import (
	"testing"
	"time"
)

// Stage 5 of marathons: lesson opening date (open mode), progressive (without teacher check), closed with
// manual opening for everyone or one student, and the mode change through the progress epoch. Every refusal
// is checked through direct API calls; answers, attempts and points survive a mode change.
func TestMarathonModes(t *testing.T) {
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
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон", "direction": "other"}, 201, &m)
	id := str(m, "id")
	if get(m, "marathon", "mode") != "open" || num(m, "marathon", "progress_epoch") != 1 {
		t.Fatalf("new marathon: %v", m["marathon"])
	}
	var course obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	lids, exs := []string{}, []string{}
	for _, title := range []string{"Урок 1", "Урок 2"} {
		var l, sec, ex obj
		teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": title}, 201, &l)
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/sections", obj{"title": "Раздел"}, 201, &sec)
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест",
			"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 5},
			"questions": []obj{testQuestion("She ___ a teacher.", "is", "are")}}, 201, &ex)
		teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": str(l, "id")}, 200, nil)
		lids, exs = append(lids, str(l, "id")), append(exs, str(ex, "id"))
	}
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{sid, sid2}}, 200, nil)
	base := "/api/marathons/" + id

	locked := func(c *client, method, path string, body any, reason string) {
		t.Helper()
		var out obj
		c.must(method, path, body, 403, &out)
		if str(out, "error", "code") != "marathon_lesson_locked" || str(out, "error", "fields", "reason") != reason {
			t.Fatalf("%s %s: %v", method, path, out)
		}
	}
	answer := func(c *client, ex string) string {
		t.Helper()
		var v obj
		c.must("POST", "/api/classrooms/"+id+"/exercises/"+ex+"/attempt", nil, 200, &v)
		q := get(v, "version", "questions", 0).(map[string]any)
		c.must("PUT", "/api/attempts/"+str(v, "attempt", "id")+"/answers/"+q["id"].(string),
			obj{"option_ids": []string{str(q, "options", 0, "id")}, "check": true}, 200, nil)
		c.must("POST", "/api/attempts/"+str(v, "attempt", "id")+"/finish", nil, 200, nil)
		return str(v, "attempt", "id")
	}

	// ---- Mode change: rights and validation ----
	teacher2.must("POST", base+"/mode", obj{"mode": "closed", "expected_epoch": 1, "confirm_reset": true}, 404, nil)
	student.must("POST", base+"/mode", obj{"mode": "closed", "expected_epoch": 1, "confirm_reset": true}, 403, nil)
	for _, body := range []obj{
		{"mode": "interval", "expected_epoch": 1, "confirm_reset": true},                                                // no schedule
		{"mode": "interval", "expected_epoch": 1, "confirm_reset": true, "interval_days": 8, "open_time": "09:00"},      // 1..7
		{"mode": "interval", "expected_epoch": 1, "confirm_reset": true, "interval_days": 1, "open_time": "09:30"},      // whole hour
		{"mode": "weekdays", "expected_epoch": 1, "confirm_reset": true, "weekdays": []int{}, "open_time": "09:00"},     // no days
		{"mode": "weekdays", "expected_epoch": 1, "confirm_reset": true, "weekdays": []int{0, 8}, "open_time": "09:00"}, // unknown days
		{"mode": "random", "expected_epoch": 1, "confirm_reset": true},
		{"mode": "closed", "expected_epoch": 1}, // the reset was not confirmed
	} {
		teacher.must("POST", base+"/mode", body, 422, nil)
	}

	// ---- Open mode: lesson opening date ----
	future := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)
	teacher2.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": future}, 404, nil)
	teacher.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": future}, 200, nil)
	var me obj
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 0, "access") != "open" || str(me, "items", 1, "access") != "locked" || str(me, "items", 1, "lock", "reason") != "opens_at" {
		t.Fatalf("me with an opening date: %v", me["items"])
	}
	locked(student, "GET", "/api/classrooms/"+id+"/lessons/"+lids[1]+"/exercises", nil, "opens_at")
	locked(student, "POST", "/api/classrooms/"+id+"/exercises/"+exs[1]+"/attempt", nil, "opens_at")
	locked(student, "POST", base+"/lessons/"+lids[1]+"/complete", nil, "opens_at")
	teacher.must("GET", base+"/students/"+sid+"/lessons/"+lids[1], nil, 200, nil) // the owner is not limited
	// A mode without lesson dates (closed) needs an explicit confirmation of the reset (Н3 = A); nothing changes without it.
	var conflict obj
	teacher.must("POST", base+"/mode", obj{"mode": "closed", "expected_epoch": 1, "confirm_reset": true}, 409, &conflict)
	if str(conflict, "error", "code") != "lesson_dates_set" {
		t.Fatalf("dates block: %v", conflict)
	}
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if get(m, "marathon", "mode") != "open" || num(m, "marathon", "progress_epoch") != 1 {
		t.Fatalf("refused change must not apply: %v", m["marathon"])
	}
	// A passed date opens the lesson; null removes it.
	teacher.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": time.Now().Add(-time.Minute).UTC()}, 200, nil)
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[1]+"/exercises", nil, 200, nil)
	teacher.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": nil}, 200, nil)

	// The student completes lesson 1 in epoch 1.
	answer(student, exs[0])
	student.must("POST", base+"/lessons/"+lids[0]+"/complete", nil, 200, nil)
	var pointsBefore obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/progress", nil, 200, &pointsBefore)

	// ---- Change to progressive: new epoch, statuses start over, answers stay; a repeat is harmless ----
	teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": 1, "confirm_reset": true}, 200, &m)
	if get(m, "marathon", "mode") != "progressive" || num(m, "marathon", "progress_epoch") != 2 {
		t.Fatalf("after change: %v", m["marathon"])
	}
	teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": 1, "confirm_reset": true}, 200, &m)
	if num(m, "marathon", "progress_epoch") != 2 {
		t.Fatalf("repeat reset progress again: %v", m["marathon"])
	}
	teacher.must("POST", base+"/mode", obj{"mode": "closed", "expected_epoch": 1, "confirm_reset": true}, 409, &conflict)
	if str(conflict, "error", "code") != "settings_changed" {
		t.Fatalf("stale epoch: %v", conflict)
	}
	student.must("GET", base+"/me", nil, 200, &me)
	if num(me, "completed_lessons") != 0 || get(me, "items", 0, "completed") != false {
		t.Fatalf("statuses must start over: %v", me)
	}
	var pointsAfter obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/progress", nil, 200, &pointsAfter)
	if mustJSONs(pointsAfter) != mustJSONs(pointsBefore) {
		t.Fatalf("answers and points must stay: %v → %v", pointsBefore, pointsAfter)
	}
	var oldRows int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_lesson_progress WHERE classroom_id = $1 AND epoch = 1`, id).Scan(&oldRows); err != nil || oldRows != 1 {
		t.Fatalf("history of epoch 1: %d %v", oldRows, err)
	}
	teacher.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": future}, 200, nil) // progressive uses dates (Н3 = A)

	// ---- Progressive: lesson 2 opens after lesson 1 is completed in this epoch ----
	if str(me, "items", 0, "access") != "open" || str(me, "items", 1, "lock", "reason") != "previous_not_completed" {
		t.Fatalf("progressive me: %v", me["items"])
	}
	locked(student, "GET", "/api/classrooms/"+id+"/lessons/"+lids[1]+"/exercises", nil, "previous_not_completed")
	locked(student, "POST", "/api/classrooms/"+id+"/exercises/"+exs[1]+"/attempt", nil, "previous_not_completed")
	locked(student, "POST", base+"/lessons/"+lids[1]+"/complete", nil, "previous_not_completed")
	student.must("POST", base+"/lessons/"+lids[0]+"/complete", nil, 200, nil)
	// Both conditions: lesson 1 completed, but the date of lesson 2 has not come yet.
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 1, "lock", "reason") != "opens_at" {
		t.Fatalf("progressive with a future date: %v", me["items"])
	}
	locked(student, "POST", "/api/classrooms/"+id+"/exercises/"+exs[1]+"/attempt", nil, "opens_at")
	teacher.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": time.Now().Add(-time.Second).UTC()}, 200, nil)
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 1, "access") != "open" || num(me, "completed_lessons") != 1 {
		t.Fatalf("lesson 2 after completing lesson 1 and its date: %v", me)
	}
	// The other student's progress is their own.
	locked(student2, "POST", "/api/classrooms/"+id+"/exercises/"+exs[1]+"/attempt", nil, "previous_not_completed")
	// Student starts lesson 2 and leaves the attempt in progress.
	var live obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exs[1]+"/attempt", nil, 200, &live)
	lq := get(live, "version", "questions", 0).(map[string]any)

	// ---- Change to closed: lesson 2 still has a date → reset only with confirmation; then everything is closed ----
	teacher.must("POST", base+"/mode", obj{"mode": "closed", "expected_epoch": 2, "confirm_reset": true}, 409, &conflict)
	teacher.must("POST", base+"/mode", obj{"mode": "closed", "expected_epoch": 2, "confirm_reset": true, "confirm_reset_dates": true}, 200, &m)
	var datesLeft, resetLogged int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_lessons WHERE classroom_id = $1 AND open_at IS NOT NULL`, id).Scan(&datesLeft); err != nil || datesLeft != 0 {
		t.Fatalf("dates reset: %d %v", datesLeft, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT dates_reset FROM marathon_mode_changes WHERE classroom_id = $1 AND to_mode = 'closed'`, id).Scan(&resetLogged); err != nil || resetLogged != 1 {
		t.Fatalf("dates reset logged: %d %v", resetLogged, err)
	}
	if num(m, "marathon", "progress_epoch") != 3 {
		t.Fatalf("closed: %v", m["marathon"])
	}
	locked(student, "GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, "not_unlocked")
	// The attempt started before the change cannot be continued while the lesson is closed.
	locked(student, "PUT", "/api/attempts/"+str(live, "attempt", "id")+"/answers/"+lq["id"].(string), obj{"option_ids": []string{str(lq, "options", 0, "id")}}, "not_unlocked")
	locked(student, "POST", "/api/attempts/"+str(live, "attempt", "id")+"/finish", nil, "not_unlocked")

	teacher2.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{}, 404, nil)
	student.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{}, 403, nil)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{"student_id": e.ids["teacher2"]}, 422, nil)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{}, 200, nil) // for everyone
	teacher.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{}, 200, nil) // repeat
	teacher.must("POST", base+"/lessons/"+lids[1]+"/unlock", obj{"student_id": sid}, 200, nil)
	var unlocks int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_lesson_unlocks WHERE classroom_id = $1`, id).Scan(&unlocks); err != nil || unlocks != 2 {
		t.Fatalf("unlocks are idempotent: %d %v", unlocks, err)
	}
	var lessons obj
	teacher.must("GET", base+"/lessons", nil, 200, &lessons)
	if get(lessons, "items", 0, "unlocked_for_all") != true || get(lessons, "items", 1, "unlocked_for_all") != false {
		t.Fatalf("teacher list: %v", lessons["items"])
	}
	var pr obj
	teacher.must("GET", base+"/students/"+sid2+"/progress", nil, 200, &pr)
	if get(pr, "items", 0, "student_lesson", "open") != true || str(pr, "items", 1, "student_lesson", "reason") != "not_unlocked" {
		t.Fatalf("progress window of student2: %v", pr["items"])
	}
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 200, nil)
	student2.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 200, nil)
	student.must("PUT", "/api/attempts/"+str(live, "attempt", "id")+"/answers/"+lq["id"].(string), obj{"option_ids": []string{str(lq, "options", 0, "id")}}, 200, nil)
	student.must("POST", "/api/attempts/"+str(live, "attempt", "id")+"/finish", nil, 200, nil)
	locked(student2, "GET", "/api/classrooms/"+id+"/lessons/"+lids[1]+"/exercises", nil, "not_unlocked")

	// ---- An opened lesson never overrides a deactivation or the end date ----
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, nil)
	var out obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 403, &out)
	if str(out, "error", "code") != "marathon_deactivated" {
		t.Fatalf("deactivated: %v", out)
	}
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": true}, 200, nil)
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_settings SET end_at = now() - interval '1 minute' WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 403, &out)
	if str(out, "error", "code") != "marathon_closed" {
		t.Fatalf("end date: %v", out)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_settings SET end_at = NULL WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	teacher.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{}, 200, nil)

	// ---- Back to open: everything opens, statuses start over again, attempts are all there ----
	teacher.must("POST", base+"/mode", obj{"mode": "open", "expected_epoch": 3, "confirm_reset": true}, 200, &m)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/unlock", obj{}, 422, nil) // only in the closed mode
	student2.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[1]+"/exercises", nil, 200, nil)
	var attempts int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM attempts WHERE classroom_id = $1 AND student_id = $2`, id, sid).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("attempts kept: %d %v", attempts, err)
	}
	var changes int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_mode_changes WHERE classroom_id = $1 AND changed_by = $2`, id, e.ids["teacher"]).Scan(&changes); err != nil || changes != 3 {
		t.Fatalf("mode history: %d %v", changes, err)
	}
}
