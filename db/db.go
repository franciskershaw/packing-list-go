package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/lib/pq"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"

	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var DB *sql.DB

func InitDB() error {
	// Get Neon connection string
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}

	// Open the database
	var err error
	DB, err = sql.Open("postgres", connString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Println("Database connection established")

	// Run migrations
	err = runMigrations()
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func runMigrations() error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to determine migrations path")
	}
	migrationsPath := filepath.Join(filepath.Dir(thisFile), "migrations")

	m, err := migrate.New(
		"file://"+migrationsPath,
		os.Getenv("DATABASE_URL"),
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println("Migrations completed successfully")
	return nil
}

func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
