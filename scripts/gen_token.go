//go:build ignore

// Generates a JWT access token for manual API testing against a local server
// and writes it into .env as DEV_TOKEN, so the requests/*.http files can
// pick it up via {{$dotenv DEV_TOKEN}} without any manual copy-paste.
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
	"strings"
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
	devAvatarURL   = "https://kitted-api.internal/dev-avatar.png"
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

	jwtSecret := os.Getenv("JWT_SECRET_ACCESS")
	if jwtSecret == "" {
		fmt.Fprintln(os.Stderr, "error: JWT_SECRET_ACCESS not set")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// ON CONFLICT DO UPDATE backfills avatar_url on a pre-existing dev row
	// that predates this column being seeded (avatar_url is nullable in the
	// schema, and GetUserByID scans it into a non-nullable Go string — a
	// NULL there fails that scan with a real error, not a clean 404).
	_, err = db.Exec(`
		INSERT INTO users (id, google_id, email, display_name, avatar_url, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (google_id) DO UPDATE SET avatar_url = EXCLUDED.avatar_url`,
		preferredID, devGoogleID, devEmail, devDisplayName, devAvatarURL, time.Now(),
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

	token, err := auth.GenerateAccessToken(devEmail, userID.String(), jwtSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "token error: %v\n", err)
		os.Exit(1)
	}

	if err := upsertEnvVar(".env", "DEV_TOKEN", token); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write DEV_TOKEN to .env: %v\n", err)
		fmt.Println(token)
		return
	}

	fmt.Println("DEV_TOKEN written to .env")
}

// upsertEnvVar replaces the line "key=..." in the file at path with
// "key=value", or appends it if no such line exists. Other lines are left
// untouched.
func upsertEnvVar(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	prefix := key + "="
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = append(lines[:len(lines)-1], prefix+value, "")
		} else {
			lines = append(lines, prefix+value)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
