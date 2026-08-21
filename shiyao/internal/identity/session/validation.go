package session

import apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"

func NewBadRequestError(message string) error {
	return apperrors.NewBadRequest(message)
}

func NewUnauthorizedError(message string) error {
	return apperrors.NewUnauthorized(message)
}
