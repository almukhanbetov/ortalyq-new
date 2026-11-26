package router_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// «Создать копию для занятия»: the copy has every object and its own picture files, it is what the class sees and
// draws on; the original does not change; the same request twice makes one copy; after closing, the copy stays in
// the teacher's boards and the class loses access.
func TestBoardCopyForLesson(t *testing.T) {
	e := setup(t)
	admin, teacher, student := e.login("admin"), e.login("teacher"), e.login("student")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	var cls, orig, img obj
	teacher.must("POST", "/api/classrooms", obj{"name": "Английский — Данияр", "student_id": e.ids["student"]}, 201, &cls)
	teacher.must("POST", "/api/boards", obj{"title": "Hello board"}, 201, &orig)
	a, oid := "/api/classrooms/"+str(cls, "id"), str(orig, "id")
	upload(t, teacher, "/api/boards/"+oid+"/images", "cat.png", testPNG(40, 20), 201, &img)
	teacher.must("PUT", "/api/boards/"+oid, obj{"version": 1, "content": obj{"items": []obj{
		{"id": "i1", "type": "image", "image_id": str(img, "id"), "x": 10, "y": 10, "w": 40, "h": 20},
		{"id": "s1", "type": "stroke", "color": "#1F2937", "width": 4, "points": [][2]float64{{1, 1}, {50, 50}}},
	}}}, 200, nil)

	teacher.must("PUT", a+"/board", obj{"board_id": oid, "copy": true}, 422, nil)
	req := uuid.NewString()
	var v1, v2 obj
	teacher.must("PUT", a+"/board", obj{"board_id": oid, "copy": true, "request_id": req}, 200, &v1)
	cid := str(v1, "copy", "id")
	want := "Hello board — Английский — Данияр — " + time.Now().In(mustLoc("Asia/Almaty")).Format("02.01.2006")
	if cid == "" || cid == oid || str(v1, "board", "id") != cid || str(v1, "copy", "title") != want {
		t.Fatalf("copy: %v", v1)
	}
	// The same request again: the same copy, nothing new, the class still on it.
	teacher.must("PUT", a+"/board", obj{"board_id": oid, "copy": true, "request_id": req}, 200, &v2)
	var list struct {
		Total int `json:"total"`
	}
	teacher.must("GET", "/api/boards", nil, 200, &list)
	if str(v2, "copy", "id") != cid || str(v2, "board", "id") != cid || list.Total != 2 {
		t.Fatalf("repeat: %v, boards %d", v2, list.Total)
	}

	// The copy has the objects; its picture is its own (new id), readable through the class; the original's is not.
	items := get(v1, "board", "content", "items").([]any)
	copyImg := str(items[0], "image_id")
	if len(items) != 2 || copyImg == "" || copyImg == str(img, "id") || str(items[1], "id") != "s1" {
		t.Fatalf("copy items: %v", items)
	}
	if st, body := student.do("GET", a+"/board/images/"+copyImg, nil); st != 200 || len(body) == 0 {
		t.Fatalf("copy picture through the class: %d", st)
	}
	if st, _ := student.do("GET", a+"/board/images/"+str(img, "id"), nil); st != 404 {
		t.Fatalf("original picture through the class: %d", st)
	}

	// Strokes of the lesson go to the copy only.
	teacher.must("PUT", a+"/board/drawers/"+e.ids["student"], nil, 200, nil)
	student.must("POST", a+"/board/strokes", obj{"board_id": cid, "client_id": uuid.NewString(), "color": "#E5484D", "width": 4,
		"points": [][2]float64{{100, 100}, {200, 200}}}, 201, nil)
	student.must("POST", a+"/board/strokes", obj{"board_id": oid, "client_id": uuid.NewString(), "color": "#E5484D", "width": 4,
		"points": [][2]float64{{100, 100}, {200, 200}}}, 409, nil)
	var cp obj
	teacher.must("GET", "/api/boards/"+cid, nil, 200, &cp)
	next := append(get(cp, "content", "items").([]any), obj{"id": "t2", "type": "stroke", "color": "#2F80ED", "width": 4, "points": [][2]float64{{300, 300}, {400, 350}}})
	teacher.must("PUT", "/api/boards/"+cid, obj{"version": num(cp, "version"), "content": obj{"items": next}}, 200, nil)

	// Closing: the class loses the copy, the copy stays the teacher's, the original never changed.
	teacher.must("DELETE", a+"/board", nil, 204, nil)
	if st, _ := student.do("GET", a+"/board/images/"+copyImg, nil); st != 404 {
		t.Fatalf("copy picture after closing: %d", st)
	}
	cp = nil
	teacher.must("GET", "/api/boards/"+cid, nil, 200, &cp)
	var o obj
	teacher.must("GET", "/api/boards/"+oid, nil, 200, &o)
	if n := len(get(cp, "content", "items").([]any)); n != 4 {
		t.Fatalf("copy after the lesson: %d items", n)
	}
	if n := len(get(o, "content", "items").([]any)); n != 2 || num(o, "version") != 2 {
		t.Fatalf("original changed: %v", o)
	}
	for _, path := range []string{"/api/boards/" + oid + "/images/" + str(img, "id"), "/api/boards/" + cid + "/images/" + copyImg} {
		if st, body := teacher.do("GET", path, nil); st != 200 || len(body) == 0 {
			t.Fatalf("picture %s: %d", path, st)
		}
	}
	// Another teacher cannot copy someone else's board into their class.
	e.login("teacher2").must("PUT", a+"/board", obj{"board_id": oid, "copy": true, "request_id": uuid.NewString()}, 404, nil)
	if strings.Contains(str(cp, "title"), "Hello board — Английский") == false {
		t.Fatalf("title: %v", cp["title"])
	}
}

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return l
}
