package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

const (
	testPackingListUserID     = "55555555-5555-5555-5555-555555555555"
	testPackingListTemplateID = "66666666-6666-6666-6666-666666666666"
	testPackingListItemID     = "77777777-7777-7777-7777-777777777777"
	testPackingListCategoryID = "88888888-8888-8888-8888-888888888888"
)

// testPackingListTemplateIDCopy exists so mock.On can take its address —
// the const itself isn't addressable.
var testPackingListTemplateIDCopy = testPackingListTemplateID

// --- Mock ---

type MockPackingListRepository struct {
	mock.Mock
}

func (m *MockPackingListRepository) CreatePackingList(ctx context.Context, userID, name string, eventDate *string, templateID *string) (*models.PackingList, error) {
	args := m.Called(ctx, userID, name, eventDate, templateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackingList), args.Error(1)
}

// --- Helpers ---

func newPackingListTestRouter(repo handler.PackingListRepository, templateRepo handler.TemplateLookupRepository) *gin.Engine {
	h := handler.NewPackingListHandler(repo, templateRepo)
	r := gin.New()
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware())
	authed.POST("/lists", h.Create)
	return r
}

func packingListOwnedTemplate() *models.Template {
	return &models.Template{
		ID:     uuid.MustParse(testPackingListTemplateID),
		Name:   "My Template",
		UserID: uuid.MustParse(testPackingListUserID),
		Items:  []models.TemplateItem{},
	}
}

func packingListResult(items []models.PackingListItem) *models.PackingList {
	if items == nil {
		items = []models.PackingListItem{}
	}
	return &models.PackingList{
		ID:     uuid.New(),
		Name:   "My List",
		UserID: uuid.MustParse(testPackingListUserID),
		Items:  items,
	}
}

// --- POST /lists ---

func TestPackingListCreate_Valid(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	created := packingListResult(nil)
	repo.On("CreatePackingList", mock.Anything, testPackingListUserID, "My List", (*string)(nil), (*string)(nil)).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "My List", body["name"])
	assert.Equal(t, []any{}, body["items"])
	assert.Nil(t, body["eventDate"])
	assert.Nil(t, body["templateId"])
	repo.AssertExpectations(t)
}

func TestPackingListCreate_WithEventDate(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	eventDate := "2026-08-01"
	created := packingListResult(nil)
	created.EventDate = &eventDate
	repo.On("CreatePackingList", mock.Anything, testPackingListUserID, "My List", &eventDate, (*string)(nil)).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "eventDate": eventDate}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, eventDate, body["eventDate"])
	repo.AssertExpectations(t)
}

func TestPackingListCreate_WithTemplateId(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	copiedItems := []models.PackingListItem{
		{ItemID: uuid.MustParse(testPackingListItemID), Name: "Test Item", CategoryID: uuid.MustParse(testPackingListCategoryID), Quantity: 1, IsPacked: false},
	}
	created := packingListResult(copiedItems)
	templateIDPtr := uuid.MustParse(testPackingListTemplateID)
	created.TemplateID = &templateIDPtr

	templateRepo.On("GetTemplateByID", mock.Anything, testPackingListTemplateID).Return(packingListOwnedTemplate(), nil)
	repo.On("CreatePackingList", mock.Anything, testPackingListUserID, "My List", (*string)(nil), &testPackingListTemplateIDCopy).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": testPackingListTemplateID}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	items, ok := body["items"].([]any)
	assert.True(t, ok)
	assert.Len(t, items, 1)
	repo.AssertExpectations(t)
	templateRepo.AssertExpectations(t)
}

func TestPackingListCreate_MissingName(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_EmptyName(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": ""}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_NameTooLong(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo)

	longName := strings.Repeat("a", 101)
	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": longName}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_InvalidEventDateFormat(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "eventDate": "08-01-2026"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_InvalidTemplateIdUUID(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": "not-a-uuid"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_TemplateNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	templateRepo.On("GetTemplateByID", mock.Anything, testPackingListTemplateID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": testPackingListTemplateID}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	templateRepo.AssertExpectations(t)
}

func TestPackingListCreate_TemplateNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	notMine := &models.Template{
		ID:     uuid.MustParse(testPackingListTemplateID),
		Name:   "Not Mine",
		UserID: uuid.MustParse(otherUserID),
		Items:  []models.TemplateItem{},
	}
	templateRepo.On("GetTemplateByID", mock.Anything, testPackingListTemplateID).Return(notMine, nil)

	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": testPackingListTemplateID}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	templateRepo.AssertExpectations(t)
}

func TestPackingListCreate_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo)

	req := httptest.NewRequest(http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
