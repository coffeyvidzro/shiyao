package users

import (
	"context"
	"strings"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetMe(
	ctx context.Context,
	userID uuid.UUID,
) (sqlc.User, error) {
	return s.repo.GetByID(ctx, userID)
}

func (s *Service) UpdateMe(
	ctx context.Context,
	userID uuid.UUID,
	req UpdateMeRequest,
) (sqlc.User, error) {
	if err := ValidateUpdateMe(req); err != nil {
		return sqlc.User{}, err
	}

	name := normalizeOptional(req.Name)

	if name == nil {
		return s.repo.GetByID(ctx, userID)
	}

	return s.repo.UpdateProfile(ctx, sqlc.UpdateUserProfileParams{
		ID:   userID,
		Name: name,
	})
}

func (s *Service) DeleteMe(
	ctx context.Context,
	userID uuid.UUID,
) error {
	if userID == uuid.Nil {
		return apperrors.NewUnauthorized("authentication required")
	}

	return s.repo.Disable(ctx, userID)
}

func normalizeOptional(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)

	if normalized == "" {
		return nil
	}

	return &normalized
}
