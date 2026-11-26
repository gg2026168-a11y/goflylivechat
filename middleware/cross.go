package middleware

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func CrossSite(c *gin.Context) {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		c.Next()
		return
	}

	allowed := false
	configuredOrigins := strings.Split(os.Getenv("GOFLY_ALLOWED_ORIGINS"), ",")
	for _, item := range configuredOrigins {
		if item = strings.TrimSpace(item); item != "" && item == origin {
			allowed = true
			break
		}
	}
	if !allowed {
		parsedOrigin, err := url.Parse(origin)
		allowed = err == nil && (parsedOrigin.Scheme == "http" || parsedOrigin.Scheme == "https") && strings.EqualFold(parsedOrigin.Host, c.Request.Host)
	}
	if !allowed {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.Writer.Header().Add("Vary", "Origin")
	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
	c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Token")
	c.Header("Access-Control-Expose-Headers", "Content-Length")
	c.Header("Access-Control-Max-Age", "600")
	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}
