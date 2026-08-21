package authn

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || principal.Subject.Type != SubjectUser || principal.Subject.ID == uuid.Nil {
		return uuid.Nil, false
	}

	return principal.Subject.ID, true
}
