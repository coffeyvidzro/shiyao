package authz

import "github.com/coffeyvidzro/shiyao/internal/authn"

type Policy interface {
	Allows(principal authn.Principal, permission Permission) bool
}

type DefaultPolicy struct{}

func (DefaultPolicy) Allows(principal authn.Principal, permission Permission) bool {
	if !permission.Valid() || principal.Subject.Type != authn.SubjectUser {
		return false
	}

	// User sessions are intentionally broad for now. PATs are restricted to
	// the scopes embedded in the credential. This keeps authorization policy
	// independent from credential storage while organizations are absent.
	if principal.Credential.Type != authn.CredentialPAT {
		return true
	}

	return false
}
