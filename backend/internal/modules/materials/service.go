package materials

import (
	"context"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/languages"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

// ChangeNotifier is told when a lesson template changes so open classrooms refresh.
type ChangeNotifier interface {
	LessonChanged(ctx context.Context, lessonID uuid.UUID)
}

// FileStore checks, writes and reads cover pictures (uploads.Service: the same upload directory as other files).
type FileStore interface {
	StoreImage(size int64, r io.Reader) (*uploads.StoredImage, error)
	OpenStored(name string) (*os.File, error)
	RemoveStored(name string)
}

type Service struct {
	repo   *Repository
	notify ChangeNotifier
	files  FileStore
}

// SetFiles connects the file store (created after this service in the router).
func (s *Service) SetFiles(f FileStore) { s.files = f }

func NewService(repo *Repository, notify ChangeNotifier) *Service {
	return &Service{repo: repo, notify: notify}
}

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func cleanTitle(v apperr.V, field string, s *string) {
	if s == nil {
		return
	}
	*s = strings.TrimSpace(*s)
	v.Check(*s != "" && len([]rune(*s)) <= 200, field, "Название должно содержать от 1 до 200 символов")
}

// OwnedLesson returns the lesson ref when p owns it (teachers only).
func (s *Service) OwnedLesson(ctx context.Context, p auth.Principal, lessonID uuid.UUID) (*LessonRef, error) {
	ref, err := s.repo.LessonRef(ctx, lessonID)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Урок")
		}
		return nil, err
	}
	if !p.CanTeach() || ref.OwnerID != p.UserID {
		return nil, apperr.NotFound("Урок")
	}
	return ref, nil
}

// LessonRef returns a lesson ref without access checks (callers check access).
func (s *Service) LessonRef(ctx context.Context, lessonID uuid.UUID) (*LessonRef, error) {
	ref, err := s.repo.LessonRef(ctx, lessonID)
	if database.IsNoRows(err) {
		return nil, apperr.NotFound("Урок")
	}
	return ref, err
}

func (s *Service) Sections(ctx context.Context, lessonID uuid.UUID) ([]Section, error) {
	return s.repo.Sections(ctx, lessonID)
}

// SectionLesson returns the lesson that owns a section.
func (s *Service) SectionLesson(ctx context.Context, sectionID uuid.UUID) (uuid.UUID, error) {
	id, err := s.repo.SectionLesson(ctx, sectionID)
	if database.IsNoRows(err) {
		return uuid.Nil, apperr.NotFound("Раздел")
	}
	return id, err
}

func (s *Service) ownedCourse(ctx context.Context, p auth.Principal, id uuid.UUID) (*Course, error) {
	c, err := s.repo.GetCourse(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Курс")
		}
		return nil, err
	}
	if c.OwnerID != p.UserID {
		return nil, apperr.NotFound("Курс")
	}
	return c, nil
}

func (s *Service) ListCourses(ctx context.Context, p auth.Principal, f LessonFilter, limit, offset int) ([]Course, int, error) {
	return s.repo.ListCourses(ctx, p.UserID, f, limit, offset)
}

func (s *Service) GetCourse(ctx context.Context, p auth.Principal, id uuid.UUID, f LessonFilter) (*Course, error) {
	c, err := s.ownedCourse(ctx, p, id)
	if err != nil {
		return nil, err
	}
	// A course-title match lists all lessons of that course.
	if f.Query != "" && strings.Contains(strings.ToLower(c.Title), strings.ToLower(f.Query)) {
		f.Query = ""
	}
	c.Lessons, err = s.repo.ListLessons(ctx, id, f)
	return c, err
}

// details checks the fields of the material form shared by create and update: the cover picture (the teacher's
// own upload), the language (the project list), the tags. It returns the cover id to set (nil: unchanged) and
// whether the picture is removed.
func (s *Service) details(ctx context.Context, p auth.Principal, v apperr.V, req *CourseRequest) (cover *uuid.UUID, clear bool, err error) {
	if req.Language != nil {
		l := strings.TrimSpace(*req.Language)
		req.Language = &l
		v.Check(l == "" || languages.Known(l), "language", "Выберите язык из списка")
	}
	if req.Tags != nil {
		tags := normList(*req.Tags)
		for _, t := range tags {
			if len([]rune(t)) > TagMax {
				v.Add("tags", "Тег не длиннее 40 символов")
			}
		}
		v.Check(len(tags) <= MaxTags, "tags", "Не больше 20 тегов")
		req.Tags = &tags
	}
	if req.CoverImageID != nil {
		if *req.CoverImageID == "" {
			return nil, true, nil
		}
		id, perr := uuid.Parse(*req.CoverImageID)
		if perr != nil {
			v.Add("cover_image_id", "Загрузите обложку ещё раз")
			return nil, false, nil
		}
		c, gerr := s.repo.Cover(ctx, id)
		if gerr != nil && !database.IsNoRows(gerr) {
			return nil, false, gerr
		}
		if c == nil || c.OwnerID != p.UserID {
			v.Add("cover_image_id", "Загрузите обложку ещё раз")
			return nil, false, nil
		}
		return &id, false, nil
	}
	return nil, false, nil
}

