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

const (
	sessionTTL = 30 * 24 * time.Hour
)

var (
	ErrInvalidSession = errors.New("invalid session")
)

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
	if userID == uuid.Nil {
		return "", sqlc.Session{}, errors.New("user id is required")
	}

	token, err := generateToken()
	if err != nil {
		return "", sqlc.Session{}, err
	}

	expiresAt := time.Now().Add(sessionTTL)

	session, err := s.repo.Create(
		ctx,
		sqlc.CreateSessionParams{
			UserID:    userID,
			TokenHash: hashToken(token),
			IpAddress: parseIP(ipAddress),
			UserAgent: userAgent,
			ExpiresAt: pgconv.NullableTimestamptz(&expiresAt),
		},
	)
	if err != nil {
		return "", sqlc.Session{}, err
	}

	return token, session, nil
}

func (s *Service) Get(
	ctx context.Context,
	token string,
) (sqlc.Session, error) {
	if token == "" {
		return sqlc.Session{}, ErrInvalidSession
	}

	session, err := s.repo.GetByTokenHash(
		ctx,
		hashToken(token),
	)
	if err != nil {
		return sqlc.Session{}, ErrInvalidSession
	}

	if !session.ExpiresAt.Valid {
		return sqlc.Session{}, ErrInvalidSession
	}

	if time.Now().After(session.ExpiresAt.Time) {
		return sqlc.Session{}, ErrInvalidSession
	}

	return session, nil
}

func (s *Service) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]sqlc.Session, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	return s.repo.ListByUserID(ctx, userID)
}

func (s *Service) Revoke(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
) error {
	if sessionID == uuid.Nil {
		return errors.New("session id is required")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	return s.repo.Revoke(
		ctx,
		sqlc.RevokeSessionParams{
			ID:     sessionID,
			UserID: userID,
		},
	)
}

func (s *Service) RevokeAll(
	ctx context.Context,
	userID uuid.UUID,
) error {
	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	return s.repo.RevokeUserSessions(
		ctx,
		userID,
	)
}

func parseIP(value *string) *netip.Addr {
	if value == nil {
		return nil
	}

	if *value == "" {
		return nil
	}

	ip, err := netip.ParseAddr(*value)
	if err != nil {
		return nil
	}

	return &ip
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
