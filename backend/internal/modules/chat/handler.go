package chat

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) List(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var cur Cursor
	cur.Limit, _ = strconv.Atoi(c.Query("limit"))
	if b := c.Query("before"); b != "" {
		t, err := time.Parse(time.RFC3339Nano, b)
		if err != nil {
			httpx.Fail(c, apperr.BadRequest("invalid_param", "Некорректный курсор"))
			return
		}
		bid, ok := httpx.UUIDQuery(c, "before_id")
		if !ok || bid == nil {
			httpx.Fail(c, apperr.BadRequest("invalid_param", "Некорректный курсор"))
			return
		}
		cur.Before, cur.BeforeID = &t, bid
	}
	page, err := h.svc.List(c.Request.Context(), principal(c), cid, cur)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *Handler) Send(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req SendRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.Send(c.Request.Context(), principal(c), cid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *Handler) Edit(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req EditRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.Edit(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), principal(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GifStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": h.svc.GifStatus()})
}

func (h *Handler) SearchGifs(c *gin.Context) {
	off, _ := strconv.Atoi(c.Query("offset"))
	items, err := h.svc.SearchGifs(c.Request.Context(), c.Query("q"), off)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
