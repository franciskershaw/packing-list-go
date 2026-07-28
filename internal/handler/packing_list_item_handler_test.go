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
)

const (
	testPackingListItemID2 = "aaaaaaaa-1111-1111-1111-111111111111"
)

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
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, &newQty, (*string)(nil), (*int)(nil), (*bool)(nil)).Return(updated, nil)

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
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, (*int)(nil), &notes, (*int)(nil), (*bool)(nil)).Return(updated, nil)

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
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, (*int)(nil), (*string)(nil), &sortOrder, (*bool)(nil)).Return(updated, nil)

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

func TestPackingListItemUpdate_IsPackedTrue(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	isPacked := true
	updated := &models.PackingListItem{
		ItemID:     uuid.MustParse(testPackingListItemID),
		Name:       "Test Item",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		Quantity:   1,
		IsPacked:   true,
	}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, (*int)(nil), (*string)(nil), (*int)(nil), &isPacked).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"isPacked": true}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["isPacked"])
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_IsPackedFalse(t *testing.T) {
	// Explicit false must be distinguishable from omission — isPacked is
	// *bool precisely so this request is recognized as "set to false", not
	// dropped by the "at least one of ... is required" check.
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	isPacked := false
	updated := &models.PackingListItem{
		ItemID:     uuid.MustParse(testPackingListItemID),
		Name:       "Test Item",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		Quantity:   1,
		IsPacked:   false,
	}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, (*int)(nil), (*string)(nil), (*int)(nil), &isPacked).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"isPacked": false}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_AllFour(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	newQty := 3
	sortOrder := 1
	isPacked := true
	notes := "combined with isPacked"
	updated := &models.PackingListItem{
		ItemID:     uuid.MustParse(testPackingListItemID),
		Name:       "Test Item",
		CategoryID: uuid.MustParse(testPackingListCategoryID),
		Quantity:   3,
		Notes:      &notes,
		SortOrder:  &sortOrder,
		IsPacked:   true,
	}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackingListItemExists", mock.Anything, testPackingListID, testPackingListItemID).Return(true, nil)
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, &newQty, &notes, &sortOrder, &isPacked).Return(updated, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{"quantity": 3, "notes": notes, "sortOrder": 1, "isPacked": true}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemUpdate_MissingAllFour(t *testing.T) {
	// Ownership is checked before the body, mirroring
	// TemplateHandler.UpdateItem's requireOwnedTemplate-first order (via
	// requireOwnedPackingList here) — so GetPackingListByID is still
	// expected even though the body itself is what ultimately 400s.
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/"+testPackingListItemID,
		jsonBody(t, map[string]any{}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertExpectations(t)
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
	repo.On("UpdatePackingListItem", mock.Anything, testPackingListID, testPackingListItemID, &newQty, (*string)(nil), (*int)(nil), (*bool)(nil)).Return(updated, nil)

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

// --- PATCH /lists/:id/items/bulk ---
// Delta contract (PACK-035): quantity 0 removes, any other quantity in
// [0, 999] adds-if-absent-or-updates-if-present. Existence branching
// happens entirely inside BulkUpdatePackingListItems (repo layer, see
// TestBulkUpdatePackingListItems_AddsUpdatesAndRemoves) — the handler
// only validates the request shape and item accessibility, then forwards
// the whole changes map in one call, so there's no separate "add" vs
// "update" path to test here.

func TestPackingListItemBulkUpdate_MixedBatchSucceeds(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	batchItems := []models.Item{*pliAccessibleItem(testPackingListItemID), *pliAccessibleItem(testPackingListItemID2)}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testPackingListItemID, testPackingListItemID2}).Return(batchItems, nil)
	repo.On("BulkUpdatePackingListItems", mock.Anything, testPackingListID, map[string]int{testPackingListItemID: 5, testPackingListItemID2: 0}).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{
			{"itemId": testPackingListItemID, "quantity": 5},
			{"itemId": testPackingListItemID2, "quantity": 0},
		}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemBulkUpdate_NoopRemoveOfAbsentItem(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	batchItems := []models.Item{*pliAccessibleItem(testPackingListItemID)}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testPackingListItemID}).Return(batchItems, nil)
	repo.On("BulkUpdatePackingListItems", mock.Anything, testPackingListID, map[string]int{testPackingListItemID: 0}).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 0}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code, "quantity 0 must pass handler validation, not be rejected as out of range")
	repo.AssertExpectations(t)
}

func TestPackingListItemBulkUpdate_EmptyArray(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkUpdate_DuplicateItemId(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{
			{"itemId": testPackingListItemID, "quantity": 1},
			{"itemId": testPackingListItemID, "quantity": 2},
		}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkUpdate_InvalidItemId(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": "not-a-uuid", "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkUpdate_QuantityTooLow(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": -1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkUpdate_QuantityTooHigh(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1000}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkUpdate_InaccessibleItem(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	batchItems := []models.Item{*pliOtherUsersItem(testPackingListItemID)}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testPackingListItemID}).Return(batchItems, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemBulkUpdate_UnknownItemId(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testPackingListItemID}).Return([]models.Item{}, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	itemRepo.AssertExpectations(t)
}

func TestPackingListItemBulkUpdate_RepoErrorReturns500(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	batchItems := []models.Item{*pliAccessibleItem(testPackingListItemID)}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testPackingListItemID}).Return(batchItems, nil)
	repo.On("BulkUpdatePackingListItems", mock.Anything, testPackingListID, map[string]int{testPackingListItemID: 3}).Return(assert.AnError)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 3}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListItemBulkUpdate_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemBulkUpdate_ListNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListItemBulkUpdate_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/not-a-uuid/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListItemBulkUpdate_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}), "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListItemBulkUpdate_SucceedsOnArchivedList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)
	batchItems := []models.Item{*pliAccessibleItem(testPackingListItemID)}

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	itemRepo.On("GetItemsByIDs", mock.Anything, []string{testPackingListItemID}).Return(batchItems, nil)
	repo.On("BulkUpdatePackingListItems", mock.Anything, testPackingListID, map[string]int{testPackingListItemID: 1}).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPatch, "/lists/"+testPackingListID+"/items/bulk",
		jsonBody(t, map[string]any{"items": []map[string]any{{"itemId": testPackingListItemID, "quantity": 1}}}),
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}
