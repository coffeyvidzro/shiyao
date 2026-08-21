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
