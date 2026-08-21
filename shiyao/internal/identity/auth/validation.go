package auth

import (
	"errors"
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
