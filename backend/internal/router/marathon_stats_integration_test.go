package router_test

import (
	"strings"
	"testing"
	"time"
)

// Stage 7 of marathons: statistics, rating, lesson results, the statistics comment and the student's results.
// Every number is checked against the per-lesson points of attempts.Points (the teacher's lesson view) after
// sending, revision, acceptance, cancellation and second completion, a mode change, removing and adding a student
// back, deactivation, removing a lesson and the end date; rights through direct API calls.
func TestMarathonStatistics(t *testing.T) {
	e := setup(t)
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2")
	sid := e.assign(admin)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	sid2 := e.ids["student2"]

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон со статистикой", "direction": "other"}, 201, &m)
	id := str(m, "id")
	base := "/api/marathons/" + id
	if get(m, "marathon", "stats_enabled") != false || get(m, "marathon", "rating_enabled") != false {
		t.Fatalf("defaults: %v", m["marathon"])
	}
	var cat obj
	teacher.must("POST", "/api/score-categories", obj{"name": "Грамматика"}, 201, &cat)
	var course obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	// Lesson 1: instant test, two questions × 5 points, category «Грамматика».
	// Lesson 2: "after" test, one question × 1 point, the default category.
	lids := []string{}
	exs := []obj{}
	for i, title := range []string{"Урок 1", "Урок 2"} {
		var l, sec, ex obj
		teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": title}, 201, &l)
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/sections", obj{"title": "Раздел"}, 201, &sec)
		body := obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест"}
		if i == 0 {
			body["settings"] = obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 5, "category_id": str(cat, "id")}
			body["questions"] = []obj{testQuestion("She ___ a teacher.", "is", "are"), testQuestion("They ___ friends.", "are", "is")}
		} else {
			body["settings"] = obj{"check_mode": "after", "scoring_enabled": true, "points_per_correct": 1}
			body["questions"] = []obj{testQuestion("I ___ here.", "am", "is")}
		}
		teacher.must("POST", "/api/lessons/"+str(l, "id")+"/exercises", body, 201, &ex)
		teacher.must("POST", base+"/lessons", obj{"lesson_id": str(l, "id")}, 200, nil)
		lids, exs = append(lids, str(l, "id")), append(exs, ex)
	}
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid, sid2}}, 200, nil)

	code := func(c *client, method, path string, body any, want int, wantCode string) {
		t.Helper()
		var out obj
		c.must(method, path, body, want, &out)
		if str(out, "error", "code") != wantCode {
			t.Fatalf("%s %s: want %s, got %v", method, path, wantCode, out)
		}
	}
	answer := func(c *client, ex obj, labels ...string) string {
		t.Helper()
		var v obj
		c.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex, "id")+"/attempt", nil, 200, &v)
		instant := get(v, "version", "settings", "check_mode") == "instant"
		for qi, label := range labels {
			q := get(v, "version", "questions", qi).(map[string]any)
			for _, o := range q["options"].([]any) {
				if o.(map[string]any)["label"] == label {
					c.must("PUT", "/api/attempts/"+str(v, "attempt", "id")+"/answers/"+q["id"].(string),
						obj{"option_ids": []string{o.(map[string]any)["id"].(string)}, "check": instant}, 200, nil)
				}
			}
		}
		return str(v, "attempt", "id")
	}
	stats := func() obj {
		t.Helper()
		var out obj
		teacher.must("GET", base+"/stats", nil, 200, &out)
		return out
	}
	row := func(st obj, student string) obj {
		t.Helper()
		for _, r := range st["items"].([]any) {
			if r.(map[string]any)["student_id"] == student {
				return r.(map[string]any)
			}
		}
		return nil
	}
	// lessonPoints is the earned points of a student over the active lessons from the teacher's lesson view,
	// i.e. attempts.Points lesson by lesson: the statistics must always equal its sum.
	lessonPoints := func(student string, lessons ...string) int {
		t.Helper()
		sum := 0
		for _, lid := range lessons {
			var rv obj
			teacher.must("GET", base+"/students/"+student+"/lessons/"+lid, nil, 200, &rv)
			sum += num(rv, "points", "earned")
		}
		return sum
	}
	check := func(what string, student string, completed, earned int, place int) {
		t.Helper()
		r := row(stats(), student)
		if r == nil {
			t.Fatalf("%s: no row for %s", what, student)
		}
		if num(r, "completed_lessons") != completed || num(r, "points", "earned") != earned || num(r, "place") != place {
			t.Fatalf("%s: %v", what, r)
		}
		if want := lessonPoints(student, lids...); want != earned {
			t.Fatalf("%s: statistics %d, sum of attempts.Points %d", what, earned, want)
		}
	}

	// ---- Off by default: everything is refused, also by direct API ----
	code(teacher, "GET", base+"/stats", nil, 403, "marathon_stats_disabled")
	code(teacher, "GET", base+"/rating", nil, 403, "marathon_stats_disabled")
	code(student, "GET", base+"/rating", nil, 403, "marathon_stats_disabled")
	code(student, "GET", base+"/lessons/"+lids[0]+"/results", nil, 403, "marathon_stats_disabled")
	var my obj
	student.must("GET", base+"/me/results", nil, 200, &my)
	if get(my, "points") != nil || get(my, "rating") != nil || num(my, "total_lessons") != 2 || get(my, "stats_enabled") != false {
		t.Fatalf("my results with the statistics off: %v", my)
	}

	// ---- Settings: rights and validation ----
	teacher2.must("PUT", base+"/stats-settings", obj{"stats_enabled": true}, 404, nil)
	student.must("PUT", base+"/stats-settings", obj{"stats_enabled": true}, 403, nil)
	teacher.must("PUT", base+"/stats-settings", obj{}, 422, nil)
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": true, "rating_limit": 4}, 422, nil)
	// The rating cannot be on without the statistics.
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": false, "rating_enabled": true}, 200, &m)
	if get(m, "marathon", "rating_enabled") != false {
		t.Fatalf("rating without statistics: %v", m["marathon"])
	}
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": true}, 200, &m)
	if get(m, "marathon", "stats_enabled") != true || get(m, "marathon", "rating_enabled") != false {
		t.Fatalf("stats on: %v", m["marathon"])
	}
	teacher2.must("GET", base+"/stats", nil, 404, nil)
	student.must("GET", base+"/stats", nil, 403, nil)
	code(student, "GET", base+"/rating", nil, 403, "marathon_rating_disabled")
	teacher.must("GET", base+"/rating", nil, 200, nil) // the owner sees the standings in the statistics anyway

	// ---- Empty statistics: both students, zero, shared first place ----
	st := stats()
	if len(st["items"].([]any)) != 2 || num(st, "total_lessons") != 2 || num(row(st, sid), "place") != 1 || num(row(st, sid2), "place") != 1 {
		t.Fatalf("empty statistics: %v", st)
	}

	// ---- Answers: student 5 (one wrong), student2 10 ----
	answer(student, exs[0], "is", "is")
	answer(student2, exs[0], "is", "are")
	check("after answers", sid, 0, 5, 2)
	check("after answers", sid2, 0, 10, 1)
	st = stats()
	// Columns: categories in lesson order — «Грамматика» (lesson 1) and the default «Универсальная» (lesson 2).
	if len(st["categories"].([]any)) != 2 || str(st, "categories", 0, "category_name") != "Грамматика" || str(st, "categories", 1, "category_name") != "Универсальная" {
		t.Fatalf("categories: %v", st["categories"])
	}
	if c := get(row(st, sid2), "points", "categories", 0).(map[string]any); c["category_name"] != "Грамматика" || c["earned"] != float64(10) || c["max"] != float64(10) {
		t.Fatalf("category of student2: %v", c)
	}
	// "after" test: an unfinished attempt gives nothing, the finished one counts.
	a2 := answer(student2, exs[1], "am")
	check("after an unfinished after-test", sid2, 0, 10, 1)
	student2.must("POST", "/api/attempts/"+a2+"/finish", nil, 200, nil)
	check("after finishing it", sid2, 0, 11, 1)

	// ---- Teacher check: sending, revision, acceptance, cancellation and second completion ----
	teacher.must("PUT", base+"/review-settings", obj{"teacher_check": true}, 200, nil)
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	check("after sending", sid, 0, 5, 2)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/review", obj{"student_id": sid, "to_status": "revision"}, 200, nil)
	check("after revision", sid, 0, 5, 2)
	var v obj
	student.must("GET", "/api/classrooms/"+id+"/exercises/"+str(exs[0], "id"), nil, 200, &v)
	q := get(v, "version", "questions", 1).(map[string]any)
	for _, o := range q["options"].([]any) {
		if o.(map[string]any)["label"] == "are" {
			student.must("PUT", "/api/attempts/"+str(v, "attempt", "id")+"/answers/"+q["id"].(string), obj{"option_ids": []string{o.(map[string]any)["id"].(string)}, "check": true}, 200, nil)
		}
	}
	check("after correcting", sid, 0, 10, 2)
	student.must("POST", base+"/lessons/"+lids[0]+"/submit", nil, 200, nil)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/review", obj{"student_id": sid, "to_status": "completed", "rating": "nice"}, 200, nil)
	check("after acceptance", sid, 1, 10, 2)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/review", obj{"student_id": sid, "to_status": "submitted"}, 200, nil)
	check("after cancelling", sid, 0, 10, 2)
	teacher.must("POST", base+"/lessons/"+lids[0]+"/review", obj{"student_id": sid, "to_status": "completed", "rating": "nice"}, 200, nil)
	check("after the second completion", sid, 1, 10, 2)

	// ---- Rating for students: names and points only, the viewer marked ----
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": true, "rating_enabled": true, "rating_limit": 3}, 200, nil)
	var rating obj
	b := student.must("GET", base+"/rating", nil, 200, &rating)
	if strings.Contains(string(b), "@test.local") || num(rating, "total") != 2 || str(rating, "items", 0, "student_id") != sid2 ||
		num(rating, "items", 0, "points") != 11 || get(rating, "items", 1, "me") != true || get(rating, "me") != nil {
		t.Fatalf("student rating: %s", b)
	}
	if strings.Contains(string(student.must("GET", base+"/lessons/"+lids[0]+"/results", nil, 200, nil)), "@test.local") {
		t.Fatal("lesson results leak emails to a student")
	}
	var lr obj
	student.must("GET", base+"/lessons/"+lids[0]+"/results", nil, 200, &lr)
	// In lesson 1 both have 10 points: a shared first place.
	if num(lr, "points", "earned") != 10 || num(lr, "rating", "items", 0, "place") != 1 || num(lr, "rating", "items", 1, "place") != 1 || get(lr, "students") != nil {
		t.Fatalf("student lesson results: %v", lr)
	}
	student.must("GET", base+"/me/results", nil, 200, &my)
	if num(my, "completed_lessons") != 1 || num(my, "finished_exercises") != 1 || num(my, "exercises") != 2 ||
		num(my, "points", "earned") != 10 || num(my, "points", "max") != 11 || num(my, "rating", "items", 1, "place") != 2 {
		t.Fatalf("my results: %v", my)
	}
	teacher.must("GET", base+"/me/results", nil, 403, nil)

	// ---- Statistics comment: owner only, this marathon only (Н7) ----
	teacher2.must("PUT", base+"/lessons/"+lids[0]+"/stats-comment", obj{"body": "x"}, 404, nil)
	student.must("PUT", base+"/lessons/"+lids[0]+"/stats-comment", obj{"body": "x"}, 403, nil)
	teacher.must("PUT", base+"/lessons/"+lids[0]+"/stats-comment", obj{"body": strings.Repeat("я", 2001)}, 422, nil)
	teacher.must("PUT", base+"/lessons/"+lids[0]+"/stats-comment", obj{"body": "  Обратите внимание на to be.  "}, 200, nil)
	teacher.must("PUT", base+"/lessons/"+lids[0]+"/stats-comment", obj{"body": "  Обратите внимание на to be.  "}, 200, nil) // repeat
	student.must("GET", base+"/lessons/"+lids[0]+"/results", nil, 200, &lr)
	if str(lr, "comment", "body") != "Обратите внимание на to be." {
		t.Fatalf("comment for the student: %v", lr["comment"])
	}
	var other obj
	teacher.must("POST", "/api/marathons", obj{"name": "Другой марафон", "direction": "other"}, 201, &other)
	teacher.must("POST", "/api/marathons/"+str(other, "id")+"/lessons", obj{"lesson_id": lids[0]}, 200, nil)
	teacher.must("PUT", "/api/marathons/"+str(other, "id")+"/stats-settings", obj{"stats_enabled": true}, 200, nil)
	var olr obj
	teacher.must("GET", "/api/marathons/"+str(other, "id")+"/lessons/"+lids[0]+"/results", nil, 200, &olr)
	if get(olr, "comment") != nil || len(olr["students"].([]any)) != 0 {
		t.Fatalf("the comment or results leaked into another marathon: %v", olr)
	}
	var tlr obj
	teacher.must("GET", base+"/lessons/"+lids[0]+"/results", nil, 200, &tlr)
	if len(tlr["students"].([]any)) != 2 || str(tlr, "students", 0, "email") == "" || get(tlr, "students", 0, "completed") == nil {
		t.Fatalf("teacher lesson results: %v", tlr)
	}

	// ---- Deactivation: out of the rating, still in the teacher's statistics ----
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid2}, "active": false}, 200, nil)
	check("student2 deactivated", sid2, 0, 11, 0)
	check("student1 alone in the rating", sid, 1, 10, 1)
	student.must("GET", base+"/rating", nil, 200, &rating)
	if num(rating, "total") != 1 || str(rating, "items", 0, "student_id") != sid {
		t.Fatalf("rating without the deactivated student: %v", rating)
	}
	code(student2, "GET", base+"/rating", nil, 403, "marathon_deactivated")
	code(student2, "GET", base+"/me/results", nil, 403, "marathon_deactivated")
	code(student2, "GET", base+"/lessons/"+lids[0]+"/results", nil, 403, "marathon_deactivated")
	teacher.must("PUT", base+"/students/active", obj{"student_ids": []string{sid2}, "active": true}, 200, nil)
	check("student2 back", sid2, 0, 11, 1)

	// ---- Mode change: lesson statuses start over, points stay ----
	var cls obj
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &cls)
	teacher.must("POST", base+"/mode", obj{"mode": "progressive", "expected_epoch": num(cls, "marathon", "progress_epoch"), "confirm_reset": true}, 200, nil)
	check("after the mode change", sid, 0, 10, 2)
	check("after the mode change", sid2, 0, 11, 1)
	// The lesson rule now applies to lesson results: lesson 2 is closed for student 1.
	code(student, "GET", base+"/lessons/"+lids[1]+"/results", nil, 403, "marathon_lesson_locked")

	// ---- Removing and adding a student back ----
	teacher.must("DELETE", base+"/students/"+sid2, nil, 200, nil)
	st = stats()
	if len(st["items"].([]any)) != 1 || row(st, sid2) != nil {
		t.Fatalf("removed student in the statistics: %v", st)
	}
	student.must("GET", base+"/rating", nil, 200, &rating)
	if num(rating, "total") != 1 {
		t.Fatalf("removed student in the rating: %v", rating)
	}
	code(student2, "GET", base+"/rating", nil, 404, "not_found")
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid2}}, 200, nil)
	check("student2 added back", sid2, 0, 11, 1)

	// ---- Removing a lesson: its points leave the statistics ----
	teacher.must("DELETE", base+"/lessons/"+lids[1], nil, 200, nil)
	st = stats()
	if num(st, "total_lessons") != 1 || num(row(st, sid2), "points", "earned") != 10 || num(row(st, sid2), "points", "max") != 10 {
		t.Fatalf("after removing lesson 2: %v", row(st, sid2))
	}
	if lessonPoints(sid2, lids[0]) != 10 {
		t.Fatal("statistics differ from attempts.Points after removing a lesson")
	}
	teacher.must("POST", base+"/lessons", obj{"lesson_id": lids[1]}, 200, nil)
	check("lesson 2 back", sid2, 0, 11, 1)

	// ---- End date: students are closed out, the final standings stay for the teacher ----
	teacher.must("PATCH", base+"/settings", obj{"end_at": time.Now().Add(-time.Minute).UTC()}, 200, nil)
	code(student, "GET", base+"/rating", nil, 403, "marathon_closed")
	teacher.must("GET", base+"/rating", nil, 200, &rating)
	if num(rating, "total") != 2 {
		t.Fatalf("final standings after the end date: %v", rating)
	}

	// ---- Statistics off again: refused for everyone ----
	teacher.must("PATCH", base+"/settings", obj{"end_at": nil}, 200, nil)
	teacher.must("PUT", base+"/stats-settings", obj{"stats_enabled": false, "rating_enabled": true, "rating_limit": 3}, 200, &m)
	if get(m, "marathon", "rating_enabled") != false || num(m, "marathon", "rating_limit") != 3 {
		t.Fatalf("stats off: %v", m["marathon"])
	}
	code(teacher, "GET", base+"/stats", nil, 403, "marathon_stats_disabled")
	code(student, "GET", base+"/rating", nil, 403, "marathon_stats_disabled")
	code(student, "GET", base+"/lessons/"+lids[0]+"/results", nil, 403, "marathon_stats_disabled")
}
