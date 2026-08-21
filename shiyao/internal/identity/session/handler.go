package session

import (
	"net/http"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	userID, err := UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	sessions, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	items := make([]Response, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, newResponse(sess))
	}
	httputil.OK(c, gin.H{"sessions": items})
}

func (h *Handler) Revoke(c *gin.Context) {
	userID, err := UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, NewBadRequestError("invalid session id"))
		return
	}
	if err := h.service.Revoke(c.Request.Context(), sessionID, userID); err != nil {
		httputil.Error(c, err)
		return
	}
	if currentID, err := SessionIDFromContext(c); err == nil && currentID == sessionID {
		ClearCookie(c)
	}
	httputil.OK(c, gin.H{"message": "session revoked"})
}

func (h *Handler) RevokeAll(c *gin.Context) {
	userID, err := UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	if err := h.service.RevokeAll(c.Request.Context(), userID); err != nil {
		httputil.Error(c, err)
		return
	}
	ClearCookie(c)
	httputil.OK(c, gin.H{"message": "logged out from all sessions"})
}

func SetCookie(c *gin.Context, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: "session", Value: token, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
func ClearCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

const (
	ContextUserID    = "auth.user_id"
	ContextSessionID = "auth.session_id"
)

func UserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(ContextUserID)
	if !exists {
		return uuid.Nil, NewUnauthorizedError("authentication required")
	}
	id, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, NewUnauthorizedError("invalid authentication context")
	}
	return id, nil
}
func SessionIDFromContext(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(ContextSessionID)
	if !exists {
		return uuid.Nil, NewUnauthorizedError("authentication required")
	}
	id, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, NewUnauthorizedError("invalid authentication context")
	}
	return id, nil
}

func newResponse(sess sqlc.Session) Response {
	var ip *string
	if sess.IpAddress != nil {
		value := sess.IpAddress.String()
		ip = &value
	}
	return Response{ID: sess.ID, UserID: sess.UserID, IPAddress: ip, UserAgent: sess.UserAgent, ExpiresAt: formatTime(sess.ExpiresAt), LastSeenAt: formatTime(sess.LastSeenAt), CreatedAt: formatTime(sess.CreatedAt)}
}
func formatTime(value pgtype.Timestamptz) string {
	t := pgconv.TimestamptzToTime(value)
	if t.IsZero() {
		return ""
	}
	return t.Format(http.TimeFormat)
}
