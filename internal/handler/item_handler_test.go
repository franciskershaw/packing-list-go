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
	testItemUserID       = "11111111-1111-1111-1111-111111111111"
	testItemID           = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	testItemCategoryID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testSystemCategoryID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	otherCategoryID      = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
)

// --- Mock ---

type MockItemRepository struct {
	mock.Mock
}

func (m *MockItemRepository) GetItems(ctx context.Context, userID string, categoryID *string, search *string) ([]models.Item, error) {
	args := m.Called(ctx, userID, categoryID, search)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Item), args.Error(1)
}

func (m *MockItemRepository) GetItemByID(ctx context.Context, id string) (*models.Item, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Item), args.Error(1)
}

func (m *MockItemRepository) CreateItem(ctx context.Context, userID, name, categoryID string) (*models.Item, error) {
	args := m.Called(ctx, userID, name, categoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Item), args.Error(1)
}

func (m *MockItemRepository) UpdateItem(ctx context.Context, id string, name *string, categoryID *string) (*models.Item, error) {
	args := m.Called(ctx, id, name, categoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Item), args.Error(1)
}

func (m *MockItemRepository) DeleteItem(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockItemRepository) ItemNameExistsInCategory(ctx context.Context, categoryID, name string, excludeID *string) (bool, error) {
	args := m.Called(ctx, categoryID, name, excludeID)
	return args.Bool(0), args.Error(1)
}

func (m *MockItemRepository) ItemIsInUse(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockItemRepository) CategoryIsAccessible(ctx context.Context, categoryID, userID string) (bool, error) {
	args := m.Called(ctx, categoryID, userID)
	return args.Bool(0), args.Error(1)
}

// --- Helpers ---

func newItemTestRouter(h *handler.ItemHandler) *gin.Engine {
	r := gin.New()
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware(testutil.TestJWTSecretAccess))
	authed.GET("/items", h.List)
	authed.POST("/items", h.Create)
	authed.PATCH("/items/:id", h.Update)
	authed.DELETE("/items/:id", h.Delete)
	return r
}

func ownedItem() *models.Item {
	uid := uuid.MustParse(testItemUserID)
	return &models.Item{
		ID:         uuid.MustParse(testItemID),
		Name:       "Dry Shampoo",
		CategoryID: uuid.MustParse(testItemCategoryID),
		IsSystem:   false,
		UserID:     &uid,
	}
}

func systemItem() *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(testItemID),
		Name:       "Toothbrush",
		CategoryID: uuid.MustParse(testSystemCategoryID),
		IsSystem:   true,
		UserID:     nil,
	}
}

// --- GET /items ---

