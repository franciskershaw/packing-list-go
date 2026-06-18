package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Email  string `json:"email"`
	UserId string `json:"userId"`
	jwt.RegisteredClaims
}

type TokenType int

const (
	AccessToken TokenType = iota
	RefreshToken
)

func generateToken(email string, userId string, tokenType TokenType) (string, error) {
	var secretKey string
	var expiry time.Duration

	switch tokenType {
	case AccessToken:
		secretKey = os.Getenv("JWT_SECRET_ACCESS")
		expiry = 15 * time.Minute
	case RefreshToken:
		secretKey = os.Getenv("JWT_SECRET_REFRESH")
		expiry = 7 * 24 * time.Hour
	default:
		return "", fmt.Errorf("unknown token type")
	}

	if secretKey == "" {
		return "", fmt.Errorf("secret key not set")
	}

	claims := CustomClaims{
		Email:  email,
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func GenerateAccessToken(email string, userId string) (string, error) {
	return generateToken(email, userId, AccessToken)
}

func GenerateRefreshToken(userId string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET_REFRESH")
	if secretKey == "" {
		return "", fmt.Errorf("secret key not set")
	}

	claims := jwt.RegisteredClaims{
		Subject:   userId,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func ValidateAccessToken(tokenString string) (*CustomClaims, error) {
	return validateToken(tokenString, os.Getenv("JWT_SECRET_ACCESS"))
}

func ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	secretKey := os.Getenv("JWT_SECRET_REFRESH")
	if secretKey == "" {
		return nil, fmt.Errorf("secret key not set")
	}

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func validateToken(tokenString string, secretKey string) (*CustomClaims, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("secret key not set")
	}

	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
