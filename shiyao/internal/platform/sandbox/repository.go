package sandbox

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
	arg sqlc.CreateSandboxParams,
) (sqlc.Sandbox, error) {
	return r.q.CreateSandbox(ctx, arg)
}

func (r *Repository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.Sandbox, error) {
	return r.q.GetSandboxByID(ctx, id)
}

func (r *Repository) ListByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]sqlc.Sandbox, error) {
	return r.q.ListSandboxesByUser(ctx, userID)
}

func (r *Repository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	return r.q.DeleteSandbox(ctx, id)
}
