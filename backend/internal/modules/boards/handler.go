package boards

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) List(c *gin.Context) {
	pg := httpx.PageQuery(c, 50, 100)
	items, total, err := h.svc.List(c.Request.Context(), principal(c), c.Query("q"), pg.Limit, pg.Offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.List[Summary]{Items: items, Total: total, Limit: pg.Limit, Offset: pg.Offset})
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	b, err := h.svc.Create(c.Request.Context(), principal(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	b, err := h.svc.Get(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *Handler) Save(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxContentSize+64<<10)
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Fail(c, apperr.New(http.StatusRequestEntityTooLarge, "board_too_large", "Доска слишком большая для сохранения"))
			return
		}
		httpx.Fail(c, apperr.BadRequest("invalid_json", "Некорректное тело запроса"))
		return
	}
	b, err := h.svc.Save(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *Handler) UploadImage(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	// The body is cut a little above the limit; the upload service checks the exact size and the content.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploads.ImageMaxBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "too large") {
			httpx.Fail(c, uploads.ErrImageTooLarge())
			return
		}
		httpx.Fail(c, apperr.Validation(map[string]string{"file": "Прикрепите картинку"}))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	img, err := h.svc.UploadImage(c.Request.Context(), principal(c), id, fh.Filename, fh.Size, f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, img)
}

// Image serves a board picture to the board's owner.
func (h *Handler) Image(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	iid, ok := httpx.UUIDParam(c, "imageId")
	if !ok {
		return
	}
	img, f, err := h.svc.OpenImage(c.Request.Context(), principal(c), id, iid)
	serveImage(c, img, f, err)
}

// ClassImage serves a picture of the board open in the lesson to a member of the class.
func (h *Handler) ClassImage(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	iid, ok := httpx.UUIDParam(c, "imageId")
	if !ok {
		return
	}
	img, f, err := h.svc.OpenClassImage(c.Request.Context(), principal(c), cid, iid)
	serveImage(c, img, f, err)
}

// serveImage writes a picture with the same safe headers as other uploads.
func serveImage(c *gin.Context, img *Image, f *os.File, err error) {
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	w := c.Writer.Header()
	w.Set("Content-Type", img.MimeType)
	w.Set("Content-Length", strconv.FormatInt(img.SizeBytes, 10))
	w.Set("Content-Disposition", `inline; filename="image"`)
	w.Set("X-Content-Type-Options", "nosniff")
	w.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Set("Cache-Control", "private, max-age=0")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

func (h *Handler) ClassBoard(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	res, err := h.svc.ClassBoard(c.Request.Context(), principal(c), cid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) OpenInClass(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req OpenRequest
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.OpenInClass(c.Request.Context(), principal(c), cid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) CloseInClass(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	if err := h.svc.CloseInClass(c.Request.Context(), principal(c), cid); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SetDrawer(allow bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid, ok := httpx.UUIDParam(c, "cid")
		if !ok {
			return
		}
		uid, ok := httpx.UUIDParam(c, "userId")
		if !ok {
			return
		}
		res, err := h.svc.SetDrawer(c.Request.Context(), principal(c), cid, uid, allow)
		if err != nil {
			httpx.Fail(c, err)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

func (h *Handler) AddStroke(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
	var req StrokeRequest
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.AddStroke(c.Request.Context(), principal(c), cid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	status := http.StatusCreated
	if res.Duplicate {
		status = http.StatusOK
	}
	c.JSON(status, res)
}
