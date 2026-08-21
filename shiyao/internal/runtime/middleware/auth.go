package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/shiyao/shiyao/internal/identity/pat"
)

const bearerScheme = "Bearer"

type Auth struct {
	sessions *session.Service
	pat      authn.Resolver
}

func NewAuth(sessions *session.Service, patResolver authn.Resolver) *Auth {
	return &Auth{
		sessions: sessions,
		pat:      patResolver,
	}
}

func (a *Auth) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if principal, ok := a.resolveBearer(c); ok {
			setPrincipal(c, principal)
			c.Next()
			return
		}

		if principal, ok := a.resolveSession(c); ok {
			setPrincipal(c, principal)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, apperrors.NewUnauthorized("authentication required"))
	}
}

func (a *Auth) resolveBearer(c *gin.Context) (authn.Principal, bool) {
	if a.pat == nil {
		return authn.Principal{}, false
	}

	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return authn.Principal{}, false
	}

	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], bearerScheme) {
		return authn.Principal{}, false
	}

	principal, err := a.pat.Resolve(c.Request.Context(), authn.CredentialInput{
		Type:  authn.CredentialPAT,
		Value: parts[1],
	})
	if err != nil {
		return authn.Principal{}, false
	}

	return principal, true
}

func (a *Auth) resolveSession(c *gin.Context) (authn.Principal, bool) {
	if a.sessions == nil {
		return authn.Principal{}, false
	}

	cookie, err := c.Request.Cookie("shiyao-session")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return authn.Principal{}, false
	}

	sess, err := a.sessions.Get(c.Request.Context(), cookie.Value)
	if err != nil {
		return authn.Principal{}, false
	}

	return authn.Principal{
		Subject: authn.Subject{
			ID:   sess.UserID,
			Type: authn.SubjectUser,
		},
		Credential: authn.Credential{
			ID:   sess.ID,
			Type: authn.CredentialSession,
		},
		Assurance: authn.AssuranceUnknown,
	}, true
}

func setPrincipal(c *gin.Context, principal authn.Principal) {
	ctx := authn.WithPrincipal(c.Request.Context(), principal)
	c.Request = c.Request.WithContext(ctx)

	c.Set(session.ContextUserID, principal.Subject.ID)
	if principal.Credential.Type == authn.CredentialSession {
		c.Set(session.ContextSessionID, principal.Credential.ID)
	}
}

var _ = uuid.Nil
