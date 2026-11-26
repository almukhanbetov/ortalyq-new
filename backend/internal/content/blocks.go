package content

import (
	"fmt"
	"strings"

	"ortalyq/backend/internal/modules/exercises"
)

// Block is a «Текст» block of a lesson section, made from the lesson guide.
type Block struct {
	Key         string // unique within the lesson; content_imports key is <course>/<lesson>/block/<Key>
	Section     string // section title
	Title       string
	Body        string
	TeacherOnly bool // hidden from the student: notes and answer keys
	AtTop       bool // before the section's exercises (otherwise after them)
}

// Request is the block in the editor's request format.
func (b Block) Request() exercises.SaveRequest {
	hidden := b.TeacherOnly
	return exercises.SaveRequest{
		Title:     b.Title,
		Settings:  exercises.Settings{CheckMode: exercises.CheckInstant, PointsPerCorrect: 1},
		Questions: []exercises.QuestionInput{{Prompt: b.Body}},
		Hidden:    &hidden,
	}
}

// Blocks are the text blocks of the lesson, in the order they go into each section:
// «Знакомство с темой» — the dialogue and the useful phrases first, then the teacher's notes;
// «Практика» — the teacher's notes with the keys; «Разговор» — the student's speaking task card and the teacher's
// notes with the speaking instructions and the keys of the test and the homework.
func (l Lesson) Blocks() []Block {
	g := l.Guide
	if len(g.Dialogue) == 0 {
		return nil // a lesson without a guide has no text blocks
	}
	var dialogue, phrases []string
	for _, x := range g.Dialogue {
		dialogue = append(dialogue, x.Speaker+": "+x.Text)
	}
	for _, p := range g.Phrases {
		phrases = append(phrases, "• "+p.EN+" — "+p.RU)
	}
	step := map[string]string{}
	for _, s := range g.Steps {
		step[s.Section] = s.Text
	}
	keys := func(list []Exercise) string {
		var b strings.Builder
		for _, e := range list {
			writePlainKey(&b, e)
		}
		return b.String()
	}
	section := func(title string) []Exercise {
		for _, s := range l.Sections {
			if s.Title == title {
				return s.Exercises
			}
		}
		return nil
	}
	const notes = "Заметки преподавателя"
	return []Block{
		{Key: "dialogue", Section: SecIntro, Title: "Dialogue", Body: strings.Join(dialogue, "\n"), AtTop: true},
		{Key: "phrases", Section: SecIntro, Title: "Useful phrases", Body: strings.Join(phrases, "\n"), AtTop: true},
		{Key: "notes-intro", Section: SecIntro, Title: notes, TeacherOnly: true,
			Body: fmt.Sprintf("Цель урока: %s\n\nХод раздела: %s\n\nКлючи:\n%s", g.Goal, step[SecIntro], keys(section(SecIntro)))},
		{Key: "notes-practice", Section: SecPractice, Title: notes, TeacherOnly: true,
			Body: fmt.Sprintf("Ход раздела: %s\n\nКлючи:\n%s", step[SecPractice], keys(section(SecPractice)))},
		{Key: "speaking", Section: SecTalk, Title: "Speaking task", Body: strings.Join(g.SpeakingCard, "\n")},
		{Key: "notes-talk", Section: SecTalk, Title: notes, TeacherOnly: true,
			Body: fmt.Sprintf("Ход раздела: %s\n\nРазговорное задание: %s\n\nКлючи:\n%s\nДомашняя работа: %s\nКлючи домашней работы:\n%s",
				step[SecTalk], g.Speaking, keys(section(SecTalk)), g.Homework, keys(l.Homework))},
	}
}

// writePlainKey writes the answer key of one exercise as plain text lines.
func writePlainKey(b *strings.Builder, e Exercise) {
	if e.Kind == exercises.KindGaps {
		q := e.Questions[0]
		fmt.Fprintf(b, "• %s: %s → %s\n", e.Title, q.Prompt, q.Options[0].Label)
		return
	}
	fmt.Fprintf(b, "• %s:\n", e.Title)
	for i, q := range e.Questions {
		for _, o := range q.Options {
			if o.IsCorrect {
				fmt.Fprintf(b, "  %d. %s → %s\n", i+1, q.Prompt, o.Label)
			}
		}
	}
}
