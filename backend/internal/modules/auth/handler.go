package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
)

const ctxPrincipal = "principal"

type CookieConfig struct {
	Name   string
	Secure bool
	Domain string
	MaxAge int
}

type Handler struct {
	svc    *Service
	cookie CookieConfig
}

func NewHandler(svc *Service, cookie CookieConfig) *Handler {
	return &Handler{svc: svc, cookie: cookie}
}

func (h *Handler) setCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: h.cookie.Name, Value: value, Path: "/", Domain: h.cookie.Domain,
		MaxAge: maxAge, HttpOnly: true, Secure: h.cookie.Secure, SameSite: http.SameSiteLaxMode,
	})
}

// SetSessionCookie gives the browser a session token issued by the service (used by the sign-up flow).
func (h *Handler) SetSessionCookie(c *gin.Context, token string) {
	h.setCookie(c, token, h.cookie.MaxAge)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if !httpx.Bind(c, &req) {
		return
	}
	token, p, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, token, h.cookie.MaxAge)
	c.JSON(http.StatusOK, ToMe(p))
}

func (h *Handler) Logout(c *gin.Context) {
	token, _ := c.Cookie(h.cookie.Name)
	if err := h.svc.Logout(c.Request.Context(), token); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	p, _ := PrincipalFrom(c)
	c.JSON(http.StatusOK, ToMe(p))
}

func (h *Handler) UpdateMe(c *gin.Context) {
	p, _ := PrincipalFrom(c)
	var req UpdateMeRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.UpdatePrefs(c.Request.Context(), p, req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if req.Locale != nil {
		p.Locale = *req.Locale
	}
	if req.Timezone != nil {
		p.Timezone = *req.Timezone
	}
	c.JSON(http.StatusOK, ToMe(p))
}

// LoadSession attaches the principal when a valid session cookie is present.
func (h *Handler) LoadSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := c.Cookie(h.cookie.Name); err == nil && token != "" {
			if p, err := h.svc.Authenticate(c.Request.Context(), token); err == nil {
				c.Set(ctxPrincipal, p)
			} else if _, ok := apperr.As(err); !ok {
				httpx.Fail(c, err)
				return
			}
		}
		c.Next()
	}
}

func PrincipalFrom(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(ctxPrincipal)
	if !ok {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}

// RequireAuth rejects anonymous requests with 401.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := PrincipalFrom(c); !ok {
			httpx.Fail(c, apperr.Unauthorized())
			return
		}
		c.Next()
	}
}

// RequireRole rejects principals whose role is not in roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := PrincipalFrom(c)
		if !ok {
			httpx.Fail(c, apperr.Unauthorized())
			return
		}
		for _, r := range roles {
			if p.Role == r {
				c.Next()
				return
			}
		}
		httpx.Fail(c, apperr.Forbidden())
	}
}
