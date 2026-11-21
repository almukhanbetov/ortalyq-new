package exercises

import "testing"

func TestValidateTest(t *testing.T) {
	ok := SaveRequest{Questions: []QuestionInput{{Prompt: "She ___ a teacher.", Options: []OptionInput{{Label: "is", IsCorrect: true}, {Label: "are"}}}}}
	if errs := (testKind{}).Validate(ok); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
	cases := map[string]SaveRequest{
		"no questions":   {},
		"empty prompt":   {Questions: []QuestionInput{{Prompt: " ", Options: ok.Questions[0].Options}}},
		"one option":     {Questions: []QuestionInput{{Prompt: "Q", Options: []OptionInput{{Label: "a", IsCorrect: true}}}}},
		"no correct":     {Questions: []QuestionInput{{Prompt: "Q", Options: []OptionInput{{Label: "a"}, {Label: "b"}}}}},
		"two for single": {Questions: []QuestionInput{{Prompt: "Q", Options: []OptionInput{{Label: "a", IsCorrect: true}, {Label: "b", IsCorrect: true}}}}},
		"empty option":   {Questions: []QuestionInput{{Prompt: "Q", Options: []OptionInput{{Label: "a", IsCorrect: true}, {Label: ""}}}}},
	}
	for name, req := range cases {
		if errs := (testKind{}).Validate(req); len(errs) == 0 {
			t.Errorf("%s: expected validation error", name)
		}
	}
	multi := SaveRequest{Questions: []QuestionInput{{Prompt: "Q", Multiple: true, Options: []OptionInput{{Label: "a", IsCorrect: true}, {Label: "b", IsCorrect: true}}}}}
	if errs := (testKind{}).Validate(multi); len(errs) != 0 {
		t.Fatalf("multiple correct allowed for multi-choice: %v", errs)
	}
}
