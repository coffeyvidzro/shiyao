package middleware

import (
	"github.com/gin-contrib/secure"
	"github.com/gin-gonic/gin"
)

func Secure() gin.HandlerFunc {
	config := secure.DefaultConfig()
	config.SSLRedirect = false
	config.STSSeconds = 0
	config.ContentSecurityPolicy = ""
	config.ReferrerPolicy = "no-referrer"
	config.IENoOpen = true

	return secure.New(config)
}
