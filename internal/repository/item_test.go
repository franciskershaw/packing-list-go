package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCategory inserts a category (system if userID == "") and returns its ID,
// registering cleanup.
func createTestCategory(t *testing.T, userID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	name := "repo-test-cat-" + uuid.NewString()
	if userID == "" {
		var id uuid.UUID
		err := db.DB.QueryRowContext(ctx, `INSERT INTO categories (name) VALUES ($1) RETURNING id`, name).Scan(&id)
		require.NoError(t, err)
		t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, id) })
		return id
	}
	cat, err := catRepo.CreateCategory(ctx, userID, name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, cat.ID) })
	return cat.ID
}

func TestCreateItem(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	name := "repo-test-item-" + uuid.NewString()

	item, err := itemRepo.CreateItem(ctx, repoUserID.String(), name, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })

	assert.Equal(t, name, item.Name)
	assert.Equal(t, catID, item.CategoryID)
	assert.False(t, item.IsSystem)
	require.NotNil(t, item.UserID)
	assert.Equal(t, repoUserID, *item.UserID)
}

func TestGetItemByID(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	name := "repo-test-getbyid-" + uuid.NewString()

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), name, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, created.ID) })

	item, err := itemRepo.GetItemByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, name, item.Name)
	assert.False(t, item.IsSystem)
}

func TestGetItemByID_NotFound(t *testing.T) {
	ctx := context.Background()

	item, err := itemRepo.GetItemByID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, item)
}

func TestUpdateItem_Name(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-update-orig-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, created.ID) })

	newName := "repo-test-update-new-" + uuid.NewString()
	updated, err := itemRepo.UpdateItem(ctx, created.ID.String(), &newName, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, catID, updated.CategoryID)
}

func TestUpdateItem_Category(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	newCatID := createTestCategory(t, repoUserID.String())

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-update-cat-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, created.ID) })

	newCatIDStr := newCatID.String()
	updated, err := itemRepo.UpdateItem(ctx, created.ID.String(), nil, &newCatIDStr)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.Name, updated.Name)
	assert.Equal(t, newCatID, updated.CategoryID)
}

func TestDeleteItem(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-delete-"+uuid.NewString(), catID.String())
	require.NoError(t, err)

	err = itemRepo.DeleteItem(ctx, created.ID.String())
	require.NoError(t, err)

	item, err := itemRepo.GetItemByID(ctx, created.ID.String())
	require.NoError(t, err)
	assert.Nil(t, item)
}

func TestItemNameExistsInCategory_True(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	name := "repo-test-exists-" + uuid.NewString()

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), name, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, created.ID) })

	exists, err := itemRepo.ItemNameExistsInCategory(ctx, catID.String(), name, nil)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestItemNameExistsInCategory_ExcludeID(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	name := "repo-test-exclude-" + uuid.NewString()

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), name, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, created.ID) })

	id := created.ID.String()
	exists, err := itemRepo.ItemNameExistsInCategory(ctx, catID.String(), name, &id)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestItemNameExistsInCategory_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	name := "repo-test-case-" + uuid.NewString()

	created, err := itemRepo.CreateItem(ctx, repoUserID.String(), name, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, created.ID) })

	exists, err := itemRepo.ItemNameExistsInCategory(ctx, catID.String(), strings.ToUpper(name), nil)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestItemNameExistsInCategory_AcrossSystemAndUserItems(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, "")
	name := "repo-test-sys-item-" + uuid.NewString()

	var sysItemID uuid.UUID
	err := db.DB.QueryRowContext(ctx,
		`INSERT INTO items (category_id, name) VALUES ($1, $2) RETURNING id`, catID, name,
	).Scan(&sysItemID)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, sysItemID) })

	// A user-owned item with the same name in the same (system) category should be a duplicate.
	exists, err := itemRepo.ItemNameExistsInCategory(ctx, catID.String(), name, nil)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCategoryIsAccessible_SystemCategory(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, "")

	accessible, err := itemRepo.CategoryIsAccessible(ctx, catID.String(), repoUserID.String())
	require.NoError(t, err)
	assert.True(t, accessible)
}

