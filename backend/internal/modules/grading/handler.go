package grading

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) Progress(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDQuery(c, "student_id")
	if !ok {
		return
	}
	pr, err := h.svc.Progress(c.Request.Context(), principal(c), cid, lid, sid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, pr)
}

func (h *Handler) SetSection(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDParam(c, "sid")
	if !ok {
		return
	}
	var req SectionCompletionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.SetSection(c.Request.Context(), principal(c), cid, sid, req.Completed); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SetCompletion(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	var req LessonCompletionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.SetCompletion(c.Request.Context(), principal(c), cid, lid, req); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SetGrade(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	var req GradeRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.SetGrade(c.Request.Context(), principal(c), cid, lid, req.Grade); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
