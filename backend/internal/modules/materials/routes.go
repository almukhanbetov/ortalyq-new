package materials

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

// Register mounts teacher-only template management routes.
func (h *Handler) Register(authed *gin.RouterGroup) {
	t := authed.Group("", auth.RequireRole(auth.TeachingRoles...))
	t.GET("/courses", h.ListCourses)
	t.POST("/courses", h.CreateCourse)
	t.PUT("/courses/order", h.ReorderCourses)
	t.GET("/courses/:id", h.GetCourse)
	t.PATCH("/courses/:id", h.UpdateCourse)
	t.POST("/courses/:id/lessons", h.CreateLesson)
	t.PUT("/courses/:id/lessons/order", h.ReorderLessons)
	t.GET("/material-filters", h.Options)
	t.GET("/lessons/:id", h.GetLesson)
	t.PATCH("/lessons/:id", h.UpdateLesson)
	t.POST("/lessons/:id/sections", h.CreateSection)
	t.PUT("/lessons/:id/sections/order", h.ReorderSections)
	t.PATCH("/sections/:id", h.UpdateSection)
}
