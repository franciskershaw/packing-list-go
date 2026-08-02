package config

import (
	"os"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET_ACCESS", "test-access-secret")
	t.Setenv("JWT_SECRET_REFRESH", "test-refresh-secret")
	t.Setenv("JWT_SECRET_OAUTH_STATE", "test-oauth-state-secret")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
}

func TestLoad_EnvironmentDefaultsToDevelopment(t *testing.T) {
	setRequiredEnv(t)
	if err := os.Unsetenv("APP_ENV"); err != nil {
		t.Fatalf("failed to unset APP_ENV: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Environment != "development" {
		t.Errorf("expected Environment %q, got %q", "development", cfg.Environment)
	}
}

func TestLoad_EnvironmentReadsAppEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Environment != "production" {
		t.Errorf("expected Environment %q, got %q", "production", cfg.Environment)
	}
}

func TestLoad_RequiresFrontendURL(t *testing.T) {
	setRequiredEnv(t)
	if err := os.Unsetenv("FRONTEND_URL"); err != nil {
		t.Fatalf("failed to unset FRONTEND_URL: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to return an error when FRONTEND_URL is unset, got nil")
	}
}

func TestLoad_RequiresJWTSecretOAuthState(t *testing.T) {
	setRequiredEnv(t)
	if err := os.Unsetenv("JWT_SECRET_OAUTH_STATE"); err != nil {
		t.Fatalf("failed to unset JWT_SECRET_OAUTH_STATE: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to return an error when JWT_SECRET_OAUTH_STATE is unset, got nil")
	}
}

func TestLoad_TrustedProxiesDefaultsToEmpty(t *testing.T) {
	setRequiredEnv(t)
	if err := os.Unsetenv("TRUSTED_PROXIES"); err != nil {
		t.Fatalf("failed to unset TRUSTED_PROXIES: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("expected TrustedProxies to be empty, got %v", cfg.TrustedProxies)
	}
}

func TestLoad_TrustedProxiesParsesCommaSeparatedList(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1,10.0.0.2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	want := []string{"10.0.0.1", "10.0.0.2"}
	if len(cfg.TrustedProxies) != len(want) {
		t.Fatalf("expected %v, got %v", want, cfg.TrustedProxies)
	}
	for i, ip := range want {
		if cfg.TrustedProxies[i] != ip {
			t.Errorf("expected TrustedProxies[%d] = %q, got %q", i, ip, cfg.TrustedProxies[i])
		}
	}
}
