package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO packing_lists (user_id, name, event_date, template_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, event_date, template_id, user_id
	`
	row := tx.QueryRowContext(ctx, query, userID, name, eventDate, templateID)
	packingList, err := scanPackingList(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create packing list: %w", err)
	}

	if templateID != nil {
		items, err := copyTemplateItemsTx(ctx, tx, packingList.ID, *templateID)
		if err != nil {
			return nil, err
		}
		packingList.Items = items
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return packingList, nil
}

// copyTemplateItemsTx copies every row in templateID's template_items into
// packing_list_items for listID, within tx, returning the copied items in
// the shape the caller returns to the client.
func copyTemplateItemsTx(ctx context.Context, tx *sql.Tx, listID uuid.UUID, templateID string) ([]models.PackingListItem, error) {
	query := `
		SELECT template_items.item_id, items.name, items.category_id, template_items.quantity, template_items.notes
		FROM template_items
		JOIN items ON items.id = template_items.item_id
		WHERE template_items.template_id = $1
		ORDER BY items.name
	`
	rows, err := tx.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query template items: %w", err)
	}

	type templateItemRow struct {
		itemID     uuid.UUID
		name       string
		categoryID uuid.UUID
		quantity   int
		notes      sql.NullString
	}
	var toCopy []templateItemRow
	for rows.Next() {
		var tir templateItemRow
		if err := rows.Scan(&tir.itemID, &tir.name, &tir.categoryID, &tir.quantity, &tir.notes); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan template item: %w", err)
		}
		toCopy = append(toCopy, tir)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("failed to read template items: %w", err)
	}
	rows.Close()

	items := make([]models.PackingListItem, 0, len(toCopy))
	for _, tir := range toCopy {
		var notesPtr *string
		if tir.notes.Valid {
			notesPtr = &tir.notes.String
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO packing_list_items (list_id, item_id, category_id, quantity, notes) VALUES ($1, $2, $3, $4, $5)`,
			listID, tir.itemID, tir.categoryID, tir.quantity, notesPtr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to copy template item: %w", err)
		}
		items = append(items, models.PackingListItem{
			ItemID:     tir.itemID,
			Name:       tir.name,
			CategoryID: tir.categoryID,
			Quantity:   tir.quantity,
			Notes:      notesPtr,
			IsPacked:   false,
		})
	}
	return items, nil
}

// scanPackingList abstracts the nullable event_date/template_id scan
// pattern for a single packing_lists row. Items is left as an empty slice —
// CreatePackingList populates it when a template was copied from.
func scanPackingList(scan func(...any) error) (*models.PackingList, error) {
	var (
		id             uuid.UUID
		name           string
		eventDateNull  sql.NullTime
		templateIDNull sql.NullString
		userID         uuid.UUID
	)
	if err := scan(&id, &name, &eventDateNull, &templateIDNull, &userID); err != nil {
		return nil, err
	}

	packingList := &models.PackingList{ID: id, Name: name, UserID: userID, Items: []models.PackingListItem{}}
	if eventDateNull.Valid {
		formatted := eventDateNull.Time.Format("2006-01-02")
		packingList.EventDate = &formatted
	}
	if templateIDNull.Valid {
		templateID, err := uuid.Parse(templateIDNull.String)
		if err != nil {
			return nil, fmt.Errorf("invalid template_id UUID %q: %w", templateIDNull.String, err)
		}
		packingList.TemplateID = &templateID
	}
	return packingList, nil
}
