package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envWithout(keys ...string) []string {
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		name := strings.SplitN(kv, "=", 2)[0]
		if !drop[name] {
			out = append(out, kv)
		}
	}
	return out
}

func TestRepositorySuite_FailsLoudWithoutDatabaseURL(t *testing.T) {
	// -count=1: Go's test cache doesn't know this package's outcome depends on DATABASE_URL.
	// -v: on a passing run "go test" suppresses stdout entirely, hiding the skip message.
	cmd := exec.Command("go", "test", "-count=1", "-v", "./internal/repository/...")
	cmd.Env = envWithout("DATABASE_URL", "ALLOW_SKIP_DB_TESTS")
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, string(out), "FATAL")
}

func TestRepositorySuite_SkipsWithOptOutFlag(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-v", "./internal/repository/...")
	cmd.Env = append(envWithout("DATABASE_URL", "ALLOW_SKIP_DB_TESTS"), "ALLOW_SKIP_DB_TESTS=1")
	out, err := cmd.CombinedOutput()

	assert.NoError(t, err)
	assert.Contains(t, string(out), "skipping repository tests")
}
