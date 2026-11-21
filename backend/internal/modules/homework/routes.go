package homework

import "github.com/gin-gonic/gin"

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.GET("/classrooms/:cid/lessons/:lid/homework", h.Get)
	authed.POST("/classrooms/:cid/lessons/:lid/homework/submit", h.Submit)
	authed.POST("/classrooms/:cid/lessons/:lid/homework/status", h.SetStatus)
}
