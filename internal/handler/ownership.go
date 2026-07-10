package handler

import (
	"github.com/franciskershaw/packing-list-go/internal/models"
	"github.com/google/uuid"
)

// isOwnedBy reports whether userID (nil for system-owned records) matches requestingUserID.
func isOwnedBy(userID *uuid.UUID, requestingUserID string) bool {
	return userID != nil && userID.String() == requestingUserID
}

// isPackingListOwned returns true only when the list exists and belongs to
// the given user. Packing lists have no system-level concept, like
// templates, so this is a plain equality check rather than a
// nil-means-system one.
func isPackingListOwned(list *models.PackingListDetail, userID string) bool {
	return list != nil && list.UserID.String() == userID
}
