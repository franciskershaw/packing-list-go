package middleware

import "github.com/gin-gonic/gin"

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
