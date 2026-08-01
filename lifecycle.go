package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// tokenSweepRepository is consumer-defined here since main is the only consumer.
type tokenSweepRepository interface {
	DeleteAllStaleFamilies(ctx context.Context) error
}

// configureGinMode returns gin.ReleaseMode iff env == "production", else gin.DebugMode.
func configureGinMode(env string) string {
	// TODO(PACK-021): stub — always returns "" until implemented.
	return ""
}

// newHTTPServer builds *http.Server with PACK-021's lifecycle timeouts, replacing gin's Run() shorthand.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	// TODO(PACK-021): stub — timeouts left at zero value until implemented.
	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

// runTokenSweeper calls DeleteAllStaleFamilies on each tick until ctx is cancelled, then signals wg.
func runTokenSweeper(ctx context.Context, repo tokenSweepRepository, interval time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()
	// TODO(PACK-021): stub — returns immediately, never ticks, until implemented.
}
