package authz

import "github.com/coffeyvidzro/shiyao/internal/authn"

type Authorizer struct {
	policy Policy
}

func New(policy Policy) *Authorizer {
	if policy == nil {
		policy = DefaultPolicy{}
	}
	return &Authorizer{policy: policy}
}

func (a *Authorizer) Allows(principal authn.Principal, permission Permission) bool {
	if a == nil || a.policy == nil {
		return false
	}
	return a.policy.Allows(principal, permission)
}
