package exercises

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func gapReq(prompt, answer string) SaveRequest {
	return SaveRequest{Title: "Gap", Questions: []QuestionInput{{Prompt: prompt, Options: []OptionInput{{Label: answer, IsCorrect: true}}}}}
}

func TestGapsValidate(t *testing.T) {
	k := gapsKind{}
	if errs := k.Validate(gapReq("I ___ a student.", "am")); len(errs) != 0 {
		t.Fatalf("valid phrase rejected: %v", errs)
	}
	cases := map[string]SaveRequest{
		"no gap":       gapReq("I am a student.", "am"),
		"two gaps":     gapReq("I ___ a ___.", "am"),
		"empty prompt": gapReq("   ", "am"),
		"empty answer": gapReq("I ___ a student.", "  "),
		"long answer":  gapReq("I ___ a student.", strings.Repeat("a", 101)),
		"gap answer":   gapReq("I ___ a student.", "a___"),
		"no questions": {Title: "Gap"},
		"two answers":  {Questions: []QuestionInput{{Prompt: "I ___", Options: []OptionInput{{Label: "am", IsCorrect: true}, {Label: "is", IsCorrect: true}}}}},
		"not correct":  {Questions: []QuestionInput{{Prompt: "I ___", Options: []OptionInput{{Label: "am"}}}}},
	}
	for name, req := range cases {
		if errs := k.Validate(req); len(errs) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestGapsGradeAndSanitize(t *testing.T) {
	yes := true
	q := Question{ID: uuid.New(), Prompt: "I ___ a student.", Options: []Option{{ID: uuid.New(), Label: "am", IsCorrect: &yes}}}
	v := &Version{Kind: KindGaps, Questions: []Question{q}}
	for text, want := range map[string]AnswerResult{
		"am": Correct, "  AM ": Correct, "Am\t": Correct,
		"is": Wrong, "a m": Wrong, "am.": Wrong,
		"": Unanswered, "   ": Unanswered,
	} {
		if got := v.Grade(q, Response{Text: text}); got != want {
			t.Errorf("%q: got %v want %v", text, got, want)
		}
	}
	s := v.Sanitized()
	if len(s.Questions[0].Options) != 0 || s.Questions[0].Prompt != q.Prompt {
		t.Fatalf("key not hidden: %+v", s.Questions[0])
	}
	if len(v.Questions[0].Options) != 1 {
		t.Fatal("sanitize changed the original")
	}
	// A test version keeps its labels and is graded by options as before.
	tv := &Version{Kind: KindTest, Questions: []Question{q}}
	if got := tv.Sanitized().Questions[0].Options; len(got) != 1 || got[0].IsCorrect != nil {
		t.Fatalf("test sanitize: %+v", got)
	}
	if tv.Grade(q, Response{Selected: []uuid.UUID{q.Options[0].ID}}) != Correct || tv.Grade(q, Response{Text: "am"}) != Unanswered {
		t.Fatal("test grading changed")
	}
}
