package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func newRateLimitedRouter(rate limiter.Rate) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit(memory.NewStore(), rate))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	return r
}

func doRateLimitedGet(r *gin.Engine, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_AllowsRequestsUnderLimit(t *testing.T) {
	r := newRateLimitedRouter(limiter.Rate{Period: time.Minute, Limit: 2})

	for i := 0; i < 2; i++ {
		if w := doRateLimitedGet(r, "1.2.3.4:1234"); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimit_BlocksRequestsOverLimit(t *testing.T) {
	r := newRateLimitedRouter(limiter.Rate{Period: time.Minute, Limit: 2})

	for i := 0; i < 2; i++ {
		if w := doRateLimitedGet(r, "1.2.3.4:1234"); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRateLimitedGet(r, "1.2.3.4:1234")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if got := w.Body.String(); got != `{"error":"rate limit exceeded"}` {
		t.Errorf("expected rate-limit-exceeded error body, got %s", got)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set, got none")
	}
}

func TestRateLimit_TracksDifferentIPsSeparately(t *testing.T) {
	r := newRateLimitedRouter(limiter.Rate{Period: time.Minute, Limit: 1})

	if w := doRateLimitedGet(r, "1.2.3.4:1234"); w.Code != http.StatusOK {
		t.Fatalf("IP A first request: expected 200, got %d", w.Code)
	}
	if w := doRateLimitedGet(r, "1.2.3.4:1234"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("IP A second request: expected 429, got %d", w.Code)
	}
	if w := doRateLimitedGet(r, "5.6.7.8:5678"); w.Code != http.StatusOK {
		t.Fatalf("IP B first request: expected 200 (separate bucket from IP A), got %d", w.Code)
	}
}
