package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ItemRepository struct {
	db *sql.DB
}

func NewItemRepository(db *sql.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

func (r *ItemRepository) GetItems(ctx context.Context, userID string, categoryID *string, search *string) ([]models.Item, error) {
	query := `
		SELECT id, name, category_id, user_id
		FROM items
		WHERE (user_id IS NULL OR user_id = $1)
		  AND ($2::uuid IS NULL OR category_id = $2::uuid)
		  AND ($3::text IS NULL OR name ILIKE '%' || $3 || '%')
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query, userID, categoryID, search)
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			slog.Error("failed to close rows", "err", cerr)
		}
	}()

	items := make([]models.Item, 0)
	for rows.Next() {
		item, err := scanItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// GetItemsByIDs returns every item matching ids, in no particular order.
// Unknown IDs are silently omitted, not errored — mirrors GetItemByID's
// not-found-is-nil convention rather than GetItemByID's per-ID error shape.
func (r *ItemRepository) GetItemsByIDs(ctx context.Context, ids []string) ([]models.Item, error) {
	if len(ids) == 0 {
		return []models.Item{}, nil
	}

	query := `SELECT id, name, category_id, user_id FROM items WHERE id = ANY($1)`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("failed to query items by ids: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			slog.Error("failed to close rows", "err", cerr)
		}
	}()

	items := make([]models.Item, 0, len(ids))
	for rows.Next() {
		item, err := scanItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *ItemRepository) GetItemByID(ctx context.Context, id string) (*models.Item, error) {
	query := `SELECT id, name, category_id, user_id FROM items WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	item, err := scanItem(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	return item, nil
}

func (r *ItemRepository) CreateItem(ctx context.Context, userID, name, categoryID string) (*models.Item, error) {
	query := `
		INSERT INTO items (user_id, name, category_id)
		VALUES ($1, $2, $3)
		RETURNING id, name, category_id, user_id
	`
	row := r.db.QueryRowContext(ctx, query, userID, name, categoryID)
	item, err := scanItem(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}
	return item, nil
}

func (r *ItemRepository) UpdateItem(ctx context.Context, id string, name *string, categoryID *string) (*models.Item, error) {
	query := `
		UPDATE items
		SET name = COALESCE($1, name),
		    category_id = COALESCE($2::uuid, category_id)
		WHERE id = $3
		RETURNING id, name, category_id, user_id
	`
	row := r.db.QueryRowContext(ctx, query, name, categoryID, id)
	item, err := scanItem(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to update item: %w", err)
	}
	return item, nil
}

func (r *ItemRepository) DeleteItem(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}
	return nil
}

func (r *ItemRepository) ItemNameExistsInCategory(ctx context.Context, categoryID, name string, excludeID *string) (bool, error) {
	var exists bool
	var err error
	if excludeID != nil {
		err = r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM items WHERE category_id = $1 AND LOWER(name) = LOWER($2) AND id != $3)`,
			categoryID, name, *excludeID,
		).Scan(&exists)
	} else {
		err = r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM items WHERE category_id = $1 AND LOWER(name) = LOWER($2))`,
			categoryID, name,
		).Scan(&exists)
	}
	if err != nil {
		return false, fmt.Errorf("failed to check item name: %w", err)
	}
	return exists, nil
}

func (r *ItemRepository) ItemIsInUse(ctx context.Context, id string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM template_items WHERE item_id = $1
			UNION ALL
			SELECT 1 FROM packing_list_items pli
			JOIN packing_lists pl ON pl.id = pli.list_id
			WHERE pli.item_id = $1 AND pl.archived_at IS NULL
		)
	`
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check item usage: %w", err)
	}
	return exists, nil
}

func (r *ItemRepository) CategoryIsAccessible(ctx context.Context, categoryID, userID string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM categories
			WHERE id = $1 AND (user_id IS NULL OR user_id = $2)
		)
	`
	if err := r.db.QueryRowContext(ctx, query, categoryID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check category access: %w", err)
	}
	return exists, nil
}

// scanItem abstracts the nullable user_id scan pattern for a single item row.
func scanItem(scan func(...any) error) (*models.Item, error) {
	var (
		id         uuid.UUID
		name       string
		categoryID uuid.UUID
		userIDNull sql.NullString
	)
	if err := scan(&id, &name, &categoryID, &userIDNull); err != nil {
		return nil, err
	}

	item := &models.Item{ID: id, Name: name, CategoryID: categoryID}
	if userIDNull.Valid {
		uid, err := uuid.Parse(userIDNull.String)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id UUID %q: %w", userIDNull.String, err)
		}
		item.UserID = &uid
	}
	item.IsSystem = item.UserID == nil
	return item, nil
}
