// Package pkllsp exposes Pkl language-server features through Connect-RPC.
//
// It keeps a single Pkl language-server subprocess per service, translates
// completion, hover, definition, diagnostics, and semantic-token requests, and
// owns the lifecycle rules needed to start and stop that subprocess cleanly.
package pkllsp
