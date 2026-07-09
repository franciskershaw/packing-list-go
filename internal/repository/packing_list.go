package repository

import (
	"context"
	"database/sql"

	"github.com/franciskershaw/packing-list-go/internal/models"
)

type PackingListRepository struct {
	db *sql.DB
}

func NewPackingListRepository(db *sql.DB) *PackingListRepository {
	return &PackingListRepository{db: db}
}

// CreatePackingList creates a packing list for userID. If templateID is
// non-nil, every row in that template's template_items is copied into
// packing_list_items (item_id/quantity/notes preserved, category_id
// populated from items.category_id, is_packed false, sort_order NULL),
// atomically with the list insert.
func (r *PackingListRepository) CreatePackingList(ctx context.Context, userID, name string, eventDate *string, templateID *string) (*models.PackingList, error) {
	panic("not implemented")
}
