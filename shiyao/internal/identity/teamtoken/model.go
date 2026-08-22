//go:build cloud

package teamtoken

import (
	"time"

	"github.com/google/uuid"
)

type CreateRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type Response struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreateResponse struct {
	Response
	Token string `json:"token"`
}
