// Package starlark is the sandboxed runtime for Switchyard scripts.
//
// It parses expressions and scripts, loads relative modules from the config
// tree, injects Switchyard builtins, enforces wall-clock and step limits, and
// returns structured results that automation and computed-entity callers can
// inspect without depending on raw Starlark values.
package starlark
