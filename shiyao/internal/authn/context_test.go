package authn

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPrincipalContext(t *testing.T) {
	ctx := context.Background()
	if _, ok := PrincipalFromContext(ctx); ok {
		t.Fatal("PrincipalFromContext() found principal in empty context")
	}

	principal := Principal{
		Subject: Subject{
			ID:   uuid.New(),
			Type: SubjectUser,
		},
		Credential: Credential{
			ID:   uuid.New(),
			Type: CredentialPAT,
		},
		Assurance: AssuranceUnknown,
	}

	ctx = WithPrincipal(ctx, principal)
	got, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("PrincipalFromContext() did not find principal")
	}

	if got != principal {
		t.Fatalf("PrincipalFromContext() = %+v, want %+v", got, principal)
	}
}
