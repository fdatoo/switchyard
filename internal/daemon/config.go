// Package daemon wires the eventstore, state cache, registry, CLI socket,
// and observability into a single runnable switchyardd.
package daemon

import (
	"log/slog"
	"time"
)

type Config struct {
	DataDir             string
	LogLevel            slog.Level
	LogFormat           string // "auto" | "tty" | "json"
	AdminPort           int    // HTTP for /metrics and /health
	SocketPath          string // UNIX socket for CLI mutative ops
	SnapshotEveryEvents int
	SnapshotEveryPeriod time.Duration
	DriversTOMLPath     string // resolved against DataDir in Run; "@data/drivers.toml" is the sentinel
	CarportSocketDir    string // resolved against DataDir in Run; "@data/carport" is the sentinel
	ConfigDir           string // resolved against DataDir in Run; "@data/config" is the sentinel
}

func (c *Config) WithDefaults() {
	if c.DataDir == "" {
		c.DataDir = "~/.local/share/switchyard"
	}
	if c.LogFormat == "" {
		c.LogFormat = "auto"
	}
	if c.AdminPort == 0 {
		c.AdminPort = 9190
	}
	if c.SocketPath == "" {
		c.SocketPath = "switchyardd.sock"
	}
	if c.SnapshotEveryEvents == 0 {
		c.SnapshotEveryEvents = 10_000
	}
	if c.SnapshotEveryPeriod == 0 {
		c.SnapshotEveryPeriod = time.Hour
	}
	if c.DriversTOMLPath == "" {
		c.DriversTOMLPath = "@data/drivers.toml"
	}
	if c.CarportSocketDir == "" {
		c.CarportSocketDir = "@data/carport"
	}
	if c.ConfigDir == "" {
		c.ConfigDir = "@data/config"
	}
}
