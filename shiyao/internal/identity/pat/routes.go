package pat

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	router *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
) {
	tokens := router.Group("/tokens")
	tokens.Use(authMiddleware)

	tokens.POST("", handler.Create)
	tokens.GET("", handler.List)
	tokens.DELETE("/:id", handler.Revoke)
}
