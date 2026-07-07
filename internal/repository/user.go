package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

// PostgresUserRepository implements the UserRepository interface against a real database.
type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// GetOrCreateUser looks up a user by Google ID, or creates one if they don't exist
func (r *PostgresUserRepository) GetOrCreateUser(ctx context.Context, email, googleID, displayName, avatarURL string) (*models.User, error) {
	// Try to get existing user by Google ID
	user, err := r.getUserByGoogleID(ctx, googleID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to look up user: %w", err)
		}
		// user not found — fall through to create
	} else {
		// User exists, update last login
		now := time.Now()
		if err := r.updateLastLogin(ctx, user.ID, now); err != nil {
			return nil, fmt.Errorf("failed to update last login: %w", err)
		}
		user.LastLoginAt = now
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

	_, err = r.db.ExecContext(ctx, query,
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

// GetUserByID retrieves a user by their ID
func (r *PostgresUserRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, google_id, email, display_name, avatar_url, created_at, last_login_at
		FROM users
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
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

// getUserByGoogleID retrieves a user by their Google ID
func (r *PostgresUserRepository) getUserByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, google_id, email, display_name, avatar_url, created_at, last_login_at
		FROM users
		WHERE google_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, googleID).Scan(
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
func (r *PostgresUserRepository) updateLastLogin(ctx context.Context, userID uuid.UUID, lastLoginAt time.Time) error {
	query := `
		UPDATE users
		SET last_login_at = $1
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, lastLoginAt, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}
