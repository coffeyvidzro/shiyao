package users

import (
	"strings"
	"unicode/utf8"

	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
)

const maxNameLength = 100

func ValidateUpdateMe(req UpdateMeRequest) error {
	if req.Name == nil {
		return nil
	}

	name := strings.TrimSpace(*req.Name)

	if name == "" {
		return apperrors.NewBadRequest("name cannot be empty")
	}

	if utf8.RuneCountInString(name) > maxNameLength {
		return apperrors.NewBadRequest("name must be 100 characters or fewer")
	}

	return nil
}
