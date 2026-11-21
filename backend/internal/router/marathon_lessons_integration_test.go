package router_test

import (
	"strings"
	"testing"
)

// Stage 2 of marathons: creating, editing, ordering, removing and copying lessons inside a marathon.
// Covers the acceptance of the plan: counters update, order survives a re-read, a lesson used in two
// marathons keeps results apart, strangers get 404, removal keeps the template and the students' progress.
func TestMarathonLessonsManage(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	ctx := t.Context()
	e.assign(admin)

	var m, m2 obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон А", "direction": "other", "cover_color": "#FF7A45"}, 201, &m)
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон Б", "direction": "other"}, 201, &m2)
	id, id2 := str(m, "id"), str(m2, "id")
	base := "/api/marathons/" + id + "/lessons"

	// ---- Create a new lesson inside the marathon ----
	for _, body := range []obj{
		{"new": obj{"title": "  "}},
		{"new": obj{}},
		{"new": obj{"title": strings.Repeat("я", 201)}},
		{"new": obj{"title": "Урок", "cover_color": "red"}},
		{"new": obj{"title": "Урок", "description": strings.Repeat("я", 2001)}},
		{},
		{"lesson_id": e.ids["teacher"], "new": obj{"title": "Урок"}},
	} {
		teacher.must("POST", base, body, 422, nil)
	}
	teacher2.must("POST", base, obj{"new": obj{"title": "Чужой"}}, 404, nil)
	student.must("POST", base, obj{"new": obj{"title": "Ученик"}}, 403, nil)

	var lessons obj
	teacher.must("POST", base, obj{"new": obj{"title": " Новый урок ", "description": "Описание", "cover_color": "#16C784"}}, 200, &lessons)
	teacher.must("POST", base, obj{"new": obj{"title": "Второй новый"}}, 200, &lessons)
	if num(lessons, "total_lessons") != 2 || str(lessons, "items", 0, "title") != "Новый урок" ||
		str(lessons, "items", 0, "description") != "Описание" || str(lessons, "items", 0, "cover_color") != "#16C784" ||
		str(lessons, "items", 1, "cover_color") != "#1AC6FF" || str(lessons, "items", 0, "course_title") != "Марафон А" {
		t.Fatalf("created lessons: %v", lessons)
	}
	newA, newB := str(lessons, "items", 0, "lesson_id"), str(lessons, "items", 1, "lesson_id")
	// Both live in one own course of the marathon, visible in the teacher's materials.
	var courses obj
	teacher.must("GET", "/api/courses", nil, 200, &courses)
	own := 0
	for _, c := range get(courses, "items").([]any) {
		if str(c, "title") == "Марафон А" {
			own++
			if num(c, "lessons_count") != 2 && get(c, "lessons_count") != nil {
				t.Fatalf("own course lessons: %v", c)
			}
		}
	}
	if own != 1 {
		t.Fatalf("own course must be created once: %v", courses)
	}
	teacher.must("GET", "/api/lessons/"+newA, nil, 200, nil)
	teacher2.must("GET", "/api/lessons/"+newA, nil, 404, nil)

	// ---- A lesson from personal courses, with a test ----
	var course, personal, sec, ex obj
	teacher.must("POST", "/api/courses", obj{"title": "Личный курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Глагол to be"}, 201, &personal)
	pid := str(personal, "id")
	teacher.must("POST", "/api/lessons/"+pid+"/sections", obj{"title": "Section 1"}, 201, &sec)
	teacher.must("POST", "/api/lessons/"+pid+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест",
		"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 5, "timer_enabled": true, "timer_seconds": 120},
		"questions": []obj{testQuestion("She ___ a teacher.", "is", "are")}}, 201, &ex)
	teacher.must("POST", base, obj{"lesson_id": pid}, 200, &lessons)
	if num(lessons, "total_lessons") != 3 || str(lessons, "items", 2, "lesson_id") != pid || str(lessons, "items", 2, "course_title") != "Личный курс" {
		t.Fatalf("personal lesson: %v", lessons)
	}
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if num(m, "lessons_count") != 3 {
		t.Fatalf("lessons_count: %v", m)
	}

	// ---- Order ----
	order := []string{pid, newB, newA}
	for _, bad := range [][]string{{pid, newB}, {pid, newB, newB}, {pid, newB, e.ids["teacher"]}, {pid, newB, newA, newA}} {
		teacher.must("PUT", base+"/order", obj{"lesson_ids": bad}, 422, nil)
	}
	teacher2.must("PUT", base+"/order", obj{"lesson_ids": order}, 404, nil)
	student.must("PUT", base+"/order", obj{"lesson_ids": order}, 403, nil)
	teacher.must("PUT", base+"/order", obj{"lesson_ids": order}, 200, nil)
	teacher.must("PUT", base+"/order", obj{"lesson_ids": order}, 200, nil) // repeat: same result
	teacher.must("GET", base, nil, 200, &lessons)
	for i, want := range order {
		if str(lessons, "items", i, "lesson_id") != want {
			t.Fatalf("order after re-read: %v", lessons)
		}
	}

	// ---- Lesson settings ----
	teacher.must("PATCH", base+"/"+newA, obj{"title": " "}, 422, nil)
	teacher.must("PATCH", base+"/"+newA, obj{"cover_color": "#zzz"}, 422, nil)
	teacher2.must("PATCH", base+"/"+newA, obj{"title": "X"}, 404, nil)
	teacher.must("PATCH", base+"/"+str(course, "id"), obj{"title": "X"}, 404, nil) // not a lesson of the marathon
	teacher.must("PATCH", base+"/"+pid, obj{"title": "Глагол to be (A1)", "description": "Тест на время", "cover_color": "#9254DE"}, 200, &lessons)
	if str(lessons, "items", 0, "title") != "Глагол to be (A1)" || str(lessons, "items", 0, "description") != "Тест на время" ||
		str(lessons, "items", 0, "cover_color") != "#9254DE" {
		t.Fatalf("settings: %v", lessons)
	}
	var tpl obj
	teacher.must("GET", "/api/lessons/"+pid, nil, 200, &tpl)
	if str(tpl, "title") != "Глагол to be (A1)" { // title and description belong to the template
		t.Fatalf("template title: %v", tpl)
	}
	teacher.must("PATCH", base+"/"+pid, obj{"cover_color": ""}, 200, &lessons) // back to the course colour
	if str(lessons, "items", 0, "cover_color") != "#1AC6FF" {
		t.Fatalf("cover reset: %v", lessons)
	}

	// ---- Students and results: the same lesson in two marathons keeps results apart ----
	teacher.must("POST", "/api/marathons/"+id+"/students", obj{"student_ids": []string{e.ids["student"]}}, 200, nil)
	teacher.must("POST", "/api/marathons/"+id2+"/lessons", obj{"lesson_id": pid}, 200, nil)
	teacher.must("POST", "/api/marathons/"+id2+"/students", obj{"student_ids": []string{e.ids["student"]}}, 200, nil)
	exID := str(ex, "id")
	var v obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exID+"/attempt", nil, 200, &v)
	q := get(v, "version", "questions", 0).(map[string]any)
	student.must("PUT", "/api/attempts/"+str(v, "attempt", "id")+"/answers/"+q["id"].(string),
		obj{"option_ids": []string{str(q, "options", 0, "id")}, "check": true}, 200, nil)
	student.must("POST", "/api/attempts/"+str(v, "attempt", "id")+"/finish", nil, 200, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+pid+"/complete", nil, 200, nil)
	var otherView obj
	student.must("GET", "/api/classrooms/"+id2+"/exercises/"+exID, nil, 200, &otherView)
	if get(otherView, "attempt") != nil {
		t.Fatalf("attempt leaked into the second marathon: %v", otherView)
	}
	var mine obj
	student.must("GET", "/api/marathons/"+id2+"/lessons", nil, 200, &mine)
	if num(mine, "completed_lessons") != 0 {
		t.Fatalf("progress leaked into the second marathon: %v", mine)
	}

	// ---- Copy: a new template in the marathon's own course, without results ----
	teacher2.must("POST", base+"/"+pid+"/copy", nil, 404, nil)
	student.must("POST", base+"/"+pid+"/copy", nil, 403, nil)
	teacher.must("POST", base+"/"+str(course, "id")+"/copy", nil, 404, nil)
	var copied obj
	teacher.must("POST", base+"/"+pid+"/copy", nil, 201, &copied)
	cid := str(copied, "lesson_id")
	if cid == "" || cid == pid || num(copied, "total_lessons") != 4 || str(copied, "items", 3, "lesson_id") != cid ||
		str(copied, "items", 3, "title") != "Глагол to be (A1) (копия)" || str(copied, "items", 3, "course_title") != "Марафон А" ||
		num(copied, "items", 3, "completed_count") != 0 || num(copied, "items", 3, "section_count") != 1 {
		t.Fatalf("copy: %v", copied)
	}
	var exs, srcExs obj
	teacher.must("GET", "/api/lessons/"+cid+"/exercises?scope=lesson", nil, 200, &exs)
	teacher.must("GET", "/api/lessons/"+pid+"/exercises?scope=lesson", nil, 200, &srcExs)
	if len(get(exs, "items").([]any)) != 1 || str(exs, "items", 0, "id") == exID || str(exs, "items", 0, "copied_from_id") != exID ||
		str(exs, "items", 0, "version", "title") != "Тест" || num(exs, "items", 0, "version", "settings", "points_per_correct") != 5 ||
		num(exs, "items", 0, "version", "settings", "timer_seconds") != 120 ||
		str(exs, "items", 0, "version", "questions", 0, "prompt") != "She ___ a teacher." ||
		get(exs, "items", 0, "version", "questions", 0, "options", 0, "is_correct") != true ||
		get(exs, "items", 0, "version", "questions", 0, "options", 1, "is_correct") != false ||
		str(exs, "items", 0, "section_id") == str(srcExs, "items", 0, "section_id") {
		t.Fatalf("copied exercise: %v", exs)
	}
	var copyView obj
	student.must("GET", "/api/classrooms/"+id+"/exercises/"+str(exs, "items", 0, "id"), nil, 200, &copyView)
	if get(copyView, "attempt") != nil {
		t.Fatalf("results were copied: %v", copyView)
	}
	// Editing the copy does not touch the original.
	teacher.must("PATCH", base+"/"+cid, obj{"title": "Копия изменена"}, 200, nil)
	teacher.must("GET", "/api/lessons/"+pid, nil, 200, &tpl)
	if str(tpl, "title") != "Глагол to be (A1)" {
		t.Fatalf("original changed by the copy: %v", tpl)
	}

	// ---- Remove from the marathon ----
	teacher2.must("DELETE", base+"/"+pid, nil, 404, nil)
	student.must("DELETE", base+"/"+pid, nil, 403, nil)
	teacher.must("DELETE", base+"/"+str(course, "id"), nil, 404, nil)
	teacher.must("DELETE", base+"/"+pid, nil, 200, &lessons)
	teacher.must("DELETE", base+"/"+pid, nil, 200, &lessons) // repeat: harmless
	if num(lessons, "total_lessons") != 3 {
		t.Fatalf("after remove: %v", lessons)
	}
	for _, it := range get(lessons, "items").([]any) {
		if str(it, "lesson_id") == pid {
			t.Fatalf("removed lesson still listed: %v", lessons)
		}
	}
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if num(m, "lessons_count") != 3 {
		t.Fatalf("lessons_count after remove: %v", m)
	}
	teacher.must("GET", "/api/lessons/"+pid, nil, 200, nil) // the template stays in materials
	// The student loses access to the removed lesson, also through direct API calls.
	student.must("GET", "/api/classrooms/"+id+"/lessons/"+pid+"/exercises", nil, 404, nil)
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+exID+"/attempt", nil, 404, nil)
	student.must("POST", "/api/marathons/"+id+"/lessons/"+pid+"/complete", nil, 404, nil)
	teacher.must("PATCH", base+"/"+pid, obj{"title": "X"}, 404, nil)
	teacher.must("POST", base+"/"+pid+"/copy", nil, 404, nil)
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, &mine)
	if num(mine, "total_lessons") != 3 || num(mine, "completed_lessons") != 0 {
		t.Fatalf("student list after remove: %v", mine)
	}
	// The other marathon is not affected.
	student.must("GET", "/api/classrooms/"+id2+"/lessons/"+pid+"/exercises", nil, 200, nil)
	// Attempts and progress rows are kept.
	var attempts, progress int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM attempts WHERE classroom_id = $1`, id).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("attempts kept: %d %v", attempts, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM marathon_lesson_progress WHERE classroom_id = $1`, id).Scan(&progress); err != nil || progress != 1 {
		t.Fatalf("progress kept: %d %v", progress, err)
	}

	// Adding it again restores it at the end with the student's progress.
	teacher.must("POST", base, obj{"lesson_id": pid}, 200, &lessons)
	if num(lessons, "total_lessons") != 4 || str(lessons, "items", 3, "lesson_id") != pid || num(lessons, "items", 3, "completed_count") != 1 {
		t.Fatalf("re-added: %v", lessons)
	}
	student.must("GET", "/api/marathons/"+id+"/lessons", nil, 200, &mine)
	if num(mine, "completed_lessons") != 1 {
		t.Fatalf("restored progress: %v", mine)
	}
	student.must("GET", "/api/classrooms/"+id+"/exercises/"+exID, nil, 200, &v)
	if str(v, "attempt", "status") != "finished" {
		t.Fatalf("restored attempt: %v", v)
	}

	// An attempt started before the removal can no longer be answered or finished (direct API calls).
	var live obj
	student.must("POST", "/api/classrooms/"+id2+"/exercises/"+exID+"/attempt", nil, 200, &live)
	lq := get(live, "version", "questions", 0).(map[string]any)
	teacher.must("DELETE", "/api/marathons/"+id2+"/lessons/"+pid, nil, 200, nil)
	student.must("PUT", "/api/attempts/"+str(live, "attempt", "id")+"/answers/"+lq["id"].(string),
		obj{"option_ids": []string{str(lq, "options", 0, "id")}}, 404, nil)
	student.must("POST", "/api/attempts/"+str(live, "attempt", "id")+"/finish", nil, 404, nil)
	var status string
	if err := e.pool.QueryRow(ctx, `SELECT status FROM attempts WHERE id = $1`, str(live, "attempt", "id")).Scan(&status); err != nil || status != "in_progress" {
		t.Fatalf("attempt must stay untouched: %s %v", status, err)
	}
}
