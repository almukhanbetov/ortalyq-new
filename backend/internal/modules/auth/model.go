// Package auth implements login, logout, sessions and the request principal.
package auth

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleStudent = "student"
)

// Principal is the authenticated user attached to a request. Services take it
// as a plain value so they never depend on the HTTP framework.
type Principal struct {
	UserID      uuid.UUID
	Role        string
	Email       string
	FullName    string
	Locale      string
	Timezone    string
	AvatarColor string
}

func (p Principal) IsAdmin() bool   { return p.Role == RoleAdmin }
func (p Principal) IsTeacher() bool { return p.Role == RoleTeacher }
func (p Principal) IsStudent() bool { return p.Role == RoleStudent }

type userRecord struct {
	Principal
	PasswordHash string
	DisabledAt   *time.Time
}
