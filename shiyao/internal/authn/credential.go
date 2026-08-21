package authn

import "github.com/google/uuid"

type CredentialType string

const (
	CredentialSession CredentialType = "session"
	CredentialPAT     CredentialType = "pat"
)

type Credential struct {
	ID   uuid.UUID
	Type CredentialType
}

type CredentialInput struct {
	Type  CredentialType
	Value string
}

func (c CredentialInput) Validate() error {
	if c.Type == "" || c.Value == "" {
		return ErrInvalidCredential
	}

	switch c.Type {
	case CredentialSession, CredentialPAT:
		return nil
	default:
		return ErrInvalidCredential
	}
}
