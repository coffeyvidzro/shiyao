package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	router *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
) {
	auth := router.Group("/auth")

	auth.POST("/start", handler.Start)

	auth.POST("/password/login", handler.LoginWithPassword)

	auth.POST("/otp/send", handler.SendOTP)
	auth.POST("/otp/verify", handler.VerifyOTP)

	protected := auth.Group("")
	protected.Use(authMiddleware)

	protected.POST("/password/enroll", handler.SetPassword)
}
