package library

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

// Register mounts «Материалы» → «Каталог» for teachers.
func (h *Handler) Register(authed *gin.RouterGroup) {
	t := authed.Group("", auth.RequireRole(auth.TeachingRoles...))
	t.GET("/material-catalog", h.List)
	t.GET("/material-catalog/:key", h.Get)
	t.POST("/material-catalog/:key/add", h.Add)
}
