package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

const (
	authTransactionTTL = 10 * time.Minute
	authChallengeTTL   = 10 * time.Minute

	otpLength      = 6
	otpMaxAttempts = 5
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// -----------------------------------------------------------------------------
// Authentication
// -----------------------------------------------------------------------------

// Start begins an authentication transaction for an email address.
//
// The transaction is intentionally short-lived. The client receives the
// transaction ID and can then choose one of the available authentication
// methods.
func (s *Service) Start(
	ctx context.Context,
	email string,
) (sqlc.AuthTransaction, error) {
	email = normalizeEmail(email)

	if email == "" {
		return sqlc.AuthTransaction{}, apperrors.NewBadRequest(
			"email is required",
		)
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// We don't expose whether the email exists.
		//
		// For the current implementation, however, the transaction requires
		// a user. This can later be changed to support passwordless account
		// creation without revealing account existence.
		return sqlc.AuthTransaction{}, err
	}

	expiresAt := time.Now().Add(authTransactionTTL)

	transaction, err := s.repo.CreateAuthTransaction(
		ctx,
		sqlc.CreateAuthTransactionParams{
			UserID:     &user.ID,
			Identifier: email,
			ExpiresAt: pgconv.NullableTimestamptz(
				&expiresAt,
			),
		},
	)
	if err != nil {
		return sqlc.AuthTransaction{}, err
	}

	return transaction, nil
}

// LoginWithPassword authenticates a user using the password associated with
// the authentication transaction.
func (s *Service) LoginWithPassword(
	ctx context.Context,
	transactionID uuid.UUID,
	password string,
) (sqlc.User, error) {
	transaction, err := s.getValidTransaction(ctx, transactionID)
	if err != nil {
		return sqlc.User{}, err
	}

	if transaction.UserID == nil {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"invalid credentials",
		)
	}

	user, err := s.repo.GetUserByID(ctx, *transaction.UserID)
	if err != nil {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"invalid credentials",
		)
	}

	if user.DisabledAt.Valid {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"account is disabled",
		)
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return sqlc.User{}, apperrors.NewBadRequest(
			"password is not enrolled",
		)
	}

	if !verifyPassword(password, *user.PasswordHash) {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"invalid credentials",
		)
	}

	if _, err := s.repo.MarkAuthTransactionAuthenticated(
		ctx,
		transactionID,
	); err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

// SetPassword enrolls or replaces the password for an authenticated user.
func (s *Service) SetPassword(
	ctx context.Context,
	userID uuid.UUID,
	password string,
) (sqlc.User, error) {
	if userID == uuid.Nil {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"authentication required",
		)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return sqlc.User{}, err
	}

	user, err := s.repo.SetUserPassword(
		ctx,
		sqlc.SetUserPasswordParams{
			ID:           userID,
			PasswordHash: &hash,
		},
	)
	if err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

// -----------------------------------------------------------------------------
// OTP
// -----------------------------------------------------------------------------

// SendOTP creates a short-lived OTP challenge for an authentication
// transaction.
//
// The returned code is currently useful for development/testing. In
// production, this value should be passed to an email/SMS delivery service
// instead of being returned to the HTTP handler.
func (s *Service) SendOTP(
	ctx context.Context,
	transactionID uuid.UUID,
) (string, error) {
	transaction, err := s.getValidTransaction(ctx, transactionID)
	if err != nil {
		return "", err
	}

	code, err := generateOTP()
	if err != nil {
		return "", apperrors.NewInternal(
			"failed to generate authentication code",
		)
	}

	expiresAt := time.Now().Add(authChallengeTTL)

	_, err = s.repo.CreateAuthChallenge(
		ctx,
		sqlc.CreateAuthChallengeParams{
			Identifier: transaction.Identifier,
			SecretHash: hashToken(code),
			ExpiresAt: pgconv.NullableTimestamptz(
				&expiresAt,
			),
			Purpose:           "otp",
			AuthTransactionID: &transactionID,
			MaxAttempts:       otpMaxAttempts,
		},
	)
	if err != nil {
		return "", err
	}

	return code, nil
}

// VerifyOTP verifies the active OTP challenge and authenticates the
// corresponding transaction.
func (s *Service) VerifyOTP(
	ctx context.Context,
	transactionID uuid.UUID,
	code string,
) (sqlc.User, error) {
	transaction, err := s.getValidTransaction(ctx, transactionID)
	if err != nil {
		return sqlc.User{}, err
	}

	challenge, err := s.repo.GetActiveAuthChallenge(
		ctx,
		sqlc.GetActiveAuthChallengeParams{
			AuthTransactionID: &transactionID,
			Purpose:           "otp",
		},
	)
	if err != nil {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"invalid or expired authentication code",
		)
	}

	if challenge.ConsumedAt.Valid {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"authentication code has already been used",
		)
	}

	if pgconv.TimestamptzToTime(challenge.ExpiresAt).Before(time.Now()) {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"authentication code has expired",
		)
	}

	if challenge.Attempts >= otpMaxAttempts {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"too many authentication attempts",
		)
	}

	code = strings.TrimSpace(code)

	expectedHash := hashToken(code)

	if subtle.ConstantTimeCompare(
		[]byte(expectedHash),
		[]byte(challenge.SecretHash),
	) != 1 {
		_, _ = s.repo.IncrementAuthChallengeAttempts(
			ctx,
			challenge.ID,
		)

		return sqlc.User{}, apperrors.NewUnauthorized(
			"invalid authentication code",
		)
	}

	if _, err := s.repo.ConsumeAuthChallenge(
		ctx,
		challenge.ID,
	); err != nil {
		return sqlc.User{}, err
	}

	user, err := s.getOrCreateOTPUser(
		ctx,
		transaction,
	)
	if err != nil {
		return sqlc.User{}, err
	}

	if user.DisabledAt.Valid {
		return sqlc.User{}, apperrors.NewUnauthorized(
			"account is disabled",
		)
	}

	if _, err := s.repo.MarkUserEmailVerified(
		ctx,
		user.ID,
	); err != nil {
		return sqlc.User{}, err
	}

	if _, err := s.repo.MarkAuthTransactionAuthenticated(
		ctx,
		transactionID,
	); err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

