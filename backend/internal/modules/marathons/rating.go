package marathons

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// RatingEntry is one participant in a rating (marathon or lesson). Emails are never part of it: other students
// see it.
type RatingEntry struct {
	StudentID   uuid.UUID `json:"student_id"`
	FullName    string    `json:"full_name"`
	AvatarColor string    `json:"avatar_color"`
	Points      int       `json:"points"`
	Place       int       `json:"place"`
	Me          bool      `json:"me"`
}

// Rank orders entries by points (descending), then by name, and gives places by standard competition ranking:
// equal points share a place and the next place skips (10, 10, 7 → 1, 1, 3). Pure and deterministic.
func Rank(entries []RatingEntry) []RatingEntry {
	out := append([]RatingEntry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		a, b := strings.ToLower(out[i].FullName), strings.ToLower(out[j].FullName)
		if a != b {
			return a < b
		}
		return out[i].StudentID.String() < out[j].StudentID.String()
	})
	for i := range out {
		if i > 0 && out[i].Points == out[i-1].Points {
			out[i].Place = out[i-1].Place
		} else {
			out[i].Place = i + 1
		}
	}
	return out
}

// VisibleRating is what a student sees: the places up to limit ("Отображать: Первых 3 / 5 / 10"; nil = everyone;
// ties at the border are all shown, as they share the place), with their own row marked. A student outside the
// shown places gets their own row separately only with showPlace ("Место ученика: Показывать"); with "Скрывать"
// they do not learn their place. me = uuid.Nil (teacher) marks nothing.
func VisibleRating(ranked []RatingEntry, limit *int, showPlace bool, me uuid.UUID) ([]RatingEntry, *RatingEntry) {
	top := []RatingEntry{}
	var own *RatingEntry
	for _, e := range ranked {
		e.Me = me != uuid.Nil && e.StudentID == me
		if limit == nil || e.Place <= *limit {
			top = append(top, e)
			continue
		}
		if e.Me && showPlace {
			x := e
			own = &x
		}
	}
	return top, own
}
