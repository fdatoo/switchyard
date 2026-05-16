// Command switchyard is the administrative CLI for a running Switchyard daemon.
//
// The CLI is intentionally a thin transport and presentation layer: it parses
// flags, calls the local daemon API, and formats output for humans or scripts.
// Domain behavior belongs in internal packages so the daemon, CLI, tests, and
// MCP tools share the same semantics.
package main
