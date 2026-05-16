package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/fdatoo/switchyard/internal/daemon"
	"github.com/fdatoo/switchyard/internal/observability"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		dataDir          = flag.String("data-dir", "", "data directory (default ~/.local/share/switchyard)")
		logLevel         = flag.String("log-level", "info", "error|warn|info|debug")
		logFormat        = flag.String("log-format", "auto", "auto|tty|json")
		adminPort        = flag.Int("admin-port", 9190, "HTTP admin port for /metrics and /health")
		snapshotEveryEvt = flag.Int("snapshot-every-events", 10_000, "snapshot cadence: events since last")
		snapshotEveryDur = flag.Duration("snapshot-every-period", time.Hour, "snapshot cadence: wall-clock period")
		configDir        = flag.String("config-dir", "", "config directory with main.pkl (default <data-dir>/config)")
		driversDir       = flag.String("drivers-dir", "", "directory containing per-driver subdirectories (default <data-dir>/drivers)")
		pklLspPath       = flag.String("pkl-lsp-path", "", "path to pkl-lsp binary for the config editor")
	)
	flag.Parse()

	level := parseLevel(*logLevel)
	logger := observability.Init(observability.LogConfig{
		Level:  level,
		Format: *logFormat,
		Output: os.Stderr,
	})
	metrics := observability.NewMetrics()

	if info, ok := debug.ReadBuildInfo(); ok {
		daemon.GoVersion = info.GoVersion
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && daemon.Commit == "unknown" {
				daemon.Commit = s.Value
			}
		}
	}

	cfg := daemon.Config{
		DataDir:             *dataDir,
		LogLevel:            level,
		LogFormat:           *logFormat,
		AdminPort:           *adminPort,
		SnapshotEveryEvents: *snapshotEveryEvt,
		SnapshotEveryPeriod: *snapshotEveryDur,
		ConfigDir:           *configDir,
		DriversDir:          *driversDir,
		PklLspPath:          *pklLspPath,
	}
	d := daemon.New(cfg, logger, metrics)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := d.Run(ctx); err != nil {
		logger.Error("daemon exited with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
