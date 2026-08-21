package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

const (
	authTransactionTTL = 10 * time.Minute
	authChallengeTTL   = 10 * time.Minute
	otpLength          = 6
	otpMaxAttempts     = 5
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) Start(
	ctx context.Context,
	email string,
) (sqlc.AuthTransaction, error) {
	email = normalizeEmail(email)

	if email == "" {
		return sqlc.AuthTransaction{}, errors.New("email is required")
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return sqlc.AuthTransaction{}, err
	}

	transaction, err := s.repo.CreateAuthTransaction(
		ctx,
		sqlc.CreateAuthTransactionParams{
			UserID:     &user.ID,
			Identifier: email,
			ExpiresAt:  pgconv.NullableTimestamptz(timePtr(time.Now().Add(authTransactionTTL))),
		},
	)
	if err != nil {
		return sqlc.AuthTransaction{}, err
	}

	return transaction, nil
}

func (s *Service) LoginWithPassword(
	ctx context.Context,
	transactionID uuid.UUID,
	password string,
) (sqlc.User, error) {
	transaction, err := s.repo.GetAuthTransactionByID(
		ctx,
		transactionID,
	)
	if err != nil {
		return sqlc.User{}, ErrInvalidTransaction
	}

	if pgconv.TimestamptzToTime(transaction.ExpiresAt).Before(time.Now()) {
		_ = s.repo.ExpireAuthTransaction(ctx, transactionID)
		return sqlc.User{}, ErrTransactionExpired
	}

	if transaction.UserID == nil {
		return sqlc.User{}, ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByID(ctx, *transaction.UserID)
	if err != nil {
		return sqlc.User{}, ErrInvalidCredentials
	}

	if user.DisabledAt.Valid {
		return sqlc.User{}, ErrUserDisabled
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return sqlc.User{}, ErrPasswordNotSet
	}

	if !verifyPassword(password, *user.PasswordHash) {
		return sqlc.User{}, ErrInvalidCredentials
	}

	_, err = s.repo.MarkAuthTransactionAuthenticated(
		ctx,
		transactionID,
	)
	if err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

func (s *Service) SetPassword(
	ctx context.Context,
	userID uuid.UUID,
	password string,
) (sqlc.User, error) {
	if err := validatePassword(password); err != nil {
		return sqlc.User{}, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return sqlc.User{}, err
	}

	return s.repo.SetUserPassword(
		ctx,
		sqlc.SetUserPasswordParams{
			ID:           userID,
			PasswordHash: &hash,
		},
	)
}

func (s *Service) SendOTP(
	ctx context.Context,
	transactionID uuid.UUID,
) (string, error) {
	transaction, err := s.repo.GetAuthTransactionByID(
		ctx,
		transactionID,
	)
	if err != nil {
		return "", ErrInvalidTransaction
	}

	if pgconv.TimestamptzToTime(transaction.ExpiresAt).Before(time.Now()) {
		_ = s.repo.ExpireAuthTransaction(ctx, transactionID)
		return "", ErrTransactionExpired
	}

	code, err := generateOTP()
	if err != nil {
		return "", err
	}

	tokenHash := hashToken(code)

	challenge, err := s.repo.CreateAuthChallenge(
		ctx,
		sqlc.CreateAuthChallengeParams{
			Identifier:        transaction.Identifier,
			SecretHash:        tokenHash,
			ExpiresAt:         pgconv.NullableTimestamptz(timePtr(time.Now().Add(authChallengeTTL))),
			Purpose:           "otp",
			AuthTransactionID: &transactionID,
			MaxAttempts:       otpMaxAttempts,
		},
	)
	if err != nil {
		return "", err
	}

	_ = challenge

	return code, nil
}

func (s *Service) VerifyOTP(
	ctx context.Context,
	transactionID uuid.UUID,
	code string,
) (sqlc.User, error) {
	transaction, err := s.repo.GetAuthTransactionByID(
		ctx,
		transactionID,
	)
	if err != nil {
		return sqlc.User{}, ErrInvalidTransaction
	}

	if pgconv.TimestamptzToTime(transaction.ExpiresAt).Before(time.Now()) {
		_ = s.repo.ExpireAuthTransaction(ctx, transactionID)
		return sqlc.User{}, ErrTransactionExpired
	}

	challenge, err := s.repo.GetActiveAuthChallenge(
		ctx,
		sqlc.GetActiveAuthChallengeParams{
			AuthTransactionID: &transactionID,
			Purpose:           "otp",
		},
	)
	if err != nil {
		return sqlc.User{}, ErrInvalidChallenge
	}

	if pgconv.TimestamptzToTime(challenge.ExpiresAt).Before(time.Now()) {
		return sqlc.User{}, ErrChallengeExpired
	}

	if challenge.ConsumedAt.Valid {
		return sqlc.User{}, ErrChallengeConsumed
	}

	if challenge.Attempts >= otpMaxAttempts {
		return sqlc.User{}, ErrTooManyAttempts
	}

	if hashToken(code) != challenge.SecretHash {
		_, _ = s.repo.IncrementAuthChallengeAttempts(
			ctx,
			challenge.ID,
		)

		return sqlc.User{}, ErrInvalidCode
	}

	_, err = s.repo.ConsumeAuthChallenge(
		ctx,
		challenge.ID,
	)
	if err != nil {
		return sqlc.User{}, err
	}

	var user sqlc.User

	if transaction.UserID != nil {
		user, err = s.repo.GetUserByID(
			ctx,
			*transaction.UserID,
		)
		if err != nil {
			return sqlc.User{}, err
		}
	} else {
		user, err = s.repo.CreateUser(
			ctx,
			sqlc.CreateUserParams{
				Email: emailFromIdentifier(transaction.Identifier),
			},
		)
		if err != nil {
			return sqlc.User{}, err
		}
	}

	if user.DisabledAt.Valid {
		return sqlc.User{}, ErrUserDisabled
	}

	_, err = s.repo.MarkUserEmailVerified(
		ctx,
		user.ID,
	)
	if err != nil {
		return sqlc.User{}, err
	}

	_, err = s.repo.MarkAuthTransactionAuthenticated(
		ctx,
		transactionID,
	)
	if err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

func (s *Service) GetOAuthAccount(
	ctx context.Context,
	provider string,
	providerUID string,
) (sqlc.OauthAccount, error) {
	if provider == "" {
		return sqlc.OauthAccount{}, errors.New("oauth provider is required")
	}

	if providerUID == "" {
		return sqlc.OauthAccount{}, errors.New("oauth provider user ID is required")
	}

	return s.repo.GetOAuthAccount(
		ctx,
		sqlc.GetOAuthAccountParams{
			Provider:    provider,
			ProviderUid: providerUID,
		},
	)
}

func (s *Service) CreateOAuthAccount(
	ctx context.Context,
	userID uuid.UUID,
	provider string,
	providerUID string,
) (sqlc.OauthAccount, error) {
	if provider != "google" && provider != "github" {
		return sqlc.OauthAccount{}, errors.New("unsupported oauth provider")
	}

	if providerUID == "" {
		return sqlc.OauthAccount{}, errors.New("oauth provider user ID is required")
	}

	return s.repo.CreateOAuthAccount(
		ctx,
		sqlc.CreateOAuthAccountParams{
			UserID:      userID,
			Provider:    provider,
			ProviderUid: providerUID,
		},
	)
}

func (s *Service) ListOAuthAccounts(
	ctx context.Context,
	userID uuid.UUID,
) ([]sqlc.OauthAccount, error) {
	return s.repo.ListOAuthAccountsByUserID(
		ctx,
		userID,
	)
}

func (s *Service) DeleteOAuthAccount(
	ctx context.Context,
	userID uuid.UUID,
	accountID uuid.UUID,
) error {
	return s.repo.DeleteOAuthAccount(
		ctx,
		sqlc.DeleteOAuthAccountParams{
			ID:     accountID,
			UserID: userID,
		},
	)
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func emailFromIdentifier(identifier string) string {
	return normalizeEmail(identifier)
}

func generateOTP() (string, error) {
	var value uint32

	b := make([]byte, 4)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	value = uint32(b[0])<<24 |
		uint32(b[1])<<16 |
		uint32(b[2])<<8 |
		uint32(b[3])

	value %= 1_000_000

	return fmt.Sprintf("%06d", value), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	if len(password) > 128 {
		return errors.New("password must not exceed 128 characters")
	}

	return nil
}

func hashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, 16)

	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		3,
		64*1024,
		4,
		32,
	)

	return fmt.Sprintf(
		"argon2id$v=19$m=65536,t=3,p=4$%s$%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(hash),
	), nil
}

func verifyPassword(password string, encoded string) bool {

	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return false
	}

	if parts[0] != "argon2id" {
		return false
	}

	var salt []byte

	var err error

	salt, err = hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	expected, err := hex.DecodeString(parts[4])
	if err != nil {
		return false
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		3,
		64*1024,
		4,
		32,
	)

	if len(actual) != len(expected) {
		return false
	}

	var diff byte

	for i := range actual {
		diff |= actual[i] ^ expected[i]
	}

	return diff == 0
}
