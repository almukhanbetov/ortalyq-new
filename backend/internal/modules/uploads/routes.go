package uploads

import "github.com/gin-gonic/gin"

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.POST("/classrooms/:cid/attachments", h.Upload)
	authed.GET("/attachments/:id", h.Download)
}
