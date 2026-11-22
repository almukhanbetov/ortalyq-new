package passwordreset

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Request(c *gin.Context) {
	var req RequestReset
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.Request(c.Request.Context(), req, c.ClientIP())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusAccepted, res)
}

func (h *Handler) Verify(c *gin.Context) {
	var req TokenRequest
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.Verify(c.Request.Context(), req.Token)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// Complete sets the new password. No session is started: the user signs in again with the new password.
func (h *Handler) Complete(c *gin.Context) {
	var req CompleteRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.Complete(c.Request.Context(), req); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
