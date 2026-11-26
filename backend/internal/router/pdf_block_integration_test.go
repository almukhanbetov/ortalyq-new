package router_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// samplePDF builds a small valid one-page PDF with a line of text (a correct cross-reference table).
func samplePDF(text string) []byte {
	stream := fmt.Sprintf("BT /F1 24 Tf 72 720 Td (%s) Tj ET", text)
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// fetch performs a GET with extra headers and returns the response with its body read.
func (c *client) fetch(path string, headers map[string]string) (*http.Response, []byte) {
	c.e.t.Helper()
	req, _ := http.NewRequest("GET", c.e.srv.URL+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

// «PDF-документ» through the API: upload checks (real PDF, at most 20 MB, the lesson owner only), a block in a
// section, viewing by the owner and by the student of the class where the lesson is open, replacing the file,
// refusals for a stranger, another class, a hidden block and a direct link; a copy of the lesson in a marathon
// opens the same PDF with the copy's own rules.
func TestPDFBlock(t *testing.T) {
	e := setup(t)
	admin, teacher, teacher2 := e.login("admin"), e.login("teacher"), e.login("teacher2")
	student, student2 := e.login("student"), e.login("student2")
	for _, s := range []string{"student", "student2"} {
		admin.must("POST", "/api/admin/assignments", obj{"teacher_id": e.ids["teacher"], "student_id": e.ids[s]}, 204, nil)
	}
	var clsA, clsB, course, lesson, sec obj
	teacher.must("POST", "/api/classrooms", obj{"name": "Class A", "student_id": e.ids["student"]}, 201, &clsA)
	teacher.must("POST", "/api/classrooms", obj{"name": "Class B", "student_id": e.ids["student2"]}, 201, &clsB)
	teacher.must("POST", "/api/courses", obj{"title": "English"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Reading"}, 201, &lesson)
	lid, cA, cB := str(lesson, "id"), str(clsA, "id"), str(clsB, "id")
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Read"}, 201, &sec)
	files := "/api/lessons/" + lid + "/files"

	// ---- Upload: only a real PDF, at most 20 MB, only into the owner's lesson ----
	pdf1, pdf2 := samplePDF("First worksheet"), samplePDF("Second worksheet")
	upload(t, teacher, files, "photo.pdf", []byte("\x89PNG\r\n\x1a\n not a pdf at all"), 422, nil)
	upload(t, teacher, files, "notes.pdf", []byte("plain text renamed to pdf"), 422, nil)
	upload(t, teacher, files, "cut.pdf", pdf1[:len(pdf1)-40], 422, nil) // no %%EOF: a cut-off download
	upload(t, teacher, files, "page.pdf", []byte("<html><body>%PDF-1.4</body></html>"), 422, nil)
	upload(t, teacher, files, "empty.pdf", nil, 422, nil)
	big := append(append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte{' '}, 20<<20)...), []byte("\n%%EOF\n")...)
	upload(t, teacher, files, "big.pdf", big, 413, nil)
	upload(t, student, files, "a.pdf", pdf1, 403, nil)
	upload(t, teacher2, files, "a.pdf", pdf1, 404, nil)
	var f1, f2, foreign obj
	upload(t, teacher, files, "Рабочий лист 1.pdf", pdf1, 201, &f1)
	if str(f1, "name") != "Рабочий лист 1.pdf" || num(f1, "size_bytes") != len(pdf1) || get(f1, "stored_name") != nil {
		t.Fatalf("uploaded file: %v", f1)
	}
	edge := append(append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{' '}, 20<<20-16)...), []byte("\n%%EOF\n")...)
	if len(edge) != 20<<20 {
		t.Fatalf("edge size %d", len(edge))
	}
	upload(t, teacher, files, "edge.pdf", edge, 201, nil) // exactly 20 MB is allowed

	// ---- The block ----
	block := func(title string, fileID any) obj {
		return obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "pdf", "title": title, "file_id": fileID,
			"settings": obj{"check_mode": "instant"}, "questions": []obj{}}
	}
	path := "/api/lessons/" + lid + "/exercises"
	teacher.must("POST", path, block("No file", nil), 422, nil)
	teacher.must("POST", path, block(" ", str(f1, "id")), 422, nil)
	teacher.must("POST", path, block("Unknown", e.ids["teacher"]), 422, nil)
	var otherCourse, otherLesson obj
	teacher2.must("POST", "/api/courses", obj{"title": "Other"}, 201, &otherCourse)
	teacher2.must("POST", "/api/courses/"+str(otherCourse, "id")+"/lessons", obj{"title": "Other"}, 201, &otherLesson)
	upload(t, teacher2, "/api/lessons/"+str(otherLesson, "id")+"/files", "theirs.pdf", pdf2, 201, &foreign)
	teacher.must("POST", path, block("Foreign file", str(foreign, "id")), 422, nil) // another teacher's upload
	hw := block("Homework", str(f1, "id"))
	hw["scope"], hw["section_id"] = "homework", nil
	teacher.must("POST", path, hw, 422, nil)
	scored := block("Scored", str(f1, "id"))
	scored["settings"] = obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1}
	teacher.must("POST", path, scored, 422, nil)

	var ex obj
	teacher.must("POST", path, block("Рабочий лист", str(f1, "id")), 201, &ex)
	eid := str(ex, "id")
	if str(ex, "kind") != "pdf" || str(ex, "version", "file", "id") != str(f1, "id") || str(ex, "version", "file", "name") != "Рабочий лист 1.pdf" {
		t.Fatalf("block: %v", ex)
	}
	teacher.must("POST", "/api/exercises/"+eid+"/copy-to-homework", nil, 409, nil)

	// ---- The owner views it in the editor ----
	resp, body := teacher.fetch("/api/exercises/"+eid+"/file", nil)
	if resp.StatusCode != 200 || !bytes.Equal(body, pdf1) || resp.Header.Get("Content-Type") != "application/pdf" ||
		!strings.HasPrefix(resp.Header.Get("Content-Disposition"), "inline;") || resp.Header.Get("X-Frame-Options") != "" ||
		!strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'self' "+origin) ||
		strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.HasPrefix(resp.Header.Get("Cache-Control"), "private") {
		t.Fatalf("teacher file: %d %v", resp.StatusCode, resp.Header)
	}
	resp, body = teacher.fetch("/api/exercises/"+eid+"/file", map[string]string{"Range": "bytes=0-4"})
	if resp.StatusCode != 206 || string(body) != "%PDF-" {
		t.Fatalf("range: %d %q", resp.StatusCode, body)
	}
	teacher2.must("GET", "/api/exercises/"+eid+"/file", nil, 404, nil)
	student.must("GET", "/api/exercises/"+eid+"/file", nil, 403, nil)

	// ---- The class: before the lesson is open nobody gets the file ----
	classFile := func(cid string) string { return "/api/classrooms/" + cid + "/exercises/" + eid + "/file" }
	student.must("GET", classFile(cA), nil, 404, nil)
	teacher.must("PUT", "/api/classrooms/"+cA+"/current-lesson", obj{"lesson_id": lid}, 200, nil)
	raw := student.must("GET", "/api/classrooms/"+cA+"/lessons/"+lid+"/exercises", nil, 200, nil)
	if !strings.Contains(string(raw), `"kind":"pdf"`) || !strings.Contains(string(raw), "Рабочий лист 1.pdf") {
		t.Fatalf("student list: %s", raw)
	}
	resp, body = student.fetch(classFile(cA), nil)
	if resp.StatusCode != 200 || !bytes.Equal(body, pdf1) || resp.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("student file: %d", resp.StatusCode)
	}
	if st, _ := teacher.do("GET", classFile(cA), nil); st != 200 {
		t.Fatalf("teacher in class: %d", st)
	}
	student.must("POST", "/api/classrooms/"+cA+"/exercises/"+eid+"/attempt", nil, 409, nil) // read only

	// A student of another class: not by the class of the lesson, not through their own class, not by the editor.
	student2.must("GET", classFile(cA), nil, 404, nil)
	student2.must("GET", classFile(cB), nil, 404, nil)
	student2.must("GET", "/api/exercises/"+eid+"/file", nil, 403, nil)
	student2.must("GET", "/api/classrooms/"+cA+"/exercises/"+eid, nil, 404, nil)
	anon := &client{e: e, http: &http.Client{}, name: "anon"}
	anon.must("GET", classFile(cA), nil, 401, nil)
	// The file is not reachable by its own id through other file routes.
	student.must("GET", "/api/attachments/"+str(f1, "id"), nil, 404, nil)

	// Hidden from the student: the block and its file disappear for the student only.
	teacher.must("PATCH", "/api/exercises/"+eid+"/hidden", obj{"hidden": true}, 204, nil)
	student.must("GET", classFile(cA), nil, 404, nil)
	teacher.must("GET", classFile(cA), nil, 200, nil)
	teacher.must("PATCH", "/api/exercises/"+eid+"/hidden", obj{"hidden": false}, 204, nil)

	// ---- Replace the file: a new version; the student gets the new PDF ----
	upload(t, teacher, files, "Рабочий лист 2.pdf", pdf2, 201, &f2)
	var edited obj
	teacher.must("PUT", "/api/exercises/"+eid, obj{"title": "Рабочий лист", "file_id": str(f2, "id"), "settings": obj{"check_mode": "instant"}, "questions": []obj{}}, 200, &edited)
	if num(edited, "version", "number") != 2 || str(edited, "version", "file", "name") != "Рабочий лист 2.pdf" {
		t.Fatalf("replace: %v", edited)
	}
	_, body = student.fetch(classFile(cA), nil)
	if !bytes.Equal(body, pdf2) {
		t.Fatal("student still gets the old file")
	}
	// Saving again without a change keeps the file.
	teacher.must("PUT", "/api/exercises/"+eid, obj{"title": "Рабочий лист (новый)", "file_id": str(f2, "id"), "settings": obj{"check_mode": "instant"}}, 200, &edited)
	if str(edited, "version", "file", "id") != str(f2, "id") {
		t.Fatalf("resave: %v", edited)
	}

	// Not graded: no exercises to count in the lesson.
	var pr obj
	teacher.must("GET", "/api/classrooms/"+cA+"/lessons/"+lid+"/progress?student_id="+e.ids["student"], nil, 200, &pr)
	if num(pr, "lesson_points", "exercises") != 0 {
		t.Fatalf("pdf counted as an exercise: %v", pr["lesson_points"])
	}

	// ---- Delete the block: the file goes with it for everyone ----
	var tmp obj
	teacher.must("POST", path, block("Temp", str(f1, "id")), 201, &tmp)
	teacher.must("DELETE", "/api/exercises/"+str(tmp, "id"), nil, 204, nil)
	teacher.must("GET", "/api/exercises/"+str(tmp, "id")+"/file", nil, 404, nil)
	student.must("GET", "/api/classrooms/"+cA+"/exercises/"+str(tmp, "id")+"/file", nil, 404, nil)

	// ---- A copy of the lesson in a marathon opens the same PDF with the copy's rules ----
	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон", "direction": "other"}, 201, &m)
	mid := str(m, "id")
	teacher.must("POST", "/api/marathons/"+mid+"/lessons", obj{"lesson_id": lid}, 200, nil)
	teacher.must("POST", "/api/marathons/"+mid+"/students", obj{"student_ids": []string{e.ids["student"]}}, 200, nil)
	var copied, exs obj
	teacher.must("POST", "/api/marathons/"+mid+"/lessons/"+lid+"/copy", nil, 201, &copied)
	copyLesson := str(copied, "lesson_id")
	teacher.must("GET", "/api/lessons/"+copyLesson+"/exercises?scope=lesson", nil, 200, &exs)
	cex := str(exs, "items", 0, "id")
	if str(exs, "items", 0, "kind") != "pdf" || cex == eid || str(exs, "items", 0, "version", "file", "id") != str(f2, "id") {
		t.Fatalf("copied block: %v", exs)
	}
	_, body = teacher.fetch("/api/exercises/"+cex+"/file", nil)
	if !bytes.Equal(body, pdf2) {
		t.Fatal("copy: teacher file")
	}
	resp, body = student.fetch("/api/classrooms/"+mid+"/exercises/"+cex+"/file", nil)
	if resp.StatusCode != 200 || !bytes.Equal(body, pdf2) {
		t.Fatalf("copy: student file %d", resp.StatusCode)
	}
	student2.must("GET", "/api/classrooms/"+mid+"/exercises/"+cex+"/file", nil, 404, nil)
	teacher2.must("GET", "/api/exercises/"+cex+"/file", nil, 404, nil)
	// The copy keeps its file on save; the source lesson's block is not touched.
	teacher.must("PUT", "/api/exercises/"+cex, obj{"title": "Копия", "file_id": str(f2, "id"), "settings": obj{"check_mode": "instant"}}, 200, nil)
	var src obj
	teacher.must("GET", "/api/exercises/"+eid, nil, 200, &src)
	if str(src, "version", "title") != "Рабочий лист (новый)" {
		t.Fatalf("source changed: %v", src)
	}
	// The copied block only opens through the class of the copy, not through another class with the source lesson.
	student.must("GET", "/api/classrooms/"+cA+"/exercises/"+cex+"/file", nil, 404, nil)
}
