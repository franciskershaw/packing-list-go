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
	"github.com/stretchr/testify/require"
)

const (
	testPackingListUserID     = "55555555-5555-5555-5555-555555555555"
	testPackingListTemplateID = "66666666-6666-6666-6666-666666666666"
	testPackingListItemID     = "77777777-7777-7777-7777-777777777777"
	testPackingListCategoryID = "88888888-8888-8888-8888-888888888888"
	testPackingListID         = "99999999-9999-9999-9999-999999999999"
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

func (m *MockPackingListRepository) GetPackingLists(ctx context.Context, userID string, archived bool) ([]models.PackingList, error) {
	args := m.Called(ctx, userID, archived)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PackingList), args.Error(1)
}

func (m *MockPackingListRepository) GetPackingListByID(ctx context.Context, id string) (*models.PackingListDetail, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackingListDetail), args.Error(1)
}

func (m *MockPackingListRepository) UpdatePackingList(ctx context.Context, id string, name *string, eventDate *string) (*models.PackingListDetail, error) {
	args := m.Called(ctx, id, name, eventDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackingListDetail), args.Error(1)
}

func (m *MockPackingListRepository) ArchivePackingList(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPackingListRepository) UnarchivePackingList(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPackingListRepository) AddPackingListItem(ctx context.Context, listID, itemID string, quantity int, notes *string) (*models.PackingListItem, error) {
	args := m.Called(ctx, listID, itemID, quantity, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackingListItem), args.Error(1)
}

func (m *MockPackingListRepository) UpdatePackingListItem(ctx context.Context, listID, itemID string, quantity *int, notes *string, sortOrder *int, isPacked *bool) (*models.PackingListItem, error) {
	args := m.Called(ctx, listID, itemID, quantity, notes, sortOrder, isPacked)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackingListItem), args.Error(1)
}

func (m *MockPackingListRepository) RemovePackingListItem(ctx context.Context, listID, itemID string) error {
	args := m.Called(ctx, listID, itemID)
	return args.Error(0)
}

