package teamtoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	tokenPrefix = "shiyao_tat_"
	prefixLen   = 12
)

var (
	ErrInvalidTeam   = errors.New("invalid team")
	ErrTokenNotFound = errors.New("team access token not found")
)

var defaultScopes = []string{"sandbox:read", "sandbox:write"}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, teamID, userID uuid.UUID, req CreateRequest) (CreateResponse, error) {
	if teamID == uuid.Nil || userID == uuid.Nil {
		return CreateResponse{}, ErrInvalidTeam
	}
	if err := ValidateCreateRequest(req); err != nil {
		return CreateResponse{}, err
	}

	token, err := generateToken()
	if err != nil {
		return CreateResponse{}, err
	}
	created, err := s.repo.Create(ctx, sqlc.CreateTeamTokenParams{
		TeamID:      teamID,
		Name:        strings.TrimSpace(req.Name),
		TokenHash:   hashToken(token),
		TokenPrefix: tokenPrefixValue(token),
		Scopes:      normalizeScopes(req.Scopes),
		ExpiresAt:   pgconv.NullableTimestamptz(req.ExpiresAt),
		CreatedBy:   userID,
	})
	if err != nil {
		return CreateResponse{}, err
	}
	return CreateResponse{Response: responseFromToken(created), Token: token}, nil
}

func (s *Service) List(ctx context.Context, teamID, userID uuid.UUID) ([]Response, error) {
	if teamID == uuid.Nil || userID == uuid.Nil {
		return nil, ErrInvalidTeam
	}
	rows, err := s.repo.List(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}
	items := make([]Response, 0, len(rows))
	for _, row := range rows {
		items = append(items, responseFromListRow(row))
	}
	return items, nil
}

func (s *Service) Revoke(ctx context.Context, teamID, userID, tokenID uuid.UUID) error {
	if teamID == uuid.Nil || userID == uuid.Nil || tokenID == uuid.Nil {
		return ErrTokenNotFound
	}
	deleted, err := s.repo.Revoke(ctx, teamID, userID, tokenID)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrTokenNotFound
	}
	return nil
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

func responseFromToken(row sqlc.TeamAccessToken) Response {
	return Response{
		ID: row.ID, Name: row.Name, Prefix: row.TokenPrefix, Scopes: row.Scopes,
		ExpiresAt: pgconv.TimestamptzToTimePtr(row.ExpiresAt), LastUsedAt: pgconv.TimestamptzToTimePtr(row.LastUsedAt),
		CreatedBy: row.CreatedBy, CreatedAt: timeFromTimestamptz(row.CreatedAt),
	}
}

func responseFromListRow(row sqlc.ListTeamTokensRow) Response {
	return Response{
		ID: row.ID, Name: row.Name, Prefix: row.TokenPrefix, Scopes: row.Scopes,
		ExpiresAt: pgconv.TimestamptzToTimePtr(row.ExpiresAt), LastUsedAt: pgconv.TimestamptzToTimePtr(row.LastUsedAt),
		CreatedBy: row.CreatedBy, CreatedAt: timeFromTimestamptz(row.CreatedAt),
	}
}

func timeFromTimestamptz(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
