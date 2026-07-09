package handler

import (
	"net/http"
	"strings"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requireOwnedTemplate resolves and validates the :id URL param, returning
// the owned template and requesting user's ID. Writes the error response
// and returns ok=false for any failure along the way (unauthenticated,
// invalid id, fetch error, or template missing/not owned).
func (h *TemplateHandler) requireOwnedTemplate(c *gin.Context) (template *models.Template, userID string, ok bool) {
	userID, ok = userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, "", false
	}

	templateID := c.Param("id")
	if _, err := uuid.Parse(templateID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return nil, "", false
	}

	template, err := h.repo.GetTemplateByID(c.Request.Context(), templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch template"})
		return nil, "", false
	}
	if !isTemplateOwned(template, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return nil, "", false
	}

	return template, userID, true
}

func (h *TemplateHandler) AddItem(c *gin.Context) {
	template, userID, ok := h.requireOwnedTemplate(c)
	if !ok {
		return
	}
	templateID := template.ID.String()

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

	exists, err := h.repo.TemplateItemExists(c.Request.Context(), templateID, req.ItemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check template item"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "item is already on this template"})
		return
	}

	created, err := h.repo.AddTemplateItem(c.Request.Context(), templateID, req.ItemID, quantity, notesPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add template item"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *TemplateHandler) UpdateItem(c *gin.Context) {
	template, _, ok := h.requireOwnedTemplate(c)
	if !ok {
		return
	}
	templateID := template.ID.String()

	itemID := c.Param("itemId")
	if _, err := uuid.Parse(itemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	var req struct {
		Quantity *int    `json:"quantity"`
		Notes    *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Quantity == nil && req.Notes == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of quantity or notes is required"})
		return
	}

	exists, err := h.repo.TemplateItemExists(c.Request.Context(), templateID, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check template item"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "template item not found"})
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

	updated, err := h.repo.UpdateTemplateItem(c.Request.Context(), templateID, itemID, quantityPtr, notesPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update template item"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *TemplateHandler) RemoveItem(c *gin.Context) {
	template, _, ok := h.requireOwnedTemplate(c)
	if !ok {
		return
	}
	templateID := template.ID.String()

	itemID := c.Param("itemId")
	if _, err := uuid.Parse(itemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	exists, err := h.repo.TemplateItemExists(c.Request.Context(), templateID, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check template item"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "template item not found"})
		return
	}

	if err := h.repo.RemoveTemplateItem(c.Request.Context(), templateID, itemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove template item"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TemplateHandler) BulkAddItems(c *gin.Context) {
	template, userID, ok := h.requireOwnedTemplate(c)
	if !ok {
		return
	}
	templateID := template.ID.String()

	var req struct {
		CategoryID string `json:"categoryId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !validateAccessibleCategory(c, h.itemRepo, req.CategoryID, userID) {
		return
	}

	categoryItems, err := h.itemRepo.GetItems(c.Request.Context(), userID, &req.CategoryID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch category items"})
		return
	}

	existing, err := h.repo.GetTemplateItems(c.Request.Context(), templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch template items"})
		return
	}
	alreadyOnTemplate := make(map[uuid.UUID]bool, len(existing))
	for _, item := range existing {
		alreadyOnTemplate[item.ItemID] = true
	}

	added := make([]models.TemplateItem, 0)
	for _, item := range categoryItems {
		if alreadyOnTemplate[item.ID] {
			continue
		}
		created, err := h.repo.AddTemplateItem(c.Request.Context(), templateID, item.ID.String(), 1, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add template item"})
			return
		}
		added = append(added, *created)
	}

	c.JSON(http.StatusCreated, added)
}

// validateQuantity validates a template item quantity is in [1, 999],
// writing the appropriate error response and returning ok=false if invalid.
func validateQuantity(c *gin.Context, quantity int) (int, bool) {
	if quantity < 1 || quantity > 999 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be between 1 and 999"})
		return 0, false
	}
	return quantity, true
}

// validateTemplateItemNotes trims and validates optional per-item notes,
// writing the appropriate error response and returning ok=false if invalid.
func validateTemplateItemNotes(c *gin.Context, notes string) (string, bool) {
	trimmed := strings.TrimSpace(notes)
	if len(trimmed) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notes must not exceed 200 characters"})
		return "", false
	}
	return trimmed, true
}

// isItemAccessible returns true only when the item exists and is either
// system-owned (nil UserID) or owned by the given user — unlike isItemOwned,
// system items are allowed here since any accessible item can be attached to
// a template.
func isItemAccessible(item *models.Item, userID string) bool {
	return item != nil && (item.UserID == nil || item.UserID.String() == userID)
}
