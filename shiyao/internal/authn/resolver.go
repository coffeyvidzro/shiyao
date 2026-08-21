package authn

import "context"

type Resolver interface {
	Resolve(ctx context.Context, input CredentialInput) (Principal, error)
}
