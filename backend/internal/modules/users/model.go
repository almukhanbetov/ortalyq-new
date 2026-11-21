// Package users implements admin user management and teacher assignments.
package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Role        string     `json:"role"`
	AvatarColor string     `json:"avatar_color"`
	DisabledAt  *time.Time `json:"disabled_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Assignment struct {
	Teacher   User      `json:"teacher"`
	Student   User      `json:"student"`
	CreatedAt time.Time `json:"created_at"`
}
