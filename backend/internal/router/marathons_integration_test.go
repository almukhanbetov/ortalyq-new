package router_test

import (
	"strings"
	"testing"
	"time"
)

// Stage 1 of marathons: create, list, update basic settings, access rules, archive.
func TestMarathonsStage1(t *testing.T) {
	e := setup(t)
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	admin := e.login("admin")

	// Validation: empty name, missing language for "languages", unknown direction/language, bad color.
	for _, body := range []obj{
		{"name": "  ", "direction": "languages", "target_language": "en"},
		{"name": "M", "direction": "languages"},
		{"name": "M", "direction": "languages", "target_language": "xx"},
		{"name": "M", "direction": "math"},
		{"name": "M", "direction": "other", "cover_color": "red;}"},
	} {
		teacher.must("POST", "/api/marathons", body, 422, nil)
	}
	// Only teachers create marathons.
	student.must("POST", "/api/marathons", obj{"name": "M", "direction": "other"}, 403, nil)
	admin.must("POST", "/api/marathons", obj{"name": "M", "direction": "other"}, 403, nil)

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": " Тестовый марафон ", "direction": "languages", "target_language": "EN"}, 201, &m)
	id := m["id"].(string)
	if m["name"] != "Тестовый марафон" || m["lesson_type"] != "marathon" || m["my_role"] != "teacher" {
		t.Fatalf("created: %v", m)
	}
	if get(m, "marathon", "mode") != "open" || get(m, "marathon", "direction") != "languages" || get(m, "marathon", "target_language") != "en" {
		t.Fatalf("settings: %v", m["marathon"])
	}
	if m["lessons_count"].(float64) != 0 || m["students_count"].(float64) != 0 {
		t.Fatalf("counts: %v", m)
	}

	// "Другое" drops the language even if one is sent.
	var other obj
	teacher.must("POST", "/api/marathons", obj{"name": "Другой", "direction": "other", "target_language": "en"}, 201, &other)
	if get(other, "marathon", "target_language") != nil {
		t.Fatalf("other keeps language: %v", other["marathon"])
	}

	// List filter type=marathon returns only marathons; individual classes are unaffected.
	var list obj
	teacher.must("GET", "/api/classrooms?type=marathon", nil, 200, &list)
	if list["total"].(float64) != 2 {
		t.Fatalf("marathon list: %v", list)
	}
	teacher.must("GET", "/api/classrooms?type=individual", nil, 200, &list)
	for _, it := range list["items"].([]any) {
		if get(it, "lesson_type") != "individual" || get(it, "marathon") != nil {
			t.Fatalf("individual list has %v", it)
		}
	}

	// Strangers: 404 on read and write, 403/404 for non-teachers.
	teacher2.must("GET", "/api/classrooms/"+id, nil, 404, nil)
	teacher2.must("PATCH", "/api/marathons/"+id, obj{"name": "X", "direction": "other"}, 404, nil)
	teacher2.must("POST", "/api/marathons/"+id+"/archive", nil, 404, nil)
	student.must("GET", "/api/classrooms/"+id, nil, 404, nil)
	student.must("PATCH", "/api/marathons/"+id, obj{"name": "X", "direction": "other"}, 403, nil)

	// Update basic settings.
	teacher.must("PATCH", "/api/marathons/"+id, obj{"name": "", "direction": "other"}, 422, nil)
	teacher.must("PATCH", "/api/marathons/"+id, obj{"name": "Новое имя", "cover_color": "#FF7A45", "direction": "languages", "target_language": "de"}, 200, &m)
	if m["name"] != "Новое имя" || m["cover_color"] != "#FF7A45" || get(m, "marathon", "target_language") != "de" {
		t.Fatalf("updated: %v", m)
	}

	// A marathon has no teacher-led current lesson; PATCH /marathons does not work on regular classes.
	var course, lesson obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+course["id"].(string)+"/lessons", obj{"title": "Урок"}, 201, &lesson)
	teacher.must("PUT", "/api/classrooms/"+id+"/current-lesson", obj{"lesson_id": lesson["id"]}, 409, nil)
	var cls obj
	teacher.must("POST", "/api/classrooms", obj{"name": "Индивидуальный", "student_id": e.assign(admin)}, 201, &cls)
	teacher.must("PATCH", "/api/marathons/"+cls["id"].(string), obj{"name": "X", "direction": "other"}, 404, nil)
	teacher.must("POST", "/api/marathons/"+cls["id"].(string)+"/archive", nil, 404, nil)

	// Archive: idempotent for the caller's view, then the marathon is gone everywhere.
	teacher.must("POST", "/api/marathons/"+id+"/archive", nil, 204, nil)
	teacher.must("GET", "/api/classrooms/"+id, nil, 404, nil)
	teacher.must("POST", "/api/marathons/"+id+"/archive", nil, 404, nil)
	teacher.must("GET", "/api/classrooms?type=marathon", nil, 200, &list)
	if list["total"].(float64) != 1 {
		t.Fatalf("after archive: %v", list)
	}
	var archived bool
	if err := e.pool.QueryRow(t.Context(), `SELECT archived_at IS NOT NULL FROM classrooms WHERE id = $1`, id).Scan(&archived); err != nil || !archived {
		t.Fatalf("row must stay archived: %v %v", archived, err)
	}
}

