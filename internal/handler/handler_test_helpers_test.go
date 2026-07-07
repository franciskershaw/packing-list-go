package handler_test

import (
	"bytes"
	"encoding/json"
	"testing"
)

// otherUserID is a user ID distinct from the primary test user, used across
// handler tests to exercise ownership/access checks.
const otherUserID = "22222222-2222-2222-2222-222222222222"

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	return bytes.NewBuffer(b)
}
