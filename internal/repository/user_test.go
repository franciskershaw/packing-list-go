package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrCreateUser_CreatesNewUser(t *testing.T) {
	ctx := context.Background()
	googleID := "repo-test-user-google-" + uuid.NewString()
	email := "repo-test-user-" + uuid.NewString() + "@example.com"

	user, err := userRepo.GetOrCreateUser(ctx, email, googleID, "Test User", "http://example.com/avatar.png")
	require.NoError(t, err)
	require.NotNil(t, user)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, user.ID) })

	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, googleID, user.GoogleID)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, "Test User", user.DisplayName)
	assert.Equal(t, "http://example.com/avatar.png", user.AvatarURL)
}

func TestGetOrCreateUser_ReturnsExistingAndUpdatesLastLogin(t *testing.T) {
	ctx := context.Background()
	googleID := "repo-test-user-google-" + uuid.NewString()
	email := "repo-test-user-" + uuid.NewString() + "@example.com"

	created, err := userRepo.GetOrCreateUser(ctx, email, googleID, "Original Name", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, created.ID) })

	time.Sleep(10 * time.Millisecond)

	fetched, err := userRepo.GetOrCreateUser(ctx, email, googleID, "Ignored Name", "ignored-avatar")
	require.NoError(t, err)
	require.NotNil(t, fetched)

	assert.Equal(t, created.ID, fetched.ID, "expected the same user record, not a new one")
	assert.Equal(t, "Original Name", fetched.DisplayName, "existing display name should not be overwritten")
	assert.True(t, fetched.LastLoginAt.After(created.LastLoginAt), "expected last_login_at to advance on repeat login")
}

func TestGetUserByID_Found(t *testing.T) {
	ctx := context.Background()
	googleID := "repo-test-user-google-" + uuid.NewString()
	email := "repo-test-user-" + uuid.NewString() + "@example.com"

	created, err := userRepo.GetOrCreateUser(ctx, email, googleID, "Test User", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, created.ID) })

	fetched, err := userRepo.GetUserByID(ctx, created.ID.String())
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, email, fetched.Email)
}

func TestGetUserByID_NotFound(t *testing.T) {
	ctx := context.Background()

	user, err := userRepo.GetUserByID(ctx, uuid.NewString())
	assert.NoError(t, err)
	assert.Nil(t, user)
}
