package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestUser inserts a standalone user and registers cleanup, for tests
// that need a second, distinct owner.
func createTestUser(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.DB.Exec(
		`INSERT INTO users (id, google_id, email) VALUES ($1, $2, $3)`,
		id, "repo-test-tmpl-google-"+id.String(), "repo-test-tmpl-"+id.String()+"@example.com",
	)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, id) })
	return id
}

func TestCreateTemplate_NameOnly(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-" + uuid.NewString()

	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	assert.Equal(t, name, created.Name)
	assert.Nil(t, created.Description)
	assert.Equal(t, repoUserID, created.UserID)
	assert.Equal(t, []models.TemplateItem{}, created.Items)
}

func TestCreateTemplate_WithDescription(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-desc-" + uuid.NewString()
	desc := "A description"

	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), name, &desc)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	require.NotNil(t, created.Description)
	assert.Equal(t, desc, *created.Description)
}

func TestGetTemplateByID_Found(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-getbyid-" + uuid.NewString()

	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	found, err := templateRepo.GetTemplateByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, name, found.Name)
	assert.Equal(t, repoUserID, found.UserID)
	assert.Equal(t, []models.TemplateItem{}, found.Items)
	assert.Equal(t, 0, found.ItemCount)
}

func TestGetTemplateByID_ItemCount(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemA := createTestItem(t, catID)
	itemB := createTestItem(t, catID)

	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-getbyid-count-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })
	_, err = templateRepo.AddTemplateItem(ctx, created.ID.String(), itemA.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, created.ID.String(), itemB.String(), 1, nil)
	require.NoError(t, err)

	found, err := templateRepo.GetTemplateByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, 2, found.ItemCount)
}

func TestGetTemplateByID_NotFound(t *testing.T) {
	ctx := context.Background()

	found, err := templateRepo.GetTemplateByID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGetTemplates_ScopedToUser(t *testing.T) {
	ctx := context.Background()
	own, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-own-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, own.ID) })

	otherUser := createTestUser(t)
	other, err := templateRepo.CreateTemplate(ctx, otherUser.String(), "repo-test-tmpl-other-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, other.ID) })

	templates, err := templateRepo.GetTemplates(ctx, repoUserID.String())
	require.NoError(t, err)

	var foundOwn, foundOther bool
	for _, tmpl := range templates {
		if tmpl.ID == own.ID {
			foundOwn = true
		}
		if tmpl.ID == other.ID {
			foundOther = true
		}
	}
	assert.True(t, foundOwn, "expected own template in results")
	assert.False(t, foundOther, "did not expect other user's template in results")
}

func TestGetTemplates_ItemCount(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemA := createTestItem(t, catID)
	itemB := createTestItem(t, catID)

	withItems, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-withitems-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, withItems.ID) })
	_, err = templateRepo.AddTemplateItem(ctx, withItems.ID.String(), itemA.String(), 1, nil)
	require.NoError(t, err)
	_, err = templateRepo.AddTemplateItem(ctx, withItems.ID.String(), itemB.String(), 1, nil)
	require.NoError(t, err)

	empty, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-noitems-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, empty.ID) })

	templates, err := templateRepo.GetTemplates(ctx, repoUserID.String())
	require.NoError(t, err)

	var foundWithItems, foundEmpty bool
	for _, tmpl := range templates {
		if tmpl.ID == withItems.ID {
			foundWithItems = true
			assert.Equal(t, 2, tmpl.ItemCount)
		}
		if tmpl.ID == empty.ID {
			foundEmpty = true
			assert.Equal(t, 0, tmpl.ItemCount)
		}
	}
	assert.True(t, foundWithItems, "expected template with items in results")
	assert.True(t, foundEmpty, "expected empty template in results")
}

func TestGetTemplates_EmptyForNewUser(t *testing.T) {
	ctx := context.Background()
	newUser := createTestUser(t)

	templates, err := templateRepo.GetTemplates(ctx, newUser.String())
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestGetTemplates_OrderedByUpdatedAtDesc(t *testing.T) {
	ctx := context.Background()

	older, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-older-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, older.ID) })

	newer, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-newer-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, newer.ID) })

	// Force explicit, unambiguous timestamps rather than relying on clock
	// resolution between the two creates above.
	_, err = db.DB.ExecContext(ctx, `UPDATE templates SET updated_at = $1 WHERE id = $2`, time.Now().Add(-1*time.Hour), older.ID)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(ctx, `UPDATE templates SET updated_at = $1 WHERE id = $2`, time.Now(), newer.ID)
	require.NoError(t, err)

	templates, err := templateRepo.GetTemplates(ctx, repoUserID.String())
	require.NoError(t, err)

	newerIdx, olderIdx := -1, -1
	for i, tmpl := range templates {
		if tmpl.ID == newer.ID {
			newerIdx = i
		}
		if tmpl.ID == older.ID {
			olderIdx = i
		}
	}
	require.NotEqual(t, -1, newerIdx, "expected newer template in results")
	require.NotEqual(t, -1, olderIdx, "expected older template in results")
	assert.Less(t, newerIdx, olderIdx, "expected more recently updated template first")
}

