package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type UserRepository interface {
	GetOrCreateUser(ctx context.Context, email, googleID, displayName, avatarURL string) (*models.User, error)
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
}

type OAuthManager interface {
	GenerateState() string
	ValidateState(state string) bool
	GetAuthURL(state string) string
	ExchangeCodeForToken(ctx context.Context, code string) (*oauth2.Token, error)
	VerifyIDToken(ctx context.Context, token *oauth2.Token) (*auth.IDTokenClaims, error)
}

type AuthHandler struct {
	userRepo     UserRepository
	oauthManager OAuthManager
	cfg          *config.Config
}

func NewAuthHandler(userRepo UserRepository, oauthManager OAuthManager, cfg *config.Config) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, oauthManager: oauthManager, cfg: cfg}
}

// setRefreshCookie sets the refreshToken cookie with consistent
// Secure/SameSite/HttpOnly attributes for both issuing (GoogleCallback)
// and clearing (Logout) it. SameSite=Lax remains correct after
// PACK-032's redirect to a separate frontend origin: the frontend proxies
// API calls through its own origin in local dev, so this cookie only
// crosses directly to this API during the OAuth redirect itself, which
// Lax always permits. Revisit if a cross-origin production deployment
// changes that assumption.
func (h *AuthHandler) setRefreshCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("refreshToken", value, maxAge, "/", "", h.cfg.Environment == "production", true)
}

func (h *AuthHandler) LoginWithGoogle(c *gin.Context) {
	state := h.oauthManager.GenerateState()
	authURL := h.oauthManager.GetAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing authorization code"})
		return
	}

	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing state parameter"})
		return
	}

	if !h.oauthManager.ValidateState(state) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid state parameter"})
		return
	}

	ctx := context.Background()
	oauthToken, err := h.oauthManager.ExchangeCodeForToken(ctx, code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to exchange code for tokens"})
		return
	}

	idTokenClaims, err := h.oauthManager.VerifyIDToken(ctx, oauthToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to verify ID token"})
		return
	}

	user, err := h.userRepo.GetOrCreateUser(ctx, idTokenClaims.Email, idTokenClaims.GoogleID, idTokenClaims.DisplayName, idTokenClaims.AvatarURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process user"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), h.cfg.JWTSecretRefresh)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	h.setRefreshCookie(c, refreshToken, 7*24*60*60)

	// No access token or user data in the redirect — the frontend mints
	// its own access token via POST /auth/refresh immediately after
	// landing on this route, using the cookie just set above.
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/auth/callback", h.cfg.FrontendURL))
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refreshToken")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token missing"})
		return
	}

	claims, err := auth.ValidateRefreshToken(refreshToken, h.cfg.JWTSecretRefresh)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	ctx := context.Background()
	user, err := h.userRepo.GetUserByID(ctx, claims.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up user"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	newAccessToken, err := auth.GenerateAccessToken(user.Email, user.ID.String(), h.cfg.JWTSecretAccess)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"accessToken": newAccessToken})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.setRefreshCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// Me handles GET /me — returns the authenticated user's profile. Mirrors
// RefreshToken's GetUserByID lookup and nil,nil-vs-error handling.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.userRepo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up user"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        user.ID,
		"email":     user.Email,
		"name":      user.DisplayName,
		"avatarUrl": user.AvatarURL,
	})
}
