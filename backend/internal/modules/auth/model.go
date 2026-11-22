// Package auth implements login, logout, sessions and the request principal.
package auth

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher" // school teacher: created by an administrator, who assigns students to them
	// RolePrivateTeacher registered on their own (sign-up). Authoring rights of a teacher, no school part:
	// students cannot be assigned to this role by an administrator.
	RolePrivateTeacher = "private_teacher"
	RoleStudent        = "student"
)

// TeachingRoles may author materials, exercises and marathons and open the teacher cabinet.
var TeachingRoles = []string{RoleTeacher, RolePrivateTeacher}

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

// CanTeach is true for a school teacher and for a private teacher (owner-scoped authoring).
func (p Principal) CanTeach() bool { return p.Role == RoleTeacher || p.Role == RolePrivateTeacher }

type userRecord struct {
	Principal
	PasswordHash string
	DisabledAt   *time.Time
}
