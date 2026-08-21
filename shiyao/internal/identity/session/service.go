package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/netip"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/google/uuid"
)

const TTL = 30 * 24 * time.Hour

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	ipAddress *string,
	userAgent *string,
) (string, sqlc.Session, error) {
	token, err := generateToken()
	if err != nil {
		return "", sqlc.Session{}, err
	}

	expiresAt := time.Now().Add(TTL)
	ip := parseIP(ipAddress)

	sess, err := s.repo.Create(
		ctx,
		sqlc.CreateSessionParams{
			TokenHash: hashToken(token),
			IpAddress: ip,
			UserAgent: userAgent,
			ExpiresAt: pgconv.NullableTimestamptz(&expiresAt),
			UserID:    userID,
		},
	)
	if err != nil {
		return "", sqlc.Session{}, err
	}

	return token, sess, nil
}

func (s *Service) Get(ctx context.Context, token string) (sqlc.Session, error) {
	if token == "" {
		return sqlc.Session{}, errors.New("session token is required")
	}

	return s.repo.GetByTokenHash(ctx, hashToken(token))
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]sqlc.Session, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *Service) Revoke(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error {
	return s.repo.Revoke(ctx, sqlc.RevokeSessionParams{
		ID:     sessionID,
		UserID: userID,
	})
}

func (s *Service) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	return s.repo.RevokeUserSessions(ctx, userID)
}

func parseIP(value *string) *netip.Addr {
	if value == nil || *value == "" {
		return nil
	}

	parsed, err := netip.ParseAddr(*value)
	if err != nil {
		return nil
	}

	return &parsed
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
