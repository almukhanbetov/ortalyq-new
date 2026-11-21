package router_test

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ortalyq/backend/internal/config"
)

func testPNG(w, h int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

// Stage 10: appearance. Covers of the marathon and its lessons and the custom background are attachments of the
// marathon class (only members open them); reactions and the background are saved with the settings; the lesson
// page receives the effective appearance (the lesson's own one when «Оформление урока» is on). Checked: rights,
// validation (types, sizes, ready values), replacing and deleting pictures (rows and files), the priority of the
// lesson appearance, isolation from another marathon with the same lesson and that students' data are untouched.
func TestMarathonAppearance(t *testing.T) {
	uploadDir := ""
	e := setup(t, func(c *config.Config) { uploadDir = c.UploadDir })
	ctx := t.Context()
	admin := e.login("admin")
	teacher := e.login("teacher")
	teacher2 := e.login("teacher2")
	student := e.login("student")
	student2 := e.login("student2")
	sid := e.assign(admin)

	var m obj
	teacher.must("POST", "/api/marathons", obj{"name": "Марафон с оформлением", "direction": "other"}, 201, &m)
	id := str(m, "id")
	base := "/api/marathons/" + id
	var course, l, l2, sec obj
	teacher.must("POST", "/api/courses", obj{"title": "Курс"}, 201, &course)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок 1"}, 201, &l)
	teacher.must("POST", "/api/courses/"+str(course, "id")+"/lessons", obj{"title": "Урок 2"}, 201, &l2)
	lid, lid2 := str(l, "id"), str(l2, "id")
	teacher.must("POST", "/api/lessons/"+lid+"/sections", obj{"title": "Раздел"}, 201, &sec)
	var ex obj
	teacher.must("POST", "/api/lessons/"+lid+"/exercises", obj{"scope": "lesson", "section_id": str(sec, "id"), "kind": "test", "title": "Тест",
		"settings":  obj{"check_mode": "instant", "scoring_enabled": true, "points_per_correct": 1},
		"questions": []obj{testQuestion("2+2", "4", "5")}}, 201, &ex)
	teacher.must("POST", base+"/lessons", obj{"lesson_id": lid}, 200, nil)
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid}}, 200, nil)

	// The student answers before any appearance exists; these data must stay as they are.
	var view obj
	student.must("POST", "/api/classrooms/"+id+"/exercises/"+str(ex, "id")+"/attempt", nil, 200, &view)
	q := get(view, "version", "questions", 0).(map[string]any)
	var right string
	for _, o := range q["options"].([]any) {
		if o.(map[string]any)["label"] == "4" {
			right = o.(map[string]any)["id"].(string)
		}
	}
	student.must("PUT", "/api/attempts/"+str(view, "attempt", "id")+"/answers/"+q["id"].(string), obj{"option_ids": []string{right}, "check": true}, 200, nil)
	snapshot := func() string {
		t.Helper()
		var s string
		if err := e.pool.QueryRow(ctx, `SELECT md5(string_agg(x::text, ',' ORDER BY x::text)) FROM (
				SELECT a::text AS x FROM attempts a UNION ALL SELECT aa::text FROM attempt_answers aa
				UNION ALL SELECT p::text FROM marathon_lesson_progress p UNION ALL SELECT en::text FROM marathon_enrollments en) z`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := snapshot()
	rows := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// ---- Rights ----
	for _, c := range []struct {
		who  *client
		want int
	}{{teacher2, 404}, {student, 403}, {admin, 403}} {
		c.who.must("GET", base+"/appearance", nil, c.want, nil)
		c.who.must("PUT", base+"/appearance", obj{"lesson_appearance": true}, c.want, nil)
		c.who.must("GET", base+"/lessons/"+lid+"/appearance", nil, c.want, nil)
		c.who.must("DELETE", base+"/images/cover", nil, c.want, nil)
		upload(t, c.who, base+"/images/cover", "a.png", testPNG(64, 64), c.want, nil)
		upload(t, c.who, base+"/lessons/"+lid+"/images/cover", "a.png", testPNG(64, 64), c.want, nil)
	}
	// The effective appearance: the owner and enrolled students; others do not see the marathon.
	teacher.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, nil)
	student2.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 404, nil)
	teacher2.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 404, nil)
	student.must("GET", base+"/lessons/"+lid2+"/appearance/effective", nil, 404, nil) // not a lesson of the marathon

	// ---- Defaults and validation ----
	var ap obj
	teacher.must("GET", base+"/appearance", nil, 200, &ap)
	if get(ap, "lesson_appearance") != false || get(ap, "appearance", "reactions_enabled") != false || str(ap, "appearance", "background") != "plain" ||
		len(get(ap, "appearance", "reactions", "correct_icon", "icons").([]any)) != 2 {
		t.Fatalf("defaults: %v", ap)
	}
	reactions := obj{
		"correct_icon":    obj{"enabled": true, "icons": []string{"fire", "brain"}},
		"incorrect_icon":  obj{"enabled": false, "icons": []string{}},
		"correct_sound":   obj{"enabled": true, "sound": 3},
		"incorrect_sound": obj{"enabled": true, "sound": 5},
	}
	req := func(patch obj) obj {
		r := obj{"lesson_appearance": false, "reactions_enabled": true, "reactions": reactions, "background_enabled": true, "background": "waves"}
		for k, v := range patch {
			r[k] = v
		}
		return r
	}
	bad := func(r obj, field string) {
		t.Helper()
		var er obj
		teacher.must("PUT", base+"/appearance", r, 422, &er)
		if _, ok := get(er, "error", "fields").(map[string]any)[field]; !ok {
			t.Fatalf("want field %s: %v", field, er)
		}
	}
	bad(obj{"reactions_enabled": false}, "lesson_appearance")
	bad(req(obj{"background": "space"}), "background")
	bad(req(obj{"background": "custom"}), "background")
	bad(req(obj{"reactions": obj{"correct_icon": obj{"enabled": true, "icons": []string{"sad"}}, "incorrect_icon": obj{"icons": []string{}},
		"correct_sound": obj{"sound": 1}, "incorrect_sound": obj{"sound": 1}}}), "reactions.correct_icon")
	bad(req(obj{"reactions": obj{"correct_icon": obj{"enabled": true, "icons": []string{}}, "incorrect_icon": obj{"icons": []string{}},
		"correct_sound": obj{"sound": 1}, "incorrect_sound": obj{"sound": 1}}}), "reactions.correct_icon")
	bad(req(obj{"reactions": obj{"correct_icon": obj{"icons": []string{}}, "incorrect_icon": obj{"icons": []string{}},
		"correct_sound": obj{"sound": 9}, "incorrect_sound": obj{"sound": 1}}}), "reactions.correct_sound")
	if n := rows(`SELECT count(*) FROM marathon_appearances`); n != 0 {
		t.Fatalf("refused requests created %d rows", n)
	}

	teacher.must("PUT", base+"/appearance", req(nil), 200, &ap)
	if get(ap, "appearance", "reactions_enabled") != true || str(ap, "appearance", "background") != "waves" ||
		num(ap, "appearance", "reactions", "incorrect_sound", "sound") != 5 {
		t.Fatalf("saved: %v", ap)
	}
	var eff obj
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "source") != "marathon" || str(eff, "background", "preset") != "waves" ||
		len(get(eff, "reactions", "correct_icon", "icons").([]any)) != 2 || len(get(eff, "reactions", "incorrect_icon", "icons").([]any)) != 0 {
		t.Fatalf("effective: %v", eff)
	}
	// Reactions off: the player receives none (nothing to play).
	teacher.must("PUT", base+"/appearance", req(obj{"reactions_enabled": false, "background_enabled": false}), 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if get(eff, "reactions") != nil || get(eff, "background") != nil {
		t.Fatalf("switched off: %v", eff)
	}

	// ---- Pictures: types and sizes ----
	refuse := func(path, name string, data []byte, want int, code string) {
		t.Helper()
		var er obj
		upload(t, teacher, path, name, data, want, &er)
		if str(er, "error", "code") != code {
			t.Fatalf("%s: want %s: %v", name, code, er)
		}
	}
	coverURL := base + "/images/cover"
	refuse(coverURL, "x.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), 422, "image_type")
	refuse(coverURL, "x.png", []byte("%PDF-1.4\n"+strings.Repeat("x", 200)), 422, "image_type") // name says png, content is pdf
	refuse(coverURL, "x.gif", append([]byte("GIF89a"), make([]byte, 60)...), 422, "image_type")
	refuse(coverURL, "x.png", testPNG(8, 8), 422, "image_size")
	refuse(coverURL, "x.png", append(testPNG(32, 32), make([]byte, 5<<20)...), 413, "file_too_large")
	upload(t, teacher, base+"/images/frame", "a.png", testPNG(64, 64), 404, nil) // unknown slot
	if n := rows(`SELECT count(*) FROM attachments`); n != 0 {
		t.Fatalf("refused pictures stored: %d", n)
	}

	// ---- Marathon cover: upload, replace, members only, delete ----
	var img1, img2 obj
	upload(t, teacher, coverURL, "cover.png", testPNG(120, 120), 200, &img1)
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if str(m, "cover_image_id") != str(img1, "image_id") {
		t.Fatalf("cover not set: %v", m)
	}
	var list obj
	student.must("GET", "/api/classrooms?type=marathon", nil, 200, &list)
	if !strings.Contains(string(mustJSON(list)), str(img1, "image_id")) {
		t.Fatal("student list without the cover")
	}
	served := func(c *client, att string) int {
		t.Helper()
		req, _ := http.NewRequest("GET", e.srv.URL+"/api/attachments/"+att+"?inline=1", nil)
		resp, err := c.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == 200 && resp.Header.Get("Content-Type") != "image/png" {
			t.Fatalf("content type %s", resp.Header.Get("Content-Type"))
		}
		return resp.StatusCode
	}
	if served(student, str(img1, "image_id")) != 200 || served(teacher, str(img1, "image_id")) != 200 ||
		served(teacher2, str(img1, "image_id")) != 404 || served(student2, str(img1, "image_id")) != 404 {
		t.Fatal("cover access")
	}
	// An appearance picture cannot be sent to the chat (and so never removed with a chat message).
	teacher.must("POST", "/api/classrooms/"+id+"/messages", obj{"client_id": "7a0c3d5e-2222-4222-8333-944455550000", "kind": "file",
		"attachment_id": str(img1, "image_id")}, 404, nil)
	var stored1 string
	if err := e.pool.QueryRow(ctx, `SELECT stored_name FROM attachments WHERE id = $1`, str(img1, "image_id")).Scan(&stored1); err != nil {
		t.Fatal(err)
	}
	upload(t, teacher, coverURL, "cover2.png", testPNG(200, 100), 200, &img2)
	if rows(`SELECT count(*) FROM attachments WHERE id = $1`, str(img1, "image_id")) != 0 || served(teacher, str(img1, "image_id")) != 404 {
		t.Fatal("replaced cover is kept")
	}
	if _, err := os.Stat(filepath.Join(uploadDir, stored1)); !os.IsNotExist(err) {
		t.Fatalf("replaced cover file is kept: %v", err)
	}
	teacher.must("DELETE", coverURL, nil, 204, nil)
	teacher.must("DELETE", coverURL, nil, 204, nil) // repeated: harmless
	teacher.must("GET", "/api/classrooms/"+id, nil, 200, &m)
	if get(m, "cover_image_id") != nil || rows(`SELECT count(*) FROM attachments`) != 0 {
		t.Fatalf("cover not removed: %v", m)
	}

	// ---- Lesson cover ----
	var limg obj
	upload(t, teacher, base+"/lessons/"+lid+"/images/cover", "lesson.png", testPNG(64, 64), 200, &limg)
	upload(t, teacher, base+"/lessons/"+lid2+"/images/cover", "lesson.png", testPNG(64, 64), 404, nil) // not in the marathon
	var ls obj
	student.must("GET", base+"/me", nil, 200, &ls)
	if str(ls, "items", 0, "cover_image_id") != str(limg, "image_id") {
		t.Fatalf("student lesson cover: %v", ls)
	}
	teacher.must("GET", base+"/lessons", nil, 200, &ls)
	if str(ls, "items", 0, "cover_image_id") != str(limg, "image_id") {
		t.Fatalf("teacher lesson cover: %v", ls)
	}

	// Concurrent uploads into one slot: one picture stays, the others are deleted (no orphan rows or files).
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			upload(t, teacher, base+"/lessons/"+lid+"/images/cover", "c.png", testPNG(40, 40), 200, nil)
		}()
	}
	wg.Wait()
	if n := rows(`SELECT count(*) FROM attachments WHERE classroom_id = $1`, id); n != 1 {
		t.Fatalf("after concurrent uploads %d pictures are stored", n)
	}
	if files, _ := os.ReadDir(uploadDir); len(files) != 1 {
		t.Fatalf("after concurrent uploads %d files are on disk", len(files))
	}
	teacher.must("GET", base+"/lessons", nil, 200, &ls)
	limg = obj{"image_id": str(ls, "items", 0, "cover_image_id")}

	// ---- Custom background: stored by the upload, selected by the save, falls back when deleted ----
	var bg obj
	upload(t, teacher, base+"/images/background", "bg.jpg", testPNG(800, 600), 200, &bg)
	teacher.must("GET", base+"/appearance", nil, 200, &ap)
	if str(ap, "appearance", "background_image_id") != str(bg, "image_id") || str(ap, "appearance", "background") != "waves" {
		t.Fatalf("upload must not select the background: %v", ap)
	}
	teacher.must("PUT", base+"/appearance", req(obj{"background": "custom"}), 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "background", "preset") != "custom" || str(eff, "background", "image_id") != str(bg, "image_id") {
		t.Fatalf("custom background: %v", eff)
	}
	if served(student, str(bg, "image_id")) != 200 {
		t.Fatal("student cannot open the background")
	}
	teacher.must("DELETE", base+"/images/background", nil, 204, nil)
	teacher.must("GET", base+"/appearance", nil, 200, &ap)
	if str(ap, "appearance", "background") != "plain" || get(ap, "appearance", "background_image_id") != nil {
		t.Fatalf("deleted custom background: %v", ap)
	}
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if get(eff, "background") != nil {
		t.Fatalf("deleted background still applied: %v", eff)
	}

	// ---- «Оформление урока»: the lesson's own appearance wins only when switched on in the marathon ----
	teacher.must("PUT", base+"/appearance", req(nil), 200, nil) // marathon: reactions on, waves
	lreq := obj{"enabled": true, "reactions_enabled": false, "reactions": reactions, "background_enabled": true, "background": "leaves"}
	teacher.must("PUT", base+"/lessons/"+lid+"/appearance", obj{"reactions_enabled": false}, 422, nil) // enabled missing
	teacher.must("PUT", base+"/lessons/"+lid2+"/appearance", lreq, 404, nil)
	var la obj
	teacher.must("PUT", base+"/lessons/"+lid+"/appearance", lreq, 200, &la)
	if get(la, "enabled") != true || str(la, "background") != "leaves" {
		t.Fatalf("lesson appearance: %v", la)
	}
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "source") != "marathon" || str(eff, "background", "preset") != "waves" {
		t.Fatalf("lesson appearance must wait for the switch: %v", eff)
	}
	teacher.must("PUT", base+"/appearance", req(obj{"lesson_appearance": true}), 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "source") != "lesson" || str(eff, "background", "preset") != "leaves" || get(eff, "reactions") != nil {
		t.Fatalf("lesson appearance wins as a whole: %v", eff)
	}
	// The lesson switches its own appearance off: the marathon's applies again.
	lreq["enabled"] = false
	teacher.must("PUT", base+"/lessons/"+lid+"/appearance", lreq, 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "source") != "marathon" || get(eff, "reactions") == nil {
		t.Fatalf("lesson own appearance off: %v", eff)
	}
	lreq["enabled"] = true
	teacher.must("PUT", base+"/lessons/"+lid+"/appearance", lreq, 200, nil)

	// Another marathon with the same lesson keeps its own (empty) appearance.
	var m2 obj
	teacher.must("POST", "/api/marathons", obj{"name": "Второй", "direction": "other"}, 201, &m2)
	base2 := "/api/marathons/" + str(m2, "id")
	teacher.must("POST", base2+"/lessons", obj{"lesson_id": lid}, 200, nil)
	teacher.must("PUT", base2+"/appearance", obj{"lesson_appearance": true, "reactions": reactions}, 200, nil)
	teacher.must("GET", base2+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "source") != "marathon" || get(eff, "reactions") != nil || get(eff, "background") != nil {
		t.Fatalf("appearance leaked to another marathon: %v", eff)
	}
	teacher.must("GET", base2+"/lessons", nil, 200, &ls)
	if get(ls, "items", 0, "cover_image_id") != nil {
		t.Fatalf("lesson cover leaked to another marathon: %v", ls)
	}

	// A removed lesson: its settings are not reachable; restoring it brings them back.
	teacher.must("DELETE", base+"/lessons/"+lid, nil, 200, nil)
	teacher.must("GET", base+"/lessons/"+lid+"/appearance", nil, 404, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 404, nil)
	teacher.must("POST", base+"/lessons", obj{"lesson_id": lid}, 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 200, &eff)
	if str(eff, "source") != "lesson" {
		t.Fatalf("restored lesson: %v", eff)
	}

	// Students' answers, attempts, enrollment and progress are untouched by the appearance.
	if after := snapshot(); after != before {
		t.Fatal("appearance changed students' data")
	}

	// A removed student loses the marathon and its pictures.
	teacher.must("DELETE", base+"/students/"+sid, nil, 200, nil)
	student.must("GET", base+"/lessons/"+lid+"/appearance/effective", nil, 404, nil)
	if served(student, str(limg, "image_id")) != 404 {
		t.Fatal("removed student still opens the lesson cover")
	}
	teacher.must("POST", base+"/students", obj{"student_ids": []string{sid}}, 200, nil)

}
