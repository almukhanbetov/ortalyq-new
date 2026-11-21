package users

import "github.com/google/uuid"

type CreateUserRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type UpdateUserRequest struct {
	FullName *string `json:"full_name"`
	Role     *string `json:"role"`
	Disabled *bool   `json:"disabled"`
	Password *string `json:"password"`
}

type AssignmentRequest struct {
	TeacherID uuid.UUID `json:"teacher_id"`
	StudentID uuid.UUID `json:"student_id"`
}

type ListFilter struct {
	Role   string
	Query  string
	Limit  int
	Offset int
}
