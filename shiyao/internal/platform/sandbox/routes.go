package sandbox

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	router *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
) {
	sandboxes := router.Group("/sandboxes")
	sandboxes.Use(authMiddleware)

	sandboxes.GET("", handler.List)
	sandboxes.GET("/:id", handler.Get)
	sandboxes.GET("/:id/exec/stream", handler.ExecStreamWS)
	sandboxes.POST("", handler.Create)
	sandboxes.DELETE("/:id", handler.Delete)
}
