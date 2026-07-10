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
}

func TestLoad_EnvironmentDefaultsToDevelopment(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("APP_ENV")

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
