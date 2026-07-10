package repository

import (
	"context"
	"errors"

	"github.com/franciskershaw/packing-list-go/internal/models"
)

// AddPackingListItem adds itemID to listID, populating category_id from the
// item's own category. PACK-012 stub, not yet implemented.
func (r *PackingListRepository) AddPackingListItem(ctx context.Context, listID, itemID string, quantity int, notes *string) (*models.PackingListItem, error) {
	return nil, errors.New("not implemented")
}

// UpdatePackingListItem updates quantity/notes/sort_order (nil = unchanged
// for each). PACK-012 stub, not yet implemented.
func (r *PackingListRepository) UpdatePackingListItem(ctx context.Context, listID, itemID string, quantity *int, notes *string, sortOrder *int) (*models.PackingListItem, error) {
	return nil, errors.New("not implemented")
}

// RemovePackingListItem removes itemID from listID. PACK-012 stub, not yet
// implemented.
func (r *PackingListRepository) RemovePackingListItem(ctx context.Context, listID, itemID string) error {
	return errors.New("not implemented")
}

// PackingListItemExists reports whether itemID is on listID. PACK-012 stub,
// not yet implemented.
func (r *PackingListRepository) PackingListItemExists(ctx context.Context, listID, itemID string) (bool, error) {
	return false, errors.New("not implemented")
}

// GetPackingListItems returns listID's items flat (unordered grouping),
// used by BulkAddItems' duplicate check. PACK-012 stub, not yet
// implemented.
func (r *PackingListRepository) GetPackingListItems(ctx context.Context, listID string) ([]models.PackingListItem, error) {
	return nil, errors.New("not implemented")
}
