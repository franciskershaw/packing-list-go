package auth

import "testing"

const (
	testAccessSecret  = "test-secret-access"
	testRefreshSecret = "test-secret-refresh"
)

func TestGenerateAccessToken(t *testing.T) {
	token, err := GenerateAccessToken("test@example.com", "user-123", testAccessSecret)
	if err != nil {
		t.Errorf("GenerateAccessToken failed: %v", err)
	}

	if token == "" {
		t.Errorf("expected token, got empty string")
	}

	claims, err := ValidateAccessToken(token, testAccessSecret)
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
	token, err := GenerateRefreshToken("user-123", testRefreshSecret)
	if err != nil {
		t.Errorf("GenerateRefreshToken failed: %v", err)
	}

	if token == "" {
		t.Errorf("expected token, got empty string")
	}

	claims, err := ValidateRefreshToken(token, testRefreshSecret)
	if err != nil {
		t.Errorf("ValidateRefreshToken failed: %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("expected Subject user-123, got %s", claims.Subject)
	}
}

func TestGenerateAccessToken_EmptySecret(t *testing.T) {
	_, err := GenerateAccessToken("test@example.com", "user-123", "")
	if err == nil {
		t.Error("expected error for empty secret, got nil")
	}
}

func TestGenerateRefreshToken_EmptySecret(t *testing.T) {
	_, err := GenerateRefreshToken("user-123", "")
	if err == nil {
		t.Error("expected error for empty secret, got nil")
	}
}

func TestValidateAccessToken_EmptySecret(t *testing.T) {
	token, err := GenerateAccessToken("test@example.com", "user-123", testAccessSecret)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = ValidateAccessToken(token, "")
	if err == nil {
		t.Error("expected error for empty secret, got nil")
	}
}

func TestValidateRefreshToken_EmptySecret(t *testing.T) {
	token, err := GenerateRefreshToken("user-123", testRefreshSecret)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = ValidateRefreshToken(token, "")
	if err == nil {
		t.Error("expected error for empty secret, got nil")
	}
}
