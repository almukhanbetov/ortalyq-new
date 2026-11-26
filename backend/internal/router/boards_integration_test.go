package router_test

import (
	"strings"
	"testing"
)

// «Интерактивные доски»: the owner creates, finds, saves and reopens a board; another teacher and a student cannot list,
// open or save it (also by a direct link); bad content is refused; a stale save is a 409 unless forced.
func TestBoards(t *testing.T) {
	e := setup(t)
	teacher, other, student := e.login("teacher"), e.login("teacher2"), e.login("student")

	teacher.must("POST", "/api/boards", obj{"title": "  "}, 422, nil)
	var b obj
	teacher.must("POST", "/api/boards", obj{"title": "English A1 — Hello!"}, 201, &b)
	id := str(b, "id")
	if num(b, "version") != 1 {
		t.Fatalf("created: %v", b)
	}
	teacher.must("POST", "/api/boards", obj{"title": "Numbers 1-10"}, 201, nil)

	content := obj{"items": []obj{
		{"type": "stroke", "color": "#1F2937", "width": 4, "points": [][2]float64{{10, 10}, {200, 120}, {5000, -3}, {99999, 0}}},
		{"type": "text", "color": "#2F80ED", "x": 100, "y": 200, "size": 40, "text": "  Hello! My name is …  "},
	}}
	var saved obj
	teacher.must("PUT", "/api/boards/"+id, obj{"content": content, "version": 1}, 200, &saved)
	if num(saved, "version") != 2 || str(saved, "content", "items", 1, "text") != "Hello! My name is …" ||
		get(saved, "content", "items", 0, "points", 2, 0).(float64) != 5000 || get(saved, "content", "items", 0, "points", 2, 1).(float64) != -3 || get(saved, "content", "items", 0, "points", 3, 0).(float64) != 20000 {
		t.Fatalf("saved: %v", saved)
	}
	var again obj
	teacher.must("GET", "/api/boards/"+id, nil, 200, &again)
	if len(get(again, "content", "items").([]any)) != 2 || str(again, "title") != "English A1 — Hello!" {
		t.Fatalf("reopened: %v", again)
	}

	// Search by title, newest first.
	var list struct {
		Items []obj `json:"items"`
		Total int   `json:"total"`
	}
	teacher.must("GET", "/api/boards?q=hello", nil, 200, &list)
	if list.Total != 1 || str(list.Items[0], "id") != id || list.Items[0]["content"] != nil {
		t.Fatalf("search: %+v", list)
	}
	list.Items = nil
	teacher.must("GET", "/api/boards?q=%25", nil, 200, &list)
	if list.Total != 0 {
		t.Fatalf("%% must not match everything: %+v", list)
	}

	// Stale save: 409, the board is unchanged; forced save wins.
	stale := obj{"content": obj{"items": []obj{}}, "version": 1}
	teacher.must("PUT", "/api/boards/"+id, stale, 409, nil)
	teacher.must("GET", "/api/boards/"+id, nil, 200, &again)
	if num(again, "version") != 2 {
		t.Fatalf("stale save changed the board: %v", again)
	}
	stale["force"] = true
	teacher.must("PUT", "/api/boards/"+id, stale, 200, &again)
	if num(again, "version") != 3 || len(get(again, "content", "items").([]any)) != 0 {
		t.Fatalf("forced: %v", again)
	}

	// Bad content.
	bad := func(item obj) {
		teacher.must("PUT", "/api/boards/"+id, obj{"content": obj{"items": []obj{item}}, "version": 3}, 422, nil)
	}
	bad(obj{"type": "stroke", "color": "red", "width": 4, "points": [][2]float64{{1, 1}}})
	bad(obj{"type": "stroke", "color": "#000000", "width": 0, "points": [][2]float64{{1, 1}}})
	bad(obj{"type": "stroke", "color": "#000000", "width": 4, "points": [][2]float64{}})
	bad(obj{"type": "text", "color": "#000000", "x": 1, "y": 1, "size": 20, "text": "  "})
	bad(obj{"type": "text", "color": "#000000", "x": 1, "y": 1, "size": 20, "text": strings.Repeat("a", 501)})
	bad(obj{"type": "image", "color": "#000000"})
	teacher.must("PUT", "/api/boards/"+id, obj{"title": "", "content": obj{"items": []obj{}}, "version": 3}, 422, nil)

	// Another teacher: not in the list, closed by a direct link, cannot save.
	list.Items = nil
	other.must("GET", "/api/boards", nil, 200, &list)
	if list.Total != 0 {
		t.Fatalf("other teacher sees boards: %+v", list)
	}
	other.must("GET", "/api/boards/"+id, nil, 404, nil)
	other.must("PUT", "/api/boards/"+id, obj{"content": obj{"items": []obj{}}, "version": 3, "force": true}, 404, nil)
	other.must("GET", "/api/boards/not-a-uuid", nil, 404, nil)
	// A student has no boards at all.
	student.must("GET", "/api/boards", nil, 403, nil)
	student.must("POST", "/api/boards", obj{"title": "x"}, 403, nil)
	student.must("GET", "/api/boards/"+id, nil, 403, nil)
	teacher.must("GET", "/api/boards/"+id, nil, 200, &again)
	if num(again, "version") != 3 {
		t.Fatalf("someone else changed the board: %v", again)
	}
}

