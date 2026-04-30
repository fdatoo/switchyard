// Package observability owns logging, metrics, and tracing setup.
package observability

import (
	"io"
	"log/slog"
	"os"

	charmlog "github.com/charmbracelet/log"
)

type LogConfig struct {
	Level  slog.Level
	Format string // "auto" | "tty" | "json"
	Output io.Writer
}

func Init(cfg LogConfig) *slog.Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	format := cfg.Format
	if format == "" || format == "auto" {
		if isTerminal(cfg.Output) {
			format = "tty"
		} else {
			format = "json"
		}
	}

	var handler slog.Handler
	switch format {
	case "tty":
		h := charmlog.NewWithOptions(cfg.Output, charmlog.Options{
			Level:           charmLevel(cfg.Level),
			ReportTimestamp: true,
			TimeFormat:      "15:04:05.000",
		})
		handler = h
	default:
		handler = slog.NewJSONHandler(cfg.Output, &slog.HandlerOptions{Level: cfg.Level})
	}

	return slog.New(handler)
}

func charmLevel(l slog.Level) charmlog.Level {
	switch {
	case l <= slog.LevelDebug:
		return charmlog.DebugLevel
	case l <= slog.LevelInfo:
		return charmlog.InfoLevel
	case l <= slog.LevelWarn:
		return charmlog.WarnLevel
	default:
		return charmlog.ErrorLevel
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
