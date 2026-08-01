package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type GoogleOAuthManager struct {
	config      *oauth2.Config
	verifier    *oidc.IDTokenVerifier
	stateSecret string
}

type IDTokenClaims struct {
	Email       string `json:"email"`
	GoogleID    string `json:"sub"`
	DisplayName string `json:"name"`
	AvatarURL   string `json:"picture"`
}

// NewGoogleOAuthManager initializes the Google OAuth2 manager
func NewGoogleOAuthManager(clientID, clientSecret, redirectURL, stateSecret string) (*GoogleOAuthManager, error) {
	ctx := context.Background()

	// Initialize OIDC provider for Google
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Create OAuth2 config
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			oidc.ScopeOpenID,
			"email",
			"profile",
		},
		Endpoint: provider.Endpoint(),
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return newGoogleOAuthManager(config, verifier, stateSecret), nil
}

// newGoogleOAuthManager builds a manager from an already-constructed
// config and verifier, so tests can inject a fake config directly
// instead of performing a live network call to Google's discovery
// endpoint (which NewGoogleOAuthManager still does for real callers).
func newGoogleOAuthManager(config *oauth2.Config, verifier *oidc.IDTokenVerifier, stateSecret string) *GoogleOAuthManager {
	return &GoogleOAuthManager{
		config:      config,
		verifier:    verifier,
		stateSecret: stateSecret,
	}
}

// GenerateState creates a signed, short-lived state token for CSRF
// protection (PACK-023 — not yet implemented).
func (g *GoogleOAuthManager) GenerateState() (string, error) {
	return "", fmt.Errorf("not implemented")
}

// ValidateState verifies the state token's signature and expiry
// (PACK-023 — not yet implemented). Stubbed to true rather than false so
// it doesn't trivially satisfy TestValidateState_InvalidSignature/_Expired,
// which both expect false.
func (g *GoogleOAuthManager) ValidateState(state string) bool {
	return true
}

// GetAuthURL returns the URL to redirect the user to Google's consent screen
func (g *GoogleOAuthManager) GetAuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCodeForToken exchanges the authorization code for tokens
func (g *GoogleOAuthManager) ExchangeCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	return token, nil
}

// VerifyIDToken parses and verifies the ID token, extracting user claims
func (g *GoogleOAuthManager) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*IDTokenClaims, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify id token: %w", err)
	}

	var claims IDTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return &claims, nil
}
