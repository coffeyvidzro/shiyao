//go:build cloud

package teamtoken

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc) {
	teams := router.Group("/teams/:team_id/tokens")
	teams.Use(authMiddleware)
	teams.POST("", handler.Create)
	teams.GET("", handler.List)
	teams.DELETE("/:id", handler.Revoke)
}
