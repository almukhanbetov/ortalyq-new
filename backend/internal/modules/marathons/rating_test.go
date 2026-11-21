package marathons

import (
	"testing"

	"github.com/google/uuid"
)

func TestRankAndVisibleRating(t *testing.T) {
	ids := make([]uuid.UUID, 6)
	for i := range ids {
		ids[i] = uuid.New()
	}
	in := []RatingEntry{
		{StudentID: ids[0], FullName: "Борис", Points: 7},
		{StudentID: ids[1], FullName: "алия", Points: 10},
		{StudentID: ids[2], FullName: "Айгерим", Points: 10},
		{StudentID: ids[3], FullName: "Дана", Points: 0},
		{StudentID: ids[4], FullName: "Ерлан", Points: 3},
		{StudentID: ids[5], FullName: "Жанна", Points: 3},
	}
	r := Rank(in)
	got := []string{}
	places := []int{}
	for _, e := range r {
		got = append(got, e.FullName)
		places = append(places, e.Place)
	}
	want := []string{"Айгерим", "алия", "Борис", "Ерлан", "Жанна", "Дана"}
	wantPlaces := []int{1, 1, 3, 4, 4, 6}
	for i := range want {
		if got[i] != want[i] || places[i] != wantPlaces[i] {
			t.Fatalf("rank: %v %v", got, places)
		}
	}
	if in[0].Place != 0 {
		t.Fatal("Rank must not change its input")
	}

	three := 3
	top, own := VisibleRating(r, &three, false, ids[3])
	if len(top) != 3 || own != nil {
		t.Fatalf("top 3, place hidden: %v %v", top, own)
	}
	top, own = VisibleRating(r, &three, true, ids[3])
	if len(top) != 3 || own == nil || own.Place != 6 || !own.Me {
		t.Fatalf("top 3, place shown: %v %v", top, own)
	}
	// Ties at the border share the place and are all shown.
	five := 5
	top, own = VisibleRating(r, &five, true, ids[4])
	if len(top) != 5 || own != nil || !top[3].Me {
		t.Fatalf("top 5 with the student inside: %v %v", top, own)
	}
	top, _ = VisibleRating(r, &three, true, uuid.Nil)
	for _, e := range top {
		if e.Me {
			t.Fatal("teacher view marks nobody")
		}
	}
	all, own := VisibleRating(r, nil, false, ids[3])
	if len(all) != 6 || own != nil || !all[5].Me {
		t.Fatalf("everyone: %v", all)
	}
	if out, _ := VisibleRating(nil, nil, true, ids[0]); len(out) != 0 {
		t.Fatal("empty rating")
	}
}
