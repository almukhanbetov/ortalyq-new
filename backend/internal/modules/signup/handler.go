package signup

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

// CookieSetter gives the browser the session (auth.Handler.SetSessionCookie).
type CookieSetter func(c *gin.Context, token string)

type Handler struct {
	svc       *Service
	setCookie CookieSetter
}

func NewHandler(svc *Service, setCookie CookieSetter) *Handler {
	return &Handler{svc: svc, setCookie: setCookie}
}

func (h *Handler) RequestEmail(c *gin.Context) {
	var req RequestEmail
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.RequestEmail(c.Request.Context(), req, c.ClientIP())
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

func (h *Handler) Complete(c *gin.Context) {
	var req CompleteRequest
	if !httpx.Bind(c, &req) {
		return
	}
	token, p, err := h.svc.Complete(c.Request.Context(), req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, token)
	c.JSON(http.StatusCreated, auth.ToMe(p))
}
