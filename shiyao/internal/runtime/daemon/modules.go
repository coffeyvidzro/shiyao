package daemon

import (
	"github.com/coffeyvidzro/shiyao/internal/identity/auth"
	"github.com/coffeyvidzro/shiyao/internal/identity/pat"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/internal/identity/teamtoken"
	"github.com/coffeyvidzro/shiyao/internal/identity/users"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
)

type Modules struct {
	Auth      AuthModule
	PAT       PATModule
	TeamToken TeamTokenModule
	Session   SessionModule
	Users     UserModule
	Sandbox   SandboxModule
}

type TeamTokenModule struct {
	Repository *teamtoken.Repository
	Service    *teamtoken.Service
	Handler    *teamtoken.Handler
}

type AuthModule struct {
	Repository *auth.Repository
	Service    *auth.Service
	Handler    *auth.Handler
}

type PATModule struct {
	Repository *pat.Repository
	Service    *pat.Service
	Handler    *pat.Handler
}

type SessionModule struct {
	Repository *session.Repository
	Service    *session.Service
	Handler    *session.Handler
}

type UserModule struct {
	Repository *users.Repository
	Service    *users.Service
	Handler    *users.Handler
}

type SandboxModule struct {
	Repository *sandbox.Repository
	Service    *sandbox.Service
	Handler    *sandbox.Handler
}
