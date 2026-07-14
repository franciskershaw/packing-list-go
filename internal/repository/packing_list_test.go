package repository_test

import (
	"context"
	"testing"
	"time"

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

// archivePackingListDirect sets archived_at on listID via raw SQL, bypassing
// the repository method under test — used by tests that need an
// already-archived fixture without depending on ArchivePackingList itself.
func archivePackingListDirect(t *testing.T, listID uuid.UUID, at time.Time) {
	t.Helper()
	_, err := db.DB.Exec(`UPDATE packing_lists SET archived_at = $1 WHERE id = $2`, at, listID)
	require.NoError(t, err)
}

// createTestCategoryNamed inserts a user-owned category with an exact name
// (unlike createTestCategory's randomized name), for tests that assert on
// alphabetical ordering.
func createTestCategoryNamed(t *testing.T, userID, name string) uuid.UUID {
	t.Helper()
	cat, err := catRepo.CreateCategory(context.Background(), userID, name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, cat.ID) })
	return cat.ID
}

// createTestItemNamed inserts an item with an exact name under catID
// (unlike createTestItem's randomized name), for tests that assert on
// alphabetical ordering.
func createTestItemNamed(t *testing.T, catID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	item, err := itemRepo.CreateItem(context.Background(), repoUserID.String(), name, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })
	return item.ID
}

func TestGetPackingLists_ActiveOnly(t *testing.T) {
	ctx := context.Background()

	active, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-active-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, active.ID) })

	archived, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archived-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, archived.ID) })
	archivePackingListDirect(t, archived.ID, time.Now())

	lists, err := packingListRepo.GetPackingLists(ctx, repoUserID.String(), false)
	require.NoError(t, err)

	var foundActive, foundArchived bool
	for _, l := range lists {
		if l.ID == active.ID {
			foundActive = true
		}
		if l.ID == archived.ID {
			foundArchived = true
		}
	}
	assert.True(t, foundActive, "expected active list in results")
	assert.False(t, foundArchived, "did not expect archived list in active results")
}

func TestGetPackingLists_ArchivedOnly(t *testing.T) {
	ctx := context.Background()

	active, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-active2-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, active.ID) })

	archived, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archived2-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, archived.ID) })
	archivePackingListDirect(t, archived.ID, time.Now())

	lists, err := packingListRepo.GetPackingLists(ctx, repoUserID.String(), true)
	require.NoError(t, err)

	var foundActive, foundArchived bool
	for _, l := range lists {
		if l.ID == active.ID {
			foundActive = true
		}
		if l.ID == archived.ID {
			foundArchived = true
		}
	}
	assert.False(t, foundActive, "did not expect active list in archived results")
	assert.True(t, foundArchived, "expected archived list in archived results")
}

func TestGetPackingLists_ItemsAlwaysEmpty(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 1, nil)
	require.NoError(t, err)

	tmplIDStr := tmplID.String()
	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-itemsempty-"+uuid.NewString(), nil, &tmplIDStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })
	require.Len(t, created.Items, 1, "sanity check: list was actually seeded with an item")

	lists, err := packingListRepo.GetPackingLists(ctx, repoUserID.String(), false)
	require.NoError(t, err)

	var found *models.PackingList
	for i := range lists {
		if lists[i].ID == created.ID {
			found = &lists[i]
		}
	}
	require.NotNil(t, found, "expected seeded list in results")
	assert.Equal(t, []models.PackingListItem{}, found.Items, "list mode must not populate items, even when the list has items")
}

func TestGetPackingLists_ActiveOrderedByUpdatedAtDesc(t *testing.T) {
	ctx := context.Background()

	older, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-older-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, older.ID) })

	newer, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-newer-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, newer.ID) })

	_, err = db.DB.ExecContext(ctx, `UPDATE packing_lists SET updated_at = $1 WHERE id = $2`, time.Now().Add(-1*time.Hour), older.ID)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(ctx, `UPDATE packing_lists SET updated_at = $1 WHERE id = $2`, time.Now(), newer.ID)
	require.NoError(t, err)

	lists, err := packingListRepo.GetPackingLists(ctx, repoUserID.String(), false)
	require.NoError(t, err)

	newerIdx, olderIdx := -1, -1
	for i, l := range lists {
		if l.ID == newer.ID {
			newerIdx = i
		}
		if l.ID == older.ID {
			olderIdx = i
		}
	}
	require.NotEqual(t, -1, newerIdx, "expected newer list in results")
	require.NotEqual(t, -1, olderIdx, "expected older list in results")
	assert.Less(t, newerIdx, olderIdx, "expected more recently updated list first")
}

func TestGetPackingLists_ArchivedOrderedByArchivedAtDesc(t *testing.T) {
	ctx := context.Background()

	archivedEarlier, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archearly-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, archivedEarlier.ID) })
	archivePackingListDirect(t, archivedEarlier.ID, time.Now().Add(-1*time.Hour))

	archivedLater, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archlater-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, archivedLater.ID) })
	archivePackingListDirect(t, archivedLater.ID, time.Now())

	lists, err := packingListRepo.GetPackingLists(ctx, repoUserID.String(), true)
	require.NoError(t, err)

	laterIdx, earlierIdx := -1, -1
	for i, l := range lists {
		if l.ID == archivedLater.ID {
			laterIdx = i
		}
		if l.ID == archivedEarlier.ID {
			earlierIdx = i
		}
	}
	require.NotEqual(t, -1, laterIdx, "expected more-recently-archived list in results")
	require.NotEqual(t, -1, earlierIdx, "expected less-recently-archived list in results")
	assert.Less(t, laterIdx, earlierIdx, "expected more recently archived list first")
}

func TestGetPackingListByID_Found(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-list-getbyid-" + uuid.NewString()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), name, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	found, err := packingListRepo.GetPackingListByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, name, found.Name)
	assert.Equal(t, repoUserID, found.UserID)
	assert.Equal(t, []models.PackingListCategory{}, found.Categories)
}

func TestGetPackingListByID_NotFound(t *testing.T) {
	ctx := context.Background()

	found, err := packingListRepo.GetPackingListByID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGetPackingListByID_ArchivedStillReturned(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archdetail-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })
	archivePackingListDirect(t, created.ID, time.Now())

	found, err := packingListRepo.GetPackingListByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found, "archived list detail must still be fetchable")
	assert.Equal(t, created.ID, found.ID)
}

func TestGetPackingListByID_GroupedByCategoryAlphabetical(t *testing.T) {
	ctx := context.Background()

	catZebra := createTestCategoryNamed(t, repoUserID.String(), "Zebra Gear "+uuid.NewString())
	catAlpha := createTestCategoryNamed(t, repoUserID.String(), "Alpha Gear "+uuid.NewString())

	itemZebraB := createTestItemNamed(t, catZebra, "Zulu Item "+uuid.NewString())
	itemZebraA := createTestItemNamed(t, catZebra, "Alpha Item "+uuid.NewString())
	itemAlpha := createTestItemNamed(t, catAlpha, "Only Item "+uuid.NewString())

	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemZebraB.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, tmplID.String(), itemZebraA.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, tmplID.String(), itemAlpha.String(), 1, nil)
	require.NoError(t, err)

	tmplIDStr := tmplID.String()
	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-grouped-"+uuid.NewString(), nil, &tmplIDStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	found, err := packingListRepo.GetPackingListByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Len(t, found.Categories, 2, "expected exactly the two categories that have items on this list")

	assert.Equal(t, catAlpha, found.Categories[0].ID, "expected alphabetically-first category first")
	assert.Equal(t, catZebra, found.Categories[1].ID)

	require.Len(t, found.Categories[1].Items, 2)
	assert.Equal(t, itemZebraA, found.Categories[1].Items[0].ItemID, "expected alphabetically-first item first within its category")
	assert.Equal(t, itemZebraB, found.Categories[1].Items[1].ItemID)

	require.Len(t, found.Categories[0].Items, 1)
	assert.Equal(t, itemAlpha, found.Categories[0].Items[0].ItemID)
}

// setPackingListItemSortOrderDirect sets sort_order on a specific
// packing_list_items row via raw SQL, bypassing UpdatePackingListItem (its
// own PACK-012 tests cover it directly) — used purely to seed an ordering
// fixture here.
func setPackingListItemSortOrderDirect(t *testing.T, listID, itemID uuid.UUID, sortOrder int) {
	t.Helper()
	_, err := db.DB.Exec(`UPDATE packing_list_items SET sort_order = $1 WHERE list_id = $2 AND item_id = $3`, sortOrder, listID, itemID)
	require.NoError(t, err)
}

func TestGetPackingListByID_OrdersBySortOrderThenAlphabetical(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	itemBravo := createTestItemNamed(t, catID, "Bravo Item "+uuid.NewString())
	itemAlpha := createTestItemNamed(t, catID, "Alpha Item "+uuid.NewString())
	itemEcho := createTestItemNamed(t, catID, "Echo Item "+uuid.NewString())
	itemZulu := createTestItemNamed(t, catID, "Zulu Item "+uuid.NewString())

	tmplID := createTestTemplate(t)
	for _, id := range []uuid.UUID{itemBravo, itemAlpha, itemEcho, itemZulu} {
		_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), id.String(), 1, nil)
		require.NoError(t, err)
	}

	tmplIDStr := tmplID.String()
	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-sortorder-"+uuid.NewString(), nil, &tmplIDStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	// itemBravo and itemAlpha get explicit sort_order; itemEcho/itemZulu
	// stay NULL and must fall back to alphabetical among themselves.
	setPackingListItemSortOrderDirect(t, created.ID, itemBravo, 1)
	setPackingListItemSortOrderDirect(t, created.ID, itemAlpha, 2)

	found, err := packingListRepo.GetPackingListByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Len(t, found.Categories, 1)
	items := found.Categories[0].Items
	require.Len(t, items, 4)

	gotOrder := make([]uuid.UUID, len(items))
	for i, item := range items {
		gotOrder[i] = item.ItemID
	}
	assert.Equal(t, []uuid.UUID{itemBravo, itemAlpha, itemEcho, itemZulu}, gotOrder,
		"expected explicit sort_order first (ascending), then NULLs alphabetically by name")
}

