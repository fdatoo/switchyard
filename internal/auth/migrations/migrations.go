// Package migrations embeds auth SQL migrations for goose.
package migrations

import "embed"

// FS contains auth schema migrations.
//
//go:embed *.sql
var FS embed.FS
