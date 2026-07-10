package handler

import (
	"context"
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
// and clearing (Logout) it. SameSite is Lax rather than None because no
// cross-origin frontend consumer exists yet; revisit if one is added.
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

	accessToken, err := auth.GenerateAccessToken(user.Email, user.ID.String(), h.cfg.JWTSecretAccess)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), h.cfg.JWTSecretRefresh)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	h.setRefreshCookie(c, refreshToken, 7*24*60*60)

	c.JSON(http.StatusOK, gin.H{
		"accessToken": accessToken,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.DisplayName,
		},
	})
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
