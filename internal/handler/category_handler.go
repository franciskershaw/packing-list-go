package handler

import (
	"context"
	"net/http"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CategoryRepository interface {
	GetCategories(ctx context.Context, userID string) ([]models.Category, error)
	CreateCategory(ctx context.Context, userID, name string) (*models.Category, error)
	GetCategoryByID(ctx context.Context, id string) (*models.Category, error)
	UpdateCategory(ctx context.Context, id, name string) (*models.Category, error)
	DeleteCategory(ctx context.Context, id string) error
	CategoryNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error)
	CategoryHasItems(ctx context.Context, id string) (bool, error)
}

type CategoryHandler struct {
	repo CategoryRepository
}

func NewCategoryHandler(repo CategoryRepository) *CategoryHandler {
	return &CategoryHandler{repo: repo}
}

func (h *CategoryHandler) List(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	categories, err := h.repo.GetCategories(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	name, ok := parseName(c)
	if !ok {
		return
	}

	exists, err := h.repo.CategoryNameExistsForUser(c.Request.Context(), userID, name, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check category name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "a category with this name already exists"})
		return
	}

	category, err := h.repo.CreateCategory(c.Request.Context(), userID, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, category)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	name, ok := parseName(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	category, err := h.repo.GetCategoryByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch category"})
		return
	}
	if !isOwned(category, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}

	exists, err := h.repo.CategoryNameExistsForUser(c.Request.Context(), userID, name, &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check category name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "a category with this name already exists"})
		return
	}

	updated, err := h.repo.UpdateCategory(c.Request.Context(), id, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update category"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *CategoryHandler) Delete(c *gin.Context) {
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
	category, err := h.repo.GetCategoryByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch category"})
		return
	}
	if !isOwned(category, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}

	hasItems, err := h.repo.CategoryHasItems(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check category items"})
		return
	}
	if hasItems {
		c.JSON(http.StatusConflict, gin.H{"error": "category has items and cannot be deleted"})
		return
	}

	if err := h.repo.DeleteCategory(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete category"})
		return
	}

	c.Status(http.StatusNoContent)
}

// parseName reads and validates the "name" field from the JSON body.
func parseName(c *gin.Context) (string, bool) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return "", false
	}
	return validateName(c, req.Name)
}

// isOwned returns true only when the category exists and belongs to the given user.
// Returns false for nil (not found), system categories (UserID == nil), or wrong owner.
func isOwned(category *models.Category, userID string) bool {
	return category != nil && isOwnedBy(category.UserID, userID)
}
