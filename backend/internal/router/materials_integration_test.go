package router_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// Personal materials: create, search by title, open, edit; only the owner sees a material, including by a direct link.
func TestPersonalMaterialsAccess(t *testing.T) {
	e := setup(t)
	teacher, other, student := e.login("teacher"), e.login("teacher2"), e.login("student")

	var mat obj
	teacher.must("POST", "/api/courses", obj{"title": "  Present Simple: практикум  ", "description": "Тестовый материал", "cover_color": "#9254DE"}, 201, &mat)
	id := str(mat, "id")
	if str(mat, "title") != "Present Simple: практикум" || str(mat, "cover_color") != "#9254DE" {
		t.Fatalf("created: %v", mat)
	}
	teacher.must("POST", "/api/courses", obj{"title": "Kazakh basics"}, 201, nil)
	var lesson obj
	teacher.must("POST", "/api/courses/"+id+"/lessons", obj{"title": "Урок 1. Daily routine"}, 201, &lesson)

	// Validation.
	if st, _ := teacher.do("POST", "/api/courses", obj{"title": "   "}); st != 422 && st != 400 {
		t.Fatalf("empty title: %d", st)
	}
	// 5000 Cyrillic characters fit (the limit counts characters, not bytes); 5001 do not.
	teacher.must("PATCH", "/api/courses/"+id, obj{"description": strings.Repeat("ж", 5000)}, 200, nil)
	if st, _ := teacher.do("PATCH", "/api/courses/"+id, obj{"description": strings.Repeat("ж", 5001)}); st != 422 && st != 400 {
		t.Fatalf("long description: %d", st)
	}

	// Search: case-insensitive by material title, and by a lesson title inside it.
	search := func(c *client, q string) []string {
		var res struct {
			Items []obj `json:"items"`
			Total int   `json:"total"`
		}
		c.must("GET", "/api/courses?q="+url.QueryEscape(q), nil, 200, &res)
		out := []string{}
		for _, it := range res.Items {
			out = append(out, str(it, "title"))
		}
		if res.Total != len(out) {
			t.Fatalf("total %d vs %d items", res.Total, len(out))
		}
		return out
	}
	for q, want := range map[string]string{"present": "Present Simple: практикум", "ПРАКТИКУМ": "Present Simple: практикум", "daily routine": "Present Simple: практикум", "kazakh": "Kazakh basics"} {
		if got := search(teacher, q); len(got) != 1 || got[0] != want {
			t.Fatalf("search %q: %v", q, got)
		}
	}
	if got := search(teacher, "нет такого"); len(got) != 0 {
		t.Fatalf("no match: %v", got)
	}
	if got := search(teacher, ""); len(got) != 2 {
		t.Fatalf("all: %v", got)
	}

	// Edit and reopen.
	teacher.must("PATCH", "/api/courses/"+id, obj{"title": "Present Simple — практикум", "description": "Обновлено", "cover_color": "#16C784"}, 200, nil)
	var got obj
	teacher.must("GET", "/api/courses/"+id, nil, 200, &got)
	if str(got, "title") != "Present Simple — практикум" || str(got, "description") != "Обновлено" || str(got, "cover_color") != "#16C784" {
		t.Fatalf("after edit: %v", got)
	}
	if ls, _ := get(got, "lessons").([]any); len(ls) != 1 {
		t.Fatalf("lessons: %v", got)
	}

	// Another teacher: not listed, and a direct link answers 404 (no hint that it exists) for every action.
	if got := search(other, ""); len(got) != 0 {
		t.Fatalf("other teacher sees: %v", got)
	}
	lid := str(lesson, "id")
	for _, r := range [][2]string{
		{"GET", "/api/courses/" + id},
		{"PATCH", "/api/courses/" + id},
		{"POST", "/api/courses/" + id + "/lessons"},
		{"PUT", "/api/courses/" + id + "/lessons/order"},
		{"GET", "/api/lessons/" + lid},
		{"PATCH", "/api/lessons/" + lid},
		{"POST", "/api/lessons/" + lid + "/sections"},
	} {
		st, b := other.do(r[0], r[1], obj{"title": "Взлом", "ids": []string{lid}})
		if st != 404 {
			t.Fatalf("other teacher %s %s: want 404, got %d %s", r[0], r[1], st, b)
		}
	}
	// Reordering own list with someone else's material is refused.
	var own obj
	other.must("POST", "/api/courses", obj{"title": "Чужой"}, 201, &own)
	if st, _ := other.do("PUT", "/api/courses/order", obj{"ids": []string{str(own, "id"), id}}); st == 204 || st == 200 {
		t.Fatalf("reorder with a foreign id accepted: %d", st)
	}
	// Students have no materials section at all.
	for _, p := range []string{"/api/courses", "/api/courses/" + id, "/api/lessons/" + lid} {
		if st, _ := student.do("GET", p, nil); st != 403 {
			t.Fatalf("student GET %s: %d", p, st)
		}
	}
	// Unchanged after the attempts.
	teacher.must("GET", "/api/courses/"+id, nil, 200, &got)
	if str(got, "title") != "Present Simple — практикум" {
		t.Fatal(fmt.Sprint("changed by another user: ", got))
	}
}
