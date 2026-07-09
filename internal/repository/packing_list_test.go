package repository_test

import (
	"context"
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePackingList_NameOnly(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-list-" + uuid.NewString()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), name, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	assert.Equal(t, name, created.Name)
	assert.Nil(t, created.EventDate)
	assert.Nil(t, created.TemplateID)
	assert.Equal(t, repoUserID, created.UserID)
	assert.Equal(t, []models.PackingListItem{}, created.Items)
}

func TestCreatePackingList_WithEventDate(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-list-date-" + uuid.NewString()
	eventDate := "2026-08-01"

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), name, &eventDate, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	require.NotNil(t, created.EventDate)
	assert.Equal(t, eventDate, *created.EventDate)
}

func TestCreatePackingList_WithTemplateItemsCopiesFidelity(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemA := createTestItem(t, catID)
	itemB := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	notesA := "pack the blue ones"
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemA.String(), 3, &notesA)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, tmplID.String(), itemB.String(), 1, nil)
	require.NoError(t, err)

	name := "repo-test-list-tmpl-" + uuid.NewString()
	tmplIDStr := tmplID.String()
	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), name, nil, &tmplIDStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	require.NotNil(t, created.TemplateID)
	assert.Equal(t, tmplID, *created.TemplateID)
	require.Len(t, created.Items, 2)

	var foundA, foundB bool
	for _, item := range created.Items {
		assert.Equal(t, catID, item.CategoryID)
		assert.False(t, item.IsPacked)
		if item.ItemID == itemA {
			foundA = true
			assert.Equal(t, 3, item.Quantity)
			require.NotNil(t, item.Notes)
			assert.Equal(t, notesA, *item.Notes)
		}
		if item.ItemID == itemB {
			foundB = true
			assert.Equal(t, 1, item.Quantity)
			assert.Nil(t, item.Notes)
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)

	// sort_order isn't exposed on the model — assert directly against the DB
	// that it was left NULL for every copied row, per the PACK-010 decision.
	rows, err := db.DB.QueryContext(ctx, `SELECT sort_order FROM packing_list_items WHERE list_id = $1`, created.ID)
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var sortOrder *int
		require.NoError(t, rows.Scan(&sortOrder))
		assert.Nil(t, sortOrder)
		count++
	}
	assert.Equal(t, 2, count)
}

func TestCreatePackingList_WithEmptyTemplate(t *testing.T) {
	ctx := context.Background()
	tmplID := createTestTemplate(t)
	tmplIDStr := tmplID.String()
	name := "repo-test-list-empty-tmpl-" + uuid.NewString()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), name, nil, &tmplIDStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	assert.Equal(t, []models.PackingListItem{}, created.Items)
}
