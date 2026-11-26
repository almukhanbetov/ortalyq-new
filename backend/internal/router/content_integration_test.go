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
	if exByScope(l1, "lesson") != 4 || exByScope(l1, "homework") != 2 {
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
		strings.Contains(string(raw), "is_correct") || strings.Contains(string(raw), "correct_answers") {
		t.Fatalf("student view: %s", raw)
	}
}
