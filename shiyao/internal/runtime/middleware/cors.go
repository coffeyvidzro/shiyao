package middleware

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(allowOrigins []string, development bool) gin.HandlerFunc {
	allowAllOrigins := development && len(allowOrigins) == 0

	return cors.New(cors.Config{
		AllowAllOrigins:  allowAllOrigins,
		AllowOrigins:     allowOrigins,
		AllowCredentials: false,
		AllowHeaders: []string{
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"X-Request-ID",
			"Authorization",
			"accept",
			"origin",
			"Cache-Control",
			"X-Requested-With",
		},
		AllowMethods: []string{
			http.MethodPost,
			http.MethodOptions,
			http.MethodGet,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		ExposeHeaders: []string{"X-Request-ID"},
	})
}
