package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/franciskershaw/packing-list-go/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testPackingListItemID2 = "aaaaaaaa-1111-1111-1111-111111111111"
)

// testPackingListCategoryIDCopy exists so mock.On can take its address —
// the const itself isn't addressable.
var testPackingListCategoryIDCopy = testPackingListCategoryID

// --- Fixtures ---
// (mirrors template_item_handler_test.go's tmplAccessibleItem/tmplSystemItem/
// tmplOtherUsersItem, but keyed to testPackingListUserID rather than
// testTemplateUserID)

func pliAccessibleItem(id string) *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(id),
		Name:       "Test Item",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		UserID:     uuidPtr(uuid.MustParse(testPackingListUserID)),
	}
}

func pliSystemItem(id string) *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(id),
		Name:       "System Item",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		UserID:     nil,
		IsSystem:   true,
	}
}

func pliOtherUsersItem(id string) *models.Item {
	return &models.Item{
		ID:         uuid.MustParse(id),
		Name:       "Not Mine",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		UserID:     uuidPtr(uuid.MustParse(otherUserID)),
	}
}

func packingListItem(itemID string, quantity int, notes *string, sortOrder *int) *models.PackingListItem {
	return &models.PackingListItem{
		ItemID:     uuid.MustParse(itemID),
		Name:       "Test Item",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		Quantity:   quantity,
		Notes:      notes,
		SortOrder:  sortOrder,
	}
}

// --- POST /lists/:id/items ---

