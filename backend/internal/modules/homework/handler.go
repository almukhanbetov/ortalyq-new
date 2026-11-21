package homework

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func ids(c *gin.Context) (cid, lid [16]byte, ok bool) {
	a, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	b, ok := httpx.UUIDParam(c, "lid")
	return a, b, ok
}

func (h *Handler) Get(c *gin.Context) {
	cid, lid, ok := ids(c)
	if !ok {
		return
	}
	sid, ok := httpx.UUIDQuery(c, "student_id")
	if !ok {
		return
	}
	p, _ := auth.PrincipalFrom(c)
	v, err := h.svc.Get(c.Request.Context(), p, cid, lid, sid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) Submit(c *gin.Context) {
	cid, lid, ok := ids(c)
	if !ok {
		return
	}
	p, _ := auth.PrincipalFrom(c)
	v, err := h.svc.Submit(c.Request.Context(), p, cid, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) SetStatus(c *gin.Context) {
	cid, lid, ok := ids(c)
	if !ok {
		return
	}
	var req StatusRequest
	if !httpx.Bind(c, &req) {
		return
	}
	p, _ := auth.PrincipalFrom(c)
	v, err := h.svc.SetStatus(c.Request.Context(), p, cid, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
