package web

import "embed"

// Assets contains the built web UI bundle embedded into switchyardd.
//
//go:embed all:dist
var Assets embed.FS
