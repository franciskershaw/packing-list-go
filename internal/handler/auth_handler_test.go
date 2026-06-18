// internal/handler/auth_handler_test.go
package handler_test

import (
	"os"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/auth"
)

func init() {
	os.Setenv("JWT_SECRET_ACCESS", "test-secret-access")
	os.Setenv("JWT_SECRET_REFRESH", "test-secret-refresh")
}

func TestGenerateAccessToken(t *testing.T) {
	token, err := auth.GenerateAccessToken("test@example.com", "user-123")
	if err != nil {
		t.Errorf("GenerateAccessToken failed: %v", err)
	}

	if token == "" {
		t.Errorf("expected token, got empty string")
	}

	// Validate it
	claims, err := auth.ValidateAccessToken(token)
	if err != nil {
		t.Errorf("ValidateAccessToken failed: %v", err)
	}

	if claims.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", claims.Email)
	}

	if claims.UserId != "user-123" {
		t.Errorf("expected userId user-123, got %s", claims.UserId)
	}
}

func TestRefreshToken(t *testing.T) {
	token, err := auth.GenerateRefreshToken("user-123")
	if err != nil {
		t.Errorf("GenerateRefreshToken failed: %v", err)
	}

	if token == "" {
		t.Errorf("expected token, got empty string")
	}

	claims, err := auth.ValidateRefreshToken(token)
	if err != nil {
		t.Errorf("ValidateRefreshToken failed: %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("expected Subject user-123, got %s", claims.Subject)
	}
}
