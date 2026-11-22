package classrooms

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.GET("/classrooms", h.List)
	authed.POST("/classrooms", auth.RequireRole(auth.TeachingRoles...), h.Create)
	authed.GET("/classrooms/:cid", h.Get)
	authed.PATCH("/classrooms/:cid", h.Rename)
	authed.PUT("/classrooms/:cid/current-lesson", h.SetCurrentLesson)
	authed.GET("/classrooms/:cid/ws", h.WebSocket)
}
