package handler_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/handler"
	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/franciskershaw/packing-list-go/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Mocks ---

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetOrCreateUser(ctx context.Context, email, googleID, displayName, avatarURL string) (*models.User, error) {
	args := m.Called(ctx, email, googleID, displayName, avatarURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

type MockOAuthManager struct {
	mock.Mock
}

func (m *MockOAuthManager) GenerateState() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockOAuthManager) ValidateState(state string) bool {
	args := m.Called(state)
	return args.Bool(0)
}

func (m *MockOAuthManager) GetAuthURL(state string) string {
	args := m.Called(state)
	return args.String(0)
}

func (m *MockOAuthManager) ExchangeCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

func (m *MockOAuthManager) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*auth.IDTokenClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.IDTokenClaims), args.Error(1)
}

type MockRefreshTokenRepository struct {
	mock.Mock
}

func (m *MockRefreshTokenRepository) CreateFamily(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) (*models.RefreshTokenFamily, error) {
	args := m.Called(ctx, id, userID, tokenHash, expiresAt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RefreshTokenFamily), args.Error(1)
}

func (m *MockRefreshTokenRepository) FindFamilyByID(ctx context.Context, id, userID string) (*models.RefreshTokenFamily, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RefreshTokenFamily), args.Error(1)
}

func (m *MockRefreshTokenRepository) RotateFamily(ctx context.Context, familyID, newTokenHash string, newExpiresAt time.Time) error {
	args := m.Called(ctx, familyID, newTokenHash, newExpiresAt)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) RevokeFamily(ctx context.Context, familyID string) error {
	args := m.Called(ctx, familyID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) DeleteStaleFamiliesForUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// --- Helpers ---

func newTestRouter(h *handler.AuthHandler) *gin.Engine {
	r := gin.New()
	r.GET("/auth/google/callback", h.GoogleCallback)
	r.POST("/auth/refresh", h.RefreshToken)
	r.POST("/auth/logout", h.Logout)
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware(testutil.TestJWTSecretAccess))
	authed.GET("/me", h.Me)
	return r
}

// testConfig builds a minimal *config.Config for handler construction.
func testConfig(environment string) *config.Config {
	return &config.Config{
		Environment:      environment,
		JWTSecretAccess:  testutil.TestJWTSecretAccess,
		JWTSecretRefresh: testutil.TestJWTSecretRefresh,
		FrontendURL:      "http://localhost:5173",
	}
}

// hashRefreshToken mirrors the handler's own hashing, for mock expectations.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func testUser() *models.User {
	return &models.User{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Email:       "test@example.com",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/avatar.png",
		GoogleID:    "google-123",
		CreatedAt:   time.Now(),
		LastLoginAt: time.Now(),
	}
}

// --- GoogleCallback tests ---

