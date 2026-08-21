package pat

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	tokenPrefix = "shiyao_pat_"
	prefixLen   = 12
)

var (
	ErrInvalidToken  = errors.New("invalid personal access token")
	ErrTokenNotFound = errors.New("personal access token not found")
)

var defaultScopes = []string{"sandbox:read", "sandbox:write"}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (CreateResponse, error) {
	if userID == uuid.Nil {
		return CreateResponse{}, ErrTokenNotFound
	}
	if err := ValidateCreateRequest(req); err != nil {
		return CreateResponse{}, err
	}

	token, err := generateToken()
	if err != nil {
		return CreateResponse{}, err
	}

	created, err := s.repo.Create(ctx, sqlc.CreateTokenParams{
		UserID:      userID,
		Name:        strings.TrimSpace(req.Name),
		TokenHash:   hashToken(token),
		TokenPrefix: tokenPrefixValue(token),
		Scopes:      normalizeScopes(req.Scopes),
		ExpiresAt:   pgconv.NullableTimestamptz(req.ExpiresAt),
	})
	if err != nil {
		return CreateResponse{}, err
	}

	return CreateResponse{Response: responseFromRow(created), Token: token}, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Response, error) {
	if userID == uuid.Nil {
		return nil, ErrTokenNotFound
	}

	rows, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]Response, 0, len(rows))
	for _, row := range rows {
		items = append(items, responseFromListRow(row))
	}
	return items, nil
}

func (s *Service) Revoke(ctx context.Context, userID, tokenID uuid.UUID) error {
	if userID == uuid.Nil || tokenID == uuid.Nil {
		return ErrTokenNotFound
	}
	return s.repo.Revoke(ctx, tokenID, userID)
}

func (s *Service) Resolve(ctx context.Context, input authn.CredentialInput) (authn.Principal, error) {
	if input.Type != authn.CredentialPAT || strings.TrimSpace(input.Value) == "" {
		return authn.Principal{}, authn.ErrInvalidCredential
	}

	row, err := s.repo.GetByHash(ctx, hashToken(input.Value))
	if err != nil {
		return authn.Principal{}, authn.ErrInvalidCredential
	}
	if err := s.repo.Touch(ctx, row.ID); err != nil {
		return authn.Principal{}, err
	}

	return authn.Principal{
		Subject:    authn.Subject{ID: row.UserID, Type: authn.SubjectUser},
		Credential: authn.Credential{ID: row.ID, Type: authn.CredentialPAT},
		Assurance:  authn.AssuranceUnknown,
	}, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return tokenPrefix + hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func tokenPrefixValue(token string) string {
	if len(token) <= prefixLen {
		return token
	}
	return token[:prefixLen]
}

func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return append([]string(nil), defaultScopes...)
	}

	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		normalized = append(normalized, strings.TrimSpace(scope))
	}
	return normalized
}

func responseFromRow(row sqlc.PersonalAccessToken) Response {
	return Response{
		ID:         row.ID,
		Name:       row.Name,
		Prefix:     row.TokenPrefix,
		Scopes:     row.Scopes,
		ExpiresAt:  pgconv.TimestamptzToTimePtr(row.ExpiresAt),
		LastUsedAt: pgconv.TimestamptzToTimePtr(row.LastUsedAt),
		CreatedAt:  timeFromTimestamptz(row.CreatedAt),
	}
}

func responseFromListRow(row sqlc.ListTokensByUserRow) Response {
	return Response{
		ID:         row.ID,
		Name:       row.Name,
		Prefix:     row.TokenPrefix,
		Scopes:     row.Scopes,
		ExpiresAt:  pgconv.TimestamptzToTimePtr(row.ExpiresAt),
		LastUsedAt: pgconv.TimestamptzToTimePtr(row.LastUsedAt),
		CreatedAt:  timeFromTimestamptz(row.CreatedAt),
	}
}

func timeFromTimestamptz(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

var _ authn.Resolver = (*Service)(nil)
