package main

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockTokenSweepRepository struct {
	mock.Mock
}

func (m *mockTokenSweepRepository) DeleteAllStaleFamilies(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestConfigureGinMode_ProductionSetsReleaseMode(t *testing.T) {
	assert.Equal(t, gin.ReleaseMode, configureGinMode("production"))
}

func TestConfigureGinMode_NonProductionSetsDebugMode(t *testing.T) {
	for _, env := range []string{"development", "", "staging"} {
		assert.Equal(t, gin.DebugMode, configureGinMode(env), "env=%q", env)
	}
}

func TestNewHTTPServer_SetsConfiguredTimeouts(t *testing.T) {
	srv := newHTTPServer(":8080", http.NewServeMux())

	assert.Equal(t, ":8080", srv.Addr)
	assert.Equal(t, 5*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 10*time.Second, srv.ReadTimeout)
	assert.Equal(t, 15*time.Second, srv.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.IdleTimeout)
}

// waitWithTimeout fails the test rather than hanging forever if the sweeper leaks.
func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("runTokenSweeper did not stop within timeout")
	}
}

func TestRunTokenSweeper_CallsDeleteAllStaleFamiliesOnEachTick(t *testing.T) {
	repo := new(mockTokenSweepRepository)
	repo.On("DeleteAllStaleFamilies", mock.Anything).Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go runTokenSweeper(ctx, repo, 20*time.Millisecond, &wg)

	time.Sleep(65 * time.Millisecond)
	cancel()
	waitWithTimeout(t, &wg, 500*time.Millisecond)

	assert.GreaterOrEqual(t, len(repo.Calls), 2, "expected multiple ticks in 65ms at a 20ms interval")
}

func TestRunTokenSweeper_StopsCleanlyOnContextCancellation(t *testing.T) {
	repo := new(mockTokenSweepRepository)
	repo.On("DeleteAllStaleFamilies", mock.Anything).Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go runTokenSweeper(ctx, repo, 15*time.Millisecond, &wg)

	time.Sleep(40 * time.Millisecond)
	callsBeforeCancel := len(repo.Calls)
	require.GreaterOrEqual(t, callsBeforeCancel, 1, "expected at least one tick before cancellation")

	cancel()
	waitWithTimeout(t, &wg, 500*time.Millisecond)

	time.Sleep(40 * time.Millisecond)
	// Allow +1 for a tick that was already in-flight at the exact moment ctx was cancelled.
	assert.LessOrEqual(t, len(repo.Calls), callsBeforeCancel+1, "no further ticks after context cancellation")
}