func TestCategoryIsAccessible_OwnCategory(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	accessible, err := itemRepo.CategoryIsAccessible(ctx, catID.String(), repoUserID.String())
	require.NoError(t, err)
	assert.True(t, accessible)
}

func TestCategoryIsAccessible_OtherUsersCategory(t *testing.T) {
	ctx := context.Background()
	otherUser := uuid.New()
	_, err := db.DB.Exec(
		`INSERT INTO users (id, google_id, email) VALUES ($1, $2, $3)`,
		otherUser, "repo-test-other-google-"+otherUser.String(), "repo-test-other-"+otherUser.String()+"@example.com",
	)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, otherUser) })

	catID := createTestCategory(t, otherUser.String())

	accessible, err := itemRepo.CategoryIsAccessible(ctx, catID.String(), repoUserID.String())
	require.NoError(t, err)
	assert.False(t, accessible)
}

func TestGetItems_ReturnsAccessibleItems(t *testing.T) {
	ctx := context.Background()
	sysCatID := createTestCategory(t, "")

	var sysItemID uuid.UUID
	err := db.DB.QueryRowContext(ctx,
		`INSERT INTO items (category_id, name) VALUES ($1, $2) RETURNING id`, sysCatID, "repo-test-sys-"+uuid.NewString(),
	).Scan(&sysItemID)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, sysItemID) })

	userItem, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-user-"+uuid.NewString(), sysCatID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, userItem.ID) })

	items, err := itemRepo.GetItems(ctx, repoUserID.String(), nil, nil)
	require.NoError(t, err)

	var foundSys, foundUser bool
	for _, it := range items {
		if it.ID == sysItemID {
			foundSys = true
			assert.True(t, it.IsSystem)
		}
		if it.ID == userItem.ID {
			foundUser = true
			assert.False(t, it.IsSystem)
		}
	}
	assert.True(t, foundSys, "expected system item in results")
	assert.True(t, foundUser, "expected user item in results")
}

func TestGetItems_FilterByCategory(t *testing.T) {
	ctx := context.Background()
	catA := createTestCategory(t, repoUserID.String())
	catB := createTestCategory(t, repoUserID.String())

	itemA, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-cata-"+uuid.NewString(), catA.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, itemA.ID) })

	itemB, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-catb-"+uuid.NewString(), catB.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, itemB.ID) })

	catAStr := catA.String()
	items, err := itemRepo.GetItems(ctx, repoUserID.String(), &catAStr, nil)
	require.NoError(t, err)

	var foundA, foundB bool
	for _, it := range items {
		if it.ID == itemA.ID {
			foundA = true
		}
		if it.ID == itemB.ID {
			foundB = true
		}
	}
	assert.True(t, foundA)
	assert.False(t, foundB)
}

func TestGetItems_FilterBySearch(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	uniqueTag := strings.ReplaceAll(uuid.NewString(), "-", "")

	match, err := itemRepo.CreateItem(ctx, repoUserID.String(), "Shampoo-"+uniqueTag, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, match.ID) })

	noMatch, err := itemRepo.CreateItem(ctx, repoUserID.String(), "Toothbrush-"+uniqueTag, catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, noMatch.ID) })

	search := "shampoo-" + uniqueTag
	items, err := itemRepo.GetItems(ctx, repoUserID.String(), nil, &search)
	require.NoError(t, err)

	var foundMatch, foundNoMatch bool
	for _, it := range items {
		if it.ID == match.ID {
			foundMatch = true
		}
		if it.ID == noMatch.ID {
			foundNoMatch = true
		}
	}
	assert.True(t, foundMatch)
	assert.False(t, foundNoMatch)
}

