package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	sessionpkg "github.com/coffeyvidzro/shiyao/internal/identity/session"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
)

type Handler struct {
	service        *Service
	sessionService *sessionpkg.Service
}

func NewHandler(service *Service, sessionService *sessionpkg.Service) *Handler {
	return &Handler{service: service, sessionService: sessionService}
}

func (h *Handler) Start(c *gin.Context) {
	var req startRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}
	transaction, err := h.service.Start(c.Request.Context(), normalizeEmail(req.Email))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.OK(c, startResponse{TransactionID: transaction.ID, Methods: authenticationMethods()})
}

func (h *Handler) LoginWithPassword(c *gin.Context) {
	var req passwordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}
	transactionID, err := parseUUID(req.TransactionID)
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid transaction_id"))
		return
	}
	user, err := h.service.LoginWithPassword(c.Request.Context(), transactionID, req.Password)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	h.createSession(c, user.ID)
}

func (h *Handler) SendOTP(c *gin.Context) {
	var req sendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}
	transactionID, err := parseUUID(req.TransactionID)
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid transaction_id"))
		return
	}
	if _, err := h.service.SendOTP(c.Request.Context(), transactionID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.OK(c, sendOTPResponse{TransactionID: transactionID})
}

func (h *Handler) VerifyOTP(c *gin.Context) {
	var req verifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}
	transactionID, err := parseUUID(req.TransactionID)
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid transaction_id"))
		return
	}
	user, err := h.service.VerifyOTP(c.Request.Context(), transactionID, strings.TrimSpace(req.Code))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	h.createSession(c, user.ID)
}

func (h *Handler) SetPassword(c *gin.Context) {
	var req setPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}
	user, err := h.service.SetPassword(c.Request.Context(), userID, req.Password)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.OK(c, authenticationResponse{UserID: user.ID})
}

func (h *Handler) createSession(c *gin.Context, userID uuid.UUID) {
	token, sess, err := h.sessionService.Create(c.Request.Context(), userID, clientIP(c), userAgent(c))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	sessionpkg.SetCookie(c, token, sess.ExpiresAt.Time)
	httputil.OK(c, authenticationResponse{UserID: userID, SessionExpiry: sess.ExpiresAt.Time.Format(http.TimeFormat)})
}

func parseUUID(value string) (uuid.UUID, error) { return uuid.Parse(strings.TrimSpace(value)) }

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

func authenticationMethods() []string { return []string{"password", "otp"} }
