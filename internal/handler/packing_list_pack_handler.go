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
		internalError(c, "failed to pack all items", err)
		return
	}

	c.Status(http.StatusNoContent)
}

// UnpackAll handles POST /lists/:id/unpack-all — resets every item on the
// list to unpacked. 204, no body, same reasoning as PackAll.
func (h *PackingListHandler) UnpackAll(c *gin.Context) {
	list, _, ok := h.requireOwnedPackingList(c)
	if !ok {
		return
	}

	if err := h.repo.UnpackAllItems(c.Request.Context(), list.ID.String()); err != nil {
		internalError(c, "failed to unpack all items", err)
		return
	}

	c.Status(http.StatusNoContent)
}
