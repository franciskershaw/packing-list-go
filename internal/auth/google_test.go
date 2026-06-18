package auth

import (
	"testing"
	"time"
)

func TestGenerateState(t *testing.T) {
	manager, err := NewGoogleOAuthManager("test-id", "test-secret", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	state := manager.GenerateState()

	if state == "" {
		t.Error("GenerateState returned empty string")
	}

	if len(state) != 32 {
		t.Errorf("expected state length 32, got %d", len(state))
	}
}

func TestValidateState(t *testing.T) {
	manager, err := NewGoogleOAuthManager("test-id", "test-secret", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	state := manager.GenerateState()

	// Valid state should return true
	if !manager.ValidateState(state) {
		t.Error("ValidateState returned false for valid state")
	}

	// Same state should return false (one-time use)
	if manager.ValidateState(state) {
		t.Error("ValidateState returned true for reused state")
	}
}

func TestValidateStateExpiry(t *testing.T) {
	manager, err := NewGoogleOAuthManager("test-id", "test-secret", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Set expiry to 1 millisecond so it expires immediately
	manager.stateExpiryTime = 1 * time.Millisecond

	state := manager.GenerateState()
	time.Sleep(2 * time.Millisecond)

	// Expired state should return false
	if manager.ValidateState(state) {
		t.Error("ValidateState returned true for expired state")
	}
}

func TestGetAuthURL(t *testing.T) {
	manager, err := NewGoogleOAuthManager("test-id", "test-secret", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	authURL := manager.GetAuthURL("test-state")

	if authURL == "" {
		t.Error("GetAuthURL returned empty string")
	}

	if !contains(authURL, "client_id=test-id") {
		t.Error("auth URL missing client_id parameter")
	}

	if !contains(authURL, "redirect_uri") {
		t.Error("auth URL missing redirect_uri parameter")
	}

	if !contains(authURL, "state=test-state") {
		t.Error("auth URL missing state parameter")
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
