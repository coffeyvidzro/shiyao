package daemon

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeyvidzro/shiyao/internal/identity/auth"
	"github.com/coffeyvidzro/shiyao/internal/identity/pat"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/internal/identity/users"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
	"github.com/coffeyvidzro/shiyao/internal/runtime/middleware"
)

func RegisterRoutes(router *gin.Engine, modules Modules) {
	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	authMiddleware := middleware.NewAuth(
		modules.Session.Service,
		modules.PAT.Service,
		modules.Users.Service,
	).Handler()

	api := router.Group("/v1")
	auth.RegisterRoutes(api, modules.Auth.Handler, authMiddleware)
	pat.RegisterRoutes(api, modules.PAT.Handler, authMiddleware)
	session.RegisterRoutes(api, modules.Session.Handler, authMiddleware)
	users.RegisterRoutes(api, modules.Users.Handler, authMiddleware)
	sandbox.RegisterRoutes(api, modules.Sandbox.Handler, authMiddleware)
}
