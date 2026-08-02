package repository_test

import (
	"context"
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/stretchr/testify/assert"
)

// Every FK/user_id column should have an index beyond the primary key.
func TestIndexes_ExistOnFKAndUserIDColumns(t *testing.T) {
	ctx := context.Background()

	expected := []struct {
		table string
		index string
	}{
		{"categories", "idx_categories_user_id"},
		{"items", "idx_items_category_id"},
		{"items", "idx_items_user_id"},
		{"templates", "idx_templates_user_id"},
		{"template_items", "idx_template_items_template_id"},
		{"template_items", "idx_template_items_item_id"},
		{"packing_lists", "idx_packing_lists_user_id"},
		{"packing_lists", "idx_packing_lists_template_id"},
		{"packing_list_items", "idx_packing_list_items_list_id"},
		{"packing_list_items", "idx_packing_list_items_item_id"},
		{"packing_list_items", "idx_packing_list_items_category_id"},
	}

	for _, exp := range expected {
		exp := exp
		t.Run(exp.index, func(t *testing.T) {
			var found string
			err := db.DB.QueryRowContext(ctx,
				`SELECT indexname FROM pg_indexes WHERE tablename = $1 AND indexname = $2`,
				exp.table, exp.index,
			).Scan(&found)
			assert.NoError(t, err, "expected index %s on table %s to exist", exp.index, exp.table)
			assert.Equal(t, exp.index, found)
		})
	}
}
