package session

import (
	"context"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/google/uuid"
)

type Repository struct{ q *sqlc.Queries }

func NewRepository(q *sqlc.Queries) *Repository { return &Repository{q: q} }

func (r *Repository) Create(ctx context.Context, arg sqlc.CreateSessionParams) (sqlc.Session, error) {
	return r.q.CreateSession(ctx, arg)
}
func (r *Repository) GetByTokenHash(ctx context.Context, tokenHash string) (sqlc.Session, error) {
	return r.q.GetSessionByTokenHash(ctx, tokenHash)
}
func (r *Repository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]sqlc.Session, error) {
	return r.q.ListSessionsByUserID(ctx, userID)
}
func (r *Repository) Revoke(ctx context.Context, arg sqlc.RevokeSessionParams) error {
	return r.q.RevokeSession(ctx, arg)
}
func (r *Repository) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	return r.q.RevokeUserSessions(ctx, userID)
}
