package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// newTestOAuthManager builds a GoogleOAuthManager from a fake config,
// with no network I/O — avoids the live call to Google's OIDC discovery
// endpoint that NewGoogleOAuthManager makes. verifier is nil, which is
// safe as long as no test calls VerifyIDToken (see PACK-017 handoff doc).
func newTestOAuthManager(clientID string) *GoogleOAuthManager {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes: []string{
			oidc.ScopeOpenID,
			"email",
			"profile",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
	return newGoogleOAuthManager(config, nil)
}

func TestGenerateState(t *testing.T) {
	manager := newTestOAuthManager("test-id")

	state := manager.GenerateState()

	if state == "" {
		t.Error("GenerateState returned empty string")
	}

	if len(state) != 32 {
		t.Errorf("expected state length 32, got %d", len(state))
	}
}

func TestValidateState(t *testing.T) {
	manager := newTestOAuthManager("test-id")

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
	manager := newTestOAuthManager("test-id")

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
	manager := newTestOAuthManager("test-id")

	authURL := manager.GetAuthURL("test-state")

	if authURL == "" {
		t.Error("GetAuthURL returned empty string")
	}

	if !strings.Contains(authURL, "client_id=test-id") {
		t.Error("auth URL missing client_id parameter")
	}

	if !strings.Contains(authURL, "redirect_uri") {
		t.Error("auth URL missing redirect_uri parameter")
	}

	if !strings.Contains(authURL, "state=test-state") {
		t.Error("auth URL missing state parameter")
	}
}
