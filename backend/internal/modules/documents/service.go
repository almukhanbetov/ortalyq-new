package documents

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

// FileStore checks, writes and reads PDF files (uploads.Service: the same upload directory as other files).
type FileStore interface {
	StorePDF(size int64, r io.Reader) (*uploads.StoredFile, error)
	OpenStored(name string) (*os.File, error)
	RemoveStored(name string)
}

type Service struct {
	repo  *Repository
	files FileStore
}

func NewService(repo *Repository, files FileStore) *Service {
	return &Service{repo: repo, files: files}
}

func notFound() error { return apperr.NotFound("Документ") }

// Upload stores a PDF as a personal material of the teacher. The title is required; the form offers the file name
// without its extension. The content and the size are checked by the file store, not by the name or declared type.
func (s *Service) Upload(ctx context.Context, p auth.Principal, title, original string, size int64, r io.Reader) (*DocumentDTO, error) {
	title = strings.TrimSpace(title)
	v := apperr.V{}
	v.Check(title != "", "title", "Введите название")
	v.Check(utf8.RuneCountInString(title) <= TitleMax, "title", "Не длиннее 200 символов")
	if err := v.Err(); err != nil {
		return nil, err
	}
	st, err := s.files.StorePDF(size, r)
	if err != nil {
		return nil, err
	}
	name := uploads.SanitizeName(original)
	if !strings.EqualFold(filepath.Ext(name), ".pdf") {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".pdf"
	}
	d, err := s.repo.Create(ctx, &Document{OwnerID: p.UserID, Title: title, OriginalName: name, StoredName: st.StoredName, MimeType: st.MimeType, SizeBytes: st.SizeBytes})
	if err != nil {
		s.files.RemoveStored(st.StoredName)
		return nil, err
	}
	out := toDTO(d)
	return &out, nil
}

func (s *Service) List(ctx context.Context, p auth.Principal, q string, limit, offset int) (*ListResponse, error) {
	items, total, err := s.repo.List(ctx, p.UserID, strings.TrimSpace(q), limit, offset)
	if err != nil {
		return nil, err
	}
	res := &ListResponse{Items: make([]DocumentDTO, 0, len(items)), Total: total}
	for i := range items {
		res.Items = append(res.Items, toDTO(&items[i]))
	}
	return res, nil
}

// own returns the teacher's document. Another user's document is reported as missing, so its existence is not revealed.
func (s *Service) own(ctx context.Context, p auth.Principal, id uuid.UUID) (*Document, error) {
	d, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil || d.OwnerID != p.UserID {
		return nil, notFound()
	}
	return d, nil
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (*DocumentDTO, error) {
	d, err := s.own(ctx, p, id)
	if err != nil {
		return nil, err
	}
	out := toDTO(d)
	return &out, nil
}

// OpenFile opens the PDF of the teacher's own document; the access check runs on every request, also for a link
// opened directly in a new tab.
func (s *Service) OpenFile(ctx context.Context, p auth.Principal, id uuid.UUID) (*Document, *os.File, error) {
	d, err := s.own(ctx, p, id)
	if err != nil {
		return nil, nil, err
	}
	f, err := s.files.OpenStored(d.StoredName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, notFound()
		}
		return nil, nil, err
	}
	return d, f, nil
}
