package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

func (r *TemplateRepository) AddTemplateItem(ctx context.Context, templateID, itemID string, quantity int, notes *string) (*models.TemplateItem, error) {
	query := `
		WITH inserted AS (
			INSERT INTO template_items (template_id, item_id, quantity, notes)
			VALUES ($1, $2, $3, $4)
			RETURNING item_id, quantity, notes
		)
		SELECT inserted.item_id, items.name, inserted.quantity, inserted.notes
		FROM inserted JOIN items ON items.id = inserted.item_id
	`
	row := r.db.QueryRowContext(ctx, query, templateID, itemID, quantity, notes)
	templateItem, err := scanTemplateItem(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to add template item: %w", err)
	}
	return templateItem, nil
}

// UpdateTemplateItem updates quantity and/or notes. A nil quantity or notes
// means "leave unchanged"; an empty-string notes clears it to "" (not NULL).
func (r *TemplateRepository) UpdateTemplateItem(ctx context.Context, templateID, itemID string, quantity *int, notes *string) (*models.TemplateItem, error) {
	query := `
		WITH updated AS (
			UPDATE template_items
			SET quantity = COALESCE($1, quantity),
			    notes = COALESCE($2, notes)
			WHERE template_id = $3 AND item_id = $4
			RETURNING item_id, quantity, notes
		)
		SELECT updated.item_id, items.name, updated.quantity, updated.notes
		FROM updated JOIN items ON items.id = updated.item_id
	`
	row := r.db.QueryRowContext(ctx, query, quantity, notes, templateID, itemID)
	templateItem, err := scanTemplateItem(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to update template item: %w", err)
	}
	return templateItem, nil
}

func (r *TemplateRepository) RemoveTemplateItem(ctx context.Context, templateID, itemID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM template_items WHERE template_id = $1 AND item_id = $2`, templateID, itemID)
	if err != nil {
		return fmt.Errorf("failed to remove template item: %w", err)
	}
	return nil
}

func (r *TemplateRepository) TemplateItemExists(ctx context.Context, templateID, itemID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM template_items WHERE template_id = $1 AND item_id = $2)`
	if err := r.db.QueryRowContext(ctx, query, templateID, itemID).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check template item: %w", err)
	}
	return exists, nil
}

func (r *TemplateRepository) GetTemplateItems(ctx context.Context, templateID string) ([]models.TemplateItem, error) {
	query := `
		SELECT template_items.item_id, items.name, template_items.quantity, template_items.notes
		FROM template_items
		JOIN items ON items.id = template_items.item_id
		WHERE template_items.template_id = $1
		ORDER BY items.name
	`
	rows, err := r.db.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query template items: %w", err)
	}
	defer rows.Close()

	items := make([]models.TemplateItem, 0)
	for rows.Next() {
		item, err := scanTemplateItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template item: %w", err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// scanTemplateItem abstracts the nullable notes scan pattern for a single
// template_items row joined against items.name.
func scanTemplateItem(scan func(...any) error) (*models.TemplateItem, error) {
	var (
		itemID    uuid.UUID
		name      string
		quantity  int
		notesNull sql.NullString
	)
	if err := scan(&itemID, &name, &quantity, &notesNull); err != nil {
		return nil, err
	}

	item := &models.TemplateItem{ItemID: itemID, Name: name, Quantity: quantity}
	if notesNull.Valid {
		item.Notes = &notesNull.String
	}
	return item, nil
}
