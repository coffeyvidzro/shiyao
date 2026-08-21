package session

import (
	"net/http"
)

func NewBadRequestError(message string) error {
	return &APIError{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: message}
}
func NewUnauthorizedError(message string) error {
	return &APIError{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: message}
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }
