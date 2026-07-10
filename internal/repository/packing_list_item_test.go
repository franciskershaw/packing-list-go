package repository_test

import (
	"context"
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestPackingList creates a name-only packing list owned by
// repoUserID and returns its ID, registering cleanup.
func createTestPackingList(t *testing.T) uuid.UUID {
	t.Helper()
	list, err := packingListRepo.CreatePackingList(context.Background(), repoUserID.String(), "repo-test-pli-list-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, list.ID) })
	return list.ID
}

func TestAddPackingListItem_NoNotes(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)

	added, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 3, nil)
	require.NoError(t, err)
	require.NotNil(t, added)
	assert.Equal(t, itemID, added.ItemID)
	assert.Equal(t, 3, added.Quantity)
	assert.Nil(t, added.Notes)
	assert.False(t, added.IsPacked)
	assert.Nil(t, added.SortOrder)
}

func TestAddPackingListItem_WithNotes(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	notes := "pack two pairs"

	added, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, &notes)
	require.NoError(t, err)
	require.NotNil(t, added)
	require.NotNil(t, added.Notes)
	assert.Equal(t, notes, *added.Notes)
}

func TestAddPackingListItem_PopulatesCategoryFromItem(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)

	added, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)
	require.NotNil(t, added)
	assert.Equal(t, catID, added.CategoryID)
}

func TestUpdatePackingListItem_QuantityOnly(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	newQty := 5
	updated, err := packingListRepo.UpdatePackingListItem(ctx, listID.String(), itemID.String(), &newQty, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 5, updated.Quantity)
	assert.Nil(t, updated.Notes)
	assert.Nil(t, updated.SortOrder)
}

func TestUpdatePackingListItem_NotesOnly(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 2, nil)
	require.NoError(t, err)

	notes := "bring spares"
	updated, err := packingListRepo.UpdatePackingListItem(ctx, listID.String(), itemID.String(), nil, &notes, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 2, updated.Quantity, "quantity should be unchanged when only notes is updated")
	require.NotNil(t, updated.Notes)
	assert.Equal(t, notes, *updated.Notes)
}

func TestUpdatePackingListItem_SortOrderOnly(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	sortOrder := -3
	updated, err := packingListRepo.UpdatePackingListItem(ctx, listID.String(), itemID.String(), nil, nil, &sortOrder)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.SortOrder)
	assert.Equal(t, -3, *updated.SortOrder, "negative sortOrder values must be accepted")
}

func TestUpdatePackingListItem_AllThree(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	qty := 4
	notes := "combined update"
	sortOrder := 0
	updated, err := packingListRepo.UpdatePackingListItem(ctx, listID.String(), itemID.String(), &qty, &notes, &sortOrder)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 4, updated.Quantity)
	require.NotNil(t, updated.Notes)
	assert.Equal(t, notes, *updated.Notes)
	require.NotNil(t, updated.SortOrder)
	assert.Equal(t, 0, *updated.SortOrder, "zero must be accepted as a real sortOrder value, not treated as absent")
}

func TestRemovePackingListItem(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	err = packingListRepo.RemovePackingListItem(ctx, listID.String(), itemID.String())
	require.NoError(t, err)

	exists, err := packingListRepo.PackingListItemExists(ctx, listID.String(), itemID.String())
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestPackingListItemExists_True(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	exists, err := packingListRepo.PackingListItemExists(ctx, listID.String(), itemID.String())
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestPackingListItemExists_False(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	listID := createTestPackingList(t)

	exists, err := packingListRepo.PackingListItemExists(ctx, listID.String(), itemID.String())
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGetPackingListItems_Flat(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemA := createTestItem(t, catID)
	itemB := createTestItem(t, catID)
	listID := createTestPackingList(t)
	_, err := packingListRepo.AddPackingListItem(ctx, listID.String(), itemA.String(), 1, nil)
	require.NoError(t, err)
	_, err = packingListRepo.AddPackingListItem(ctx, listID.String(), itemB.String(), 2, nil)
	require.NoError(t, err)

	items, err := packingListRepo.GetPackingListItems(ctx, listID.String())
	require.NoError(t, err)
	require.Len(t, items, 2)

	var foundA, foundB bool
	for _, item := range items {
		assert.Equal(t, catID, item.CategoryID)
		if item.ItemID == itemA {
			foundA = true
		}
		if item.ItemID == itemB {
			foundB = true
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)
}

func TestGetPackingListItems_EmptyForListWithNoItems(t *testing.T) {
	ctx := context.Background()
	listID := createTestPackingList(t)

	items, err := packingListRepo.GetPackingListItems(ctx, listID.String())
	require.NoError(t, err)
	assert.Empty(t, items)
}
