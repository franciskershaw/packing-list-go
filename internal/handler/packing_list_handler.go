package handler

import (
	"context"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
)

type PackingListRepository interface {
	CreatePackingList(ctx context.Context, userID, name string, eventDate *string, templateID *string) (*models.PackingList, error)
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
	panic("not implemented")
}
