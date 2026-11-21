package exercises

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

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
