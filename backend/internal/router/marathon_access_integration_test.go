package router_test

import (
	"testing"
	"time"
)

// Stage 4 of marathons: delayed start, individual auto-deactivation, GET /marathons/:cid/me and the single access
// decision (access.go) on every path — lesson list status, direct lesson URL and the attempts API, including an
// attempt started earlier. Progress, answers and points survive every refusal.
func TestMarathonAccessDates(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	student := e.login("student")
	ctx := t.Context()
	sid := e.assign(admin)

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон", "direction": "other"}, 201, &m)
	id := str(m, "id")
	settings := "/api/marathons/" + id + "/settings"
	var course, lesson, sec, ex1, ex2 obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок"}, 201, &lesson)
	lid := str(lesson, "id")
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Раздел"}, 201, &sec)
	for _, out := range []*obj{&ex1, &ex2} {
		teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест",
			"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 5},
			"questions": []obj{testQuestion("She ___ a teacher.", "is", "are")}}, 201, out)
	}
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": lid}, 200, nil)
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{sid}}, 200, nil)

	// ---- Validation of the passage settings ----
	now := time.Now().UTC().Truncate(time.Second)
	for _, body := range []obj{
		{"start_at": now.Add(2 * time.Hour), "end_at": now.Add(time.Hour)},
		{"start_at": now.Add(time.Hour), "end_at": now.Add(time.Hour)},
		{"auto_deactivate_days": 0},
		{"auto_deactivate_days": 3651},
	} {
		teacher.must("PATCH", settings, body, 422, nil)
	}
	student.must("GET", "/api/marathons/"+id+"/me", nil, 200, nil)
	teacher.must("GET", "/api/marathons/"+id+"/me", nil, 403, nil)

	// ---- Delayed start: lessons listed, everything else refused with opens_at ----
	start := now.Add(2 * time.Hour)
	teacher.must("PATCH", settings, obj{"start_at": start, "end_at": now.Add(72 * time.Hour)}, 200, &m)
	if get(m, "marathon", "started") != false || str(m, "marathon", "start_at") == "" {
		t.Fatalf("marathon info: %v", m["marathon"])
	}
	var me obj
	student.must("GET", "/api/marathons/"+id+"/me", nil, 200, &me)
	if str(me, "access", "status") != "not_started" || str(me, "items", 0, "access") != "not_started" || num(me, "total_lessons") != 1 {
		t.Fatalf("me before start: %v", me)
	}
	if got, _ := time.Parse(time.RFC3339, str(me, "access", "opens_at")); !got.Equal(start) {
		t.Fatalf("opens_at: %v", me["access"])
	}
	notStarted := func(method, path string, body any) {
		t.Helper()
		var out obj
		student.must(method, path, body, 403, &out)
		if str(out, "error", "code") != "marathon_not_started" || str(out, "error", "fields", "opens_at") == "" {
			t.Fatalf("%s %s: %v", method, path, out)
		}
	}
	notStarted("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/exercises", nil)
	notStarted("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/progress", nil)
	notStarted("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil)
	notStarted("POST", "/api/marathons/"+id+"/lessons/"+lid+"/complete", nil)
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, nil)
	teacher.must("GET", "/api/marathons/"+id+"/students/"+sid+"/lessons/"+lid, nil, 200, nil) // the owner is not limited

	// ---- Start reached: open; repeated start and finish are idempotent ----
	teacher.must("PATCH", settings, obj{"start_at": now.Add(-time.Minute), "end_at": now.Add(72 * time.Hour)}, 200, &m)
	if get(m, "marathon", "started") != true {
		t.Fatalf("started: %v", m["marathon"])
	}
	var a1, a1b, fin1, fin1b obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 200, &a1)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 200, &a1b)
	if str(a1, "attempt", "id") != str(a1b, "attempt", "id") {
		t.Fatalf("second start created another attempt")
	}
	q := get(a1, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(a1, "attempt", "id")+"/answers/"+q["id"].(string),
		obj{"option_ids": []string{str(q, "options", 0, "id")}, "check": true}, 200, nil)
	student.must("POST", "/api/attempts/"+str(a1, "attempt", "id")+"/finish", nil, 200, &fin1)
	student.must("POST", "/api/attempts/"+str(a1, "attempt", "id")+"/finish", nil, 200, &fin1b)
	// server_now is the response time, not part of the result.
	delete(get(fin1, "attempt").(map[string]any), "server_now")
	delete(get(fin1b, "attempt").(map[string]any), "server_now")
	if mustJSONs(get(fin1, "attempt")) != mustJSONs(get(fin1b, "attempt")) {
		t.Fatalf("finish twice differs: %v / %v", fin1, fin1b)
	}
	student.must("POST", "/api/marathons/"+id+"/lessons/"+lid+"/complete", nil, 200, nil)
	var a2 obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex2, "id")+"/attempt", nil, 200, &a2) // left in progress
	q2 := get(a2, "version", "questions", 0).(map[string]any)
	var pointsBefore obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/progress", nil, 200, &pointsBefore)

	// ---- Auto-deactivation: deadline = activated_at + N days, shown in /me and to the teacher ----
	teacher.must("PATCH", settings, obj{"start_at": now.Add(-time.Minute), "end_at": now.Add(72 * time.Hour), "auto_deactivate_days": 30}, 200, nil)
	student.must("GET", "/api/marathons/"+id+"/me", nil, 200, &me)
	if str(me, "access", "status") != "open" || str(me, "access", "deactivation_reason") != "end_date" {
		t.Fatalf("end date is earlier than 30 days: %v", me["access"])
	}
	// The student was activated 31 days ago (moved with SQL; the rule itself is covered by access_test.go).
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_enrollments SET activated_at = now() - interval '31 days' WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	student.must("GET", "/api/marathons/"+id+"/me", nil, 200, &me)
	if str(me, "access", "status") != "deactivated" || str(me, "access", "reason") != "auto_deactivation" || num(me, "completed_lessons") != 1 {
		t.Fatalf("me after auto-deactivation: %v", me)
	}
	auto := func(method, path string, body any) {
		t.Helper()
		var out obj
		student.must(method, path, body, 403, &out)
		if str(out, "error", "code") != "marathon_auto_deactivated" {
			t.Fatalf("%s %s: %v", method, path, out)
		}
	}
	auto("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/exercises", nil)
	auto("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil)
	auto("PUT", "/api/attempts/"+str(a2, "attempt", "id")+"/answers/"+q2["id"].(string), obj{"option_ids": []string{str(q2, "options", 0, "id")}})
	auto("POST", "/api/attempts/"+str(a2, "attempt", "id")+"/finish", nil)
	auto("POST", "/api/marathons/"+id+"/lessons/"+lid+"/complete", nil)
	var list obj
	teacher.must("GET", "/api/marathons/"+id+"/students", nil, 200, &list)
	if str(list, "items", 0, "status") != "inactive" || str(list, "items", 0, "inactive_reason") != "auto_deactivation" ||
		str(list, "items", 0, "access_status") != "deactivated" || num(list, "items", 0, "completed_lessons") != 1 {
		t.Fatalf("teacher list after auto-deactivation: %v", list)
	}

	// Manual deactivation wins (reason manual); manual activation does not override the auto period.
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, &list)
	if str(list, "items", 0, "inactive_reason") != "manual" {
		t.Fatalf("manual over auto: %v", list)
	}
	var out obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 403, &out)
	if str(out, "error", "code") != "marathon_deactivated" {
		t.Fatalf("manual code: %v", out)
	}
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}, "active": true}, 200, &list)
	if str(list, "items", 0, "inactive_reason") != "auto_deactivation" {
		t.Fatalf("activation must not override the auto period: %v", list)
	}
	auto("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil)

	// ---- A longer period gives access back: progress, points and the started attempt are intact ----
	teacher.must("PATCH", settings, obj{"start_at": now.Add(-time.Minute), "end_at": now.Add(72 * time.Hour), "auto_deactivate_days": 60}, 200, nil)
	student.must("GET", "/api/marathons/"+id+"/me", nil, 200, &me)
	if str(me, "access", "status") != "open" || num(me, "completed_lessons") != 1 {
		t.Fatalf("me after extending: %v", me)
	}
	var pointsAfter obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/progress", nil, 200, &pointsAfter)
	if mustJSONs(pointsAfter) != mustJSONs(pointsBefore) {
		t.Fatalf("points changed: %v → %v", pointsBefore, pointsAfter)
	}
	student.must("PUT", "/api/attempts/"+str(a2, "attempt", "id")+"/answers/"+q2["id"].(string), obj{"option_ids": []string{str(q2, "options", 0, "id")}}, 200, nil)
	student.must("POST", "/api/attempts/"+str(a2, "attempt", "id")+"/finish", nil, 200, nil)

	// Switching settings off: null removes start, end and auto period.
	teacher.must("PATCH", settings, obj{"start_at": nil, "end_at": nil, "auto_deactivate_days": nil}, 200, &m)
	if get(m, "marathon", "start_at") != nil || get(m, "marathon", "end_at") != nil || get(m, "marathon", "auto_deactivate_days") != nil {
		t.Fatalf("switched off: %v", m["marathon"])
	}
	student.must("GET", "/api/marathons/"+id+"/me", nil, 200, &me)
	if str(me, "access", "status") != "open" || get(me, "access", "deactivates_at") != nil {
		t.Fatalf("no dates: %v", me["access"])
	}
}
