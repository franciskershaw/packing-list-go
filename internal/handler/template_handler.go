package handler

import (
	"context"
	"net/http"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TemplateRepository interface {
	GetTemplates(ctx context.Context, userID string) ([]models.Template, error)
	GetTemplateByID(ctx context.Context, id string) (*models.Template, error)
	CreateTemplate(ctx context.Context, userID, name string, description *string) (*models.Template, error)
	UpdateTemplate(ctx context.Context, id string, name *string, description *string) (*models.Template, error)
	DeleteTemplate(ctx context.Context, id string) error
	TemplateNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error)
}

type TemplateHandler struct {
	repo TemplateRepository
}

func NewTemplateHandler(repo TemplateRepository) *TemplateHandler {
	return &TemplateHandler{repo: repo}
}

func (h *TemplateHandler) List(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	templates, err := h.repo.GetTemplates(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch templates"})
		return
	}

	c.JSON(http.StatusOK, templates)
}

func (h *TemplateHandler) Create(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name, ok := validateName(c, req.Name)
	if !ok {
		return
	}

	descPtr, ok := parseOptionalDescription(c, req.Description)
	if !ok {
		return
	}

	exists, err := h.repo.TemplateNameExistsForUser(c.Request.Context(), userID, name, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check template name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "a template with this name already exists"})
		return
	}

	created, err := h.repo.CreateTemplate(c.Request.Context(), userID, name, descPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *TemplateHandler) GetByID(c *gin.Context) {
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

	template, err := h.repo.GetTemplateByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch template"})
		return
	}
	if !isTemplateOwned(template, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	c.JSON(http.StatusOK, template)
}

func (h *TemplateHandler) Update(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Name == nil && req.Description == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of name or description is required"})
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

	descPtr, ok := parseOptionalDescription(c, req.Description)
	if !ok {
		return
	}

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	template, err := h.repo.GetTemplateByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch template"})
		return
	}
	if !isTemplateOwned(template, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	if namePtr != nil {
		exists, err := h.repo.TemplateNameExistsForUser(c.Request.Context(), userID, *namePtr, &id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check template name"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "a template with this name already exists"})
			return
		}
	}

	updated, err := h.repo.UpdateTemplate(c.Request.Context(), id, namePtr, descPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update template"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *TemplateHandler) Delete(c *gin.Context) {
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

	template, err := h.repo.GetTemplateByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch template"})
		return
	}
	if !isTemplateOwned(template, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	if err := h.repo.DeleteTemplate(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete template"})
		return
	}

	c.Status(http.StatusNoContent)
}

// parseOptionalDescription validates description if present, leaving the
// result nil if the field was omitted. Writes the error response and
// returns ok=false if description is present but invalid.
func parseOptionalDescription(c *gin.Context, description *string) (*string, bool) {
	if description == nil {
		return nil, true
	}
	desc, ok := validateDescription(c, *description)
	if !ok {
		return nil, false
	}
	return &desc, true
}

// isTemplateOwned returns true only when the template exists and belongs to
// the given user. Templates have no system-level concept, unlike categories/
// items, so this is a plain equality check rather than a nil-means-system one.
func isTemplateOwned(template *models.Template, userID string) bool {
	return template != nil && template.UserID.String() == userID
}
