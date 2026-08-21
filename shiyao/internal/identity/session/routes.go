package session

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	router *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
) {
	sessions := router.Group("/sessions")
	sessions.Use(authMiddleware)

	sessions.GET("", handler.List)
	sessions.DELETE("/:id", handler.Revoke)
	sessions.DELETE("", handler.RevokeAll)
}
