package auth

import (
	"errors"
	"strings"
	"unicode/utf8"

	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidTransaction = errors.New("invalid authentication transaction")
	ErrTransactionExpired = errors.New("authentication transaction expired")
	ErrInvalidChallenge   = errors.New("invalid authentication challenge")
	ErrChallengeExpired   = errors.New("authentication challenge expired")
	ErrChallengeConsumed  = errors.New("authentication challenge already consumed")
	ErrTooManyAttempts    = errors.New("too many attempts")
	ErrInvalidCode        = errors.New("invalid verification code")
	ErrPasswordNotSet     = errors.New("password not set")
	ErrPasswordAlreadySet = errors.New("password already set")
	ErrUserDisabled       = errors.New("user disabled")
)

const (
	minPasswordLength = 8
	maxPasswordLength = 128

	otpCodeLength = 6
)

// ValidateStart validates the email used to start an authentication flow.
func ValidateStart(req startRequest) error {
	email := strings.TrimSpace(req.Email)

	if email == "" {
		return apperrors.NewBadRequest("email is required")
	}

	if utf8.RuneCountInString(email) > 254 {
		return apperrors.NewBadRequest("email is too long")
	}

	return nil
}

// ValidatePasswordLogin validates a password login request.
func ValidatePasswordLogin(req passwordLoginRequest) error {
	if strings.TrimSpace(req.TransactionID) == "" {
		return apperrors.NewBadRequest("transaction_id is required")
	}

	if req.Password == "" {
		return apperrors.NewBadRequest("password is required")
	}

	return validatePassword(req.Password)
}

// ValidateSendOTP validates an OTP send request.
func ValidateSendOTP(req sendOTPRequest) error {
	if strings.TrimSpace(req.TransactionID) == "" {
		return apperrors.NewBadRequest("transaction_id is required")
	}

	return nil
}

// ValidateVerifyOTP validates an OTP verification request.
func ValidateVerifyOTP(req verifyOTPRequest) error {
	if strings.TrimSpace(req.TransactionID) == "" {
		return apperrors.NewBadRequest("transaction_id is required")
	}

	code := strings.TrimSpace(req.Code)

	if code == "" {
		return apperrors.NewBadRequest("code is required")
	}

	if len(code) != otpCodeLength {
		return apperrors.NewBadRequest("code must be 6 digits")
	}

	for _, char := range code {
		if char < '0' || char > '9' {
			return apperrors.NewBadRequest("code must contain only digits")
		}
	}

	return nil
}

// ValidateSetPassword validates a password enrollment request.
func ValidateSetPassword(req setPasswordRequest) error {
	if req.Password == "" {
		return apperrors.NewBadRequest("password is required")
	}

	return validatePassword(req.Password)
}

// validatePassword contains password policy shared by login/enrollment.
func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)

	if length < minPasswordLength {
		return apperrors.NewBadRequest(
			"password must be at least 8 characters",
		)
	}

	if length > maxPasswordLength {
		return apperrors.NewBadRequest(
			"password must not exceed 128 characters",
		)
	}

	return nil
}
