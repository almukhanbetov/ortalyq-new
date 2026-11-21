package grading

import "github.com/gin-gonic/gin"

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.GET("/classrooms/:cid/lessons/:lid/progress", h.Progress)
	authed.PUT("/classrooms/:cid/lessons/:lid/completion", h.SetCompletion)
	authed.PUT("/classrooms/:cid/lessons/:lid/grade", h.SetGrade)
	authed.PUT("/classrooms/:cid/sections/:sid/completion", h.SetSection)
}
