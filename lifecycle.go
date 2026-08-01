package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// tokenSweepRepository is consumer-defined here since main is the only consumer.
type tokenSweepRepository interface {
	DeleteAllStaleFamilies(ctx context.Context) error
}

// configureGinMode returns gin.ReleaseMode iff env == "production", else gin.DebugMode.
func configureGinMode(env string) string {
	if env == "production" {
		return gin.ReleaseMode
	}
	return gin.DebugMode
}

// newHTTPServer builds *http.Server with PACK-021's lifecycle timeouts, replacing gin's Run() shorthand.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// runTokenSweeper calls DeleteAllStaleFamilies on each tick until ctx is cancelled, then signals wg.
func runTokenSweeper(ctx context.Context, repo tokenSweepRepository, interval time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := repo.DeleteAllStaleFamilies(ctx); err != nil {
				slog.Error("token sweeper: failed to delete stale refresh token families", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
