package handler

import "github.com/gin-gonic/gin"

// userIDFromCtx retrieves the userId string set by AuthMiddleware.
func userIDFromCtx(c *gin.Context) (string, bool) {
	val, exists := c.Get("userId")
	if !exists {
		return "", false
	}
	id, ok := val.(string)
	return id, ok && id != ""
}
