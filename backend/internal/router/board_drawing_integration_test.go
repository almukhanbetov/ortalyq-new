package router_test

import (
	"testing"

	"github.com/google/uuid"
)

// Drawing together on the lesson board: a student draws only with the teacher's permission for the board open in the
// class; a stroke is its own action (a stale teacher save gets 409 and the editor merges); the same stroke twice is
// stored once; revoking, closing and replacing the board end the permission.
func TestBoardDrawing(t *testing.T) {
	e := setup(t)
	admin, teacher, student, student2 := e.login("admin"), e.login("teacher"), e.login("student"), e.login("student2")
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student"]}, 204, nil)
	admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids["student2"]}, 204, nil)
	var clsA, clsB, board, other obj
	teacher.must("POST", "/api/classrooms", obj{"name": "A", "student_id": e.ids["student"]}, 201, &clsA)
	teacher.must("POST", "/api/classrooms", obj{"name": "B", "student_id": e.ids["student2"]}, 201, &clsB)
	teacher.must("POST", "/api/boards", obj{"title": "Together"}, 201, &board)
	teacher.must("POST", "/api/boards", obj{"title": "Other"}, 201, &other)
	a, bid := "/api/classrooms/"+str(clsA, "id"), str(board, "id")
	stroke := func(client string, boardID string) obj {
		return obj{"board_id": boardID, "client_id": client, "color": "#E5484D", "width": 4, "points": [][2]float64{{10, 10}, {60, 40}}}
	}
	sid := e.ids["student"]

	// Nothing open: no permission can be given, no stroke accepted.
	teacher.must("PUT", a+"/board/drawers/"+sid, nil, 409, nil)
	teacher.must("PUT", a+"/board", obj{"board_id": bid}, 200, nil)

	// View only by default.
	var cb obj
	student.must("GET", a+"/board", nil, 200, &cb)
	if cb["can_draw"] != false || cb["drawers"] != nil {
		t.Fatalf("student by default: %v", cb)
	}
	cb = nil
	teacher.must("GET", a+"/board", nil, 200, &cb)
	if cb["can_draw"] != true || len(get(cb, "drawers").([]any)) != 0 {
		t.Fatalf("teacher: %v", cb)
	}
	c1 := uuid.NewString()
	student.must("POST", a+"/board/strokes", stroke(c1, bid), 403, nil)

	// Only the class teacher grants, only to a student of this class.
	student.must("PUT", a+"/board/drawers/"+sid, nil, 403, nil)
	teacher.must("PUT", a+"/board/drawers/"+e.ids["student2"], nil, 404, nil)
	teacher.must("PUT", a+"/board/drawers/"+e.ids["teacher"], nil, 422, nil)
	e.login("teacher2").must("PUT", a+"/board/drawers/"+sid, nil, 404, nil)
	cb = nil
	teacher.must("PUT", a+"/board/drawers/"+sid, nil, 200, &cb)
	if str(cb, "drawers", 0) != sid {
		t.Fatalf("drawers: %v", cb["drawers"])
	}
	cb = nil
	student.must("GET", a+"/board", nil, 200, &cb)
	if cb["can_draw"] != true {
		t.Fatalf("student allowed: %v", cb)
	}

	// The teacher's editor loaded version 1; the student's stroke lands first.
	var r1, r2 obj
	student.must("POST", a+"/board/strokes", stroke(c1, bid), 201, &r1)
	student.must("POST", a+"/board/strokes", stroke(c1, bid), 200, &r2) // resent: stored once
	if r2["duplicate"] != true || num(r1, "version") != 2 || num(r2, "version") != 2 || str(r1, "item_id") != "s-"+c1 {
		t.Fatalf("stroke: %v / %v", r1, r2)
	}
	teacherStroke := obj{"id": "t1", "type": "stroke", "color": "#1F2937", "width": 4, "points": [][2]float64{{100, 100}, {200, 150}}}
	teacher.must("PUT", "/api/boards/"+bid, obj{"version": 1, "content": obj{"items": []obj{teacherStroke}}}, 409, nil)
	// The editor merges what it has not seen and saves again: both strokes stay.
	var cur obj
	teacher.must("GET", "/api/boards/"+bid, nil, 200, &cur)
	merged := append([]any{teacherStroke}, get(cur, "content", "items").([]any)...)
	teacher.must("PUT", "/api/boards/"+bid, obj{"version": num(cur, "version"), "content": obj{"items": merged}}, 200, nil)
	cb = nil
	student.must("GET", a+"/board", nil, 200, &cb)
	items := get(cb, "board", "content", "items").([]any)
	if len(items) != 2 || str(items[0], "id") != "t1" || str(items[1], "id") != "s-"+c1 || str(items[1], "color") != "#E5484D" {
		t.Fatalf("both strokes: %v", items)
	}

	// Bad strokes; a stroke of another board; a student of another class.
	bad := stroke(uuid.NewString(), bid)
	bad["color"] = "red"
	student.must("POST", a+"/board/strokes", bad, 422, nil)
	student.must("POST", a+"/board/strokes", obj{"board_id": bid, "color": "#000000", "width": 4, "points": [][2]float64{{1, 1}}}, 422, nil)
	student.must("POST", a+"/board/strokes", stroke(uuid.NewString(), str(other, "id")), 409, nil)
	student2.must("POST", a+"/board/strokes", stroke(uuid.NewString(), bid), 404, nil)
	teacher.must("POST", a+"/board/strokes", stroke(uuid.NewString(), bid), 403, nil)

	// Revoking stops the next stroke at once.
	teacher.must("DELETE", a+"/board/drawers/"+sid, nil, 200, nil)
	student.must("POST", a+"/board/strokes", stroke(uuid.NewString(), bid), 403, nil)

	// Closing and opening again starts without permissions.
	teacher.must("PUT", a+"/board/drawers/"+sid, nil, 200, nil)
	teacher.must("DELETE", a+"/board", nil, 204, nil)
	student.must("POST", a+"/board/strokes", stroke(uuid.NewString(), bid), 409, nil)
	teacher.must("PUT", a+"/board", obj{"board_id": bid}, 200, nil)
	cb = nil
	student.must("GET", a+"/board", nil, 200, &cb)
	if cb["can_draw"] != false {
		t.Fatalf("permission survived closing: %v", cb)
	}
	student.must("POST", a+"/board/strokes", stroke(uuid.NewString(), bid), 403, nil)

	// Replacing the board with another one also ends it; a stroke for the old board is refused.
	teacher.must("PUT", a+"/board/drawers/"+sid, nil, 200, nil)
	teacher.must("PUT", a+"/board", obj{"board_id": str(other, "id")}, 200, nil)
	student.must("POST", a+"/board/strokes", stroke(uuid.NewString(), bid), 409, nil)
	student.must("POST", a+"/board/strokes", stroke(uuid.NewString(), str(other, "id")), 403, nil)
	cur = nil
	teacher.must("GET", "/api/boards/"+bid, nil, 200, &cur)
	if len(get(cur, "content", "items").([]any)) != 2 {
		t.Fatalf("refused strokes changed the board: %v", cur["content"])
	}
}
