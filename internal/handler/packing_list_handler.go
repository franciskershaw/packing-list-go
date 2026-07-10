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
}

func NewPackingListHandler(repo PackingListRepository, templateRepo TemplateLookupRepository) *PackingListHandler {
	return &PackingListHandler{repo: repo, templateRepo: templateRepo}
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

// GetByID handles GET /lists/:id. PACK-011 stub, not yet implemented.
func (h *PackingListHandler) GetByID(c *gin.Context) {
	panic("not implemented")
}

// Update handles PATCH /lists/:id. PACK-011 stub, not yet implemented.
func (h *PackingListHandler) Update(c *gin.Context) {
	panic("not implemented")
}

// Delete handles DELETE /lists/:id. PACK-011 stub, not yet implemented.
func (h *PackingListHandler) Delete(c *gin.Context) {
	panic("not implemented")
}
