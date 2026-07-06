package repository_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	catRepo    *repository.CategoryRepository
	repoUserID uuid.UUID
)

func TestMain(m *testing.M) {
	if os.Getenv("DATABASE_URL") == "" {
		fmt.Println("skipping repository tests: DATABASE_URL not set")
		os.Exit(0)
	}

	if err := db.InitDB(); err != nil {
		fmt.Printf("failed to init db: %v\n", err)
		os.Exit(1)
	}

	catRepo = repository.NewCategoryRepository(db.DB)
	itemRepo = repository.NewItemRepository(db.DB)
	repoUserID = uuid.New()

	_, err := db.DB.Exec(
		`INSERT INTO users (id, google_id, email) VALUES ($1, $2, $3)`,
		repoUserID,
		"repo-test-google-"+repoUserID.String(),
		"repo-test-"+repoUserID.String()+"@example.com",
	)
	if err != nil {
		fmt.Printf("failed to create test user: %v\n", err)
		db.CloseDB()
		os.Exit(1)
	}

	code := m.Run()

	// ON DELETE CASCADE on categories.user_id handles category cleanup
	db.DB.Exec(`DELETE FROM users WHERE id = $1`, repoUserID)
	db.CloseDB()
	os.Exit(code)
}

func TestGetCategories_ReturnsMixed(t *testing.T) {
	ctx := context.Background()

	// Seed a system category and a user category
	var sysID uuid.UUID
	err := db.DB.QueryRowContext(ctx,
		`INSERT INTO categories (name) VALUES ($1) RETURNING id`, "repo-test-sys-"+uuid.NewString(),
	).Scan(&sysID)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, sysID) })

	userCat, err := catRepo.CreateCategory(ctx, repoUserID.String(), "repo-test-user-"+uuid.NewString())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, userCat.ID) })

	cats, err := catRepo.GetCategories(ctx, repoUserID.String())
	require.NoError(t, err)

	var foundSys, foundUser bool
	for _, c := range cats {
		if c.ID == sysID {
			foundSys = true
			assert.True(t, c.IsSystem)
		}
		if c.ID == userCat.ID {
			foundUser = true
			assert.False(t, c.IsSystem)
		}
	}
	assert.True(t, foundSys, "expected system category in results")
	assert.True(t, foundUser, "expected user category in results")
}

func TestGetCategories_EmptySlice(t *testing.T) {
	ctx := context.Background()
	newUserID := uuid.New()
	_, err := db.DB.Exec(
		`INSERT INTO users (id, google_id, email) VALUES ($1, $2, $3)`,
		newUserID, "tmp-google-"+newUserID.String(), "tmp-"+newUserID.String()+"@example.com",
	)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, newUserID) })

	cats, err := catRepo.GetCategories(ctx, newUserID.String())
	require.NoError(t, err)

	// Result may include system categories seeded in other tests; the slice should be non-nil
	assert.NotNil(t, cats)
}

func TestCreateCategory(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-create-" + uuid.NewString()

	cat, err := catRepo.CreateCategory(ctx, repoUserID.String(), name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, cat.ID) })

	assert.Equal(t, name, cat.Name)
	assert.False(t, cat.IsSystem)
	assert.NotEqual(t, uuid.Nil, cat.ID)
	require.NotNil(t, cat.UserID)
	assert.Equal(t, repoUserID, *cat.UserID)
}

func TestGetCategoryByID(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-getbyid-" + uuid.NewString()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID) })

	cat, err := catRepo.GetCategoryByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, cat)
	assert.Equal(t, name, cat.Name)
	assert.False(t, cat.IsSystem)
}

func TestGetCategoryByID_NotFound(t *testing.T) {
	ctx := context.Background()

	cat, err := catRepo.GetCategoryByID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, cat)
}

func TestUpdateCategory(t *testing.T) {
	ctx := context.Background()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), "repo-test-update-orig-"+uuid.NewString())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID) })

	newName := "repo-test-update-new-" + uuid.NewString()
	updated, err := catRepo.UpdateCategory(ctx, created.ID.String(), newName)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, created.ID, updated.ID)
}

func TestDeleteCategory(t *testing.T) {
	ctx := context.Background()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), "repo-test-delete-"+uuid.NewString())
	require.NoError(t, err)

	err = catRepo.DeleteCategory(ctx, created.ID.String())
	require.NoError(t, err)

	cat, err := catRepo.GetCategoryByID(ctx, created.ID.String())
	require.NoError(t, err)
	assert.Nil(t, cat)
}

func TestCategoryNameExistsForUser_True(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-exists-" + uuid.NewString()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID) })

	exists, err := catRepo.CategoryNameExistsForUser(ctx, repoUserID.String(), name, nil)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCategoryNameExistsForUser_ExcludeID(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-exclude-" + uuid.NewString()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID) })

	id := created.ID.String()
	// When the current category is excluded the name should not conflict with itself
	exists, err := catRepo.CategoryNameExistsForUser(ctx, repoUserID.String(), name, &id)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCategoryNameExistsForUser_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	name := "repo-test-case-" + uuid.NewString()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), name)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID) })

	exists, err := catRepo.CategoryNameExistsForUser(ctx, repoUserID.String(), strings.ToUpper(name), nil)
	require.NoError(t, err)
	assert.True(t, exists, "name check should be case-insensitive")
}

func TestCategoryHasItems_False(t *testing.T) {
	ctx := context.Background()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), "repo-test-hasitems-"+uuid.NewString())
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID) })

	has, err := catRepo.CategoryHasItems(ctx, created.ID.String())
	require.NoError(t, err)
	assert.False(t, has)
}

func TestCategoryHasItems_True(t *testing.T) {
	ctx := context.Background()

	created, err := catRepo.CreateCategory(ctx, repoUserID.String(), "repo-test-hasitems-true-"+uuid.NewString())
	require.NoError(t, err)
	t.Cleanup(func() {
		db.DB.Exec(`DELETE FROM items WHERE category_id = $1`, created.ID)
		db.DB.Exec(`DELETE FROM categories WHERE id = $1`, created.ID)
	})

	_, err = db.DB.ExecContext(ctx,
		`INSERT INTO items (category_id, user_id, name) VALUES ($1, $2, $3)`,
		created.ID, repoUserID, "test-item",
	)
	require.NoError(t, err)

	has, err := catRepo.CategoryHasItems(ctx, created.ID.String())
	require.NoError(t, err)
	assert.True(t, has)
}
