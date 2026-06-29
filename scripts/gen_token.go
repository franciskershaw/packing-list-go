//go:build ignore

// Generates a JWT access token for manual API testing against a local server.
//
// Usage:
//
//	go run scripts/gen_token.go
//
// Requires JWT_SECRET_ACCESS in .env (loaded automatically).
// Also requires DATABASE_URL — upserts a stable dev user so the token's userId
// satisfies the FK constraint on categories.user_id.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

const (
	devGoogleID    = "dev-tools-kitted-test-account"
	devEmail       = "dev-test@kitted-api.internal"
	devDisplayName = "Dev Test User"
)

func main() {
	if os.Getenv("APP_ENV") == "production" {
		fmt.Fprintln(os.Stderr, "refusing to run in production")
		os.Exit(1)
	}

	preferredID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := preferredID

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "error: DATABASE_URL not set")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// ON CONFLICT DO NOTHING handles any unique constraint collision.
	_, err = db.Exec(`
		INSERT INTO users (id, google_id, email, display_name, last_login_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		preferredID, devGoogleID, devEmail, devDisplayName, time.Now(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert user error: %v\n", err)
		os.Exit(1)
	}

	// Look up by google_id to get the actual row ID in case of a prior conflict.
	if err = db.QueryRow(
		`SELECT id FROM users WHERE google_id = $1`, devGoogleID,
	).Scan(&userID); err != nil {
		fmt.Fprintf(os.Stderr, "lookup user error: %v\n", err)
		os.Exit(1)
	}

	token, err := auth.GenerateAccessToken(devEmail, userID.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "token error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(token)
}