func (s *Service) CreateCourse(ctx context.Context, p auth.Principal, req CourseRequest) (*Course, error) {
	v := apperr.V{}
	if req.Title == nil {
		v.Add("title", "Укажите название")
	}
	cleanTitle(v, "title", req.Title)
	n := NewCourse{Color: "#1AC6FF", Tags: []string{}, SectionsEnabled: true}
	if req.Description != nil {
		n.Description = strings.TrimSpace(*req.Description)
		v.Check(len([]rune(n.Description)) <= 5000, "description", "Слишком длинное описание")
	}
	if req.CoverColor != nil {
		v.Check(colorRe.MatchString(*req.CoverColor), "cover_color", "Некорректный цвет")
		n.Color = *req.CoverColor
	}
	cover, _, err := s.details(ctx, p, v, &req)
	if err != nil {
		return nil, err
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	n.Title, n.CoverImageID = *req.Title, cover
	if req.Language != nil {
		n.Language = *req.Language
	}
	if req.Tags != nil {
		n.Tags = *req.Tags
	}
	// Without the field (older clients, scripts) a course keeps the existing structure with sections; the
	// material form sends it, off by default.
	if req.SectionsEnabled != nil {
		n.SectionsEnabled = *req.SectionsEnabled
	}
	return s.repo.CreateCourse(ctx, p.UserID, n)
}

// UploadCover stores a cover picture of the teacher for the material form. The form refers to it by id when it
// saves the material; the server checks the content (PNG, JPEG or WebP, at most 5 MB).
func (s *Service) UploadCover(ctx context.Context, p auth.Principal, size int64, r io.Reader) (*Cover, error) {
	img, err := s.files.StoreImage(size, r)
	if err != nil {
		return nil, err
	}
	c, err := s.repo.CreateCover(ctx, &Cover{OwnerID: p.UserID, StoredName: img.StoredName, MimeType: img.MimeType, SizeBytes: img.SizeBytes, Width: img.Width, Height: img.Height})
	if err != nil {
		s.files.RemoveStored(img.StoredName)
		return nil, err
	}
	return c, nil
}

// OpenCover serves a cover picture to its uploader only.
func (s *Service) OpenCover(ctx context.Context, p auth.Principal, id uuid.UUID) (*Cover, *os.File, error) {
	c, err := s.repo.Cover(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, nil, apperr.NotFound("Обложка")
		}
		return nil, nil, err
	}
	if c.OwnerID != p.UserID {
		return nil, nil, apperr.NotFound("Обложка")
	}
	f, err := s.files.OpenStored(c.StoredName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, apperr.NotFound("Обложка")
		}
		return nil, nil, err
	}
	return c, f, nil
}

func (s *Service) UpdateCourse(ctx context.Context, p auth.Principal, id uuid.UUID, req CourseRequest) (*Course, error) {
	if _, err := s.ownedCourse(ctx, p, id); err != nil {
		return nil, err
	}
	v := apperr.V{}
	cleanTitle(v, "title", req.Title)
	if req.CoverColor != nil {
		v.Check(colorRe.MatchString(*req.CoverColor), "cover_color", "Некорректный цвет")
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		req.Description = &d
		v.Check(len([]rune(d)) <= 5000, "description", "Слишком длинное описание")
	}
	cover, clear, err := s.details(ctx, p, v, &req)
	if err != nil {
		return nil, err
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	ch := CourseChanges{Title: req.Title, Description: req.Description, Color: req.CoverColor, Language: req.Language,
		CoverImageID: cover, ClearCover: clear, SectionsEnabled: req.SectionsEnabled}
	if req.Tags != nil {
		ch.Tags = *req.Tags
	}
	if err := s.repo.UpdateCourse(ctx, id, ch); err != nil {
		return nil, err
	}
	return s.repo.GetCourse(ctx, id)
}

func (s *Service) ReorderCourses(ctx context.Context, p auth.Principal, ids []uuid.UUID) error {
	return s.reorder(ctx, "courses", p.UserID, ids)
}

func (s *Service) reorder(ctx context.Context, table string, parent uuid.UUID, ids []uuid.UUID) error {
	ok, err := s.repo.Reorder(ctx, table, parent, ids)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Validation(map[string]string{"ids": "Список должен содержать все элементы ровно один раз"})
	}
	return nil
}

