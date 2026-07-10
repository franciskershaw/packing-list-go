package repository_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/repository"
	"github.com/google/uuid"
)

var (
	catRepo         *repository.CategoryRepository
	itemRepo        *repository.ItemRepository
	templateRepo    *repository.TemplateRepository
	packingListRepo *repository.PackingListRepository
	userRepo        *repository.PostgresUserRepository
	repoUserID      uuid.UUID
)

func TestMain(m *testing.M) {
	if os.Getenv("DATABASE_URL") == "" {
		fmt.Println("skipping repository tests: DATABASE_URL not set")
		os.Exit(0)
	}

	if err := db.InitDB(os.Getenv("DATABASE_URL")); err != nil {
		fmt.Printf("failed to init db: %v\n", err)
		os.Exit(1)
	}

	catRepo = repository.NewCategoryRepository(db.DB)
	itemRepo = repository.NewItemRepository(db.DB)
	templateRepo = repository.NewTemplateRepository(db.DB)
	packingListRepo = repository.NewPackingListRepository(db.DB)
	userRepo = repository.NewPostgresUserRepository(db.DB)
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
