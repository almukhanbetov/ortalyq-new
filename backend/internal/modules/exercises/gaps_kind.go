package exercises

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// gapsKind is «Слова и пропуски», first step: one phrase with exactly one gap («I ___ a student.») and the accepted
// word («am»). The phrase is the question prompt, the word is its single option with is_correct, so versions,
// copies and statistics reuse the test tables. The student types the word; the server compares it to the key
// ignoring letter case and spaces around it.
type gapsKind struct{}

// GapMarker matches the gap in a phrase: three or more underscores.
var GapMarker = regexp.MustCompile(`_{3,}`)

const gapAnswerMax = 100

func (gapsKind) Validate(req SaveRequest) map[string]string {
	errs := map[string]string{}
	if len(req.Questions) != 1 {
		errs["questions"] = "Добавьте одну фразу с пропуском"
		return errs
	}
	q := req.Questions[0]
	prompt := strings.TrimSpace(q.Prompt)
	switch n := len(GapMarker.FindAllStringIndex(prompt, -1)); {
	case prompt == "":
		errs["questions.0.prompt"] = "Введите фразу с пропуском"
	case utf8.RuneCountInString(prompt) > 2000:
		errs["questions.0.prompt"] = "Фраза не длиннее 2000 символов"
	case n == 0:
		errs["questions.0.prompt"] = "Отметьте пропуск тремя подчёркиваниями: ___"
	case n > 1:
		errs["questions.0.prompt"] = "На этом этапе во фразе может быть только один пропуск"
	}
	if q.Multiple {
		errs["questions.0.multiple"] = "Пропуск принимает одно слово"
	}
	if len(q.Options) != 1 {
		errs["questions.0.options"] = "Укажите один правильный ответ"
		return errs
	}
	answer := strings.TrimSpace(q.Options[0].Label)
	switch {
	case answer == "":
		errs["questions.0.options.0.label"] = "Введите правильный ответ"
	case utf8.RuneCountInString(answer) > gapAnswerMax:
		errs["questions.0.options.0.label"] = "Ответ не длиннее 100 символов"
	case GapMarker.MatchString(answer):
		errs["questions.0.options.0.label"] = "Ответ не может содержать пропуск"
	}
	if !q.Options[0].IsCorrect {
		errs["questions.0.options.0.label"] = "Ответ должен быть отмечен правильным"
	}
	return errs
}

// NormalizeGapAnswer is how a typed word and the key are compared: no spaces around, any letter case.
func NormalizeGapAnswer(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (gapsKind) Grade(q Question, r Response) AnswerResult {
	given := NormalizeGapAnswer(r.Text)
	if given == "" {
		return Unanswered
	}
	for _, o := range q.Options {
		if o.IsCorrect != nil && *o.IsCorrect && NormalizeGapAnswer(o.Label) == given {
			return Correct
		}
	}
	return Wrong
}

// Sanitize removes the options entirely: their label is the answer itself.
func (gapsKind) Sanitize(q Question) Question {
	nq := q
	nq.Options = []Option{}
	return nq
}

// CorrectTexts returns the accepted words of a gap question (shown only when the answer is revealed).
func CorrectTexts(q Question) []string {
	out := []string{}
	for _, o := range q.Options {
		if o.IsCorrect != nil && *o.IsCorrect {
			out = append(out, o.Label)
		}
	}
	return out
}
