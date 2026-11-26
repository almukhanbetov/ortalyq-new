package router_test

import (
	"strings"
	"testing"
)

func gapQuestion(prompt, answer string) obj {
	return obj{"prompt": prompt, "options": []obj{{"label": answer, "is_correct": true}}}
}

// «Слова и пропуски»: the teacher creates and edits a one-gap exercise; the student types a word; the server checks
// it ignoring case and spaces around it; the answer stays hidden until the check.
func TestGapsExercise(t *testing.T) {
	e := setup(t)
	admin, teacher, student := e.login("admin"), e.login("teacher"), e.login("student")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	var cls, course, lesson, sec obj
	teacher.must("POST", "/api/classrooms", obj{"name": "Gaps class", "student_id": e.ids["student"]}, 201, &cls)
	cid := str(cls, "id")
	teacher.must("POST", "/api/courses", obj{"title": "English"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "To be"}, 201, &lesson)
	lid := str(lesson, "id")
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Practice"}, 201, &sec)

	// The catalog offers the type.
	var cat struct {
		Items []obj `json:"items"`
	}
	teacher.must("GET", "/api/exercise-types", nil, 200, &cat)
	found := false
	for _, c := range cat.Items {
		if c["kind"] == "gaps" && c["available"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps not available in the catalog: %v", cat)
	}

	// Validation on the server.
	create := func(q obj, want int, out any) {
		teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "gaps",
			"title": "Слова и пропуски", "settings": obj{"check_mode": "instant"}, "questions": []obj{q}}, want, out)
	}
	create(gapQuestion("I am a student.", "am"), 422, nil)
	create(gapQuestion("I ___ a ___.", "am"), 422, nil)
	create(gapQuestion("I ___ a student.", " "), 422, nil)

	// Create, then edit: a new version with the changed phrase and answer.
	var ex obj
	create(gapQuestion("I ___ a teacher.", "is"), 201, &ex)
	exID := str(ex, "id")
	var edited obj
	teacher.must("PUT", "/api/exercises/"+exID, obj{"title": "Слова и пропуски", "settings": obj{"check_mode": "instant"},
		"questions": []obj{gapQuestion("I ___ a student.", "am")}}, 200, &edited)
	if num(edited, "version", "number") != 2 || str(edited, "kind") != "gaps" ||
		str(edited, "version", "questions", 0, "prompt") != "I ___ a student." || str(edited, "version", "questions", 0, "options", 0, "label") != "am" {
		t.Fatalf("edit: %v", edited)
	}

	// The student sees the phrase, but not the answer.
	teacher.must("PUT", "/api/classrooms/"+cid+"/current-lesson", obj{"lesson_id": lid}, 200, nil)
	raw := student.must("GET", "/api/classrooms/"+cid+"/lessons/"+lid+"/exercises", nil, 200, nil)
	if !strings.Contains(string(raw), "I ___ a student.") || strings.Contains(string(raw), `"am"`) || strings.Contains(string(raw), "is_correct") {
		t.Fatalf("key leaked or phrase missing: %s", raw)
	}
	var v obj
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+exID+"/attempt", nil, 200, &v)
	aid, qid := str(v, "attempt", "id"), str(v, "version", "questions", 0, "id")
	if opts, _ := get(v, "version", "questions", 0, "options").([]any); len(opts) != 0 {
		t.Fatalf("options sent to the student: %v", opts)
	}

	// Typing without a check is saved but not graded, and the key is still hidden.
	raw = student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"text": "  is "}, 200, nil)
	if strings.Contains(string(raw), `"am"`) || strings.Contains(string(raw), `"result":"`) {
		t.Fatalf("revealed before the check: %s", raw)
	}
	// Bad requests: options for a gap, an empty check, an over-long word.
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"option_ids": []string{qid}}, 422, nil)
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"text": "  ", "check": true}, 422, nil)
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"text": strings.Repeat("a", 101)}, 422, nil)

	// Check: case and spaces around the word do not matter; the answer is revealed only now.
	var checked obj
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"text": "  AM ", "check": true}, 200, &checked)
	if str(checked, "attempt", "answers", 0, "result") != "correct" || str(checked, "attempt", "answers", 0, "text") != "AM" ||
		str(checked, "attempt", "answers", 0, "correct_answers", 0) != "am" {
		t.Fatalf("check: %v", checked["attempt"])
	}
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"text": "AM", "check": true}, 200, nil) // idempotent retry
	student.must("PUT", "/api/attempts/"+aid+"/answers/"+qid, obj{"text": "is", "check": true}, 409, nil)
	var fin obj
	student.must("POST", "/api/attempts/"+aid+"/finish", nil, 200, &fin)
	if num(fin, "attempt", "summary", "correct") != 1 || num(fin, "attempt", "summary", "wrong") != 0 {
		t.Fatalf("summary: %v", fin["attempt"])
	}

	// A wrong word after the "check after finishing" mode: graded wrong at the end, answer hidden until then.
	var ex2, v2, a2, f2 obj
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "gaps",
		"title": "После завершения", "settings": obj{"check_mode": "after"}, "questions": []obj{gapQuestion("She ___ happy.", "is")}}, 201, &ex2)
	student.must("POST", "/api/classrooms/"+cid+"/exercises/"+str(ex2, "id")+"/attempt", nil, 200, &v2)
	q2 := str(v2, "version", "questions", 0, "id")
	student.must("PUT", "/api/attempts/"+str(v2, "attempt", "id")+"/answers/"+q2, obj{"text": "are", "check": true}, 400, nil)
	raw = student.must("PUT", "/api/attempts/"+str(v2, "attempt", "id")+"/answers/"+q2, obj{"text": "are"}, 200, &a2)
	if strings.Contains(string(raw), `"is"`) {
		t.Fatalf("after-mode revealed early: %s", raw)
	}
	student.must("POST", "/api/attempts/"+str(v2, "attempt", "id")+"/finish", nil, 200, &f2)
	if str(f2, "attempt", "answers", 0, "result") != "wrong" || str(f2, "attempt", "answers", 0, "correct_answers", 0) != "is" ||
		num(f2, "attempt", "summary", "wrong") != 1 {
		t.Fatalf("after-mode finish: %v", f2["attempt"])
	}

	// Teacher statistics count the typed answers.
	var st obj
	teacher.must("GET", "/api/exercises/"+exID+"/stats", nil, 200, &st)
	if num(st, "questions", 0, "correct") != 1 {
		t.Fatalf("stats: %v", st)
	}
	// Copy to homework keeps the kind and the key.
	var cp obj
	teacher.must("POST", "/api/exercises/"+exID+"/copy-to-homework", nil, 201, &cp)
	if str(cp, "kind") != "gaps" || str(cp, "version", "questions", 0, "options", 0, "label") != "am" {
		t.Fatalf("copy: %v", cp)
	}
}
