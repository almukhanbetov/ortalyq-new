package exercises

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type testKind struct{}

func (testKind) Validate(req SaveRequest) map[string]string {
	errs := map[string]string{}
	if len(req.Questions) == 0 {
		errs["questions"] = "Добавьте хотя бы один вопрос"
	}
	if len(req.Questions) > 100 {
		errs["questions"] = "Не более 100 вопросов"
	}
	for i, q := range req.Questions {
		k := fmt.Sprintf("questions.%d", i)
		if strings.TrimSpace(q.Prompt) == "" {
			errs[k+".prompt"] = "Введите текст вопроса"
		}
		if len(q.Options) < 2 {
			errs[k+".options"] = "Нужно минимум два варианта ответа"
		}
		if len(q.Options) > 20 {
			errs[k+".options"] = "Не более 20 вариантов"
		}
		correct := 0
		for j, o := range q.Options {
			if strings.TrimSpace(o.Label) == "" {
				errs[fmt.Sprintf("%s.options.%d.label", k, j)] = "Введите текст варианта"
			}
			if o.IsCorrect {
				correct++
			}
		}
		if correct == 0 {
			errs[k+".correct"] = "Отметьте правильный ответ"
		} else if !q.Multiple && correct > 1 {
			errs[k+".correct"] = "Для одиночного выбора отметьте ровно один правильный ответ"
		}
	}
	return errs
}

// Grade: the selection must match the set of correct options exactly; a typed text is ignored.
func (testKind) Grade(q Question, r Response) AnswerResult {
	correct := map[uuid.UUID]bool{}
	for _, o := range q.Options {
		if o.IsCorrect != nil && *o.IsCorrect {
			correct[o.ID] = true
		}
	}
	return GradeQuestion(correct, r.Selected)
}

// Sanitize keeps the option labels (the student chooses among them) and drops is_correct.
func (testKind) Sanitize(q Question) Question {
	nq := q
	nq.Options = make([]Option, len(q.Options))
	for j, o := range q.Options {
		nq.Options[j] = Option{ID: o.ID, Label: o.Label}
	}
	return nq
}

// AnswerResult is the outcome of grading one question.
type AnswerResult int

const (
	Unanswered AnswerResult = iota
	Correct
	Wrong
)

// GradeQuestion compares a selection to the key: exact set match is correct.
func GradeQuestion(correct map[uuid.UUID]bool, selected []uuid.UUID) AnswerResult {
	if len(selected) == 0 {
		return Unanswered
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range selected {
		if !correct[id] {
			return Wrong
		}
		seen[id] = true
	}
	if len(seen) != len(correct) {
		return Wrong
	}
	return Correct
}
