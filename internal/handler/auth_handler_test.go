package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/handler"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

func init() {
	os.Setenv("JWT_SECRET_ACCESS", "test-secret-access")
	os.Setenv("JWT_SECRET_REFRESH", "test-secret-refresh")
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

// --- Helpers ---

func newTestRouter(h *handler.AuthHandler) *gin.Engine {
	r := gin.New()
	r.GET("/auth/google/callback", h.GoogleCallback)
	r.POST("/auth/refresh", h.RefreshToken)
	r.POST("/auth/logout", h.Logout)
	return r
}

// testConfig builds a minimal *config.Config for handler construction.
func testConfig(environment string) *config.Config {
	return &config.Config{Environment: environment}
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

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code&state=valid-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.NotEmpty(t, body["accessToken"])

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
}

func TestGoogleCallback_HappyPath_SecureCookieInProduction(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}

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

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("production"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=auth-code&state=valid-state", nil)
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
	assert.NotNil(t, refreshCookie, "expected refreshToken cookie to be set")
	assert.True(t, refreshCookie.Secure, "expected Secure=true in production")
	assert.Equal(t, http.SameSiteLaxMode, refreshCookie.SameSite)

	oauthMgr.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestGoogleCallback_InvalidState(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}

	oauthMgr.On("ValidateState", "bad-state").Return(false)

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
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

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=some-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGoogleCallback_MissingState(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
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
	user := testUser()

	refreshToken, err := auth.GenerateRefreshToken(user.ID.String())
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	userRepo.On("GetUserByID", mock.Anything, user.ID.String()).Return(user, nil)

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
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
}

func TestRefreshToken_MissingCookie(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: "not.a.valid.token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Logout tests ---

func TestLogout_ClearsCookie(t *testing.T) {
	userRepo := &MockUserRepository{}
	oauthMgr := &MockOAuthManager{}

	h := handler.NewAuthHandler(userRepo, oauthMgr, testConfig("development"))
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

