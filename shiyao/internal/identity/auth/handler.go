package auth

import (
	"net/http"
	"strings"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	sessionpkg "github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
)

type Handler struct {
	service        *Service
	sessionService *sessionpkg.Service
}

func NewHandler(service *Service, sessionService *sessionpkg.Service) *Handler {
	return &Handler{
		service:        service,
		sessionService: sessionService,
	}
}

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

	httputil.OK(c, startResponse{
		TransactionID: transaction.ID,
		Methods:       authenticationMethods(transaction),
	})
}

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

	token, sess, err := h.sessionService.Create(
		c.Request.Context(),
		user.ID,
		clientIP(c),
		userAgent(c),
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sessionpkg.SetCookie(
		c,
		token,
		sess.ExpiresAt.Time,
	)

	httputil.OK(c, authenticationResponse{
		UserID:        user.ID,
		SessionExpiry: sess.ExpiresAt.Time.Format(http.TimeFormat),
	})
}

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

	token, sess, err := h.sessionService.Create(
		c.Request.Context(),
		user.ID,
		clientIP(c),
		userAgent(c),
	)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sessionpkg.SetCookie(
		c,
		token,
		sess.ExpiresAt.Time,
	)

	httputil.OK(c, authenticationResponse{
		UserID:        user.ID,
		SessionExpiry: sess.ExpiresAt.Time.Format(http.TimeFormat),
	})
}

func (h *Handler) SetPassword(c *gin.Context) {
	var req setPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(
			c,
			NewBadRequestError("invalid request body"),
		)
		return
	}

	userID, err := sessionpkg.UserIDFromContext(c)
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

func authenticationMethods(
	transaction sqlc.AuthTransaction,
) []string {

	return []string{
		"password",
		"otp",
	}
}

func NewBadRequestError(message string) error {
	return apperrors.NewBadRequest(message)
}

func NewUnauthorizedError(message string) error {
	return apperrors.NewUnauthorized(message)
}
