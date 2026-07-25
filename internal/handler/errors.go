package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// internalError attaches the real error to the Gin context — logged
// centrally by middleware.ErrorLogger — and responds with the given
// generic message. The client never sees repository/driver error text.
func internalError(c *gin.Context, message string, err error) {
	_ = c.Error(fmt.Errorf("%s: %w", message, err))
	c.JSON(http.StatusInternalServerError, gin.H{"error": message})
}
