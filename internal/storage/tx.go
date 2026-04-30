// Package storage owns SQLite open/PRAGMA/migration plumbing and the
// minimal Tx interface projectors use. Nothing outside this package
// should import database/sql directly except cmd/* wiring.
package storage

import (
	"context"
	"database/sql"
)

// Tx is the transactional surface projectors work against.
// *sql.Tx satisfies this interface directly; test fakes can substitute.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
