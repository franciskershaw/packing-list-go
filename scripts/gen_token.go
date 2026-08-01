//go:build ignore

// Generates a JWT access token for manual API testing against a local server
// and writes it into .env as DEV_TOKEN, so the requests/*.http files can
// pick it up via {{$dotenv DEV_TOKEN}} without any manual copy-paste. Also
// generates a refresh token and its matching refresh_tokens row (PACK-027),
// written to .env as DEV_REFRESH_TOKEN, so requests/auth.http can seed a
// refresh-cookie session without a real Google OAuth round-trip.
//
// Usage:
//
//	go run scripts/gen_token.go
//
// Requires JWT_SECRET_ACCESS and JWT_SECRET_REFRESH in .env (loaded
// automatically). Also requires DATABASE_URL — upserts a stable dev user so
// the token's userId satisfies the FK constraint on categories.user_id.
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

	jwtSecretRefresh := os.Getenv("JWT_SECRET_REFRESH")
	if jwtSecretRefresh == "" {
		fmt.Fprintln(os.Stderr, "error: JWT_SECRET_REFRESH not set")
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

	familyID := uuid.NewString()
	refreshToken, err := auth.GenerateRefreshToken(userID.String(), familyID, jwtSecretRefresh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "refresh token error: %v\n", err)
		os.Exit(1)
	}

	// Drop any prior dev refresh_tokens rows so this run's family is the
	// only one live for the dev user — old rows would just be dead weight.
	if _, err := db.Exec(`DELETE FROM refresh_tokens WHERE user_id = $1`, userID); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup refresh_tokens error: %v\n", err)
		os.Exit(1)
	}

	sum := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(sum[:])
	if _, err := db.Exec(
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		familyID, userID, tokenHash, time.Now().Add(7*24*time.Hour),
	); err != nil {
		fmt.Fprintf(os.Stderr, "insert refresh_tokens error: %v\n", err)
		os.Exit(1)
	}

	if err := upsertEnvVar(".env", "DEV_REFRESH_TOKEN", refreshToken); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write DEV_REFRESH_TOKEN to .env: %v\n", err)
		fmt.Println(refreshToken)
		return
	}
	fmt.Println("DEV_REFRESH_TOKEN written to .env")
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
