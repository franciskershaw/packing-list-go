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

func newTemplateTestRouter(repo handler.TemplateRepository, itemRepo handler.ItemLookupRepository) *gin.Engine {
	h := handler.NewTemplateHandler(repo, itemRepo)
	r := gin.New()
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware(testutil.TestJWTSecretAccess))
	authed.GET("/templates", h.List)
	authed.POST("/templates", h.Create)
	authed.GET("/templates/:id", h.GetByID)
	authed.PATCH("/templates/:id", h.Update)
	authed.DELETE("/templates/:id", h.Delete)
	authed.POST("/templates/:id/items", h.AddItem)
	authed.PATCH("/templates/:id/items/:itemId", h.UpdateItem)
	authed.DELETE("/templates/:id/items/:itemId", h.RemoveItem)
	authed.POST("/templates/:id/items/bulk", h.BulkAddItems)
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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates", nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates", nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
	repo.AssertExpectations(t)
}

func TestTemplateList_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- POST /templates ---

func TestTemplateCreate_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	created := ownedTemplate()
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "My Template", (*string)(nil)).Return(false, nil)
	repo.On("CreateTemplate", mock.Anything, testTemplateUserID, "My Template", (*string)(nil)).Return(created, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "My Template"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "My Template", "description": desc}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, desc, body["description"])
	repo.AssertExpectations(t)
}

func TestTemplateCreate_MissingName(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateCreate_NameTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	longName := strings.Repeat("a", 101)
	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": longName}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateCreate_DescriptionTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	longDesc := strings.Repeat("a", 501)
	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "Valid Name", "description": longDesc}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateCreate_DuplicateName(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "Existing", (*string)(nil)).Return(true, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "Existing"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateCreate_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/templates", jsonBody(t, map[string]string{"name": "Test"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GET /templates/:id ---

func TestTemplateGetByID_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates/"+testTemplateID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "My Template", body["name"])
	assert.Equal(t, []any{}, body["items"])
	repo.AssertExpectations(t)
}

func TestTemplateGetByID_InvalidUUID(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates/not-a-uuid", nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateGetByID_NotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates/"+testTemplateID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates/"+testTemplateID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateGetByID_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/templates/"+testTemplateID, nil, "")

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"description": newDesc}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_NeitherFieldProvided(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_NameTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	longName := strings.Repeat("a", 101)
	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": longName}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_DescriptionTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	longDesc := strings.Repeat("a", 501)
	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"description": longDesc}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_NotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_DuplicateName(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	excludeID := testTemplateID

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateNameExistsForUser", mock.Anything, testTemplateUserID, "Taken Name", &excludeID).Return(true, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "Taken Name"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateUpdate_InvalidUUID(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/not-a-uuid", jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateUpdate_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID, jsonBody(t, map[string]string{"name": "New Name"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- DELETE /templates/:id ---

func TestTemplateDelete_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("DeleteTemplate", mock.Anything, testTemplateID).Return(nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateDelete_NotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

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

	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateDelete_InvalidUUID(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/templates/not-a-uuid", nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateDelete_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	r := newTemplateTestRouter(repo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
