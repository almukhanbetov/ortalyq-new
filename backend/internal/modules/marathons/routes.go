package marathons

import (
	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/modules/auth"
)

func (h *Handler) Register(authed *gin.RouterGroup) {
	t := authed.Group("", auth.RequireRole(auth.RoleTeacher))
	t.GET("/marathon-languages", h.Languages)
	t.POST("/marathons", h.Create)
	t.PATCH("/marathons/:cid", h.Update)
	t.POST("/marathons/:cid/archive", h.Archive)
	t.PATCH("/marathons/:cid/settings", h.SetPassage)
	t.POST("/marathons/:cid/lessons", h.AddLesson)
	t.PUT("/marathons/:cid/lessons/order", h.ReorderLessons)
	t.PATCH("/marathons/:cid/lessons/:lid", h.UpdateLesson)
	t.DELETE("/marathons/:cid/lessons/:lid", h.RemoveLesson)
	t.POST("/marathons/:cid/lessons/:lid/copy", h.CopyLesson)
	t.PUT("/marathons/:cid/lessons/:lid/open-at", h.SetLessonOpenAt)
	t.POST("/marathons/:cid/lessons/:lid/unlock", h.UnlockLesson)
	t.POST("/marathons/:cid/mode", h.ChangeMode)
	t.PUT("/marathons/:cid/review-settings", h.SetReviewSettings)
	t.PUT("/marathons/:cid/stats-settings", h.SetStatsSettings)
	t.GET("/marathons/:cid/stats", h.Stats)
	t.PUT("/marathons/:cid/lessons/:lid/stats-comment", h.SetLessonComment)
	t.POST("/marathons/:cid/lessons/:lid/review", h.ReviewDecision)
	t.GET("/marathons/:cid/students", h.Students)
	t.POST("/marathons/:cid/students", h.AddStudents)
	t.GET("/marathons/:cid/invitations", h.Invitations)
	t.POST("/marathons/:cid/invitations", h.Invite)
	t.POST("/marathons/:cid/invitations/:iid/resend", h.ResendInvitation)
	t.DELETE("/marathons/:cid/invitations/:iid", h.RevokeInvitation)
	t.PUT("/marathons/:cid/students/active", h.SetStudentsActive)
	t.DELETE("/marathons/:cid/students/:sid", h.RemoveStudent)
	t.GET("/marathons/:cid/students/:sid/progress", h.StudentProgress)
	t.GET("/marathons/:cid/students/:sid/lessons/:lid", h.ReviewLesson)
	// Appearance (stage 10): pictures take effect at once; reactions and background are saved with the settings.
	t.POST("/marathons/:cid/images/:slot", h.UploadImage)
	t.DELETE("/marathons/:cid/images/:slot", h.RemoveImage)
	t.POST("/marathons/:cid/lessons/:lid/images/:slot", h.UploadImage)
	t.DELETE("/marathons/:cid/lessons/:lid/images/:slot", h.RemoveImage)
	t.GET("/marathons/:cid/appearance", h.AppearanceSettings)
	t.PUT("/marathons/:cid/appearance", h.SetAppearance)
	t.GET("/marathons/:cid/lessons/:lid/appearance", h.LessonAppearance)
	t.PUT("/marathons/:cid/lessons/:lid/appearance", h.SetLessonAppearance)

	// Teacher and enrolled students; the service checks membership and, for students, the end date.
	authed.GET("/marathons/:cid/lessons", h.Lessons)
	authed.GET("/marathons/:cid/me", h.Me)
	authed.POST("/marathons/:cid/lessons/:lid/complete", h.CompleteLesson)
	authed.POST("/marathons/:cid/lessons/:lid/submit", h.SubmitLesson)
	// Statistics (stage 7): the owner, or a participant with open access when the settings allow it.
	authed.GET("/marathons/:cid/rating", h.Rating)
	authed.GET("/marathons/:cid/lessons/:lid/results", h.LessonResults)
	authed.GET("/marathons/:cid/me/results", h.MyResults)
	// What the lesson page applies (reactions, background): enrolled students and the owner (preview).
	authed.GET("/marathons/:cid/lessons/:lid/appearance/effective", h.EffectiveAppearance)
	// Accepting an invitation needs the invited student signed in.
	authed.POST("/invitations/:token/accept", h.AcceptInvitation)
}

// RegisterPublic adds what works without signing in: the page of an invitation link (masked, no personal data).
func (h *Handler) RegisterPublic(api *gin.RouterGroup) {
	api.GET("/invitations/:token", h.InvitationInfo)
}