// assign links the test student to the teacher (admin action) and returns the student id.
func (e *env) assign(admin *client) string {
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	return e.ids["student"]
}

// Finished marathon: before end_at students work normally; after it (by now() of the DB) the server rejects
// starting, answering, finishing and completing — also for direct API calls — while participants, answers,
// attempts, points and progress stay intact and the owner keeps full read access.
func TestMarathonFinished(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2")
	ctx := t.Context()
	e.assign(admin)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон", "direction": "other"}, 201, &m)
	id := str(m, "id")
	if get(m, "marathon", "end_at") != nil || get(m, "marathon", "finished") != false {
		t.Fatalf("new marathon has an end date: %v", m["marathon"])
	}

	// End date in the future: not finished yet. Only the owner may change it.
	future := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	teacher2.must("PATCH", "/api/marathons/"+id+"/settings", obj{"end_at": future}, 404, nil)
	student.must("PATCH", "/api/marathons/"+id+"/settings", obj{"end_at": future}, 403, nil)
	teacher.must("PATCH", "/api/marathons/"+id+"/settings", obj{"end_at": future}, 200, &m)
	if got, _ := time.Parse(time.RFC3339, str(m, "marathon", "end_at")); !got.Equal(future) || get(m, "marathon", "finished") != false {
		t.Fatalf("end date: %v", m["marathon"])
	}

	// Lessons: two lessons with a test; adding is idempotent; someone else's lesson is 404.
	var course, l1, l2, sec, foreignCourse, foreign obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок 1"}, 201, &l1)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок 2"}, 201, &l2)
	lid1, lid2 := str(l1, "id"), str(l2, "id")
	teacher.must("POST", "/api/lessons/"+lid1+"/sections", obj{"title": "Раздел"}, 201, &sec)
	var ex1, ex2 obj
	teacher.must("POST", "/api/lessons/"+lid1+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Мгновенный",
		"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1},
		"questions": []obj{testQuestion("She ___ a teacher.", "is", "are")}}, 201, &ex1)
	teacher.must("POST", "/api/lessons/"+lid1+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "После",
		"settings":  obj{"check_mode": "after"},
		"questions": []obj{testQuestion("I ___ fine.", "am", "is"), testQuestion("We ___ here.", "are", "is")}}, 201, &ex2)
	teacher2.must("POST", "/api/courses", obj{"title": "Чужой"}, 201, &foreignCourse)
	teacher2.must("POST", "/api/courses/"+str(foreignCourse, "id")+"/lessons", obj{"title": "Чужой урок"}, 201, &foreign)
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": str(foreign, "id")}, 404, nil)
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": lid1}, 200, nil)
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": lid2}, 200, nil)
	var lessons obj
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": lid1}, 200, &lessons)
	if num(lessons, "total_lessons") != 2 || str(lessons, "items", 0, "lesson_id") != lid1 {
		t.Fatalf("lessons: %v", lessons)
	}

	// Students: only assigned ones; repeat adds no duplicate.
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{e.ids["admin"]}}, 422, nil)
	teacher2.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{e.ids["student"]}}, 404, nil)
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{e.ids["student"], e.ids["student2"]}}, 200, nil)
	var students obj
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{e.ids["student"]}}, 200, &students)
	if len(get(students, "items").([]any)) != 2 {
		t.Fatalf("students: %v", students)
	}
	student.must("GET", "/api/marathons/"+id+"/students", nil, 403, nil)
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if num(m, "students_count") != 2 || num(m, "lessons_count") != 2 {
		t.Fatalf("counts: %v", m)
	}

	// Before the end date the student works normally.
	ex1ID, ex2ID := str(ex1, "id"), str(ex2, "id")
	var v1, v2 obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+ex1ID+"/attempt", nil, 200, &v1)
	q1 := get(v1, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(v1, "attempt", "id")+"/answers/"+q1["id"].(string),
		obj{"option_ids": []string{str(q1, "options", 0, "id")}, "check": true}, 200, nil)
	student.must("POST", "/api/attempts/"+str(v1, "attempt", "id")+"/finish", nil, 200, nil)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+ex2ID+"/attempt", nil, 200, &v2)
	aid2 := str(v2, "attempt", "id")
	q2 := get(v2, "version", "questions", 0).(map[string]any)
	q2b := get(v2, "version", "questions", 1).(map[string]any)
	student.must("PUT", "/api/attempts/"+aid2+"/answers/"+q2["id"].(string), obj{"option_ids": []string{str(q2, "options", 0, "id")}}, 200, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+lid1+"/complete", nil, 200, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+lid1+"/complete", nil, 200, &lessons)
	if num(lessons, "completed_lessons") != 1 || num(lessons, "total_lessons") != 2 || get(lessons, "items", 0, "completed") != true {
		t.Fatalf("student progress: %v", lessons)
	}
	teacher.must("POST", "/api/marathons/"+id+"/lessons/"+lid1+"/complete", nil, 403, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+str(foreign, "id")+"/complete", nil, 404, nil)
	teacher.must("GET", "/api/marathons/"+id+"/students", nil, 200, &students)
	if str(students, "items", 0, "status") != "active" || num(students, "items", 0, "completed_lessons") != 1 {
		t.Fatalf("active student: %v", students)
	}

	snapshot := func() [4]int {
		var c [4]int
		if err := e.pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM attempts WHERE classroom_id = $1),
			(SELECT count(*) FROM attempt_answers a JOIN attempts t ON t.id = a.attempt_id WHERE t.classroom_id = $1),
			(SELECT count(*) FROM marathon_lesson_progress WHERE classroom_id = $1),
			(SELECT count(*) FROM marathon_enrollments WHERE classroom_id = $1)`, id).Scan(&c[0], &c[1], &c[2], &c[3]); err != nil {
			t.Fatal(err)
		}
		return c
	}
	before := snapshot()

	// Time passes: the end date is now in the past (moved in the test DB, as if the clock reached it).
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_settings SET end_at = now() - interval '1 second' WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	student.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if get(m, "marathon", "finished") != true || get(m, "marathon", "end_at") == nil {
		t.Fatalf("not finished: %v", m["marathon"])
	}

	closed := func(c *client, method, path string, body any) {
		t.Helper()
		raw := c.must(method, path, body, 403, nil)
		if !strings.Contains(string(raw), `"marathon_closed"`) {
			t.Fatalf("%s %s: %s", method, path, raw)
		}
	}
	closed(student, "POST", "/api/classrooms/"+id+"/exercises/"+ex2ID+"/attempt", nil)
	closed(student, "POST", "/api/classrooms/"+id+"/exercises/"+ex1ID+"/attempt", nil)
	closed(student, "PUT", "/api/attempts/"+aid2+"/answers/"+q2b["id"].(string), obj{"option_ids": []string{str(q2b, "options", 0, "id")}})
	closed(student, "POST", "/api/attempts/"+aid2+"/finish", nil)
	closed(student, "GET", "/api/classrooms/"+id+"/lessons/"+lid1+"/exercises?scope=lesson", nil)
	closed(student, "GET", "/api/classrooms/"+id+"/exercises/"+ex2ID, nil)
	closed(student, "GET", "/api/classrooms/"+id+"/lessons/"+lid1+"/progress", nil)
	closed(student, "POST", "/api/marathons/"+id+"/lessons/"+lid2+"/complete", nil)
	closed(student2, "POST", "/api/classrooms/"+id+"/exercises/"+ex1ID+"/attempt", nil)

	// The student still sees the marathon and their own progress (read-only).
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, &lessons)
	if num(lessons, "completed_lessons") != 1 || num(lessons, "total_lessons") != 2 {
		t.Fatalf("student progress after end: %v", lessons)
	}

	// Nothing was deleted or reset.
	if after := snapshot(); after != before {
		t.Fatalf("data changed by finishing: before %v after %v", before, after)
	}

	// The owner keeps the marathon, settings, participants and their progress.
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if num(m, "students_count") != 2 || num(m, "lessons_count") != 2 || get(m, "marathon", "finished") != true {
		t.Fatalf("teacher view: %v", m)
	}
	teacher.must("GET", "/api/marathons/"+id+"/students", nil, 200, &students)
	for _, it := range get(students, "items").([]any) {
		if get(it, "status") != "inactive" || get(it, "inactive_reason") != "end_date" || num(it, "total_lessons") != 2 {
			t.Fatalf("inactive student: %v", it)
		}
	}
	if str(students, "items", 0, "id") != e.ids["student"] || num(students, "items", 0, "completed_lessons") != 1 {
		t.Fatalf("progress kept: %v", students)
	}
	teacher.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, &lessons)
	if num(lessons, "items", 0, "completed_count") != 1 {
		t.Fatalf("lesson counter: %v", lessons)
	}
	var views obj
	teacher.must("GET", "/api/classrooms/"+id+"/lessons/"+lid1+"/exercises?scope=lesson&student_id="+e.ids["student"], nil, 200, &views)
	if str(views, "items", 0, "attempt", "status") != "finished" || num(views, "items", 0, "attempt", "summary", "points_earned") != 1 ||
		len(get(views, "items", 1, "attempt", "answers").([]any)) == 0 {
		t.Fatalf("answers and points kept: %v", views)
	}
	var progress obj
	teacher.must("GET", "/api/classrooms/"+id+"/lessons/"+lid1+"/progress?student_id="+e.ids["student"], nil, 200, &progress)

	// Removing the end date reopens access with everything in place.
	teacher.must("PATCH", "/api/marathons/"+id+"/settings", obj{"end_at": nil}, 200, &m)
	if get(m, "marathon", "finished") != false {
		t.Fatalf("reopened: %v", m["marathon"])
	}
	student.must("PUT", "/api/attempts/"+aid2+"/answers/"+q2b["id"].(string), obj{"option_ids": []string{str(q2b, "options", 0, "id")}}, 200, nil)
}

// "Прогресс ученика": the owner sees one student's status, lessons and saved answers — also after the end date —
// and looking never writes anything (no attempts started or finished, no answers, points or progress changed).
func TestMarathonStudentProgress(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	ctx := t.Context()
	e.assign(admin)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон", "direction": "other"}, 201, &m)
	id := str(m, "id")
	end := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	teacher.must("PATCH", "/api/marathons/"+id+"/settings", obj{"end_at": end}, 200, nil)
	var course, l1, l2, sec, ex1, ex2 obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок 1"}, 201, &l1)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок 2"}, 201, &l2)
	lid1, lid2 := str(l1, "id"), str(l2, "id")
	teacher.must("POST", "/api/lessons/"+lid1+"/sections", obj{"title": "Раздел"}, 201, &sec)
	teacher.must("POST", "/api/lessons/"+lid1+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Мгновенный",
		"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1},
		"questions": []obj{testQuestion("She ___ a teacher.", "is", "are")}}, 201, &ex1)
	teacher.must("POST", "/api/lessons/"+lid1+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "С таймером",
		"settings":  obj{"check_mode": "after", "timer_enabled": true, "timer_seconds": 60},
		"questions": []obj{testQuestion("I ___ fine.", "am", "is"), testQuestion("We ___ here.", "are", "is")}}, 201, &ex2)
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": lid1}, 200, nil)
	teacher.must("POST", "/api/marathons/"+id+"/lessons", obj{"lesson_id": lid2}, 200, nil)
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{e.ids["student"], e.ids["student2"]}}, 200, nil)
	sid := e.ids["student"]

	// The student answers: instant test finished, timed test left in progress; lesson 1 completed.
	var v1, v2 obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex1, "id")+"/attempt", nil, 200, &v1)
	q1 := get(v1, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(v1, "attempt", "id")+"/answers/"+q1["id"].(string), obj{"option_ids": []string{str(q1, "options", 0, "id")}, "check": true}, 200, nil)
	student.must("POST", "/api/attempts/"+str(v1, "attempt", "id")+"/finish", nil, 200, nil)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex2, "id")+"/attempt", nil, 200, &v2)
	q2 := get(v2, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(v2, "attempt", "id")+"/answers/"+q2["id"].(string), obj{"option_ids": []string{str(q2, "options", 0, "id")}}, 200, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+lid1+"/complete", nil, 200, nil)

	progressURL := "/api/marathons/" + id + "/students/" + sid + "/progress"
	reviewURL := "/api/marathons/" + id + "/students/" + sid + "/lessons/" + lid1

	// Access: owner only; strangers and students get 404/403; unknown student or lesson is 404.
	teacher2.must("GET", progressURL, nil, 404, nil)
	teacher2.must("GET", reviewURL, nil, 404, nil)
	student.must("GET", progressURL, nil, 403, nil)
	student.must("GET", reviewURL, nil, 403, nil)
	teacher.must("GET", "/api/marathons/"+id+"/students/"+e.ids["teacher2"]+"/progress", nil, 404, nil)
	teacher.must("GET", "/api/marathons/"+id+"/students/"+sid+"/lessons/"+str(course, "id"), nil, 404, nil)

	var pr obj
	teacher.must("GET", progressURL, nil, 200, &pr)
	if str(pr, "student", "id") != sid || str(pr, "student", "email") != "student@test.local" || str(pr, "student", "status") != "active" ||
		str(pr, "student", "deactivation_reason") != "end_date" || num(pr, "completed_lessons") != 1 || num(pr, "total_lessons") != 2 ||
		get(pr, "items", 0, "completed") != true || get(pr, "items", 1, "completed") != false {
		t.Fatalf("progress: %v", pr)
	}
	if got, _ := time.Parse(time.RFC3339, str(pr, "student", "deactivates_at")); !got.Equal(end) {
		t.Fatalf("deactivates_at: %v", pr["student"])
	}

	// The timed attempt's deadline passes and the marathon ends (clock moved in the test DB).
	if _, err := e.pool.Exec(ctx, `UPDATE attempts SET deadline_at = now() - interval '1 second' WHERE id = $1`, str(v2, "attempt", "id")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE marathon_settings SET end_at = now() - interval '1 second' WHERE classroom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	digest := func() string {
		var d string
		if err := e.pool.QueryRow(ctx, `SELECT
			(SELECT md5(COALESCE(string_agg(a::text, '|' ORDER BY a.id), '')) FROM attempts a WHERE a.classroom_id = $1) ||
			(SELECT md5(COALESCE(string_agg(x::text, '|' ORDER BY x::text), '')) FROM attempt_answers x JOIN attempts a ON a.id = x.attempt_id WHERE a.classroom_id = $1) ||
			(SELECT md5(COALESCE(string_agg(p::text, '|' ORDER BY p::text), '')) FROM marathon_lesson_progress p WHERE p.classroom_id = $1) ||
			(SELECT md5(COALESCE(string_agg(n::text, '|' ORDER BY n::text), '')) FROM marathon_enrollments n WHERE n.classroom_id = $1)`, id).Scan(&d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	before := digest()

	// After the end the owner still sees status, progress and saved answers with results.
	teacher.must("GET", progressURL, nil, 200, &pr)
	if str(pr, "student", "status") != "inactive" || str(pr, "student", "inactive_reason") != "end_date" || num(pr, "completed_lessons") != 1 {
		t.Fatalf("progress after end: %v", pr)
	}
	var rv obj
	for i := 0; i < 2; i++ {
		teacher.must("GET", reviewURL, nil, 200, &rv)
	}
	if str(rv, "lesson_title") != "Урок 1" || get(rv, "completed") != true || len(get(rv, "sections").([]any)) != 1 || len(get(rv, "exercises").([]any)) != 2 {
		t.Fatalf("review: %v", rv)
	}
	if str(rv, "exercises", 0, "attempt", "status") != "finished" || num(rv, "exercises", 0, "attempt", "summary", "points_earned") != 1 ||
		get(rv, "exercises", 0, "version", "questions", 0, "options", 0, "is_correct") != true {
		t.Fatalf("finished exercise with key and points: %v", rv["exercises"])
	}
	// The expired timed attempt is shown as saved: not finalized by the teacher's look.
	if str(rv, "exercises", 1, "attempt", "status") != "in_progress" || len(get(rv, "exercises", 1, "attempt", "answers").([]any)) != 2 {
		t.Fatalf("timed attempt: %v", get(rv, "exercises", 1))
	}
	if num(rv, "points", "earned") != 1 {
		t.Fatalf("points: %v", rv["points"])
	}
	// A lesson the student never opened: exercises without attempts, nothing created.
	teacher.must("GET", "/api/marathons/"+id+"/students/"+sid+"/lessons/"+lid2, nil, 200, &rv)
	if get(rv, "completed") != false || len(get(rv, "exercises").([]any)) != 0 {
		t.Fatalf("lesson 2: %v", rv)
	}
	teacher.must("GET", "/api/marathons/"+id+"/students/"+e.ids["student2"]+"/lessons/"+lid1, nil, 200, &rv)
	for _, ex := range get(rv, "exercises").([]any) {
		if get(ex, "attempt") != nil {
			t.Fatalf("student2 has no attempts: %v", ex)
		}
	}
	if after := digest(); after != before {
		t.Fatal("viewing the student's progress changed stored data")
	}

	// The student stays locked out of the finished marathon.
	raw := student.must("GET", "/api/classrooms/"+id+"/lessons/"+lid1+"/exercises?scope=lesson", nil, 403, nil)
	if !strings.Contains(string(raw), "marathon_closed") {
		t.Fatalf("student after end: %s", raw)
	}
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex2, "id")+"/attempt", nil, 403, nil)
}
