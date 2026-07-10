package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/handler"
	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/franciskershaw/packing-list-go/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// JWT env vars and gin.TestMode are set in auth_handler_test.go init().

const (
	testCategoryUserID = "11111111-1111-1111-1111-111111111111"
	testCategoryID     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

// --- Mock ---

type MockCategoryRepository struct {
	mock.Mock
}

func (m *MockCategoryRepository) GetCategories(ctx context.Context, userID string) ([]models.Category, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Category), args.Error(1)
}

func (m *MockCategoryRepository) CreateCategory(ctx context.Context, userID, name string) (*models.Category, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Category), args.Error(1)
}

func (m *MockCategoryRepository) GetCategoryByID(ctx context.Context, id string) (*models.Category, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Category), args.Error(1)
}

func (m *MockCategoryRepository) UpdateCategory(ctx context.Context, id, name string) (*models.Category, error) {
	args := m.Called(ctx, id, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Category), args.Error(1)
}

func (m *MockCategoryRepository) DeleteCategory(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCategoryRepository) CategoryNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error) {
	args := m.Called(ctx, userID, name, excludeID)
	return args.Bool(0), args.Error(1)
}

func (m *MockCategoryRepository) CategoryHasItems(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

// --- Helpers ---

func newCategoryTestRouter(h *handler.CategoryHandler) *gin.Engine {
	r := gin.New()
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware(testutil.TestJWTSecretAccess))
	authed.GET("/categories", h.List)
	authed.POST("/categories", h.Create)
	authed.PATCH("/categories/:id", h.Update)
	authed.DELETE("/categories/:id", h.Delete)
	return r
}

func ownedCategory() *models.Category {
	uid := uuid.MustParse(testCategoryUserID)
	return &models.Category{
		ID:       uuid.MustParse(testCategoryID),
		Name:     "My Category",
		IsSystem: false,
		UserID:   &uid,
	}
}

func systemCategory() *models.Category {
	return &models.Category{
		ID:       uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Name:     "Toiletries",
		IsSystem: true,
		UserID:   nil,
	}
}

// --- GET /categories ---

func TestList_HappyPath(t *testing.T) {
	repo := &MockCategoryRepository{}
	uid := uuid.MustParse(testCategoryUserID)
	categories := []models.Category{
		{ID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), Name: "Toiletries", IsSystem: true, UserID: nil},
		{ID: uuid.MustParse(testCategoryID), Name: "My Category", IsSystem: false, UserID: &uid},
	}
	repo.On("GetCategories", mock.Anything, testCategoryUserID).Return(categories, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/categories", nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusOK, w.Code)

	var body []map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 2)
	assert.Equal(t, true, body[0]["isSystem"])
	assert.Equal(t, false, body[1]["isSystem"])
	repo.AssertExpectations(t)
}

