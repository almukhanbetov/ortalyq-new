package documents

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

type Handler struct {
	svc *Service
	// frameAncestors are the web app origins that may show the PDF inside the page.
	frameAncestors []string
}

func NewHandler(svc *Service, frameAncestors []string) *Handler {
	return &Handler{svc: svc, frameAncestors: frameAncestors}
}

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func respond(c *gin.Context, status int, v any, err error) {
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(status, v)
}

func (h *Handler) List(c *gin.Context) {
	pg := httpx.PageQuery(c, 100, 100)
	res, err := h.svc.List(c.Request.Context(), principal(c), c.Query("q"), pg.Limit, pg.Offset)
	respond(c, http.StatusOK, res, err)
}

// Upload takes a PDF (multipart field "file") and its title (field "title").
func (h *Handler) Upload(c *gin.Context) {
	// The body is cut a little above the limit; the file store checks the exact size and the content.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploads.PDFMaxBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "too large") {
			httpx.Fail(c, uploads.ErrPDFTooLarge())
			return
		}
		httpx.Fail(c, apperr.Validation(map[string]string{"file": "Выберите PDF-файл"}))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	doc, err := h.svc.Upload(c.Request.Context(), principal(c), c.PostForm("title"), fh.Filename, fh.Size, f)
	respond(c, http.StatusCreated, doc, err)
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	doc, err := h.svc.Get(c.Request.Context(), principal(c), id)
	respond(c, http.StatusOK, doc, err)
}

// File serves the PDF to its owner, inline for the browser's viewer.
func (h *Handler) File(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	d, f, err := h.svc.OpenFile(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	uploads.ServePDF(c, f, d.OriginalName, d.CreatedAt, h.frameAncestors)
}
