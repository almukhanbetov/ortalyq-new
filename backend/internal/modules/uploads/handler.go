package uploads

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Upload(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.MaxBytes()+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			httpx.Fail(c, apperr.New(http.StatusRequestEntityTooLarge, "file_too_large", "Файл слишком большой"))
			return
		}
		httpx.Fail(c, apperr.Validation(map[string]string{"file": "Прикрепите файл"}))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	p, _ := auth.PrincipalFrom(c)
	a, err := h.svc.Save(c.Request.Context(), p, cid, fh.Filename, fh.Size, f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, a)
}

func (h *Handler) Download(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	p, _ := auth.PrincipalFrom(c)
	a, f, err := h.svc.Open(c.Request.Context(), p, id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	disp := "attachment"
	if c.Query("inline") == "1" && strings.HasPrefix(a.MimeType, "image/") {
		disp = "inline"
	}
	w := c.Writer.Header()
	w.Set("Content-Type", a.MimeType)
	w.Set("Content-Length", strconv.FormatInt(a.SizeBytes, 10))
	w.Set("Content-Disposition", fmt.Sprintf(`%s; filename="file"; filename*=UTF-8''%s`, disp, url.PathEscape(a.OriginalName)))
	w.Set("X-Content-Type-Options", "nosniff")
	w.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Set("Cache-Control", "private, max-age=0")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

