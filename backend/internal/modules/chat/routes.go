package chat

import "github.com/gin-gonic/gin"

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.GET("/classrooms/:cid/messages", h.List)
	authed.POST("/classrooms/:cid/messages", h.Send)
	authed.PATCH("/messages/:id", h.Edit)
	authed.DELETE("/messages/:id", h.Delete)
	authed.GET("/gifs/status", h.GifStatus)
	authed.GET("/gifs/search", h.SearchGifs)
}
