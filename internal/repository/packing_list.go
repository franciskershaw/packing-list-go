package repository

import (
	"context"
	"database/sql"
	"errors"
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

// GetPackingLists returns userID's lists — active (archived_at IS NULL,
// ordered by updated_at DESC) or archived (archived_at IS NOT NULL, ordered
// by archived_at DESC) depending on archived. Items is always left empty —
// list mode never populates it, matching GetTemplates' precedent.
//
// item_count/packed_count are correlated subqueries rather than
// scanPackingList's shared scan — no other caller's query selects these
// columns, and a LEFT JOIN/GROUP BY would drag ORDER BY into the group-by
// list. Mirrors GetTemplates' own ItemCount precedent exactly.
func (r *PackingListRepository) GetPackingLists(ctx context.Context, userID string, archived bool) ([]models.PackingList, error) {
	query := `
		SELECT id, name, event_date, template_id, user_id,
			(SELECT COUNT(*) FROM packing_list_items WHERE list_id = packing_lists.id) AS item_count,
			(SELECT COUNT(*) FROM packing_list_items WHERE list_id = packing_lists.id AND is_packed = true) AS packed_count
		FROM packing_lists
		WHERE user_id = $1 AND archived_at IS NULL
		ORDER BY updated_at DESC
	`
	if archived {
		query = `
			SELECT id, name, event_date, template_id, user_id,
				(SELECT COUNT(*) FROM packing_list_items WHERE list_id = packing_lists.id) AS item_count,
				(SELECT COUNT(*) FROM packing_list_items WHERE list_id = packing_lists.id AND is_packed = true) AS packed_count
			FROM packing_lists
			WHERE user_id = $1 AND archived_at IS NOT NULL
			ORDER BY archived_at DESC
		`
	}

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query packing lists: %w", err)
	}
	defer rows.Close()

	lists := make([]models.PackingList, 0)
	for rows.Next() {
		var (
			id             uuid.UUID
			name           string
			eventDateNull  sql.NullTime
			templateIDNull sql.NullString
			userIDScanned  uuid.UUID
			itemCount      int
			packedCount    int
		)
		if err := rows.Scan(&id, &name, &eventDateNull, &templateIDNull, &userIDScanned, &itemCount, &packedCount); err != nil {
			return nil, fmt.Errorf("failed to scan packing list: %w", err)
		}
		list := models.PackingList{
			ID:          id,
			Name:        name,
			UserID:      userIDScanned,
			Items:       []models.PackingListItem{},
			ItemCount:   itemCount,
			PackedCount: packedCount,
		}
		if eventDateNull.Valid {
			formatted := eventDateNull.Time.Format("2006-01-02")
			list.EventDate = &formatted
		}
		if templateIDNull.Valid {
			templateID, err := uuid.Parse(templateIDNull.String)
			if err != nil {
				return nil, fmt.Errorf("invalid template_id UUID %q: %w", templateIDNull.String, err)
			}
			list.TemplateID = &templateID
		}
		lists = append(lists, list)
	}
	return lists, rows.Err()
}

