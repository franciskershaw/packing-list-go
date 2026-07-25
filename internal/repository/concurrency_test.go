package repository_test

import (
	"context"
	"sync"
	"testing"
)

// TestConcurrentQueries_DoNotErrorOnPooledConnection guards against the
// pooler-incompatible prepared-statement bug fixed 2026-07-25: lib/pq's
// unnamed prepared statements colliding across concurrent requests on
// Neon's PgBouncer transaction-pooling endpoint (see LESSONS.md). Fires
// GetCategories and GetItems concurrently, repeatedly, the same way the
// frontend does on every page load.
func TestConcurrentQueries_DoNotErrorOnPooledConnection(t *testing.T) {
	ctx := context.Background()
	const rounds = 15

	var wg sync.WaitGroup
	errCh := make(chan error, rounds*2)

	for range rounds {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := catRepo.GetCategories(ctx, repoUserID.String()); err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := itemRepo.GetItems(ctx, repoUserID.String(), nil, nil); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent query failed: %v", err)
	}
}
