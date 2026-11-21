package chat

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
	"ortalyq/backend/internal/realtime"
)

const (
	maxBody    = 4000
	maxCaption = 500
)

type Members interface {
	MemberRole(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (string, error)
}

type Attachments interface {
	Get(ctx context.Context, id uuid.UUID) (*uploads.Attachment, error)
	Remove(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo    *Repository
	members Members
	files   Attachments
	giphy   *Giphy
	pub     realtime.Publisher
}

func NewService(repo *Repository, members Members, files Attachments, giphy *Giphy, pub realtime.Publisher) *Service {
	return &Service{repo: repo, members: members, files: files, giphy: giphy, pub: pub}
}

func (s *Service) List(ctx context.Context, p auth.Principal, classroomID uuid.UUID, c Cursor) (*Page, error) {
	if _, err := s.members.MemberRole(ctx, p, classroomID); err != nil {
		return nil, err
	}
	if c.Limit <= 0 || c.Limit > 100 {
		c.Limit = 50
	}
	items, err := s.repo.List(ctx, classroomID, c.Before, c.BeforeID, c.Limit+1)
	if err != nil {
		return nil, err
	}
	page := &Page{HasMore: len(items) > c.Limit}
	if page.HasMore {
		items = items[:c.Limit]
	}
	// Return oldest-first for display.
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	page.Items = items
	return page, nil
}

// Send stores a message. A repeated client_id returns the original message.
func (s *Service) Send(ctx context.Context, p auth.Principal, classroomID uuid.UUID, req SendRequest) (*Message, error) {
	if _, err := s.members.MemberRole(ctx, p, classroomID); err != nil {
		return nil, err
	}
	if req.ClientID == uuid.Nil {
		return nil, apperr.Validation(map[string]string{"client_id": "Отсутствует идентификатор сообщения"})
	}
	if existing, err := s.repo.ByClientID(ctx, p.UserID, req.ClientID); err == nil {
		if existing.ClassroomID != classroomID {
			return nil, apperr.Conflict("client_id_reused", "Идентификатор сообщения уже использован")
		}
		return existing, nil
	} else if !database.IsNoRows(err) {
		return nil, err
	}
	body := strings.TrimSpace(req.Body)
	var gif *Gif
	var attachment *uuid.UUID
	switch req.Kind {
	case "text":
		if body == "" || utf8.RuneCountInString(body) > maxBody {
			return nil, apperr.Validation(map[string]string{"body": "Сообщение должно содержать от 1 до 4000 символов"})
		}
	case "gif":
		if utf8.RuneCountInString(body) > maxCaption {
			return nil, apperr.Validation(map[string]string{"body": "Подпись не длиннее 500 символов"})
		}
		g, err := s.giphy.ByID(ctx, req.GifID)
		if err != nil {
			return nil, err
		}
		gif = g
	case "file":
		if utf8.RuneCountInString(body) > maxCaption {
			return nil, apperr.Validation(map[string]string{"body": "Подпись не длиннее 500 символов"})
		}
		if req.AttachmentID == nil {
			return nil, apperr.Validation(map[string]string{"attachment_id": "Прикрепите файл"})
		}
		a, err := s.files.Get(ctx, *req.AttachmentID)
		if err != nil {
			return nil, err
		}
		if a.ClassroomID != classroomID || a.UploaderID == nil || *a.UploaderID != p.UserID || a.Purpose != uploads.PurposeChat {
			return nil, apperr.NotFound("Файл")
		}
		used, err := s.repo.AttachmentInUse(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		if used {
			return nil, apperr.Conflict("attachment_used", "Файл уже отправлен")
		}
		attachment = &a.ID
	default:
		return nil, apperr.Validation(map[string]string{"kind": "Неизвестный тип сообщения"})
	}
	id, created, err := s.repo.Insert(ctx, classroomID, p.UserID, req.ClientID, req.Kind, body, gif, attachment)
	if err != nil {
		return nil, err
	}
	if !created { // concurrent duplicate
		return s.repo.ByClientID(ctx, p.UserID, req.ClientID)
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.pub.Publish(classroomID, realtime.EventMessageCreated, m)
	return m, nil
}

func (s *Service) own(ctx context.Context, p auth.Principal, id uuid.UUID) (*Message, error) {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, apperr.NotFound("Сообщение")
		}
		return nil, err
	}
	if _, err := s.members.MemberRole(ctx, p, m.ClassroomID); err != nil {
		return nil, apperr.NotFound("Сообщение")
	}
	if m.DeletedAt != nil {
		return nil, apperr.NotFound("Сообщение")
	}
	if m.Author.ID != p.UserID {
		return nil, apperr.New(403, "not_author", "Можно изменять только свои сообщения")
	}
	return m, nil
}

// Edit changes text (or the caption of a GIF/file message).
func (s *Service) Edit(ctx context.Context, p auth.Principal, id uuid.UUID, req EditRequest) (*Message, error) {
	m, err := s.own(ctx, p, id)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(req.Body)
	limit := maxCaption
	if m.Kind == "text" {
		limit = maxBody
		if body == "" {
			return nil, apperr.Validation(map[string]string{"body": "Сообщение не может быть пустым"})
		}
	}
	if utf8.RuneCountInString(body) > limit {
		return nil, apperr.Validation(map[string]string{"body": "Слишком длинный текст"})
	}
	if body == m.Body {
		return m, nil
	}
	if err := s.repo.UpdateBody(ctx, id, body); err != nil {
		return nil, err
	}
	m, err = s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.pub.Publish(m.ClassroomID, realtime.EventMessageUpdated, m)
	return m, nil
}

// Delete removes a message for every participant. Repeated deletes succeed.
func (s *Service) Delete(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		if database.IsNoRows(err) {
			return apperr.NotFound("Сообщение")
		}
		return err
	}
	if _, err := s.members.MemberRole(ctx, p, m.ClassroomID); err != nil {
		return apperr.NotFound("Сообщение")
	}
	if m.Author.ID != p.UserID {
		return apperr.New(403, "not_author", "Можно удалять только свои сообщения")
	}
	if m.DeletedAt != nil {
		return nil
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	if m.Attachment != nil {
		if err := s.files.Remove(ctx, m.Attachment.ID); err != nil {
			return err
		}
	}
	s.pub.Publish(m.ClassroomID, realtime.EventMessageDeleted, map[string]any{"id": id})
	return nil
}

func (s *Service) GifStatus() bool { return s.giphy.Enabled() }

func (s *Service) SearchGifs(ctx context.Context, q string, offset int) ([]Gif, error) {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) > 50 {
		return nil, apperr.Validation(map[string]string{"q": "Слишком длинный запрос"})
	}
	if offset < 0 || offset > 4999 {
		offset = 0
	}
	return s.giphy.Search(ctx, q, offset)
}
