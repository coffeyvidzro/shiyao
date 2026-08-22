//go:build cloud

package daemon

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/identity/teamtoken"
)

type cloudModule struct {
	teamTokens *teamtoken.Handler
}

func configureEditionModules(queries *sqlc.Queries, modules *Modules) {
	repository := teamtoken.NewRepository(queries)
	service := teamtoken.NewService(repository)
	modules.Edition = cloudModule{teamTokens: teamtoken.NewHandler(service)}
}

func (m cloudModule) RegisterRoutes(router *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	teamtoken.RegisterRoutes(router, m.teamTokens, authMiddleware)
}
