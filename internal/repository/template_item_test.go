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

// createTestItem inserts a user-owned item under catID and returns its ID,
// registering cleanup.
func createTestItem(t *testing.T, catID uuid.UUID) uuid.UUID {
	t.Helper()
	item, err := itemRepo.CreateItem(context.Background(), repoUserID.String(), "repo-test-tmplitem-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })
	return item.ID
}

// createTestTemplate creates a template owned by repoUserID and returns its ID,
// registering cleanup.
func createTestTemplate(t *testing.T) uuid.UUID {
	t.Helper()
	tmpl, err := templateRepo.CreateTemplate(context.Background(), repoUserID.String(), "repo-test-tmplitem-tmpl-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, tmpl.ID) })
	return tmpl.ID
}

func TestAddTemplateItem_NoNotes(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	added, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 3, nil)
	require.NoError(t, err)
	require.NotNil(t, added)
	assert.Equal(t, itemID, added.ItemID)
	assert.Equal(t, 3, added.Quantity)
	assert.Nil(t, added.Notes)
}

func TestAddTemplateItem_WithNotes(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	notes := "pack two pairs"

	added, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, &notes)
	require.NoError(t, err)
	require.NotNil(t, added)
	require.NotNil(t, added.Notes)
	assert.Equal(t, notes, *added.Notes)
}

func TestUpdateTemplateItem_QuantityOnly(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	notes := "keep me"
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, &notes)
	require.NoError(t, err)

	newQty := 5
	updated, err := templateRepo.UpdateTemplateItem(ctx, tmplID.String(), itemID.String(), &newQty, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 5, updated.Quantity)
	require.NotNil(t, updated.Notes)
	assert.Equal(t, notes, *updated.Notes)
}

func TestUpdateTemplateItem_NotesOnly(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 2, nil)
	require.NoError(t, err)

	newNotes := "new notes"
	updated, err := templateRepo.UpdateTemplateItem(ctx, tmplID.String(), itemID.String(), nil, &newNotes)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 2, updated.Quantity)
	require.NotNil(t, updated.Notes)
	assert.Equal(t, newNotes, *updated.Notes)
}

func TestUpdateTemplateItem_Both(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	newQty := 7
	newNotes := "both updated"
	updated, err := templateRepo.UpdateTemplateItem(ctx, tmplID.String(), itemID.String(), &newQty, &newNotes)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 7, updated.Quantity)
	require.NotNil(t, updated.Notes)
	assert.Equal(t, newNotes, *updated.Notes)
}

func TestUpdateTemplateItem_EmptyStringNotesNotNull(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	notes := "will be cleared"
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, &notes)
	require.NoError(t, err)

	empty := ""
	updated, err := templateRepo.UpdateTemplateItem(ctx, tmplID.String(), itemID.String(), nil, &empty)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Notes, "expected empty string, not NULL")
	assert.Equal(t, "", *updated.Notes)
}

func TestRemoveTemplateItem(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	err = templateRepo.RemoveTemplateItem(ctx, tmplID.String(), itemID.String())
	require.NoError(t, err)

	exists, err := templateRepo.TemplateItemExists(ctx, tmplID.String(), itemID.String())
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTemplateItemExists_True(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	exists, err := templateRepo.TemplateItemExists(ctx, tmplID.String(), itemID.String())
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTemplateItemExists_False(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	exists, err := templateRepo.TemplateItemExists(ctx, tmplID.String(), itemID.String())
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGetTemplateItems_Multiple(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemA := createTestItem(t, catID)
	itemB := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemA.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, tmplID.String(), itemB.String(), 2, nil)
	require.NoError(t, err)

	items, err := templateRepo.GetTemplateItems(ctx, tmplID.String())
	require.NoError(t, err)
	require.Len(t, items, 2)

	var foundA, foundB bool
	for _, it := range items {
		if it.ItemID == itemA {
			foundA = true
		}
		if it.ItemID == itemB {
			foundB = true
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)
}

func TestBulkUpdateTemplateItems_AddsUpdatesAndRemoves(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemToUpdate := createTestItem(t, catID)
	itemToRemove := createTestItem(t, catID)
	itemToAdd := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemToUpdate.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, tmplID.String(), itemToRemove.String(), 1, nil)
	require.NoError(t, err)

	changes := map[string]int{
		itemToUpdate.String(): 5,
		itemToRemove.String(): 0,
		itemToAdd.String():    2,
	}
	err = templateRepo.BulkUpdateTemplateItems(ctx, tmplID.String(), changes)
	require.NoError(t, err)

	items, err := templateRepo.GetTemplateItems(ctx, tmplID.String())
	require.NoError(t, err)
	require.Len(t, items, 2, "itemToRemove should be gone, itemToUpdate and itemToAdd should remain")

	byID := make(map[uuid.UUID]models.TemplateItem, len(items))
	for _, item := range items {
		byID[item.ItemID] = item
	}
	updated, ok := byID[itemToUpdate]
	require.True(t, ok, "itemToUpdate should still be on the template")
	assert.Equal(t, 5, updated.Quantity)
	added, ok := byID[itemToAdd]
	require.True(t, ok, "itemToAdd should have been added")
	assert.Equal(t, 2, added.Quantity)
	_, stillPresent := byID[itemToRemove]
	assert.False(t, stillPresent, "itemToRemove should have been deleted")
}

func TestBulkUpdateTemplateItems_RollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemValid := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemValid.String(), 1, nil)
	require.NoError(t, err)

	changes := map[string]int{
		itemValid.String(): 9,
		uuid.NewString():   3, // references no row in items — violates the item_id FK
	}
	err = templateRepo.BulkUpdateTemplateItems(ctx, tmplID.String(), changes)
	require.Error(t, err)

	items, err := templateRepo.GetTemplateItems(ctx, tmplID.String())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 1, items[0].Quantity, "the valid item's quantity must be unchanged — the whole batch should have rolled back")
}

func TestGetTemplateItems_Empty(t *testing.T) {
	ctx := context.Background()
	tmplID := createTestTemplate(t)

	items, err := templateRepo.GetTemplateItems(ctx, tmplID.String())
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestGetTemplateByID_ReturnsItemsWithoutDuplication(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemA := createTestItem(t, catID)
	itemB := createTestItem(t, catID)
	tmplID := createTestTemplate(t)

	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemA.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, tmplID.String(), itemB.String(), 2, nil)
	require.NoError(t, err)

	found, err := templateRepo.GetTemplateByID(ctx, tmplID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tmplID, found.ID)
	require.Len(t, found.Items, 2, "expected exactly one Template with both items, not one row per item")

	var foundA, foundB bool
	for _, it := range found.Items {
		if it.ItemID == itemA {
			foundA = true
		}
		if it.ItemID == itemB {
			foundB = true
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)
}

func TestGetTemplateByID_NoItems(t *testing.T) {
	ctx := context.Background()
	tmplID := createTestTemplate(t)

	found, err := templateRepo.GetTemplateByID(ctx, tmplID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, []models.TemplateItem{}, found.Items)
}
