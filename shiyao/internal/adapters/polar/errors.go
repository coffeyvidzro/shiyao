package polar

import "errors"

var (
	ErrUnauthorized = errors.New("polar: unauthorized")
	ErrNotFound     = errors.New("polar: resource not found")
	ErrValidation   = errors.New("polar: validation failed")
	ErrUnavailable  = errors.New("polar: service unavailable")
)
