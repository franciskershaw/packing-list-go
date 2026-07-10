package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Email  string `json:"email"`
	UserID string `json:"userID"`
	jwt.RegisteredClaims
}

const (
	accessTokenExpiry  = 15 * time.Minute
	refreshTokenExpiry = 7 * 24 * time.Hour
)

func GenerateAccessToken(email string, userID string, secret string) (string, error) {
	claims := CustomClaims{
		Email:  email,
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return signToken(claims, secret)
}

func GenerateRefreshToken(userID string, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(refreshTokenExpiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return signToken(claims, secret)
}

// signToken is the shared signing helper for both access and refresh
// tokens; only the claims shape and expiry differ between them.
func signToken(claims jwt.Claims, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret key not set")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateAccessToken(tokenString string, secret string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	if err := parseToken(tokenString, secret, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func ValidateRefreshToken(tokenString string, secret string) (*jwt.RegisteredClaims, error) {
	claims := &jwt.RegisteredClaims{}
	if err := parseToken(tokenString, secret, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// parseToken is the shared parsing/verification helper for both access
// and refresh tokens; claims must be a pointer satisfying jwt.Claims so
// jwt.ParseWithClaims can populate it in place.
func parseToken(tokenString string, secret string, claims jwt.Claims) error {
	if secret == "" {
		return fmt.Errorf("secret key not set")
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}
