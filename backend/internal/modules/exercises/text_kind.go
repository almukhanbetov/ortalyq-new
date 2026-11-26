package exercises

import (
	"strings"
	"unicode/utf8"
)

// textKind is «Текст»: a block of text in a lesson section for reading and speaking practice (a dialogue, useful
// phrases, a speaking task). It has no answer field and no grade: the student never starts an attempt on it, and it
// is left out of points, progress and snapshots (see Gradable). The text is the prompt of its single question, so
// versions, hiding, ordering and realtime reuse the exercise tables. «Скрыть от ученика» (hidden) keeps a block for
// the teacher only, e.g. notes and answer keys.
type textKind struct{}

// TextMax is the longest text of one block (the question prompt column allows 2000 characters).
const TextMax = 2000

func (textKind) Validate(req SaveRequest) map[string]string {
	errs := map[string]string{}
	if len(req.Questions) != 1 {
		errs["questions"] = "Добавьте текст блока"
		return errs
	}
	q := req.Questions[0]
	switch body := strings.TrimSpace(q.Prompt); {
	case body == "":
		errs["questions.0.prompt"] = "Введите текст"
	case utf8.RuneCountInString(body) > TextMax:
		errs["questions.0.prompt"] = "Текст не длиннее 2000 символов"
	}
	if len(q.Options) != 0 || q.Multiple {
		errs["questions.0.options"] = "У текстового блока нет вариантов ответа"
	}
	if req.Settings.TimerEnabled || req.Settings.ScoringEnabled {
		errs["settings"] = "У текстового блока нет таймера и баллов"
	}
	return errs
}

func (textKind) Grade(Question, Response) AnswerResult { return Unanswered }

func (textKind) Sanitize(q Question) Question { return q }

// Gradable tells whether students answer this kind (attempts, points, progress). Text blocks are read only.
func Gradable(kind string) bool { return kind != KindText }

// OnlyGradable drops read-only blocks from a list of exercises.
func OnlyGradable(list []Exercise) []Exercise {
	out := list[:0:0]
	for _, e := range list {
		if Gradable(e.Kind) {
			out = append(out, e)
		}
	}
	return out
}
