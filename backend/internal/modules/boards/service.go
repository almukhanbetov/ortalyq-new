package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/classrooms"
	"ortalyq/backend/internal/modules/uploads"
	"ortalyq/backend/internal/realtime"
)

// ImageStore checks and stores picture files (uploads.Service).
type ImageStore interface {
	StoreImage(size int64, r io.Reader) (*uploads.StoredImage, error)
	OpenStored(name string) (*os.File, error)
	CopyStored(name string) (string, error)
	RemoveStored(name string)
}

// Classrooms tells the caller's role in a class (classrooms.Service): 404 for a non-member.
type Classrooms interface {
	MemberRole(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (string, error)
}

type Service struct {
	repo    *Repository
	images  ImageStore
	classes Classrooms
	pub     realtime.Publisher
}

func NewService(repo *Repository, images ImageStore, classes Classrooms, pub realtime.Publisher) *Service {
	return &Service{repo: repo, images: images, classes: classes, pub: pub}
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func teaching(p auth.Principal) bool {
	for _, r := range auth.TeachingRoles {
		if p.Role == r {
			return true
		}
	}
	return false
}

func errNotFound() error { return apperr.NotFound("Доска") }

func cleanTitle(v apperr.V, s string) string {
	s = strings.TrimSpace(s)
	v.Check(s != "" && utf8.RuneCountInString(s) <= 200, "title", "Введите название до 200 символов")
	return s
}

func (s *Service) List(ctx context.Context, p auth.Principal, q string, limit, offset int) ([]Summary, int, error) {
	if !teaching(p) {
		return nil, 0, apperr.Forbidden()
	}
	return s.repo.List(ctx, p.UserID, strings.TrimSpace(q), limit, offset)
}

func (s *Service) Create(ctx context.Context, p auth.Principal, req CreateRequest) (*Board, error) {
	if !teaching(p) {
		return nil, apperr.Forbidden()
	}
	v := apperr.V{}
	title := cleanTitle(v, req.Title)
	if err := v.Err(); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, p.UserID, title)
}

// Get opens a board for its owner; for anyone else the board does not exist (404, no hint that it does).
func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (*Board, error) {
	if !teaching(p) {
		return nil, apperr.Forbidden()
	}
	b, err := s.repo.Owned(ctx, p.UserID, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, errNotFound()
	}
	if b.OpenIn, err = s.repo.OpenIn(ctx, id); err != nil {
		return nil, err
	}
	return b, nil
}

// Save stores the drawing of an owned board. A save made from a stale copy is refused with 409 board_conflict
// unless forced, so the editor can keep the teacher's changes and offer to overwrite.
func (s *Service) Save(ctx context.Context, p auth.Principal, id uuid.UUID, req SaveRequest) (*Board, error) {
	if !teaching(p) {
		return nil, apperr.Forbidden()
	}
	v := apperr.V{}
	if req.Title != nil {
		t := cleanTitle(v, *req.Title)
		req.Title = &t
	}
	images := validateContent(v, &req.Content)
	if err := v.Err(); err != nil {
		return nil, err
	}
	if len(images) > 0 {
		// Pictures must be this board's own uploads (not another teacher's file, not another board's).
		ok, err := s.repo.ImagesBelong(ctx, id, p.UserID, images)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperr.Validation(map[string]string{"content": "Картинка не найдена на этой доске"})
		}
	}
	raw, err := json.Marshal(req.Content)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxContentSize {
		return nil, apperr.Validation(map[string]string{"content": "Доска слишком большая для сохранения"})
	}
	found, saved, err := s.repo.Save(ctx, p.UserID, id, req.Title, raw, req.Version, req.Force)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errNotFound()
	}
	if !saved {
		return nil, apperr.Conflict("board_conflict", "Доска изменилась в другой вкладке")
	}
	b, err := s.Get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	// Classes where the board is open re-read it (the event only notifies).
	for _, c := range b.OpenIn {
		s.pub.Publish(c.ClassroomID, realtime.EventBoardUpdated, map[string]any{"board_id": id, "version": b.Version})
	}
	return b, nil
}

