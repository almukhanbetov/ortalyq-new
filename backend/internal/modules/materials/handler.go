package materials

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/uploads"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func filterFrom(c *gin.Context) LessonFilter {
	d, _ := strconv.Atoi(c.Query("max_duration"))
	return LessonFilter{
		Query: c.Query("q"), Language: c.Query("language"), Age: c.Query("age"), Level: c.Query("level"),
		LessonType: c.Query("lesson_type"), Skill: c.Query("skill"), MaxDuration: d, Grammar: c.Query("grammar"),
		Vocabulary: c.Query("vocabulary"), Function: c.Query("function"), Tag: c.Query("tag"),
	}
}

func principal(c *gin.Context) auth.Principal { p, _ := auth.PrincipalFrom(c); return p }

func (h *Handler) ListCourses(c *gin.Context) {
	pg := httpx.PageQuery(c, 50, 100)
	items, total, err := h.svc.ListCourses(c.Request.Context(), principal(c), filterFrom(c), pg.Limit, pg.Offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.List[Course]{Items: items, Total: total, Limit: pg.Limit, Offset: pg.Offset})
}

func (h *Handler) GetCourse(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	course, err := h.svc.GetCourse(c.Request.Context(), principal(c), id, filterFrom(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, course)
}

func (h *Handler) CreateCourse(c *gin.Context) {
	var req CourseRequest
	if !httpx.Bind(c, &req) {
		return
	}
	course, err := h.svc.CreateCourse(c.Request.Context(), principal(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, course)
}

func (h *Handler) UpdateCourse(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req CourseRequest
	if !httpx.Bind(c, &req) {
		return
	}
	course, err := h.svc.UpdateCourse(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, course)
}

func (h *Handler) ReorderCourses(c *gin.Context) {
	var req ReorderRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.ReorderCourses(c.Request.Context(), principal(c), req.IDs); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req LessonRequest
	if !httpx.Bind(c, &req) {
		return
	}
	l, err := h.svc.CreateLesson(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, l)
}

func (h *Handler) ReorderLessons(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req ReorderRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.ReorderLessons(c.Request.Context(), principal(c), id, req.IDs); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	l, err := h.svc.GetLesson(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, l)
}

func (h *Handler) UpdateLesson(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req LessonRequest
	if !httpx.Bind(c, &req) {
		return
	}
	l, err := h.svc.UpdateLesson(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, l)
}

func (h *Handler) CreateSection(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req SectionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	s, err := h.svc.CreateSection(c.Request.Context(), principal(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h *Handler) UpdateSection(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req SectionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.UpdateSection(c.Request.Context(), principal(c), id, req); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ReorderSections(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	var req ReorderRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.svc.ReorderSections(c.Request.Context(), principal(c), id, req.IDs); err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Options(c *gin.Context) {
	o, err := h.svc.Options(c.Request.Context(), principal(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

// UploadCover takes a cover picture of the material form (multipart field "file").
func (h *Handler) UploadCover(c *gin.Context) {
	// The body is cut a little above the limit; the file store checks the exact size and the content.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploads.ImageMaxBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "too large") {
			httpx.Fail(c, uploads.ErrImageTooLarge())
			return
		}
		httpx.Fail(c, apperr.Validation(map[string]string{"file": "Выберите картинку"}))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	cover, err := h.svc.UploadCover(c.Request.Context(), principal(c), fh.Size, f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, cover)
}

// Cover serves a cover picture to its uploader.
func (h *Handler) Cover(c *gin.Context) {
	id, ok := httpx.UUIDParam(c, "id")
	if !ok {
		return
	}
	cover, f, err := h.svc.OpenCover(c.Request.Context(), principal(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	w := c.Writer.Header()
	w.Set("Content-Type", cover.MimeType)
	w.Set("Content-Length", strconv.FormatInt(cover.SizeBytes, 10))
	w.Set("Content-Disposition", `inline; filename="cover"`)
	w.Set("X-Content-Type-Options", "nosniff")
	w.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Set("Cache-Control", "private, max-age=86400, immutable") // a cover id never changes its picture
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}
