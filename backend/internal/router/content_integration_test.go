package router_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"ortalyq/backend/internal/content"
)

// The author course loader (`api seed-course`): the first load creates the course; a repeated load creates no
// duplicates, keeps what the teacher changed, does not bring back a deleted lesson and adds only new lessons.
// The teacher's other courses stay; a student in a class gets no answer keys.
func TestContentLoad(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	teacher, student := e.login("teacher"), e.login("student")
	owner := uuid.MustParse(e.ids["teacher"])
	var own obj
	teacher.must("POST", "/api/courses", obj{"title": "English Starter A1"}, 201, &own)

	count := func(q string, args ...any) int {
		var n int
		if err := e.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	r, err := content.Load(ctx, e.pool, owner, content.EnglishA1)
	if err != nil {
		t.Fatal(err)
	}
	if !r.CourseCreated || len(r.Created) != 3 {
		t.Fatalf("first load: %+v", r)
	}
	cid := r.CourseID.String()
	var course obj
	teacher.must("GET", "/api/courses/"+cid, nil, 200, &course)
	lessons, _ := course["lessons"].([]any)
	if str(course, "title") != "English A1 — Первые разговоры" || len(lessons) != 3 || str(lessons[0], "title") != "Lesson 1. Hello!" {
		t.Fatalf("course: %v", course)
	}
	l1 := str(lessons[0], "id")
	var lesson obj
	teacher.must("GET", "/api/lessons/"+l1, nil, 200, &lesson)
	if secs, _ := lesson["sections"].([]any); len(secs) != 3 || str(secs[0], "title") != "Знакомство с темой" {
		t.Fatalf("sections: %v", lesson["sections"])
	}
	if !strings.Contains(str(lesson, "description"), "Методичка") {
		t.Fatalf("guide not stored: %v", lesson["description"])
	}
	exByScope := func(lid, scope string) int {
		return count(`SELECT count(*) FROM exercises WHERE lesson_id = $1 AND scope = $2 AND deleted_at IS NULL`, lid, scope)
	}
	if exByScope(l1, "lesson") != 4+6 || exByScope(l1, "homework") != 2 { // 4 exercises + 6 text blocks
		t.Fatalf("exercises: lesson %d, homework %d", exByScope(l1, "lesson"), exByScope(l1, "homework"))
	}
	total := count(`SELECT count(*) FROM exercises`)

	// The teacher edits lesson 1 and one of its exercises.
	teacher.must("PATCH", "/api/lessons/"+l1, obj{"title": "Hello! (my version)"}, 200, nil)
	var exs []obj
	var list struct {
		Items []obj `json:"items"`
	}
	teacher.must("GET", "/api/lessons/"+l1+"/exercises?scope=lesson", nil, 200, &list)
	exs = list.Items
	exID := ""
	for _, x := range exs {
		if str(x, "kind") == "gaps" && exID == "" {
			exID = str(x, "id")
		}
	}
	if exID == "" {
		t.Fatal("no gaps exercise in the editor")
	}
	teacher.must("PUT", "/api/exercises/"+exID, obj{"title": "Changed by me", "settings": obj{"check_mode": "instant"},
		"questions": []obj{gapQuestion("Hello, my name ___ Ben.", "is")}}, 200, nil)

	// A repeated load changes nothing.
	r, err = content.Load(ctx, e.pool, owner, content.EnglishA1)
	if err != nil {
		t.Fatal(err)
	}
	if r.CourseCreated || len(r.Created) != 0 || len(r.Kept) != 3 || r.CourseID.String() != cid {
		t.Fatalf("second load: %+v", r)
	}
	if n := count(`SELECT count(*) FROM courses WHERE owner_id = $1`, owner); n != 2 {
		t.Fatalf("courses after reload: %d", n)
	}
	if n := count(`SELECT count(*) FROM exercises`); n != total {
		t.Fatalf("exercises after reload: %d, want %d", n, total)
	}
	teacher.must("GET", "/api/lessons/"+l1, nil, 200, &lesson)
	if str(lesson, "title") != "Hello! (my version)" {
		t.Fatalf("teacher's title overwritten: %v", lesson["title"])
	}
	if n := count(`SELECT count(*) FROM exercise_versions WHERE exercise_id = $1`, exID); n != 2 {
		t.Fatalf("versions of the edited exercise: %d", n)
	}

	// A deleted lesson does not come back; a lesson added to the content later is added alone.
	if _, err := e.pool.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, str(lessons[1], "id")); err != nil {
		t.Fatal(err)
	}
	more := content.EnglishA1
	more.Lessons = append(append([]content.Lesson{}, more.Lessons...), content.Lesson{Key: "extra", Title: "Lesson 4. Extra",
		Sections: []content.Section{{Title: content.SecIntro, Exercises: []content.Exercise{content.Gap("Extra", "We ___ friends.", "are", "instant")}}}})
	r, err = content.Load(ctx, e.pool, owner, more)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Created) != 1 || r.Created[0] != "Lesson 4. Extra" || len(r.Deleted) != 1 || len(r.Kept) != 2 {
		t.Fatalf("third load: %+v", r)
	}
	if n := count(`SELECT count(*) FROM lessons WHERE course_id = $1`, r.CourseID); n != 3 {
		t.Fatalf("lessons: %d", n)
	}
	// The teacher's own course is untouched.
	teacher.must("GET", "/api/courses/"+str(own, "id"), nil, 200, nil)

	// Another teacher gets an own copy; the first teacher's course stays private.
	r2, err := content.Load(ctx, e.pool, uuid.MustParse(e.ids["teacher2"]), content.EnglishA1)
	if err != nil || !r2.CourseCreated || r2.CourseID == r.CourseID {
		t.Fatalf("teacher2: %+v %v", r2, err)
	}
	e.login("teacher2").must("GET", "/api/courses/"+cid, nil, 404, nil)

	// A deleted course is not restored either.
	if _, err := e.pool.Exec(ctx, `DELETE FROM courses WHERE id = $1`, r2.CourseID); err != nil {
		t.Fatal(err)
	}
	if r2, err = content.Load(ctx, e.pool, uuid.MustParse(e.ids["teacher2"]), content.EnglishA1); err != nil || !r2.CourseDeleted {
		t.Fatalf("deleted course: %+v %v", r2, err)
	}

	// In a class the student sees the phrases and the test, but no keys.
	e.login("admin").must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	var cls obj
	teacher.must("POST", "/api/classrooms", obj{"name": "A1", "student_id": e.ids["student"]}, 201, &cls)
	l3 := str(lessons[2], "id")
	teacher.must("PUT", "/api/classrooms/"+str(cls, "id")+"/current-lesson", obj{"lesson_id": l3}, 200, nil)
	raw := student.must("GET", "/api/classrooms/"+str(cls, "id")+"/lessons/"+l3+"/exercises", nil, 200, nil)
	if !strings.Contains(string(raw), "This ___ my friend Tom.") || !strings.Contains(string(raw), "Choose the best answer") ||
		!strings.Contains(string(raw), "Ali: Hi, Sara! This is my friend Tom.") || !strings.Contains(string(raw), "Speaking task") ||
		strings.Contains(string(raw), "is_correct") || strings.Contains(string(raw), "correct_answers") ||
		strings.Contains(string(raw), "Заметки преподавателя") || strings.Contains(string(raw), "→") {
		t.Fatalf("student view: %s", raw)
	}
}

