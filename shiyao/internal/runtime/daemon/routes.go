package daemon

// import (
// 	"net/http"

// // 	"github.com/coffeyvidzro/shiyao/internal/identity/auth"
// // 	"github.com/coffeyvidzro/shiyao/internal/identity/session"
// // 	"github.com/coffeyvidzro/shiyao/internal/identity/users"
// )

// func RegisterRoutes(e *echo.Echo, modules Modules) {
// 	e.GET("/healthz", func(c *echo.Context) error {
// 		return c.NoContent(http.StatusNoContent)
// 	})

// 	api := e.Group("/v1")
// 	auth.RegisterRoutes(api, modules.Auth.Handler)
// 	session.RegisterRoutes(api, modules.Session.Handler)
// 	users.RegisterRoutes(api, modules.Users.Handler)
// }