func (s *Service) CreateLesson(ctx context.Context, p auth.Principal, courseID uuid.UUID, req LessonRequest) (*Lesson, error) {
	if _, err := s.ownedCourse(ctx, p, courseID); err != nil {
		return nil, err
	}
	v := apperr.V{}
	if req.Title == nil {
		v.Add("title", "Укажите название")
	}
	cleanTitle(v, "title", req.Title)
	if err := v.Err(); err != nil {
		return nil, err
	}
	l, err := s.repo.CreateLesson(ctx, courseID, *req.Title)
	if err != nil {
		return nil, err
	}
	if req.Attrs != nil || req.Description != nil {
		return s.UpdateLesson(ctx, p, l.ID, LessonRequest{Description: req.Description, Attrs: req.Attrs})
	}
	return l, nil
}

func normList(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range in {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] && len(x) <= 60 {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func (s *Service) GetLesson(ctx context.Context, p auth.Principal, id uuid.UUID) (*Lesson, error) {
	if _, err := s.OwnedLesson(ctx, p, id); err != nil {
		return nil, err
	}
	l, err := s.repo.GetLesson(ctx, id)
	if err != nil {
		return nil, err
	}
	l.Sections, err = s.repo.Sections(ctx, id)
	return l, err
}

func (s *Service) UpdateLesson(ctx context.Context, p auth.Principal, id uuid.UUID, req LessonRequest) (*Lesson, error) {
	if _, err := s.OwnedLesson(ctx, p, id); err != nil {
		return nil, err
	}
	v := apperr.V{}
	cleanTitle(v, "title", req.Title)
	if a := req.Attrs; a != nil {
		a.Skills, a.Grammar, a.Vocabulary, a.Functions, a.Tags = normList(a.Skills), normList(a.Grammar), normList(a.Vocabulary), normList(a.Functions), normList(a.Tags)
		a.Language, a.Age, a.Level, a.LessonType = strings.TrimSpace(a.Language), strings.TrimSpace(a.Age), strings.TrimSpace(a.Level), strings.TrimSpace(a.LessonType)
		if a.DurationMin != nil {
			v.Check(*a.DurationMin > 0 && *a.DurationMin <= 600, "attrs.duration_min", "Длительность от 1 до 600 минут")
		}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateLesson(ctx, id, req.Title, req.Description, req.Attrs); err != nil {
		return nil, err
	}
	s.notify.LessonChanged(ctx, id)
	return s.GetLesson(ctx, p, id)
}

func (s *Service) ReorderLessons(ctx context.Context, p auth.Principal, courseID uuid.UUID, ids []uuid.UUID) error {
	if _, err := s.ownedCourse(ctx, p, courseID); err != nil {
		return err
	}
	return s.reorder(ctx, "lessons", courseID, ids)
}

func (s *Service) CreateSection(ctx context.Context, p auth.Principal, lessonID uuid.UUID, req SectionRequest) (*Section, error) {
	if _, err := s.OwnedLesson(ctx, p, lessonID); err != nil {
		return nil, err
	}
	v := apperr.V{}
	cleanTitle(v, "title", &req.Title)
	if err := v.Err(); err != nil {
		return nil, err
	}
	sec, err := s.repo.CreateSection(ctx, lessonID, req.Title)
	if err == nil {
		s.notify.LessonChanged(ctx, lessonID)
	}
	return sec, err
}

func (s *Service) UpdateSection(ctx context.Context, p auth.Principal, sectionID uuid.UUID, req SectionRequest) error {
	lessonID, err := s.SectionLesson(ctx, sectionID)
	if err != nil {
		return err
	}
	if _, err := s.OwnedLesson(ctx, p, lessonID); err != nil {
		return apperr.NotFound("Раздел")
	}
	v := apperr.V{}
	cleanTitle(v, "title", &req.Title)
	if err := v.Err(); err != nil {
		return err
	}
	if err := s.repo.UpdateSection(ctx, sectionID, req.Title); err != nil {
		return err
	}
	s.notify.LessonChanged(ctx, lessonID)
	return nil
}

func (s *Service) ReorderSections(ctx context.Context, p auth.Principal, lessonID uuid.UUID, ids []uuid.UUID) error {
	if _, err := s.OwnedLesson(ctx, p, lessonID); err != nil {
		return err
	}
	if err := s.reorder(ctx, "sections", lessonID, ids); err != nil {
		return err
	}
	s.notify.LessonChanged(ctx, lessonID)
	return nil
}

func (s *Service) Options(ctx context.Context, p auth.Principal) (*FilterOptions, error) {
	return s.repo.Options(ctx, p.UserID)
}
