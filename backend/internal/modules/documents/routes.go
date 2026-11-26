package documents

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

// Register mounts the teacher's PDF documents («Личные материалы»).
func (h *Handler) Register(authed *gin.RouterGroup) {
	t := authed.Group("/documents", auth.RequireRole(auth.TeachingRoles...))
	t.GET("", h.List)
	t.POST("", h.Upload)
	t.GET("/:id", h.Get)
	t.GET("/:id/file", h.File)
}
