package handler

import (
	"context"
	"net/http"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ItemRepository interface {
	GetItems(ctx context.Context, userID string, categoryID *string, search *string) ([]models.Item, error)
	GetItemByID(ctx context.Context, id string) (*models.Item, error)
	CreateItem(ctx context.Context, userID, name, categoryID string) (*models.Item, error)
	UpdateItem(ctx context.Context, id string, name *string, categoryID *string) (*models.Item, error)
	DeleteItem(ctx context.Context, id string) error
	ItemNameExistsInCategory(ctx context.Context, categoryID, name string, excludeID *string) (bool, error)
	ItemIsInUse(ctx context.Context, id string) (bool, error)
	CategoryIsAccessible(ctx context.Context, categoryID, userID string) (bool, error)
}

type ItemHandler struct {
	repo ItemRepository
}

func NewItemHandler(repo ItemRepository) *ItemHandler {
	return &ItemHandler{repo: repo}
}

func (h *ItemHandler) List(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var categoryIDPtr *string
	if categoryID := c.Query("category_id"); categoryID != "" {
		if _, err := uuid.Parse(categoryID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
			return
		}
		accessible, err := h.repo.CategoryIsAccessible(c.Request.Context(), categoryID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check category"})
			return
		}
		if !accessible {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category not accessible"})
			return
		}
		categoryIDPtr = &categoryID
	}

	var searchPtr *string
	if search := c.Query("search"); search != "" {
		searchPtr = &search
	}

	items, err := h.repo.GetItems(c.Request.Context(), userID, categoryIDPtr, searchPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch items"})
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *ItemHandler) Create(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name       string `json:"name"`
		CategoryID string `json:"categoryId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name, ok := validateName(c, req.Name)
	if !ok {
		return
	}

	if !h.validateAccessibleCategory(c, req.CategoryID, userID) {
		return
	}

	exists, err := h.repo.ItemNameExistsInCategory(c.Request.Context(), req.CategoryID, name, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check item name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "an item with this name already exists in this category"})
		return
	}

	item, err := h.repo.CreateItem(c.Request.Context(), userID, name, req.CategoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create item"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *ItemHandler) Update(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name       *string `json:"name"`
		CategoryID *string `json:"categoryId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Name == nil && req.CategoryID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of name or categoryId is required"})
		return
	}

	var namePtr *string
	if req.Name != nil {
		name, ok := validateName(c, *req.Name)
		if !ok {
			return
		}
		namePtr = &name
	}

	if req.CategoryID != nil {
		if _, err := uuid.Parse(*req.CategoryID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid categoryId"})
			return
		}
	}

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	item, err := h.repo.GetItemByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch item"})
		return
	}
	if !isItemOwned(item, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	if req.CategoryID != nil {
		if !h.validateAccessibleCategory(c, *req.CategoryID, userID) {
			return
		}
	}

	targetName := item.Name
	if namePtr != nil {
		targetName = *namePtr
	}
	targetCategory := item.CategoryID.String()
	if req.CategoryID != nil {
		targetCategory = *req.CategoryID
	}

	exists, err := h.repo.ItemNameExistsInCategory(c.Request.Context(), targetCategory, targetName, &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check item name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "an item with this name already exists in this category"})
		return
	}

	updated, err := h.repo.UpdateItem(c.Request.Context(), id, namePtr, req.CategoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update item"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *ItemHandler) Delete(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	item, err := h.repo.GetItemByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch item"})
		return
	}
	if !isItemOwned(item, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	inUse, err := h.repo.ItemIsInUse(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check item usage"})
		return
	}
	if inUse {
		c.JSON(http.StatusConflict, gin.H{"error": "item is in use and cannot be deleted"})
		return
	}

	if err := h.repo.DeleteItem(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete item"})
		return
	}

	c.Status(http.StatusNoContent)
}

// validateAccessibleCategory checks the categoryId is a valid UUID and accessible to userID,
// writing the appropriate error response and returning false if not.
func (h *ItemHandler) validateAccessibleCategory(c *gin.Context, categoryID, userID string) bool {
	if categoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "categoryId is required"})
		return false
	}
	if _, err := uuid.Parse(categoryID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid categoryId"})
		return false
	}
	accessible, err := h.repo.CategoryIsAccessible(c.Request.Context(), categoryID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check category"})
		return false
	}
	if !accessible {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category not accessible"})
		return false
	}
	return true
}

// isItemOwned returns true only when the item exists and belongs to the given user.
// Returns false for nil (not found), system items (UserID == nil), or wrong owner.
func isItemOwned(item *models.Item, userID string) bool {
	return item != nil && isOwnedBy(item.UserID, userID)
}
