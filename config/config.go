package config

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Config struct {
	Port               string
	Environment        string
	DatabaseURL        string
	JWTSecretAccess    string
	JWTSecretRefresh   string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	GoogleOAuth2Config *oauth2.Config
	FrontendURL        string
	TrustedProxies     []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		Environment:        getEnv("APP_ENV", "development"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecretAccess:    os.Getenv("JWT_SECRET_ACCESS"),
		JWTSecretRefresh:   os.Getenv("JWT_SECRET_REFRESH"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URI"),
		FrontendURL:        os.Getenv("FRONTEND_URL"),
		TrustedProxies:     parseTrustedProxies(os.Getenv("TRUSTED_PROXIES")),
	}

	// Validate required env vars
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}
	if cfg.JWTSecretAccess == "" {
		return nil, fmt.Errorf("JWT_SECRET_ACCESS not set")
	}
	if cfg.JWTSecretRefresh == "" {
		return nil, fmt.Errorf("JWT_SECRET_REFRESH not set")
	}
	if cfg.FrontendURL == "" {
		return nil, fmt.Errorf("FRONTEND_URL not set")
	}

	// Initialize Google OAuth2 config
	cfg.GoogleOAuth2Config = &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func parseTrustedProxies(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			proxies = append(proxies, trimmed)
		}
	}
	return proxies
}
