package auth

import "github.com/gin-gonic/gin"

func (h *Handler) Register(public, authed *gin.RouterGroup) {
	public.POST("/auth/login", h.Login)
	public.POST("/auth/logout", h.Logout)
	authed.GET("/auth/me", h.Me)
	authed.PATCH("/auth/me", h.UpdateMe)
}
