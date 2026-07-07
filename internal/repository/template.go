package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

type TemplateRepository struct {
	db *sql.DB
}

func NewTemplateRepository(db *sql.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) GetTemplates(ctx context.Context, userID string) ([]models.Template, error) {
	query := `
		SELECT id, user_id, name, description
		FROM templates
		WHERE user_id = $1
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query templates: %w", err)
	}
	defer rows.Close()

	templates := make([]models.Template, 0)
	for rows.Next() {
		tmpl, err := scanTemplate(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}
		templates = append(templates, *tmpl)
	}
	return templates, rows.Err()
}

func (r *TemplateRepository) GetTemplateByID(ctx context.Context, id string) (*models.Template, error) {
	query := `SELECT id, user_id, name, description FROM templates WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	tmpl, err := scanTemplate(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return tmpl, nil
}

func (r *TemplateRepository) CreateTemplate(ctx context.Context, userID, name string, description *string) (*models.Template, error) {
	query := `
		INSERT INTO templates (user_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, description
	`
	row := r.db.QueryRowContext(ctx, query, userID, name, description)
	tmpl, err := scanTemplate(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	return tmpl, nil
}

// UpdateTemplate updates name (if non-nil) and description. descriptionSet
// distinguishes "description not provided" (leave unchanged) from "provided,
// possibly null" (set to the given value, which may be nil to clear it).
func (r *TemplateRepository) UpdateTemplate(ctx context.Context, id string, name *string, descriptionSet bool, description *string) (*models.Template, error) {
	query := `
		UPDATE templates
		SET name = COALESCE($1, name),
		    description = CASE WHEN $2 THEN $3 ELSE description END,
		    updated_at = now()
		WHERE id = $4
		RETURNING id, user_id, name, description
	`
	row := r.db.QueryRowContext(ctx, query, name, descriptionSet, description, id)
	tmpl, err := scanTemplate(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}
	return tmpl, nil
}

func (r *TemplateRepository) DeleteTemplate(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	return nil
}

func (r *TemplateRepository) TemplateNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error) {
	var exists bool
	var err error
	if excludeID != nil {
		err = r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM templates WHERE LOWER(name) = LOWER($1) AND user_id = $2 AND id != $3)`,
			name, userID, *excludeID,
		).Scan(&exists)
	} else {
		err = r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM templates WHERE LOWER(name) = LOWER($1) AND user_id = $2)`,
			name, userID,
		).Scan(&exists)
	}
	if err != nil {
		return false, fmt.Errorf("failed to check template name: %w", err)
	}
	return exists, nil
}

// scanTemplate scans a single template row. Items are always empty — there's
// no way to attach items to a template yet.
func scanTemplate(scan func(...any) error) (*models.Template, error) {
	var (
		id          uuid.UUID
		userID      uuid.UUID
		name        string
		description sql.NullString
	)
	if err := scan(&id, &userID, &name, &description); err != nil {
		return nil, err
	}

	tmpl := &models.Template{
		ID:     id,
		UserID: userID,
		Name:   name,
		Items:  make([]models.TemplateItem, 0),
	}
	if description.Valid {
		tmpl.Description = &description.String
	}
	return tmpl, nil
}
