package users

import "github.com/google/uuid"

type UpdateMeRequest struct {
	Name *string `json:"name"`
}

type Response struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	Name          *string   `json:"name,omitempty"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
}
