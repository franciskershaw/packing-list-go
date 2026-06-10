package utils

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

func GenerateAccessToken(email string, userId string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET_ACCESS")
	if secretKey == "" {
		return "", fmt.Errorf("secret key not set")
	}

	claims := CustomClaims{
		Email:  email,
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func GenerateRefreshToken(email string, userId string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET_REFRESH")
	if secretKey == "" {
		return "", fmt.Errorf("secret key not set")
	}

	claims := CustomClaims{
		Email:  email,
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func ValidateToken(tokenString string, isRefresh bool) (*CustomClaims, error) {
	var secretKey string
	if isRefresh {
		secretKey = os.Getenv("JWT_SECRET_REFRESH")
	} else {
		secretKey = os.Getenv("JWT_SECRET_ACCESS")
	}

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
