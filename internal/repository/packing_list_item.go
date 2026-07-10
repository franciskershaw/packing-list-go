package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

// AddPackingListItem adds itemID to listID, populating category_id from the
// item's own category (not caller-supplied).
func (r *PackingListRepository) AddPackingListItem(ctx context.Context, listID, itemID string, quantity int, notes *string) (*models.PackingListItem, error) {
	query := `
		WITH inserted AS (
			INSERT INTO packing_list_items (list_id, item_id, category_id, quantity, notes)
			SELECT $1, $2, items.category_id, $3, $4
			FROM items WHERE items.id = $2
			RETURNING item_id, category_id, quantity, notes, is_packed, sort_order
		)
		SELECT inserted.item_id, items.name, inserted.category_id, inserted.quantity, inserted.notes, inserted.is_packed, inserted.sort_order
		FROM inserted JOIN items ON items.id = inserted.item_id
	`
	row := r.db.QueryRowContext(ctx, query, listID, itemID, quantity, notes)
	item, err := scanPackingListItem(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to add packing list item: %w", err)
	}
	return item, nil
}

// UpdatePackingListItem updates quantity/notes/sort_order/is_packed (nil =
// unchanged for each field independently).
//
// PACK-013 stub: isPacked is accepted (signature matches the interface) but
// not yet persisted — the COALESCE list below doesn't include it yet. This
// keeps the pre-existing quantity/notes/sortOrder behavior real and passing
// while leaving is_packed's actual persistence for Phase A to implement;
// tests asserting isPacked took effect fail on the assertion, not a panic
// or build error.
func (r *PackingListRepository) UpdatePackingListItem(ctx context.Context, listID, itemID string, quantity *int, notes *string, sortOrder *int, isPacked *bool) (*models.PackingListItem, error) {
	query := `
		WITH updated AS (
			UPDATE packing_list_items
			SET quantity = COALESCE($1, quantity),
			    notes = COALESCE($2, notes),
			    sort_order = COALESCE($3, sort_order)
			WHERE list_id = $4 AND item_id = $5
			RETURNING item_id, category_id, quantity, notes, is_packed, sort_order
		)
		SELECT updated.item_id, items.name, updated.category_id, updated.quantity, updated.notes, updated.is_packed, updated.sort_order
		FROM updated JOIN items ON items.id = updated.item_id
	`
	row := r.db.QueryRowContext(ctx, query, quantity, notes, sortOrder, listID, itemID)
	item, err := scanPackingListItem(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to update packing list item: %w", err)
	}
	return item, nil
}

// PackAllItems sets is_packed = true for every item on listID. PACK-013
// stub, not yet implemented.
func (r *PackingListRepository) PackAllItems(ctx context.Context, listID string) error {
	return errors.New("not implemented")
}

// UnpackAllItems sets is_packed = false for every item on listID. PACK-013
// stub, not yet implemented.
func (r *PackingListRepository) UnpackAllItems(ctx context.Context, listID string) error {
	return errors.New("not implemented")
}

// RemovePackingListItem removes itemID from listID.
func (r *PackingListRepository) RemovePackingListItem(ctx context.Context, listID, itemID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM packing_list_items WHERE list_id = $1 AND item_id = $2`, listID, itemID)
	if err != nil {
		return fmt.Errorf("failed to remove packing list item: %w", err)
	}
	return nil
}

// PackingListItemExists reports whether itemID is on listID.
func (r *PackingListRepository) PackingListItemExists(ctx context.Context, listID, itemID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM packing_list_items WHERE list_id = $1 AND item_id = $2)`
	if err := r.db.QueryRowContext(ctx, query, listID, itemID).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check packing list item: %w", err)
	}
	return exists, nil
}

// GetPackingListItems returns listID's items flat, ordered by item_id for
// determinism — used by BulkAddItems' duplicate check, not for display.
func (r *PackingListRepository) GetPackingListItems(ctx context.Context, listID string) ([]models.PackingListItem, error) {
	query := `
		SELECT pli.item_id, items.name, pli.category_id, pli.quantity, pli.notes, pli.is_packed, pli.sort_order
		FROM packing_list_items pli
		JOIN items ON items.id = pli.item_id
		WHERE pli.list_id = $1
		ORDER BY pli.item_id
	`
	rows, err := r.db.QueryContext(ctx, query, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to query packing list items: %w", err)
	}
	defer rows.Close()

	items := make([]models.PackingListItem, 0)
	for rows.Next() {
		item, err := scanPackingListItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan packing list item: %w", err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// scanPackingListItem abstracts the nullable notes/sort_order scan pattern
// for a single packing_list_items row joined against items.name.
func scanPackingListItem(scan func(...any) error) (*models.PackingListItem, error) {
	var (
		itemID        uuid.UUID
		name          string
		categoryID    uuid.UUID
		quantity      int
		notesNull     sql.NullString
		isPacked      bool
		sortOrderNull sql.NullInt64
	)
	if err := scan(&itemID, &name, &categoryID, &quantity, &notesNull, &isPacked, &sortOrderNull); err != nil {
		return nil, err
	}

	item := &models.PackingListItem{
		ItemID:     itemID,
		Name:       name,
		CategoryID: categoryID,
		Quantity:   quantity,
		IsPacked:   isPacked,
	}
	if notesNull.Valid {
		item.Notes = &notesNull.String
	}
	if sortOrderNull.Valid {
		sortOrder := int(sortOrderNull.Int64)
		item.SortOrder = &sortOrder
	}
	return item, nil
}
