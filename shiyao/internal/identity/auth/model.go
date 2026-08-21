package auth

import "github.com/google/uuid"

// -----------------------------------------------------------------------------
// Requests
// -----------------------------------------------------------------------------

type startRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type passwordLoginRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	Password      string `json:"password" binding:"required"`
}

type sendOTPRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
}

type verifyOTPRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	Code          string `json:"code" binding:"required"`
}

type setPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// -----------------------------------------------------------------------------
// Responses
// -----------------------------------------------------------------------------

type startResponse struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Methods       []string  `json:"methods"`
}

type authenticationResponse struct {
	UserID        uuid.UUID `json:"user_id"`
	SessionExpiry string    `json:"session_expires_at"`
}

type sendOTPResponse struct {
	TransactionID uuid.UUID `json:"transaction_id"`
}
