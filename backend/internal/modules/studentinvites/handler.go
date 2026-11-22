package studentinvites

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

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) Invite(c *gin.Context) {
	var req InviteRequest
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.Invite(c.Request.Context(), principal(c), req)
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

func (h *Handler) Accept(c *gin.Context) {
	var req TokenRequest
	if !httpx.Bind(c, &req) {
		return
	}
	res, err := h.svc.Accept(c.Request.Context(), principal(c), req.Token)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// RegisterStudent creates the student account from the link and signs it in.
func (h *Handler) RegisterStudent(c *gin.Context) {
	var req RegisterRequest
	if !httpx.Bind(c, &req) {
		return
	}
	token, p, err := h.svc.Register(c.Request.Context(), req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, token)
	c.JSON(http.StatusCreated, auth.ToMe(p))
}
