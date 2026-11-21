package router_test

import (
	"testing"
	"time"
)

// Stage 5 schedules (Н1 = A): "По интервалам" and "По дням недели" count from the anchor
// max(start_at, activated_at); lesson 0 opens at the anchor, the next ones by the schedule in the marathon time
// zone. Each student has their own schedule (late adding), refusals hold for direct API calls, a schedule change
// within the mode keeps statuses, and a mode change keeps answers and points.
func TestMarathonSchedules(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	student := e.login("student")
	student2 := e.login("student2")
	ctx := t.Context()
	sid := e.assign(admin)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	sid2 := e.ids["student2"]

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон по расписанию", "direction": "other"}, 201, &m)
	id := str(m, "id")
	tz, err := time.LoadLocation(str(m, "marathon", "timezone"))
	if err != nil {
		t.Fatal(err)
	}
	var course obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	lids, exs := []string{}, []string{}
	for _, title := range []string{"Урок 1", "Урок 2", "Урок 3"} {
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
	// The first student was added 5 days ago; the second one just now (a late student).
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_enrollments SET activated_at = now() - interval '5 days' WHERE classroom_id = $1 AND student_id = $2`, id, sid); err != nil {
		t.Fatal(err)
	}
	// The student answers lesson 1 in the open mode.
	var v obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exs[0]+"/attempt", nil, 200, &v)
	q := get(v, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(v, "attempt", "id")+"/answers/"+q["id"].(string), obj{"option_ids": []string{str(q, "options", 0, "id")}, "check": true}, 200, nil)
	student.must("POST", "/api/attempts/"+str(v, "attempt", "id")+"/finish", nil, 200, nil)
	var pointsBefore obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/progress", nil, 200, &pointsBefore)

	// ---- «По интервалам»: раз в 1 день в 09:00 ----
	teacher.must("POST", base+"/mode", obj{"mode": "interval", "expected_epoch": 1, "confirm_reset": true, "interval_days": 1, "open_time": "09:00"}, 200, &m)
	if get(m, "marathon", "mode") != "interval" || num(m, "marathon", "interval_days") != 1 || str(m, "marathon", "open_time") != "09:00" {
		t.Fatalf("interval settings: %v", m["marathon"])
	}
	teacher.must("PUT", base+"/lessons/"+lids[1]+"/open-at", obj{"open_at": time.Now().Add(time.Hour).UTC()}, 422, nil) // no dates in schedules

	// Early student: anchor 5 days ago → lessons 1 and 2 opened on the following days at 09:00.
	var me obj
	student.must("GET", base+"/me", nil, 200, &me)
	for i := 0; i < 3; i++ {
		if str(me, "items", i, "access") != "open" {
			t.Fatalf("early student, lesson %d: %v", i, me["items"])
		}
	}
	// Late student: lesson 0 at once, lesson 1 tomorrow (or today) at 09:00 in the marathon zone.
	var me2 obj
	student2.must("GET", base+"/me", nil, 200, &me2)
	if str(me2, "items", 0, "access") != "open" || str(me2, "items", 1, "lock", "reason") != "scheduled" || str(me2, "items", 2, "lock", "reason") != "scheduled" {
		t.Fatalf("late student: %v", me2["items"])
	}
	opens1, _ := time.Parse(time.RFC3339, str(me2, "items", 1, "lock", "opens_at"))
	opens2, _ := time.Parse(time.RFC3339, str(me2, "items", 2, "lock", "opens_at"))
	if l := opens1.In(tz); l.Hour() != 9 || l.Minute() != 0 || !opens1.After(time.Now()) || opens1.Sub(time.Now()) > 48*time.Hour {
		t.Fatalf("lesson 1 of the late student opens at %v", l)
	}
	if opens2.Sub(opens1) != 24*time.Hour {
		t.Fatalf("lesson 2 one day after lesson 1: %v → %v", opens1, opens2)
	}
	var out obj
	student2.must("POST", "/api/classrooms/"+id+"/exercises/"+exs[1]+"/attempt", nil, 403, &out)
	if str(out, "error", "code") != "marathon_lesson_locked" || str(out, "error", "fields", "reason") != "scheduled" || str(out, "error", "fields", "opens_at") == "" {
		t.Fatalf("direct API for the late student: %v", out)
	}
	student2.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[2]+"/exercises", nil, 403, nil)
	student2.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 200, nil)
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[2]+"/exercises", nil, 200, nil)
	// The owner sees each student's own lock in "Прогресс ученика".
	var pr obj
	teacher.must("GET", base+"/students/"+sid2+"/progress", nil, 200, &pr)
	if str(pr, "items", 1, "student_lesson", "reason") != "scheduled" {
		t.Fatalf("progress window: %v", pr["items"])
	}

	// A later start_at becomes the anchor of the early student too: lesson 1 closes until its new moment.
	teacher.must("PATCH", base+"/settings", obj{"start_at": time.Now().Add(-time.Hour).UTC()}, 200, nil)
	student.must("GET", base+"/me", nil, 200, &me)
	if str(me, "items", 0, "access") != "open" || str(me, "items", 1, "lock", "reason") != "scheduled" {
		t.Fatalf("anchor = start_at: %v", me["items"])
	}
	teacher.must("PATCH", base+"/settings", obj{"start_at": nil}, 200, nil)

	// ---- Changing the schedule within the mode does not start a new epoch ----
	teacher.must("POST", base+"/mode", obj{"mode": "interval", "expected_epoch": 2, "interval_days": 3, "open_time": "18:00"}, 200, &m)
	if num(m, "marathon", "progress_epoch") != 2 || num(m, "marathon", "interval_days") != 3 || str(m, "marathon", "open_time") != "18:00" {
		t.Fatalf("schedule update: %v", m["marathon"])
	}
	student2.must("GET", base+"/me", nil, 200, &me2)
	o1, _ := time.Parse(time.RFC3339, str(me2, "items", 1, "lock", "opens_at"))
	o2, _ := time.Parse(time.RFC3339, str(me2, "items", 2, "lock", "opens_at"))
	if o1.In(tz).Hour() != 18 || o2.Sub(o1) != 72*time.Hour {
		t.Fatalf("new schedule: %v → %v", o1.In(tz), o2.In(tz))
	}

	// ---- «По дням недели»: every day at 07:00 → a new epoch, the same answers and points ----
	teacher.must("POST", base+"/mode", obj{"mode": "weekdays", "expected_epoch": 2, "confirm_reset": true, "weekdays": []int{1, 2, 3, 4, 5, 6, 7}, "open_time": "07:00"}, 200, &m)
	if get(m, "marathon", "mode") != "weekdays" || num(m, "marathon", "progress_epoch") != 3 || len(get(m, "marathon", "weekdays").([]any)) != 7 || get(m, "marathon", "interval_days") != nil {
		t.Fatalf("weekdays settings: %v", m["marathon"])
	}
	student2.must("GET", base+"/me", nil, 200, &me2)
	w1, _ := time.Parse(time.RFC3339, str(me2, "items", 1, "lock", "opens_at"))
	if l := w1.In(tz); l.Hour() != 7 || !w1.After(time.Now()) || w1.Sub(time.Now()) > 25*time.Hour {
		t.Fatalf("weekdays, lesson 1 of the late student: %v", l)
	}
	// Only Mondays: lesson 1 falls on a Monday at 07:00 in the marathon zone.
	teacher.must("POST", base+"/mode", obj{"mode": "weekdays", "expected_epoch": 3, "weekdays": []int{1}, "open_time": "07:00"}, 200, nil)
	student2.must("GET", base+"/me", nil, 200, &me2)
	w1, _ = time.Parse(time.RFC3339, str(me2, "items", 1, "lock", "opens_at"))
	w2, _ := time.Parse(time.RFC3339, str(me2, "items", 2, "lock", "opens_at"))
	if l := w1.In(tz); l.Weekday() != time.Monday || l.Hour() != 7 || w2.In(tz).Weekday() != time.Monday || w2.Sub(w1) != 7*24*time.Hour {
		t.Fatalf("Mondays: %v, %v", w1.In(tz), w2.In(tz))
	}
	var pointsAfter obj
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/progress", nil, 200, &pointsAfter)
	if mustJSONs(pointsAfter) != mustJSONs(pointsBefore) {
		t.Fatalf("answers and points must stay: %v → %v", pointsBefore, pointsAfter)
	}

	// A deactivated student gets the student-level refusal whatever the schedule says.
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid}, "active": false}, 200, nil)
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+lids[0]+"/exercises", nil, 403, &out)
	if str(out, "error", "code") != "marathon_deactivated" {
		t.Fatalf("deactivated: %v", out)
	}
}
