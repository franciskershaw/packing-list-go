package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/repository"
	"github.com/gin-gonic/gin"
)

// LoginWithGoogle redirects the user to Google's consent screen
func LoginWithGoogle(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		manager, err := auth.NewGoogleOAuthManager(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.GoogleRedirectURL,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to initialize Google OAuth",
			})
			return
		}

		// Generate state for CSRF protection
		state := manager.GenerateState()

		// Get the Google consent URL
		authURL := manager.GetAuthURL(state)

		// Redirect to Google
		c.Redirect(http.StatusTemporaryRedirect, authURL)
	}
}

// GoogleCallback handles the OAuth callback from Google
func GoogleCallback(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract code and state from query params
		code := c.Query("code")
		state := c.Query("state")

		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "missing authorization code",
			})
			return
		}

		if state == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "missing state parameter",
			})
			return
		}

		// Initialize Google OAuth manager
		manager, err := auth.NewGoogleOAuthManager(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.GoogleRedirectURL,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to initialize Google OAuth",
			})
			return
		}

		// Verify state (CSRF protection)
		if !manager.ValidateState(state) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid state parameter",
			})
			return
		}

		// Exchange authorization code for tokens
		ctx := context.Background()
		oauthToken, err := manager.ExchangeCodeForToken(ctx, code)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "failed to exchange code for tokens",
			})
			return
		}

		// Verify ID token and extract user claims
		idTokenClaims, err := manager.VerifyIDToken(ctx, oauthToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "failed to verify ID token",
			})
			return
		}

		// Look up or create user in database
		user, err := repository.GetOrCreateUser(ctx, idTokenClaims.Email, idTokenClaims.GoogleID, idTokenClaims.DisplayName, idTokenClaims.AvatarURL)
		if err != nil {
			fmt.Printf("Failed to get or create user: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to process user",
			})
			return
		}

		// Generate our own access and refresh tokens
		accessToken, err := auth.GenerateAccessToken(user.Email, user.ID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to generate access token",
			})
			return
		}

		refreshToken, err := auth.GenerateRefreshToken(user.ID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to generate refresh token",
			})
			return
		}

		// Set refresh token as HTTP-only cookie
		c.SetCookie(
			"refreshToken",
			refreshToken,
			7*24*60*60, // 7 days in seconds
			"/",
			"",    // domain
			false, // secure (set to true in production with HTTPS)
			true,  // httpOnly
		)

		// Return access token to client
		c.JSON(http.StatusOK, gin.H{
			"accessToken": accessToken,
			"user": gin.H{
				"id":    user.ID,
				"email": user.Email,
				"name":  user.DisplayName,
			},
		})
	}
}

// RefreshToken issues a new access token using a valid refresh token
func RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refreshToken")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "refresh token missing",
		})
		return
	}

	claims, err := auth.ValidateRefreshToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid refresh token",
		})
		return
	}

	// Look up user to get email
	ctx := context.Background()
	user, err := repository.GetUserByID(ctx, claims.Subject)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not found",
		})
		return
	}

	// Issue new access token
	newAccessToken, err := auth.GenerateAccessToken(user.Email, user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to generate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken": newAccessToken,
	})
}

// Logout clears the refresh token cookie
func Logout(c *gin.Context) {
	c.SetCookie(
		"refreshToken",
		"",
		-1, // maxAge -1 deletes the cookie
		"/",
		"",
		false,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "logged out successfully",
	})
}
