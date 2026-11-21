// Package apperr defines application errors mapped to a single API error format.
package apperr

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, msg string) *Error { return &Error{Status: status, Code: code, Message: msg} }

func BadRequest(code, msg string) *Error { return New(http.StatusBadRequest, code, msg) }
func Unauthorized() *Error             { return New(http.StatusUnauthorized, "unauthorized", "Требуется вход") }
func Forbidden() *Error                { return New(http.StatusForbidden, "forbidden", "Недостаточно прав") }
func NotFound(what string) *Error      { return New(http.StatusNotFound, "not_found", what+" не найден") }
func Conflict(code, msg string) *Error { return New(http.StatusConflict, code, msg) }
func TooMany(msg string) *Error        { return New(http.StatusTooManyRequests, "rate_limited", msg) }
func Unavailable(code, msg string) *Error {
	return New(http.StatusServiceUnavailable, code, msg)
}

// Validation builds a 422 error with per-field messages.
func Validation(fields map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: "Проверьте введённые данные", Fields: fields}
}

// V accumulates validation errors.
type V map[string]string

func (v V) Add(field, msg string) {
	if _, ok := v[field]; !ok {
		v[field] = msg
	}
}
func (v V) Check(cond bool, field, msg string) {
	if !cond {
		v.Add(field, msg)
	}
}
func (v V) Err() error {
	if len(v) == 0 {
		return nil
	}
	return Validation(v)
}

func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