func TestList_EmptyResult(t *testing.T) {
	repo := &MockCategoryRepository{}
	repo.On("GetCategories", mock.Anything, testCategoryUserID).Return([]models.Category{}, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/categories", nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
	repo.AssertExpectations(t)
}

func TestList_Unauthorized(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/categories", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- POST /categories ---

func TestCreate_Valid(t *testing.T) {
	repo := &MockCategoryRepository{}
	created := ownedCategory()
	repo.On("CategoryNameExistsForUser", mock.Anything, testCategoryUserID, "My Category", (*string)(nil)).Return(false, nil)
	repo.On("CreateCategory", mock.Anything, testCategoryUserID, "My Category").Return(created, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]string{"name": "My Category"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusCreated, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "My Category", body["name"])
	assert.Equal(t, false, body["isSystem"])
	repo.AssertExpectations(t)
}

func TestCreate_MissingName(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreate_MalformedJSON is the representative regression test for the
// shared bindJSON helper (internal/handler/validation.go) — no test
// anywhere in the handler package covered a malformed-JSON-body request
// before PACK-018 introduced bindJSON to replace 13 duplicated
// ShouldBindJSON-then-400 blocks across 6 files. Not duplicated across
// the other 12 call sites since the logic is now provably identical.
func TestCreate_MalformedJSON(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", strings.NewReader(`{"name":`), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "invalid request body", body["error"])
}

func TestCreate_EmptyName(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]string{"name": ""}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreate_NameTooLong(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	longName := strings.Repeat("a", 101)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]string{"name": longName}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreate_DuplicateName(t *testing.T) {
	repo := &MockCategoryRepository{}
	repo.On("CategoryNameExistsForUser", mock.Anything, testCategoryUserID, "Existing", (*string)(nil)).Return(true, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]string{"name": "Existing"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestCreate_SystemCategoryNameAllowed(t *testing.T) {
	repo := &MockCategoryRepository{}
	created := &models.Category{
		ID:       uuid.MustParse(testCategoryID),
		Name:     "Toiletries",
		IsSystem: false,
		UserID:   func() *uuid.UUID { u := uuid.MustParse(testCategoryUserID); return &u }(),
	}
	// System category names don't block creation — only user-owned duplicates do
	repo.On("CategoryNameExistsForUser", mock.Anything, testCategoryUserID, "Toiletries", (*string)(nil)).Return(false, nil)
	repo.On("CreateCategory", mock.Anything, testCategoryUserID, "Toiletries").Return(created, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]string{"name": "Toiletries"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestCreate_Unauthorized(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/categories", jsonBody(t, map[string]string{"name": "Test"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /categories/:id ---

func TestUpdate_Valid(t *testing.T) {
	repo := &MockCategoryRepository{}
	cat := ownedCategory()
	updated := &models.Category{
		ID:       cat.ID,
		Name:     "New Name",
		IsSystem: false,
		UserID:   cat.UserID,
	}
	excludeID := testCategoryID

	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)
	repo.On("CategoryNameExistsForUser", mock.Anything, testCategoryUserID, "New Name", &excludeID).Return(false, nil)
	repo.On("UpdateCategory", mock.Anything, testCategoryID, "New Name").Return(updated, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "New Name", body["name"])
	repo.AssertExpectations(t)
}

func TestUpdate_OtherUsersCategory(t *testing.T) {
	repo := &MockCategoryRepository{}
	other := uuid.MustParse(otherUserID)
	cat := &models.Category{
		ID:       uuid.MustParse(testCategoryID),
		Name:     "Someone Else's",
		IsSystem: false,
		UserID:   &other,
	}
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestUpdate_SystemCategory(t *testing.T) {
	repo := &MockCategoryRepository{}
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(systemCategory(), nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestUpdate_NotFound(t *testing.T) {
	repo := &MockCategoryRepository{}
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(nil, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestUpdate_DuplicateName(t *testing.T) {
	repo := &MockCategoryRepository{}
	cat := ownedCategory()
	excludeID := testCategoryID

	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)
	repo.On("CategoryNameExistsForUser", mock.Anything, testCategoryUserID, "Taken Name", &excludeID).Return(true, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "Taken Name"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestUpdate_SameNameIsOK(t *testing.T) {
	repo := &MockCategoryRepository{}
	cat := ownedCategory() // Name is "My Category"
	excludeID := testCategoryID
	updated := ownedCategory()

	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)
	// excludeID means the current category is excluded → no conflict found
	repo.On("CategoryNameExistsForUser", mock.Anything, testCategoryUserID, "My Category", &excludeID).Return(false, nil)
	repo.On("UpdateCategory", mock.Anything, testCategoryID, "My Category").Return(updated, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "My Category"}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestUpdate_InvalidName(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": ""}), testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdate_Unauthorized(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/categories/"+testCategoryID, jsonBody(t, map[string]string{"name": "New Name"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- DELETE /categories/:id ---

func TestDelete_Valid(t *testing.T) {
	repo := &MockCategoryRepository{}
	cat := ownedCategory()
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)
	repo.On("CategoryHasItems", mock.Anything, testCategoryID).Return(false, nil)
	repo.On("DeleteCategory", mock.Anything, testCategoryID).Return(nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/categories/"+testCategoryID, nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestDelete_HasItems(t *testing.T) {
	repo := &MockCategoryRepository{}
	cat := ownedCategory()
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)
	repo.On("CategoryHasItems", mock.Anything, testCategoryID).Return(true, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/categories/"+testCategoryID, nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestDelete_OtherUsersCategory(t *testing.T) {
	repo := &MockCategoryRepository{}
	other := uuid.MustParse(otherUserID)
	cat := &models.Category{
		ID:       uuid.MustParse(testCategoryID),
		Name:     "Not Mine",
		IsSystem: false,
		UserID:   &other,
	}
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(cat, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/categories/"+testCategoryID, nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestDelete_SystemCategory(t *testing.T) {
	repo := &MockCategoryRepository{}
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(systemCategory(), nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/categories/"+testCategoryID, nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestDelete_NotFound(t *testing.T) {
	repo := &MockCategoryRepository{}
	repo.On("GetCategoryByID", mock.Anything, testCategoryID).Return(nil, nil)

	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/categories/"+testCategoryID, nil, testutil.AuthHeader(t, "test@example.com", testCategoryUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestDelete_Unauthorized(t *testing.T) {
	repo := &MockCategoryRepository{}
	h := handler.NewCategoryHandler(repo)
	r := newCategoryTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/categories/"+testCategoryID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
