package exercises

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
	// frameAncestors are the web app origins that may show a PDF inside the page.
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
	if v == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(status, v)
}

func (h *Handler) Catalog(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"items": h.svc.Catalog()}) }

func (h *Handler) List(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	scope := c.DefaultQuery("scope", ScopeLesson)
	items, err := h.svc.ListForEditor(c.Request.Context(), principal(c), id, scope)
	respond(c, http.StatusOK, gin.H{"items": items}, err)
}

func (h *Handler) Create(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req CreateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	e, err := h.svc.Create(c.Request.Context(), principal(c), id, req)
	respond(c, http.StatusCreated, e, err)
}

func (h *Handler) Reorder(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req ReorderRequest
	if !httpx.Bind(c, &req) {
		return
	}
	respond(c, 0, nil, h.svc.Reorder(c.Request.Context(), principal(c), id, req))
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	e, err := h.svc.Get(c.Request.Context(), principal(c), id)
	respond(c, http.StatusOK, e, err)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req SaveRequest
	if !httpx.Bind(c, &req) {
		return
	}
	e, err := h.svc.Update(c.Request.Context(), principal(c), id, req)
	respond(c, http.StatusOK, e, err)
}

func (h *Handler) SetHidden(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req HiddenRequest
	if !httpx.Bind(c, &req) {
		return
	}
	respond(c, 0, nil, h.svc.SetHidden(c.Request.Context(), principal(c), id, req.Hidden))
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	respond(c, 0, nil, h.svc.Delete(c.Request.Context(), principal(c), id))
}

func (h *Handler) CopyToHomework(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	e, err := h.svc.CopyToHomework(c.Request.Context(), principal(c), id)
	respond(c, http.StatusCreated, e, err)
}

func (h *Handler) Stats(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	st, err := h.svc.Stats(c.Request.Context(), principal(c), id)
	respond(c, http.StatusOK, st, err)
}

func (h *Handler) Categories(c *gin.Context) {
	items, err := h.svc.Categories(c.Request.Context(), principal(c))
	respond(c, http.StatusOK, gin.H{"items": items}, err)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req CategoryRequest
	if !httpx.Bind(c, &req) {
		return
	}
	cat, err := h.svc.CreateCategory(c.Request.Context(), principal(c), req)
	respond(c, http.StatusCreated, cat, err)
}

// UploadFile takes the PDF of a «PDF-документ» block (multipart field "file").
func (h *Handler) UploadFile(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	// The body is cut a little above the limit; the upload service checks the exact size and the content.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploads.PDFMaxBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "too large") {
			httpx.Fail(c, uploads.ErrPDFTooLarge())
			return
		}
		httpx.Fail(c, apperr.Validation(map[string]string{"file": "Прикрепите PDF-файл"}))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	file, err := h.svc.UploadFile(c.Request.Context(), principal(c), id, fh.Filename, fh.Size, f)
	respond(c, http.StatusCreated, file, err)
}

// File serves the PDF of a block to the owner of its lesson.
func (h *Handler) File(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	file, f, err := h.svc.OpenFile(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	uploads.ServePDF(c, f, file.Name, file.CreatedAt, h.frameAncestors)
}
