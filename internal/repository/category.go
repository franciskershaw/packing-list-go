package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) GetCategories(ctx context.Context, userID string) ([]models.Category, error) {
	query := `
		SELECT id, name, user_id
		FROM categories
		WHERE user_id IS NULL OR user_id = $1
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			slog.Error("failed to close rows", "err", cerr)
		}
	}()

	categories := make([]models.Category, 0)
	for rows.Next() {
		cat, err := scanCategory(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, *cat)
	}
	return categories, rows.Err()
}

func (r *CategoryRepository) CreateCategory(ctx context.Context, userID, name string) (*models.Category, error) {
	query := `
		INSERT INTO categories (user_id, name)
		VALUES ($1, $2)
		RETURNING id, name, user_id
	`
	row := r.db.QueryRowContext(ctx, query, userID, name)
	cat, err := scanCategory(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return cat, nil
}

func (r *CategoryRepository) GetCategoryByID(ctx context.Context, id string) (*models.Category, error) {
	query := `SELECT id, name, user_id FROM categories WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	cat, err := scanCategory(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return cat, nil
}

func (r *CategoryRepository) UpdateCategory(ctx context.Context, id, name string) (*models.Category, error) {
	query := `
		UPDATE categories SET name = $1 WHERE id = $2
		RETURNING id, name, user_id
	`
	row := r.db.QueryRowContext(ctx, query, name, id)
	cat, err := scanCategory(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}
	return cat, nil
}

func (r *CategoryRepository) DeleteCategory(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) CategoryNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error) {
	var exists bool
	var err error
	if excludeID != nil {
		err = r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(name) = LOWER($1) AND user_id = $2 AND id != $3)`,
			name, userID, *excludeID,
		).Scan(&exists)
	} else {
		err = r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(name) = LOWER($1) AND user_id = $2)`,
			name, userID,
		).Scan(&exists)
	}
	if err != nil {
		return false, fmt.Errorf("failed to check category name: %w", err)
	}
	return exists, nil
}

func (r *CategoryRepository) CategoryHasItems(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM items WHERE category_id = $1)`, id,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check category items: %w", err)
	}
	return exists, nil
}

// scanCategory abstracts the nullable user_id scan pattern for a single category row.
func scanCategory(scan func(...any) error) (*models.Category, error) {
	var (
		id         uuid.UUID
		name       string
		userIDNull sql.NullString
	)
	if err := scan(&id, &name, &userIDNull); err != nil {
		return nil, err
	}

	cat := &models.Category{ID: id, Name: name}
	if userIDNull.Valid {
		uid, err := uuid.Parse(userIDNull.String)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id UUID %q: %w", userIDNull.String, err)
		}
		cat.UserID = &uid
	}
	cat.IsSystem = cat.UserID == nil
	return cat, nil
}
