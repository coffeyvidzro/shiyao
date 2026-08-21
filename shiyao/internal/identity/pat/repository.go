package pat

import (
	"context"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/google/uuid"
)

type Repository struct {
	q *sqlc.Queries
}

func NewRepository(q *sqlc.Queries) *Repository {
	return &Repository{q: q}
}

func (r *Repository) Create(
	ctx context.Context,
	arg sqlc.CreateTokenParams,
) (sqlc.PersonalAccessToken, error) {
	return r.q.CreateToken(ctx, arg)
}

func (r *Repository) GetByHash(
	ctx context.Context,
	tokenHash string,
) (sqlc.PersonalAccessToken, error) {
	return r.q.GetTokenByHash(ctx, tokenHash)
}

func (r *Repository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]sqlc.ListTokensByUserRow, error) {
	return r.q.ListTokensByUser(ctx, userID)
}

func (r *Repository) Revoke(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) error {
	return r.q.RevokeToken(ctx, sqlc.RevokeTokenParams{ID: id, UserID: userID})
}

func (r *Repository) Touch(
	ctx context.Context,
	id uuid.UUID,
) error {
	return r.q.TouchToken(ctx, id)
}
