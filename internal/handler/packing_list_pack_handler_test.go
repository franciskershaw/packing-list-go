package handler_test

import (
	"net/http"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- POST /lists/:id/pack-all ---

func TestPackingListPackAll_Success(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackAllItems", mock.Anything, testPackingListID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/pack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListPackAll_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/pack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListPackAll_ListNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/pack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListPackAll_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/not-a-uuid/pack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListPackAll_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/pack-all", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListPackAll_SucceedsOnArchivedList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("PackAllItems", mock.Anything, testPackingListID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/pack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

// --- POST /lists/:id/unpack-all ---

func TestPackingListUnpackAll_Success(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("UnpackAllItems", mock.Anything, testPackingListID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unpack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}

func TestPackingListUnpackAll_ListNotOwned(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(otherUserID, nil)
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unpack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListUnpackAll_ListNotFound(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(nil, nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unpack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackingListUnpackAll_InvalidListID(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/not-a-uuid/unpack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackingListUnpackAll_Unauthorized(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unpack-all", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPackingListUnpackAll_SucceedsOnArchivedList(t *testing.T) {
	repo := &MockPackingListRepository{}
	itemRepo := &MockItemRepository{}
	templateRepo := &MockTemplateRepository{}
	list := packingListDetail(testPackingListUserID, nil)

	repo.On("GetPackingListByID", mock.Anything, testPackingListID).Return(list, nil)
	repo.On("UnpackAllItems", mock.Anything, testPackingListID).Return(nil)

	r := newPackingListTestRouter(repo, templateRepo, itemRepo)

	w := doRequest(t, r, http.MethodPost, "/lists/"+testPackingListID+"/unpack-all", nil,
		testutil.AuthHeader(t, "test@example.com", testPackingListUserID))

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}