func TestUpdateTemplate_NameOnly(t *testing.T) {
	ctx := context.Background()
	desc := "Keep me"
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-upd-name-"+uuid.NewString(), &desc)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	newName := "repo-test-tmpl-upd-name-new-" + uuid.NewString()
	updated, err := templateRepo.UpdateTemplate(ctx, created.ID.String(), &newName, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	require.NotNil(t, updated.Description)
	assert.Equal(t, desc, *updated.Description)
}

func TestUpdateTemplate_DescriptionOnly(t *testing.T) {
	ctx := context.Background()
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-upd-desc-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	newDesc := "repo-test-tmpl-upd-desc-new-" + uuid.NewString()
	updated, err := templateRepo.UpdateTemplate(ctx, created.ID.String(), nil, &newDesc)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.Name, updated.Name)
	require.NotNil(t, updated.Description)
	assert.Equal(t, newDesc, *updated.Description)
}

func TestUpdateTemplate_DescriptionEmptyStringNotNull(t *testing.T) {
	ctx := context.Background()
	desc := "Will be cleared"
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-upd-clear-"+uuid.NewString(), &desc)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	empty := ""
	updated, err := templateRepo.UpdateTemplate(ctx, created.ID.String(), nil, &empty)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Description, "expected empty string, not NULL")
	assert.Equal(t, "", *updated.Description)
}

func TestUpdateTemplate_Both(t *testing.T) {
	ctx := context.Background()
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-upd-both-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	newName := "repo-test-tmpl-upd-both-new-" + uuid.NewString()
	newDesc := "both updated"
	updated, err := templateRepo.UpdateTemplate(ctx, created.ID.String(), &newName, &newDesc)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	require.NotNil(t, updated.Description)
	assert.Equal(t, newDesc, *updated.Description)
}

func TestUpdateTemplate_PreservesItems(t *testing.T) {
	ctx := context.Background()
	catID := createTestCategory(t, repoUserID.String())
	itemID := createTestItem(t, catID)
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-preserve-items-"+uuid.NewString(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	_, err = templateRepo.AddTemplateItem(ctx, created.ID.String(), itemID.String(), 2, nil)
	require.NoError(t, err)

	newName := "repo-test-tmpl-preserve-items-new-" + uuid.NewString()
	updated, err := templateRepo.UpdateTemplate(ctx, created.ID.String(), &newName, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	require.Len(t, updated.Items, 1, "expected the attached item to survive the update, not be dropped from the response")
	assert.Equal(t, itemID, updated.Items[0].ItemID)
	assert.Equal(t, 1, updated.ItemCount)
}

func TestDeleteTemplate(t *testing.T) {
	ctx := context.Background()
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), "repo-test-tmpl-del-"+uuid.NewString(), nil)
	require.NoError(t, err)

	err = templateRepo.DeleteTemplate(ctx, created.ID.String())
	require.NoError(t, err)

	found, err := templateRepo.GetTemplateByID(ctx, created.ID.String())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTemplateNameExistsForUser_True(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-exists-" + uuid.NewString()
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	exists, err := templateRepo.TemplateNameExistsForUser(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTemplateNameExistsForUser_ExcludeID(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-exclude-" + uuid.NewString()
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	id := created.ID.String()
	exists, err := templateRepo.TemplateNameExistsForUser(ctx, repoUserID.String(), name, &id)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTemplateNameExistsForUser_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-case-" + uuid.NewString()
	created, err := templateRepo.CreateTemplate(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	exists, err := templateRepo.TemplateNameExistsForUser(ctx, repoUserID.String(), strings.ToUpper(name), nil)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTemplateNameExistsForUser_ScopedToUser(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-tmpl-scoped-" + uuid.NewString()
	otherUser := createTestUser(t)
	created, err := templateRepo.CreateTemplate(ctx, otherUser.String(), name, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM templates WHERE id = $1`, created.ID) })

	exists, err := templateRepo.TemplateNameExistsForUser(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	assert.False(t, exists, "another user's template name should not count as a conflict")
}
