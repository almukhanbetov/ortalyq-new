package boards

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

// Register mounts «Интерактивные доски» for teachers; the service also checks the owner of every board.
func (h *Handler) Register(authed *gin.RouterGroup) {
	t := authed.Group("", auth.RequireRole(auth.TeachingRoles...))
	t.GET("/boards", h.List)
	t.POST("/boards", h.Create)
	t.GET("/boards/:id", h.Get)
	t.PUT("/boards/:id", h.Save)
	t.POST("/boards/:id/images", h.UploadImage)
	t.GET("/boards/:id/images/:imageId", h.Image)
	// The board of a lesson: members of the class read it; the teacher of the class opens and closes it
	// (the service checks the class role and the board owner).
	authed.GET("/classrooms/:cid/board", h.ClassBoard)
	authed.GET("/classrooms/:cid/board/images/:imageId", h.ClassImage)
	authed.PUT("/classrooms/:cid/board", h.OpenInClass)
	authed.DELETE("/classrooms/:cid/board", h.CloseInClass)
	// Drawing together: the teacher lets a student draw; the student's strokes are added one by one.
	authed.PUT("/classrooms/:cid/board/drawers/:userId", h.SetDrawer(true))
	authed.DELETE("/classrooms/:cid/board/drawers/:userId", h.SetDrawer(false))
	authed.POST("/classrooms/:cid/board/strokes", h.AddStroke)
}
