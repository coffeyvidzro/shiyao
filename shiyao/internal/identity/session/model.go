package session

import "github.com/google/uuid"

type Response struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	UserAgent  *string   `json:"user_agent,omitempty"`
	ExpiresAt  string    `json:"expires_at"`
	LastSeenAt string    `json:"last_seen_at,omitempty"`
	CreatedAt  string    `json:"created_at"`
}
