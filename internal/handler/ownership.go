package handler

import "github.com/google/uuid"

// isOwnedBy reports whether userID (nil for system-owned records) matches requestingUserID.
func isOwnedBy(userID *uuid.UUID, requestingUserID string) bool {
	return userID != nil && userID.String() == requestingUserID
}