// GetPackingListByID fetches a single list with its items grouped by
// category, regardless of archived state — archiving only changes which
// GetPackingLists view a list appears in, not whether its detail is
// reachable.
func (r *PackingListRepository) GetPackingListByID(ctx context.Context, id string) (*models.PackingListDetail, error) {
	query := `SELECT id, name, event_date, template_id, user_id FROM packing_lists WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	base, err := scanPackingList(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get packing list: %w", err)
	}

	categories, err := r.getPackingListCategories(ctx, id)
	if err != nil {
		return nil, err
	}

	return &models.PackingListDetail{
		ID:         base.ID,
		Name:       base.Name,
		EventDate:  base.EventDate,
		TemplateID: base.TemplateID,
		Categories: categories,
		UserID:     base.UserID,
	}, nil
}

// UpdatePackingList updates name and/or eventDate (nil = unchanged) and
// returns the full grouped detail, re-fetched — mirrors UpdateTemplate
// returning the full Template with items re-fetched rather than just the
// bare updated columns.
func (r *PackingListRepository) UpdatePackingList(ctx context.Context, id string, name *string, eventDate *string) (*models.PackingListDetail, error) {
	query := `
		UPDATE packing_lists
		SET name = COALESCE($1, name),
		    event_date = COALESCE($2, event_date),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING id, name, event_date, template_id, user_id
	`
	row := r.db.QueryRowContext(ctx, query, name, eventDate, id)
	base, err := scanPackingList(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to update packing list: %w", err)
	}

	categories, err := r.getPackingListCategories(ctx, id)
	if err != nil {
		return nil, err
	}

	return &models.PackingListDetail{
		ID:         base.ID,
		Name:       base.Name,
		EventDate:  base.EventDate,
		TemplateID: base.TemplateID,
		Categories: categories,
		UserID:     base.UserID,
	}, nil
}

// ArchivePackingList sets archived_at unconditionally, so calling it again
// on an already-archived list is a no-op success, not an error.
func (r *PackingListRepository) ArchivePackingList(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE packing_lists SET archived_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to archive packing list: %w", err)
	}
	return nil
}

// UnarchivePackingList clears archived_at unconditionally, so calling it
// again on an already-active list is a no-op success, not an error —
// mirrors ArchivePackingList.
func (r *PackingListRepository) UnarchivePackingList(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE packing_lists SET archived_at = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to unarchive packing list: %w", err)
	}
	return nil
}

// getPackingListCategories groups listID's packing_list_items by category,
// categories ordered alphabetically by name. Within each category, items are
// ordered by sort_order (NULLS LAST) then alphabetically by name — explicitly
// ordered items surface first in that order, anything still NULL falls back
// to alphabetical. Only categories with at least one item on this list
// appear.
func (r *PackingListRepository) getPackingListCategories(ctx context.Context, listID string) ([]models.PackingListCategory, error) {
	query := `
		SELECT c.id, c.name, pli.item_id, i.name, pli.quantity, pli.notes, pli.is_packed, pli.sort_order
		FROM packing_list_items pli
		JOIN items i ON i.id = pli.item_id
		JOIN categories c ON c.id = pli.category_id
		WHERE pli.list_id = $1
		ORDER BY c.name, pli.sort_order NULLS LAST, i.name
	`
	rows, err := r.db.QueryContext(ctx, query, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to query packing list items: %w", err)
	}
	defer rows.Close()

	categories := make([]models.PackingListCategory, 0)
	var current *models.PackingListCategory
	for rows.Next() {
		var (
			categoryID   uuid.UUID
			categoryName string
			itemID       uuid.UUID
			itemName     string
			quantity     int
			notes        sql.NullString
			isPacked     bool
			sortOrder    sql.NullInt64
		)
		if err := rows.Scan(&categoryID, &categoryName, &itemID, &itemName, &quantity, &notes, &isPacked, &sortOrder); err != nil {
			return nil, fmt.Errorf("failed to scan packing list item: %w", err)
		}

		if current == nil || current.ID != categoryID {
			categories = append(categories, models.PackingListCategory{
				ID:    categoryID,
				Name:  categoryName,
				Items: []models.PackingListDetailItem{},
			})
			current = &categories[len(categories)-1]
		}

		var notesPtr *string
		if notes.Valid {
			notesPtr = &notes.String
		}
		var sortOrderPtr *int
		if sortOrder.Valid {
			sortOrderInt := int(sortOrder.Int64)
			sortOrderPtr = &sortOrderInt
		}
		current.Items = append(current.Items, models.PackingListDetailItem{
			ItemID:    itemID,
			Name:      itemName,
			Quantity:  quantity,
			Notes:     notesPtr,
			IsPacked:  isPacked,
			SortOrder: sortOrderPtr,
		})
	}
	return categories, rows.Err()
}