func TestUpdatePackingList_NameOnly(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-upd-name-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	newName := "repo-test-list-upd-name-new-" + uuid.NewString()
	updated, err := packingListRepo.UpdatePackingList(ctx, created.ID.String(), &newName, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
}

func TestUpdatePackingList_EventDateOnly(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-upd-date-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	eventDate := "2026-09-15"
	updated, err := packingListRepo.UpdatePackingList(ctx, created.ID.String(), nil, &eventDate)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.EventDate)
	assert.Equal(t, eventDate, *updated.EventDate)
	assert.Equal(t, created.Name, updated.Name, "name should be unchanged when only eventDate is updated")
}

func TestUpdatePackingList_Both(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-upd-both-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	newName := "repo-test-list-upd-both-new-" + uuid.NewString()
	eventDate := "2026-10-01"
	updated, err := packingListRepo.UpdatePackingList(ctx, created.ID.String(), &newName, &eventDate)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	require.NotNil(t, updated.EventDate)
	assert.Equal(t, eventDate, *updated.EventDate)
}

func TestUpdatePackingList_OnArchivedList(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-upd-arch-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })
	archivePackingListDirect(t, created.ID, time.Now())

	newName := "repo-test-list-upd-arch-new-" + uuid.NewString()
	updated, err := packingListRepo.UpdatePackingList(ctx, created.ID.String(), &newName, nil)
	require.NoError(t, err, "updating an archived list must still succeed")
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
}

func TestUpdatePackingList_ReturnsGroupedCategories(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	tmplID := createTestTemplate(t)
	_, err := templateRepo.AddTemplateItem(ctx, tmplID.String(), itemID.String(), 2, nil)
	require.NoError(t, err)

	tmplIDStr := tmplID.String()
	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-upd-grouped-"+uuid.NewString(), nil, &tmplIDStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	newName := "repo-test-list-upd-grouped-new-" + uuid.NewString()
	updated, err := packingListRepo.UpdatePackingList(ctx, created.ID.String(), &newName, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, updated.Categories, 1)
	require.Len(t, updated.Categories[0].Items, 1)
	assert.Equal(t, itemID, updated.Categories[0].Items[0].ItemID)
}

func TestArchivePackingList_SetsArchivedAt(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archive-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	err = packingListRepo.ArchivePackingList(ctx, created.ID.String())
	require.NoError(t, err)

	var archivedAt *time.Time
	err = db.DB.QueryRowContext(ctx, `SELECT archived_at FROM packing_lists WHERE id = $1`, created.ID).Scan(&archivedAt)
	require.NoError(t, err)
	require.NotNil(t, archivedAt)
}

func TestArchivePackingList_Idempotent(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-archidem-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	err = packingListRepo.ArchivePackingList(ctx, created.ID.String())
	require.NoError(t, err)
	err = packingListRepo.ArchivePackingList(ctx, created.ID.String())
	require.NoError(t, err, "archiving an already-archived list must not error")

	var archivedAt *time.Time
	err = db.DB.QueryRowContext(ctx, `SELECT archived_at FROM packing_lists WHERE id = $1`, created.ID).Scan(&archivedAt)
	require.NoError(t, err)
	require.NotNil(t, archivedAt)
}

func TestUnarchivePackingList_ClearsArchivedAt(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-unarchive-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })
	archivePackingListDirect(t, created.ID, time.Now())

	err = packingListRepo.UnarchivePackingList(ctx, created.ID.String())
	require.NoError(t, err)

	var archivedAt *time.Time
	err = db.DB.QueryRowContext(ctx, `SELECT archived_at FROM packing_lists WHERE id = $1`, created.ID).Scan(&archivedAt)
	require.NoError(t, err)
	require.Nil(t, archivedAt)
}

func TestUnarchivePackingList_IdempotentOnAlreadyActive(t *testing.T) {
	ctx := context.Background()

	created, err := packingListRepo.CreatePackingList(ctx, repoUserID.String(), "repo-test-list-unarchidem-"+uuid.NewString(), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, created.ID) })

	err = packingListRepo.UnarchivePackingList(ctx, created.ID.String())
	require.NoError(t, err, "unarchiving an already-active list must not error")

	var archivedAt *time.Time
	err = db.DB.QueryRowContext(ctx, `SELECT archived_at FROM packing_lists WHERE id = $1`, created.ID).Scan(&archivedAt)
	require.NoError(t, err)
	require.Nil(t, archivedAt)
}
