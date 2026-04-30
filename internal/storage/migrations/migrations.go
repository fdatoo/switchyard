// Package migrations embeds eventstore SQL migrations for goose.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