// -----------------------------------------------------------------------------
// OAuth
// -----------------------------------------------------------------------------

func (s *Service) GetOAuthAccount(
	ctx context.Context,
	provider string,
	providerUID string,
) (sqlc.OauthAccount, error) {
	provider = normalizeOAuthProvider(provider)
	providerUID = strings.TrimSpace(providerUID)

	if provider == "" {
		return sqlc.OauthAccount{}, apperrors.NewBadRequest(
			"oauth provider is required",
		)
	}

	if providerUID == "" {
		return sqlc.OauthAccount{}, apperrors.NewBadRequest(
			"oauth provider user ID is required",
		)
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
	if userID == uuid.Nil {
		return sqlc.OauthAccount{}, apperrors.NewUnauthorized(
			"authentication required",
		)
	}

	provider = normalizeOAuthProvider(provider)
	providerUID = strings.TrimSpace(providerUID)

	if !isSupportedOAuthProvider(provider) {
		return sqlc.OauthAccount{}, apperrors.NewBadRequest(
			"unsupported oauth provider",
		)
	}

	if providerUID == "" {
		return sqlc.OauthAccount{}, apperrors.NewBadRequest(
			"oauth provider user ID is required",
		)
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
	if userID == uuid.Nil {
		return nil, apperrors.NewUnauthorized(
			"authentication required",
		)
	}

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
	if userID == uuid.Nil {
		return apperrors.NewUnauthorized(
			"authentication required",
		)
	}

	if accountID == uuid.Nil {
		return apperrors.NewBadRequest(
			"invalid oauth account ID",
		)
	}

	return s.repo.DeleteOAuthAccount(
		ctx,
		sqlc.DeleteOAuthAccountParams{
			ID:     accountID,
			UserID: userID,
		},
	)
}

// -----------------------------------------------------------------------------
// Transaction helpers
// -----------------------------------------------------------------------------

func (s *Service) getValidTransaction(
	ctx context.Context,
	transactionID uuid.UUID,
) (sqlc.AuthTransaction, error) {
	if transactionID == uuid.Nil {
		return sqlc.AuthTransaction{}, apperrors.NewBadRequest(
			"invalid transaction_id",
		)
	}

	transaction, err := s.repo.GetAuthTransactionByID(
		ctx,
		transactionID,
	)
	if err != nil {
		return sqlc.AuthTransaction{}, apperrors.NewUnauthorized(
			"invalid authentication transaction",
		)
	}

	if transaction.ExpiresAt.Valid {
		expiresAt := pgconv.TimestamptzToTime(transaction.ExpiresAt)

		if expiresAt.Before(time.Now()) {
			_ = s.repo.ExpireAuthTransaction(
				ctx,
				transactionID,
			)

			return sqlc.AuthTransaction{}, apperrors.NewUnauthorized(
				"authentication transaction has expired",
			)
		}
	}

	return transaction, nil
}

func (s *Service) getOrCreateOTPUser(
	ctx context.Context,
	transaction sqlc.AuthTransaction,
) (sqlc.User, error) {
	if transaction.UserID != nil {
		user, err := s.repo.GetUserByID(
			ctx,
			*transaction.UserID,
		)
		if err != nil {
			return sqlc.User{}, err
		}

		return user, nil
	}

	email := normalizeEmail(transaction.Identifier)

	if email == "" {
		return sqlc.User{}, apperrors.NewBadRequest(
			"authentication identifier is missing",
		)
	}

	user, err := s.repo.CreateUser(
		ctx,
		sqlc.CreateUserParams{
			Email: email,
		},
	)
	if err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

// -----------------------------------------------------------------------------
// Password hashing
// -----------------------------------------------------------------------------

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)

	if _, err := rand.Read(salt); err != nil {
		return "", apperrors.NewInternal(
			"failed to generate password salt",
		)
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

	salt, err := hex.DecodeString(parts[3])
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

	return subtle.ConstantTimeCompare(
		actual,
		expected,
	) == 1
}

// -----------------------------------------------------------------------------
// OTP helpers
// -----------------------------------------------------------------------------

func generateOTP() (string, error) {
	var value uint32

	buffer := make([]byte, 4)

	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	value =
		uint32(buffer[0])<<24 |
			uint32(buffer[1])<<16 |
			uint32(buffer[2])<<8 |
			uint32(buffer[3])

	value %= 1_000_000

	return fmt.Sprintf("%0*d", otpLength, value), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// -----------------------------------------------------------------------------
// Normalization
// -----------------------------------------------------------------------------

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeOAuthProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func isSupportedOAuthProvider(provider string) bool {
	switch provider {
	case "google":
		return true
	default:
		return false
	}
}
