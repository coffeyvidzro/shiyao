package authn

import "errors"

var (
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrInvalidCredential   = errors.New("invalid credential")
)
