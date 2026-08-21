package users

import (
	"context"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/google/uuid"
)

type Repository struct {
	q *sqlc.Queries
}

func NewRepository(q *sqlc.Queries) *Repository {
	return &Repository{
		q: q,
	}
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *Repository) UpdateProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return r.q.UpdateUserProfile(ctx, arg)
}

func (r *Repository) Disable(ctx context.Context, id uuid.UUID) error {
	return r.q.DisableUser(ctx, id)
}