// Text blocks reach lessons loaded before blocks existed: they are added once, the dialogue and the phrases at the
// top of «Знакомство с темой»; the teacher's changes to exercises and to blocks survive every later load.
func TestContentBlocksForLoadedLessons(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	teacher := e.login("teacher")
	owner := uuid.MustParse(e.ids["teacher"])
	r, err := content.Load(ctx, e.pool, owner, content.EnglishA1)
	if err != nil || r.BlocksAdded != 18 {
		t.Fatalf("first load: %+v %v", r, err)
	}
	// Turn the course into one loaded before text blocks existed.
	if _, err := e.pool.Exec(ctx, `DELETE FROM exercises WHERE kind = 'text'; DELETE FROM content_imports WHERE key LIKE '%/block/%'`); err != nil {
		t.Fatal(err)
	}
	var course obj
	teacher.must("GET", "/api/courses/"+r.CourseID.String(), nil, 200, &course)
	l1 := str(course, "lessons", 0, "id")
	var list struct {
		Items []obj `json:"items"`
	}
	teacher.must("GET", "/api/lessons/"+l1+"/exercises?scope=lesson", nil, 200, &list)
	gapID, introID := str(list.Items[0], "id"), str(list.Items[0], "section_id")
	list.Items = nil // json reuses map elements of a non-empty slice
	teacher.must("PUT", "/api/exercises/"+gapID, obj{"title": "My intro gap", "settings": obj{"check_mode": "instant"},
		"questions": []obj{gapQuestion("Hi! My name ___ Dana.", "is")}}, 200, nil)

	r, err = content.Load(ctx, e.pool, owner, content.EnglishA1)
	if err != nil || r.BlocksAdded != 18 || r.BlocksKept != 0 || len(r.Created) != 0 {
		t.Fatalf("blocks load: %+v %v", r, err)
	}
	list.Items = nil
	teacher.must("GET", "/api/lessons/"+l1+"/exercises?scope=lesson", nil, 200, &list)
	intro := []string{}
	for _, x := range list.Items {
		if str(x, "section_id") == introID {
			intro = append(intro, str(x, "version", "title"))
		}
	}
	if strings.Join(intro, " | ") != "Dialogue | Useful phrases | My intro gap | Заметки преподавателя" {
		t.Fatalf("intro order: %v", intro)
	}
	// The teacher edits a block; a repeated load changes nothing.
	var dialogue obj
	for _, x := range list.Items {
		if str(x, "version", "title") == "Dialogue" {
			dialogue = x
		}
	}
	teacher.must("PUT", "/api/exercises/"+str(dialogue, "id"), obj{"title": "Our dialogue", "settings": obj{"check_mode": "instant"},
		"questions": []obj{{"prompt": "Dana: Hi!\nTom: Hello!"}}}, 200, nil)
	before := 0
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM exercises`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	r, err = content.Load(ctx, e.pool, owner, content.EnglishA1)
	if err != nil || r.BlocksAdded != 0 || r.BlocksKept != 18 {
		t.Fatalf("repeated load: %+v %v", r, err)
	}
	after := 0
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM exercises`).Scan(&after); err != nil || after != before {
		t.Fatalf("exercises %d → %d", before, after)
	}
	var got obj
	teacher.must("GET", "/api/exercises/"+str(dialogue, "id"), nil, 200, &got)
	if str(got, "version", "title") != "Our dialogue" || str(got, "version", "questions", 0, "prompt") != "Dana: Hi!\nTom: Hello!" {
		t.Fatalf("teacher's block overwritten: %v", got["version"])
	}
	got = nil
	teacher.must("GET", "/api/exercises/"+gapID, nil, 200, &got)
	if str(got, "version", "title") != "My intro gap" {
		t.Fatalf("teacher's gap overwritten: %v", got["version"])
	}
}
