// Package migrations embeds eventstore SQL migrations for goose.
package migrations

import "embed"

// FS contains eventstore schema migrations.
//
//go:embed *.sql
var FS embed.FS
