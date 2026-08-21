package auth

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

func (r *Repository) GetUserByEmail(
	ctx context.Context,
	email string,
) (sqlc.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}

func (r *Repository) GetUserByID(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *Repository) CreateUser(
	ctx context.Context,
	arg sqlc.CreateUserParams,
) (sqlc.User, error) {
	return r.q.CreateUser(ctx, arg)
}

func (r *Repository) SetUserPassword(
	ctx context.Context,
	arg sqlc.SetUserPasswordParams,
) (sqlc.User, error) {
	return r.q.SetUserPassword(ctx, arg)
}

func (r *Repository) MarkUserEmailVerified(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.User, error) {
	return r.q.MarkUserEmailVerified(ctx, id)
}

func (r *Repository) CreateAuthTransaction(
	ctx context.Context,
	arg sqlc.CreateAuthTransactionParams,
) (sqlc.AuthTransaction, error) {
	return r.q.CreateAuthTransaction(ctx, arg)
}

func (r *Repository) GetAuthTransactionByID(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.AuthTransaction, error) {
	return r.q.GetAuthTransactionByID(ctx, id)
}

func (r *Repository) SetAuthTransactionMethod(
	ctx context.Context,
	arg sqlc.SetAuthTransactionMethodParams,
) (sqlc.AuthTransaction, error) {
	return r.q.SetAuthTransactionMethod(ctx, arg)
}

func (r *Repository) SetAuthTransactionState(
	ctx context.Context,
	arg sqlc.SetAuthTransactionStateParams,
) (sqlc.AuthTransaction, error) {
	return r.q.SetAuthTransactionState(ctx, arg)
}

func (r *Repository) SetAuthTransactionUser(
	ctx context.Context,
	arg sqlc.SetAuthTransactionUserParams,
) (sqlc.AuthTransaction, error) {
	return r.q.SetAuthTransactionUser(ctx, arg)
}

func (r *Repository) MarkAuthTransactionAuthenticated(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.AuthTransaction, error) {
	return r.q.MarkAuthTransactionAuthenticated(ctx, id)
}

func (r *Repository) ExpireAuthTransaction(
	ctx context.Context,
	id uuid.UUID,
) error {
	return r.q.ExpireAuthTransaction(ctx, id)
}

func (r *Repository) CreateAuthChallenge(
	ctx context.Context,
	arg sqlc.CreateAuthChallengeParams,
) (sqlc.AuthChallenge, error) {
	return r.q.CreateAuthChallenge(ctx, arg)
}

func (r *Repository) GetActiveAuthChallenge(
	ctx context.Context,
	arg sqlc.GetActiveAuthChallengeParams,
) (sqlc.AuthChallenge, error) {
	return r.q.GetActiveAuthChallenge(ctx, arg)
}

func (r *Repository) IncrementAuthChallengeAttempts(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.AuthChallenge, error) {
	return r.q.IncrementAuthChallengeAttempts(ctx, id)
}

func (r *Repository) ConsumeAuthChallenge(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.AuthChallenge, error) {
	return r.q.ConsumeAuthChallenge(ctx, id)
}

func (r *Repository) CreateOAuthAccount(
	ctx context.Context,
	arg sqlc.CreateOAuthAccountParams,
) (sqlc.OauthAccount, error) {
	return r.q.CreateOAuthAccount(ctx, arg)
}

func (r *Repository) GetOAuthAccount(
	ctx context.Context,
	arg sqlc.GetOAuthAccountParams,
) (sqlc.OauthAccount, error) {
	return r.q.GetOAuthAccount(ctx, arg)
}

func (r *Repository) GetOAuthAccountByID(
	ctx context.Context,
	id uuid.UUID,
) (sqlc.OauthAccount, error) {
	return r.q.GetOAuthAccountByID(ctx, id)
}

func (r *Repository) ListOAuthAccountsByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]sqlc.OauthAccount, error) {
	return r.q.ListOAuthAccountsByUserID(ctx, userID)
}

func (r *Repository) DeleteOAuthAccount(
	ctx context.Context,
	arg sqlc.DeleteOAuthAccountParams,
) error {
	return r.q.DeleteOAuthAccount(ctx, arg)
}
