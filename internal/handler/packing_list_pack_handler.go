package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PackAll handles POST /lists/:id/pack-all — sets every item on the list to
// packed. 204, no body: the mutation is a single deterministic UPDATE, so
// an optimistic-update client already knows the resulting state.
func (h *PackingListHandler) PackAll(c *gin.Context) {
	list, _, ok := h.requireOwnedPackingList(c)
	if !ok {
		return
	}

	if err := h.repo.PackAllItems(c.Request.Context(), list.ID.String()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to pack all items"})
		return
	}

	c.Status(http.StatusNoContent)
}

// UnpackAll handles POST /lists/:id/unpack-all. PACK-013 stub, not yet
// implemented.
func (h *PackingListHandler) UnpackAll(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
