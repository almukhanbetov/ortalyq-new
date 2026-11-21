package materials

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/httpx"
	"ortalyq/backend/internal/modules/auth"
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
