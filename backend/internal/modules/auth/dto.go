package auth

import "github.com/google/uuid"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type MeResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	Locale      string    `json:"locale"`
	Timezone    string    `json:"timezone"`
	AvatarColor string    `json:"avatar_color"`
}

type UpdateMeRequest struct {
	Locale   *string `json:"locale"`
	Timezone *string `json:"timezone"`
}

func ToMe(p Principal) MeResponse {
	return MeResponse{ID: p.UserID, Email: p.Email, FullName: p.FullName, Role: p.Role, Locale: p.Locale, Timezone: p.Timezone, AvatarColor: p.AvatarColor}
}
