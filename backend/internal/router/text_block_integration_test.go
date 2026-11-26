package router_test

import (
	"strings"
	"testing"
)

// «Текст» through the API: the teacher creates and edits a block in a section; the student reads it in the class but
// cannot start an attempt; a teacher-only block never reaches the student; text cannot go to homework.
func TestTextBlock(t *testing.T) {
	e := setup(t)
	admin, teacher, student := e.login("admin"), e.login("teacher"), e.login("student")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	var cls, course, lesson, sec obj
	teacher.must("POST", "/api/classrooms", obj{"name": "Text class", "student_id": e.ids["student"]}, 201, &cls)
	teacher.must("POST", "/api/courses", obj{"title": "English"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Hello"}, 201, &lesson)
	lid, cid := str(lesson, "id"), str(cls, "id")
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Intro"}, 201, &sec)
	text := func(title, body string, hidden bool) obj {
		return obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "text", "title": title, "hidden": hidden,
			"settings": obj{"check_mode": "instant"}, "questions": []obj{{"prompt": body}}}
	}
	path := "/api/lessons/" + lid + "/exercises"
	teacher.must("POST", path, text("Empty", "  ", false), 422, nil)
	teacher.must("POST", path, text("Long", strings.Repeat("a", 2001), false), 422, nil)
	withOptions := text("Options", "Read it.", false)
	withOptions["questions"] = []obj{gapQuestion("Read ___ it.", "x")}
	teacher.must("POST", path, withOptions, 422, nil)
	hw := text("Homework text", "Read it.", false)
	hw["scope"], hw["section_id"] = "homework", nil
	teacher.must("POST", path, hw, 422, nil)

	var block, notes, edited obj
	teacher.must("POST", path, text("Dialogue", "Ann: Hello!\nBen: Hi!", false), 201, &block)
	teacher.must("POST", path, text("Notes", "Секретный ключ: am", true), 201, &notes)
	teacher.must("PUT", "/api/exercises/"+str(block, "id"), obj{"title": "Dialogue", "settings": obj{"check_mode": "instant"},
		"questions": []obj{{"prompt": "Ann: Hello! I'm Ann.\nBen: Hi, Ann!"}}}, 200, &edited)
	if num(edited, "version", "number") != 2 || str(edited, "kind") != "text" {
		t.Fatalf("edit: %v", edited)
	}
	teacher.must("POST", "/api/exercises/"+str(block, "id")+"/copy-to-homework", nil, 409, nil)

	teacher.must("PUT", "/api/classrooms/"+cid+"/current-lesson", obj{"lesson_id": lid}, 200, nil)
	raw := student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises", nil, 200, nil)
	if !strings.Contains(string(raw), "Ann: Hello! I'm Ann.\\nBen: Hi, Ann!") || strings.Contains(string(raw), "Секретный") || strings.Contains(string(raw), str(notes, "id")) {
		t.Fatalf("student list: %s", raw)
	}
	student.must("GET", "/api/classrooms/"+cid+"/exercises/"+str(notes, "id"), nil, 404, nil)
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+str(notes, "id")+"/attempt", nil, 404, nil)
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+str(block, "id")+"/attempt", nil, 409, nil)
	traw := teacher.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises", nil, 200, nil)
	if !strings.Contains(string(traw), "Секретный ключ") {
		t.Fatalf("teacher misses the notes: %s", traw)
	}
	// Text blocks are not exercises to grade: the lesson has nothing to count.
	var pr obj
	teacher.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/progress?student_id="+e.ids["student"], nil, 200, &pr)
	if _, ok := pr["lesson_points"].(map[string]any); !ok || num(pr, "lesson_points", "exercises") != 0 {
		t.Fatalf("text counted as an exercise: %v", pr["lesson_points"])
	}
}
