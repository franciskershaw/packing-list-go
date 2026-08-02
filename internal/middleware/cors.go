package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS is a stub, not yet implemented.
func CORS(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNotImplemented)
	}
}
