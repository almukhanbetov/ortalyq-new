// Package httpx contains small helpers shared by HTTP handlers.
package httpx

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
)

// Fail writes err in the unified format {"error": {...}}.
func Fail(c *gin.Context, err error) {
	if e, ok := apperr.As(err); ok {
		c.AbortWithStatusJSON(e.Status, gin.H{"error": e})
		return
	}
	slog.Error("internal error", "path", c.FullPath(), "err", err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": apperr.New(500, "internal", "Внутренняя ошибка сервера")})
}

func JSON(c *gin.Context, status int, v any) { c.JSON(status, v) }

// Bind decodes a JSON body; malformed bodies produce a 400.
func Bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		Fail(c, apperr.BadRequest("invalid_json", "Некорректное тело запроса"))
		return false
	}
	return true
}

// UUIDParam parses a path parameter as UUID; writes 404 on failure.
func UUIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		Fail(c, apperr.NotFound("Объект"))
		return uuid.Nil, false
	}
	return id, true
}

// UUIDQuery parses an optional query UUID.
func UUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	s := c.Query(name)
	if s == "" {
		return nil, true
	}
	id, err := uuid.Parse(s)
	if err != nil {
		Fail(c, apperr.BadRequest("invalid_param", "Некорректный параметр "+name))
		return nil, false
	}
	return &id, true
}

// Page reads limit/offset pagination with sane bounds.
type Page struct {
	Limit  int
	Offset int
}

func PageQuery(c *gin.Context, def, max int) Page {
	l, err := strconv.Atoi(c.Query("limit"))
	if err != nil || l <= 0 {
		l = def
	}
	if l > max {
		l = max
	}
	o, err := strconv.Atoi(c.Query("offset"))
	if err != nil || o < 0 {
		o = 0
	}
	return Page{Limit: l, Offset: o}
}

// List is the standard paginated response envelope.
type List[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
