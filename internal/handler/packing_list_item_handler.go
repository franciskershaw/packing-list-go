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
		internalError(c, "failed to fetch packing list", err)
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
	if !bindJSON(c, &req) {
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
		internalError(c, "failed to fetch item", err)
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
		notes, ok := validateItemNotes(c, *req.Notes)
		if !ok {
			return
		}
		notesPtr = &notes
	}

	exists, err := h.repo.PackingListItemExists(c.Request.Context(), listID, req.ItemID)
	if err != nil {
		internalError(c, "failed to check packing list item", err)
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "item is already on this list"})
		return
	}

	created, err := h.repo.AddPackingListItem(c.Request.Context(), listID, req.ItemID, quantity, notesPtr)
	if err != nil {
		internalError(c, "failed to add packing list item", err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

// UpdateItem handles PATCH /lists/:id/items/:itemId — quantity/notes/
// sortOrder/isPacked, at least one required. sortOrder accepts any integer,
// including negative or zero — no domain constraint like quantity's range.
// isPacked is *bool so an explicit false is distinguishable from omission.
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
		IsPacked  *bool   `json:"isPacked"`
	}
	if !bindJSON(c, &req) {
		return
	}

	if req.Quantity == nil && req.Notes == nil && req.SortOrder == nil && req.IsPacked == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of quantity, notes, sortOrder, or isPacked is required"})
		return
	}

	exists, err := h.repo.PackingListItemExists(c.Request.Context(), listID, itemID)
	if err != nil {
		internalError(c, "failed to check packing list item", err)
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
		notes, ok := validateItemNotes(c, *req.Notes)
		if !ok {
			return
		}
		notesPtr = &notes
	}

	updated, err := h.repo.UpdatePackingListItem(c.Request.Context(), listID, itemID, quantityPtr, notesPtr, req.SortOrder, req.IsPacked)
	if err != nil {
		internalError(c, "failed to update packing list item", err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

// RemoveItem handles DELETE /lists/:id/items/:itemId.
func (h *PackingListHandler) RemoveItem(c *gin.Context) {
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

	exists, err := h.repo.PackingListItemExists(c.Request.Context(), listID, itemID)
	if err != nil {
		internalError(c, "failed to check packing list item", err)
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "packing list item not found"})
		return
	}

	if err := h.repo.RemovePackingListItem(c.Request.Context(), listID, itemID); err != nil {
		internalError(c, "failed to remove packing list item", err)
		return
	}

	c.Status(http.StatusNoContent)
}

// BulkUpdateItems handles PATCH /lists/:id/items/bulk — a delta of
// itemId -> quantity changes, applied atomically. quantity 0 removes an
// item (no-op if already absent); any other quantity in [0, 999] adds the
// item if absent or updates it if present. See PACK-035.
func (h *PackingListHandler) BulkUpdateItems(c *gin.Context) {
	list, userID, ok := h.requireOwnedPackingList(c)
	if !ok {
		return
	}
	listID := list.ID.String()

	var req struct {
		Items []struct {
			ItemID   string `json:"itemId"`
			Quantity int    `json:"quantity"`
		} `json:"items"`
	}
	if !bindJSON(c, &req) {
		return
	}

	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one item is required"})
		return
	}

	seen := make(map[string]bool, len(req.Items))
	ids := make([]string, 0, len(req.Items))
	changes := make(map[string]int, len(req.Items))
	for _, item := range req.Items {
		if _, err := uuid.Parse(item.ItemID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
			return
		}
		if seen[item.ItemID] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duplicate itemId in request"})
			return
		}
		seen[item.ItemID] = true
		if item.Quantity < 0 || item.Quantity > 999 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be between 0 and 999"})
			return
		}
		ids = append(ids, item.ItemID)
		changes[item.ItemID] = item.Quantity
	}

	items, err := h.itemRepo.GetItemsByIDs(c.Request.Context(), ids)
	if err != nil {
		internalError(c, "failed to fetch items", err)
		return
	}
	accessible := make(map[string]*models.Item, len(items))
	for i := range items {
		accessible[items[i].ID.String()] = &items[i]
	}
	for _, id := range ids {
		item, found := accessible[id]
		if !found || !isItemAccessible(item, userID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "itemId not accessible"})
			return
		}
	}

	if err := h.repo.BulkUpdatePackingListItems(c.Request.Context(), listID, changes); err != nil {
		internalError(c, "failed to bulk update packing list items", err)
		return
	}

	c.Status(http.StatusNoContent)
}
