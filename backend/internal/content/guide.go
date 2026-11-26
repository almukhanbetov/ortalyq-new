package content

import (
	"fmt"
	"strings"

	"ortalyq/backend/internal/modules/exercises"
)

// GuideText is the teacher's note as plain text; the loader stores it as the lesson description.
func (l Lesson) GuideText() string {
	g := l.Guide
	var b strings.Builder
	fmt.Fprintf(&b, "Методичка для преподавателя\n\nЦель: %s\n\nДиалог:\n", g.Goal)
	for _, x := range g.Dialogue {
		fmt.Fprintf(&b, "%s: %s\n", x.Speaker, x.Text)
	}
	b.WriteString("\nПолезные фразы:\n")
	for _, p := range g.Phrases {
		fmt.Fprintf(&b, "• %s — %s\n", p.EN, p.RU)
	}
	b.WriteString("\nХод урока:\n")
	for _, s := range g.Steps {
		fmt.Fprintf(&b, "«%s». %s\n", s.Section, s.Text)
	}
	fmt.Fprintf(&b, "\nРазговорное задание («%s»): %s\nКарточка ученика:\n%s\n\nДомашняя работа: %s", SecTalk, g.Speaking, strings.Join(g.SpeakingCard, "\n"), g.Homework)
	return b.String()
}

// GuideMarkdown is the whole course as a teacher's handbook: guides and the answer keys of every exercise.
// docs/courses/english-a1-first-conversations.md is generated from it (see TestGuideFileUpToDate).
func (c Course) GuideMarkdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — методичка преподавателя\n\n", c.Title)
	b.WriteString("<!-- Generated from backend/internal/content (UPDATE_GUIDE=1 go test ./internal/content). Do not edit by hand. -->\n\n")
	fmt.Fprintf(&b, "%s\n\n", c.Description)
	b.WriteString("Курс загружается в «Материалы» командой `api seed-course`. Диалог и полезные фразы — текстовые блоки " +
		"раздела «Знакомство с темой», карточка разговорного задания — блок «Speaking task» в «Разговоре»: ученик видит их в классе. " +
		"Заметки и ключи — блоки «Заметки преподавателя», скрытые от ученика (сервер их ученику не отдаёт). " +
		"Полная методичка также хранится в описании урока (поле `description`).\n")
	for i, l := range c.Lessons {
		g := l.Guide
		fmt.Fprintf(&b, "\n## Урок %d. %s\n\n**Цель.** %s\n\n**Уровень** %s · **длительность** %d мин\n\n### Диалог\n\n",
			i+1, strings.TrimPrefix(l.Title, fmt.Sprintf("Lesson %d. ", i+1)), g.Goal, l.Attrs.Level, l.Attrs.DurationMin)
		for _, x := range g.Dialogue {
			fmt.Fprintf(&b, "> **%s:** %s  \n", x.Speaker, x.Text)
		}
		b.WriteString("\n### Полезные фразы\n\n| English | Русский |\n|---|---|\n")
		for _, p := range g.Phrases {
			fmt.Fprintf(&b, "| %s | %s |\n", p.EN, p.RU)
		}
		b.WriteString("\n### Ход урока\n\n")
		for _, s := range g.Steps {
			fmt.Fprintf(&b, "- **%s.** %s\n", s.Section, s.Text)
		}
		fmt.Fprintf(&b, "\n### Разговорное задание\n\n%s\n\nКарточка ученика (блок «Speaking task»):\n\n", g.Speaking)
		for _, x := range g.SpeakingCard {
			fmt.Fprintf(&b, "> %s  \n", x)
		}
		b.WriteString("\n### Упражнения и ключи\n\n")
		for _, s := range l.Sections {
			for _, e := range s.Exercises {
				writeKey(&b, s.Title, e)
			}
		}
		for _, e := range l.Homework {
			writeKey(&b, "Домашняя работа", e)
		}
		fmt.Fprintf(&b, "\n**Домашняя работа.** %s\n", g.Homework)
	}
	return b.String()
}

func writeKey(b *strings.Builder, where string, e Exercise) {
	kind := "тест"
	if e.Kind == exercises.KindGaps {
		kind = "слова и пропуски"
	}
	fmt.Fprintf(b, "- *%s* — %s «%s»", where, kind, e.Title)
	if e.Kind == exercises.KindGaps {
		q := e.Questions[0]
		fmt.Fprintf(b, ": `%s` → **%s**\n", q.Prompt, q.Options[0].Label)
		return
	}
	b.WriteString(":\n")
	for i, q := range e.Questions {
		for _, o := range q.Options {
			if o.IsCorrect {
				fmt.Fprintf(b, "  %d. %s → **%s**\n", i+1, q.Prompt, o.Label)
			}
		}
	}
}
