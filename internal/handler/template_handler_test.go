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

// JWT env vars and gin.TestMode are set in auth_handler_test.go init().

const (
	testTemplateUserID = "33333333-3333-3333-3333-333333333333"
	testTemplateID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

// --- Mock ---

type MockTemplateRepository struct {
	mock.Mock
}

func (m *MockTemplateRepository) GetTemplates(ctx context.Context, userID string) ([]models.Template, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Template), args.Error(1)
}

func (m *MockTemplateRepository) GetTemplateByID(ctx context.Context, id string) (*models.Template, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Template), args.Error(1)
}

func (m *MockTemplateRepository) CreateTemplate(ctx context.Context, userID, name string, description *string) (*models.Template, error) {
	args := m.Called(ctx, userID, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Template), args.Error(1)
}

func (m *MockTemplateRepository) UpdateTemplate(ctx context.Context, id string, name *string, description *string) (*models.Template, error) {
	args := m.Called(ctx, id, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Template), args.Error(1)
}

func (m *MockTemplateRepository) DeleteTemplate(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTemplateRepository) TemplateNameExistsForUser(ctx context.Context, userID, name string, excludeID *string) (bool, error) {
	args := m.Called(ctx, userID, name, excludeID)
	return args.Bool(0), args.Error(1)
}

// --- Helpers ---

func newTemplateTestRouter(h *handler.TemplateHandler) *gin.Engine {
	r := gin.New()
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware())
	authed.GET("/templates", h.List)
	authed.POST("/templates", h.Create)
	authed.GET("/templates/:id", h.GetByID)
	authed.PATCH("/templates/:id", h.Update)
	authed.DELETE("/templates/:id", h.Delete)
	return r
}

func ownedTemplate() *models.Template {
	return &models.Template{
		ID:     uuid.MustParse(testTemplateID),
		Name:   "My Template",
		UserID: uuid.MustParse(testTemplateUserID),
		Items:  []models.TemplateItem{},
	}
}

// --- GET /templates ---

func TestTemplateList_HappyPath(t *testing.T) {
	repo := &MockTemplateRepository{}
	templates := []models.Template{*ownedTemplate()}
	repo.On("GetTemplates", mock.Anything, testTemplateUserID).Return(templates, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 1)
	assert.Equal(t, "My Template", body[0]["name"])
	assert.Equal(t, []any{}, body[0]["items"])
	repo.AssertExpectations(t)
}

func TestTemplateList_EmptyResult(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplates", mock.Anything, testTemplateUserID).Return([]models.Template{}, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
	repo.AssertExpectations(t)
}

func TestTemplateList_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- POST /templates ---

func TestTemplateCreate_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	created := ownedTemplate()
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "My Template", (*string)(nil)).Return(false, nil)
	repo.On("CreateTemplate", mock.Anything, testTemplateUserID, "My Template", (*string)(nil)).Return(created, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "My Template"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "My Template", body["name"])
	repo.AssertExpectations(t)
}

func TestTemplateCreate_WithDescription(t *testing.T) {
	repo := &MockTemplateRepository{}
	desc := "A description"
	created := ownedTemplate()
	created.Description = &desc
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "My Template", (*string)(nil)).Return(false, nil)
	repo.On("CreateTemplate", mock.Anything, testTemplateUserID, "My Template", &desc).Return(created, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "My Template", "description": desc}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, desc, body["description"])
	repo.AssertExpectations(t)
}

func TestTemplateCreate_MissingName(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]any{}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateCreate_NameTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	longName := strings.Repeat("a", 101)
	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": longName}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateCreate_DescriptionTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	longDesc := strings.Repeat("a", 501)
	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "Valid Name", "description": longDesc}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateCreate_DuplicateName(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "Existing", (*string)(nil)).Return(true, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "Existing"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateCreate_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "Test"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GET /templates/:id ---

func TestTemplateGetByID_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates/"+testTemplateID, nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "My Template", body["name"])
	assert.Equal(t, []any{}, body["items"])
	repo.AssertExpectations(t)
}

func TestTemplateGetByID_InvalidUUID(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates/not-a-uuid", nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateGetByID_NotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates/"+testTemplateID, nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateGetByID_OtherUsersTemplate(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := &models.Template{
		ID:     uuid.MustParse(testTemplateID),
		Name:   "Not Mine",
		UserID: uuid.MustParse(otherUserID),
		Items:  []models.TemplateItem{},
	}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates/"+testTemplateID, nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateGetByID_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/templates/"+testTemplateID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /templates/:id ---

func TestTemplateUpdate_NameOnly(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	updated := ownedTemplate()
	updated.Name = "New Name"
	excludeID := testTemplateID

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "New Name", &excludeID).Return(false, nil)
	repo.On("UpdateTemplate", mock.Anything, testTemplateID, mock.MatchedBy(func(n *string) bool { return n != nil && *n == "New Name" }), (*string)(nil)).Return(updated, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "New Name", body["name"])
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_DescriptionOnly_NoNameCheck(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	newDesc := "New description"
	updated := ownedTemplate()
	updated.Description = &newDesc

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	// No TemplateNameExistsForUser expectation set: since name isn't being
	// changed, the handler must not call it. repo.AssertExpectations would
	// still pass even if it were called incidentally, so this test also
	// relies on mock.Mock panicking on an unexpected call.
	repo.On("UpdateTemplate", mock.Anything, testTemplateID, (*string)(nil), mock.MatchedBy(func(d *string) bool { return d != nil && *d == newDesc })).Return(updated, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"description": newDesc}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_NeitherFieldProvided(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]any{}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_NameTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	longName := strings.Repeat("a", 101)
	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": longName}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_DescriptionTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	longDesc := strings.Repeat("a", 501)
	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"description": longDesc}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_NotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_OtherUsersTemplate(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := &models.Template{
		ID:     uuid.MustParse(testTemplateID),
		Name:   "Not Mine",
		UserID: uuid.MustParse(otherUserID),
		Items:  []models.TemplateItem{},
	}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_DuplicateName(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	excludeID := testTemplateID

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "Taken Name", &excludeID).Return(true, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "Taken Name"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_InvalidUUID(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/not-a-uuid", jsonBody(t, map[string]string{"name": "New Name"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- DELETE /templates/:id ---

func TestTemplateDelete_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("DeleteTemplate", mock.Anything, testTemplateID).Return(nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/templates/"+testTemplateID, nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateDelete_NotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/templates/"+testTemplateID, nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateDelete_OtherUsersTemplate(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := &models.Template{
		ID:     uuid.MustParse(testTemplateID),
		Name:   "Not Mine",
		UserID: uuid.MustParse(otherUserID),
		Items:  []models.TemplateItem{},
	}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/templates/"+testTemplateID, nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateDelete_InvalidUUID(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/templates/not-a-uuid", nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, "test@example.com", testTemplateUserID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateDelete_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	h := handler.NewTemplateHandler(repo)
	r := newTemplateTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/templates/"+testTemplateID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
