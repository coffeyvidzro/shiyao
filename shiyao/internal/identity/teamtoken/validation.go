//go:build cloud

package teamtoken

import (
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
)

const (
	maxNameLength  = 100
	maxScopes      = 32
	maxScopeLength = 100
)

func ValidateCreateRequest(req CreateRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return apperrors.NewBadRequest("name is required")
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return apperrors.NewBadRequest("name must be 100 characters or fewer")
	}
	if len(req.Scopes) > maxScopes {
		return apperrors.NewBadRequest("too many scopes")
	}
	for _, scope := range req.Scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || utf8.RuneCountInString(scope) > maxScopeLength {
			return apperrors.NewBadRequest("invalid scope")
		}
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		return apperrors.NewBadRequest("expires_at must be in the future")
	}
	return nil
}
