package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/franciskershaw/packing-list-go/internal/models"
)

type PostgresRefreshTokenRepository struct {
	db *sql.DB
}

func NewPostgresRefreshTokenRepository(db *sql.DB) *PostgresRefreshTokenRepository {
	return &PostgresRefreshTokenRepository{db: db}
}

// CreateFamily takes an explicit id (the caller generates it) rather than
// relying on the DB default — the id must be known before this call so it
// can be embedded in the refresh token's familyId claim (PACK-027).
func (r *PostgresRefreshTokenRepository) CreateFamily(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) (*models.RefreshTokenFamily, error) {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token_hash, previous_token_hash, previous_token_rotated_at, expires_at, revoked_at, created_at
	`
	row := r.db.QueryRowContext(ctx, query, id, userID, tokenHash, expiresAt)
	family, err := scanRefreshTokenFamily(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token family: %w", err)
	}
	return family, nil
}

// FindFamilyByID looks up a family by its id (from the token's familyId
// claim), not by hash — so a family is always found regardless of how many
// rotations stale the presented token is, letting reuse always be revoked.
func (r *PostgresRefreshTokenRepository) FindFamilyByID(ctx context.Context, id, userID string) (*models.RefreshTokenFamily, error) {
	query := `
		SELECT id, user_id, token_hash, previous_token_hash, previous_token_rotated_at, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE id = $1 AND user_id = $2
	`
	row := r.db.QueryRowContext(ctx, query, id, userID)
	family, err := scanRefreshTokenFamily(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find refresh token family: %w", err)
	}
	return family, nil
}

// RotateFamily shifts the current hash into previous_token_hash and sets
// the new current hash/expiry — one overwritten row, never an appended chain.
func (r *PostgresRefreshTokenRepository) RotateFamily(ctx context.Context, familyID, newTokenHash string, newExpiresAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET previous_token_hash = token_hash,
		    previous_token_rotated_at = CURRENT_TIMESTAMP,
		    token_hash = $1,
		    expires_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, newTokenHash, newExpiresAt, familyID)
	if err != nil {
		return fmt.Errorf("failed to rotate refresh token family: %w", err)
	}
	return nil
}

func (r *PostgresRefreshTokenRepository) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = $1`, familyID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token family: %w", err)
	}
	return nil
}

func (r *PostgresRefreshTokenRepository) DeleteStaleFamiliesForUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1 AND (revoked_at IS NOT NULL OR expires_at < CURRENT_TIMESTAMP)`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete stale refresh token families: %w", err)
	}
	return nil
}

// scanRefreshTokenFamily abstracts the nullable previous-hash/rotated-at/
// revoked-at scan pattern for a single refresh_tokens row.
func scanRefreshTokenFamily(scan func(...any) error) (*models.RefreshTokenFamily, error) {
	var (
		family                 models.RefreshTokenFamily
		previousTokenHash      sql.NullString
		previousTokenRotatedAt sql.NullTime
		revokedAt              sql.NullTime
	)
	if err := scan(&family.ID, &family.UserID, &family.TokenHash, &previousTokenHash, &previousTokenRotatedAt, &family.ExpiresAt, &revokedAt, &family.CreatedAt); err != nil {
		return nil, err
	}

	if previousTokenHash.Valid {
		family.PreviousTokenHash = &previousTokenHash.String
	}
	if previousTokenRotatedAt.Valid {
		family.PreviousTokenRotatedAt = &previousTokenRotatedAt.Time
	}
	if revokedAt.Valid {
		family.RevokedAt = &revokedAt.Time
	}
	return &family, nil
}
