package signup

import "github.com/gin-gonic/gin"

// Register mounts the public sign-up steps (no session needed). Tokens go in the body, never in the URL.
func (h *Handler) Register(public *gin.RouterGroup) {
	public.POST("/signup/email", h.RequestEmail)
	public.POST("/signup/verify", h.Verify)
	public.POST("/signup/complete", h.Complete)
}
