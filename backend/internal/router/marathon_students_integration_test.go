package router_test

import (
	"testing"
	"time"
)

// Stage 3 of marathons: manual deactivation and activation (bulk, with date and author), removal from the
// marathon, and the server-side lock of a deactivated student (stage 4 requirement): the lesson list, a direct
// lesson URL and the attempts API — including an attempt started before the deactivation — are refused.
// Activation gives everything back with answers, points and progress intact. Nothing is deleted.
func TestMarathonStudentsStatus(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2")
	ctx := t.Context()
	e.assign(admin)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	sid, sid2 := e.ids["student"], e.ids["student2"]

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон", "direction": "other"}, 201, &m)
	id := str(m, "id")
	future := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	teacher.must("PATCH", "/api/marathons/"+id+"/settings", obj{"end_at": future}, 200, nil)

	// One lesson with two tests.
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
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{sid, sid2}}, 200, nil)

	// The student finishes exercise 1 and the lesson, and starts exercise 2 (left in progress).
	var v1, v2 obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 200, &v1)
	q1 := get(v1, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(v1, "attempt", "id")+"/answers/"+q1["id"].(string),
		obj{"option_ids": []string{str(q1, "options", 0, "id")}, "check": true}, 200, nil)
	student.must("POST", "/api/attempts/"+str(v1, "attempt", "id")+"/finish", nil, 200, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+lid+"/complete", nil, 200, nil)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex2, "id")+"/attempt", nil, 200, &v2)
	aid2 := str(v2, "attempt", "id")
	q2 := get(v2, "version", "questions", 0).(map[string]any)
	var pointsBefore obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/progress", nil, 200, &pointsBefore)

	// ---- Rights and validation ----
	body := obj{"student_ids": []string{sid}, "active": false}
	teacher2.must("PUT", "/api/marathons/"+id+"/students/active", body, 404, nil)
	student.must("PUT", "/api/marathons/"+id+"/students/active", body, 403, nil)
	admin.must("PUT", "/api/marathons/"+id+"/students/active", body, 403, nil)
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{}, "active": false}, 422, nil)
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}}, 422, nil)
	// A non-participant in the list: nothing changes for anyone.
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid, e.ids["teacher2"]}, "active": false}, 422, nil)
	var list obj
	teacher.must("GET", "/api/marathons/"+id+"/students", nil, 200, &list)
	for _, it := range get(list, "items").([]any) {
		if get(it, "deactivated_at") != nil {
			t.Fatalf("partial bulk change: %v", list)
		}
	}

	// ---- Bulk deactivation of both students ----
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid, sid2, sid}, "active": false}, 200, &list)
	byID := func(l obj, who string) any {
		for _, it := range get(l, "items").([]any) {
			if str(it, "id") == who {
				return it
			}
		}
		t.Fatalf("student %s not listed: %v", who, l)
		return nil
	}
	s1 := byID(list, sid)
	if str(s1, "status") != "inactive" || str(s1, "inactive_reason") != "manual" || str(s1, "deactivation_reason") != "manual" ||
		str(s1, "deactivated_by_name") != "Teacher" || get(s1, "deactivated_at") == nil || num(s1, "completed_lessons") != 1 {
		t.Fatalf("deactivated student: %v", s1)
	}
	firstAt := str(s1, "deactivated_at")
	// Repeating keeps the first date and author.
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, &list)
	if str(byID(list, sid), "deactivated_at") != firstAt {
		t.Fatalf("repeat changed the date: %v", list)
	}
	var pr obj
	teacher.must("GET", "/api/marathons/"+id+"/students/"+sid+"/progress", nil, 200, &pr)
	if str(pr, "student", "status") != "inactive" || num(pr, "completed_lessons") != 1 {
		t.Fatalf("progress window: %v", pr)
	}
	// The owner still reads the answers.
	teacher.must("GET", "/api/marathons/"+id+"/students/"+sid+"/lessons/"+lid, nil, 200, nil)

	// ---- The deactivated student is refused everywhere (403 marathon_deactivated) ----
	deny := func(method, path string, body any) {
		t.Helper()
		var out obj
		student.must(method, path, body, 403, &out)
		if str(out, "error", "code") != "marathon_deactivated" {
			t.Fatalf("%s %s: %v", method, path, out)
		}
	}
	deny("GET", "/api/marathons/"+id+"/lessons", nil)
	deny("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/exercises", nil)
	deny("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/progress", nil)
	deny("GET", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id"), nil)
	deny("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil)
	deny("POST", "/api/marathons/"+id+"/lessons/"+lid+"/complete", nil)
	// The attempt started before the deactivation cannot be answered or finished.
	deny("PUT", "/api/attempts/"+aid2+"/answers/"+q2["id"].(string), obj{"option_ids": []string{str(q2, "options", 0, "id")}})
	deny("POST", "/api/attempts/"+aid2+"/finish", nil)
	var status string
	if err := e.pool.QueryRow(ctx, `SELECT status FROM attempts WHERE id = $1`, aid2).Scan(&status); err != nil || status != "in_progress" {
		t.Fatalf("attempt must stay untouched: %s %v", status, err)
	}
	// The class card stays in the student's list (they learn why on the page).
	student.must("GET", "/api/classrooms/"+id, nil, 200, nil)

	// ---- Activation of one student: access, answers, points and progress come back ----
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}, "active": true}, 200, &list)
	if s := byID(list, sid); str(s, "status") != "active" || get(s, "deactivated_at") != nil || get(s, "reactivated_at") == nil || num(s, "completed_lessons") != 1 {
		t.Fatalf("activated student: %v", s)
	}
	if str(byID(list, sid2), "status") != "inactive" {
		t.Fatalf("the other student must stay deactivated: %v", list)
	}
	var mine, pointsAfter, view obj
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, &mine)
	if num(mine, "completed_lessons") != 1 {
		t.Fatalf("progress after activation: %v", mine)
	}
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lid+"/progress", nil, 200, &pointsAfter)
	if mustJSONs(pointsAfter) != mustJSONs(pointsBefore) {
		t.Fatalf("points changed: %v → %v", pointsBefore, pointsAfter)
	}
	student.must("GET", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id"), nil, 200, &view)
	if str(view, "attempt", "status") != "finished" {
		t.Fatalf("finished attempt after activation: %v", view)
	}
	// The attempt left in progress continues.
	student.must("PUT", "/api/attempts/"+aid2+"/answers/"+q2["id"].(string), obj{"option_ids": []string{str(q2, "options", 0, "id")}}, 200, nil)
	student.must("POST", "/api/attempts/"+aid2+"/finish", nil, 200, nil)
	student2.must("GET", "/api/marathons/"+id+"/lessons", nil, 403, nil)

	// Events keep who did what.
	var events int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_enrollment_events WHERE classroom_id = $1 AND actor_id = $2
		AND event IN ('deactivated', 'activated')`, id, e.ids["teacher"]).Scan(&events); err != nil || events != 3 {
		t.Fatalf("events: %d %v", events, err)
	}

	// ---- Removal from the marathon ----
	teacher2.must("DELETE", "/api/marathons/"+id+"/students/"+sid, nil, 404, nil)
	student.must("DELETE", "/api/marathons/"+id+"/students/"+sid, nil, 403, nil)
	teacher.must("DELETE", "/api/marathons/"+id+"/students/"+e.ids["teacher2"], nil, 404, nil)
	teacher.must("DELETE", "/api/marathons/"+id+"/students/"+sid2, nil, 200, &list)
	teacher.must("DELETE", "/api/marathons/"+id+"/students/"+sid2, nil, 200, &list) // repeat: harmless
	if len(get(list, "items").([]any)) != 1 {
		t.Fatalf("after removal: %v", list)
	}
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if num(m, "students_count") != 1 {
		t.Fatalf("students_count: %v", m)
	}
	student2.must("GET", "/api/classrooms/"+id, nil, 404, nil)
	student2.must("GET", "/api/marathons/"+id+"/lessons", nil, 404, nil)
	teacher.must("GET", "/api/marathons/"+id+"/students/"+sid2+"/progress", nil, 404, nil)
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid2}, "active": true}, 422, nil)

	// The student removed while active keeps their data and comes back active when added again.
	teacher.must("DELETE", "/api/marathons/"+id+"/students/"+sid, nil, 200, nil)
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 404, nil)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 404, nil)
	var kept int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM attempts WHERE classroom_id = $1 AND student_id = $2`, id, sid).Scan(&kept); err != nil || kept != 2 {
		t.Fatalf("attempts kept: %d %v", kept, err)
	}
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{sid, sid2}}, 200, &list)
	if len(get(list, "items").([]any)) != 2 || str(byID(list, sid2), "status") != "active" || num(byID(list, sid), "completed_lessons") != 1 {
		t.Fatalf("re-added: %v", list)
	}
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, &mine)
	if num(mine, "completed_lessons") != 1 {
		t.Fatalf("progress after re-adding: %v", mine)
	}
	student2.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, nil)

	// ---- With a passed end date: manual activation does not reopen the marathon ----
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_settings SET end_at = now() - interval '1 minute' WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	var closed obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 403, &closed)
	if str(closed, "error", "code") != "marathon_closed" {
		t.Fatalf("end date: %v", closed)
	}
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, &list)
	if str(byID(list, sid), "inactive_reason") != "manual" || str(byID(list, sid2), "inactive_reason") != "end_date" {
		t.Fatalf("reasons after the end date: %v", list)
	}
	teacher.must("PUT", "/api/marathons/"+id+"/students/active", obj{"student_ids": []string{sid}, "active": true}, 200, &list)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 403, &closed)
	if str(closed, "error", "code") != "marathon_closed" {
		t.Fatalf("activation must not override the end date: %v", closed)
	}
}

func mustJSONs(v any) string { return string(mustJSON(v)) }