func TestGetItemsByIDs_ReturnsMatches(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	itemA, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-byids-a-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, itemA.ID) })

	itemB, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-byids-b-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, itemB.ID) })

	// A third item exists but isn't requested — must not appear in results.
	itemC, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-byids-c-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, itemC.ID) })

	items, err := itemRepo.GetItemsByIDs(ctx, []string{itemA.ID.String(), itemB.ID.String()})
	require.NoError(t, err)
	require.Len(t, items, 2)

	var foundA, foundB, foundC bool
	for _, it := range items {
		if it.ID == itemA.ID {
			foundA = true
		}
		if it.ID == itemB.ID {
			foundB = true
		}
		if it.ID == itemC.ID {
			foundC = true
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)
	assert.False(t, foundC, "item not in the requested ID list must not be returned")
}

func TestGetItemsByIDs_EmptyIDsReturnsEmpty(t *testing.T) {
	ctx := context.Background()

	items, err := itemRepo.GetItemsByIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestGetItemsByIDs_UnknownIDOmittedNotErrored(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	itemA, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-byids-known-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, itemA.ID) })

	items, err := itemRepo.GetItemsByIDs(ctx, []string{itemA.ID.String(), uuid.NewString()})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, itemA.ID, items[0].ID)
}

func TestItemIsInUse_False(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	item, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-notinuse-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })

	inUse, err := itemRepo.ItemIsInUse(ctx, item.ID.String())
	require.NoError(t, err)
	assert.False(t, inUse)
}

func TestItemIsInUse_TemplateReference(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	item, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-tmplref-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })

	var templateID uuid.UUID
	err = db.DB.QueryRowContext(ctx,
		`INSERT INTO templates (user_id, name) VALUES ($1, $2) RETURNING id`,
		repoUserID, "repo-test-template-"+uuid.NewString(),
	).Scan(&templateID)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, templateID) })

	_, err = db.DB.ExecContext(ctx,
		`INSERT INTO template_items (template_id, item_id) VALUES ($1, $2)`, templateID, item.ID,
	)
	require.NoError(t, err)

	inUse, err := itemRepo.ItemIsInUse(ctx, item.ID.String())
	require.NoError(t, err)
	assert.True(t, inUse)
}

func TestItemIsInUse_ActiveListReference(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	item, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-activeref-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })

	var listID uuid.UUID
	err = db.DB.QueryRowContext(ctx,
		`INSERT INTO packing_lists (user_id, name) VALUES ($1, $2) RETURNING id`,
		repoUserID, "repo-test-list-"+uuid.NewString(),
	).Scan(&listID)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, listID) })

	_, err = db.DB.ExecContext(ctx,
		`INSERT INTO packing_list_items (list_id, item_id, category_id) VALUES ($1, $2, $3)`,
		listID, item.ID, catID,
	)
	require.NoError(t, err)

	inUse, err := itemRepo.ItemIsInUse(ctx, item.ID.String())
	require.NoError(t, err)
	assert.True(t, inUse)
}

func TestItemIsInUse_ArchivedListReference_NotBlocked(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())

	item, err := itemRepo.CreateItem(ctx, repoUserID.String(), "repo-test-archivedref-"+uuid.NewString(), catID.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM items WHERE id = $1`, item.ID) })

	var listID uuid.UUID
	err = db.DB.QueryRowContext(ctx,
		`INSERT INTO packing_lists (user_id, name, archived_at) VALUES ($1, $2, CURRENT_TIMESTAMP) RETURNING id`,
		repoUserID, "repo-test-archived-list-"+uuid.NewString(),
	).Scan(&listID)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM packing_lists WHERE id = $1`, listID) })

	_, err = db.DB.ExecContext(ctx,
		`INSERT INTO packing_list_items (list_id, item_id, category_id) VALUES ($1, $2, $3)`,
		listID, item.ID, catID,
	)
	require.NoError(t, err)

	inUse, err := itemRepo.ItemIsInUse(ctx, item.ID.String())
	require.NoError(t, err)
	assert.False(t, inUse)
}
