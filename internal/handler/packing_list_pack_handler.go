package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PackAll handles POST /lists/:id/pack-all. PACK-013 stub, not yet
// implemented.
func (h *PackingListHandler) PackAll(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// UnpackAll handles POST /lists/:id/unpack-all. PACK-013 stub, not yet
// implemented.
func (h *PackingListHandler) UnpackAll(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
