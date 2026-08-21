package session

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/helper"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
)

const sessionCookieName = "shiyao-session"

type Handler struct { service *Service }
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) userID(c *gin.Context) (uuid.UUID, error) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok { return uuid.Nil, apperrors.NewUnauthorized("authentication required") }
	return userID, nil
}

func (h *Handler) sessionID(c *gin.Context) (uuid.UUID, error) {
	principal, ok := authn.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Credential.Type != authn.CredentialSession || principal.Credential.ID == uuid.Nil {
		return uuid.Nil, apperrors.NewUnauthorized("session authentication required")
	}
	return principal.Credential.ID, nil
}

func (h *Handler) List(c *gin.Context) {
	userID, err := h.userID(c); if err != nil { httputil.Error(c, err); return }
	sessions, err := h.service.List(c.Request.Context(), userID); if err != nil { httputil.Error(c, err); return }
	items := make([]Response, 0, len(sessions)); for _, sess := range sessions { items = append(items, newResponse(sess)) }
	httputil.OK(c, gin.H{"sessions": items})
}

func (h *Handler) Revoke(c *gin.Context) {
	userID, err := h.userID(c); if err != nil { httputil.Error(c, err); return }
	sessionID, err := uuid.Parse(c.Param("id")); if err != nil { httputil.Error(c, apperrors.NewBadRequest("invalid session id")); return }
	if err := h.service.Revoke(c.Request.Context(), sessionID, userID); err != nil { httputil.Error(c, err); return }
	currentSessionID, err := h.sessionID(c); if err == nil && currentSessionID == sessionID { ClearCookie(c) }
	httputil.OK(c, gin.H{"message": "session revoked"})
}

func (h *Handler) RevokeAll(c *gin.Context) {
	userID, err := h.userID(c); if err != nil { httputil.Error(c, err); return }
	if err := h.service.RevokeAll(c.Request.Context(), userID); err != nil { httputil.Error(c, err); return }
	ClearCookie(c)
	httputil.OK(c, gin.H{"message": "logged out from all sessions"})
}

func SetCookie(c *gin.Context, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds()); if maxAge < 0 { maxAge = 0 }
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

func ClearCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

const (
	ContextUserID = "auth.user_id"
	ContextSessionID = "auth.session_id"
)

func UserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(ContextUserID); if !exists { return uuid.Nil, apperrors.NewUnauthorized("authentication required") }
	userID, ok := value.(uuid.UUID); if !ok { return uuid.Nil, apperrors.NewUnauthorized("invalid authentication context") }
	return userID, nil
}

func SessionIDFromContext(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(ContextSessionID); if !exists { return uuid.Nil, apperrors.NewUnauthorized("authentication required") }
	sessionID, ok := value.(uuid.UUID); if !ok { return uuid.Nil, apperrors.NewUnauthorized("invalid authentication context") }
	return sessionID, nil
}

func newResponse(sess sqlc.Session) Response {
	var ipAddress *string; if sess.IpAddress != nil { value := sess.IpAddress.String(); ipAddress = &value }
	return Response{ID: sess.ID, UserID: sess.UserID, IPAddress: ipAddress, UserAgent: sess.UserAgent, ExpiresAt: helper.FormatTime(sess.ExpiresAt), LastSeenAt: helper.FormatTime(sess.LastSeenAt), CreatedAt: helper.FormatTime(sess.CreatedAt)}
}
