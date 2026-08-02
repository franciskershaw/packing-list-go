package middleware_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/gin-gonic/gin"
)

func withCapturedLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	return &buf
}

func TestErrorLogger_LogsAttachedErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	buf := withCapturedLog(t)

	r := gin.New()
	r.Use(middleware.ErrorLogger())
	r.GET("/boom", func(c *gin.Context) {
		_ = c.Error(errors.New("boom"))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something broke"})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	for _, want := range []string{"request failed", "method=GET", "path=/boom", "status=500", "err=boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q, got: %s", want, out)
		}
	}
}

func TestErrorLogger_NoAttachedErrors_LogsNothing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	buf := withCapturedLog(t)

	r := gin.New()
	r.Use(middleware.ErrorLogger())
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "fine"})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log output for a successful request, got: %s", buf.String())
	}
}
