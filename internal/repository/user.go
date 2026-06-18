package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

// GetOrCreateUser looks up a user by Google ID, or creates one if they don't exist
func GetOrCreateUser(ctx context.Context, email, googleID, displayName, avatarURL string) (*models.User, error) {
	// Try to get existing user by Google ID
	user, err := GetUserByGoogleID(ctx, googleID)
	if err == nil {
		// User exists, update last login
		err = updateLastLogin(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update last login: %w", err)
		}
		return user, nil
	}

	// User doesn't exist, create a new one
	user = &models.User{
		ID:          uuid.New(),
		GoogleID:    googleID,
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		CreatedAt:   time.Now(),
		LastLoginAt: time.Now(),
	}

	query := `
		INSERT INTO users (id, google_id, email, display_name, avatar_url, created_at, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = db.DB.ExecContext(ctx, query,
		user.ID,
		user.GoogleID,
		user.Email,
		user.DisplayName,
		user.AvatarURL,
		user.CreatedAt,
		user.LastLoginAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByGoogleID retrieves a user by their Google ID
func GetUserByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, google_id, email, display_name, avatar_url, created_at, last_login_at
		FROM users
		WHERE google_id = $1
	`

	err := db.DB.QueryRowContext(ctx, query, googleID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.DisplayName,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.LastLoginAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by their ID
func GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, google_id, email, display_name, avatar_url, created_at, last_login_at
		FROM users
		WHERE id = $1
	`

	err := db.DB.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.DisplayName,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.LastLoginAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}

// updateLastLogin updates the last_login_at timestamp for a user
func updateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET last_login_at = $1
		WHERE id = $2
	`

	_, err := db.DB.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}
