package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// uploadDocument posts a PDF document with its title (fields "file" and "title").
func uploadDocument(t *testing.T, c *client, title, name string, data []byte, want int) obj {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", title)
	fw, _ := w.CreateFormFile("file", name)
	fw.Write(data)
	w.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/api/documents", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", origin)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("upload %s: want %d got %d: %s", name, want, resp.StatusCode, b)
	}
	var out obj
	_ = json.Unmarshal(b, &out)
	return out
}

// «Личные материалы» → «Загрузить PDF»: only a real PDF of at most 20 MB is stored; the document is listed and
// found by the search of its owner; the owner opens the file inline; another teacher, a student and an anonymous
// request get no document and no file, also by a direct link.
func TestMaterialDocuments(t *testing.T) {
	e := setup(t)
	teacher, teacher2, student := e.login("teacher"), e.login("teacher2"), e.login("student")
	pdf := samplePDF("Worksheet one")

	// Refusals: another format (also renamed to .pdf), an empty or cut-off file, a missing title, a file above 20 MB.
	for _, bad := range []struct {
		name string
		data []byte
	}{
		{"photo.png", []byte("\x89PNG\r\n\x1a\n not a pdf")},
		{"notes.pdf", []byte("plain text renamed to pdf")},
		{"cut.pdf", pdf[:len(pdf)-40]},
	} {
		if out := uploadDocument(t, teacher, "Bad", bad.name, bad.data, 422); str(out, "error", "code") != "pdf_type" {
			t.Fatalf("%s: %v", bad.name, out)
		}
	}
	uploadDocument(t, teacher, "Empty", "empty.pdf", nil, 422)
	if out := uploadDocument(t, teacher, "  ", "a.pdf", pdf, 422); str(out, "error", "fields", "title") == "" {
		t.Fatalf("no title: %v", out)
	}
	big := append(append([]byte{}, pdf...), bytes.Repeat([]byte(" "), 20<<20)...)
	if out := uploadDocument(t, teacher, "Big", "big.pdf", big, 413); str(out, "error", "code") != "file_too_large" {
		t.Fatalf("big: %v", out)
	}
	uploadDocument(t, student, "Student", "s.pdf", pdf, 403)
	var list struct {
		Items []obj `json:"items"`
		Total int   `json:"total"`
	}
	teacher.must("GET", "/api/documents", nil, 200, &list)
	if list.Total != 0 {
		t.Fatalf("refused uploads were stored: %v", list.Items)
	}

	// Upload, list and search.
	doc := uploadDocument(t, teacher, "  Grammar worksheet ", "grammar sheet.pdf", pdf, 201)
	if str(doc, "title") != "Grammar worksheet" || str(doc, "file_name") != "grammar sheet.pdf" || doc["size_bytes"].(float64) != float64(len(pdf)) {
		t.Fatalf("document: %v", doc)
	}
	uploadDocument(t, teacher, "Reading", "reading.pdf", samplePDF("Reading"), 201)
	id := str(doc, "id")
	list.Items = nil
	teacher.must("GET", "/api/documents?q=GRAMMAR", nil, 200, &list)
	if list.Total != 1 || str(list.Items[0], "id") != id {
		t.Fatalf("search: %+v", list)
	}
	list.Items = nil
	teacher.must("GET", "/api/documents", nil, 200, &list)
	if list.Total != 2 || str(list.Items[0], "title") != "Reading" { // newest first
		t.Fatalf("list: %+v", list)
	}

	// The owner opens the file inline; the page of the web app may show it in a frame.
	resp, body := teacher.fetch("/api/documents/"+id+"/file", nil)
	if resp.StatusCode != 200 || !bytes.Equal(body, pdf) || resp.Header.Get("Content-Type") != "application/pdf" ||
		!strings.HasPrefix(resp.Header.Get("Content-Disposition"), "inline") || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors") {
		t.Fatalf("file: %d %v", resp.StatusCode, resp.Header)
	}
	teacher.must("GET", "/api/documents/"+id, nil, 200, nil)

	// Another teacher, a student and an anonymous request: no document, no file, nothing in the list.
	teacher2.must("GET", "/api/documents/"+id, nil, 404, nil)
	if resp, body := teacher2.fetch("/api/documents/"+id+"/file", nil); resp.StatusCode != 404 || bytes.Contains(body, []byte("%PDF")) {
		t.Fatalf("teacher2 file: %d", resp.StatusCode)
	}
	list.Items = nil
	teacher2.must("GET", "/api/documents?q=Grammar", nil, 200, &list)
	if list.Total != 0 {
		t.Fatalf("teacher2 sees: %+v", list)
	}
	if resp, _ := student.fetch("/api/documents/"+id+"/file", nil); resp.StatusCode != 403 {
		t.Fatalf("student file: %d", resp.StatusCode)
	}
	anon := &client{e: e, http: &http.Client{}, name: "anon"}
	if resp, _ := anon.fetch("/api/documents/"+id+"/file", nil); resp.StatusCode != 401 {
		t.Fatalf("anonymous file: %d", resp.StatusCode)
	}
	teacher.must("GET", "/api/documents/not-a-uuid", nil, 404, nil)
}
