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
		SELECT id, name, description, user_id
		FROM templates
		WHERE user_id = $1
		ORDER BY updated_at DESC
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
	query := `SELECT id, name, description, user_id FROM templates WHERE id = $1`
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
		RETURNING id, name, description, user_id
	`
	row := r.db.QueryRowContext(ctx, query, userID, name, description)
	tmpl, err := scanTemplate(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	return tmpl, nil
}

// UpdateTemplate updates name and/or description. A nil name or description
// means "leave unchanged"; an empty-string description clears it to "" (not
// NULL) — there is no way to explicitly set description back to NULL.
func (r *TemplateRepository) UpdateTemplate(ctx context.Context, id string, name *string, description *string) (*models.Template, error) {
	query := `
		UPDATE templates
		SET name = COALESCE($1, name),
		    description = COALESCE($2, description),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING id, name, description, user_id
	`
	row := r.db.QueryRowContext(ctx, query, name, description, id)
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

// scanTemplate abstracts the nullable description scan pattern for a single
// template row. Items is always populated as an empty slice — the join
// against template_items is PACK-009's job.
func scanTemplate(scan func(...any) error) (*models.Template, error) {
	var (
		id              uuid.UUID
		name            string
		descriptionNull sql.NullString
		userID          uuid.UUID
	)
	if err := scan(&id, &name, &descriptionNull, &userID); err != nil {
		return nil, err
	}

	tmpl := &models.Template{ID: id, Name: name, UserID: userID, Items: []models.TemplateItem{}}
	if descriptionNull.Valid {
		tmpl.Description = &descriptionNull.String
	}
	return tmpl, nil
}
