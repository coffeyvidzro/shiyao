package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/pkg/httputil"
)

// Handler handles authentication HTTP requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new authentication handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// -----------------------------------------------------------------------------
// Start authentication
// -----------------------------------------------------------------------------

// Start begins an authentication transaction.
//
// POST /auth/start
//
// Request:
//
//	{
//	    "email": "user@example.com"
//	}
//
// Response:
//
//	{
//	    "success": true,
//	    "data": {
//	        "transaction_id": "...",
//	        "methods": ["password", "otp"]
//	    }
//	}
func (h *Handler) Start(c *gin.Context) {
	var req startRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid request body"),
		)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	transaction, err := h.service.Start(
		c.Request.Context(),
		email,
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	// Do not expose database fields from AuthTransaction directly.
	httputil.OK(c, startResponse{
		TransactionID: transaction.ID,
		Methods:       authenticationMethods(transaction),
	})
}

// -----------------------------------------------------------------------------
// Password login
// -----------------------------------------------------------------------------

// LoginWithPassword authenticates a user using their password.
//
// POST /auth/password
//
// Request:
//
//	{
//	    "transaction_id": "...",
//	    "password": "password"
//	}
func (h *Handler) LoginWithPassword(c *gin.Context) {
	var req passwordLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid request body"),
		)
		return
	}

	transactionID, err := parseUUID(req.TransactionID)
	if err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid transaction_id"),
		)
		return
	}

	user, err := h.service.LoginWithPassword(
		c.Request.Context(),
		transactionID,
		req.Password,
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	token, session, err := h.service.CreateSession(
		c.Request.Context(),
		user.ID,
		clientIP(c),
		userAgent(c),
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	setSessionCookie(
		c,
		token,
		session.ExpiresAt,
	)

	httputil.OK(c, authenticationResponse{
		UserID:        user.ID,
		SessionExpiry: session.ExpiresAt.Format(http.TimeFormat),
	})
}

// -----------------------------------------------------------------------------
// OTP
// -----------------------------------------------------------------------------

// SendOTP sends a one-time authentication code.
//
// POST /auth/otp/send
//
// Request:
//
//	{
//	    "transaction_id": "..."
//	}
func (h *Handler) SendOTP(c *gin.Context) {
	var req sendOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid request body"),
		)
		return
	}

	transactionID, err := parseUUID(req.TransactionID)
	if err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid transaction_id"),
		)
		return
	}

	_, err = h.service.SendOTP(
		c.Request.Context(),
		transactionID,
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, sendOTPResponse{
		TransactionID: transactionID,
	})
}

// VerifyOTP verifies a one-time authentication code.
//
// POST /auth/otp/verify
//
// Request:
//
//	{
//	    "transaction_id": "...",
//	    "code": "123456"
//	}
func (h *Handler) VerifyOTP(c *gin.Context) {
	var req verifyOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid request body"),
		)
		return
	}

	transactionID, err := parseUUID(req.TransactionID)
	if err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid transaction_id"),
		)
		return
	}

	code := strings.TrimSpace(req.Code)

	user, err := h.service.VerifyOTP(
		c.Request.Context(),
		transactionID,
		code,
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	token, session, err := h.service.CreateSession(
		c.Request.Context(),
		user.ID,
		clientIP(c),
		userAgent(c),
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	setSessionCookie(
		c,
		token,
		session.ExpiresAt,
	)

	httputil.OK(c, authenticationResponse{
		UserID:        user.ID,
		SessionExpiry: session.ExpiresAt.Format(http.TimeFormat),
	})
}

// -----------------------------------------------------------------------------
// Password management
// -----------------------------------------------------------------------------

// SetPassword sets a password for the authenticated user.
//
// POST /auth/password/set
//
// This endpoint assumes authentication middleware has placed the user ID
// into the Gin context.
func (h *Handler) SetPassword(c *gin.Context) {
	var req setPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid request body"),
		)
		return
	}

	userID, err := userIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	user, err := h.service.SetPassword(
		c.Request.Context(),
		userID,
		req.Password,
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, gin.H{
		"user_id": user.ID,
	})
}

// -----------------------------------------------------------------------------
// Session
// -----------------------------------------------------------------------------

// Logout revokes the current session.
//
// POST /auth/logout
func (h *Handler) Logout(c *gin.Context) {
	sessionID, err := sessionIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	userID, err := userIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.service.Logout(
		c.Request.Context(),
		sessionID,
		userID,
	); err != nil {
		httputil.Error(c, err)
		return
	}

	clearSessionCookie(c)

	httputil.OK(c, gin.H{
		"message": "logged out",
	})
}

// LogoutAll revokes every active session belonging to the current user.
//
// POST /auth/logout-all
func (h *Handler) LogoutAll(c *gin.Context) {
	userID, err := userIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.service.LogoutAll(
		c.Request.Context(),
		userID,
	); err != nil {
		httputil.Error(c, err)
		return
	}

	clearSessionCookie(c)

	httputil.OK(c, gin.H{
		"message": "logged out from all sessions",
	})
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func parseUUID(value string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(value))
}

func clientIP(c *gin.Context) *string {
	ip := c.ClientIP()

	if ip == "" {
		return nil
	}

	return &ip
}

func userAgent(c *gin.Context) *string {
	value := c.GetHeader("User-Agent")

	if value == "" {
		return nil
	}

	return &value
}

func setSessionCookie(
	c *gin.Context,
	token string,
	expiresAt interface {
		Unix() int64
	},
) {
	maxAge := int(expiresAt.Unix() - c.Request.Context().Value(sessionNowKey{}).(int64))

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Domain:   "",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// -----------------------------------------------------------------------------
// Context helpers
// -----------------------------------------------------------------------------

const (
	contextUserID    = "auth.user_id"
	contextSessionID = "auth.session_id"
)

func userIDFromContext(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(contextUserID)

	if !exists {
		return uuid.Nil, NewUnauthorizedError("authentication required")
	}

	id, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, NewUnauthorizedError("invalid authentication context")
	}

	return id, nil
}

func sessionIDFromContext(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(contextSessionID)

	if !exists {
		return uuid.Nil, NewUnauthorizedError("authentication required")
	}

	id, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, NewUnauthorizedError("invalid authentication context")
	}

	return id, nil
}

// -----------------------------------------------------------------------------
// Authentication method helpers
// -----------------------------------------------------------------------------

func authenticationMethods(
	transaction sqlc.AuthTransaction,
) []string {
	// This should eventually be determined by the transaction state created
	// by Service.Start().
	//
	// Keep this helper here until your exact auth_transactions schema/model
	// is finalized.
	//
	// For now, password + otp are the supported authentication methods.

	return []string{
		"password",
		"otp",
	}
}

// -----------------------------------------------------------------------------
// Temporary API errors
// -----------------------------------------------------------------------------

// These helpers assume your pkg/errors package exposes AppError.
//
// If your existing package already has equivalent constructors, use those
// instead.

func NewBadRequestError(message string) error {
	return &APIError{
		Status:  http.StatusBadRequest,
		Code:    "BAD_REQUEST",
		Message: message,
	}
}

func NewUnauthorizedError(message string) error {
	return &APIError{
		Status:  http.StatusUnauthorized,
		Code:    "UNAUTHORIZED",
		Message: message,
	}
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}
