package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

// doRequest builds a request, optionally attaching Content-Type (when body
// is non-nil) and Authorization (when authHeader is non-empty), sends it
// through r, and returns the recorded response. Collapses the
// NewRequest+headers+NewRecorder+ServeHTTP block every handler test would
// otherwise repeat verbatim.
func doRequest(t *testing.T, r *gin.Engine, method, path string, body io.Reader, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, body)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
