package classrooms

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/realtime"
)

type Handler struct {
	svc *Service
	hub *realtime.Hub
}

func NewHandler(svc *Service, hub *realtime.Hub) *Handler { return &Handler{svc: svc, hub: hub} }

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) List(c *gin.Context) {
	pg := httpx.PageQuery(c, 24, 100)
	f := ListFilter{Query: c.Query("q"), Type: c.Query("type"), Sort: c.Query("sort"), Desc: c.DefaultQuery("order", "desc") != "asc", Limit: pg.Limit, Offset: pg.Offset}
	items, total, err := h.svc.List(c.Request.Context(), principal(c), f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.List[Classroom]{Items: items, Total: total, Limit: pg.Limit, Offset: pg.Offset})
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	cl, err := h.svc.Get(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, cl)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	cl, err := h.svc.Create(c.Request.Context(), principal(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, cl)
}

func (h *Handler) Rename(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req UpdateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	cl, err := h.svc.Rename(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, cl)
}

func (h *Handler) SetCurrentLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req SetLessonRequest
	if !httpx.Bind(c, &req) {
		return
	}
	cl, err := h.svc.SetCurrentLesson(c.Request.Context(), principal(c), id, req.LessonID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, cl)
}

// WebSocket upgrades only after the session and classroom membership are verified.
func (h *Handler) WebSocket(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	p := principal(c)
	if _, err := h.svc.MemberRole(c.Request.Context(), p, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.hub.Serve(c.Writer, c.Request, id, p.UserID)
}
