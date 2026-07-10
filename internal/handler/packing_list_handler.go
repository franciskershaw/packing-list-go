package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PackingListRepository interface {
	CreatePackingList(ctx context.Context, userID, name string, eventDate *string, templateID *string) (*models.PackingList, error)
	GetPackingLists(ctx context.Context, userID string, archived bool) ([]models.PackingList, error)
	GetPackingListByID(ctx context.Context, id string) (*models.PackingListDetail, error)
	UpdatePackingList(ctx context.Context, id string, name *string, eventDate *string) (*models.PackingListDetail, error)
	ArchivePackingList(ctx context.Context, id string) error
	AddPackingListItem(ctx context.Context, listID, itemID string, quantity int, notes *string) (*models.PackingListItem, error)
	UpdatePackingListItem(ctx context.Context, listID, itemID string, quantity *int, notes *string, sortOrder *int) (*models.PackingListItem, error)
	RemovePackingListItem(ctx context.Context, listID, itemID string) error
	PackingListItemExists(ctx context.Context, listID, itemID string) (bool, error)
	GetPackingListItems(ctx context.Context, listID string) ([]models.PackingListItem, error)
}

// TemplateLookupRepository exposes just what PackingListHandler needs to
// validate a caller-referenced templateId — mirrors ItemLookupRepository's
// role for TemplateHandler in PACK-009.
type TemplateLookupRepository interface {
	GetTemplateByID(ctx context.Context, id string) (*models.Template, error)
}

type PackingListHandler struct {
	repo         PackingListRepository
	templateRepo TemplateLookupRepository
	itemRepo     ItemLookupRepository
}

func NewPackingListHandler(repo PackingListRepository, templateRepo TemplateLookupRepository, itemRepo ItemLookupRepository) *PackingListHandler {
	return &PackingListHandler{repo: repo, templateRepo: templateRepo, itemRepo: itemRepo}
}

func (h *PackingListHandler) Create(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name       string  `json:"name"`
		EventDate  *string `json:"eventDate"`
		TemplateID *string `json:"templateId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name, ok := validateName(c, req.Name)
	if !ok {
		return
	}

	if req.EventDate != nil {
		if _, err := time.Parse("2006-01-02", *req.EventDate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid eventDate"})
			return
		}
	}

	if req.TemplateID != nil {
		if _, err := uuid.Parse(*req.TemplateID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid templateId"})
			return
		}

		template, err := h.templateRepo.GetTemplateByID(c.Request.Context(), *req.TemplateID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch template"})
			return
		}
		if !isTemplateOwned(template, userID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "templateId not accessible"})
			return
		}
	}

	created, err := h.repo.CreatePackingList(c.Request.Context(), userID, name, req.EventDate, req.TemplateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create packing list"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// List handles GET /lists and GET /lists?archived=true. Only the literal
// string "true" selects the archived branch — any other value, or the
// param's absence, falls back to active lists.
func (h *PackingListHandler) List(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	archived := c.Query("archived") == "true"

	lists, err := h.repo.GetPackingLists(c.Request.Context(), userID, archived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packing lists"})
		return
	}

	c.JSON(http.StatusOK, lists)
}

// GetByID handles GET /lists/:id — full detail, items grouped by category.
// Works identically for archived lists; archiving only changes which List
// view a list appears in, not whether its detail is reachable.
func (h *PackingListHandler) GetByID(c *gin.Context) {
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

	list, err := h.repo.GetPackingListByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packing list"})
		return
	}
	if !isPackingListOwned(list, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "packing list not found"})
		return
	}

	c.JSON(http.StatusOK, list)
}

// Update handles PATCH /lists/:id — name and/or eventDate. No uniqueness
// check (duplicate list names are fine, per PACK-010). Allowed on archived
// lists — archiving doesn't freeze the record.
func (h *PackingListHandler) Update(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name      *string `json:"name"`
		EventDate *string `json:"eventDate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Name == nil && req.EventDate == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of name or eventDate is required"})
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

	if req.EventDate != nil {
		if _, err := time.Parse("2006-01-02", *req.EventDate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid eventDate"})
			return
		}
	}

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	list, err := h.repo.GetPackingListByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packing list"})
		return
	}
	if !isPackingListOwned(list, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "packing list not found"})
		return
	}

	updated, err := h.repo.UpdatePackingList(c.Request.Context(), id, namePtr, req.EventDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update packing list"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// Delete handles DELETE /lists/:id — soft delete via archived_at. Idempotent:
// archiving an already-archived list still returns 204, no special-case
// check needed since ArchivePackingList itself is unconditional.
func (h *PackingListHandler) Delete(c *gin.Context) {
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

	list, err := h.repo.GetPackingListByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packing list"})
		return
	}
	if !isPackingListOwned(list, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "packing list not found"})
		return
	}

	if err := h.repo.ArchivePackingList(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to archive packing list"})
		return
	}

	c.Status(http.StatusNoContent)
}
