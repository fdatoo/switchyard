package storage

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"

	eventMigrations "github.com/fdatoo/gohome/internal/storage/migrations"
)

type Config struct {
	Path string // absolute path to .db file; use ":memory:" for tests
}

// Open opens a SQLite database, applies runtime PRAGMAs, and runs eventstore migrations.
// Registry migrations are applied via Migrate after this returns.
// Use OpenReadOnly for read-only clients (e.g. CLI) that must not run migrations.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(ctx, db, eventMigrations.FS, "eventstore"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// OpenReadOnly opens the SQLite file with PRAGMAs only — no migrations.
// Safe to call from CLI processes reading a live daemon database concurrently.
func OpenReadOnly(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}

// Migrate applies a caller-provided embed.FS of goose SQL migrations.
// Exposed so registry can run its own migration set after Open returns.
// Each label gets its own goose_db_version_<label> table so migration sets
// don't collide.
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS, label string) error {
	tableName := "goose_db_version_" + label
	p, err := goose.NewProvider(goose.DialectSQLite3, db, fsys,
		goose.WithTableName(tableName),
	)
	if err != nil {
		return fmt.Errorf("goose provider %s: %w", label, err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("goose up %s: %w", label, err)
	}
	return nil
}
