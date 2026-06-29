package testutil

import (
	"fmt"
	"os"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/auth"
)

func AuthHeader(t *testing.T, email, userID string) string {
	t.Helper()
	os.Setenv("JWT_SECRET_ACCESS", "test-secret-access")
	token, err := auth.GenerateAccessToken(email, userID)
	if err != nil {
		t.Fatalf("failed to generate test auth token: %v", err)
	}
	return fmt.Sprintf("Bearer %s", token)
}
