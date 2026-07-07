package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/franciskershaw/packing-list-go/internal/models"
)

type TemplateRepository struct {
	db *sql.DB
}

func NewTemplateRepository(db *sql.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

// TODO(TDD): stub — implement once repository tests are written and red.

func (r *TemplateRepository) GetTemplates(ctx context.Context, userID string) ([]models.Template, error) {
	return nil, errors.New("not implemented")
}

func (r *TemplateRepository) GetTemplateByID(ctx context.Context, id string) (*models.Template, error) {
	return nil, errors.New("not implemented")
}

func (r *TemplateRepository) CreateTemplate(ctx context.Context, userID, name string, description *string) (*models.Template, error) {
	return nil, errors.New("not implemented")
}

func (r *TemplateRepository) UpdateTemplate(ctx context.Context, id string, name *string, descriptionSet bool, description *string) (*models.Template, error) {
	return nil, errors.New("not implemented")
}

func (r *TemplateRepository) DeleteTemplate(ctx context.Context, id string) error {
	return errors.New("not implemented")
}

func (r *TemplateRepository) TemplateNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error) {
	return false, errors.New("not implemented")
}
