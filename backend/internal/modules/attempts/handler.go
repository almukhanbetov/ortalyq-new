package attempts

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

type Handler struct {
	svc *Service
	// frameAncestors are the web app origins that may show a PDF inside the page.
	frameAncestors []string
}

func NewHandler(svc *Service, frameAncestors []string) *Handler {
	return &Handler{svc: svc, frameAncestors: frameAncestors}
}

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func reply(c *gin.Context, v any, err error) {
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) LessonExercises(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDQuery(c, "student_id")
	if !ok {
		return
	}
	items, err := h.svc.LessonExercises(c.Request.Context(), principal(c), cid, lid, c.DefaultQuery("scope", "lesson"), sid)
	reply(c, gin.H{"items": items}, err)
}

func (h *Handler) View(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	eid, ok := httpx.UUIDParam(c, "eid")
	if !ok {
		return
	}
	sid, ok := httpx.UUIDQuery(c, "student_id")
	if !ok {
		return
	}
	v, err := h.svc.View(c.Request.Context(), principal(c), cid, eid, sid)
	reply(c, v, err)
}

func (h *Handler) Start(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	eid, ok := httpx.UUIDParam(c, "eid")
	if !ok {
		return
	}
	v, err := h.svc.Start(c.Request.Context(), principal(c), cid, eid)
	reply(c, v, err)
}

func (h *Handler) Reset(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	eid, ok := httpx.UUIDParam(c, "eid")
	if !ok {
		return
	}
	var req ResetRequest
	if !httpx.Bind(c, &req) {
		return
	}
	v, err := h.svc.Reset(c.Request.Context(), principal(c), cid, eid, req.StudentID)
	reply(c, v, err)
}

func (h *Handler) Answer(c *gin.Context) {
	aid, ok := httpx.UUIDParam(c, "aid")
	if !ok {
		return
	}
	qid, ok := httpx.UUIDParam(c, "qid")
	if !ok {
		return
	}
	var req AnswerRequest
	if !httpx.Bind(c, &req) {
		return
	}
	v, err := h.svc.Answer(c.Request.Context(), principal(c), aid, qid, req)
	reply(c, v, err)
}

func (h *Handler) Finish(c *gin.Context) {
	aid, ok := httpx.UUIDParam(c, "aid")
	if !ok {
		return
	}
	v, err := h.svc.Finish(c.Request.Context(), principal(c), aid)
	reply(c, v, err)
}

// ---- teacher preview (stage 8) ----

func (h *Handler) PreviewLesson(c *gin.Context) {
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	items, err := h.svc.PreviewLesson(c.Request.Context(), principal(c), lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) PreviewResetLesson(c *gin.Context) {
	lid, ok := httpx.UUIDParam(c, "lid")
	if !ok {
		return
	}
	items, err := h.svc.PreviewResetLesson(c.Request.Context(), principal(c), lid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) PreviewView(c *gin.Context) {
	eid, ok := httpx.UUIDParam(c, "eid")
	if !ok {
		return
	}
	v, err := h.svc.PreviewView(c.Request.Context(), principal(c), eid)
	reply(c, v, err)
}

func (h *Handler) PreviewStart(c *gin.Context) {
	eid, ok := httpx.UUIDParam(c, "eid")
	if !ok {
		return
	}
	v, err := h.svc.PreviewStart(c.Request.Context(), principal(c), eid)
	reply(c, v, err)
}

func (h *Handler) PreviewAnswer(c *gin.Context) {
	aid, ok := httpx.UUIDParam(c, "aid")
	if !ok {
		return
	}
	qid, ok := httpx.UUIDParam(c, "qid")
	if !ok {
		return
	}
	var req AnswerRequest
	if !httpx.Bind(c, &req) {
		return
	}
	v, err := h.svc.PreviewAnswer(c.Request.Context(), principal(c), aid, qid, req)
	reply(c, v, err)
}

func (h *Handler) PreviewFinish(c *gin.Context) {
	aid, ok := httpx.UUIDParam(c, "aid")
	if !ok {
		return
	}
	v, err := h.svc.PreviewFinish(c.Request.Context(), principal(c), aid)
	reply(c, v, err)
}

func (h *Handler) PreviewResetAttempt(c *gin.Context) {
	aid, ok := httpx.UUIDParam(c, "aid")
	if !ok {
		return
	}
	v, err := h.svc.PreviewResetAttempt(c.Request.Context(), principal(c), aid)
	reply(c, v, err)
}

// File serves the PDF of a «PDF-документ» block to a member of the class who may see the block.
func (h *Handler) File(c *gin.Context) {
	cid, ok := httpx.UUIDParam(c, "cid")
	if !ok {
		return
	}
	eid, ok := httpx.UUIDParam(c, "eid")
	if !ok {
		return
	}
	file, f, err := h.svc.ClassFile(c.Request.Context(), principal(c), cid, eid)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	uploads.ServePDF(c, f, file.Name, file.CreatedAt, h.frameAncestors)
}
