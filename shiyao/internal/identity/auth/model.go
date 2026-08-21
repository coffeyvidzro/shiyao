package auth

import "github.com/google/uuid"

type StartRequest struct {
	Email string `json:"email"`
}

type StartResponse struct {
	TransactionID string   `json:"transaction_id"`
	Methods       []string `json:"methods"`
}

type SendOTPRequest struct {
	TransactionID string `json:"transaction_id"`
}

type VerifyOTPRequest struct {
	TransactionID string `json:"transaction_id"`
	Code          string `json:"code"`
}

type PasswordLoginRequest struct {
	TransactionID string `json:"transaction_id"`
	Password      string `json:"password"`
}

type PasswordEnrollRequest struct {
	TransactionID string `json:"transaction_id"`
	Password      string `json:"password"`
}

type AuthResponse struct {
	UserID string `json:"user_id"`
}

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

type sessionResponse struct {
	UserID        uuid.UUID `json:"user_id"`
	SessionExpiry string    `json:"session_expires_at"`
}
