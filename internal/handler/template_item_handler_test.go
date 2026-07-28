package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/franciskershaw/packing-list-go/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testTemplateItemItemID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	testTemplateItemItem2  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	testBulkCategoryID     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
)

// --- MockTemplateRepository: template-items methods ---
// (MockTemplateRepository itself is declared in template_handler_test.go;
// a Go type's methods can span files in the same package.)

func (m *MockTemplateRepository) AddTemplateItem(ctx context.Context, templateID, itemID string, quantity int, notes *string) (*models.TemplateItem, error) {
	args := m.Called(ctx, templateID, itemID, quantity, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TemplateItem), args.Error(1)
}

func (m *MockTemplateRepository) UpdateTemplateItem(ctx context.Context, templateID, itemID string, quantity *int, notes *string) (*models.TemplateItem, error) {
	args := m.Called(ctx, templateID, itemID, quantity, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TemplateItem), args.Error(1)
}

func (m *MockTemplateRepository) BulkUpdateTemplateItems(ctx context.Context, templateID string, changes map[string]int) error {
	args := m.Called(ctx, templateID, changes)
	return args.Error(0)
}

func (m *MockTemplateRepository) RemoveTemplateItem(ctx context.Context, templateID, itemID string) error {
	args := m.Called(ctx, templateID, itemID)
	return args.Error(0)
}

func (m *MockTemplateRepository) TemplateItemExists(ctx context.Context, templateID, itemID string) (bool, error) {
	args := m.Called(ctx, templateID, itemID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTemplateRepository) GetTemplateItems(ctx context.Context, templateID string) ([]models.TemplateItem, error) {
	args := m.Called(ctx, templateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.TemplateItem), args.Error(1)
}

// --- Helpers ---

func tmplAccessibleItem(id string) *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(id),
		Name:       "Test Item",
		CategoryID: uuid.MustParse(testBulkCategoryID),
		UserID:     uuidPtr(uuid.MustParse(testTemplateUserID)),
	}
}

func tmplSystemItem(id string) *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(id),
		Name:       "System Item",
		CategoryID: uuid.MustParse(testBulkCategoryID),
		UserID:     nil,
		IsSystem:   true,
	}
}

func tmplOtherUsersItem(id string) *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(id),
		Name:       "Not Mine",
		CategoryID: uuid.MustParse(testBulkCategoryID),
		UserID:     uuidPtr(uuid.MustParse(otherUserID)),
	}
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

func templateItem(itemID string, quantity int, notes *string) *models.TemplateItem {
	return &models.TemplateItem{
		ItemID:   uuid.MustParse(itemID),
		Name:     "Test Item",
		Quantity: quantity,
		Notes:    notes,
	}
}

// --- POST /templates/:id/items ---