func TestPackingListItemAdd_DefaultQuantity(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	created := packingListItem(testPackingListItemID, 1, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemByID", mock.Anything, testPackingListItemID).Return(pliAccessibleItem(testPackingListItemID), nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(false, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID, 1, (*string)(nil)).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["quantity"])
	assert.Nil(t, body["sortOrder"])
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemAdd_QuantityAndNotes(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	notes := "bring spares"
	created := packingListItem(testPackingListItemID, 4, &notes, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemByID", mock.Anything, testPackingListItemID).Return(pliAccessibleItem(testPackingListItemID), nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(false, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID, 4, &notes).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID, "quantity": 4, "notes": notes}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemAdd_SystemItemAccessible(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	created := packingListItem(testPackingListItemID, 1, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemByID", mock.Anything, testPackingListItemID).Return(pliSystemItem(testPackingListItemID), nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(false, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID, 1, (*string)(nil)).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemAdd_MissingItemID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemAdd_InvalidItemID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": "not-a-uuid"}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemAdd_InaccessibleItem(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemByID", mock.Anything, testPackingListItemID).Return(pliOtherUsersItem(testPackingListItemID), nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemAdd_Duplicate(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemByID", mock.Anything, testPackingListItemID).Return(pliAccessibleItem(testPackingListItemID), nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemAdd_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemAdd_ListNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemAdd_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/not-a-uuid/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemAdd_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListItemAdd_SucceedsOnArchivedList(t *testing.T) {
	// The handler has no way to know a list is archived (archivedAt isn't
	// exposed on the model), so this asserts the absence of any archived
	// check, mirroring TestPackingListUpdate_SucceedsOnArchivedList.
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	created := packingListItem(testPackingListItemID, 1, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemByID", mock.Anything, testPackingListItemID).Return(pliAccessibleItem(testPackingListItemID), nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(false, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID, 1, (*string)(nil)).Return(created, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items",
		jsonBody(t, map[string]any{"itemId": testPackingListItemID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

// --- PATCH /lists/:id/items/:itemId ---

func TestPackingListItemUpdate_QuantityOnly(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	newQty := 5
	updated := packingListItem(testPackingListItemID, 5, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, &newQty, (*string)(nil), (*int)(nil)).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 5}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_NotesOnly(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	notes := "updated notes"
	updated := packingListItem(testPackingListItemID, 1, &notes, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, (*int)(nil), &notes, (*int)(nil)).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"notes": notes}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_SortOrderOnlyNegative(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	sortOrder := -3
	updated := packingListItem(testPackingListItemID, 1, nil, &sortOrder)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, (*int)(nil), (*string)(nil), &sortOrder).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"sortOrder": -3}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(-3), body["sortOrder"])
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_MissingAllThree(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemUpdate_ItemNotOnList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(false, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 2}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 2}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemUpdate_InvalidItemID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/not-a-uuid",
		jsonBody(t, map[string]any{"quantity": 2}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemUpdate_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/not-a-uuid/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 2}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemUpdate_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 2}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListItemUpdate_SucceedsOnArchivedList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	newQty := 5
	updated := packingListItem(testPackingListItemID, 5, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, &newQty, (*string)(nil), (*int)(nil)).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 5}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

// --- DELETE /lists/:id/items/:itemId ---

func TestPackingListItemRemove_Valid(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("RemovePackingListItem", mock.Anything, testPackingListID, testPackingListItemID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/"+testPackingListItemID, nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemRemove_ItemNotOnList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(false, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/"+testPackingListItemID, nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemRemove_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/"+testPackingListItemID, nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemRemove_ListNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/"+testPackingListItemID, nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemRemove_InvalidItemID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/not-a-uuid", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemRemove_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/not-a-uuid/items/"+testPackingListItemID, nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemRemove_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/"+testPackingListItemID, nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListItemRemove_SucceedsOnArchivedList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("RemovePackingListItem", mock.Anything, testPackingListID, testPackingListItemID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodDelete, "/lists/"+testPackingListID+"/items/"+testPackingListItemID, nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

// --- POST /lists/:id/items/bulk ---

func TestPackingListItemBulkAdd_SomeSkipped(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	categoryItems := []models.Item{*pliAccessibleItem(testPackingListItemID), *pliAccessibleItem(testPackingListItemID2)}
	existing := []models.PackingListItem{*packingListItem(testPackingListItemID, 1, nil, nil)}
	added := packingListItem(testPackingListItemID2, 1, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("CategoryIsAccessible", mock.Anything, testPackingListCategoryID, testPackingListUserID).Return(true, nil)
	itemRepo.On("GetItems", mock.Anything, testPackingListUserID, &testPackingListCategoryIDCopy, (*string)(nil)).Return(categoryItems, nil)
	repo.On("GetPackingListItems", mock.Anything, testPackingListID).Return(existing, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID2, 1, (*string)(nil)).Return(added, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	var body []map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, testPackingListItemID2, body[0]["itemId"])
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemBulkAdd_NoneSkipped(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	categoryItems := []models.Item{*pliAccessibleItem(testPackingListItemID)}
	added := packingListItem(testPackingListItemID, 1, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("CategoryIsAccessible", mock.Anything, testPackingListCategoryID, testPackingListUserID).Return(true, nil)
	itemRepo.On("GetItems", mock.Anything, testPackingListUserID, &testPackingListCategoryIDCopy, (*string)(nil)).Return(categoryItems, nil)
	repo.On("GetPackingListItems", mock.Anything, testPackingListID).Return([]models.PackingListItem{}, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID, 1, (*string)(nil)).Return(added, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	var body []map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 1)
	repo.AssertExpectations(t)
}

func TestPackingListItemBulkAdd_EmptyCategory(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("CategoryIsAccessible", mock.Anything, testPackingListCategoryID, testPackingListUserID).Return(true, nil)
	itemRepo.On("GetItems", mock.Anything, testPackingListUserID, &testPackingListCategoryIDCopy, (*string)(nil)).Return([]models.Item{}, nil)
	repo.On("GetPackingListItems", mock.Anything, testPackingListID).Return([]models.PackingListItem{}, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "[]", w.Body.String())
	repo.AssertExpectations(t)
}

func TestPackingListItemBulkAdd_MissingCategoryID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkAdd_InvalidCategoryID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": "not-a-uuid"}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkAdd_InaccessibleCategoryID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("CategoryIsAccessible", mock.Anything, testPackingListCategoryID, testPackingListUserID).Return(false, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemBulkAdd_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemBulkAdd_ListNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemBulkAdd_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/not-a-uuid/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkAdd_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListItemBulkAdd_SucceedsOnArchivedList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	categoryItems := []models.Item{*pliAccessibleItem(testPackingListItemID)}
	added := packingListItem(testPackingListItemID, 1, nil, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("CategoryIsAccessible", mock.Anything, testPackingListCategoryID, testPackingListUserID).Return(true, nil)
	itemRepo.On("GetItems", mock.Anything, testPackingListUserID, &testPackingListCategoryIDCopy, (*string)(nil)).Return(categoryItems, nil)
	repo.On("GetPackingListItems", mock.Anything, testPackingListID).Return([]models.PackingListItem{}, nil)
	repo.On("AddPackingListItem", mock.Anything, testPackingListID, testPackingListItemID, 1, (*string)(nil)).Return(added, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"categoryId": testPackingListCategoryID}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}