func TestGoogleCallback_HappyPath(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	fakeToken := &oauth2.Token{}
	fakeClaims := &auth.IDTokenClaims{
		Email:       "test@example.com",
		GoogleID:    "google-123",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/avatar.png",
	}
	user := testUser()

	oauthMgr.On("ValidateState", "valid-state").Return(true)
	oauthMgr.On("ExchangeCodeForToken", mock.Anything, "auth-code").Return(fakeToken, nil)
	oauthMgr.On("VerifyIDToken", mock.Anything, fakeToken).Return(fakeClaims, nil)
	userRepo.On("GetOrCreateUser", mock.Anything, "test@example.com", "google-123", "Test User", "https://example.com/avatar.png").Return(user, nil)
	refreshTokenRepo.On("DeleteStaleFamiliesForUser", mock.Anything, user.ID.String()).Return(nil)
	refreshTokenRepo.On("CreateFamily", mock.Anything, mock.AnythingOfType("string"), user.ID.String(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(&models.RefreshTokenFamily{ID: uuid.New(), UserID: user.ID}, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code&state=valid-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "http://localhost:5173/auth/callback", w.Header().Get("Location"))
	assert.NotContains(t, w.Body.String(), "accessToken", "access token must never appear in the callback response body")
	assert.NotContains(t, w.Body.String(), user.AvatarURL, "user data must never appear in the callback response body")

	// Refresh token cookie must be set
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refreshToken" {
			refreshCookie = c
		}
	}
	assert.NotNil(t, refreshCookie, "expected refreshToken cookie to be set")
	assert.True(t, refreshCookie.HttpOnly)
	assert.False(t, refreshCookie.Secure, "expected Secure=false in development")
	assert.Equal(t, http.SameSiteLaxMode, refreshCookie.SameSite)

	oauthMgr.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestGoogleCallback_HappyPath_SecureCookieInProduction(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	fakeToken := &oauth2.Token{}
	fakeClaims := &auth.IDTokenClaims{
		Email:       "test@example.com",
		GoogleID:    "google-123",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/avatar.png",
	}
	user := testUser()

	oauthMgr.On("ValidateState", "valid-state").Return(true)
	oauthMgr.On("ExchangeCodeForToken", mock.Anything, "auth-code").Return(fakeToken, nil)
	oauthMgr.On("VerifyIDToken", mock.Anything, fakeToken).Return(fakeClaims, nil)
	userRepo.On("GetOrCreateUser", mock.Anything, "test@example.com", "google-123", "Test User", "https://example.com/avatar.png").Return(user, nil)
	refreshTokenRepo.On("DeleteStaleFamiliesForUser", mock.Anything, user.ID.String()).Return(nil)
	refreshTokenRepo.On("CreateFamily", mock.Anything, mock.AnythingOfType("string"), user.ID.String(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(&models.RefreshTokenFamily{ID: uuid.New(), UserID: user.ID}, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("production"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code&state=valid-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "http://localhost:5173/auth/callback", w.Header().Get("Location"))

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refreshToken" {
			refreshCookie = c
		}
	}
	assert.NotNil(t, refreshCookie, "expected refreshToken cookie to be set")
	assert.True(t, refreshCookie.Secure, "expected Secure=true in production")
	assert.Equal(t, http.SameSiteLaxMode, refreshCookie.SameSite)

	oauthMgr.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestGoogleCallback_CreatesFamilyAndSweepsStale(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	fakeToken := &oauth2.Token{}
	fakeClaims := &auth.IDTokenClaims{
		Email:       "test@example.com",
		GoogleID:    "google-123",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/avatar.png",
	}
	user := testUser()

	oauthMgr.On("ValidateState", "valid-state").Return(true)
	oauthMgr.On("ExchangeCodeForToken", mock.Anything, "auth-code").Return(fakeToken, nil)
	oauthMgr.On("VerifyIDToken", mock.Anything, fakeToken).Return(fakeClaims, nil)
	userRepo.On("GetOrCreateUser", mock.Anything, "test@example.com", "google-123", "Test User", "https://example.com/avatar.png").Return(user, nil)
	refreshTokenRepo.On("DeleteStaleFamiliesForUser", mock.Anything, user.ID.String()).Return(nil)
	refreshTokenRepo.On("CreateFamily", mock.Anything, mock.AnythingOfType("string"), user.ID.String(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(&models.RefreshTokenFamily{ID: uuid.New(), UserID: user.ID}, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code&state=valid-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refreshToken" {
			refreshCookie = c
		}
	}
	assert.NotNil(t, refreshCookie, "expected refreshToken cookie to be set")

	oauthMgr.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestGoogleCallback_InvalidState(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	oauthMgr.On("ValidateState", "bad-state").Return(false)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code&state=bad-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	oauthMgr.AssertExpectations(t)
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=some-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGoogleCallback_MissingState(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- RefreshToken tests ---

func TestRefreshToken_ValidCookie(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()
	familyID := uuid.New()

	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	family := &models.RefreshTokenFamily{ID: familyID, UserID: user.ID, TokenHash: hashRefreshToken(refreshToken)}
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	refreshTokenRepo.On("RotateFamily", mock.Anything, family.ID.String(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.NotEmpty(t, body["accessToken"])

	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestRefreshToken_UserNotFound(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()
	familyID := uuid.New()

	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	family := &models.RefreshTokenFamily{ID: familyID, UserID: user.ID, TokenHash: hashRefreshToken(refreshToken)}
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(nil, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "user not found", body["error"])

	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestRefreshToken_UserLookupError(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()
	familyID := uuid.New()

	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	family := &models.RefreshTokenFamily{ID: familyID, UserID: user.ID, TokenHash: hashRefreshToken(refreshToken)}
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(nil, errors.New("db error"))

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "failed to look up user", body["error"])

	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestRefreshToken_MissingCookie(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: "not.a.valid.token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefreshToken_RotatesOnCurrentHash(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()

	familyID := uuid.New()
	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	family := &models.RefreshTokenFamily{
		ID:        familyID,
		UserID:    user.ID,
		TokenHash: hashRefreshToken(refreshToken),
	}

	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil)
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	refreshTokenRepo.On("RotateFamily", mock.Anything, familyID.String(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.NotEmpty(t, body["accessToken"])

	cookies := w.Result().Cookies()
	var newRefreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refreshToken" {
			newRefreshCookie = c
		}
	}
	require.NotNil(t, newRefreshCookie, "expected a rotated refreshToken cookie")
	assert.NotEqual(t, refreshToken, newRefreshCookie.Value, "expected a new refresh token value on rotation")

	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestRefreshToken_RotatesWithinGraceWindowOnPreviousHash(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()

	familyID := uuid.New()
	staleToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	rotatedAt := time.Now().Add(-2 * time.Second) // within the 10s grace window
	previousHash := hashRefreshToken(staleToken)
	family := &models.RefreshTokenFamily{
		ID:                     familyID,
		UserID:                 user.ID,
		TokenHash:              "repo-current-hash-not-presented",
		PreviousTokenHash:      &previousHash,
		PreviousTokenRotatedAt: &rotatedAt,
	}

	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil)
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	refreshTokenRepo.On("RotateFamily", mock.Anything, familyID.String(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: staleToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "a previous-hash match within the grace window should still succeed")

	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

func TestRefreshToken_RevokesOnStaleReuseOutsideGraceWindow(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()

	familyID := uuid.New()
	staleToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	rotatedAt := time.Now().Add(-30 * time.Second) // outside the 10s grace window
	previousHash := hashRefreshToken(staleToken)
	family := &models.RefreshTokenFamily{
		ID:                     familyID,
		UserID:                 user.ID,
		TokenHash:              "repo-current-hash-not-presented",
		PreviousTokenHash:      &previousHash,
		PreviousTokenRotatedAt: &rotatedAt,
	}

	// .Maybe(): the handler rejects before user lookup, so this shouldn't
	// be called — .Maybe() just avoids a testify panic if that regresses.
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil).Maybe()
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	refreshTokenRepo.On("RevokeFamily", mock.Anything, familyID.String()).Return(nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: staleToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "reuse outside the grace window must be rejected")

	refreshTokenRepo.AssertExpectations(t)
}

func TestRefreshToken_RejectsAlreadyRevokedFamily(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()

	familyID := uuid.New()
	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	revokedAt := time.Now().Add(-time.Minute)
	family := &models.RefreshTokenFamily{
		ID:        familyID,
		UserID:    user.ID,
		TokenHash: hashRefreshToken(refreshToken),
		RevokedAt: &revokedAt,
	}

	// .Maybe(): see TestRefreshToken_RevokesOnStaleReuseOutsideGraceWindow.
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil).Maybe()
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	refreshTokenRepo.AssertExpectations(t)
}

// TestRefreshToken_RevokesOnMultiGenerationStaleReuse covers the gap manual
// verification found: a token whose hash matches neither the family's
// current nor previous hash (2+ rotations stale) must still resolve to its
// family via the familyId claim and get revoked — not silently 401 with the
// live family left untouched.
func TestRefreshToken_RevokesOnMultiGenerationStaleReuse(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()

	familyID := uuid.New()
	staleToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	currentHash := "repo-current-hash-not-presented"
	previousHash := "repo-previous-hash-not-presented"
	family := &models.RefreshTokenFamily{
		ID:                familyID,
		UserID:            user.ID,
		TokenHash:         currentHash,
		PreviousTokenHash: &previousHash,
	}

	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil).Maybe()
	refreshTokenRepo.On("FindFamilyByID", mock.Anything, familyID.String(), user.ID.String()).Return(family, nil)
	refreshTokenRepo.On("RevokeFamily", mock.Anything, familyID.String()).Return(nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: staleToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	refreshTokenRepo.AssertExpectations(t)
}

// --- Logout tests ---

func TestLogout_ClearsCookie(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: "some-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "logged out successfully", body["message"])

	// Cookie should be cleared (MaxAge < 0)
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refreshToken" {
			refreshCookie = c
		}
	}
	assert.NotNil(t, refreshCookie)
	assert.True(t, refreshCookie.MaxAge < 0, "expected refreshToken cookie to be expired, got MaxAge=%d", refreshCookie.MaxAge)
	assert.False(t, refreshCookie.Secure, "expected Secure=false in development")
	assert.Equal(t, http.SameSiteLaxMode, refreshCookie.SameSite)
}

func TestLogout_RevokesMatchingFamily(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()

	familyID := uuid.New()
	refreshToken, err := auth.GenerateRefreshToken(user.ID.String(), familyID.String(), testutil.TestJWTSecretRefresh)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	refreshTokenRepo.On("RevokeFamily", mock.Anything, familyID.String()).Return(nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	refreshTokenRepo.AssertExpectations(t)
}

// Note: passes even pre-implementation — Logout has always cleared the
// cookie unconditionally. Kept as an explicit AC, not assumed coverage.
func TestLogout_ClearsCookieEvenWithoutValidRefreshToken(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: "not.a.valid.token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refreshToken" {
			refreshCookie = c
		}
	}
	require.NotNil(t, refreshCookie)
	assert.True(t, refreshCookie.MaxAge < 0, "expected refreshToken cookie to be expired even when the presented token is invalid")

	refreshTokenRepo.AssertExpectations(t)
}

// --- Me tests ---

func TestMe_Success(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/me", nil, testutil.AuthHeader(t, user.Email, user.ID.String()))

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, user.ID.String(), body["id"])
	assert.Equal(t, user.Email, body["email"])
	assert.Equal(t, user.DisplayName, body["name"])
	assert.Equal(t, user.AvatarURL, body["avatarUrl"])

	userRepo.AssertExpectations(t)
}

func TestMe_UserNotFound(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(nil, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/me", nil, testutil.AuthHeader(t, user.Email, user.ID.String()))

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "user not found", body["error"])

	userRepo.AssertExpectations(t)
}

func TestMe_UserLookupError(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}
	user := testUser()
	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(nil, errors.New("db error"))

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/me", nil, testutil.AuthHeader(t, user.Email, user.ID.String()))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	userRepo.AssertExpectations(t)
}

func TestMe_Unauthorized(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}
	refreshTokenRepo := &MockRefreshTokenRepository{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, refreshTokenRepo, testConfig("development"))
	r := newTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/me", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	userRepo.AssertExpectations(t)
}