// New board objects: stickers, rectangles, ellipses, arrows and pictures of this board; an old board (strokes and
// texts without ids) still loads and saves; another teacher can neither read nor upload nor reuse a picture.
func TestBoardObjectsAndImages(t *testing.T) {
	e := setup(t)
	teacher, other := e.login("teacher"), e.login("teacher2")
	var b, ob obj
	teacher.must("POST", "/api/boards", obj{"title": "Shapes"}, 201, &b)
	other.must("POST", "/api/boards", obj{"title": "Other"}, 201, &ob)
	id, oid := str(b, "id"), str(ob, "id")

	// An old board: items without ids (saved before ids existed) load and save unchanged.
	old := obj{"items": []obj{
		{"type": "stroke", "color": "#1F2937", "width": 4, "points": [][2]float64{{1, 1}, {2, 2}}},
		{"type": "text", "color": "#2F80ED", "x": 10, "y": 10, "size": 40, "text": "Old"},
	}}
	var got obj
	teacher.must("PUT", "/api/boards/"+id, obj{"content": old, "version": 1}, 200, &got)
	if _, has := get(got, "content", "items", 0).(map[string]any)["id"]; has || str(got, "content", "items", 1, "text") != "Old" {
		t.Fatalf("old board: %v", got["content"])
	}

	// A picture: PNG of this board; the owner reads it, nobody else does.
	var img obj
	upload(t, teacher, "/api/boards/"+id+"/images", "cat.png", testPNG(64, 32), 201, &img)
	if num(img, "width") != 64 || num(img, "height") != 32 {
		t.Fatalf("image: %v", img)
	}
	upload(t, teacher, "/api/boards/"+id+"/images", "note.txt", []byte("not a picture"), 422, nil)
	upload(t, other, "/api/boards/"+id+"/images", "cat.png", testPNG(64, 32), 404, nil)
	imgPath := "/api/boards/" + id + "/images/" + str(img, "id")
	if st, body := teacher.do("GET", imgPath, nil); st != 200 || len(body) == 0 {
		t.Fatalf("owner image: %d", st)
	}
	if st, _ := other.do("GET", imgPath, nil); st != 404 {
		t.Fatalf("other teacher reads the picture: %d", st)
	}
	if st, _ := other.do("GET", "/api/boards/"+oid+"/images/"+str(img, "id"), nil); st != 404 {
		t.Fatalf("picture through another board: %d", st)
	}

	all := obj{"items": []obj{
		{"id": "s1", "type": "stroke", "color": "#1F2937", "width": 4, "points": [][2]float64{{1, 1}, {50, 50}}},
		{"id": "t1", "type": "text", "color": "#2F80ED", "x": 100, "y": 100, "size": 40, "text": "Hello!"},
		{"id": "n1", "type": "sticky", "color": "#FFF1A8", "x": 200, "y": 200, "w": 220, "h": 160, "text": "My name is …"},
		{"id": "n2", "type": "sticky", "color": "#FFF1A8", "x": 500, "y": 200, "w": 220, "h": 160, "text": ""},
		{"id": "r1", "type": "rect", "color": "#E5484D", "width": 4, "x": -300, "y": 50, "w": 300, "h": 120},
		{"id": "e1", "type": "ellipse", "color": "#16A34A", "width": 4, "x": 700, "y": 600, "w": 200, "h": 100},
		{"id": "a1", "type": "arrow", "color": "#9254DE", "width": 4, "points": [][2]float64{{100, 900}, {400, 700}}},
		{"id": "i1", "type": "image", "image_id": str(img, "id"), "x": 1000, "y": 100, "w": 320, "h": 160, "color": "#123456", "text": "dropped"},
	}}
	teacher.must("PUT", "/api/boards/"+id, obj{"content": all, "version": 2}, 200, &got)
	teacher.must("GET", "/api/boards/"+id, nil, 200, &got)
	items := get(got, "content", "items").([]any)
	if len(items) != 8 || str(items[2], "text") != "My name is …" || str(items[3], "type") != "sticky" || num(items[4], "x") != -300 ||
		str(items[6], "type") != "arrow" || str(items[7], "image_id") != str(img, "id") || str(items[7], "color") != "" || str(items[7], "text") != "" {
		t.Fatalf("objects: %v", items)
	}

	bad := func(item obj) {
		t.Helper()
		teacher.must("PUT", "/api/boards/"+id, obj{"content": obj{"items": []obj{item}}, "version": 3}, 422, nil)
	}
	bad(obj{"type": "rect", "color": "#000000", "width": 4, "x": 0, "y": 0, "w": 0, "h": 10})
	bad(obj{"type": "ellipse", "color": "#000000", "width": 0, "x": 0, "y": 0, "w": 10, "h": 10})
	bad(obj{"type": "sticky", "color": "#000000", "x": 0, "y": 0, "w": 20, "h": 160})
	bad(obj{"type": "sticky", "color": "#000000", "x": 0, "y": 0, "w": 200, "h": 160, "text": strings.Repeat("a", 1001)})
	bad(obj{"type": "arrow", "color": "#000000", "width": 4, "points": [][2]float64{{1, 1}}})
	bad(obj{"type": "image", "image_id": "nope", "x": 0, "y": 0, "w": 10, "h": 10})
	bad(obj{"type": "image", "image_id": "00000000-0000-0000-0000-000000000001", "x": 0, "y": 0, "w": 10, "h": 10})
	bad(obj{"id": "bad id!", "type": "text", "color": "#000000", "x": 0, "y": 0, "size": 20, "text": "x"})
	teacher.must("PUT", "/api/boards/"+id, obj{"content": obj{"items": []obj{
		{"id": "x", "type": "text", "color": "#000000", "x": 0, "y": 0, "size": 20, "text": "a"},
		{"id": "x", "type": "text", "color": "#000000", "x": 0, "y": 0, "size": 20, "text": "b"},
	}}, "version": 3}, 422, nil)
	// Another teacher cannot put this board's picture on their own board.
	other.must("PUT", "/api/boards/"+oid, obj{"content": obj{"items": []obj{
		{"type": "image", "image_id": str(img, "id"), "x": 0, "y": 0, "w": 10, "h": 10},
	}}, "version": 1}, 422, nil)
}

