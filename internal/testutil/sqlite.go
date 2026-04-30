// Package testutil offers shared helpers: in-memory DB, event builders,
// fixture loading. Test-only code.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/fdatoo/gohome/internal/storage"
)

// NewTestDB opens a file-backed SQLite in t.TempDir(), applies migrations,
// and returns the handle. File-backed rather than :memory: because WAL
// behavior differs and we want realism.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), storage.Config{
		Path: filepath.Join(dir, "test.db"),
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
