package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/franciskershaw/packing-list-go/internal/models"
)

type PostgresRefreshTokenRepository struct {
	db *sql.DB
}

func NewPostgresRefreshTokenRepository(db *sql.DB) *PostgresRefreshTokenRepository {
	return &PostgresRefreshTokenRepository{db: db}
}

func (r *PostgresRefreshTokenRepository) CreateFamily(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*models.RefreshTokenFamily, error) {
	return nil, errors.New("not implemented")
}

func (r *PostgresRefreshTokenRepository) FindFamilyByHash(ctx context.Context, userID, tokenHash string) (*models.RefreshTokenFamily, error) {
	return nil, errors.New("not implemented")
}

func (r *PostgresRefreshTokenRepository) RotateFamily(ctx context.Context, familyID, newTokenHash string, newExpiresAt time.Time) error {
	return errors.New("not implemented")
}

func (r *PostgresRefreshTokenRepository) RevokeFamily(ctx context.Context, familyID string) error {
	return errors.New("not implemented")
}

func (r *PostgresRefreshTokenRepository) DeleteStaleFamiliesForUser(ctx context.Context, userID string) error {
	return errors.New("not implemented")
}
