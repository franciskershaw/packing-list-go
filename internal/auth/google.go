package auth

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type GoogleOAuthManager struct {
	config          *oauth2.Config
	verifier        *oidc.IDTokenVerifier
	stateStore      map[string]time.Time
	stateStoreMutex sync.Mutex
	stateExpiryTime time.Duration
}

type IDTokenClaims struct {
	Email       string `json:"email"`
	GoogleID    string `json:"sub"`
	DisplayName string `json:"name"`
	AvatarURL   string `json:"picture"`
}

// NewGoogleOAuthManager initializes the Google OAuth2 manager
func NewGoogleOAuthManager(clientID, clientSecret, redirectURL string) (*GoogleOAuthManager, error) {
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

	return &GoogleOAuthManager{
		config:          config,
		verifier:        verifier,
		stateStore:      make(map[string]time.Time),
		stateExpiryTime: 10 * time.Minute,
	}, nil
}

// GenerateState creates a random state string for CSRF protection
func (g *GoogleOAuthManager) GenerateState() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	state := make([]byte, 32)
	for i := range state {
		state[i] = charset[rand.Intn(len(charset))]
	}
	stateStr := string(state)

	// Store state with expiry
	g.stateStoreMutex.Lock()
	g.stateStore[stateStr] = time.Now().Add(g.stateExpiryTime)
	g.stateStoreMutex.Unlock()

	return stateStr
}

// ValidateState checks if the state is valid and hasn't expired
func (g *GoogleOAuthManager) ValidateState(state string) bool {
	g.stateStoreMutex.Lock()
	defer g.stateStoreMutex.Unlock()

	expiry, exists := g.stateStore[state]
	if !exists {
		return false
	}

	// Check if state has expired
	if time.Now().After(expiry) {
		delete(g.stateStore, state)
		return false
	}

	// State is valid, remove it (one-time use)
	delete(g.stateStore, state)
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
