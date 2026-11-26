// Package content holds ready-made author courses and loads them into a teacher's «Материалы»
// (`api seed-course`). A course is plain data in the existing structure course → lesson → section → exercise;
// the loader writes it with the same exercise rules as the editor and remembers what it created
// (content_imports), so a repeated load adds only what is missing and never overwrites the teacher's edits.
package content

import "ortalyq/backend/internal/modules/exercises"

// Course is one ready-made material.
type Course struct {
	Key         string // stable id of the content, never shown; lesson keys are Key + "/" + Lesson.Key
	Title       string
	Description string
	CoverColor  string
	Lessons     []Lesson
}

// Lesson is a template lesson: its filter attributes, the teacher's guide, three sections and homework.
type Lesson struct {
	Key   string
	Title string
	Attrs Attrs
	Guide Guide
	// Sections in order; each holds the exercises of the class part.
	Sections []Section
	// Homework exercises (scope homework): the student does them after the lesson and sends them for review.
	Homework []Exercise
}

type Attrs struct {
	Language, Age, Level, LessonType string
	DurationMin                      int
	Skills, Grammar, Vocabulary      []string
	Functions, Tags                  []string
}

// Guide is the teacher's methodical note («методичка»). The loader turns it into text blocks of the sections
// (see Blocks) and also stores it as the lesson description.
type Guide struct {
	Goal     string   // RU
	Dialogue []Line   // EN
	Phrases  []Phrase // EN + RU
	Steps    []Step   // RU: what to do in each section
	Speaking string   // RU: how the teacher runs the speaking task
	// SpeakingCard is the student's task card (EN), shown as the «Speaking task» text block of «Разговор».
	SpeakingCard []string
	Homework     string // RU
}

type Line struct{ Speaker, Text string }

type Phrase struct{ EN, RU string }

type Step struct{ Section, Text string }

type Section struct {
	Title     string
	Exercises []Exercise
}

// Exercise is the body of one exercise version, in the editor's request format.
type Exercise struct {
	Kind string
	exercises.SaveRequest
}

// Gap is one «Слова и пропуски» exercise: one phrase with one ___ and one accepted word.
func Gap(title, phrase, answer string, mode string) Exercise {
	return Exercise{Kind: exercises.KindGaps, SaveRequest: exercises.SaveRequest{
		Title:     title,
		Settings:  exercises.Settings{CheckMode: mode, PointsPerCorrect: 1},
		Questions: []exercises.QuestionInput{{Prompt: phrase, Options: []exercises.OptionInput{{Label: answer, IsCorrect: true}}}},
	}}
}

// Test is a single-choice test; each question has exactly one correct option.
func Test(title, mode string, qs ...exercises.QuestionInput) Exercise {
	return Exercise{Kind: exercises.KindTest, SaveRequest: exercises.SaveRequest{
		Title:     title,
		Settings:  exercises.Settings{CheckMode: mode, PointsPerCorrect: 1},
		Questions: qs,
	}}
}

// Q is a test question; correct is the index of the right option.
func Q(prompt string, correct int, options ...string) exercises.QuestionInput {
	q := exercises.QuestionInput{Prompt: prompt}
	for i, o := range options {
		q.Options = append(q.Options, exercises.OptionInput{Label: o, IsCorrect: i == correct})
	}
	return q
}
