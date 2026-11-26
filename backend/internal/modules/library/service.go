package library

import (
	"context"
	"strings"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/content"
	"ortalyq/backend/internal/modules/auth"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func teaching(p auth.Principal) bool {
	for _, r := range auth.TeachingRoles {
		if p.Role == r {
			return true
		}
	}
	return false
}

// List returns the catalog courses whose title contains q (any letter case), with the teacher's copies.
func (s *Service) List(ctx context.Context, p auth.Principal, q string) (*ListResponse, error) {
	if !teaching(p) {
		return nil, apperr.Forbidden()
	}
	q = strings.ToLower(strings.TrimSpace(q))
	var found []content.Course
	for _, c := range content.Catalog {
		if q == "" || strings.Contains(strings.ToLower(c.Title), q) {
			found = append(found, c)
		}
	}
	items, err := s.views(ctx, p, found)
	if err != nil {
		return nil, err
	}
	return &ListResponse{Items: items, Total: len(items)}, nil
}

// Get returns one catalog course.
func (s *Service) Get(ctx context.Context, p auth.Principal, key string) (*CatalogCourse, error) {
	if !teaching(p) {
		return nil, apperr.Forbidden()
	}
	c, ok := content.Find(key)
	if !ok {
		return nil, apperr.NotFound("Курс каталога")
	}
	items, err := s.views(ctx, p, []content.Course{c})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// Add is «Добавить в мои материалы»: an own, editable copy for the caller. Adding again returns the same copy.
func (s *Service) Add(ctx context.Context, p auth.Principal, key string) (*AddResponse, error) {
	if !teaching(p) {
		return nil, apperr.Forbidden()
	}
	c, ok := content.Find(key)
	if !ok {
		return nil, apperr.NotFound("Курс каталога")
	}
	r, err := s.repo.Add(ctx, p.UserID, c)
	if err != nil {
		return nil, err
	}
	return &AddResponse{CourseID: r.CourseID, Created: r.CourseCreated}, nil
}

func (s *Service) views(ctx context.Context, p auth.Principal, list []content.Course) ([]CatalogCourse, error) {
	keys := make([]string, len(list))
	for i, c := range list {
		keys[i] = c.Key
	}
	owned, err := s.repo.Owned(ctx, p.UserID, keys)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogCourse, 0, len(list))
	for _, c := range list {
		v := CatalogCourse{Key: c.Key, Title: c.Title, Description: c.Description, CoverColor: c.CoverColor,
			LessonCount: len(c.Lessons), Lessons: []CatalogLesson{}}
		if id, ok := owned[c.Key]; ok {
			v.OwnedCourseID = &id
		}
		for _, l := range c.Lessons {
			if v.Level == "" {
				v.Level, v.Language = l.Attrs.Level, l.Attrs.Language
			}
			cl := CatalogLesson{Title: l.Title, Level: l.Attrs.Level, DurationMin: l.Attrs.DurationMin, Sections: []string{}, Homework: len(l.Homework)}
			for _, sec := range l.Sections {
				cl.Sections = append(cl.Sections, sec.Title)
				cl.Exercises += len(sec.Exercises)
			}
			for _, b := range l.Blocks() {
				if !b.TeacherOnly {
					cl.TextBlocks++
				}
			}
			v.Lessons = append(v.Lessons, cl)
		}
		out = append(out, v)
	}
	return out, nil
}
