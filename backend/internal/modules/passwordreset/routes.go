package passwordreset

import "github.com/gin-gonic/gin"

// Register mounts the public reset steps (no session needed). Tokens go in the body, never in the URL.
func (h *Handler) Register(public *gin.RouterGroup) {
	public.POST("/password-reset/request", h.Request)
	public.POST("/password-reset/verify", h.Verify)
	public.POST("/password-reset/complete", h.Complete)
}
