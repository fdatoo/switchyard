package observability_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/fdatoo/gohome/internal/observability"
)

func TestInit_JSONFormatEmitsValidJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.Init(observability.LogConfig{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})
	logger.Info("hello", "entity", "light.lr", "event_position", uint64(42))

	dec := json.NewDecoder(&buf)
	var rec map[string]any
	if err := dec.Decode(&rec); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if rec["msg"] != "hello" {
		t.Fatalf("msg = %v, want hello", rec["msg"])
	}
	if rec["entity"] != "light.lr" {
		t.Fatalf("entity = %v, want light.lr", rec["entity"])
	}
}

func TestInit_TTYFormatIncludesMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.Init(observability.LogConfig{
		Level:  slog.LevelInfo,
		Format: "tty",
		Output: &buf,
	})
	logger.Info("startup complete")
	if !strings.Contains(buf.String(), "startup complete") {
		t.Fatalf("tty output missing message: %q", buf.String())
	}
}

func TestInit_LevelFilters(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.Init(observability.LogConfig{
		Level:  slog.LevelWarn,
		Format: "json",
		Output: &buf,
	})
	logger.Info("should not appear")
	logger.Warn("should appear")
	out := buf.String()
	if strings.Contains(out, "should not appear") {
		t.Fatalf("info leaked at WARN level: %q", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Fatalf("warn missing: %q", out)
	}
}