func TestTemplateItemAdd_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	created := templateItem(testTemplateItemItemID, 3, nil)

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(false, nil)
	repo.On("AddTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID, 3, (*string)(nil)).Return(created, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID, "quantity": 3}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(3), body["quantity"])
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestTemplateItemAdd_DefaultQuantity(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	created := templateItem(testTemplateItemItemID, 1, nil)

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(false, nil)
	repo.On("AddTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID, 1, (*string)(nil)).Return(created, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemAdd_SystemItemAccessible(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	created := templateItem(testTemplateItemItemID, 1, nil)

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplSystemItem(testTemplateItemItemID), nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(false, nil)
	repo.On("AddTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID, 1, (*string)(nil)).Return(created, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemAdd_WithNotes(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	notes := "two pairs"
	created := templateItem(testTemplateItemItemID, 1, &notes)

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(false, nil)
	repo.On("AddTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID, 1, &notes).Return(created, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID, "notes": notes}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemAdd_MissingItemID(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_InvalidItemID(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": "not-a-uuid"}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_InaccessibleItem(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplOtherUsersItem(testTemplateItemItemID), nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestTemplateItemAdd_ItemDoesNotExist(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(nil, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_QuantityTooLow(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID, "quantity": 0}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_QuantityTooHigh(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID, "quantity": 1000}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_NotesTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)

	r := newTemplateTestRouter(repo, itemRepo)

	longNotes := strings.Repeat("a", 201)
	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID, "notes": longNotes}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_Duplicate(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemByID", mock.Anything, testTemplateItemItemID).Return(tmplAccessibleItem(testTemplateItemItemID), nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(true, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemAdd_TemplateNotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemAdd_OtherUsersTemplate(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := &models.Template{ID: uuid.MustParse(testTemplateID), Name: "Not Mine", UserID: uuid.MustParse(otherUserID), Items: []models.TemplateItem{}}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemAdd_InvalidTemplateID(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/not-a-uuid/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemAdd_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/templates/"+testTemplateID+"/items", jsonBody(t, map[string]any{"itemId": testTemplateItemItemID}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /templates/:id/items/:itemId ---

func TestTemplateItemUpdate_QuantityOnly(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	updated := templateItem(testTemplateItemItemID, 5, nil)

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(true, nil)
	repo.On("UpdateTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID, mock.MatchedBy(func(q *int) bool { return q != nil && *q == 5 }), (*string)(nil)).Return(updated, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"quantity": 5}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemUpdate_NotesOnly(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	newNotes := "updated notes"
	updated := templateItem(testTemplateItemItemID, 1, &newNotes)

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(true, nil)
	repo.On("UpdateTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID, (*int)(nil), &newNotes).Return(updated, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"notes": newNotes}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemUpdate_NeitherFieldProvided(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemUpdate_QuantityOutOfRange(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(true, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"quantity": 1000}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemUpdate_NotesTooLong(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(true, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	longNotes := strings.Repeat("a", 201)
	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"notes": longNotes}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemUpdate_TemplateNotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"quantity": 2}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemUpdate_ItemNotOnTemplate(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(false, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"quantity": 2}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemUpdate_InvalidItemID(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/not-a-uuid", jsonBody(t, map[string]any{"quantity": 2}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemUpdate_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, jsonBody(t, map[string]any{"quantity": 2}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- DELETE /templates/:id/items/:itemId ---

func TestTemplateItemRemove_Valid(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(true, nil)
	repo.On("RemoveTemplateItem", mock.Anything, testTemplateID, testTemplateItemItemID).Return(nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemRemove_TemplateNotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemRemove_ItemNotOnTemplate(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	repo.On("TemplateItemExists", mock.Anything, testTemplateID, testTemplateItemItemID).Return(false, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemRemove_InvalidItemID(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID+"/items/not-a-uuid", nil, testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemRemove_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/templates/"+testTemplateID+"/items/"+testTemplateItemItemID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /templates/:id/items/bulk ---
// Delta contract (PACK-035): quantity 0 removes, any other quantity in
// [0, 999] adds-if-absent-or-updates-if-present. Existence branching
// happens entirely inside BulkUpdateTemplateItems (repo layer, see
// TestBulkUpdateTemplateItems_AddsUpdatesAndRemoves) — the handler only
// validates the request shape and item accessibility, then forwards the
// whole changes map in one call. Mirrors packing_list_item_handler_test.go's
// equivalent block; no "NotOwned" case here, matching this file's existing
// precedent of only testing TemplateNotFound (isTemplateOwned folds into
// the same 404 as not-found).

func TestTemplateItemBulkUpdate_MixedBatchSucceeds(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	batchItems := []models.Item{*tmplAccessibleItem(testTemplateItemItemID), *tmplAccessibleItem(testTemplateItemItem2)}

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testTemplateItemItemID, testTemplateItemItem2}).Return(batchItems, nil)
	repo.On("BulkUpdateTemplateItems", mock.Anything, testTemplateID, map[string]int{testTemplateItemItemID: 5, testTemplateItemItem2: 0}).Return(nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{
		{"itemId": testTemplateItemItemID, "quantity": 5},
		{"itemId": testTemplateItemItem2, "quantity": 0},
	}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestTemplateItemBulkUpdate_NoopRemoveOfAbsentItem(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	batchItems := []models.Item{*tmplAccessibleItem(testTemplateItemItemID)}

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testTemplateItemItemID}).Return(batchItems, nil)
	repo.On("BulkUpdateTemplateItems", mock.Anything, testTemplateID, map[string]int{testTemplateItemItemID: 0}).Return(nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 0}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNoContent, w.Code, "quantity 0 must pass handler validation, not be rejected as out of range")
	repo.AssertExpectations(t)
}

func TestTemplateItemBulkUpdate_EmptyArray(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemBulkUpdate_DuplicateItemId(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{
		{"itemId": testTemplateItemItemID, "quantity": 1},
		{"itemId": testTemplateItemItemID, "quantity": 2},
	}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemBulkUpdate_InvalidItemId(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": "not-a-uuid", "quantity": 1}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemBulkUpdate_QuantityTooLow(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": -1}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemBulkUpdate_QuantityTooHigh(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 1000}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTemplateItemBulkUpdate_InaccessibleItem(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	batchItems := []models.Item{*tmplOtherUsersItem(testTemplateItemItemID)}

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testTemplateItemItemID}).Return(batchItems, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 1}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	itemRepo.AssertExpectations(t)
}

func TestTemplateItemBulkUpdate_UnknownItemId(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testTemplateItemItemID}).Return([]models.Item{}, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 1}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	itemRepo.AssertExpectations(t)
}

func TestTemplateItemBulkUpdate_RepoErrorReturns500(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	tmpl := ownedTemplate()
	batchItems := []models.Item{*tmplAccessibleItem(testTemplateItemItemID)}

	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(tmpl, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testTemplateItemItemID}).Return(batchItems, nil)
	repo.On("BulkUpdateTemplateItems", mock.Anything, testTemplateID, map[string]int{testTemplateItemItemID: 3}).Return(assert.AnError)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 3}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemBulkUpdate_TemplateNotFound(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	repo.On("GetTemplateByID", mock.Anything, testTemplateID).Return(nil, nil)

	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 1}}}), testutil.AuthHeader(t, "test@example.com", testTemplateUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestTemplateItemBulkUpdate_Unauthorized(t *testing.T) {
	repo := &MockTemplateRepository{}
	itemRepo := &MockItemRepository{}
	r := newTemplateTestRouter(repo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/templates/"+testTemplateID+"/items/bulk", jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testTemplateItemItemID, "quantity": 1}}}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
