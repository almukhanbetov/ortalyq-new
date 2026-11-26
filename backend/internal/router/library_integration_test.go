package router_test

import (
	"context"
	"testing"
)

// «Материалы» → «Каталог»: a teacher finds «English A1 — Первые разговоры», adds it and gets an own editable copy
// (lessons, exercises, homework, text blocks with teacher-only notes still hidden); adding again returns the same copy.
func TestMaterialCatalog(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	teacher, other, student := e.login("teacher"), e.login("teacher2"), e.login("student")
	const key = "english-a1-first-conversations"

	var list struct {
		Items []obj `json:"items"`
		Total int   `json:"total"`
	}
	teacher.must("GET", "/api/material-catalog?q="+"%D0%9F%D0%95%D0%A0%D0%92%D0%AB%D0%95", nil, 200, &list) // «ПЕРВЫЕ»
	if list.Total != 1 || str(list.Items[0], "key") != key || list.Items[0]["owned_course_id"] != nil ||
		str(list.Items[0], "level") != "A1" || num(list.Items[0], "lesson_count") != 3 {
		t.Fatalf("search: %+v", list)
	}
	list.Items = nil
	teacher.must("GET", "/api/material-catalog?q=chemistry", nil, 200, &list)
	if list.Total != 0 {
		t.Fatalf("no match expected: %+v", list)
	}
	var c obj
	teacher.must("GET", "/api/material-catalog/"+key, nil, 200, &c)
	if str(c, "lessons", 0, "title") != "Lesson 1. Hello!" || num(c, "lessons", 0, "homework") != 2 || num(c, "lessons", 0, "exercises") != 4 {
		t.Fatalf("detail: %v", c)
	}
	teacher.must("GET", "/api/material-catalog/nope", nil, 404, nil)
	student.must("GET", "/api/material-catalog", nil, 403, nil)
	student.must("POST", "/api/material-catalog/"+key+"/add", nil, 403, nil)

	var added, again obj
	teacher.must("POST", "/api/material-catalog/"+key+"/add", nil, 201, &added)
	cid := str(added, "course_id")
	teacher.must("POST", "/api/material-catalog/"+key+"/add", nil, 200, &again)
	if str(again, "course_id") != cid || again["created"] != false {
		t.Fatalf("second add: %v", again)
	}
	count := func(q string, args ...any) int {
		var n int
		if err := e.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM courses WHERE owner_id = $1 AND title = 'English A1 — Первые разговоры'`, e.ids["teacher"]); n != 1 {
		t.Fatalf("copies: %d", n)
	}
	if n := count(`SELECT count(*) FROM exercises e JOIN lessons l ON l.id = e.lesson_id WHERE l.course_id = $1 AND e.kind = 'text' AND e.hidden`, cid); n != 9 {
		t.Fatalf("teacher-only notes: %d", n)
	}
	if n := count(`SELECT count(*) FROM exercises e JOIN lessons l ON l.id = e.lesson_id WHERE l.course_id = $1 AND e.scope = 'homework'`, cid); n != 6 {
		t.Fatalf("homework: %d", n)
	}
	c = nil
	teacher.must("GET", "/api/material-catalog/"+key, nil, 200, &c)
	if str(c, "owned_course_id") != cid {
		t.Fatalf("owned: %v", c["owned_course_id"])
	}
	// The copy is the teacher's own: editable, private.
	var course obj
	teacher.must("GET", "/api/courses/"+cid, nil, 200, &course)
	teacher.must("PATCH", "/api/lessons/"+str(course, "lessons", 0, "id"), obj{"title": "Hello! (mine)"}, 200, nil)
	other.must("GET", "/api/courses/"+cid, nil, 404, nil)
	c = nil
	other.must("GET", "/api/material-catalog/"+key, nil, 200, &c)
	if c["owned_course_id"] != nil {
		t.Fatalf("another teacher sees a copy: %v", c)
	}
	teacher.must("POST", "/api/material-catalog/"+key+"/add", nil, 200, nil)
	teacher.must("GET", "/api/courses/"+cid, nil, 200, &course)
	if str(course, "lessons", 0, "title") != "Hello! (mine)" {
		t.Fatalf("edit overwritten: %v", course["lessons"])
	}

	// A deleted copy can be added again from the catalog: a fresh one.
	if _, err := e.pool.Exec(ctx, `DELETE FROM courses WHERE id = $1`, cid); err != nil {
		t.Fatal(err)
	}
	var fresh obj
	teacher.must("POST", "/api/material-catalog/"+key+"/add", nil, 201, &fresh)
	if str(fresh, "course_id") == cid || count(`SELECT count(*) FROM lessons WHERE course_id = $1`, str(fresh, "course_id")) != 3 {
		t.Fatalf("fresh copy: %v", fresh)
	}
}
