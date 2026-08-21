package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all authentication routes.
func RegisterRoutes(
	router *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
) {
	auth := router.Group("/auth")

	// -------------------------------------------------------------------------
	// Authentication flow
	// -------------------------------------------------------------------------

	// Start an authentication transaction.
	//
	// POST /auth/start
	auth.POST("/start", handler.Start)

	// Authenticate using an existing password.
	//
	// POST /auth/password
	auth.POST("/password", handler.LoginWithPassword)

	// Send a one-time code.
	//
	// POST /auth/otp/send
	auth.POST("/otp/send", handler.SendOTP)

	// Verify a one-time code.
	//
	// POST /auth/otp/verify
	auth.POST("/otp/verify", handler.VerifyOTP)

	// -------------------------------------------------------------------------
	// Authenticated user operations
	// -------------------------------------------------------------------------

	protected := auth.Group("")
	protected.Use(authMiddleware)

	// Set/change the authenticated user's password.
	//
	// POST /auth/password/set
	protected.POST("/password/set", handler.SetPassword)
}
