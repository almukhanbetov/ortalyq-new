package router_test

import (
	"testing"
)

// A personal board opened in a lesson: members of that class read it and its pictures (only this board's); a student
// of another class, a non-member teacher and the student's own writes are refused; closing ends the lesson access
// while the board stays the owner's.
func TestClassBoard(t *testing.T) {
	e := setup(t)
	admin, teacher, other, student, student2 := e.login("admin"), e.login("teacher"), e.login("teacher2"), e.login("student"), e.login("student2")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	var clsA, clsB obj
	teacher.must("POST", "/api/classrooms", obj{"name": "A", "student_id": e.ids["student"]}, 201, &clsA)
	teacher.must("POST", "/api/classrooms", obj{"name": "B", "student_id": e.ids["student2"]}, 201, &clsB)
	a := "/api/classrooms/" + str(clsA, "id")

	var board, spare, foreign, img, spareImg obj
	teacher.must("POST", "/api/boards", obj{"title": "English A1 — Hello!"}, 201, &board)
	teacher.must("POST", "/api/boards", obj{"title": "Spare"}, 201, &spare)
	other.must("POST", "/api/boards", obj{"title": "Not yours"}, 201, &foreign)
	bid := str(board, "id")
	upload(t, teacher, "/api/boards/"+bid+"/images", "cat.png", testPNG(40, 20), 201, &img)
	upload(t, teacher, "/api/boards/"+str(spare, "id")+"/images", "dog.png", testPNG(40, 20), 201, &spareImg)
	teacher.must("PUT", "/api/boards/"+bid, obj{"version": 1, "content": obj{"items": []obj{
		{"id": "s1", "type": "stroke", "color": "#1F2937", "width": 4, "points": [][2]float64{{1, 1}, {50, 50}}},
		{"id": "i1", "type": "image", "image_id": str(img, "id"), "x": 10, "y": 10, "w": 40, "h": 20},
	}}}, 200, nil)

	// Nothing is open yet.
	var cb obj
	student.must("GET", a+"/board", nil, 200, &cb)
	if cb["board"] != nil {
		t.Fatalf("board before opening: %v", cb)
	}
	// Only the teacher of the class opens, and only an own board.
	student.must("PUT", a+"/board", obj{"board_id": bid}, 403, nil)
	other.must("PUT", a+"/board", obj{"board_id": str(foreign, "id")}, 404, nil)
	teacher.must("PUT", a+"/board", obj{"board_id": str(foreign, "id")}, 404, nil)
	teacher.must("PUT", a+"/board", obj{"board_id": bid}, 200, nil)

	// The student of this class reads the board and its picture, not another board's picture.
	cb = nil
	student.must("GET", a+"/board", nil, 200, &cb)
	if str(cb, "board", "id") != bid || len(get(cb, "board", "content", "items").([]any)) != 2 || get(cb, "board").(map[string]any)["open_in"] != nil {
		t.Fatalf("student board: %v", cb)
	}
	if st, body := student.do("GET", a+"/board/images/"+str(img, "id"), nil); st != 200 || len(body) == 0 {
		t.Fatalf("student picture: %d", st)
	}
	if st, _ := student.do("GET", a+"/board/images/"+str(spareImg, "id"), nil); st != 404 {
		t.Fatalf("another board's picture through the lesson: %d", st)
	}
	// Still no direct access to the board, no writes.
	student.must("GET", "/api/boards/"+bid, nil, 403, nil)
	student.must("PUT", "/api/boards/"+bid, obj{"version": 2, "force": true, "content": obj{"items": []obj{}}}, 403, nil)
	student.must("DELETE", a+"/board", nil, 403, nil)
	// A student of another class and a non-member teacher see nothing.
	student2.must("GET", a+"/board", nil, 404, nil)
	if st, _ := student2.do("GET", a+"/board/images/"+str(img, "id"), nil); st != 404 {
		t.Fatalf("other class student reads the picture: %d", st)
	}
	other.must("GET", a+"/board", nil, 404, nil)
	other.must("DELETE", a+"/board", nil, 404, nil)
	cb = nil
	student2.must("GET", "/api/classrooms/"+str(clsB, "id")+"/board", nil, 200, &cb)
	if cb["board"] != nil {
		t.Fatalf("class B got the board: %v", cb)
	}

	// The owner sees where the board is open; a save is visible to the class at once.
	var own obj
	teacher.must("GET", "/api/boards/"+bid, nil, 200, &own)
	if str(own, "open_in", 0, "classroom_id") != str(clsA, "id") || str(own, "open_in", 0, "name") != "A" {
		t.Fatalf("open_in: %v", own["open_in"])
	}
	teacher.must("PUT", "/api/boards/"+bid, obj{"version": 2, "content": obj{"items": []obj{
		{"id": "t1", "type": "text", "color": "#2F80ED", "x": 5, "y": 5, "size": 40, "text": "Hello!"},
	}}}, 200, nil)
	cb = nil
	student.must("GET", a+"/board", nil, 200, &cb)
	if str(cb, "board", "content", "items", 0, "text") != "Hello!" {
		t.Fatalf("student after save: %v", cb)
	}

	// Closing ends the lesson access; the board stays the owner's.
	teacher.must("DELETE", a+"/board", nil, 204, nil)
	teacher.must("DELETE", a+"/board", nil, 204, nil) // idempotent
	cb = nil
	student.must("GET", a+"/board", nil, 200, &cb)
	if cb["board"] != nil {
		t.Fatalf("board after closing: %v", cb)
	}
	if st, _ := student.do("GET", a+"/board/images/"+str(img, "id"), nil); st != 404 {
		t.Fatalf("picture after closing: %d", st)
	}
	own = nil
	teacher.must("GET", "/api/boards/"+bid, nil, 200, &own)
	if own["open_in"] != nil || str(own, "content", "items", 0, "text") != "Hello!" {
		t.Fatalf("board after closing: %v", own)
	}
}