func finite(xs ...float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

var itemID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// validateContent checks every item, keeps coordinates inside the world and clears fields a type does not use.
// It returns the picture ids the items refer to (the caller checks they belong to the board).
func validateContent(v apperr.V, c *Content) []uuid.UUID {
	if c.Items == nil {
		c.Items = []Item{}
	}
	if len(c.Items) > MaxItems {
		v.Add("content", fmt.Sprintf("Не более %d элементов на доске", MaxItems))
		return nil
	}
	const lim = WorldLimit
	var images []uuid.UUID
	seen := map[string]bool{}
	for i := range c.Items {
		it := &c.Items[i]
		k := fmt.Sprintf("content.items.%d", i)
		if it.ID != "" {
			if !itemID.MatchString(it.ID) || seen[it.ID] {
				v.Add(k+".id", "Некорректный или повторяющийся id элемента")
			}
			seen[it.ID] = true
		}
		if it.Type != ItemImage && !hexColor.MatchString(it.Color) {
			v.Add(k+".color", "Некорректный цвет")
		}
		if !finite(it.X, it.Y, it.W, it.H, it.Width, it.Size) {
			v.Add(k, "Некорректные координаты")
			continue
		}
		box := func(min float64) {
			if it.W < min || it.H < min || it.W > MaxShape || it.H > MaxShape {
				v.Add(k+".size", fmt.Sprintf("Размер от %g до %d", min, MaxShape))
			}
			it.X, it.Y = clamp(it.X, -lim, lim), clamp(it.Y, -lim, lim)
		}
		line := func() {
			if !finite(it.Width) || it.Width < 1 || it.Width > 40 {
				v.Add(k+".width", "Толщина линии от 1 до 40")
			}
		}
		points := func() {
			for j := range it.Points {
				pt := &it.Points[j]
				if !finite(pt[0], pt[1]) {
					v.Add(k+".points", "Некорректные координаты")
					return
				}
				pt[0], pt[1] = clamp(pt[0], -lim, lim), clamp(pt[1], -lim, lim)
			}
		}
		switch it.Type {
		case ItemStroke:
			if len(it.Points) == 0 || len(it.Points) > MaxPoints {
				v.Add(k+".points", fmt.Sprintf("Линия: от 1 до %d точек", MaxPoints))
			}
			line()
			points()
			*it = Item{ID: it.ID, Type: it.Type, Color: it.Color, Width: it.Width, Points: it.Points}
		case ItemArrow:
			if len(it.Points) != 2 {
				v.Add(k+".points", "Стрелка задаётся началом и концом")
			}
			line()
			points()
			*it = Item{ID: it.ID, Type: it.Type, Color: it.Color, Width: it.Width, Points: it.Points}
		case ItemText:
			it.Text = strings.TrimSpace(it.Text)
			if it.Text == "" || utf8.RuneCountInString(it.Text) > MaxText {
				v.Add(k+".text", fmt.Sprintf("Текст от 1 до %d символов", MaxText))
			}
			if it.Size < 8 || it.Size > 120 {
				v.Add(k+".size", "Размер текста от 8 до 120")
			}
			it.X, it.Y = clamp(it.X, -lim, lim), clamp(it.Y, -lim, lim)
			*it = Item{ID: it.ID, Type: it.Type, Color: it.Color, X: it.X, Y: it.Y, Size: it.Size, Text: it.Text}
		case ItemSticky:
			if utf8.RuneCountInString(it.Text) > MaxSticky {
				v.Add(k+".text", fmt.Sprintf("Текст стикера до %d символов", MaxSticky))
			}
			box(40)
			*it = Item{ID: it.ID, Type: it.Type, Color: it.Color, X: it.X, Y: it.Y, W: it.W, H: it.H, Text: it.Text}
		case ItemRect, ItemEllipse:
			line()
			box(MinShape)
			*it = Item{ID: it.ID, Type: it.Type, Color: it.Color, Width: it.Width, X: it.X, Y: it.Y, W: it.W, H: it.H}
		case ItemImage:
			id, err := uuid.Parse(it.ImageID)
			if err != nil {
				v.Add(k+".image_id", "Картинка не найдена")
			} else {
				images = append(images, id)
			}
			box(MinShape)
			*it = Item{ID: it.ID, Type: it.Type, X: it.X, Y: it.Y, W: it.W, H: it.H, ImageID: it.ImageID}
		default:
			v.Add(k+".type", "Неизвестный элемент доски")
		}
	}
	return images
}

// UploadImage stores a picture for an owned board (the same checks as class pictures).
func (s *Service) UploadImage(ctx context.Context, p auth.Principal, boardID uuid.UUID, original string, size int64, r io.Reader) (*Image, error) {
	if _, err := s.Get(ctx, p, boardID); err != nil {
		return nil, err
	}
	n, err := s.repo.ImageCount(ctx, boardID)
	if err != nil {
		return nil, err
	}
	if n >= MaxImages {
		return nil, apperr.Conflict("board_images_limit", fmt.Sprintf("На доске не больше %d картинок", MaxImages))
	}
	st, err := s.images.StoreImage(size, r)
	if err != nil {
		return nil, err
	}
	img, err := s.repo.AddImage(ctx, boardID, p.UserID, original, st)
	if err != nil {
		s.images.RemoveStored(st.StoredName)
		return nil, err
	}
	return img, nil
}

// OpenImage returns a picture of an owned board and its file; for anyone else it does not exist.
func (s *Service) OpenImage(ctx context.Context, p auth.Principal, boardID, imageID uuid.UUID) (*Image, *os.File, error) {
	if !teaching(p) {
		return nil, nil, apperr.Forbidden()
	}
	img, err := s.repo.Image(ctx, boardID, p.UserID, imageID)
	if err != nil {
		return nil, nil, err
	}
	if img == nil {
		return nil, nil, apperr.NotFound("Картинка")
	}
	f, err := s.images.OpenStored(img.StoredName)
	if err != nil {
		return nil, nil, apperr.NotFound("Картинка")
	}
	return img, f, nil
}

// requireClassTeacher: the caller teaches this class (a non-member gets 404, a student 403).
func (s *Service) requireClassTeacher(ctx context.Context, p auth.Principal, classroomID uuid.UUID) error {
	role, err := s.classes.MemberRole(ctx, p, classroomID)
	if err != nil {
		return err
	}
	if role != classrooms.MemberTeacher || !teaching(p) {
		return apperr.Forbidden()
	}
	return nil
}

// OpenInClass is «Открыть в классе»: the teacher of the class opens an own board in the lesson. The class members
// can then read this board (and its pictures) through the lesson until it is closed. With «Создать копию для
// занятия» (req.Copy) a copy of the board is made and opened instead, so the lesson's strokes go to the copy and the
// original stays as it was; a repeated request (same request_id) returns that copy and opens nothing again.
func (s *Service) OpenInClass(ctx context.Context, p auth.Principal, classroomID uuid.UUID, req OpenRequest) (*ClassBoard, error) {
	if err := s.requireClassTeacher(ctx, p, classroomID); err != nil {
		return nil, err
	}
	boardID := req.BoardID
	var made *Summary
	if req.Copy {
		if req.RequestID == uuid.Nil {
			return nil, apperr.Validation(map[string]string{"request_id": "Нет идентификатора запроса"})
		}
		if prev, err := s.repo.CopyByRequest(ctx, p.UserID, req.RequestID); err != nil {
			return nil, err
		} else if prev != nil {
			return s.answerCopy(ctx, p, classroomID, *prev)
		}
		src, err := s.Get(ctx, p, req.BoardID)
		if err != nil {
			return nil, err // not the caller's board: 404
		}
		id, created, err := s.makeCopy(ctx, p, src, classroomID, req.RequestID)
		if err != nil {
			return nil, err
		}
		if !created {
			return s.answerCopy(ctx, p, classroomID, id)
		}
		boardID = id
	} else if _, err := s.Get(ctx, p, req.BoardID); err != nil {
		return nil, err // not the caller's board: 404
	}
	if err := s.repo.OpenInClass(ctx, classroomID, boardID, p.UserID); err != nil {
		return nil, err
	}
	s.pub.Publish(classroomID, realtime.EventBoardOpened, map[string]any{"board_id": boardID})
	view, err := s.ClassBoard(ctx, p, classroomID)
	if err != nil {
		return nil, err
	}
	if req.Copy {
		if made, err = s.summary(ctx, p, boardID); err != nil {
			return nil, err
		}
		view.Copy = made
	}
	return view, nil
}

func (s *Service) summary(ctx context.Context, p auth.Principal, id uuid.UUID) (*Summary, error) {
	b, err := s.Get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	return &b.Summary, nil
}

// answerCopy is the answer to a repeated copy request: the class as it is now and the copy made the first time.
func (s *Service) answerCopy(ctx context.Context, p auth.Principal, classroomID, copyID uuid.UUID) (*ClassBoard, error) {
	view, err := s.ClassBoard(ctx, p, classroomID)
	if err != nil {
		return nil, err
	}
	if view.Copy, err = s.summary(ctx, p, copyID); err != nil {
		return nil, err
	}
	return view, nil
}

// CopyTitle is «Название доски — Название класса — Дата», at most 200 characters (the board title is shortened).
func CopyTitle(board, class string, day time.Time) string {
	tail := " — " + class + " — " + day.Format("02.01.2006")
	room := 200 - utf8.RuneCountInString(tail)
	r := []rune(board)
	if room < 1 {
		return string([]rune(board + tail)[:200])
	}
	if len(r) > room {
		r = append(r[:room-1], '…')
	}
	return string(r) + tail
}

// makeCopy copies the board: every object, every picture (as its own file) with the picture ids renamed in the
// objects. The copied files are removed again when nothing is recorded (an error or a parallel repeat).
func (s *Service) makeCopy(ctx context.Context, p auth.Principal, src *Board, classroomID, requestID uuid.UUID) (uuid.UUID, bool, error) {
	class, err := s.repo.ClassName(ctx, classroomID)
	if err != nil {
		return uuid.Nil, false, err
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil || p.Timezone == "" {
		loc = time.UTC
	}
	title := CopyTitle(src.Title, class, time.Now().In(loc))
	imgs, err := s.repo.Images(ctx, src.ID)
	if err != nil {
		return uuid.Nil, false, err
	}
	var copied []CopiedImage
	cleanup := func() {
		for _, c := range copied {
			s.images.RemoveStored(c.StoredName)
		}
	}
	ids := map[string]string{}
	for _, img := range imgs {
		name, err := s.images.CopyStored(img.StoredName)
		if err != nil {
			cleanup()
			return uuid.Nil, false, err
		}
		c := CopiedImage{ID: uuid.New(), Source: img, StoredName: name}
		copied = append(copied, c)
		ids[img.ID.String()] = c.ID.String()
	}
	items := make([]Item, len(src.Content.Items))
	for i, it := range src.Content.Items {
		if it.Type == ItemImage {
			it.ImageID = ids[it.ImageID]
		}
		items[i] = it
	}
	raw, err := json.Marshal(Content{Items: items})
	if err != nil {
		cleanup()
		return uuid.Nil, false, err
	}
	id, created, err := s.repo.CreateCopy(ctx, p.UserID, src.ID, classroomID, requestID, title, raw, copied)
	if err != nil || !created {
		cleanup()
	}
	return id, created, err
}

// CloseInClass is «Закрыть доску в классе»: the lesson access ends at once; the board stays in «Материалы».
func (s *Service) CloseInClass(ctx context.Context, p auth.Principal, classroomID uuid.UUID) error {
	if err := s.requireClassTeacher(ctx, p, classroomID); err != nil {
		return err
	}
	closed, err := s.repo.CloseInClass(ctx, classroomID)
	if err != nil {
		return err
	}
	if closed != nil {
		s.pub.Publish(classroomID, realtime.EventBoardClosed, map[string]any{"board_id": *closed})
	}
	return nil
}

// ClassBoard returns the board open in the lesson to a member of the class (board null when none is open).
func (s *Service) ClassBoard(ctx context.Context, p auth.Principal, classroomID uuid.UUID) (*ClassBoard, error) {
	role, err := s.classes.MemberRole(ctx, p, classroomID)
	if err != nil {
		return nil, err
	}
	b, at, err := s.repo.ClassBoard(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return &ClassBoard{}, nil
	}
	b.OpenIn = nil
	out := &ClassBoard{Board: b, OpenedAt: at}
	drawers, err := s.repo.Drawers(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if role == classrooms.MemberTeacher {
		own, err := s.repo.Owned(ctx, p.UserID, b.ID)
		if err != nil {
			return nil, err
		}
		out.CanDraw = own != nil
		out.Drawers = drawers
	} else {
		for _, id := range drawers {
			out.CanDraw = out.CanDraw || id == p.UserID
		}
	}
	return out, nil
}

// OpenClassImage serves a picture of the board open in the lesson to a member of the class. Pictures of any other
// board (even the same owner's) are not reachable through the lesson.
func (s *Service) OpenClassImage(ctx context.Context, p auth.Principal, classroomID, imageID uuid.UUID) (*Image, *os.File, error) {
	if _, err := s.classes.MemberRole(ctx, p, classroomID); err != nil {
		return nil, nil, err
	}
	img, err := s.repo.ClassImage(ctx, classroomID, imageID)
	if err != nil {
		return nil, nil, err
	}
	if img == nil {
		return nil, nil, apperr.NotFound("Картинка")
	}
	f, err := s.images.OpenStored(img.StoredName)
	if err != nil {
		return nil, nil, apperr.NotFound("Картинка")
	}
	return img, f, nil
}

// SetDrawer is «Разрешить рисовать» / its withdrawal: the teacher of the class lets one student of the class draw on
// the board open now, or takes it back (effective at once: the next stroke is refused).
func (s *Service) SetDrawer(ctx context.Context, p auth.Principal, classroomID, userID uuid.UUID, allow bool) (*ClassBoard, error) {
	if err := s.requireClassTeacher(ctx, p, classroomID); err != nil {
		return nil, err
	}
	open, err := s.repo.OpenBoardID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if open == nil {
		return nil, errBoardNotOpen()
	}
	// The target must be a student of this class (MemberRole looks only at the user id).
	role, err := s.classes.MemberRole(ctx, auth.Principal{UserID: userID}, classroomID)
	if err != nil {
		return nil, apperr.NotFound("Ученик")
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.Validation(map[string]string{"user_id": "Рисовать можно разрешить только ученику класса"})
	}
	if allow {
		err = s.repo.Grant(ctx, classroomID, *open, userID, p.UserID)
	} else {
		err = s.repo.Revoke(ctx, classroomID, userID)
	}
	if err != nil {
		return nil, err
	}
	s.pub.Publish(classroomID, realtime.EventBoardDrawers, map[string]any{"user_id": userID, "can_draw": allow})
	return s.ClassBoard(ctx, p, classroomID)
}

func errBoardNotOpen() error {
	return apperr.Conflict("board_not_open", "Доска закрыта или заменена другой")
}

// AddStroke is a student's pen stroke on the lesson board: the board must be open in this class (the one the student
// drew on), the student a member of the class with the permission for this board. The stroke is added as its own
// action; the same client_id sent again adds nothing.
func (s *Service) AddStroke(ctx context.Context, p auth.Principal, classroomID uuid.UUID, req StrokeRequest) (*StrokeResult, error) {
	role, err := s.classes.MemberRole(ctx, p, classroomID)
	if err != nil {
		return nil, err
	}
	if role != classrooms.MemberStudent {
		return nil, apperr.New(403, "drawing_not_allowed", "Преподаватель рисует в редакторе доски")
	}
	v := apperr.V{}
	v.Check(req.ClientID != uuid.Nil, "client_id", "Нет идентификатора штриха")
	it := Item{ID: "s-" + req.ClientID.String(), Type: ItemStroke, Color: req.Color, Width: req.Width, Points: req.Points}
	c := Content{Items: []Item{it}}
	validateContent(v, &c)
	if err := v.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(c.Items[0])
	if err != nil {
		return nil, err
	}
	res, refusal, err := s.repo.AddStroke(ctx, classroomID, p.UserID, req, raw, it.ID)
	if err != nil {
		return nil, err
	}
	switch refusal {
	case strokeNoBoard:
		return nil, errBoardNotOpen()
	case strokeNotAllowed:
		return nil, apperr.New(403, "drawing_not_allowed", "Преподаватель не разрешил рисовать на доске")
	case strokeFull:
		return nil, apperr.Validation(map[string]string{"content": fmt.Sprintf("Не более %d элементов на доске", MaxItems)})
	}
	if !res.Duplicate {
		s.pub.Publish(classroomID, realtime.EventBoardUpdated, map[string]any{"board_id": req.BoardID, "version": res.Version})
	}
	return &res, nil
}
