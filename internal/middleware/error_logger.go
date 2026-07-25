package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// ErrorLogger logs any errors handlers attached via c.Error, once per
// request, after the handler chain runs — the single place 5xx failures
// get logged, so handlers never need to log directly themselves.
func ErrorLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		for _, ginErr := range c.Errors {
			slog.Error("request failed",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"status", c.Writer.Status(),
				"err", ginErr.Err,
			)
		}
	}
}
