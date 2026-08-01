package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/gin-gonic/gin"
)

func newBodyLimitedRouter(maxBytes int64, handlerCalled *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyLimit(maxBytes))
	r.POST("/echo", func(c *gin.Context) {
		*handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	return r
}

func TestBodyLimit_AllowsRequestsUnderCap(t *testing.T) {
	var called bool
	r := newBodyLimitedRouter(10, &called)

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("small"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !called {
		t.Error("expected handler to be called for a body under the cap")
	}
}

func TestBodyLimit_BlocksRequestsOverCap(t *testing.T) {
	var called bool
	r := newBodyLimitedRouter(10, &called)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(make([]byte, 100)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
	if got := w.Body.String(); got != `{"error":"request body too large"}` {
		t.Errorf("expected body-too-large error, got %s", got)
	}
	if called {
		t.Error("expected handler NOT to be called for a body over the cap")
	}
}
