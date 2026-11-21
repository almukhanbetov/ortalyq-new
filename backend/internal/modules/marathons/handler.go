package marathons

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) Languages(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"items": Languages}) }

func (h *Handler) Create(c *gin.Context) {
	var req SaveRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.Create(c.Request.Context(), principal(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req SaveRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.Update(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) Archive(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	if err := h.svc.Archive(c.Request.Context(), principal(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SetPassage(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req PassageRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.SetPassage(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) Lessons(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.Lessons(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) AddLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req AddLessonRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.AddLesson(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CompleteLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	out, err := h.svc.CompleteLesson(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Students(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	list, err := h.svc.Students(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *Handler) AddStudents(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req AddStudentsRequest
	if !httpx.Bind(c, &req) {
		return
	}
	list, err := h.svc.AddStudents(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *Handler) StudentProgress(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDParam(c, "sid")
	if !ok {
		return
	}
	out, err := h.svc.StudentProgress(c.Request.Context(), principal(c), id, sid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ReviewLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDParam(c, "sid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	out, err := h.svc.ReviewLesson(c.Request.Context(), principal(c), id, sid, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// lessonParams reads :cid and :lid.
func lessonParams(c *gin.Context) (id, lid uuid.UUID, ok bool) {
	if id, ok = httpx.UUIDParam(c, "cid"); !ok {
		return
	}
	lid, ok = httpx.UUIDParam(c, "lid")
	return
}

func (h *Handler) UpdateLesson(c *gin.Context) {
	id, lid, ok := lessonParams(c)
	if !ok {
		return
	}
	var req LessonFields
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.UpdateLesson(c.Request.Context(), principal(c), id, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ReorderLessons(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req ReorderLessonsRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.ReorderLessons(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) RemoveLesson(c *gin.Context) {
	id, lid, ok := lessonParams(c)
	if !ok {
		return
	}
	out, err := h.svc.RemoveLesson(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CopyLesson(c *gin.Context) {
	id, lid, ok := lessonParams(c)
	if !ok {
		return
	}
	out, err := h.svc.CopyLesson(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) SetStudentsActive(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req SetActiveRequest
	if !httpx.Bind(c, &req) {
		return
	}
	list, err := h.svc.SetStudentsActive(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *Handler) RemoveStudent(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDParam(c, "sid")
	if !ok {
		return
	}
	list, err := h.svc.RemoveStudent(c.Request.Context(), principal(c), id, sid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *Handler) Me(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.Me(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SetLessonOpenAt(c *gin.Context) {
	id, lid, ok := lessonParams(c)
	if !ok {
		return
	}
	var req OpenAtRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.SetLessonOpenAt(c.Request.Context(), principal(c), id, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) UnlockLesson(c *gin.Context) {
	id, lid, ok := lessonParams(c)
	if !ok {
		return
	}
	var req UnlockRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.UnlockLesson(c.Request.Context(), principal(c), id, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ChangeMode(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req ModeRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.ChangeMode(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) SubmitLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	out, err := h.svc.SubmitLesson(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ReviewDecision(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	var req ReviewRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.Review(c.Request.Context(), principal(c), id, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SetReviewSettings(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req ReviewSettingsRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.SetReviewSettings(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) SetStatsSettings(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req StatsSettingsRequest
	if !httpx.Bind(c, &req) {
		return
	}
	m, err := h.svc.SetStatsSettings(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) Stats(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.Stats(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Rating(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.Rating(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) LessonResults(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	out, err := h.svc.LessonResults(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SetLessonComment(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	var req CommentRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.SetLessonComment(c.Request.Context(), principal(c), id, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"comment": out})
}

func (h *Handler) MyResults(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.MyResults(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ---- Invitations (stage 9) ----

func (h *Handler) Invite(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req InviteRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.Invite(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) Invitations(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.Invitations(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) ResendInvitation(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	iid, ok := httpx.UUIDParam(c, "iid")
	if !ok {
		return
	}
	out, err := h.svc.ResendInvitation(c.Request.Context(), principal(c), id, iid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) RevokeInvitation(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	iid, ok := httpx.UUIDParam(c, "iid")
	if !ok {
		return
	}
	out, err := h.svc.RevokeInvitation(c.Request.Context(), principal(c), id, iid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// validToken accepts only the shape of our tokens (43 base64url characters), so junk never reaches the DB.
func validToken(t string) bool {
	if len(t) != 43 {
		return false
	}
	for _, r := range t {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (h *Handler) InvitationInfo(c *gin.Context) {
	t := c.Param("token")
	if !validToken(t) {
		httpx.Fail(c, apperr.NotFound("Приглашение"))
		return
	}
	out, err := h.svc.InvitationInfo(c.Request.Context(), t)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) AcceptInvitation(c *gin.Context) {
	t := c.Param("token")
	if !validToken(t) {
		httpx.Fail(c, apperr.NotFound("Приглашение"))
		return
	}
	out, err := h.svc.AcceptInvitation(c.Request.Context(), principal(c), t)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ---- Appearance (stage 10) ----

// lessonParam reads the optional :lid of the lesson image and appearance routes.
func lessonParam(c *gin.Context) (*uuid.UUID, bool) {
	if c.Param("lid") == "" {
		return nil, true
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return nil, false
	}
	return &lid, true
}

func (h *Handler) UploadImage(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := lessonParam(c)
	if !ok {
		return
	}
	// The body is cut a little above the limit; the service checks the exact size and the content.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploads.ImageMaxBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "too large") {
			httpx.Fail(c, uploads.ErrImageTooLarge())
			return
		}
		httpx.Fail(c, apperr.Validation(map[string]string{"file": "Прикрепите картинку"}))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	out, err := h.svc.UploadImage(c.Request.Context(), principal(c), id, lid, c.Param("slot"), fh.Filename, fh.Size, f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) RemoveImage(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := lessonParam(c)
	if !ok {
		return
	}
	if err := h.svc.RemoveImage(c.Request.Context(), principal(c), id, lid, c.Param("slot")); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) AppearanceSettings(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	out, err := h.svc.AppearanceSettings(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SetAppearance(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	var req AppearanceRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.SetAppearance(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) LessonAppearance(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	out, err := h.svc.LessonAppearance(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SetLessonAppearance(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	var req AppearanceRequest
	if !httpx.Bind(c, &req) {
		return
	}
	out, err := h.svc.SetLessonAppearance(c.Request.Context(), principal(c), id, lid, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) EffectiveAppearance(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	out, err := h.svc.EffectiveAppearance(c.Request.Context(), principal(c), id, lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
