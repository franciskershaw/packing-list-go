package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fetchRefreshTokenRow reads a row directly, for assertions not exposed
// through the repository's own methods.
func fetchRefreshTokenRow(t *testing.T, id uuid.UUID) *models.RefreshTokenFamily {
	t.Helper()

	var (
		row                    models.RefreshTokenFamily
		previousTokenHash      sql.NullString
		previousTokenRotatedAt sql.NullTime
		revokedAt              sql.NullTime
	)
	err := db.DB.QueryRow(
		`SELECT id, user_id, token_hash, previous_token_hash, previous_token_rotated_at, expires_at, revoked_at, created_at
		 FROM refresh_tokens WHERE id = $1`, id,
	).Scan(&row.ID, &row.UserID, &row.TokenHash, &previousTokenHash, &previousTokenRotatedAt, &row.ExpiresAt, &revokedAt, &row.CreatedAt)
	require.NoError(t, err)

	if previousTokenHash.Valid {
		row.PreviousTokenHash = &previousTokenHash.String
	}
	if previousTokenRotatedAt.Valid {
		row.PreviousTokenRotatedAt = &previousTokenRotatedAt.Time
	}
	if revokedAt.Valid {
		row.RevokedAt = &revokedAt.Time
	}
	return &row
}

func TestCreateFamily_PersistsRow(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewString()
	hash := "repo-test-hash-" + uuid.NewString()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	family, err := refreshTokenRepo.CreateFamily(ctx, id, repoUserID.String(), hash, expiresAt)
	require.NoError(t, err)
	require.NotNil(t, family)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, family.ID) })

	assert.Equal(t, id, family.ID.String())
	assert.Equal(t, repoUserID, family.UserID)
	assert.Equal(t, hash, family.TokenHash)
	assert.Nil(t, family.PreviousTokenHash)
	assert.Nil(t, family.RevokedAt)
	assert.WithinDuration(t, expiresAt, family.ExpiresAt, time.Second)
}

func TestFindFamilyByID_ReturnsFamily(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewString()

	family, err := refreshTokenRepo.CreateFamily(ctx, id, repoUserID.String(), "repo-test-hash-"+uuid.NewString(), time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, family.ID) })

	found, err := refreshTokenRepo.FindFamilyByID(ctx, id, repoUserID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, family.ID, found.ID)
}

func TestFindFamilyByID_ReturnsNilForUnknownID(t *testing.T) {
	ctx := context.Background()

	family, err := refreshTokenRepo.FindFamilyByID(ctx, uuid.NewString(), repoUserID.String())
	require.NoError(t, err)
	assert.Nil(t, family)
}

func TestFindFamilyByID_ReturnsNilForWrongUser(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewString()

	family, err := refreshTokenRepo.CreateFamily(ctx, id, repoUserID.String(), "repo-test-hash-"+uuid.NewString(), time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, family.ID) })

	found, err := refreshTokenRepo.FindFamilyByID(ctx, id, uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRotateFamily_ShiftsCurrentIntoPrevious(t *testing.T) {
	ctx := context.Background()
	hash1 := "repo-test-hash1-" + uuid.NewString()
	hash2 := "repo-test-hash2-" + uuid.NewString()
	newExpiry := time.Now().Add(6 * 24 * time.Hour)

	family, err := refreshTokenRepo.CreateFamily(ctx, uuid.NewString(), repoUserID.String(), hash1, time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, family.ID) })

	beforeRotate := time.Now()
	err = refreshTokenRepo.RotateFamily(ctx, family.ID.String(), hash2, newExpiry)
	require.NoError(t, err)

	row := fetchRefreshTokenRow(t, family.ID)
	assert.Equal(t, hash2, row.TokenHash)
	require.NotNil(t, row.PreviousTokenHash)
	assert.Equal(t, hash1, *row.PreviousTokenHash)
	require.NotNil(t, row.PreviousTokenRotatedAt)
	assert.WithinDuration(t, beforeRotate, *row.PreviousTokenRotatedAt, 5*time.Second)
	assert.WithinDuration(t, newExpiry, row.ExpiresAt, time.Second)
}

func TestRevokeFamily_SetsRevokedAt(t *testing.T) {
	ctx := context.Background()
	hash := "repo-test-hash-" + uuid.NewString()

	family, err := refreshTokenRepo.CreateFamily(ctx, uuid.NewString(), repoUserID.String(), hash, time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, family.ID) })

	err = refreshTokenRepo.RevokeFamily(ctx, family.ID.String())
	require.NoError(t, err)

	row := fetchRefreshTokenRow(t, family.ID)
	require.NotNil(t, row.RevokedAt)
	assert.WithinDuration(t, time.Now(), *row.RevokedAt, 5*time.Second)
}

func TestDeleteStaleFamiliesForUser_RemovesOnlyRevokedOrExpiredForThatUser(t *testing.T) {
	ctx := context.Background()

	// Active family for repoUserID — must survive.
	active, err := refreshTokenRepo.CreateFamily(ctx, uuid.NewString(), repoUserID.String(), "repo-test-active-"+uuid.NewString(), time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, active.ID) })

	// Revoked family for repoUserID — must be removed.
	revoked, err := refreshTokenRepo.CreateFamily(ctx, uuid.NewString(), repoUserID.String(), "repo-test-revoked-"+uuid.NewString(), time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	require.NoError(t, refreshTokenRepo.RevokeFamily(ctx, revoked.ID.String()))
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, revoked.ID) })

	// Expired family for repoUserID — must be removed.
	expired, err := refreshTokenRepo.CreateFamily(ctx, uuid.NewString(), repoUserID.String(), "repo-test-expired-"+uuid.NewString(), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, expired.ID) })

	// Revoked family for a *different* user — must survive (scoping check).
	otherUserID := uuid.New()
	_, err = db.DB.Exec(
		`INSERT INTO users (id, google_id, email) VALUES ($1, $2, $3)`,
		otherUserID, "repo-test-google-"+otherUserID.String(), "repo-test-"+otherUserID.String()+"@example.com",
	)
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM users WHERE id = $1`, otherUserID) })

	otherRevoked, err := refreshTokenRepo.CreateFamily(ctx, uuid.NewString(), otherUserID.String(), "repo-test-other-revoked-"+uuid.NewString(), time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	require.NoError(t, refreshTokenRepo.RevokeFamily(ctx, otherRevoked.ID.String()))
	t.Cleanup(func() { db.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, otherRevoked.ID) })

	err = refreshTokenRepo.DeleteStaleFamiliesForUser(ctx, repoUserID.String())
	require.NoError(t, err)

	var count int
	require.NoError(t, db.DB.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE id = $1`, active.ID).Scan(&count))
	assert.Equal(t, 1, count, "active family should survive")

	require.NoError(t, db.DB.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE id = $1`, revoked.ID).Scan(&count))
	assert.Equal(t, 0, count, "revoked family should be removed")

	require.NoError(t, db.DB.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE id = $1`, expired.ID).Scan(&count))
	assert.Equal(t, 0, count, "expired family should be removed")

	require.NoError(t, db.DB.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE id = $1`, otherRevoked.ID).Scan(&count))
	assert.Equal(t, 1, count, "another user's revoked family should not be touched")
}
