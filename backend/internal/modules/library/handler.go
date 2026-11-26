package library

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) List(c *gin.Context) {
	res, err := h.svc.List(c.Request.Context(), principal(c), c.Query("q"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) Get(c *gin.Context) {
	res, err := h.svc.Get(c.Request.Context(), principal(c), c.Param("key"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) Add(c *gin.Context) {
	res, err := h.svc.Add(c.Request.Context(), principal(c), c.Param("key"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	c.JSON(status, res)
}
