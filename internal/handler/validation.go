package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// bindJSON binds the request body JSON into target, writing a 400 error
// response and returning ok=false if the body is missing or malformed.
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

// validateName trims and validates a name, writing the appropriate error response
// and returning ok=false if invalid.
func validateName(c *gin.Context, name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return "", false
	}
	if len(trimmed) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must not exceed 100 characters"})
		return "", false
	}
	return trimmed, true
}

// validateDescription trims and validates an optional description, writing
// the appropriate error response and returning ok=false if invalid.
func validateDescription(c *gin.Context, description string) (string, bool) {
	trimmed := strings.TrimSpace(description)
	if len(trimmed) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description must not exceed 500 characters"})
		return "", false
	}
	return trimmed, true
}
