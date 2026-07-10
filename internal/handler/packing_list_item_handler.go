package handler

import (
	"net/http"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requireOwnedPackingList resolves and validates the :id URL param,
// returning the owned list and requesting user's ID. Writes the error
// response and returns ok=false for any failure along the way
// (unauthenticated, invalid id, fetch error, or list missing/not owned).
// Mirrors TemplateHandler.requireOwnedTemplate.
func (h *PackingListHandler) requireOwnedPackingList(c *gin.Context) (list *models.PackingListDetail, userID string, ok bool) {
	userID, ok = userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, "", false
	}

	listID := c.Param("id")
	if _, err := uuid.Parse(listID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return nil, "", false
	}

	list, err := h.repo.GetPackingListByID(c.Request.Context(), listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packing list"})
		return nil, "", false
	}
	if !isPackingListOwned(list, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "packing list not found"})
		return nil, "", false
	}

	return list, userID, true
}

// AddItem handles POST /lists/:id/items — categoryId is never in the
// request body, it's derived server-side from the item's own category.
// Allowed on archived lists (requireOwnedPackingList doesn't check
// archived state).
func (h *PackingListHandler) AddItem(c *gin.Context) {
	list, userID, ok := h.requireOwnedPackingList(c)
	if !ok {
		return
	}
	listID := list.ID.String()

	var req struct {
		ItemID   string  `json:"itemId"`
		Quantity *int    `json:"quantity"`
		Notes    *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}
	if _, err := uuid.Parse(req.ItemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	item, err := h.itemRepo.GetItemByID(c.Request.Context(), req.ItemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch item"})
		return
	}
	if !isItemAccessible(item, userID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId not accessible"})
		return
	}

	quantity := 1
	if req.Quantity != nil {
		quantity, ok = validateQuantity(c, *req.Quantity)
		if !ok {
			return
		}
	}

	var notesPtr *string
	if req.Notes != nil {
		notes, ok := validateTemplateItemNotes(c, *req.Notes)
		if !ok {
			return
		}
		notesPtr = &notes
	}

	exists, err := h.repo.PackingListItemExists(c.Request.Context(), listID, req.ItemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check packing list item"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "item is already on this list"})
		return
	}

	created, err := h.repo.AddPackingListItem(c.Request.Context(), listID, req.ItemID, quantity, notesPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add packing list item"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// UpdateItem handles PATCH /lists/:id/items/:itemId — quantity/notes/
// sortOrder, at least one required. sortOrder accepts any integer,
// including negative or zero — no domain constraint like quantity's range.
func (h *PackingListHandler) UpdateItem(c *gin.Context) {
	list, _, ok := h.requireOwnedPackingList(c)
	if !ok {
		return
	}
	listID := list.ID.String()

	itemID := c.Param("itemId")
	if _, err := uuid.Parse(itemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	var req struct {
		Quantity  *int    `json:"quantity"`
		Notes     *string `json:"notes"`
		SortOrder *int    `json:"sortOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Quantity == nil && req.Notes == nil && req.SortOrder == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of quantity, notes, or sortOrder is required"})
		return
	}

	exists, err := h.repo.PackingListItemExists(c.Request.Context(), listID, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check packing list item"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "packing list item not found"})
		return
	}

	var quantityPtr *int
	if req.Quantity != nil {
		quantity, ok := validateQuantity(c, *req.Quantity)
		if !ok {
			return
		}
		quantityPtr = &quantity
	}

	var notesPtr *string
	if req.Notes != nil {
		notes, ok := validateTemplateItemNotes(c, *req.Notes)
		if !ok {
			return
		}
		notesPtr = &notes
	}

	updated, err := h.repo.UpdatePackingListItem(c.Request.Context(), listID, itemID, quantityPtr, notesPtr, req.SortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update packing list item"})
		return
	}

	c.JSON(http.StatusOK, updated)
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
