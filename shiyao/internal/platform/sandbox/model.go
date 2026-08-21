package sandbox

import "github.com/google/uuid"

type CreateRequest struct {
	Template       string   `json:"template"`
	VCPU           int32    `json:"vcpu"`
	MemoryMB       int32    `json:"memory_mb"`
	TimeoutSeconds int32    `json:"timeout_seconds"`
	AllowedHosts   []string `json:"allowed_hosts"`
}

type Response struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	VMID           string    `json:"vm_id"`
	Template       string    `json:"template"`
	Status         string    `json:"status"`
	VCPU           int32     `json:"vcpu"`
	MemoryMB       int32     `json:"memory_mb"`
	TimeoutSeconds int32     `json:"timeout_seconds"`
	AllowedHosts   []string  `json:"allowed_hosts"`
	CreatedAt      string    `json:"created_at"`
	StartedAt      string    `json:"started_at,omitempty"`
	StoppedAt      string    `json:"stopped_at,omitempty"`
	UpdatedAt      string    `json:"updated_at"`
}
