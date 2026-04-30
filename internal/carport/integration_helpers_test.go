//go:build integration

package carport_test

import (
	"bytes"
	"log/slog"

	"github.com/fdatoo/gohome/internal/observability"
)

func newTestLogger() *slog.Logger {
	return observability.Init(observability.LogConfig{Level: slog.LevelWarn, Format: "json", Output: &bytes.Buffer{}})
}

func newTestMetrics() *observability.Metrics {
	return observability.NewMetrics()
}
