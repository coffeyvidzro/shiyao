package authn

import "github.com/google/uuid"

type SubjectType string

const (
	SubjectUser SubjectType = "user"
)

type Subject struct {
	ID   uuid.UUID
	Type SubjectType
}

type Principal struct {
	Subject    Subject
	Credential Credential
	Assurance  AssuranceLevel
}
