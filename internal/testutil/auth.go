package testutil

import (
	"fmt"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/auth"
)

// Shared JWT test secrets, so handler tests that need to construct an
// AuthMiddleware (to validate tokens minted by AuthHeader) reference the
// same value instead of duplicating the literal string.
const (
	TestJWTSecretAccess  = "test-secret-access"
	TestJWTSecretRefresh = "test-secret-refresh"
)

func AuthHeader(t *testing.T, email, userID string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(email, userID, TestJWTSecretAccess)
	if err != nil {
		t.Fatalf("failed to generate test auth token: %v", err)
	}
	return fmt.Sprintf("Bearer %s", token)
}
