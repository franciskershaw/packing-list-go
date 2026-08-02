package repository_test

import (
	"testing"

	"github.com/franciskershaw/packing-list-go/db"
	"github.com/stretchr/testify/assert"
)

// cleanupExec registers a t.Cleanup that runs query/args and asserts it succeeded.
func cleanupExec(t *testing.T, query string, args ...any) {
	t.Helper()
	t.Cleanup(func() {
		_, err := db.DB.Exec(query, args...)
		assert.NoError(t, err)
	})
}
