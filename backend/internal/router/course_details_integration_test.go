package router_test

import (
	"bytes"
	"net/http"
	"testing"
)

// «Создать материал» through the API: the cover picture (only a real picture, only the uploader attaches and reads
// it), the language from the project list, the tags and the «Разделы» switch. With «Разделы» off a new lesson gets
// one section; on — the lesson starts empty and the teacher adds named sections (the existing structure).
func TestCourseDetails(t *testing.T) {
	e := setup(t)
	teacher, teacher2 := e.login("teacher"), e.login("teacher2")

	// Cover: a picture is stored; a PDF or text is refused; another teacher cannot read or attach it.
	upload(t, teacher, "/api/course-covers", "cover.pdf", samplePDF("not a picture"), 422, nil)
	upload(t, teacher, "/api/course-covers", "cover.png", []byte("text renamed"), 422, nil)
	pic := testPNG(64, 40)
	var cover obj
	upload(t, teacher, "/api/course-covers", "cover.png", pic, 201, &cover)
	cid := str(cover, "id")
	if resp, body := teacher.fetch("/api/course-covers/"+cid, nil); resp.StatusCode != 200 || !bytes.Equal(body, pic) || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("owner cover: %d %v", resp.StatusCode, resp.Header)
	}
	if resp, _ := teacher2.fetch("/api/course-covers/"+cid, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("teacher2 reads the cover: %d", resp.StatusCode)
	}
	teacher2.must("POST", "/api/courses", obj{"title": "Stolen", "cover_image_id": cid}, 422, nil)

	// Validation: an unknown language, too many tags.
	teacher.must("POST", "/api/courses", obj{"title": "X", "language": "xx"}, 422, nil)
	many := make([]string, 21)
	for i := range many {
		many[i] = string(rune('a'+i)) + "tag"
	}
	teacher.must("POST", "/api/courses", obj{"title": "X", "tags": many}, 422, nil)

	// Create with every field of the form; the values come back on a repeated read.
	var c obj
	teacher.must("POST", "/api/courses", obj{"title": "  Grammar A2 ", "description": "Tenses", "cover_image_id": cid,
		"language": "en", "sections_enabled": false, "tags": []string{"grammar", " A2 ", "grammar", ""}}, 201, &c)
	id := str(c, "id")
	var got obj
	teacher.must("GET", "/api/courses/"+id, nil, 200, &got)
	tags, _ := got["tags"].([]any)
	if str(got, "title") != "Grammar A2" || str(got, "cover_image_id") != cid || str(got, "language") != "en" ||
		got["sections_enabled"] != false || len(tags) != 2 || tags[0] != "grammar" || tags[1] != "A2" {
		t.Fatalf("course: %v", got)
	}

	// «Разделы» off: a new lesson has one section and the lesson says so.
	var l1 obj
	teacher.must("POST", "/api/courses/"+id+"/lessons", obj{"title": "Lesson 1"}, 201, &l1)
	var lesson obj
	teacher.must("GET", "/api/lessons/"+str(l1, "id"), nil, 200, &lesson)
	if secs, _ := lesson["sections"].([]any); len(secs) != 1 || lesson["sections_enabled"] != false {
		t.Fatalf("lesson without sections: %v", lesson)
	}

	// Update: switch «Разделы» on, change tags and language, remove the cover. A new lesson starts without sections.
	teacher.must("PATCH", "/api/courses/"+id, obj{"sections_enabled": true, "tags": []string{"B1"}, "language": "", "cover_image_id": ""}, 200, &got)
	if got["sections_enabled"] != true || got["cover_image_id"] != nil || str(got, "language") != "" || len(got["tags"].([]any)) != 1 {
		t.Fatalf("updated: %v", got)
	}
	var l2 obj
	teacher.must("POST", "/api/courses/"+id+"/lessons", obj{"title": "Lesson 2"}, 201, &l2)
	lesson = nil
	teacher.must("GET", "/api/lessons/"+str(l2, "id"), nil, 200, &lesson)
	if secs, _ := lesson["sections"].([]any); len(secs) != 0 || lesson["sections_enabled"] != true {
		t.Fatalf("lesson with sections: %v", lesson)
	}
	// A PATCH without the new fields keeps them.
	teacher.must("PATCH", "/api/courses/"+id, obj{"title": "Grammar B1"}, 200, &got)
	if got["sections_enabled"] != true || len(got["tags"].([]any)) != 1 {
		t.Fatalf("patch title only: %v", got)
	}
	// A course created without the field (older clients, loaders) keeps the structure with sections.
	var old obj
	teacher.must("POST", "/api/courses", obj{"title": "Old client"}, 201, &old)
	if old["sections_enabled"] != true || len(old["tags"].([]any)) != 0 {
		t.Fatalf("default course: %v", old)
	}
	teacher2.must("PATCH", "/api/courses/"+id, obj{"title": "Mine"}, 404, nil)
}
