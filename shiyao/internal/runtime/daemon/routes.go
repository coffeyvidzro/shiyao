package daemon

import (
	"net/http"

	"github.com/coffeyvidzro/shiyao/internal/identity/auth"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/internal/identity/users"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, modules Modules) {
	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	api := router.Group("/v1")
	auth.RegisterRoutes(api, modules.Auth.Handler)
	session.RegisterRoutes(api, modules.Session.Handler)
	users.RegisterRoutes(api, modules.Users.Handler)
	sandbox.RegisterRoutes(api, modules.Sandbox.Handler)
}
