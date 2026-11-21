package users

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

func (h *Handler) Register(authed *gin.RouterGroup) {
	admin := authed.Group("/admin", auth.RequireRole(auth.RoleAdmin))
	admin.GET("/users", h.List)
	admin.POST("/users", h.Create)
	admin.PATCH("/users/:id", h.Update)
	admin.GET("/assignments", h.Assignments)
	admin.POST("/assignments", h.Assign)
	admin.DELETE("/assignments", h.Unassign)

	authed.GET("/teacher/students", auth.RequireRole(auth.RoleTeacher), h.MyStudents)
}
