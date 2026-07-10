package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AddItem handles POST /lists/:id/items. PACK-012 stub, not yet implemented.
func (h *PackingListHandler) AddItem(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// UpdateItem handles PATCH /lists/:id/items/:itemId. PACK-012 stub, not yet
// implemented.
func (h *PackingListHandler) UpdateItem(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// RemoveItem handles DELETE /lists/:id/items/:itemId. PACK-012 stub, not
// yet implemented.
func (h *PackingListHandler) RemoveItem(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// BulkAddItems handles POST /lists/:id/items/bulk. PACK-012 stub, not yet
// implemented.
func (h *PackingListHandler) BulkAddItems(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
