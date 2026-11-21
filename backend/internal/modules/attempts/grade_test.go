package attempts

import (
	"testing"

	"github.com/google/uuid"

	"ortalyq/backend/internal/modules/exercises"
)

func bp(b bool) *bool { return &b }

func version(scoring bool, multiple bool) (*exercises.Version, []uuid.UUID, []uuid.UUID) {
	q1, q2 := uuid.New(), uuid.New()
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	v := &exercises.Version{Settings: exercises.Settings{ScoringEnabled: scoring, PointsPerCorrect: 2},
		Questions: []exercises.Question{
			{ID: q1, Options: []exercises.Option{{ID: a, IsCorrect: bp(true)}, {ID: b, IsCorrect: bp(false)}}},
			{ID: q2, Multiple: multiple, Options: []exercises.Option{{ID: c, IsCorrect: bp(true)}, {ID: d, IsCorrect: bp(multiple)}}},
		}}
	return v, []uuid.UUID{q1, q2}, []uuid.UUID{a, b, c, d}
}

func TestGradeCountsAndPoints(t *testing.T) {
	v, q, o := version(true, false)
	s := Grade(v, map[uuid.UUID]Answer{q[0]: {Selected: []uuid.UUID{o[0]}}}, false)
	if s.Correct != 1 || s.Wrong != 0 || s.Unanswered != 1 || s.PointsEarned != 2 || s.PointsMax != 4 {
		t.Fatalf("%+v", s)
	}
	// Recomputing gives the same result: no accumulation.
	if Grade(v, map[uuid.UUID]Answer{q[0]: {Selected: []uuid.UUID{o[0]}}}, false) != s {
		t.Fatal("not deterministic")
	}
}

func TestGradeOnlyChecked(t *testing.T) {
	v, q, o := version(true, false)
	s := Grade(v, map[uuid.UUID]Answer{q[0]: {Selected: []uuid.UUID{o[0]}, Checked: false}}, true)
	if s.Correct != 0 || s.PointsEarned != 0 {
		t.Fatalf("unchecked answers must not score in instant mode: %+v", s)
	}
}

func TestGradeMultipleRequiresExactSet(t *testing.T) {
	v, q, o := version(false, true)
	partial := Grade(v, map[uuid.UUID]Answer{q[1]: {Selected: []uuid.UUID{o[2]}}}, false)
	full := Grade(v, map[uuid.UUID]Answer{q[1]: {Selected: []uuid.UUID{o[2], o[3]}}}, false)
	if partial.Wrong != 1 || full.Correct != 1 || full.PointsMax != 0 {
		t.Fatalf("partial=%+v full=%+v", partial, full)
	}
}

func TestSanitizedHidesKey(t *testing.T) {
	v, _, _ := version(true, false)
	s := v.Sanitized()
	for _, q := range s.Questions {
		for _, o := range q.Options {
			if o.IsCorrect != nil {
				t.Fatal("key leaked")
			}
		}
	}
	if v.Questions[0].Options[0].IsCorrect == nil {
		t.Fatal("original mutated")
	}
}
