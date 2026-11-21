package uploads

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
)

type Members interface {
	MemberRole(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (string, error)
}

type Service struct {
	repo     *Repository
	members  Members
	dir      string
	maxBytes int64
}

func NewService(repo *Repository, members Members, dir string, maxBytes int64) (*Service, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &Service{repo: repo, members: members, dir: abs, maxBytes: maxBytes}, nil
}

func (s *Service) MaxBytes() int64 { return s.maxBytes }

func sanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	var b strings.Builder
	for _, r := range name {
		if r < 32 || r == 127 || strings.ContainsRune(`/\:*?"<>|`, r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" || out == "." || out == ".." {
		out = "file"
	}
	for len(out) > 150 {
		_, size := utf8.DecodeLastRuneInString(out)
		out = out[:len(out)-size]
	}
	return out
}

// detect returns the verified content type and storage extension.
func detect(head []byte, original string) (string, string, bool) {
	ct := http.DetectContentType(head)
	if i := strings.Index(ct, ";"); i > 0 {
		ct = ct[:i]
	}
	ext := strings.ToLower(filepath.Ext(original))
	if ct == "application/zip" {
		if mt, ok := officeTypes[ext]; ok {
			return mt, ext, true
		}
		return "", "", false
	}
	if ct == "text/plain" && ext != ".txt" {
		return "", "", false
	}
	if e, ok := allowed[ct]; ok {
		return ct, e, true
	}
	return "", "", false
}

// Save stores an uploaded file for a classroom member.
func (s *Service) Save(ctx context.Context, p auth.Principal, classroomID uuid.UUID, original string, size int64, r io.Reader) (*Attachment, error) {
	if _, err := s.members.MemberRole(ctx, p, classroomID); err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, apperr.Validation(map[string]string{"file": "Файл пустой"})
	}
	if size > s.maxBytes {
		return nil, apperr.New(http.StatusRequestEntityTooLarge, "file_too_large", "Файл слишком большой")
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = head[:n]
	mime, ext, ok := detect(head, original)
	if !ok {
		return nil, apperr.Validation(map[string]string{"file": "Недопустимый тип файла"})
	}
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return nil, err
	}
	stored := hex.EncodeToString(rnd) + ext
	path := filepath.Join(s.dir, stored)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, err
	}
	written, err := io.Copy(f, io.LimitReader(io.MultiReader(bytes.NewReader(head), r), s.maxBytes+1))
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err == nil && written > s.maxBytes {
		err = apperr.New(http.StatusRequestEntityTooLarge, "file_too_large", "Файл слишком большой")
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	uid := p.UserID
	a := &Attachment{ClassroomID: classroomID, UploaderID: &uid, OriginalName: sanitizeName(original), StoredName: stored, MimeType: mime, SizeBytes: written}
	if err := s.repo.Insert(ctx, a); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return a, nil
}

// Open returns an attachment and its file if the principal is a member of its classroom.
func (s *Service) Open(ctx context.Context, p auth.Principal, id uuid.UUID) (*Attachment, *os.File, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, nil, apperr.NotFound("Файл")
		}
		return nil, nil, err
	}
	if _, err := s.members.MemberRole(ctx, p, a.ClassroomID); err != nil {
		return nil, nil, apperr.NotFound("Файл")
	}
	f, err := os.Open(filepath.Join(s.dir, filepath.Base(a.StoredName)))
	if err != nil {
		return nil, nil, apperr.NotFound("Файл")
	}
	return a, f, nil
}

// Get returns attachment metadata without access checks (used by chat after its own checks).
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Attachment, error) {
	a, err := s.repo.Get(ctx, id)
	if database.IsNoRows(err) {
		return nil, apperr.NotFound("Файл")
	}
	return a, err
}

// Remove deletes the metadata and file (used when a chat message is deleted).
func (s *Service) Remove(ctx context.Context, id uuid.UUID) error {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil
		}
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(s.dir, filepath.Base(a.StoredName)))
	return nil
}
