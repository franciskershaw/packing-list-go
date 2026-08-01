package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// Sliding refresh expiry and reuse-detection grace window (PACK-027).
const (
	refreshTokenTTL    = 7 * 24 * time.Hour
	refreshGraceWindow = 10 * time.Second
)

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type UserRepository interface {
	GetOrCreateUser(ctx context.Context, email, googleID, displayName, avatarURL string) (*models.User, error)
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
}

// RefreshTokenRepository backs PACK-027's rotation-with-reuse-detection scheme.
type RefreshTokenRepository interface {
	CreateFamily(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) (*models.RefreshTokenFamily, error)
	FindFamilyByID(ctx context.Context, id, userID string) (*models.RefreshTokenFamily, error)
	RotateFamily(ctx context.Context, familyID, newTokenHash string, newExpiresAt time.Time) error
	RevokeFamily(ctx context.Context, familyID string) error
	DeleteStaleFamiliesForUser(ctx context.Context, userID string) error
}

type OAuthManager interface {
	GenerateState() (string, error)
	ValidateState(state string) bool
	GetAuthURL(state string) string
	ExchangeCodeForToken(ctx context.Context, code string) (*oauth2.Token, error)
	VerifyIDToken(ctx context.Context, token *oauth2.Token) (*auth.IDTokenClaims, error)
}

type AuthHandler struct {
	userRepo         UserRepository
	oauthManager     OAuthManager
	refreshTokenRepo RefreshTokenRepository
	cfg              *config.Config
}

func NewAuthHandler(userRepo UserRepository, oauthManager OAuthManager, refreshTokenRepo RefreshTokenRepository, cfg *config.Config) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, oauthManager: oauthManager, refreshTokenRepo: refreshTokenRepo, cfg: cfg}
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

// LoginWithGoogle (PACK-023 — not yet implemented).
func (h *AuthHandler) LoginWithGoogle(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// validOAuthStateCookie is the PACK-023 double-submit check comparing the
// oauthState cookie to the query-param state (not yet implemented).
func (h *AuthHandler) validOAuthStateCookie(c *gin.Context, queryState string) bool {
	return false
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

	if !h.validOAuthStateCookie(c, state) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid state parameter"})
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
		internalError(c, "failed to process user", err)
		return
	}

	familyID := uuid.NewString()
	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID, h.cfg.JWTSecretRefresh)
	if err != nil {
		internalError(c, "failed to generate refresh token", err)
		return
	}

	if err := h.refreshTokenRepo.DeleteStaleFamiliesForUser(ctx, user.ID.String()); err != nil {
		internalError(c, "failed to clean up refresh tokens", err)
		return
	}
	if _, err := h.refreshTokenRepo.CreateFamily(ctx, familyID, user.ID.String(), hashRefreshToken(refreshToken), time.Now().Add(refreshTokenTTL)); err != nil {
		internalError(c, "failed to persist refresh token", err)
		return
	}

	h.setRefreshCookie(c, refreshToken, int(refreshTokenTTL.Seconds()))

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
	family, err := h.refreshTokenRepo.FindFamilyByID(ctx, claims.FamilyID, claims.Subject)
	if err != nil {
		internalError(c, "failed to look up refresh token", err)
		return
	}
	if family == nil || family.RevokedAt != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	// Found by family id, not by hash, so this always resolves to a real
	// family to kill — not just when the reuse happens to be one rotation
	// stale. Previous-hash match within the grace window is a benign
	// multi-tab race, not reuse; anything else here is reuse.
	hash := hashRefreshToken(refreshToken)
	matchedCurrent := hash == family.TokenHash
	if !matchedCurrent {
		matchedPreviousInWindow := family.PreviousTokenHash != nil && hash == *family.PreviousTokenHash &&
			family.PreviousTokenRotatedAt != nil && time.Since(*family.PreviousTokenRotatedAt) <= refreshGraceWindow
		if !matchedPreviousInWindow {
			if err := h.refreshTokenRepo.RevokeFamily(ctx, family.ID.String()); err != nil {
				internalError(c, "failed to revoke refresh token", err)
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}
	}

	user, err := h.userRepo.GetUserByID(ctx, claims.Subject)
	if err != nil {
		internalError(c, "failed to look up user", err)
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	newRefreshToken, err := auth.GenerateRefreshToken(user.ID.String(), family.ID.String(), h.cfg.JWTSecretRefresh)
	if err != nil {
		internalError(c, "failed to generate token", err)
		return
	}
	if err := h.refreshTokenRepo.RotateFamily(ctx, family.ID.String(), hashRefreshToken(newRefreshToken), time.Now().Add(refreshTokenTTL)); err != nil {
		internalError(c, "failed to rotate refresh token", err)
		return
	}
	h.setRefreshCookie(c, newRefreshToken, int(refreshTokenTTL.Seconds()))

	newAccessToken, err := auth.GenerateAccessToken(user.Email, user.ID.String(), h.cfg.JWTSecretAccess)
	if err != nil {
		internalError(c, "failed to generate token", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"accessToken": newAccessToken})
}

// Logout best-effort revokes the token's family (PACK-027) — a missing/
// invalid cookie or a lookup/revoke failure never blocks the response.
func (h *AuthHandler) Logout(c *gin.Context) {
	if refreshToken, err := c.Cookie("refreshToken"); err == nil {
		if claims, err := auth.ValidateRefreshToken(refreshToken, h.cfg.JWTSecretRefresh); err == nil {
			if err := h.refreshTokenRepo.RevokeFamily(context.Background(), claims.FamilyID); err != nil {
				_ = c.Error(fmt.Errorf("failed to revoke refresh token family during logout: %w", err))
			}
		}
	}

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
		internalError(c, "failed to look up user", err)
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
