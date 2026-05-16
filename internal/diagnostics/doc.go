// Package diagnostics builds downloadable support bundles for operators.
//
// A bundle is a deterministic zip archive containing build metadata, health
// state, recent events, projection cursors, metrics, a redacted config snapshot,
// and goroutine stacks. The package does not collect live data itself; callers
// pass already-authorized inputs through Options.
package diagnostics
