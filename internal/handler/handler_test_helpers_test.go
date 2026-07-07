package handler_test

import (
	"bytes"
	"encoding/json"
	"testing"
)

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	return bytes.NewBuffer(b)
}
