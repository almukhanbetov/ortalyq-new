package studentinvites

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

// Register mounts the routes. Tokens go in the body, never in the URL.
func (h *Handler) Register(public, authed *gin.RouterGroup) {
	authed.POST("/teacher/invitations", auth.RequireRole(auth.RolePrivateTeacher), h.Invite)
	public.POST("/student-invitations/verify", h.Verify)
	public.POST("/student-invitations/register", h.RegisterStudent)
	authed.POST("/student-invitations/accept", h.Accept)
}
