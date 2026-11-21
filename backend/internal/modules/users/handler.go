package users

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(c *gin.Context) {
	pg := httpx.PageQuery(c, 50, 200)
	items, total, err := h.svc.List(c.Request.Context(), ListFilter{Role: c.Query("role"), Query: c.Query("q"), Limit: pg.Limit, Offset: pg.Offset})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.List[User]{Items: items, Total: total, Limit: pg.Limit, Offset: pg.Offset})
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateUserRequest
	if !httpx.Bind(c, &req) {
		return
	}
	u, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req UpdateUserRequest
	if !httpx.Bind(c, &req) {
		return
	}
	p, _ := auth.PrincipalFrom(c)
	u, err := h.svc.Update(c.Request.Context(), p, id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) Assignments(c *gin.Context) {
	pg := httpx.PageQuery(c, 50, 200)
	items, total, err := h.svc.Assignments(c.Request.Context(), pg.Limit, pg.Offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.List[Assignment]{Items: items, Total: total, Limit: pg.Limit, Offset: pg.Offset})
}

func (h *Handler) Assign(c *gin.Context) {
	var req AssignmentRequest
	if !httpx.Bind(c, &req) {
		return
	}
	p, _ := auth.PrincipalFrom(c)
	if err := h.svc.Assign(c.Request.Context(), p, req); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Unassign(c *gin.Context) {
	var req AssignmentRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.Unassign(c.Request.Context(), req); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) MyStudents(c *gin.Context) {
	p, _ := auth.PrincipalFrom(c)
	items, err := h.svc.MyStudents(c.Request.Context(), p)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
