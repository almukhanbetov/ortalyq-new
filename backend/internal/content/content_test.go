package content

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"ortalyq/backend/internal/modules/exercises"
)

// The author rules of «English A1 — Первые разговоры»: three lessons, each with the three sections, a dialogue and
// 5–7 phrases; in class one test of three questions and three one-gap exercises; two homework exercises.
func TestEnglishA1Rules(t *testing.T) {
	c := EnglishA1
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	if len(c.Lessons) != 3 {
		t.Fatalf("lessons: %d", len(c.Lessons))
	}
	cyrillic := regexp.MustCompile(`\p{Cyrillic}`)
	answers := map[string]bool{}
	for _, l := range c.Lessons {
		if len(l.Guide.Dialogue) < 4 || len(l.Guide.Phrases) < 5 || len(l.Guide.Phrases) > 7 || l.Guide.Speaking == "" || l.Guide.Goal == "" {
			t.Errorf("%s: guide incomplete", l.Title)
		}
		if len(l.Sections) != 3 || l.Sections[0].Title != SecIntro || l.Sections[1].Title != SecPractice || l.Sections[2].Title != SecTalk {
			t.Errorf("%s: sections %v", l.Title, l.Sections)
		}
		tests, gaps := 0, 0
		for _, s := range l.Sections {
			for _, e := range s.Exercises {
				switch e.Kind {
				case exercises.KindTest:
					tests++
					if len(e.Questions) != 3 {
						t.Errorf("%s: test has %d questions", l.Title, len(e.Questions))
					}
				case exercises.KindGaps:
					gaps++
				}
			}
		}
		if tests != 1 || gaps != 3 || len(l.Homework) != 2 {
			t.Errorf("%s: tests %d, gaps %d, homework %d", l.Title, tests, gaps, len(l.Homework))
		}
		all := append([]Exercise{}, l.Homework...)
		for _, s := range l.Sections {
			all = append(all, s.Exercises...)
		}
		for _, e := range all {
			// Tasks and options are in English; only the teacher's notes are in Russian.
			for _, q := range e.Questions {
				if cyrillic.MatchString(e.Title + q.Prompt) {
					t.Errorf("%s: not English: %q / %q", l.Title, e.Title, q.Prompt)
				}
				for _, o := range q.Options {
					if cyrillic.MatchString(o.Label) {
						t.Errorf("%s: option not English: %q", l.Title, o.Label)
					}
				}
			}
			if e.Kind == exercises.KindGaps {
				q := e.Questions[0]
				// One plain word: the server compares the whole answer, so no alternatives in one field.
				if strings.ContainsAny(q.Options[0].Label, " /,") {
					t.Errorf("%s: gap answer must be one word: %q", l.Title, q.Options[0].Label)
				}
				answers[q.Prompt] = true
			}
		}
	}
	if !answers["I ___ from Almaty."] {
		t.Error("the example gap «I ___ from Almaty.» → «am» is missing")
	}
}

// Text blocks: the dialogue and the phrases open «Знакомство с темой», the speaking card is in «Разговор» (the
// student sees them); the notes with the keys are teacher-only. Every block fits one text block.
func TestEnglishA1Blocks(t *testing.T) {
	cyrillic := regexp.MustCompile(`\p{Cyrillic}`)
	for _, l := range EnglishA1.Lessons {
		bs := l.Blocks()
		byKey := map[string]Block{}
		for _, b := range bs {
			byKey[b.Key] = b
			if n := len([]rune(b.Body)); n == 0 || n > exercises.TextMax {
				t.Errorf("%s / %s: %d runes", l.Title, b.Key, n)
			}
		}
		d, p, s := byKey["dialogue"], byKey["phrases"], byKey["speaking"]
		if d.Section != SecIntro || !d.AtTop || d.TeacherOnly || p.Section != SecIntro || !p.AtTop || p.TeacherOnly {
			t.Errorf("%s: dialogue/phrases placement %+v %+v", l.Title, d, p)
		}
		if s.Section != SecTalk || s.TeacherOnly || cyrillic.MatchString(s.Body+d.Body) {
			t.Errorf("%s: speaking card must be a visible English block in «Разговор»", l.Title)
		}
		for _, k := range []string{"notes-intro", "notes-practice", "notes-talk"} {
			if !byKey[k].TeacherOnly || !strings.Contains(byKey[k].Body, "→") {
				t.Errorf("%s: %s must be teacher-only and hold keys", l.Title, k)
			}
		}
		// Every answer key of the lesson is in a teacher-only block and in no visible block.
		for _, e := range append(append([]Exercise{}, l.Homework...), allExercises(l)...) {
			for _, q := range e.Questions {
				line := q.Prompt + " → "
				found := false
				for _, b := range bs {
					if strings.Contains(b.Body, line) {
						if !b.TeacherOnly {
							t.Errorf("%s: key of %q in a visible block", l.Title, q.Prompt)
						}
						found = true
					}
				}
				if !found {
					t.Errorf("%s: key of %q is missing in the notes", l.Title, q.Prompt)
				}
			}
		}
	}
}

func allExercises(l Lesson) []Exercise {
	var out []Exercise
	for _, s := range l.Sections {
		out = append(out, s.Exercises...)
	}
	return out
}

func TestGuideTextFitsDescription(t *testing.T) {
	for _, l := range EnglishA1.Lessons {
		g := l.GuideText()
		if n := len([]rune(g)); n > 5000 {
			t.Errorf("%s: guide %d runes", l.Title, n)
		}
		if !strings.Contains(g, l.Guide.Dialogue[0].Text) || !strings.Contains(g, "Разговорное задание") {
			t.Errorf("%s: guide misses the dialogue or the speaking task", l.Title)
		}
	}
}

// The teacher's handbook in docs is generated from the data. Regenerate: UPDATE_GUIDE=1 go test ./internal/content
func TestGuideFileUpToDate(t *testing.T) {
	const path = "../../../docs/courses/english-a1-first-conversations.md"
	want := EnglishA1.GuideMarkdown()
	if os.Getenv("UPDATE_GUIDE") == "1" {
		if err := os.MkdirAll("../../../docs/courses", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("docs/courses/english-a1-first-conversations.md is stale: run UPDATE_GUIDE=1 go test ./internal/content")
	}
}

func TestValidateRejectsBadContent(t *testing.T) {
	bad := Course{Key: "x", Title: "X", Lessons: []Lesson{{Key: "a", Title: "A", Sections: []Section{{Title: "S", Exercises: []Exercise{
		Gap("Two gaps", "I ___ a ___.", "am", instant),
		Test("Two correct", instant, exercises.QuestionInput{Prompt: "?", Options: []exercises.OptionInput{{Label: "a", IsCorrect: true}, {Label: "b", IsCorrect: true}}}),
	}}}}}}
	err := Validate(bad)
	if err == nil || !strings.Contains(err.Error(), "Two gaps") || !strings.Contains(err.Error(), "Two correct") {
		t.Fatalf("want both errors, got %v", err)
	}
}
