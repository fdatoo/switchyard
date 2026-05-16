package migrations

import "embed"

// FS contains registry schema migrations.
//
//go:embed *.sql
var FS embed.FS