func TestItemsList_NoFilters(t *testing.T) {
	repo := &MockItemRepository{}
	uid := uuid.MustParse(testItemUserID)
	items := []models.Item{
		{ID: uuid.MustParse(testSystemCategoryID), Name: "Toothbrush", CategoryID: uuid.MustParse(testSystemCategoryID), IsSystem: true, UserID: nil},
		{ID: uuid.MustParse(testItemID), Name: "Dry Shampoo", CategoryID: uuid.MustParse(testItemCategoryID), IsSystem: false, UserID: &uid},
	}
	repo.On("GetItems", mock.Anything, testItemUserID, (*string)(nil), (*string)(nil)).Return(items, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items", nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 2)
	repo.AssertExpectations(t)
}

func TestItemsList_FilterByCategory(t *testing.T) {
	repo := &MockItemRepository{}
	categoryID := testItemCategoryID
	items := []models.Item{*ownedItem()}
	repo.On("CategoryIsAccessible", mock.Anything, categoryID, testItemUserID).Return(true, nil)
	repo.On("GetItems", mock.Anything, testItemUserID, &categoryID, (*string)(nil)).Return(items, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items?category_id="+categoryID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsList_FilterBySearch(t *testing.T) {
	repo := &MockItemRepository{}
	search := "sham"
	items := []models.Item{*ownedItem()}
	repo.On("GetItems", mock.Anything, testItemUserID, (*string)(nil), &search).Return(items, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items?search=sham", nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsList_BothFilters(t *testing.T) {
	repo := &MockItemRepository{}
	categoryID := testItemCategoryID
	search := "sham"
	items := []models.Item{*ownedItem()}
	repo.On("CategoryIsAccessible", mock.Anything, categoryID, testItemUserID).Return(true, nil)
	repo.On("GetItems", mock.Anything, testItemUserID, &categoryID, &search).Return(items, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items?category_id="+categoryID+"&search=sham", nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsList_InvalidCategoryUUID(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items?category_id=not-a-uuid", nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsList_InaccessibleCategory(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("CategoryIsAccessible", mock.Anything, otherCategoryID, testItemUserID).Return(false, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items?category_id="+otherCategoryID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsList_EmptyResult(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("GetItems", mock.Anything, testItemUserID, (*string)(nil), (*string)(nil)).Return([]models.Item{}, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items", nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
	repo.AssertExpectations(t)
}

func TestItemsList_Unauthorized(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodGet, "/items", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- POST /items ---

func TestItemsCreate_Valid(t *testing.T) {
	repo := &MockItemRepository{}
	created := ownedItem()
	repo.On("CategoryIsAccessible", mock.Anything, testItemCategoryID, testItemUserID).Return(true, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testItemCategoryID, "Dry Shampoo", (*string)(nil)).Return(false, nil)
	repo.On("CreateItem", mock.Anything, testItemUserID, "Dry Shampoo", testItemCategoryID).Return(created, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Dry Shampoo", "categoryId": testItemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Dry Shampoo", body["name"])
	assert.Equal(t, false, body["isSystem"])
	repo.AssertExpectations(t)
}

func TestItemsCreate_MissingName(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"categoryId": testItemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsCreate_EmptyName(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "", "categoryId": testItemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsCreate_NameTooLong(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	longName := strings.Repeat("a", 101)
	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": longName, "categoryId": testItemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsCreate_MissingCategoryID(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Test Item"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsCreate_InvalidCategoryUUID(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Test Item", "categoryId": "not-a-uuid"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsCreate_InaccessibleCategory(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("CategoryIsAccessible", mock.Anything, otherCategoryID, testItemUserID).Return(false, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Test Item", "categoryId": otherCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsCreate_DuplicateName(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("CategoryIsAccessible", mock.Anything, testItemCategoryID, testItemUserID).Return(true, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testItemCategoryID, "Existing", (*string)(nil)).Return(true, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Existing", "categoryId": testItemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsCreate_SameNameDifferentCategory(t *testing.T) {
	repo := &MockItemRepository{}
	created := ownedItem()
	repo.On("CategoryIsAccessible", mock.Anything, otherCategoryID, testItemUserID).Return(true, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, otherCategoryID, "Dry Shampoo", (*string)(nil)).Return(false, nil)
	repo.On("CreateItem", mock.Anything, testItemUserID, "Dry Shampoo", otherCategoryID).Return(created, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Dry Shampoo", "categoryId": otherCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsCreate_DuplicateSystemItemSameCategory(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("CategoryIsAccessible", mock.Anything, testSystemCategoryID, testItemUserID).Return(true, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testSystemCategoryID, "Toothbrush", (*string)(nil)).Return(true, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Toothbrush", "categoryId": testSystemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsCreate_Unauthorized(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPost, "/items", jsonBody(t, map[string]string{"name": "Test", "categoryId": testItemCategoryID}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /items/:id ---

func TestItemsUpdate_NameOnly(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	excludeID := testItemID
	updated := &models.Item{ID: item.ID, Name: "New Name", CategoryID: item.CategoryID, UserID: item.UserID}

	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testItemCategoryID, "New Name", &excludeID).Return(false, nil)
	repo.On("UpdateItem", mock.Anything, testItemID, mock.MatchedBy(func(n *string) bool { return n != nil && *n == "New Name" }), (*string)(nil)).Return(updated, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "New Name", body["name"])
	repo.AssertExpectations(t)
}

func TestItemsUpdate_CategoryOnly(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	excludeID := testItemID
	updated := &models.Item{ID: item.ID, Name: item.Name, CategoryID: uuid.MustParse(testSystemCategoryID), UserID: item.UserID}

	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("CategoryIsAccessible", mock.Anything, testSystemCategoryID, testItemUserID).Return(true, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testSystemCategoryID, item.Name, &excludeID).Return(false, nil)
	repo.On("UpdateItem", mock.Anything, testItemID, (*string)(nil), mock.MatchedBy(func(c *string) bool { return c != nil && *c == testSystemCategoryID })).Return(updated, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"categoryId": testSystemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_BothFields(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	excludeID := testItemID
	updated := &models.Item{ID: item.ID, Name: "New Name", CategoryID: uuid.MustParse(testSystemCategoryID), UserID: item.UserID}

	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("CategoryIsAccessible", mock.Anything, testSystemCategoryID, testItemUserID).Return(true, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testSystemCategoryID, "New Name", &excludeID).Return(false, nil)
	repo.On("UpdateItem", mock.Anything, testItemID,
		mock.MatchedBy(func(n *string) bool { return n != nil && *n == "New Name" }),
		mock.MatchedBy(func(c *string) bool { return c != nil && *c == testSystemCategoryID }),
	).Return(updated, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "New Name", "categoryId": testSystemCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_NoFieldsProvided(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsUpdate_NotOwned(t *testing.T) {
	repo := &MockItemRepository{}
	other := uuid.MustParse(otherUserID)
	item := &models.Item{ID: uuid.MustParse(testItemID), Name: "Not Mine", CategoryID: uuid.MustParse(testItemCategoryID), UserID: &other}
	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_SystemItem(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("GetItemByID", mock.Anything, testItemID).Return(systemItem(), nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_NotFound(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("GetItemByID", mock.Anything, testItemID).Return(nil, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_InvalidName(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": ""}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsUpdate_InvalidCategoryUUID(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"categoryId": "not-a-uuid"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsUpdate_InaccessibleCategory(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("CategoryIsAccessible", mock.Anything, otherCategoryID, testItemUserID).Return(false, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"categoryId": otherCategoryID}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_DuplicateName(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	excludeID := testItemID
	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("ItemNameExistsInCategory", mock.Anything, testItemCategoryID, "Taken Name", &excludeID).Return(true, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "Taken Name"}), testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsUpdate_Unauthorized(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodPatch, "/items/"+testItemID, jsonBody(t, map[string]string{"name": "New Name"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- DELETE /items/:id ---

func TestItemsDelete_Valid(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("ItemIsInUse", mock.Anything, testItemID).Return(false, nil)
	repo.On("DeleteItem", mock.Anything, testItemID).Return(nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/items/"+testItemID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsDelete_InUse(t *testing.T) {
	repo := &MockItemRepository{}
	item := ownedItem()
	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)
	repo.On("ItemIsInUse", mock.Anything, testItemID).Return(true, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/items/"+testItemID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsDelete_NotOwned(t *testing.T) {
	repo := &MockItemRepository{}
	other := uuid.MustParse(otherUserID)
	item := &models.Item{ID: uuid.MustParse(testItemID), Name: "Not Mine", CategoryID: uuid.MustParse(testItemCategoryID), UserID: &other}
	repo.On("GetItemByID", mock.Anything, testItemID).Return(item, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/items/"+testItemID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsDelete_SystemItem(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("GetItemByID", mock.Anything, testItemID).Return(systemItem(), nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/items/"+testItemID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsDelete_NotFound(t *testing.T) {
	repo := &MockItemRepository{}
	repo.On("GetItemByID", mock.Anything, testItemID).Return(nil, nil)

	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/items/"+testItemID, nil, testutil.AuthHeader(t, "test@example.com", testItemUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestItemsDelete_Unauthorized(t *testing.T) {
	repo := &MockItemRepository{}
	h := handler.NewItemHandler(repo)
	r := newItemTestRouter(h)

	w := doRequest(t, r, http.MethodDelete, "/items/"+testItemID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
