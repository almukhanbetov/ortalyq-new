package exercises

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.GET("/exercise-types", h.Catalog)
	t := authed.Group("", auth.RequireRole(auth.TeachingRoles...))
	t.GET("/lessons/:id/exercises", h.List)
	t.POST("/lessons/:id/exercises", h.Create)
	t.POST("/lessons/:id/files", h.UploadFile)
	t.PUT("/lessons/:id/exercises/order", h.Reorder)
	t.GET("/exercises/:id", h.Get)
	t.PUT("/exercises/:id", h.Update)
	t.PATCH("/exercises/:id/hidden", h.SetHidden)
	t.DELETE("/exercises/:id", h.Delete)
	t.POST("/exercises/:id/copy-to-homework", h.CopyToHomework)
	t.GET("/exercises/:id/stats", h.Stats)
	t.GET("/exercises/:id/file", h.File)
	t.GET("/score-categories", h.Categories)
	t.POST("/score-categories", h.CreateCategory)
}
