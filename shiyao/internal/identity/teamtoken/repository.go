//go:build cloud

package teamtoken

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

func (r *Repository) Create(ctx context.Context, arg sqlc.CreateTeamTokenParams) (sqlc.TeamAccessToken, error) {
	return r.q.CreateTeamToken(ctx, arg)
}

func (r *Repository) List(ctx context.Context, teamID, userID uuid.UUID) ([]sqlc.ListTeamTokensRow, error) {
	return r.q.ListTeamTokens(ctx, sqlc.ListTeamTokensParams{TeamID: teamID, UserID: userID})
}

func (r *Repository) Revoke(ctx context.Context, teamID, userID, tokenID uuid.UUID) (int64, error) {
	return r.q.RevokeTeamToken(ctx, sqlc.RevokeTeamTokenParams{ID: tokenID, TeamID: teamID, UserID: userID})
}
