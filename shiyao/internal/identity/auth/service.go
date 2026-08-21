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
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

const (
	authTransactionTTL = 10 * time.Minute
	authChallengeTTL   = 10 * time.Minute
	sessionTTL         = 30 * 24 * time.Hour

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
// The transaction is intentionally separate from the eventual session.
// It represents the short-lived process of proving that the person controls
// the requested authentication method.
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
		// Do not leak whether an account exists.
		//
		// The exact handling of sql.ErrNoRows should be added here depending
		// on your sqlc error behavior.
		//
		// For now, return the database error.
		return sqlc.AuthTransaction{}, err
	}

	// Your generated CreateAuthTransactionParams may have slightly different
	// fields depending on your migration/query definitions.
	//
	// The transaction should contain:
	//   - email / identifier
	//   - user ID
	//   - expiration
	//   - initial state
	//
	// Adjust these fields to match the generated sqlc struct.
	transaction, err := s.repo.CreateAuthTransaction(
		ctx,
		sqlc.CreateAuthTransactionParams{
			UserID:     &user.ID,
			Identifier: email,
			ExpiresAt:  time.Now().Add(authTransactionTTL),
		},
	)
	if err != nil {
		return sqlc.AuthTransaction{}, err
	}

	return transaction, nil
}

// -----------------------------------------------------------------------------
// Password authentication
// -----------------------------------------------------------------------------

// LoginWithPassword verifies a user's password and authenticates the
// authentication transaction.
//
// The transaction must already have been created by Start.
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

	if transaction.ExpiresAt.Before(time.Now()) {
		_, _ = s.repo.ExpireAuthTransaction(ctx, transactionID)
		return sqlc.User{}, ErrTransactionExpired
	}

	if transaction.UserID == nil {
		return sqlc.User{}, ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByID(ctx, *transaction.UserID)
	if err != nil {
		return sqlc.User{}, ErrInvalidCredentials
	}

	if user.DisabledAt != nil {
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

// SetPassword sets the password for an authenticated user.
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

// -----------------------------------------------------------------------------
// One-time codes
// -----------------------------------------------------------------------------

// SendOTP creates a one-time authentication challenge.
//
// This method creates the challenge. The actual email/SMS delivery should
// happen outside the repository.
//
// You can later inject a Mailer into Service.
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

	if transaction.ExpiresAt.Before(time.Now()) {
		_, _ = s.repo.ExpireAuthTransaction(ctx, transactionID)
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
			Identifier: transaction.Identifier,
			TokenHash:  tokenHash,
			ExpiresAt:  time.Now().Add(authChallengeTTL),
			Purpose:    "otp",
		},
	)
	if err != nil {
		return "", err
	}

	_ = challenge

	// IMPORTANT:
	//
	// Do not return `code` from the production HTTP handler.
	//
	// This return value is useful while developing/testing the auth flow.
	// Once you add a Mailer, this becomes:
	//
	//     s.mailer.SendLoginCode(ctx, transaction.Identifier, code)
	//
	// and this method should return only an error.
	return code, nil
}

// VerifyOTP verifies the current one-time code.
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

	if transaction.ExpiresAt.Before(time.Now()) {
		_, _ = s.repo.ExpireAuthTransaction(ctx, transactionID)
		return sqlc.User{}, ErrTransactionExpired
	}

	challenge, err := s.repo.GetActiveAuthChallenge(
		ctx,
		sqlc.GetActiveAuthChallengeParams{
			Identifier: transaction.Identifier,
			Purpose:    "otp",
		},
	)
	if err != nil {
		return sqlc.User{}, ErrInvalidChallenge
	}

	if challenge.ExpiresAt.Before(time.Now()) {
		return sqlc.User{}, ErrChallengeExpired
	}

	if challenge.ConsumedAt != nil {
		return sqlc.User{}, ErrChallengeConsumed
	}

	if challenge.Attempts >= otpMaxAttempts {
		return sqlc.User{}, ErrTooManyAttempts
	}

	if hashToken(code) != challenge.TokenHash {
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
		// New user.
		//
		// This is where you create the user after successfully proving
		// ownership of the email address.
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

	if user.DisabledAt != nil {
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

// -----------------------------------------------------------------------------
// Sessions
// -----------------------------------------------------------------------------

// CreateSession creates a persistent login session after authentication has
// succeeded.
//
// The raw token should be given to the client as a secure cookie.
// Only the hash is stored in PostgreSQL.
func (s *Service) CreateSession(
	ctx context.Context,
	userID uuid.UUID,
	ipAddress *string,
	userAgent *string,
) (string, sqlc.Session, error) {
	token, err := generateSessionToken()
	if err != nil {
		return "", sqlc.Session{}, err
	}

	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(sessionTTL)

	var ip *string
	if ipAddress != nil {
		ip = ipAddress
	}

	session, err := s.repo.CreateSession(
		ctx,
		sqlc.CreateSessionParams{
			UserID:    userID,
			TokenHash: tokenHash,
			IPAddress: ip,
			UserAgent: userAgent,
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		return "", sqlc.Session{}, err
	}

	return token, session, nil
}

func (s *Service) GetSession(
	ctx context.Context,
	token string,
) (sqlc.Session, error) {
	if token == "" {
		return sqlc.Session{}, errors.New("session token is required")
	}

	return s.repo.GetSessionByTokenHash(
		ctx,
		hashToken(token),
	)
}

func (s *Service) Logout(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
) error {
	return s.repo.RevokeSession(
		ctx,
		sqlc.RevokeSessionParams{
			ID:     sessionID,
			UserID: userID,
		},
	)
}

func (s *Service) LogoutAll(
	ctx context.Context,
	userID uuid.UUID,
) error {
	return s.repo.RevokeUserSessions(ctx, userID)
}

// -----------------------------------------------------------------------------
// OAuth
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

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

func generateSessionToken() (string, error) {
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

	// NOTE:
	// This is a placeholder configuration. In production, tune Argon2id
	// parameters according to your server hardware and security requirements.

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
	// This should parse the encoded Argon2id string and recompute the hash.
	//
	// Keep this helper isolated so the password implementation can later
	// be replaced with a dedicated password hasher.

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
