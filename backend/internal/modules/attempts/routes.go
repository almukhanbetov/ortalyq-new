package attempts

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

func (h *Handler) Register(authed *gin.RouterGroup) {
	authed.GET("/classrooms/:cid/lessons/:lid/exercises", h.LessonExercises)
	authed.GET("/classrooms/:cid/exercises/:eid", h.View)
	authed.GET("/classrooms/:cid/exercises/:eid/file", h.File)
	authed.POST("/classrooms/:cid/exercises/:eid/attempt", h.Start)
	authed.POST("/classrooms/:cid/exercises/:eid/reset", h.Reset)
	authed.PUT("/attempts/:aid/answers/:qid", h.Answer)
	authed.POST("/attempts/:aid/finish", h.Finish)

	// Teacher preview (stage 8): the lesson owner only; separate preview attempts, no classroom.
	t := authed.Group("/preview", auth.RequireRole(auth.TeachingRoles...))
	t.GET("/lessons/:lid/exercises", h.PreviewLesson)
	t.POST("/lessons/:lid/reset", h.PreviewResetLesson)
	t.GET("/exercises/:eid", h.PreviewView)
	t.POST("/exercises/:eid/attempt", h.PreviewStart)
	t.PUT("/attempts/:aid/answers/:qid", h.PreviewAnswer)
	t.POST("/attempts/:aid/finish", h.PreviewFinish)
	t.POST("/attempts/:aid/reset", h.PreviewResetAttempt)
}
