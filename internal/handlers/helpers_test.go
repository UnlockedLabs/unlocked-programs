package handlers

import (
	"testing"

	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/UnlockedLabs/unlocked-programs/internal/testutil"
)

// newTestServer returns a Programs server on an empty test database.
func newTestServer(t *testing.T) *server {
	t.Helper()
	return &server{store: database.NewDB(testutil.NewDB(t))}
}