func (m *MockPackingListRepository) PackingListItemExists(ctx context.Context, listID, itemID string) (bool, error) {
	args := m.Called(ctx, listID, itemID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPackingListRepository) GetPackingListItems(ctx context.Context, listID string) ([]models.PackingListItem, error) {
	args := m.Called(ctx, listID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PackingListItem), args.Error(1)
}

func (m *MockPackingListRepository) PackAllItems(ctx context.Context, listID string) error {
	args := m.Called(ctx, listID)
	return args.Error(0)
}

func (m *MockPackingListRepository) UnpackAllItems(ctx context.Context, listID string) error {
	args := m.Called(ctx, listID)
	return args.Error(0)
}

// --- Helpers ---

func newPackingListTestRouter(repo handler.PackingListRepository, templateRepo handler.TemplateLookupRepository, itemRepo handler.ItemLookupRepository) *gin.Engine {
	h := handler.NewPackingListHandler(repo, templateRepo, itemRepo)
	r := gin.New()
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware(testutil.TestJWTSecretAccess))
	authed.POST("/lists", h.Create)
	authed.GET("/lists", h.List)
	authed.GET("/lists/:id", h.GetByID)
	authed.PATCH("/lists/:id", h.Update)
	authed.DELETE("/lists/:id", h.Delete)
	authed.POST("/lists/:id/unarchive", h.Unarchive)
	authed.POST("/lists/:id/items", h.AddItem)
	authed.PATCH("/lists/:id/items/:itemId", h.UpdateItem)
	authed.DELETE("/lists/:id/items/:itemId", h.RemoveItem)
	authed.POST("/lists/:id/items/bulk", h.BulkAddItems)
	authed.POST("/lists/:id/pack-all", h.PackAll)
	authed.POST("/lists/:id/unpack-all", h.UnpackAll)
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

func packingListDetail(ownerID string, categories []models.PackingListCategory) *models.PackingListDetail {
	if categories == nil {
		categories = []models.PackingListCategory{}
	}
	return &models.PackingListDetail{
		ID:         uuid.MustParse(testPackingListID),
		Name:       "My List",
		UserID:     uuid.MustParse(ownerID),
		Categories: categories,
	}
}

// --- POST /lists ---

func TestPackingListCreate_Valid(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	created := packingListResult(nil)
	repo.On("CreatePackingList", mock.Anything, testPackingListUserID, "My List", (*string)(nil), (*string)(nil)).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

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

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "eventDate": eventDate}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

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

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": testPackingListTemplateID}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

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
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_EmptyName(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": ""}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_NameTooLong(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	longName := strings.Repeat("a", 101)
	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": longName}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_InvalidEventDateFormat(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "eventDate": "08-01-2026"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_InvalidTemplateIdUUID(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": "not-a-uuid"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListCreate_TemplateNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	templateRepo.On("GetTemplateByID", mock.Anything, testPackingListTemplateID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": testPackingListTemplateID}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

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

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List", "templateId": testPackingListTemplateID}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	templateRepo.AssertExpectations(t)
}

func TestPackingListCreate_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists", jsonBody(t, map[string]any{"name": "My List"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GET /lists ---

func TestPackingListList_Active(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	lists := []models.PackingList{{ID: uuid.New(), Name: "Active List", UserID: uuid.MustParse(testPackingListUserID), Items: []models.PackingListItem{}}}
	repo.On("GetPackingLists", mock.Anything, testPackingListUserID, false).Return(lists, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, "Active List", body[0]["name"])
	assert.Equal(t, []any{}, body[0]["items"])
	assert.NotContains(t, body[0], "archivedAt")
	repo.AssertExpectations(t)
}

func TestPackingListList_ArchivedTrue(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	lists := []models.PackingList{{ID: uuid.New(), Name: "Archived List", UserID: uuid.MustParse(testPackingListUserID), Items: []models.PackingListItem{}}}
	repo.On("GetPackingLists", mock.Anything, testPackingListUserID, true).Return(lists, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists?archived=true", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListList_ArchivedGarbageFallsBackToActive(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingLists", mock.Anything, testPackingListUserID, false).Return([]models.PackingList{}, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists?archived=banana", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListList_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	repo.AssertExpectations(t)
}

// --- GET /lists/:id ---

func TestPackingListGetByID_Owned(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	categories := []models.PackingListCategory{
		{ID: uuid.MustParse(testPackingListCategoryID), Name: "Clothes", Items: []models.PackingListDetailItem{
			{ItemID: uuid.MustParse(testPackingListItemID), Name: "Socks", Quantity: 2, IsPacked: false},
		}},
	}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, categories), nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	cats, ok := body["categories"].([]any)
	require.True(t, ok)
	require.Len(t, cats, 1)
	cat := cats[0].(map[string]any)
	assert.Equal(t, "Clothes", cat["name"])
	items := cat["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "Socks", item["name"])
	assert.NotContains(t, item, "categoryId")
	assert.NotContains(t, body, "archivedAt")
	repo.AssertExpectations(t)
}

func TestPackingListGetByID_NotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(otherUserID, nil), nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListGetByID_NotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListGetByID_InvalidID(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists/not-a-uuid", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListGetByID_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodGet, "/lists/"+testPackingListID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	repo.AssertExpectations(t)
}

// --- PATCH /lists/:id ---

func TestPackingListUpdate_NameOnly(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil)
	newName := "Renamed List"
	updated := packingListDetail(testPackingListUserID, nil)
	updated.Name = newName
	repo.On("UpdatePackingList", mock.Anything, testPackingListID, &newName, (*string)(nil)).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": newName}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, newName, body["name"])
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_EventDateOnly(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil)
	eventDate := "2026-09-15"
	updated := packingListDetail(testPackingListUserID, nil)
	updated.EventDate = &eventDate
	repo.On("UpdatePackingList", mock.Anything, testPackingListID, (*string)(nil), &eventDate).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"eventDate": eventDate}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, eventDate, body["eventDate"])
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_Both(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil)
	newName := "Renamed List"
	eventDate := "2026-10-01"
	updated := packingListDetail(testPackingListUserID, nil)
	updated.Name = newName
	updated.EventDate = &eventDate
	repo.On("UpdatePackingList", mock.Anything, testPackingListID, &newName, &eventDate).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": newName, "eventDate": eventDate}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_MissingBoth(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_EmptyName(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": ""}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_InvalidEventDateFormat(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"eventDate": "08-01-2026"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_NotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(otherUserID, nil), nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_NotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_InvalidID(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/not-a-uuid", jsonBody(t, map[string]any{"name": "New Name"}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": "New Name"}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUpdate_SucceedsOnArchivedList(t *testing.T) {
	// The handler has no way to know a list is archived (archivedAt isn't
	// exposed on the model), so this asserts the absence of any archived
	// check: GetPackingListByID returning an owned list is enough for the
	// update to proceed, whatever its archived state actually is.
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil)
	newName := "Renamed Archived List"
	updated := packingListDetail(testPackingListUserID, nil)
	updated.Name = newName
	repo.On("UpdatePackingList", mock.Anything, testPackingListID, &newName, (*string)(nil)).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID, jsonBody(t, map[string]any{"name": newName}), testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

// --- DELETE /lists/:id ---

func TestPackingListDelete_Success(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil)
	repo.On("ArchivePackingList", mock.Anything, testPackingListID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListDelete_NotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(otherUserID, nil), nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListDelete_NotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListDelete_InvalidID(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/lists/not-a-uuid", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListDelete_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListDelete_IdempotentOnAlreadyArchived(t *testing.T) {
	// Mirrors the repository-level idempotency guarantee: the handler makes
	// no "already archived" check of its own, so calling Delete a second
	// time follows the exact same path and still returns 204.
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil).Twice()
	repo.On("ArchivePackingList", mock.Anything, testPackingListID).Return(nil).Twice()

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	for i := 0; i < 2; i++ {
		w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID, nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
	repo.AssertExpectations(t)
}

// --- POST /lists/:id/unarchive ---

func TestPackingListUnarchive_Success(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil)
	repo.On("UnarchivePackingList", mock.Anything, testPackingListID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unarchive", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUnarchive_NotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(otherUserID, nil), nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unarchive", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUnarchive_NotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unarchive", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUnarchive_InvalidID(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists/not-a-uuid/unarchive", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUnarchive_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unarchive", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUnarchive_IdempotentOnAlreadyActive(t *testing.T) {
	// Mirrors TestPackingListDelete_IdempotentOnAlreadyArchived: the handler
	// makes no "already active" check of its own, so calling Unarchive a
	// second time follows the exact same path and still returns 204.
	repo := &MockPackingListRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(packingListDetail(testPackingListUserID, nil), nil).Twice()
	repo.On("UnarchivePackingList", mock.Anything, testPackingListID).Return(nil).Twice()

	r := newPackingListTestRouter(repo, templateRepo, &MockItemRepository{})

	for i := 0; i < 2; i++ {
		w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unarchive", nil, testutil.AuthHeader(t, "test@example.com", testPackingListUserID))
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
	repo.AssertExpectations(t)
}
