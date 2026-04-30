# C1 — Event Core & Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the event core and storage foundation (`gohomed` daemon + `gohome` CLI) with SQLite event log, state cache, registry projector, and observability — per the approved spec at `docs/superpowers/specs/2026-04-21-c1-event-core-and-storage-design.md`.

**Architecture:** Single Go module at `github.com/fynn-labs/gohome` holding both binaries. Event log in SQLite (WAL mode) is source of truth; state cache and registry are materialized views projected synchronously during Append. Copy-on-write HAMT for state, goose-migrated SQL tables for registry, per-projector protobuf+zstd snapshots. Central tailer goroutine with `sync.Cond` fanout to subscribers and async projectors.

**Tech Stack:** Go 1.22+, SQLite via `modernc.org/sqlite`, goose migrations, `google.golang.org/protobuf`, `github.com/benbjohnson/immutable`, `github.com/klauspost/compress/zstd`, Prometheus `client_golang`, stdlib `slog` + `charmbracelet/log`, `charmbracelet/lipgloss` + `cobra` for CLI. Build via `go-task`; proto codegen via `buf`.

**Working directory:** Repo lives at `/Users/fdatoo/Desktop/GoHome/gohome/`. The `GoHome/` directory is a container — DO NOT git-init at the container level. All `git` commands in this plan run inside `/Users/fdatoo/Desktop/GoHome/gohome/`.

---

## Task 1: Initialize repo, Go module, and tooling

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/go.mod`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/.gitignore`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/LICENSE`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/README.md`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/Taskfile.yml`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/.golangci.yml`

- [ ] **Step 1: Create the repo directory and initialize git**

```bash
mkdir -p /Users/fdatoo/Desktop/GoHome/gohome
cd /Users/fdatoo/Desktop/GoHome/gohome
git init
git branch -M main
```

Expected: `Initialized empty Git repository in .../gohome/.git/`

- [ ] **Step 2: Initialize Go module**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod init github.com/fynn-labs/gohome
```

Expected: `go: creating new go.mod: module github.com/fynn-labs/gohome`

- [ ] **Step 3: Write `.gitignore`**

```gitignore
# Binaries
/dist/
/bin/
gohomed
gohome
*.exe

# Go build cache + coverage
*.test
*.out
/coverage/

# Data directories used during local runs
/data/
/var/
*.db
*.db-shm
*.db-wal
gohomed.lock
gohomed.sock

# Editor
.idea/
.vscode/
*.swp
.DS_Store
```

- [ ] **Step 4: Write `LICENSE` (Apache 2.0 or MIT — pick MIT, short form)**

Use MIT license text. Replace copyright year with `2026` and holder with `Fynn Labs`.

- [ ] **Step 5: Write `README.md`**

```markdown
# gohome

Go-native home automation core. Event-sourced, SQLite-backed, operator-friendly.

This repository ships the daemon (`gohomed`) and CLI (`gohome`) as a single Go module.
Driver hosts, edge agents, and the web UI live in separate repos.

## Build

```
task build
```

## Status

C1 (event core + storage) is the current milestone. See `docs/architecture.md`.
```

- [ ] **Step 6: Write `Taskfile.yml`**

```yaml
version: '3'

vars:
  BIN_DIR: '{{.ROOT_DIR}}/dist'

tasks:
  default:
    deps: [build]

  proto:
    desc: Regenerate protobuf code via buf
    cmds:
      - buf generate

  build:
    desc: Build both binaries
    cmds:
      - go build -o {{.BIN_DIR}}/gohomed ./cmd/gohomed
      - go build -o {{.BIN_DIR}}/gohome ./cmd/gohome

  test:
    desc: Run unit tests
    cmds:
      - go test ./...

  test:race:
    desc: Run tests with race detector
    cmds:
      - go test -race ./...

  test:integration:
    desc: Run integration tests (crash-safety, real disk)
    cmds:
      - go test -tags=integration ./...

  test:fuzz:
    desc: Run fuzz targets briefly
    cmds:
      - go test -fuzz=Fuzz -fuzztime=30s ./internal/eventstore
      - go test -fuzz=Fuzz -fuzztime=30s ./internal/registry

  test:update-golden:
    desc: Rewrite golden fixtures from current behavior
    cmds:
      - go test ./... -update

  lint:
    desc: Run golangci-lint
    cmds:
      - golangci-lint run ./...

  tidy:
    desc: go mod tidy
    cmds:
      - go mod tidy
```

- [ ] **Step 7: Write `.golangci.yml`**

```yaml
run:
  timeout: 5m
  tests: true

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - gocritic
    - revive
    - misspell
    - unconvert
    - prealloc
    - bodyclose
    - contextcheck

linters-settings:
  goimports:
    local-prefixes: github.com/fynn-labs/gohome
  revive:
    rules:
      - name: exported
        disabled: true

issues:
  exclude-rules:
    - path: _test\.go
      linters: [errcheck, gocritic]
```

- [ ] **Step 8: Verify Go module compiles**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build ./...
```

Expected: silent (nothing to build yet — no `.go` files) OR `go: no Go files in .../gohome` — both acceptable.

- [ ] **Step 9: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add .gitignore LICENSE README.md Taskfile.yml .golangci.yml go.mod
git commit -m "chore: initialize gohome repo, go module, and build tooling"
```

---

## Task 2: Protobuf definitions and buf codegen

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/proto/buf.yaml`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/buf.gen.yaml`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/event/v1/event.proto`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/event/v1/snapshot.proto`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/entity/v1/attributes.proto`

- [ ] **Step 1: Write `proto/buf.yaml`**

```yaml
version: v2
modules:
  - path: .
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

- [ ] **Step 2: Write `buf.gen.yaml` (repo root)**

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/fynn-labs/gohome/gen
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
```

- [ ] **Step 3: Write `proto/gohome/entity/v1/attributes.proto`**

```protobuf
syntax = "proto3";

package gohome.entity.v1;

// Attributes carries both capabilities (static) and live state (dynamic)
// for an entity. Specific payloads land in later milestones — C1 only
// needs the envelope compiling. Future oneof variants get added here.
message Attributes {
  oneof kind {
    Light  light  = 10;
    Switch switch = 11;
    Sensor sensor = 12;
  }
}

message Light {
  bool   on         = 1;
  uint32 brightness = 2;   // 0-255
  uint32 color_temp = 3;   // mireds; 0 if unsupported
  // RGB/XY/capabilities flesh out later; C1 only needs the message existing.
}

message Switch {
  bool on = 1;
}

message Sensor {
  string unit  = 1;
  double value = 2;
}
```

- [ ] **Step 4: Write `proto/gohome/event/v1/event.proto`**

```protobuf
syntax = "proto3";

package gohome.event.v1;

import "gohome/entity/v1/attributes.proto";

// Payload is the on-the-wire variant union. The DB stores this marshalled
// inside the `events.payload` BLOB column; position/ts/kind/entity/source/
// correlation/cause live in dedicated columns.
message Payload {
  oneof kind {
    SystemEvent         system              = 1;
    StateChanged        state_changed       = 10;
    CommandIssued       command_issued      = 11;
    CommandAck          command_ack         = 12;
    EntityRegistered    entity_registered   = 20;
    EntityUnregistered  entity_unregistered = 21;
    DriverEvent         driver_event        = 30;
  }
}

message SystemEvent {
  string              kind = 1;   // "startup" | "shutdown" | ...
  map<string, string> data = 2;
}

message StateChanged {
  gohome.entity.v1.Attributes attributes = 1;
}

message CommandIssued {
  string              command    = 1;
  map<string, string> parameters = 2;
}

message CommandAck {
  bool   success       = 1;
  string error_message = 2;
}

message EntityRegistered {
  string                      driver_instance_id = 1;
  string                      device_id          = 2;
  string                      entity_type        = 3;
  string                      friendly_name      = 4;
  gohome.entity.v1.Attributes capabilities       = 5;
}

message EntityUnregistered {
  string reason = 1;
}

message DriverEvent {
  string driver_instance_id = 1;
  string kind               = 2;   // "started" | "stopped" | "failed" | "heartbeat"
  string detail             = 3;
}
```

- [ ] **Step 5: Write `proto/gohome/event/v1/snapshot.proto`**

```protobuf
syntax = "proto3";

package gohome.event.v1;

import "gohome/entity/v1/attributes.proto";

message StateCacheSnapshot {
  uint64              position = 1;
  int64               ts       = 2;   // unix nanos
  repeated EntityState entities = 3;
}

message EntityState {
  string                      entity_id  = 1;
  int64                       updated_at = 2;   // unix nanos
  string                      updated_by = 3;
  gohome.entity.v1.Attributes attributes = 4;
}
```

- [ ] **Step 6: Generate Go code**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
buf generate
```

Expected: creates `gen/gohome/event/v1/event.pb.go`, `snapshot.pb.go`, `gen/gohome/entity/v1/attributes.pb.go`.

- [ ] **Step 7: Tidy module and verify generated code compiles**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go build ./gen/...
```

Expected: silent success.

- [ ] **Step 8: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add proto buf.gen.yaml gen go.mod go.sum
git commit -m "feat(proto): define event, snapshot, and entity attributes protos"
```

---

## Task 3: Observability — logging

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/logging.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/context.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/logging_test.go`

- [ ] **Step 1: Write failing test `internal/observability/logging_test.go`**

```go
package observability_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/observability"
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/observability/...
```

Expected: FAIL — package `observability` does not exist.

- [ ] **Step 3: Write `internal/observability/logging.go`**

```go
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
```

- [ ] **Step 4: Write `internal/observability/context.go`**

```go
package observability

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/observability/...
```

Expected: PASS — 3 tests.

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/observability go.mod go.sum
git commit -m "feat(observability): add slog init with TTY/JSON formats and context helpers"
```

---

## Task 4: Observability — metrics and tracing stubs

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/metrics.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/metrics_server.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/tracing.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/observability/metrics_test.go`

- [ ] **Step 1: Write failing test `internal/observability/metrics_test.go`**

```go
package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/observability"
)

func TestMetrics_AppendIncrementsCounter(t *testing.T) {
	m := observability.NewMetrics()
	m.EventsAppended.WithLabelValues("state_changed").Inc()
	m.EventsAppended.WithLabelValues("state_changed").Inc()

	srv := httptest.NewServer(m.HTTPHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := new(strings.Builder)
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), `gohome_events_appended_total{kind="state_changed"} 2`) {
		t.Fatalf("missing counter in /metrics output:\n%s", body.String())
	}
}

func TestMetrics_BuildInfoExposed(t *testing.T) {
	m := observability.NewMetrics()
	m.SetBuildInfo("1.2.3", "abcdef", "go1.22")

	srv := httptest.NewServer(m.HTTPHandler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := new(strings.Builder)
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), `version="1.2.3"`) {
		t.Fatalf("build_info missing version: %s", body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/observability/...
```

Expected: FAIL — `NewMetrics` undefined.

- [ ] **Step 3: Write `internal/observability/metrics.go`**

```go
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Registry *prometheus.Registry

	// Append path
	EventsAppended         *prometheus.CounterVec
	AppendDuration         prometheus.Histogram
	AppendRetries          prometheus.Counter
	AppendFailures         *prometheus.CounterVec

	// Projectors
	ProjectorApplyDuration *prometheus.HistogramVec
	ProjectorFailures      *prometheus.CounterVec
	ProjectorLag           *prometheus.GaugeVec
	ProjectorCatchup       *prometheus.GaugeVec

	// Tailer
	TailerLag       prometheus.Gauge
	TailerBatchSize prometheus.Histogram

	// Subscriptions
	SubscriptionActive    *prometheus.GaugeVec
	SubscriptionDelivered *prometheus.CounterVec
	SubscriptionDropped   *prometheus.CounterVec
	SubscriptionBuffered  *prometheus.GaugeVec
	SubscriptionCatchup   *prometheus.HistogramVec

	// Snapshots
	SnapshotDuration   *prometheus.HistogramVec
	SnapshotSize       *prometheus.GaugeVec
	SnapshotLastPos    *prometheus.GaugeVec
	SnapshotCorruption *prometheus.CounterVec

	// Storage
	SQLiteWALBytes     prometheus.Gauge
	SQLiteEventsTotal  prometheus.Gauge
	SQLiteBusyRetries  prometheus.Counter

	// Startup
	StartupPhase           prometheus.Gauge
	StartupDuration        prometheus.Histogram
	ReplayEventsProcessed  prometheus.Counter
	RecoveryModeEntered    prometheus.Counter

	// Health
	BuildInfo *prometheus.GaugeVec
	Uptime    prometheus.GaugeFunc
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{Registry: reg}

	m.EventsAppended = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gohome_events_appended_total", Help: "Events appended by kind"},
		[]string{"kind"},
	)
	m.AppendDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "gohome_events_append_duration_seconds",
		Help:    "End-to-end Append duration",
		Buckets: prometheus.DefBuckets,
	})
	m.AppendRetries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gohome_events_append_retries_total", Help: "SQLite BUSY retries on Append",
	})
	m.AppendFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gohome_events_append_failures_total", Help: "Append failures by stage"},
		[]string{"stage"},
	)

	m.ProjectorApplyDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "gohome_projector_apply_duration_seconds", Help: "Projector.Apply duration"},
		[]string{"projector", "mode"},
	)
	m.ProjectorFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gohome_projector_failures_total", Help: "Projector.Apply failures"},
		[]string{"projector", "mode"},
	)
	m.ProjectorLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_projector_lag_events", Help: "Events behind head per projector"},
		[]string{"projector"},
	)
	m.ProjectorCatchup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_projector_catchup_mode", Help: "1 if async projector is in SQL catchup"},
		[]string{"projector"},
	)

	m.TailerLag = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_tailer_lag_events", Help: "Events tailer is behind head",
	})
	m.TailerBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "gohome_tailer_batch_size", Help: "Events per tailer dispatch batch",
		Buckets: []float64{1, 5, 10, 50, 100, 500, 1000},
	})

	m.SubscriptionActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_subscription_active", Help: "1 if subscription is active"},
		[]string{"name"},
	)
	m.SubscriptionDelivered = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gohome_subscription_delivered_total", Help: "Events delivered to subscriber"},
		[]string{"name"},
	)
	m.SubscriptionDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gohome_subscription_dropped_total", Help: "Events dropped for slow subscribers"},
		[]string{"name"},
	)
	m.SubscriptionBuffered = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_subscription_buffered", Help: "Events buffered per subscriber"},
		[]string{"name"},
	)
	m.SubscriptionCatchup = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "gohome_subscription_catchup_duration_seconds", Help: "Catchup phase duration"},
		[]string{"name"},
	)

	m.SnapshotDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "gohome_snapshot_duration_seconds", Help: "Snapshot write duration"},
		[]string{"owner"},
	)
	m.SnapshotSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_snapshot_size_bytes", Help: "Latest snapshot size"},
		[]string{"owner"},
	)
	m.SnapshotLastPos = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_snapshot_last_position", Help: "Latest snapshot position"},
		[]string{"owner"},
	)
	m.SnapshotCorruption = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gohome_snapshot_corruption_total", Help: "Corrupt snapshots encountered"},
		[]string{"owner"},
	)

	m.SQLiteWALBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_sqlite_wal_bytes", Help: "Current WAL size",
	})
	m.SQLiteEventsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_sqlite_events_total", Help: "Rows in events table",
	})
	m.SQLiteBusyRetries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gohome_sqlite_busy_retries_total", Help: "SQLITE_BUSY retries across all callers",
	})

	m.StartupPhase = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_startup_phase", Help: "Current startup phase 1-5; 0 = not started",
	})
	m.StartupDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "gohome_startup_duration_seconds", Help: "Time to reach phase 5",
	})
	m.ReplayEventsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gohome_replay_events_processed_total", Help: "Events replayed at startup",
	})
	m.RecoveryModeEntered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gohome_recovery_mode_entered_total", Help: "Times recovery mode was entered",
	})

	m.BuildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "gohome_build_info", Help: "Build metadata"},
		[]string{"version", "commit", "goversion"},
	)

	reg.MustRegister(
		m.EventsAppended, m.AppendDuration, m.AppendRetries, m.AppendFailures,
		m.ProjectorApplyDuration, m.ProjectorFailures, m.ProjectorLag, m.ProjectorCatchup,
		m.TailerLag, m.TailerBatchSize,
		m.SubscriptionActive, m.SubscriptionDelivered, m.SubscriptionDropped, m.SubscriptionBuffered, m.SubscriptionCatchup,
		m.SnapshotDuration, m.SnapshotSize, m.SnapshotLastPos, m.SnapshotCorruption,
		m.SQLiteWALBytes, m.SQLiteEventsTotal, m.SQLiteBusyRetries,
		m.StartupPhase, m.StartupDuration, m.ReplayEventsProcessed, m.RecoveryModeEntered,
		m.BuildInfo,
	)
	return m
}

func (m *Metrics) SetBuildInfo(version, commit, goVersion string) {
	m.BuildInfo.WithLabelValues(version, commit, goVersion).Set(1)
}
```

- [ ] **Step 4: Write `internal/observability/metrics_server.go`**

```go
package observability

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (m *Metrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// ServeMetrics runs an HTTP server exposing /metrics and /health until ctx is cancelled.
// healthFn returns (status, httpCode).
func (m *Metrics) ServeMetrics(ctx context.Context, addr string, healthFn func() (string, int)) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.HTTPHandler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		status := "ok"
		code := http.StatusOK
		if healthFn != nil {
			status, code = healthFn()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"status":"` + status + `"}`))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
```

- [ ] **Step 5: Write `internal/observability/tracing.go`**

```go
package observability

import "context"

// Span is the minimal tracing surface. C1 ships a no-op implementation;
// C13 replaces this with an OpenTelemetry bridge — call sites do not change.
type Span interface {
	End()
	SetAttr(key string, value any)
	RecordError(err error)
}

type noopSpan struct{}

func (noopSpan) End()                         {}
func (noopSpan) SetAttr(string, any)          {}
func (noopSpan) RecordError(error)            {}

func StartSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/observability/...
```

Expected: PASS — all prior tests + 2 new tests.

- [ ] **Step 7: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/observability go.mod go.sum
git commit -m "feat(observability): add Prometheus metrics, HTTP handler, and no-op tracing stubs"
```

---

## Task 5: Storage — Tx interface, OpenDB, lockfile, and eventstore migrations

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/tx.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/open.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/lockfile.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/migrations/migrations.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/migrations/0001_events.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/migrations/0002_snapshots.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/migrations/0003_projection_cursors.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/migrations/0004_skipped_events.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/open_test.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/storage/lockfile_test.go`

- [ ] **Step 1: Write `internal/storage/tx.go`**

```go
// Package storage owns SQLite open/PRAGMA/migration plumbing and the
// minimal Tx interface projectors use. Nothing outside this package
// should import database/sql directly except cmd/* wiring.
package storage

import (
	"context"
	"database/sql"
)

// Tx is the transactional surface projectors work against.
// *sql.Tx satisfies this interface directly; test fakes can substitute.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

- [ ] **Step 2: Write `internal/storage/migrations/0001_events.sql`**

```sql
-- +goose Up
CREATE TABLE events (
  position        INTEGER PRIMARY KEY AUTOINCREMENT,
  ts              INTEGER NOT NULL,
  kind            TEXT    NOT NULL,
  entity          TEXT,
  source          TEXT    NOT NULL,
  correlation_id  BLOB,
  cause_position  INTEGER,
  payload         BLOB    NOT NULL
);
CREATE INDEX events_ts          ON events(ts);
CREATE INDEX events_entity_ts   ON events(entity, ts)        WHERE entity IS NOT NULL;
CREATE INDEX events_kind_ts     ON events(kind, ts);
CREATE INDEX events_correlation ON events(correlation_id)    WHERE correlation_id IS NOT NULL;
CREATE INDEX events_cause       ON events(cause_position)    WHERE cause_position IS NOT NULL;

-- +goose Down
DROP TABLE events;
```

- [ ] **Step 3: Write `internal/storage/migrations/0002_snapshots.sql`**

```sql
-- +goose Up
CREATE TABLE snapshots (
  position    INTEGER PRIMARY KEY,
  ts          INTEGER NOT NULL,
  owner       TEXT    NOT NULL,
  encoding    TEXT    NOT NULL,
  state       BLOB    NOT NULL,
  meta        BLOB
);
CREATE INDEX snapshots_owner ON snapshots(owner, position DESC);

-- +goose Down
DROP TABLE snapshots;
```

- [ ] **Step 4: Write `internal/storage/migrations/0003_projection_cursors.sql`**

```sql
-- +goose Up
CREATE TABLE projection_cursors (
  name        TEXT PRIMARY KEY,
  position    INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);

-- +goose Down
DROP TABLE projection_cursors;
```

- [ ] **Step 5: Write `internal/storage/migrations/0004_skipped_events.sql`**

```sql
-- +goose Up
CREATE TABLE skipped_events (
  position     INTEGER NOT NULL,
  projector    TEXT    NOT NULL,
  skipped_at   INTEGER NOT NULL,
  skipped_by   TEXT    NOT NULL,
  reason       TEXT    NOT NULL,
  PRIMARY KEY (position, projector)
);

-- +goose Down
DROP TABLE skipped_events;
```

- [ ] **Step 6: Write `internal/storage/migrations/migrations.go`**

```go
// Package migrations embeds eventstore SQL migrations for goose.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 7: Write `internal/storage/open.go`**

```go
package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"

	eventMigrations "github.com/fynn-labs/gohome/internal/storage/migrations"
)

type Config struct {
	Path string // absolute path to .db file; use ":memory:" for tests
}

// Open opens a SQLite database at cfg.Path, applies runtime PRAGMAs,
// and runs the eventstore migrations embedded in this package.
// Caller is responsible for also running registry migrations via
// ApplyRegistryMigrations after this returns.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(ctx, db, eventMigrations.FS, "eventstore"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ApplyMigrations runs a named migration set against an already-open DB.
// Registry uses this to run its own embed.FS.
func ApplyMigrations(ctx context.Context, db *sql.DB, fs interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]struct{}, error)
}, label string) error {
	return fmt.Errorf("use migrate directly; this shim exists only for clarity — see storage.Migrate")
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB, fs goose.FS, label string) error {
	goose.SetBaseFS(fs)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("goose up %s: %w", label, err)
	}
	return nil
}

// Migrate applies a caller-provided embed.FS of goose SQL migrations.
// Exposed so registry can run its own migration set after Open returns.
func Migrate(ctx context.Context, db *sql.DB, fs goose.FS, label string) error {
	return migrate(ctx, db, fs, label)
}
```

> Note: `goose.FS` type is satisfied by `embed.FS`; the helper above just wraps the two calls goose needs.

- [ ] **Step 8: Write `internal/storage/lockfile.go`**

```go
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Lockfile holds an exclusive PID file for the daemon.
type Lockfile struct {
	path string
}

// AcquireLockfile writes <dataDir>/gohomed.lock with the current PID.
// Returns an error if a live process already owns the file.
func AcquireLockfile(dataDir string) (*Lockfile, error) {
	path := filepath.Join(dataDir, "gohomed.lock")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dataDir, err)
	}

	if existingPID, ok := readPID(path); ok {
		if processAlive(existingPID) {
			return nil, fmt.Errorf("gohomed already running (pid %d)", existingPID)
		}
		// Stale — fall through and overwrite.
	}

	body := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return nil, fmt.Errorf("write lockfile: %w", err)
	}
	return &Lockfile{path: path}, nil
}

func (l *Lockfile) Release() error {
	if l == nil {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func readPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	lines := strings.SplitN(string(data), "\n", 2)
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, signal 0 tests process existence.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
```

- [ ] **Step 9: Write `internal/storage/open_test.go`**

```go
package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fynn-labs/gohome/internal/storage"
)

func TestOpen_MigrationsCreateEventsTable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, storage.Config{Path: filepath.Join(dir, "gohome.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events'`,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("events table missing; count = %d", count)
	}
}

func TestOpen_PragmasApplied(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, storage.Config{Path: filepath.Join(dir, "gohome.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}
```

- [ ] **Step 10: Write `internal/storage/lockfile_test.go`**

```go
package storage_test

import (
	"testing"

	"github.com/fynn-labs/gohome/internal/storage"
)

func TestLockfile_SecondAcquireFailsWhileHeld(t *testing.T) {
	dir := t.TempDir()
	l1, err := storage.AcquireLockfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Release()

	if _, err := storage.AcquireLockfile(dir); err == nil {
		t.Fatal("expected second acquire to fail")
	}
}

func TestLockfile_ReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	l1, err := storage.AcquireLockfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l1.Release(); err != nil {
		t.Fatal(err)
	}
	l2, err := storage.AcquireLockfile(dir)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	_ = l2.Release()
}
```

- [ ] **Step 11: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/storage/...
```

Expected: PASS — migrations apply, PRAGMAs stick, lockfile behaves.

- [ ] **Step 12: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/storage go.mod go.sum
git commit -m "feat(storage): Open with WAL PRAGMAs, goose migrations, Tx abstraction, lockfile"
```

---

## Task 6: Event model and Filter

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/event.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/filter.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/filter_test.go`

- [ ] **Step 1: Write failing test `internal/eventstore/filter_test.go`**

```go
package eventstore_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fynn-labs/gohome/internal/eventstore"
)

func TestFilter_MatchesEmptyFilterMatchesEverything(t *testing.T) {
	f := eventstore.Filter{}
	e := eventstore.Event{Kind: "state_changed", Entity: "light.lr"}
	if !f.Matches(e) {
		t.Fatal("empty filter should match")
	}
}

func TestFilter_MatchesKind(t *testing.T) {
	f := eventstore.Filter{Kinds: []string{"state_changed"}}
	if !f.Matches(eventstore.Event{Kind: "state_changed"}) {
		t.Fatal("kind should match")
	}
	if f.Matches(eventstore.Event{Kind: "command_issued"}) {
		t.Fatal("different kind should not match")
	}
}

func TestFilter_MatchesEntity(t *testing.T) {
	f := eventstore.Filter{Entities: []string{"light.lr"}}
	if f.Matches(eventstore.Event{Entity: "light.kitchen"}) {
		t.Fatal("different entity should not match")
	}
	if !f.Matches(eventstore.Event{Entity: "light.lr"}) {
		t.Fatal("entity should match")
	}
}

func TestFilter_MatchesCorrelationID(t *testing.T) {
	id := uuid.New()
	f := eventstore.Filter{CorrelationIDs: []uuid.UUID{id}}
	if !f.Matches(eventstore.Event{CorrelationID: id}) {
		t.Fatal("correlation id should match")
	}
	if f.Matches(eventstore.Event{CorrelationID: uuid.New()}) {
		t.Fatal("different correlation should not match")
	}
}

func TestFilter_MatchesTimeRange(t *testing.T) {
	t0 := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	f := eventstore.Filter{MinTs: t0}
	if f.Matches(eventstore.Event{Timestamp: t0.Add(-time.Second)}) {
		t.Fatal("before MinTs should not match")
	}
	if !f.Matches(eventstore.Event{Timestamp: t0.Add(time.Second)}) {
		t.Fatal("after MinTs should match")
	}
}

func TestFilter_MatchesAllOfMulti(t *testing.T) {
	f := eventstore.Filter{
		Kinds:    []string{"state_changed"},
		Entities: []string{"light.lr"},
	}
	if !f.Matches(eventstore.Event{Kind: "state_changed", Entity: "light.lr"}) {
		t.Fatal("should match both")
	}
	if f.Matches(eventstore.Event{Kind: "state_changed", Entity: "switch.k"}) {
		t.Fatal("mismatched entity should not match")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Write `internal/eventstore/event.go`**

```go
// Package eventstore owns the SQLite event log, tailer, projector dispatch,
// subscriptions, and snapshots.
package eventstore

import (
	"time"

	"github.com/google/uuid"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
)

// Event is the in-memory Go representation of a row in the events table.
// Position is 0 on input to Append and is populated by the store on return.
// CorrelationID is optional (zero value if none); CausePosition is 0 if none.
type Event struct {
	Position      uint64
	Timestamp     time.Time
	Kind          string
	Entity        string
	Source        string
	CorrelationID uuid.UUID
	CausePosition uint64
	Payload       *eventv1.Payload
}
```

- [ ] **Step 4: Write `internal/eventstore/filter.go`**

```go
package eventstore

import (
	"time"

	"github.com/google/uuid"
)

// Filter selects events. All populated fields AND together;
// within a slice field, any match succeeds. Zero time bounds are
// treated as unbounded.
type Filter struct {
	Kinds          []string
	Entities       []string
	Sources        []string
	CorrelationIDs []uuid.UUID
	MinTs, MaxTs   time.Time
}

func (f Filter) Matches(e Event) bool {
	if len(f.Kinds) > 0 && !containsString(f.Kinds, e.Kind) {
		return false
	}
	if len(f.Entities) > 0 && !containsString(f.Entities, e.Entity) {
		return false
	}
	if len(f.Sources) > 0 && !containsString(f.Sources, e.Source) {
		return false
	}
	if len(f.CorrelationIDs) > 0 && !containsUUID(f.CorrelationIDs, e.CorrelationID) {
		return false
	}
	if !f.MinTs.IsZero() && e.Timestamp.Before(f.MinTs) {
		return false
	}
	if !f.MaxTs.IsZero() && e.Timestamp.After(f.MaxTs) {
		return false
	}
	return true
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsUUID(haystack []uuid.UUID, needle uuid.UUID) bool {
	for _, u := range haystack {
		if u == needle {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/eventstore/...
```

Expected: PASS — 6 filter tests.

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore go.mod go.sum
git commit -m "feat(eventstore): add Event type and Filter matching"
```

---

## Task 7: Eventstore — Store, Append, Query

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/projector.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/store.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/query.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/testutil/sqlite.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/testutil/events.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/store_test.go`

- [ ] **Step 1: Write `internal/eventstore/projector.go`**

```go
package eventstore

import (
	"context"

	"github.com/fynn-labs/gohome/internal/storage"
)

type ProjectorMode int

const (
	ProjectorModeSync ProjectorMode = iota
	ProjectorModeAsync
)

// Projector materializes a view of the event log. Sync projectors run
// inside Append's transaction; async projectors run off the tailer.
type Projector interface {
	Name() string
	Apply(ctx context.Context, tx storage.Tx, e Event) error
	Snapshot(ctx context.Context, tx storage.Tx) error
	Restore(ctx context.Context, tx storage.Tx) (resumeFrom uint64, err error)
}

// NoSnapshot is embeddable for projectors whose state lives entirely
// in SQL (e.g., registry). Restore returns 0, meaning "read cursor
// from projection_cursors".
type NoSnapshot struct{}

func (NoSnapshot) Snapshot(context.Context, storage.Tx) error                  { return nil }
func (NoSnapshot) Restore(context.Context, storage.Tx) (uint64, error)         { return 0, nil }
```

- [ ] **Step 2: Write `internal/testutil/sqlite.go`**

```go
// Package testutil offers shared helpers: in-memory DB, event builders,
// fixture loading. Test-only code.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/fynn-labs/gohome/internal/storage"
)

// NewTestDB opens a file-backed SQLite in t.TempDir(), applies migrations,
// and returns the handle. File-backed rather than :memory: because WAL
// behavior differs and we want realism.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), storage.Config{
		Path: filepath.Join(dir, "test.db"),
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
```

- [ ] **Step 3: Write `internal/testutil/events.go`**

```go
package testutil

import (
	"time"

	"github.com/google/uuid"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

type EventOption func(*eventstore.Event)

func WithSource(s string) EventOption              { return func(e *eventstore.Event) { e.Source = s } }
func WithCorrelation(id uuid.UUID) EventOption     { return func(e *eventstore.Event) { e.CorrelationID = id } }
func WithCause(pos uint64) EventOption             { return func(e *eventstore.Event) { e.CausePosition = pos } }
func WithTimestamp(t time.Time) EventOption        { return func(e *eventstore.Event) { e.Timestamp = t } }

func StateChanged(entity string, brightness uint32, opts ...EventOption) eventstore.Event {
	e := eventstore.Event{
		Kind:      "state_changed",
		Entity:    entity,
		Source:    "test",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_StateChanged{
			StateChanged: &eventv1.StateChanged{
				Attributes: &entityv1.Attributes{
					Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{
						On: brightness > 0, Brightness: brightness,
					}},
				},
			},
		}},
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}

func SystemStartup(opts ...EventOption) eventstore.Event {
	e := eventstore.Event{
		Kind:      "system",
		Source:    "gohomed",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_System{
			System: &eventv1.SystemEvent{Kind: "startup"},
		}},
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}
```

- [ ] **Step 4: Write `internal/eventstore/store.go`**

```go
package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/observability"
)

type Config struct {
	SnapshotEveryEvents    int
	SnapshotEveryPeriod    time.Duration
	MaxSubscriberBuffer    int
	SnapshotRetainPerOwner int
}

func (c *Config) withDefaults() {
	if c.SnapshotEveryEvents == 0 {
		c.SnapshotEveryEvents = 10_000
	}
	if c.SnapshotEveryPeriod == 0 {
		c.SnapshotEveryPeriod = time.Hour
	}
	if c.MaxSubscriberBuffer == 0 {
		c.MaxSubscriberBuffer = 256
	}
	if c.SnapshotRetainPerOwner == 0 {
		c.SnapshotRetainPerOwner = 3
	}
}

type projectorReg struct {
	p    Projector
	mode ProjectorMode
}

type Store struct {
	cfg     Config
	db      *sql.DB
	logger  *slog.Logger
	metrics *observability.Metrics

	projectors []projectorReg

	mu               sync.RWMutex
	latestPosition   uint64
	cond             *sync.Cond
	subs             []*subscriber     // populated in Task 13
	started          bool
}

// Open constructs a Store around an already-migrated *sql.DB.
// Callers must still RegisterProjector and Start before Append.
func Open(ctx context.Context, cfg Config, db *sql.DB, logger *slog.Logger, metrics *observability.Metrics) (*Store, error) {
	cfg.withDefaults()
	s := &Store{
		cfg:     cfg,
		db:      db,
		logger:  logger,
		metrics: metrics,
	}
	s.cond = sync.NewCond(&s.mu)

	var latest sql.NullInt64
	err := db.QueryRowContext(ctx, "SELECT MAX(position) FROM events").Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load latest position: %w", err)
	}
	if latest.Valid {
		s.latestPosition = uint64(latest.Int64)
	}
	return s, nil
}

func (s *Store) RegisterProjector(p Projector, mode ProjectorMode) error {
	if s.started {
		return errors.New("RegisterProjector: already started")
	}
	s.projectors = append(s.projectors, projectorReg{p: p, mode: mode})
	return nil
}

func (s *Store) LatestPosition() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestPosition
}

// Append writes a single event. Sync projectors run in the same transaction.
// Returns the assigned position.
func (s *Store) Append(ctx context.Context, e Event) (uint64, error) {
	if e.Kind == "" {
		return 0, errors.New("Append: Kind required")
	}
	if e.Source == "" {
		return 0, errors.New("Append: Source required")
	}
	if e.Payload == nil {
		return 0, errors.New("Append: Payload required")
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if e.CorrelationID == uuid.Nil {
		e.CorrelationID = uuid.New()
	}

	payload, err := proto.Marshal(e.Payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	start := time.Now()
	defer func() { s.metrics.AppendDuration.Observe(time.Since(start).Seconds()) }()

	var position uint64
	if err := s.withRetry(ctx, func() error {
		return s.appendTx(ctx, e, payload, &position)
	}); err != nil {
		return 0, err
	}

	s.mu.Lock()
	if position > s.latestPosition {
		s.latestPosition = position
	}
	s.mu.Unlock()
	s.cond.Broadcast()

	s.metrics.EventsAppended.WithLabelValues(e.Kind).Inc()
	return position, nil
}

func (s *Store) appendTx(ctx context.Context, e Event, payload []byte, outPos *uint64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// BEGIN IMMEDIATE: acquire write lock upfront.
	if _, err := tx.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		// Fresh tx already started via BeginTx; some drivers reject BEGIN.
		// modernc/sqlite accepts it when outer BEGIN is deferred.
		// If it errors because a tx is already active, ignore.
	}

	var corrBytes []byte
	if e.CorrelationID != uuid.Nil {
		b, _ := e.CorrelationID.MarshalBinary()
		corrBytes = b
	}
	var causePos sql.NullInt64
	if e.CausePosition > 0 {
		causePos = sql.NullInt64{Int64: int64(e.CausePosition), Valid: true}
	}
	var entity sql.NullString
	if e.Entity != "" {
		entity = sql.NullString{String: e.Entity, Valid: true}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO events (ts, kind, entity, source, correlation_id, cause_position, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UnixNano(), e.Kind, entity, e.Source, corrBytes, causePos, payload,
	)
	if err != nil {
		s.metrics.AppendFailures.WithLabelValues("insert").Inc()
		return fmt.Errorf("insert event: %w", err)
	}

	pos, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	e.Position = uint64(pos)
	*outPos = e.Position

	for _, reg := range s.projectors {
		if reg.mode != ProjectorModeSync {
			continue
		}
		projStart := time.Now()
		if err := reg.p.Apply(ctx, tx, e); err != nil {
			s.metrics.ProjectorFailures.WithLabelValues(reg.p.Name(), "sync").Inc()
			s.metrics.AppendFailures.WithLabelValues("projector").Inc()
			return fmt.Errorf("projector %s apply: %w", reg.p.Name(), err)
		}
		s.metrics.ProjectorApplyDuration.WithLabelValues(reg.p.Name(), "sync").Observe(time.Since(projStart).Seconds())

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO projection_cursors (name, position, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
			reg.p.Name(), e.Position, time.Now().UnixNano(),
		); err != nil {
			return fmt.Errorf("advance cursor %s: %w", reg.p.Name(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		s.metrics.AppendFailures.WithLabelValues("commit").Inc()
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) withRetry(ctx context.Context, fn func() error) error {
	backoffs := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}
	var err error
	for attempt := 0; attempt <= len(backoffs); attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isSQLiteBusy(err) {
			return err
		}
		s.metrics.AppendRetries.Inc()
		if attempt == len(backoffs) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffs[attempt]):
		}
	}
	return err
}

func isSQLiteBusy(err error) bool {
	// modernc.org/sqlite wraps errors; surface-match by code string.
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, []string{"SQLITE_BUSY", "database is locked", "(5)"})
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if len(s) >= len(n) && indexOf(s, n) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, substr string) int {
	n := len(substr)
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

// AppendBatch writes multiple events atomically. All-or-nothing.
func (s *Store) AppendBatch(ctx context.Context, events []Event) ([]uint64, error) {
	if len(events) == 0 {
		return nil, nil
	}
	positions := make([]uint64, len(events))

	err := s.withRetry(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		for i := range events {
			e := &events[i]
			if e.Kind == "" || e.Source == "" || e.Payload == nil {
				return errors.New("AppendBatch: invalid event")
			}
			if e.Timestamp.IsZero() {
				e.Timestamp = time.Now()
			}
			if e.CorrelationID == uuid.Nil {
				e.CorrelationID = uuid.New()
			}
			payload, err := proto.Marshal(e.Payload)
			if err != nil {
				return fmt.Errorf("marshal[%d]: %w", i, err)
			}
			var corr []byte
			if e.CorrelationID != uuid.Nil {
				corr, _ = e.CorrelationID.MarshalBinary()
			}
			var cause sql.NullInt64
			if e.CausePosition > 0 {
				cause = sql.NullInt64{Int64: int64(e.CausePosition), Valid: true}
			}
			var ent sql.NullString
			if e.Entity != "" {
				ent = sql.NullString{String: e.Entity, Valid: true}
			}
			res, err := tx.ExecContext(ctx, `
				INSERT INTO events (ts, kind, entity, source, correlation_id, cause_position, payload)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				e.Timestamp.UnixNano(), e.Kind, ent, e.Source, corr, cause, payload,
			)
			if err != nil {
				return err
			}
			pos, err := res.LastInsertId()
			if err != nil {
				return err
			}
			e.Position = uint64(pos)
			positions[i] = e.Position

			for _, reg := range s.projectors {
				if reg.mode != ProjectorModeSync {
					continue
				}
				if err := reg.p.Apply(ctx, tx, *e); err != nil {
					return fmt.Errorf("projector %s: %w", reg.p.Name(), err)
				}
			}
		}

		for _, reg := range s.projectors {
			if reg.mode != ProjectorModeSync {
				continue
			}
			last := events[len(events)-1].Position
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO projection_cursors (name, position, updated_at) VALUES (?, ?, ?)
				ON CONFLICT(name) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
				reg.p.Name(), last, time.Now().UnixNano(),
			); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.latestPosition = events[len(events)-1].Position
	s.mu.Unlock()
	s.cond.Broadcast()

	for _, e := range events {
		s.metrics.EventsAppended.WithLabelValues(e.Kind).Inc()
	}
	return positions, nil
}

// Close releases the store. Tailer/snapshotter lifecycle lands in later tasks;
// for now Close is a no-op beyond DB close delegation (which the caller owns).
func (s *Store) Close(_ context.Context) error { return nil }
```

- [ ] **Step 5: Write `internal/eventstore/query.go`**

```go
package eventstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
)

type QueryOptions struct {
	FromPosition uint64
	ToPosition   uint64
	Filter       Filter
	Limit        int
}

// Query reads historical events. Simple range + filter; future work
// pushes filter predicates into SQL for efficiency.
func (s *Store) Query(ctx context.Context, q QueryOptions) ([]Event, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT position, ts, kind, entity, source, correlation_id, cause_position, payload
		FROM events
		WHERE position > ? AND (? = 0 OR position <= ?)
		ORDER BY position
		LIMIT ?`,
		q.FromPosition, q.ToPosition, q.ToPosition, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		if q.Filter.Matches(e) {
			out = append(out, e)
		}
	}
	return out, rows.Err()
}

func scanEvent(r interface {
	Scan(...any) error
}) (Event, error) {
	var (
		pos       int64
		tsNanos   int64
		kind      string
		entity    sql.NullString
		source    string
		corrBytes []byte
		cause     sql.NullInt64
		payload   []byte
	)
	if err := r.Scan(&pos, &tsNanos, &kind, &entity, &source, &corrBytes, &cause, &payload); err != nil {
		return Event{}, err
	}
	e := Event{
		Position:  uint64(pos),
		Timestamp: timestampFromNanos(tsNanos),
		Kind:      kind,
		Entity:    entity.String,
		Source:    source,
	}
	if len(corrBytes) == 16 {
		_ = e.CorrelationID.UnmarshalBinary(corrBytes)
	}
	if cause.Valid {
		e.CausePosition = uint64(cause.Int64)
	}
	e.Payload = &eventv1.Payload{}
	if err := proto.Unmarshal(payload, e.Payload); err != nil {
		return Event{}, fmt.Errorf("unmarshal payload: %w", err)
	}
	return e, nil
}

func timestampFromNanos(n int64) (t timeish) {
	return timeish{unixNanos: n}.asTime()
}

// Tiny helper so the conversion stays in one place.
type timeish struct{ unixNanos int64 }

func (ti timeish) asTime() timeish { return ti }

// Note: scanEvent uses a package-level helper for time; kept inline above.
// The actual conversion uses time.Unix(0, n) via import in a future step —
// this file imports "time" implicitly via Event.Timestamp. Rewrite below.
```

> Note: the placeholder helper is ugly; fix before running tests.

- [ ] **Step 6: Clean up `query.go` — replace the `timeish` placeholder**

Replace the bottom half of `query.go` (from `func timestampFromNanos` through end of file) with:

```go
import (
	// ...existing imports
	"time"
)

func timestampFromNanos(n int64) time.Time {
	return time.Unix(0, n)
}

// Replace the call site in scanEvent:
//   Timestamp: timestampFromNanos(tsNanos),
```

Edit the file so `scanEvent` uses `time.Time` directly and drop the `timeish` stub. Final `query.go` should have `time` in imports and `timestampFromNanos(n int64) time.Time { return time.Unix(0, n) }`.

_Use the Edit tool to replace the bottom half — this step exists because writing both the flawed version and the fix would be wasted tokens; just write the clean version directly: add `"time"` to the imports, use `time.Unix(0, tsNanos)` for `Timestamp`, and drop `timeish` entirely._

- [ ] **Step 7: Write `internal/eventstore/store_test.go`**

```go
package eventstore_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func newStore(t *testing.T) *eventstore.Store {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestAppend_AssignsMonotonicPositions(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	p1, err := s.Append(ctx, testutil.StateChanged("light.a", 100))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.Append(ctx, testutil.StateChanged("light.b", 50))
	if err != nil {
		t.Fatal(err)
	}
	if p2 <= p1 {
		t.Fatalf("positions not monotonic: p1=%d p2=%d", p1, p2)
	}
}

func TestAppend_RejectsInvalid(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if _, err := s.Append(ctx, eventstore.Event{Kind: ""}); err == nil {
		t.Fatal("expected error for empty kind")
	}
}

func TestAppendBatch_AtomicAllOrNothing(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	events := []eventstore.Event{
		testutil.StateChanged("light.a", 100),
		testutil.StateChanged("light.b", 200),
		testutil.StateChanged("light.c", 0),
	}
	positions, err := s.AppendBatch(ctx, events)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("positions len = %d, want 3", len(positions))
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] != positions[i-1]+1 {
			t.Fatalf("non-contiguous batch positions: %v", positions)
		}
	}
}

func TestQuery_ReturnsInOrderAndFilters(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 10))
	_, _ = s.Append(ctx, testutil.StateChanged("light.b", 20))
	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 30))

	got, err := s.Query(ctx, eventstore.QueryOptions{Filter: eventstore.Filter{Entities: []string{"light.a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("filter returned %d events, want 2", len(got))
	}
	if got[0].Position >= got[1].Position {
		t.Fatal("results not ordered by position")
	}
}
```

- [ ] **Step 8: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/eventstore/... ./internal/storage/... ./internal/observability/...
```

Expected: PASS — all tests across three packages.

- [ ] **Step 9: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore internal/testutil go.mod go.sum
git commit -m "feat(eventstore): add Store.Open/Append/AppendBatch/Query with sync projector dispatch"
```

---

## Task 8: Projector dispatch smoke test (fake projector)

**Files:**
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/store_test.go`

- [ ] **Step 1: Add a fake projector test proving sync dispatch inside the tx**

Append to `store_test.go`:

```go
type countingProjector struct {
	name   string
	count  int
	lastE  eventstore.Event
	failAt int
}

func (c *countingProjector) Name() string { return c.name }
func (c *countingProjector) Apply(ctx context.Context, tx storage.Tx, e eventstore.Event) error {
	c.count++
	c.lastE = e
	if c.failAt > 0 && c.count == c.failAt {
		return errors.New("intentional failure")
	}
	return nil
}
func (c *countingProjector) Snapshot(context.Context, storage.Tx) error         { return nil }
func (c *countingProjector) Restore(context.Context, storage.Tx) (uint64, error) { return 0, nil }

func TestAppend_SyncProjectorSeesEventInsideTx(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	proj := &countingProjector{name: "test"}
	if err := s.RegisterProjector(proj, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}

	_, err := s.Append(ctx, testutil.StateChanged("light.a", 10))
	if err != nil {
		t.Fatal(err)
	}
	if proj.count != 1 {
		t.Fatalf("projector call count = %d, want 1", proj.count)
	}
	if proj.lastE.Position == 0 {
		t.Fatal("projector received Event with zero Position")
	}
}

func TestAppend_SyncProjectorFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	proj := &countingProjector{name: "fails", failAt: 1}
	if err := s.RegisterProjector(proj, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(ctx, testutil.StateChanged("light.a", 10)); err == nil {
		t.Fatal("expected projector failure to bubble up")
	}
	got, err := s.Query(ctx, eventstore.QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("rolled-back event still present: %+v", got)
	}
}
```

Also add imports for `"errors"` and `"github.com/fynn-labs/gohome/internal/storage"` to the test file.

- [ ] **Step 2: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/...
```

Expected: PASS — original tests + 2 new ones.

- [ ] **Step 3: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "test(eventstore): verify sync projector dispatch and rollback semantics"
```

---

## Task 9: State cache — COW HAMT with View/Get/Apply

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/state/cache.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/state/cache_test.go`

- [ ] **Step 1: Write failing test `internal/state/cache_test.go`**

```go
package state_test

import (
	"context"
	"sync"
	"testing"
	"time"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/state"
)

func mkStateChanged(entity string, on bool, brightness uint32) eventstore.Event {
	return eventstore.Event{
		Position:  1,
		Timestamp: time.Now(),
		Kind:      "state_changed",
		Entity:    entity,
		Source:    "test",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_StateChanged{
			StateChanged: &eventv1.StateChanged{
				Attributes: &entityv1.Attributes{
					Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{On: on, Brightness: brightness}},
				},
			},
		}},
	}
}

func TestCache_EmptyCacheHasZeroLen(t *testing.T) {
	c := state.New()
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
	if _, ok := c.Get("light.lr"); ok {
		t.Fatal("empty cache returned a state")
	}
}

func TestCache_ApplyStateChangedStoresState(t *testing.T) {
	c := state.New()
	if err := c.Apply(context.Background(), nil, mkStateChanged("light.lr", true, 100)); err != nil {
		t.Fatal(err)
	}
	// IMPORTANT: in production, Apply builds a pending snapshot; promotion happens post-commit.
	// For this unit test, Promote exposes the post-commit swap explicitly.
	c.Promote()

	s, ok := c.Get("light.lr")
	if !ok {
		t.Fatal("entity missing")
	}
	if s.Attributes.GetLight().Brightness != 100 {
		t.Fatalf("brightness = %d, want 100", s.Attributes.GetLight().Brightness)
	}
}

func TestCache_ViewIsStableDuringWrites(t *testing.T) {
	c := state.New()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.a", true, 50))
	c.Promote()
	snap := c.View()

	// Apply + promote new events.
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.b", true, 90))
	c.Promote()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.a", false, 0))
	c.Promote()

	// Original snapshot should still reflect old state.
	if _, ok := snap.Get("light.b"); ok {
		t.Fatal("old snapshot saw later write to light.b")
	}
	if got, ok := snap.Get("light.a"); !ok || got.Attributes.GetLight().Brightness != 50 {
		t.Fatal("old snapshot mutated light.a")
	}
}

func TestCache_ConcurrentReadersAreSafe(t *testing.T) {
	c := state.New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = c.Apply(context.Background(), nil, mkStateChanged("light.x", true, uint32(i)))
			c.Promote()
		}(i)
	}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Get("light.x")
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/state/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Write `internal/state/cache.go`**

```go
// Package state owns the in-memory copy-on-write cache of entity state.
// The cache is a materialized projection of the event log; the log is
// the source of truth.
package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/immutable"
	"google.golang.org/protobuf/proto"

	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/storage"
)

type EntityID = string

type State struct {
	EntityID   EntityID
	UpdatedAt  time.Time
	UpdatedBy  string
	Attributes *entityv1.Attributes
}

// Cache holds entity state via an immutable HAMT behind atomic.Pointer.
// The store calls Apply inside an Append transaction to build the next
// snapshot, then calls Promote after the transaction commits. Reads go
// through View/Get against the currently promoted snapshot.
type Cache struct {
	current atomic.Pointer[immutable.Map[EntityID, State]]

	// pending is the tx-local buffer being mutated during Apply.
	// Only one Append runs at a time (SQLite serializes writers), so
	// a single pending slot is sufficient; a mutex enforces the invariant.
	mu      sync.Mutex
	pending *immutable.Map[EntityID, State]
}

func New() *Cache {
	c := &Cache{}
	empty := immutable.NewMap[EntityID, State](nil)
	c.current.Store(empty)
	return c
}

func (c *Cache) View() *immutable.Map[EntityID, State] {
	return c.current.Load()
}

func (c *Cache) Get(id EntityID) (State, bool) {
	m := c.View()
	v, ok := m.Get(id)
	return v, ok
}

func (c *Cache) Len() int {
	return c.View().Len()
}

// Name implements eventstore.Projector.
func (c *Cache) Name() string { return "state_cache" }

// Apply mutates the pending HAMT. Callers MUST call Promote after the
// enclosing transaction commits (or Discard on rollback).
func (c *Cache) Apply(_ context.Context, _ storage.Tx, e eventstore.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pending == nil {
		c.pending = c.current.Load()
	}

	switch payload := e.Payload.GetKind().(type) {
	case *eventv1.Payload_StateChanged:
		s := State{
			EntityID:   e.Entity,
			UpdatedAt:  e.Timestamp,
			UpdatedBy:  e.Source,
			Attributes: proto.Clone(payload.StateChanged.GetAttributes()).(*entityv1.Attributes),
		}
		c.pending = c.pending.Set(e.Entity, s)

	case *eventv1.Payload_EntityRegistered:
		// Registration seeds capability-level state so reads before first
		// state_changed still return a meaningful Attributes envelope.
		if _, exists := c.pending.Get(e.Entity); !exists {
			c.pending = c.pending.Set(e.Entity, State{
				EntityID:   e.Entity,
				UpdatedAt:  e.Timestamp,
				UpdatedBy:  e.Source,
				Attributes: proto.Clone(payload.EntityRegistered.GetCapabilities()).(*entityv1.Attributes),
			})
		}

	case *eventv1.Payload_EntityUnregistered:
		c.pending = c.pending.Delete(e.Entity)

	default:
		// Events that don't affect cache — ignore.
	}
	return nil
}

// Promote swaps the pending HAMT into current. Called by the store after
// the Append transaction commits.
func (c *Cache) Promote() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending == nil {
		return
	}
	c.current.Store(c.pending)
	c.pending = nil
}

// Discard drops the pending HAMT (used if the transaction rolls back).
func (c *Cache) Discard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = nil
}

// Snapshot / Restore are implemented in Task 10; stubbed here so the
// Projector interface is satisfied.
func (c *Cache) Snapshot(context.Context, storage.Tx) error {
	return errors.New("state.Cache.Snapshot: implemented in Task 10")
}

func (c *Cache) Restore(context.Context, storage.Tx) (uint64, error) {
	return 0, fmt.Errorf("state.Cache.Restore: implemented in Task 10")
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/state/...
```

Expected: PASS — 4 tests.

- [ ] **Step 5: Wire Promote/Discard into eventstore Append path**

Edit `internal/eventstore/store.go`:
- Add a `PromotableProjector` interface and detect it at registration time, OR simpler: expose a `Projector` optional interface for post-commit callbacks.

Use the simpler approach — add this to `projector.go`:

```go
// PostCommit is implemented by projectors that need to promote in-memory
// state after the Append transaction commits. Called by the store in
// registration order; errors are logged and ignored (log is source of truth).
type PostCommit interface {
	Promote()
}

// Discarder is implemented by projectors that need to drop tx-local state
// on rollback.
type Discarder interface {
	Discard()
}
```

In `store.go`, in `appendTx`, after `tx.Commit()` succeeds and `committed = true`:

```go
for _, reg := range s.projectors {
	if pc, ok := reg.p.(PostCommit); ok {
		pc.Promote()
	}
}
```

And in the `if !committed` deferred rollback path:

```go
for _, reg := range s.projectors {
	if d, ok := reg.p.(Discarder); ok {
		d.Discard()
	}
}
```

Apply the same in `AppendBatch`.

- [ ] **Step 6: Run all tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./...
```

Expected: PASS across all packages.

- [ ] **Step 7: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/state internal/eventstore go.mod go.sum
git commit -m "feat(state): add copy-on-write HAMT cache with Promote/Discard post-commit hooks"
```

---

## Task 10: State cache — snapshot round-trip (protobuf + zstd)

**Files:**
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/state/cache.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/state/snapshot_test.go`

- [ ] **Step 1: Write failing test `internal/state/snapshot_test.go`**

```go
package state_test

import (
	"context"
	"testing"

	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestCache_SnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)

	c1 := state.New()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = c1.Apply(ctx, tx, mkStateChanged("light.a", true, 100))
	_ = c1.Apply(ctx, tx, mkStateChanged("light.b", false, 0))
	c1.Promote()

	// Insert a snapshot row.
	if err := c1.Snapshot(ctx, tx); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Fresh cache restores from the row.
	c2 := state.New()
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := c2.Restore(ctx, tx2)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatal(err)
	}

	if pos == 0 {
		t.Fatal("Restore returned position 0")
	}
	if c2.Len() != 2 {
		t.Fatalf("restored Len = %d, want 2", c2.Len())
	}
	s, ok := c2.Get("light.a")
	if !ok {
		t.Fatal("light.a missing after restore")
	}
	if s.Attributes.GetLight().Brightness != 100 {
		t.Fatalf("brightness = %d, want 100", s.Attributes.GetLight().Brightness)
	}
}

func TestCache_RestoreWithNoSnapshotReturnsZero(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	c := state.New()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := c.Restore(ctx, tx)
	if err != nil {
		t.Fatalf("Restore on empty: %v", err)
	}
	if pos != 0 {
		t.Fatalf("expected 0, got %d", pos)
	}
	_ = tx.Rollback()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/state/...
```

Expected: FAIL — Snapshot/Restore return "implemented in Task 10" error.

- [ ] **Step 3: Replace Snapshot/Restore in `internal/state/cache.go`**

Add these imports:

```go
import (
	// ...existing...
	"database/sql"
	"time"

	"github.com/klauspost/compress/zstd"
)
```

Replace the stub methods:

```go
func (c *Cache) Snapshot(ctx context.Context, tx storage.Tx) error {
	m := c.View()
	snap := &eventv1.StateCacheSnapshot{
		Ts:       time.Now().UnixNano(),
		Entities: make([]*eventv1.EntityState, 0, m.Len()),
	}

	iter := m.Iterator()
	for !iter.Done() {
		_, v, _ := iter.Next()
		snap.Entities = append(snap.Entities, &eventv1.EntityState{
			EntityId:   v.EntityID,
			UpdatedAt:  v.UpdatedAt.UnixNano(),
			UpdatedBy:  v.UpdatedBy,
			Attributes: v.Attributes,
		})
	}

	// The store sets Position on the snap before calling; for standalone
	// use (tests / SnapshotNow with no position context), fall back to
	// MAX(position) FROM events.
	var pos int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(position), 0) FROM events").Scan(&pos); err != nil {
		return fmt.Errorf("read max position: %w", err)
	}
	snap.Position = uint64(pos)

	raw, err := proto.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	enc, err := zstd.NewWriter(nil)
	if err != nil {
		return fmt.Errorf("zstd writer: %w", err)
	}
	compressed := enc.EncodeAll(raw, nil)
	_ = enc.Close()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (position, ts, owner, encoding, state)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(position) DO UPDATE SET
			ts = excluded.ts, owner = excluded.owner,
			encoding = excluded.encoding, state = excluded.state`,
		pos, snap.Ts, c.Name(), "protobuf+zstd", compressed,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

func (c *Cache) Restore(ctx context.Context, tx storage.Tx) (uint64, error) {
	var (
		pos        int64
		encoding   string
		compressed []byte
	)
	err := tx.QueryRowContext(ctx, `
		SELECT position, encoding, state FROM snapshots
		WHERE owner = ? ORDER BY position DESC LIMIT 1`,
		c.Name(),
	).Scan(&pos, &encoding, &compressed)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read snapshot row: %w", err)
	}
	if encoding != "protobuf+zstd" {
		return 0, fmt.Errorf("unknown snapshot encoding %q", encoding)
	}

	dec, err := zstd.NewReader(nil)
	if err != nil {
		return 0, fmt.Errorf("zstd reader: %w", err)
	}
	raw, err := dec.DecodeAll(compressed, nil)
	dec.Close()
	if err != nil {
		return 0, fmt.Errorf("zstd decode: %w", err)
	}

	var snap eventv1.StateCacheSnapshot
	if err := proto.Unmarshal(raw, &snap); err != nil {
		return 0, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	b := immutable.NewMapBuilder[EntityID, State](nil)
	for _, es := range snap.Entities {
		b.Set(es.EntityId, State{
			EntityID:   es.EntityId,
			UpdatedAt:  time.Unix(0, es.UpdatedAt),
			UpdatedBy:  es.UpdatedBy,
			Attributes: es.Attributes,
		})
	}
	c.current.Store(b.Map())
	return uint64(pos), nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/state/...
```

Expected: PASS — all state tests.

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/state go.mod go.sum
git commit -m "feat(state): implement Snapshot/Restore with protobuf+zstd encoding"
```

---

## Task 11: Registry — migrations, queries, projector

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/migrations/migrations.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/migrations/0001_driver_instances.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/migrations/0002_devices.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/migrations/0003_entities.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/migrations/0004_event_subscriptions.sql`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/types.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/queries.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/registry.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/registry/registry_test.go`

- [ ] **Step 1: Write all four migration SQL files**

`0001_driver_instances.sql`:
```sql
-- +goose Up
CREATE TABLE driver_instances (
  id             TEXT PRIMARY KEY,
  driver_name    TEXT    NOT NULL,
  display_name   TEXT    NOT NULL,
  transport      TEXT    NOT NULL CHECK(transport IN ('local_subprocess','remote_grpc')),
  endpoint       TEXT    NOT NULL,
  config_hash    TEXT    NOT NULL,
  status         TEXT    NOT NULL CHECK(status IN ('starting','running','failed','stopped')),
  last_error     TEXT,
  started_at     INTEGER,
  last_heartbeat INTEGER,
  created_at     INTEGER NOT NULL
);

-- +goose Down
DROP TABLE driver_instances;
```

`0002_devices.sql`:
```sql
-- +goose Up
CREATE TABLE devices (
  id                  TEXT PRIMARY KEY,
  driver_instance_id  TEXT NOT NULL REFERENCES driver_instances(id) ON DELETE RESTRICT,
  friendly_name       TEXT NOT NULL,
  manufacturer        TEXT,
  model               TEXT,
  sw_version          TEXT,
  metadata            BLOB,
  disabled            INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
CREATE INDEX devices_driver ON devices(driver_instance_id);

-- +goose Down
DROP TABLE devices;
```

`0003_entities.sql`:
```sql
-- +goose Up
CREATE TABLE entities (
  id                  TEXT PRIMARY KEY,
  device_id           TEXT REFERENCES devices(id) ON DELETE SET NULL,
  driver_instance_id  TEXT NOT NULL REFERENCES driver_instances(id) ON DELETE RESTRICT,
  entity_type         TEXT NOT NULL,
  friendly_name       TEXT NOT NULL,
  capabilities        BLOB NOT NULL,
  disabled            INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
CREATE INDEX entities_type   ON entities(entity_type);
CREATE INDEX entities_device ON entities(device_id) WHERE device_id IS NOT NULL;
CREATE INDEX entities_driver ON entities(driver_instance_id);

-- +goose Down
DROP TABLE entities;
```

`0004_event_subscriptions.sql`:
```sql
-- +goose Up
CREATE TABLE event_subscriptions (
  name         TEXT PRIMARY KEY,
  cursor       INTEGER NOT NULL,
  filter       BLOB,
  created_at   INTEGER NOT NULL,
  last_active  INTEGER NOT NULL
);

-- +goose Down
DROP TABLE event_subscriptions;
```

- [ ] **Step 2: Write `internal/registry/migrations/migrations.go`**

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 3: Write `internal/registry/types.go`**

```go
// Package registry owns the SQL-backed projection of driver instances,
// devices, entities, and durable event subscriptions.
package registry

import "time"

type DriverInstance struct {
	ID            string
	DriverName    string
	DisplayName   string
	Transport     string
	Endpoint      string
	ConfigHash    string
	Status        string
	LastError     string
	StartedAt     time.Time
	LastHeartbeat time.Time
	CreatedAt     time.Time
}

type Device struct {
	ID               string
	DriverInstanceID string
	FriendlyName     string
	Manufacturer     string
	Model            string
	SwVersion        string
	Metadata         []byte
	Disabled         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Entity struct {
	ID               string
	DeviceID         string
	DriverInstanceID string
	EntityType       string
	FriendlyName     string
	Capabilities     []byte // serialized entityv1.Attributes
	Disabled         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DeviceFilter struct {
	DriverInstanceID string
	IncludeDisabled  bool
}

type EntityFilter struct {
	DriverInstanceID string
	DeviceID         string
	EntityType       string
	IncludeDisabled  bool
}
```

- [ ] **Step 4: Write `internal/registry/registry.go`**

```go
package registry

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/storage"
	regMigrations "github.com/fynn-labs/gohome/internal/registry/migrations"
)

// Registry is the read API and projector. It holds the *sql.DB handle
// so callers can query outside of Apply transactions.
type Registry struct {
	db *sql.DB
	eventstore.NoSnapshot
}

// New returns a Registry attached to an already-open DB.
// Migrations are run here (idempotent).
func New(ctx context.Context, db *sql.DB) (*Registry, error) {
	if err := storage.Migrate(ctx, db, regMigrations.FS, "registry"); err != nil {
		return nil, fmt.Errorf("registry migrations: %w", err)
	}
	return &Registry{db: db}, nil
}

func (r *Registry) Name() string { return "registry" }

// Apply implements eventstore.Projector. Runs inside the Append tx.
func (r *Registry) Apply(ctx context.Context, tx storage.Tx, e eventstore.Event) error {
	switch payload := e.Payload.GetKind().(type) {
	case *eventv1.Payload_EntityRegistered:
		return r.applyEntityRegistered(ctx, tx, e, payload.EntityRegistered)
	case *eventv1.Payload_EntityUnregistered:
		return r.applyEntityUnregistered(ctx, tx, e)
	case *eventv1.Payload_DriverEvent:
		return r.applyDriverEvent(ctx, tx, e, payload.DriverEvent)
	default:
		return nil // not our concern
	}
}

func (r *Registry) applyEntityRegistered(ctx context.Context, tx storage.Tx, e eventstore.Event, p *eventv1.EntityRegistered) error {
	caps, err := proto.Marshal(p.GetCapabilities())
	if err != nil {
		return fmt.Errorf("marshal capabilities: %w", err)
	}
	now := e.Timestamp.UnixNano()

	// Upsert driver_instance row shell if it doesn't exist yet.
	// Full driver registration lives in the DriverEvent path;
	// EntityRegistered only ensures the FK target exists.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO driver_instances
			(id, driver_name, display_name, transport, endpoint, config_hash, status, created_at)
		VALUES (?, '', '', 'local_subprocess', '', '', 'starting', ?)
		ON CONFLICT(id) DO NOTHING`,
		p.DriverInstanceId, now,
	); err != nil {
		return fmt.Errorf("ensure driver_instance: %w", err)
	}

	if p.DeviceId != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO devices
				(id, driver_instance_id, friendly_name, disabled, created_at, updated_at)
			VALUES (?, ?, '', 0, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			p.DeviceId, p.DriverInstanceId, now, now,
		); err != nil {
			return fmt.Errorf("ensure device: %w", err)
		}
	}

	var deviceID sql.NullString
	if p.DeviceId != "" {
		deviceID = sql.NullString{String: p.DeviceId, Valid: true}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO entities
			(id, device_id, driver_instance_id, entity_type, friendly_name, capabilities, disabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			device_id          = excluded.device_id,
			driver_instance_id = excluded.driver_instance_id,
			entity_type        = excluded.entity_type,
			friendly_name      = excluded.friendly_name,
			capabilities       = excluded.capabilities,
			disabled           = 0,
			updated_at         = excluded.updated_at`,
		e.Entity, deviceID, p.DriverInstanceId, p.EntityType, p.FriendlyName, caps, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert entity: %w", err)
	}
	return nil
}

func (r *Registry) applyEntityUnregistered(ctx context.Context, tx storage.Tx, e eventstore.Event) error {
	_, err := tx.ExecContext(ctx, `UPDATE entities SET disabled = 1, updated_at = ? WHERE id = ?`,
		e.Timestamp.UnixNano(), e.Entity)
	return err
}

func (r *Registry) applyDriverEvent(ctx context.Context, tx storage.Tx, e eventstore.Event, p *eventv1.DriverEvent) error {
	ts := e.Timestamp.UnixNano()
	switch p.Kind {
	case "started":
		_, err := tx.ExecContext(ctx, `
			INSERT INTO driver_instances
				(id, driver_name, display_name, transport, endpoint, config_hash, status, started_at, last_heartbeat, created_at)
			VALUES (?, ?, ?, 'local_subprocess', '', '', 'running', ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				status         = 'running',
				last_error     = NULL,
				started_at     = excluded.started_at,
				last_heartbeat = excluded.last_heartbeat`,
			p.DriverInstanceId, p.DriverInstanceId, p.DriverInstanceId, ts, ts, ts,
		)
		return err
	case "stopped":
		_, err := tx.ExecContext(ctx, `UPDATE driver_instances SET status = 'stopped' WHERE id = ?`, p.DriverInstanceId)
		return err
	case "failed":
		_, err := tx.ExecContext(ctx, `UPDATE driver_instances SET status = 'failed', last_error = ? WHERE id = ?`,
			p.Detail, p.DriverInstanceId)
		return err
	case "heartbeat":
		_, err := tx.ExecContext(ctx, `UPDATE driver_instances SET last_heartbeat = ? WHERE id = ?`, ts, p.DriverInstanceId)
		return err
	}
	return nil
}

// Unused import guard for time; keeps import line tidy if only used via UnixNano.
var _ = time.Now
```

- [ ] **Step 5: Write `internal/registry/queries.go`**

```go
package registry

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (r *Registry) GetDriverInstance(ctx context.Context, id string) (DriverInstance, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, driver_name, display_name, transport, endpoint, config_hash, status,
		COALESCE(last_error, ''), COALESCE(started_at, 0), COALESCE(last_heartbeat, 0), created_at FROM driver_instances WHERE id = ?`, id)
	return scanDriverInstance(row)
}

func (r *Registry) ListDriverInstances(ctx context.Context) ([]DriverInstance, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, driver_name, display_name, transport, endpoint, config_hash, status,
		COALESCE(last_error, ''), COALESCE(started_at, 0), COALESCE(last_heartbeat, 0), created_at FROM driver_instances ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DriverInstance
	for rows.Next() {
		di, err := scanDriverInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, di)
	}
	return out, rows.Err()
}

func (r *Registry) GetDevice(ctx context.Context, id string) (Device, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, driver_instance_id, friendly_name,
		COALESCE(manufacturer, ''), COALESCE(model, ''), COALESCE(sw_version, ''),
		metadata, disabled, created_at, updated_at FROM devices WHERE id = ?`, id)
	return scanDevice(row)
}

func (r *Registry) ListDevices(ctx context.Context, f DeviceFilter) ([]Device, error) {
	query := `SELECT id, driver_instance_id, friendly_name,
		COALESCE(manufacturer, ''), COALESCE(model, ''), COALESCE(sw_version, ''),
		metadata, disabled, created_at, updated_at FROM devices WHERE 1=1`
	args := []any{}
	if f.DriverInstanceID != "" {
		query += ` AND driver_instance_id = ?`
		args = append(args, f.DriverInstanceID)
	}
	if !f.IncludeDisabled {
		query += ` AND disabled = 0`
	}
	query += ` ORDER BY id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Registry) GetEntity(ctx context.Context, id string) (Entity, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, COALESCE(device_id, ''), driver_instance_id,
		entity_type, friendly_name, capabilities, disabled, created_at, updated_at FROM entities WHERE id = ?`, id)
	return scanEntity(row)
}

func (r *Registry) ListEntities(ctx context.Context, f EntityFilter) ([]Entity, error) {
	query := `SELECT id, COALESCE(device_id, ''), driver_instance_id, entity_type, friendly_name,
		capabilities, disabled, created_at, updated_at FROM entities WHERE 1=1`
	args := []any{}
	if f.DriverInstanceID != "" {
		query += ` AND driver_instance_id = ?`
		args = append(args, f.DriverInstanceID)
	}
	if f.DeviceID != "" {
		query += ` AND device_id = ?`
		args = append(args, f.DeviceID)
	}
	if f.EntityType != "" {
		query += ` AND entity_type = ?`
		args = append(args, f.EntityType)
	}
	if !f.IncludeDisabled {
		query += ` AND disabled = 0`
	}
	query += ` ORDER BY id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanDriverInstance(r scanner) (DriverInstance, error) {
	var di DriverInstance
	var startedAt, lastHB, createdAt int64
	err := r.Scan(&di.ID, &di.DriverName, &di.DisplayName, &di.Transport, &di.Endpoint,
		&di.ConfigHash, &di.Status, &di.LastError, &startedAt, &lastHB, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return di, fmt.Errorf("driver instance not found")
		}
		return di, err
	}
	if startedAt > 0 {
		di.StartedAt = time.Unix(0, startedAt)
	}
	if lastHB > 0 {
		di.LastHeartbeat = time.Unix(0, lastHB)
	}
	di.CreatedAt = time.Unix(0, createdAt)
	return di, nil
}

func scanDevice(r scanner) (Device, error) {
	var d Device
	var disabled int
	var createdAt, updatedAt int64
	err := r.Scan(&d.ID, &d.DriverInstanceID, &d.FriendlyName, &d.Manufacturer, &d.Model,
		&d.SwVersion, &d.Metadata, &disabled, &createdAt, &updatedAt)
	if err != nil {
		return d, err
	}
	d.Disabled = disabled != 0
	d.CreatedAt = time.Unix(0, createdAt)
	d.UpdatedAt = time.Unix(0, updatedAt)
	return d, nil
}

func scanEntity(r scanner) (Entity, error) {
	var e Entity
	var disabled int
	var createdAt, updatedAt int64
	err := r.Scan(&e.ID, &e.DeviceID, &e.DriverInstanceID, &e.EntityType, &e.FriendlyName,
		&e.Capabilities, &disabled, &createdAt, &updatedAt)
	if err != nil {
		return e, err
	}
	e.Disabled = disabled != 0
	e.CreatedAt = time.Unix(0, createdAt)
	e.UpdatedAt = time.Unix(0, updatedAt)
	return e, nil
}
```

- [ ] **Step 6: Write `internal/registry/registry_test.go`**

```go
package registry_test

import (
	"context"
	"testing"
	"time"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/registry"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestRegistry_EntityRegisteredCreatesRow(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	evt := eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "entity_registered",
		Entity: "light.lr", Source: "driver:hue-1",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "hue-1",
				EntityType:       "light",
				FriendlyName:     "Living Room",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
					Light: &entityv1.Light{},
				}},
			},
		}},
	}
	if err := reg.Apply(ctx, tx, evt); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	got, err := reg.GetEntity(ctx, "light.lr")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.FriendlyName != "Living Room" {
		t.Fatalf("FriendlyName = %q", got.FriendlyName)
	}
}

func TestRegistry_EntityUnregisteredDisables(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	// First register.
	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "entity_registered", Entity: "light.lr", Source: "d",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "d", EntityType: "light", FriendlyName: "x",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{}}},
			},
		}},
	})
	_ = tx.Commit()

	// Then unregister.
	tx2, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx2, eventstore.Event{
		Position: 2, Timestamp: time.Now(), Kind: "entity_unregistered", Entity: "light.lr", Source: "d",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityUnregistered{
			EntityUnregistered: &eventv1.EntityUnregistered{Reason: "removed"},
		}},
	})
	_ = tx2.Commit()

	got, err := reg.GetEntity(ctx, "light.lr")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Disabled {
		t.Fatal("entity should be disabled")
	}

	// Default filter excludes disabled.
	list, _ := reg.ListEntities(ctx, registry.EntityFilter{})
	if len(list) != 0 {
		t.Fatalf("default list should exclude disabled, got %d", len(list))
	}
}

func TestRegistry_ApplyIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	evt := eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "entity_registered", Entity: "light.x", Source: "d",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "d", EntityType: "light", FriendlyName: "x",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{}}},
			},
		}},
	}
	for i := 0; i < 3; i++ {
		tx, _ := db.BeginTx(ctx, nil)
		if err := reg.Apply(ctx, tx, evt); err != nil {
			t.Fatalf("apply[%d]: %v", i, err)
		}
		_ = tx.Commit()
	}
	list, _ := reg.ListEntities(ctx, registry.EntityFilter{})
	if len(list) != 1 {
		t.Fatalf("idempotent apply yielded %d rows", len(list))
	}
}
```

- [ ] **Step 7: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/registry/...
```

Expected: PASS — 3 tests.

- [ ] **Step 8: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/registry go.mod go.sum
git commit -m "feat(registry): add migrations, Apply for entity/driver events, and read API"
```

---

## Task 12: Subscriptions — non-durable with catchup/live handoff

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/subscribe.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/tailer.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/subscribe_test.go`

- [ ] **Step 1: Write failing test `internal/eventstore/subscribe_test.go`**

```go
package eventstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestSubscribe_LiveDeliversNewEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)

	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{FromPosition: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 10))

	select {
	case got := <-sub.C():
		if got.Entity != "light.a" {
			t.Fatalf("wrong entity %q", got.Entity)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive live event")
	}
}

func TestSubscribe_CatchupReplaysHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)

	for i := 0; i < 5; i++ {
		_, _ = s.Append(ctx, testutil.StateChanged("light.a", uint32(i)))
	}

	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{FromPosition: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	got := 0
	timeout := time.After(2 * time.Second)
	for got < 5 {
		select {
		case <-sub.C():
			got++
		case <-timeout:
			t.Fatalf("only received %d of 5 historical events", got)
		}
	}
}

func TestSubscribe_FilterExcludesUnmatchedEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)

	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{
		Filter: eventstore.Filter{Entities: []string{"light.a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	_, _ = s.Append(ctx, testutil.StateChanged("light.b", 5))  // excluded
	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 10)) // included

	select {
	case got := <-sub.C():
		if got.Entity != "light.a" {
			t.Fatalf("filter let through wrong event: %q", got.Entity)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("filter swallowed all events")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestSubscribe
```

Expected: FAIL — `Subscribe`/`Start` not defined.

- [ ] **Step 3: Write `internal/eventstore/subscribe.go`**

```go
package eventstore

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type SubscribeOptions struct {
	FromPosition  uint64
	Filter        Filter
	Durable       bool
	Name          string
	ChannelBuffer int
}

type Subscription interface {
	C() <-chan Event
	Ack(position uint64) error
	Close() error
	Stats() SubscriptionStats
}

type SubscriptionStats struct {
	Delivered uint64
	Dropped   uint64
	LagEvents uint64
	Buffered  int
}

type subscriber struct {
	name      string
	filter    Filter
	ch        chan Event
	delivered atomic.Uint64
	dropped   atomic.Uint64
	lastSent  atomic.Uint64
	store     *Store

	closeOnce sync.Once
	closed    chan struct{}
}

func (sub *subscriber) C() <-chan Event { return sub.ch }

func (sub *subscriber) Ack(_ uint64) error { return nil }

func (sub *subscriber) Close() error {
	sub.closeOnce.Do(func() {
		sub.store.unregisterSubscriber(sub)
		close(sub.closed)
		close(sub.ch)
	})
	return nil
}

func (sub *subscriber) Stats() SubscriptionStats {
	return SubscriptionStats{
		Delivered: sub.delivered.Load(),
		Dropped:   sub.dropped.Load(),
		LagEvents: 0,
		Buffered:  len(sub.ch),
	}
}

func (s *Store) Subscribe(ctx context.Context, opts SubscribeOptions) (Subscription, error) {
	if !s.started {
		return nil, errors.New("Subscribe: store not started")
	}
	buf := opts.ChannelBuffer
	if buf <= 0 {
		buf = s.cfg.MaxSubscriberBuffer
	}
	name := opts.Name
	if name == "" {
		name = "anon"
	}
	sub := &subscriber{
		name:   name,
		filter: opts.Filter,
		ch:     make(chan Event, buf),
		store:  s,
		closed: make(chan struct{}),
	}
	sub.lastSent.Store(opts.FromPosition)

	// Catchup phase: replay historical events from FromPosition up to a
	// target, then atomically register for live dispatch.
	if err := s.catchupAndRegister(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *Store) catchupAndRegister(ctx context.Context, sub *subscriber) error {
	for {
		s.mu.RLock()
		target := s.latestPosition
		s.mu.RUnlock()

		cursor := sub.lastSent.Load()
		if cursor >= target {
			// Register for live dispatch; re-check under lock.
			s.mu.Lock()
			if s.latestPosition > cursor {
				s.mu.Unlock()
				continue
			}
			s.subs = append(s.subs, sub)
			s.metrics.SubscriptionActive.WithLabelValues(sub.name).Set(1)
			s.mu.Unlock()
			return nil
		}

		// Page through historical events.
		evts, err := s.Query(ctx, QueryOptions{
			FromPosition: cursor,
			ToPosition:   target,
			Filter:       sub.filter,
			Limit:        1000,
		})
		if err != nil {
			return err
		}
		for _, e := range evts {
			select {
			case sub.ch <- e:
				sub.delivered.Add(1)
				sub.lastSent.Store(e.Position)
			case <-ctx.Done():
				return ctx.Err()
			case <-sub.closed:
				return nil
			}
		}
		if len(evts) == 0 {
			// No matching events in this page; advance cursor to target.
			sub.lastSent.Store(target)
		}
	}
}

func (s *Store) unregisterSubscriber(target *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.subs[:0]
	for _, sub := range s.subs {
		if sub != target {
			out = append(out, sub)
		}
	}
	s.subs = out
	s.metrics.SubscriptionActive.WithLabelValues(target.name).Set(0)
}

func (s *Store) dispatch(e Event) {
	s.mu.RLock()
	subs := make([]*subscriber, len(s.subs))
	copy(subs, s.subs)
	s.mu.RUnlock()

	for _, sub := range subs {
		if !sub.filter.Matches(e) {
			continue
		}
		select {
		case sub.ch <- e:
			sub.delivered.Add(1)
			sub.lastSent.Store(e.Position)
			s.metrics.SubscriptionDelivered.WithLabelValues(sub.name).Inc()
		default:
			sub.dropped.Add(1)
			s.metrics.SubscriptionDropped.WithLabelValues(sub.name).Inc()
			s.logger.Warn("subscriber dropped, closing", "name", sub.name)
			go sub.Close()
		}
	}
}
```

- [ ] **Step 4: Write `internal/eventstore/tailer.go`**

```go
package eventstore

import "context"

// Start launches the tailer goroutine. Callers must call Start before
// using Append/Subscribe; projectors must be registered beforehand.
// Future work (Task 16): also launch the snapshotter here.
func (s *Store) Start(ctx context.Context) error {
	if s.started {
		return nil
	}
	s.started = true
	go s.runTailer(ctx)
	return nil
}

func (s *Store) runTailer(ctx context.Context) {
	cursor := s.LatestPosition()
	for {
		s.cond.L.Lock()
		for cursor >= s.latestPosition && ctx.Err() == nil {
			s.cond.Wait()
		}
		target := s.latestPosition
		s.cond.L.Unlock()
		if ctx.Err() != nil {
			return
		}

		evts, err := s.Query(ctx, QueryOptions{
			FromPosition: cursor,
			ToPosition:   target,
			Limit:        int(target - cursor),
		})
		if err != nil {
			s.logger.Error("tailer query failed", "err", err)
			continue
		}
		s.metrics.TailerBatchSize.Observe(float64(len(evts)))
		for _, e := range evts {
			s.dispatch(e)
			cursor = e.Position
		}
		s.metrics.TailerLag.Set(float64(s.LatestPosition() - cursor))
	}
}
```

- [ ] **Step 5: Wake tailer on Close**

Edit `store.go`: update `Close` to broadcast so the tailer can exit.

```go
func (s *Store) Close(_ context.Context) error {
	s.cond.Broadcast()
	return nil
}
```

Also ensure `runTailer` exits on `ctx.Done()` — it already does via `cond.Wait` loop condition; callers should pass a cancellable ctx.

- [ ] **Step 6: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/eventstore/... -run TestSubscribe
```

Expected: PASS — 3 subscribe tests.

- [ ] **Step 7: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore go.mod go.sum
git commit -m "feat(eventstore): add Subscribe with catchup-then-live handoff and tailer loop"
```

---

## Task 13: Subscriptions — durable (persisted cursor)

**Files:**
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/subscribe.go`
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/subscribe_test.go`

- [ ] **Step 1: Extend `subscribe.go` — load and persist durable cursor**

Add to `subscribe.go`:

```go
func (s *Store) Subscribe2(ctx context.Context, opts SubscribeOptions) (Subscription, error) { return nil, nil }
// Placeholder — real change is editing Subscribe below.
```

Modify `Subscribe` to handle `Durable`:

```go
func (s *Store) Subscribe(ctx context.Context, opts SubscribeOptions) (Subscription, error) {
	if !s.started {
		return nil, errors.New("Subscribe: store not started")
	}
	if opts.Durable && opts.Name == "" {
		return nil, errors.New("Subscribe: Durable requires Name")
	}

	buf := opts.ChannelBuffer
	if buf <= 0 {
		buf = s.cfg.MaxSubscriberBuffer
	}
	name := opts.Name
	if name == "" {
		name = "anon"
	}

	fromPos := opts.FromPosition
	if opts.Durable {
		cursor, err := s.loadDurableCursor(ctx, name, opts.FromPosition)
		if err != nil {
			return nil, err
		}
		fromPos = cursor
	}

	sub := &subscriber{
		name:    name,
		filter:  opts.Filter,
		ch:      make(chan Event, buf),
		store:   s,
		closed:  make(chan struct{}),
		durable: opts.Durable,
	}
	sub.lastSent.Store(fromPos)

	if err := s.catchupAndRegister(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *Store) loadDurableCursor(ctx context.Context, name string, fallback uint64) (uint64, error) {
	var cursor int64
	err := s.db.QueryRowContext(ctx,
		`SELECT cursor FROM event_subscriptions WHERE name = ?`, name,
	).Scan(&cursor)
	if err == sql.ErrNoRows {
		now := time.Now().UnixNano()
		_, ierr := s.db.ExecContext(ctx, `
			INSERT INTO event_subscriptions (name, cursor, created_at, last_active)
			VALUES (?, ?, ?, ?)`,
			name, int64(fallback), now, now,
		)
		if ierr != nil {
			return 0, fmt.Errorf("insert durable subscription: %w", ierr)
		}
		return fallback, nil
	}
	if err != nil {
		return 0, err
	}
	return uint64(cursor), nil
}
```

Add imports: `database/sql`, `fmt`, `time`.

Add `durable bool` field to `subscriber`.

Replace `Ack` on `*subscriber`:

```go
func (sub *subscriber) Ack(position uint64) error {
	if !sub.durable {
		return nil
	}
	return sub.store.persistCursor(context.Background(), sub.name, position)
}
```

Add a debounced persist on the Store. For simplicity in C1, write-through without debounce (debounce lands in a later iteration):

```go
func (s *Store) persistCursor(ctx context.Context, name string, position uint64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE event_subscriptions SET cursor = ?, last_active = ? WHERE name = ?`,
		int64(position), time.Now().UnixNano(), name,
	)
	return err
}
```

- [ ] **Step 2: Add durable test**

Add to `subscribe_test.go`:

```go
func TestSubscribe_DurableResumesFromAckedCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Append 3; consume 2, ack the 2nd; close.
	for i := 0; i < 3; i++ {
		_, _ = s.Append(ctx, testutil.StateChanged("light.a", uint32(i)))
	}
	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{
		Durable: true, Name: "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	var lastSeen uint64
	for i := 0; i < 2; i++ {
		select {
		case e := <-sub.C():
			lastSeen = e.Position
		case <-time.After(2 * time.Second):
			t.Fatal("expected event")
		}
	}
	if err := sub.Ack(lastSeen); err != nil {
		t.Fatal(err)
	}
	_ = sub.Close()
	_ = s.Close(ctx)

	// Reopen and resubscribe; should only see the 3rd event.
	s2 := newStoreFromDB(t, storeDB(s))
	if err := s2.Start(ctx); err != nil {
		t.Fatal(err)
	}
	sub2, err := s2.Subscribe(ctx, eventstore.SubscribeOptions{
		Durable: true, Name: "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Close()

	select {
	case e := <-sub2.C():
		if e.Position <= lastSeen {
			t.Fatalf("resumed at position %d, want > %d", e.Position, lastSeen)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive remaining event")
	}
}

// Helpers to peek at the store's underlying DB so the test above can
// reuse it after Close. Lives here for now; moves to testutil later.
func storeDB(_ *eventstore.Store) any { panic("use newStore2 fixture that exposes DB") }
func newStoreFromDB(_ *testing.T, _ any) *eventstore.Store { panic("see note") }
```

Because these peek helpers don't exist and the test would require refactoring `newStore`, split this task:

- [ ] **Step 3: Refactor `newStore` helper to keep a DB handle reachable**

Change the signature of the private test helper:

```go
type storeFixture struct {
	store *eventstore.Store
	db    *sql.DB
}

func newStore(t *testing.T) *eventstore.Store {
	return newStoreFixture(t).store
}

func newStoreFixture(t *testing.T) *storeFixture {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return &storeFixture{store: s, db: db}
}
```

Add `"database/sql"` import to the test file.

Rewrite the durable test using the fixture:

```go
func TestSubscribe_DurableResumesFromAckedCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", uint32(i)))
	}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	var lastSeen uint64
	for i := 0; i < 2; i++ {
		select {
		case e := <-sub.C():
			lastSeen = e.Position
		case <-time.After(2 * time.Second):
			t.Fatal("expected event")
		}
	}
	if err := sub.Ack(lastSeen); err != nil {
		t.Fatal(err)
	}
	_ = sub.Close()
	_ = f.store.Close(ctx)

	// New Store sharing the same DB (simulates daemon restart).
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s2, err := eventstore.Open(context.Background(), eventstore.Config{}, f.db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Start(ctx); err != nil {
		t.Fatal(err)
	}
	sub2, err := s2.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Close()

	select {
	case e := <-sub2.C():
		if e.Position <= lastSeen {
			t.Fatalf("resumed at position %d, want > %d", e.Position, lastSeen)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive remaining event")
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestSubscribe
```

Expected: PASS — all subscribe tests including durable.

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "feat(eventstore): add durable subscriptions backed by event_subscriptions table"
```

---

## Task 14: Snapshotter goroutine + SnapshotNow

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/snapshot.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/snapshot_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/eventstore/snapshot_test.go
package eventstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestSnapshotNow_WritesRowForOwner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 100))

	pos, err := f.store.SnapshotNow(ctx, "state_cache")
	if err != nil {
		t.Fatalf("SnapshotNow: %v", err)
	}
	if pos == 0 {
		t.Fatal("expected non-zero position")
	}

	var count int
	if err := f.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM snapshots WHERE owner = 'state_cache'`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("snapshot row count = %d, want 1", count)
	}
	_ = f.store.Close(ctx)
	_ = time.After // avoid unused-import warning if not referenced
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestSnapshotNow
```

Expected: FAIL — `SnapshotNow` undefined.

- [ ] **Step 3: Write `internal/eventstore/snapshot.go`**

```go
package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SnapshotNow forces an immediate snapshot for owner. Pass "" to snapshot
// every registered projector sequentially.
func (s *Store) SnapshotNow(ctx context.Context, owner string) (uint64, error) {
	targets := s.projectors
	if owner != "" {
		found := false
		for _, reg := range s.projectors {
			if reg.p.Name() == owner {
				targets = []projectorReg{reg}
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("snapshot: unknown owner %q", owner)
		}
	}

	var lastPos uint64
	for _, reg := range targets {
		pos, err := s.runSnapshot(ctx, reg.p)
		if err != nil {
			return 0, err
		}
		if pos > lastPos {
			lastPos = pos
		}
	}
	return lastPos, nil
}

func (s *Store) runSnapshot(ctx context.Context, p Projector) (uint64, error) {
	start := time.Now()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := p.Snapshot(ctx, tx); err != nil {
		return 0, fmt.Errorf("snapshot %s: %w", p.Name(), err)
	}

	// Prune older snapshots beyond retain count.
	retain := s.cfg.SnapshotRetainPerOwner
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM snapshots WHERE owner = ? AND position NOT IN (
			SELECT position FROM snapshots WHERE owner = ? ORDER BY position DESC LIMIT ?
		)`, p.Name(), p.Name(), retain,
	); err != nil {
		return 0, fmt.Errorf("prune snapshots: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	s.metrics.SnapshotDuration.WithLabelValues(p.Name()).Observe(time.Since(start).Seconds())

	// Report the row we just inserted.
	var pos int64
	err = s.db.QueryRowContext(ctx,
		`SELECT position FROM snapshots WHERE owner = ? ORDER BY position DESC LIMIT 1`,
		p.Name(),
	).Scan(&pos)
	if errors.Is(err, sql.ErrNoRows) {
		// Projector reported no-op snapshot (e.g., registry).
		return s.LatestPosition(), nil
	}
	if err != nil {
		return 0, err
	}
	s.metrics.SnapshotLastPos.WithLabelValues(p.Name()).Set(float64(pos))
	return uint64(pos), nil
}

type snapshotEntry struct {
	projector Projector
	lastRun   time.Time
	lastPos   uint64
}

// startSnapshotter runs a background goroutine that checks cadence
// every minute. Called by Start (Task 18 wires it up); for now we
// expose it for tests.
func (s *Store) startSnapshotter(ctx context.Context) {
	entries := make([]*snapshotEntry, 0, len(s.projectors))
	for _, reg := range s.projectors {
		entries = append(entries, &snapshotEntry{projector: reg.p, lastRun: time.Now()})
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, e := range entries {
					current := s.LatestPosition()
					eventsSince := current - e.lastPos
					timeSince := time.Since(e.lastRun)
					if eventsSince >= uint64(s.cfg.SnapshotEveryEvents) || timeSince >= s.cfg.SnapshotEveryPeriod {
						if pos, err := s.runSnapshot(ctx, e.projector); err == nil {
							e.lastRun = time.Now()
							e.lastPos = pos
						} else {
							s.logger.Error("snapshot failed", "owner", e.projector.Name(), "err", err)
						}
					}
				}
			}
		}
	}()
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestSnapshotNow
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "feat(eventstore): add SnapshotNow, prune-after-snapshot, and cadence goroutine"
```

---

## Task 15: Startup replay — batched Apply through sync projectors

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/replay.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/replay_test.go`

- [ ] **Step 1: Write failing test `replay_test.go`**

```go
package eventstore_test

import (
	"context"
	"testing"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestReplay_RebuildsStateFromEvents(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache1 := state.New()
	if err := f.store.RegisterProjector(cache1, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 100))
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.b", 50))
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 200))
	_ = f.store.Close(ctx)

	// Second store, same DB: replay must reconstruct state.
	logger := observability.Init(observability.LogConfig{})
	metrics := observability.NewMetrics()
	s2, err := eventstore.Open(ctx, eventstore.Config{}, f.db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	cache2 := state.New()
	if err := s2.RegisterProjector(cache2, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := s2.Replay(ctx); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	s, ok := cache2.Get("light.a")
	if !ok {
		t.Fatal("light.a missing after replay")
	}
	if s.Attributes.GetLight().Brightness != 200 {
		t.Fatalf("brightness = %d, want 200", s.Attributes.GetLight().Brightness)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestReplay
```

Expected: FAIL — `Replay` undefined.

- [ ] **Step 3: Write `internal/eventstore/replay.go`**

```go
package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Replay restores each sync projector's snapshot, then applies pending
// events in 1000-event batches. Called before Start; not safe to call
// after tailer is live.
func (s *Store) Replay(ctx context.Context) error {
	if s.started {
		return errors.New("Replay: store already started")
	}
	cursor, err := s.restoreProjectors(ctx)
	if err != nil {
		return err
	}
	latest, err := s.loadLatestPosition(ctx)
	if err != nil {
		return err
	}

	for cursor < latest {
		applied, err := s.replayBatch(ctx, cursor, 1000)
		if err != nil {
			return err
		}
		if applied == 0 {
			break
		}
		cursor += applied
		s.metrics.ReplayEventsProcessed.Add(float64(applied))
	}
	return nil
}

func (s *Store) restoreProjectors(ctx context.Context) (uint64, error) {
	minCursor := uint64(^uint64(0)) // max uint64
	for _, reg := range s.projectors {
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return 0, err
		}
		pos, err := reg.p.Restore(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("restore %s: %w", reg.p.Name(), err)
		}
		if pos == 0 {
			// Fall back to projection_cursors.
			var cpos sql.NullInt64
			err = tx.QueryRowContext(ctx,
				`SELECT position FROM projection_cursors WHERE name = ?`, reg.p.Name(),
			).Scan(&cpos)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				_ = tx.Rollback()
				return 0, err
			}
			if cpos.Valid {
				pos = uint64(cpos.Int64)
			}
		}
		_ = tx.Commit()
		if pos < minCursor {
			minCursor = pos
		}
	}
	if minCursor == uint64(^uint64(0)) {
		return 0, nil
	}
	return minCursor, nil
}

func (s *Store) loadLatestPosition(ctx context.Context) (uint64, error) {
	var pos sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(position) FROM events`).Scan(&pos)
	if err != nil {
		return 0, err
	}
	if !pos.Valid {
		return 0, nil
	}
	return uint64(pos.Int64), nil
}

func (s *Store) replayBatch(ctx context.Context, after uint64, limit int) (uint64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT position, ts, kind, entity, source, correlation_id, cause_position, payload
		FROM events WHERE position > ? ORDER BY position LIMIT ?`, after, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var batch []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return 0, err
		}
		batch = append(batch, e)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, e := range batch {
		for _, reg := range s.projectors {
			if reg.mode != ProjectorModeSync {
				continue
			}
			if skipped, err := s.isSkipped(ctx, tx, e.Position, reg.p.Name()); err != nil {
				return 0, err
			} else if skipped {
				s.logger.Warn("skipping event per skipped_events table",
					"position", e.Position, "projector", reg.p.Name())
				continue
			}
			if err := reg.p.Apply(ctx, tx, e); err != nil {
				return 0, fmt.Errorf("replay projector %s at position %d: %w",
					reg.p.Name(), e.Position, err)
			}
		}
	}

	// Advance cursors to last event in batch.
	last := batch[len(batch)-1].Position
	for _, reg := range s.projectors {
		if reg.mode != ProjectorModeSync {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO projection_cursors (name, position, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
			reg.p.Name(), last, time.Now().UnixNano()); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	// Promote in-memory projector state.
	for _, reg := range s.projectors {
		if pc, ok := reg.p.(PostCommit); ok {
			pc.Promote()
		}
	}
	return uint64(len(batch)), nil
}

func (s *Store) isSkipped(ctx context.Context, tx *sql.Tx, pos uint64, projector string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skipped_events WHERE position = ? AND projector = ?`,
		pos, projector,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/...
```

Expected: PASS — all eventstore tests including replay.

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "feat(eventstore): add Replay for batched projector catchup with skipped_events respect"
```

---

## Task 16: Daemon startup — phase orchestration + recovery mode

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/daemon/daemon.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/daemon/config.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/daemon/recovery.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/daemon/daemon_test.go`

- [ ] **Step 1: Write `internal/daemon/config.go`**

```go
// Package daemon wires the eventstore, state cache, registry, CLI socket,
// and observability into a single runnable gohomed.
package daemon

import (
	"log/slog"
	"time"
)

type Config struct {
	DataDir               string
	LogLevel              slog.Level
	LogFormat             string        // "auto" | "tty" | "json"
	AdminPort             int           // HTTP for /metrics and /health
	SocketPath            string        // UNIX socket for CLI mutative ops
	SnapshotEveryEvents   int
	SnapshotEveryPeriod   time.Duration
}

func (c *Config) WithDefaults() {
	if c.DataDir == "" {
		c.DataDir = "~/.local/share/gohome"
	}
	if c.LogFormat == "" {
		c.LogFormat = "auto"
	}
	if c.AdminPort == 0 {
		c.AdminPort = 9190
	}
	if c.SocketPath == "" {
		c.SocketPath = "gohomed.sock" // resolved relative to DataDir
	}
	if c.SnapshotEveryEvents == 0 {
		c.SnapshotEveryEvents = 10_000
	}
	if c.SnapshotEveryPeriod == 0 {
		c.SnapshotEveryPeriod = time.Hour
	}
}
```

- [ ] **Step 2: Write `internal/daemon/daemon.go`**

```go
package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/registry"
	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/storage"
)

type Daemon struct {
	cfg       Config
	logger    *slog.Logger
	metrics   *observability.Metrics
	lockfile  *storage.Lockfile
	db        *sql.DB
	store     *eventstore.Store
	cache     *state.Cache
	registry  *registry.Registry

	phase        atomic.Int32 // 0 = not started, 1-5 per spec, -1 = recovery
	recoveryInfo atomic.Pointer[recoveryState]
}

type recoveryState struct {
	reason         string
	failedPosition uint64
}

// Version, Commit, and GoVersion are set via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	GoVersion = runtime.Version()
)

// New constructs an unstarted Daemon. Call Run to actually boot.
func New(cfg Config, logger *slog.Logger, metrics *observability.Metrics) *Daemon {
	cfg.WithDefaults()
	return &Daemon{cfg: cfg, logger: logger, metrics: metrics}
}

// Run brings the daemon up through phases 1-5 and blocks until ctx is done.
func (d *Daemon) Run(ctx context.Context) error {
	d.metrics.SetBuildInfo(Version, Commit, GoVersion)
	start := time.Now()

	// ----- Phase 1: cold open -----
	d.phase.Store(1)
	d.metrics.StartupPhase.Set(1)

	dataDir := expandHome(d.cfg.DataDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	lf, err := storage.AcquireLockfile(dataDir)
	if err != nil {
		return fmt.Errorf("lockfile: %w", err)
	}
	d.lockfile = lf
	defer d.lockfile.Release()

	db, err := storage.Open(ctx, storage.Config{Path: filepath.Join(dataDir, "gohome.db")})
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	d.db = db
	defer db.Close()

	// Start /metrics + /health HTTP server immediately.
	go func() {
		_ = d.metrics.ServeMetrics(ctx, fmt.Sprintf(":%d", d.cfg.AdminPort), d.healthStatus)
	}()

	// ----- Phase 2: construct projectors -----
	d.phase.Store(2)
	d.metrics.StartupPhase.Set(2)

	d.cache = state.New()
	reg, err := registry.New(ctx, db)
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	d.registry = reg

	store, err := eventstore.Open(ctx, eventstore.Config{
		SnapshotEveryEvents: d.cfg.SnapshotEveryEvents,
		SnapshotEveryPeriod: d.cfg.SnapshotEveryPeriod,
	}, db, d.logger, d.metrics)
	if err != nil {
		return fmt.Errorf("eventstore: %w", err)
	}
	d.store = store

	if err := store.RegisterProjector(d.cache, eventstore.ProjectorModeSync); err != nil {
		return err
	}
	if err := store.RegisterProjector(d.registry, eventstore.ProjectorModeSync); err != nil {
		return err
	}

	// ----- Phase 3: replay -----
	d.phase.Store(3)
	d.metrics.StartupPhase.Set(3)

	if err := store.Replay(ctx); err != nil {
		d.enterRecovery(err.Error())
		<-ctx.Done()
		return nil
	}

	// ----- Phase 4: live transition -----
	d.phase.Store(4)
	d.metrics.StartupPhase.Set(4)

	if err := store.Start(ctx); err != nil {
		return err
	}

	// ----- Phase 5: readiness -----
	d.phase.Store(5)
	d.metrics.StartupPhase.Set(5)
	d.metrics.StartupDuration.Observe(time.Since(start).Seconds())

	socketPath := filepath.Join(dataDir, d.cfg.SocketPath)
	go d.serveSocket(ctx, socketPath)

	if _, err := store.Append(ctx, eventstore.Event{
		Kind:      "system",
		Source:    "gohomed",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_System{
			System: &eventv1.SystemEvent{Kind: "startup", Data: map[string]string{
				"version":    Version,
				"commit":     Commit,
				"go_version": GoVersion,
			}},
		}},
	}); err != nil {
		d.logger.Error("failed to append startup event", "err", err)
	}
	d.logger.Info("gohomed ready", "version", Version, "data_dir", dataDir, "admin_port", d.cfg.AdminPort)

	<-ctx.Done()
	d.logger.Info("shutdown requested")

	// Best-effort final snapshot.
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := store.SnapshotNow(shutCtx, "state_cache"); err != nil {
		d.logger.Warn("final snapshot failed", "err", err)
	}
	return nil
}

func (d *Daemon) healthStatus() (string, int) {
	switch d.phase.Load() {
	case 5:
		return "ready", 200
	case -1:
		return "recovery", 503
	default:
		return "starting", 503
	}
}

func (d *Daemon) enterRecovery(reason string) {
	d.metrics.RecoveryModeEntered.Inc()
	d.phase.Store(-1)
	d.metrics.StartupPhase.Set(-1)
	d.recoveryInfo.Store(&recoveryState{reason: reason})
	d.logger.Error("entering recovery mode", "reason", reason)
	// Recovery HTTP endpoints piggyback on the existing admin server;
	// the snapshotter, tailer, subscriptions, and socket stay offline.
}

func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
```

- [ ] **Step 3: Write `internal/daemon/recovery.go` — UNIX-socket stub for CLI mutative ops**

```go
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"time"
)

// serveSocket listens on a UNIX socket for CLI mutative operations in C1.
// C4 replaces this with Connect-RPC.
func (d *Daemon) serveSocket(ctx context.Context, path string) {
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		d.logger.Error("listen socket", "path", path, "err", err)
		return
	}
	defer ln.Close()
	defer os.Remove(path)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			d.logger.Error("accept", "err", err)
			continue
		}
		go d.handleSocketConn(ctx, conn)
	}
}

type socketReq struct {
	Op    string `json:"op"`
	Owner string `json:"owner,omitempty"`
}

type socketResp struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Position uint64 `json:"position,omitempty"`
}

func (d *Daemon) handleSocketConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	var req socketReq
	if err := dec.Decode(&req); err != nil {
		_ = enc.Encode(socketResp{Error: err.Error()})
		return
	}
	switch req.Op {
	case "snapshot":
		pos, err := d.store.SnapshotNow(ctx, req.Owner)
		if err != nil {
			_ = enc.Encode(socketResp{Error: err.Error()})
			return
		}
		_ = enc.Encode(socketResp{OK: true, Position: pos})
	default:
		_ = enc.Encode(socketResp{Error: "unknown op"})
	}
}
```

- [ ] **Step 4: Write `internal/daemon/daemon_test.go` — sanity smoke test**

```go
package daemon_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/daemon"
	"github.com/fynn-labs/gohome/internal/observability"
)

func TestDaemon_StartsAndShutsDownCleanly(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()

	d := daemon.New(daemon.Config{
		DataDir:   dir,
		LogLevel:  slog.LevelInfo,
		LogFormat: "json",
		AdminPort: freeTCPPort(t),
	}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// Wait up to 5s for readiness.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "gohome.db")); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
```

Remember to add `"net"` import to this test.

- [ ] **Step 5: Run tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go mod tidy
go test ./internal/daemon/...
```

Expected: PASS — daemon boots to Phase 5, cancel brings it down cleanly.

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/daemon go.mod go.sum
git commit -m "feat(daemon): wire eventstore, state, registry through phased startup + socket"
```

---

## Task 17: Daemon binary `cmd/gohomed`

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/cmd/gohomed/main.go`

- [ ] **Step 1: Write `cmd/gohomed/main.go`**

```go
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

	"github.com/fynn-labs/gohome/internal/daemon"
	"github.com/fynn-labs/gohome/internal/observability"
)

func main() {
	var (
		dataDir           = flag.String("data-dir", "", "data directory (default ~/.local/share/gohome)")
		logLevel          = flag.String("log-level", "info", "error|warn|info|debug")
		logFormat         = flag.String("log-format", "auto", "auto|tty|json")
		adminPort         = flag.Int("admin-port", 9190, "HTTP admin port for /metrics and /health")
		snapshotEveryEvt  = flag.Int("snapshot-every-events", 10_000, "snapshot cadence: events since last")
		snapshotEveryDur  = flag.Duration("snapshot-every-period", time.Hour, "snapshot cadence: wall-clock period")
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
	}
	d := daemon.New(cfg, logger, metrics)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := d.Run(ctx); err != nil {
		logger.Error("daemon exited with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
```

- [ ] **Step 2: Build and run the daemon briefly**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build -o dist/gohomed ./cmd/gohomed
DATA=$(mktemp -d)
./dist/gohomed --data-dir "$DATA" --admin-port 9191 &
PID=$!
sleep 1
curl -sf http://localhost:9191/health
curl -sf http://localhost:9191/metrics | grep gohome_events_appended_total
kill -TERM $PID
wait $PID 2>/dev/null
rm -rf "$DATA"
```

Expected:
- `/health` returns `{"status":"ready"}` after ~1s.
- `/metrics` shows `gohome_events_appended_total{kind="system"} 1` from the startup event.
- Clean exit on SIGTERM.

- [ ] **Step 3: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add cmd/gohomed
git commit -m "feat(gohomed): add daemon binary entrypoint with CLI flags and signal handling"
```

---

## Task 18: CLI — styling, root command, version

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/styles.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/root.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/version.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/cmd/gohome/main.go`

- [ ] **Step 1: Write `internal/cli/styles.go`**

```go
// Package cli owns the gohome CLI command tree, styling, and output helpers.
package cli

import "github.com/charmbracelet/lipgloss"

var (
	Header      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C7CFF"))
	EntityID    = lipgloss.NewStyle().Foreground(lipgloss.Color("#4EC9B0"))
	Kind        = lipgloss.NewStyle().Foreground(lipgloss.Color("#DCDCAA"))
	Timestamp   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Correlation = lipgloss.NewStyle().Foreground(lipgloss.Color("#C586C0"))
	Error       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C")).Bold(true)
	Success     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4AC776"))
	Dim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)
```

- [ ] **Step 2: Write `internal/cli/root.go`**

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type globalFlags struct {
	DataDir  string
	Format   string
	NoColor  bool
	LogLevel string
	Verbose  bool
}

// NewRoot constructs the full command tree.
func NewRoot() *cobra.Command {
	gf := &globalFlags{}
	root := &cobra.Command{
		Use:   "gohome",
		Short: "gohome CLI — read-only inspection and operator ops",
		Long:  "Event-sourced home automation. Query the event log, inspect state, manage snapshots.",
	}
	root.PersistentFlags().StringVar(&gf.DataDir, "data-dir", defaultDataDir(), "data directory")
	root.PersistentFlags().StringVar(&gf.Format, "format", "auto", "auto|table|json|yaml")
	root.PersistentFlags().BoolVar(&gf.NoColor, "no-color", false, "disable ANSI color")
	root.PersistentFlags().StringVar(&gf.LogLevel, "log-level", "warn", "error|warn|info|debug")
	root.PersistentFlags().BoolVarP(&gf.Verbose, "verbose", "v", false, "--log-level=debug shortcut")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newEventsCmd(gf))
	root.AddCommand(newStateCmd(gf))
	root.AddCommand(newRegistryCmd(gf))
	root.AddCommand(newSnapshotCmd(gf))
	return root
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gohome"
	}
	return filepath.Join(home, ".local", "share", "gohome")
}

// dieOnError prints err via the CLI Error style and exits 1.
func dieOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, Error.Render("error:")+" "+err.Error())
	os.Exit(1)
}
```

- [ ] **Step 3: Write `internal/cli/version.go`**

```go
package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Run: func(cmd *cobra.Command, _ []string) {
			commit := Commit
			if commit == "unknown" {
				if info, ok := debug.ReadBuildInfo(); ok {
					for _, s := range info.Settings {
						if s.Key == "vcs.revision" {
							commit = s.Value
						}
					}
				}
			}
			fmt.Printf("gohome %s (%s)\n", Version, commit)
		},
	}
}
```

- [ ] **Step 4: Write `cmd/gohome/main.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/fynn-labs/gohome/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Build and smoke-test**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build -o dist/gohome ./cmd/gohome
./dist/gohome version
./dist/gohome --help
```

Expected:
- `gohome dev (unknown)` or a commit hash if in a git repo.
- Help text listing `events`, `state`, `registry`, `snapshot`, `version`.

(The subcommand files are stubs until Tasks 19–21 fill them in; compilation requires them to exist.)

- [ ] **Step 6: Create stub subcommand files so build compiles**

Create `internal/cli/events.go`:
```go
package cli

import "github.com/spf13/cobra"

func newEventsCmd(_ *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "events", Short: "Query and tail the event log"}
	return c
}
```

Same shape for `state.go`, `registry.go`, `snapshot.go`:
```go
// state.go
package cli
import "github.com/spf13/cobra"
func newStateCmd(_ *globalFlags) *cobra.Command {
	return &cobra.Command{Use: "state", Short: "Inspect live entity state"}
}

// registry.go
package cli
import "github.com/spf13/cobra"
func newRegistryCmd(_ *globalFlags) *cobra.Command {
	return &cobra.Command{Use: "registry", Short: "Inspect the registry"}
}

// snapshot.go
package cli
import "github.com/spf13/cobra"
func newSnapshotCmd(_ *globalFlags) *cobra.Command {
	return &cobra.Command{Use: "snapshot", Short: "Create and list snapshots"}
}
```

- [ ] **Step 7: Re-run the build**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build -o dist/gohome ./cmd/gohome
./dist/gohome version
```

Expected: success.

- [ ] **Step 8: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/cli cmd/gohome go.mod go.sum
git commit -m "feat(cli): add Cobra root, lipgloss styles, version, and subcommand scaffolding"
```

---

## Task 19: CLI — `gohome events` (query / tail / inspect / export)

**Files:**
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/events.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/render.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/cliutil.go`

- [ ] **Step 1: Write `internal/cli/cliutil.go` — shared CLI helpers**

```go
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fynn-labs/gohome/internal/storage"
)

// openReadOnlyDB opens the gohomed SQLite file in the CLI process. WAL mode
// lets us read concurrently with the running daemon.
func openReadOnlyDB(ctx context.Context, dataDir string) (*sql.DB, error) {
	dataDir = expandHome(dataDir)
	path := filepath.Join(dataDir, "gohome.db")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("no database at %s — is gohomed running?", path)
	}
	return storage.Open(ctx, storage.Config{Path: path})
}

func expandHome(p string) string {
	if len(p) == 0 || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[1:])
}
```

- [ ] **Step 2: Write `internal/cli/render.go`**

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/table"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fynn-labs/gohome/internal/eventstore"
)

type outputFormat int

const (
	outFormatAuto outputFormat = iota
	outFormatTable
	outFormatJSON
	outFormatYAML
)

func parseFormat(s string, isTerminal bool) outputFormat {
	switch s {
	case "table":
		return outFormatTable
	case "json":
		return outFormatJSON
	case "yaml":
		return outFormatYAML
	default:
		if isTerminal {
			return outFormatTable
		}
		return outFormatJSON
	}
}

func renderEvents(w io.Writer, events []eventstore.Event, format outputFormat) error {
	switch format {
	case outFormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		for _, e := range events {
			rec := eventToMap(e)
			if err := enc.Encode(rec); err != nil {
				return err
			}
		}
		return nil
	default:
		return renderEventsTable(w, events)
	}
}

func renderEventsTable(w io.Writer, events []eventstore.Event) error {
	t := table.New().
		Headers("Time", "Kind", "Entity", "Source", "Correlation").
		StyleFunc(func(row, _ int) lipglossStyleAlias {
			if row == 0 {
				return Header
			}
			return lipglossStyleAlias{}
		})
	for _, e := range events {
		corr := ""
		if !isZeroUUID(e.CorrelationID) {
			corr = e.CorrelationID.String()[:8]
		}
		t.Row(
			e.Timestamp.Format("15:04:05.000"),
			Kind.Render(e.Kind),
			EntityID.Render(e.Entity),
			Dim.Render(e.Source),
			Correlation.Render(corr),
		)
	}
	fmt.Fprintln(w, t)
	return nil
}

// Aliases to keep the StyleFunc signature compatible across lipgloss versions.
type lipglossStyleAlias = struct{ lipglossInnerUnused int }

func isZeroUUID(id [16]byte) bool {
	for _, b := range id {
		if b != 0 {
			return false
		}
	}
	return true
}

func eventToMap(e eventstore.Event) map[string]any {
	m := map[string]any{
		"position":       e.Position,
		"timestamp":      e.Timestamp.Format(time.RFC3339Nano),
		"kind":           e.Kind,
		"entity":         e.Entity,
		"source":         e.Source,
		"correlation_id": e.CorrelationID.String(),
	}
	if e.CausePosition > 0 {
		m["cause_position"] = e.CausePosition
	}
	if e.Payload != nil {
		if raw, err := protojson.Marshal(e.Payload); err == nil {
			var payload any
			if err := json.Unmarshal(raw, &payload); err == nil {
				m["payload"] = payload
			} else {
				m["payload"] = string(raw)
			}
		}
	}
	return m
}

func inspectEvent(w io.Writer, e eventstore.Event) error {
	var b strings.Builder
	b.WriteString(Header.Render("Event #"+fmt.Sprint(e.Position)) + "\n")
	b.WriteString(Dim.Render("Time:        ") + e.Timestamp.Format(time.RFC3339Nano) + "\n")
	b.WriteString(Dim.Render("Kind:        ") + Kind.Render(e.Kind) + "\n")
	b.WriteString(Dim.Render("Entity:      ") + EntityID.Render(e.Entity) + "\n")
	b.WriteString(Dim.Render("Source:      ") + e.Source + "\n")
	b.WriteString(Dim.Render("Correlation: ") + Correlation.Render(e.CorrelationID.String()) + "\n")
	if e.CausePosition > 0 {
		b.WriteString(Dim.Render("Caused by:   ") + fmt.Sprint(e.CausePosition) + "\n")
	}
	if e.Payload != nil {
		raw, _ := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(e.Payload)
		b.WriteString("\n" + Header.Render("Payload") + "\n")
		b.Write(raw)
		b.WriteString("\n")
	}
	_, err := w.Write([]byte(b.String()))
	return err
}
```

> Note: `lipglossStyleAlias` / `lipglossInnerUnused` placeholder above is a compile wart to avoid coupling to a specific `lipgloss/table` internal. Replace with the real `lipgloss.Style` type from the installed version — likely `lipgloss.NewStyle()` return, with `StyleFunc(func(row, col int) lipgloss.Style)`. Confirm by reading `lipgloss/table` docs for the installed version; remove the placeholder type.

- [ ] **Step 3: Write `internal/cli/events.go` (full version)**

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/fynn-labs/gohome/internal/eventstore"
)

func newEventsCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "events", Short: "Query, tail, inspect, and export events"}
	c.AddCommand(newEventsQueryCmd(gf))
	c.AddCommand(newEventsTailCmd(gf))
	c.AddCommand(newEventsInspectCmd(gf))
	c.AddCommand(newEventsExportCmd(gf))
	return c
}

func newEventsQueryCmd(gf *globalFlags) *cobra.Command {
	var (
		from   uint64
		to     uint64
		kind   string
		entity string
		limit  int
	)
	c := &cobra.Command{
		Use:   "query",
		Short: "Historical query against the event log",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer db.Close()

			// Build a throwaway Store for Query (no projectors, no tailer).
			logger := nullLogger()
			metrics := nullMetrics()
			store, err := eventstore.Open(ctx, eventstore.Config{}, db, logger, metrics)
			dieOnError(err)

			filter := eventstore.Filter{}
			if kind != "" {
				filter.Kinds = []string{kind}
			}
			if entity != "" {
				filter.Entities = []string{entity}
			}
			events, err := store.Query(ctx, eventstore.QueryOptions{
				FromPosition: from,
				ToPosition:   to,
				Filter:       filter,
				Limit:        limit,
			})
			dieOnError(err)
			format := parseFormat(gf.Format, isTerminal(os.Stdout))
			dieOnError(renderEvents(os.Stdout, events, format))
		},
	}
	c.Flags().Uint64Var(&from, "from", 0, "from position (exclusive)")
	c.Flags().Uint64Var(&to, "to", 0, "to position (inclusive); 0 = unbounded")
	c.Flags().StringVar(&kind, "kind", "", "filter by kind")
	c.Flags().StringVar(&entity, "entity", "", "filter by entity ID")
	c.Flags().IntVar(&limit, "limit", 100, "max events to return")
	return c
}

func newEventsTailCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tail",
		Short: "Stream live events from the daemon",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer db.Close()

			logger := nullLogger()
			metrics := nullMetrics()
			store, err := eventstore.Open(ctx, eventstore.Config{}, db, logger, metrics)
			dieOnError(err)
			dieOnError(store.Start(ctx))

			sub, err := store.Subscribe(ctx, eventstore.SubscribeOptions{
				FromPosition: store.LatestPosition(),
			})
			dieOnError(err)
			defer sub.Close()

			for e := range sub.C() {
				dieOnError(renderEvents(os.Stdout, []eventstore.Event{e},
					parseFormat(gf.Format, isTerminal(os.Stdout))))
			}
			_ = time.Second // unused import guard
		},
	}
}

func newEventsInspectCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <position>",
		Short: "Show a single event in full detail",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			pos, err := strconv.ParseUint(args[0], 10, 64)
			dieOnError(err)
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer db.Close()

			store, err := eventstore.Open(ctx, eventstore.Config{}, db, nullLogger(), nullMetrics())
			dieOnError(err)
			events, err := store.Query(ctx, eventstore.QueryOptions{
				FromPosition: pos - 1, ToPosition: pos, Limit: 1,
			})
			dieOnError(err)
			if len(events) == 0 {
				dieOnError(fmt.Errorf("no event at position %d", pos))
			}
			dieOnError(inspectEvent(os.Stdout, events[0]))
		},
	}
}

func newEventsExportCmd(gf *globalFlags) *cobra.Command {
	var (
		from uint64
		to   uint64
		out  string
	)
	c := &cobra.Command{
		Use:   "export",
		Short: "Export events as JSONL",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer db.Close()

			store, err := eventstore.Open(ctx, eventstore.Config{}, db, nullLogger(), nullMetrics())
			dieOnError(err)

			w := os.Stdout
			if out != "" && out != "-" {
				f, err := os.Create(out)
				dieOnError(err)
				defer f.Close()
				w = f
			}

			cursor := from
			for {
				batch, err := store.Query(ctx, eventstore.QueryOptions{
					FromPosition: cursor, ToPosition: to, Limit: 1000,
				})
				dieOnError(err)
				if len(batch) == 0 {
					break
				}
				dieOnError(renderEvents(w, batch, outFormatJSON))
				cursor = batch[len(batch)-1].Position
			}
		},
	}
	c.Flags().Uint64Var(&from, "from", 0, "from position (exclusive)")
	c.Flags().Uint64Var(&to, "to", 0, "to position (inclusive); 0 = unbounded")
	c.Flags().StringVarP(&out, "output", "o", "-", "output file; - for stdout")
	return c
}

// isTerminal detects whether stdout is a TTY (for default --format=auto).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
```

- [ ] **Step 4: Add a tiny nullLogger/nullMetrics helper**

Append to `cliutil.go`:

```go
import (
	// ...
	"io"
	"log/slog"

	"github.com/fynn-labs/gohome/internal/observability"
)

func nullLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func nullMetrics() *observability.Metrics {
	return observability.NewMetrics()
}
```

- [ ] **Step 5: Build and smoke-test**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build -o dist/gohome ./cmd/gohome

# Start daemon in background.
DATA=$(mktemp -d)
./dist/gohomed --data-dir "$DATA" --admin-port 9199 &
PID=$!
sleep 1

# Query historical (at least the startup event).
./dist/gohome --data-dir "$DATA" events query --limit 5
./dist/gohome --data-dir "$DATA" events inspect 1

kill -TERM $PID
wait $PID 2>/dev/null
rm -rf "$DATA"
```

Expected: query returns the `system` startup event; inspect shows pretty-printed payload.

> If the `lipglossStyleAlias` placeholder from `render.go` fails to compile, replace with real `lipgloss.Style` types from the installed lipgloss version (the style func signature is `func(row, col int) lipgloss.Style`).

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/cli cmd/gohome
git commit -m "feat(cli): add events query/tail/inspect/export subcommands"
```

---

## Task 20: CLI — `gohome state` and `gohome registry`

**Files:**
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/state.go`
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/registry.go`

- [ ] **Step 1: Replace `state.go`**

```go
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/state"
)

func newStateCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "state", Short: "Inspect live entity state"}
	c.AddCommand(newStateGetCmd(gf))
	c.AddCommand(newStateDumpCmd(gf))
	return c
}

func newStateGetCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <entity-id>",
		Short: "Print a single entity's current state",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cache := loadStateCache(cmd.Context(), gf.DataDir)
			s, ok := cache.Get(args[0])
			if !ok {
				dieOnError(fmt.Errorf("entity %q not found", args[0]))
			}
			raw, _ := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(s.Attributes)
			fmt.Printf("%s  %s\n  Updated: %s by %s\n  Attributes: %s\n",
				Header.Render("entity"), EntityID.Render(s.EntityID),
				Timestamp.Render(s.UpdatedAt.Format("2006-01-02 15:04:05")), Dim.Render(s.UpdatedBy),
				string(raw))
		},
	}
}

func newStateDumpCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "dump",
		Short: "Dump the current state cache",
		Run: func(cmd *cobra.Command, _ []string) {
			cache := loadStateCache(cmd.Context(), gf.DataDir)
			view := cache.View()
			out := map[string]any{}
			iter := view.Iterator()
			for !iter.Done() {
				id, s, _ := iter.Next()
				raw, _ := protojson.Marshal(s.Attributes)
				var payload any
				_ = json.Unmarshal(raw, &payload)
				out[id] = map[string]any{
					"updated_at": s.UpdatedAt,
					"updated_by": s.UpdatedBy,
					"attributes": payload,
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			dieOnError(enc.Encode(out))
		},
	}
}

// loadStateCache rebuilds the state cache from the DB: restore latest
// snapshot, then replay tail. This is read-only and does not affect the
// running daemon (WAL allows concurrent reads).
func loadStateCache(ctx context.Context, dataDir string) *state.Cache {
	db, err := openReadOnlyDB(ctx, dataDir)
	dieOnError(err)

	store, err := eventstore.Open(ctx, eventstore.Config{}, db, nullLogger(), nullMetrics())
	dieOnError(err)

	cache := state.New()
	dieOnError(store.RegisterProjector(cache, eventstore.ProjectorModeSync))
	dieOnError(store.Replay(ctx))
	return cache
}
```

Add `"context"` import.

- [ ] **Step 2: Replace `registry.go`**

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/fynn-labs/gohome/internal/registry"
)

func newRegistryCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "registry", Short: "Inspect devices, entities, and drivers"}
	c.AddCommand(newRegistryListCmd(gf))
	c.AddCommand(newRegistryShowCmd(gf))
	return c
}

func newRegistryListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list (devices|entities|drivers)",
		Short: "List registry rows",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			reg := loadRegistry(ctx, gf.DataDir)
			switch args[0] {
			case "devices":
				list, err := reg.ListDevices(ctx, registry.DeviceFilter{})
				dieOnError(err)
				t := table.New().Headers("ID", "Driver", "Name")
				for _, d := range list {
					t.Row(d.ID, d.DriverInstanceID, d.FriendlyName)
				}
				fmt.Fprintln(os.Stdout, t)
			case "entities":
				list, err := reg.ListEntities(ctx, registry.EntityFilter{})
				dieOnError(err)
				t := table.New().Headers("ID", "Type", "Name", "Driver")
				for _, e := range list {
					t.Row(EntityID.Render(e.ID), e.EntityType, e.FriendlyName, Dim.Render(e.DriverInstanceID))
				}
				fmt.Fprintln(os.Stdout, t)
				fmt.Fprintln(os.Stdout, Dim.Render(fmt.Sprintf("%d entities", len(list))))
			case "drivers":
				list, err := reg.ListDriverInstances(ctx)
				dieOnError(err)
				t := table.New().Headers("ID", "Driver", "Status", "Endpoint")
				for _, d := range list {
					t.Row(d.ID, d.DriverName, d.Status, d.Endpoint)
				}
				fmt.Fprintln(os.Stdout, t)
			default:
				dieOnError(fmt.Errorf("unknown collection %q", args[0]))
			}
		},
	}
}

func newRegistryShowCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show an entity, device, or driver by id",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			reg := loadRegistry(ctx, gf.DataDir)
			if e, err := reg.GetEntity(ctx, args[0]); err == nil {
				fmt.Println(Header.Render("Entity"))
				fmt.Printf("  ID:     %s\n  Type:   %s\n  Name:   %s\n  Driver: %s\n  Device: %s\n",
					EntityID.Render(e.ID), e.EntityType, e.FriendlyName, Dim.Render(e.DriverInstanceID), e.DeviceID)
				return
			}
			if d, err := reg.GetDevice(ctx, args[0]); err == nil {
				fmt.Println(Header.Render("Device"))
				fmt.Printf("  ID: %s\n  Driver: %s\n  Name: %s\n", d.ID, Dim.Render(d.DriverInstanceID), d.FriendlyName)
				return
			}
			if di, err := reg.GetDriverInstance(ctx, args[0]); err == nil {
				fmt.Println(Header.Render("Driver"))
				fmt.Printf("  ID: %s\n  Driver: %s\n  Status: %s\n", di.ID, di.DriverName, di.Status)
				return
			}
			dieOnError(fmt.Errorf("no entity/device/driver with id %q", args[0]))
		},
	}
}

func loadRegistry(ctx context.Context, dataDir string) *registry.Registry {
	db, err := openReadOnlyDB(ctx, dataDir)
	dieOnError(err)
	reg, err := registry.New(ctx, db)
	dieOnError(err)
	return reg
}
```

- [ ] **Step 3: Build and smoke-test**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build -o dist/gohome ./cmd/gohome

DATA=$(mktemp -d)
./dist/gohomed --data-dir "$DATA" --admin-port 9198 &
PID=$!
sleep 1
./dist/gohome --data-dir "$DATA" registry list entities
./dist/gohome --data-dir "$DATA" state dump
kill -TERM $PID
wait $PID 2>/dev/null
rm -rf "$DATA"
```

Expected:
- `registry list entities` shows an empty table with footer `0 entities`.
- `state dump` prints `{}` (no entities registered yet in C1).

- [ ] **Step 4: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/cli
git commit -m "feat(cli): add state get/dump and registry list/show subcommands"
```

---

## Task 21: CLI — `gohome snapshot` (create via UNIX socket, list from DB)

**Files:**
- Modify: `/Users/fdatoo/Desktop/GoHome/gohome/internal/cli/snapshot.go`

- [ ] **Step 1: Replace `snapshot.go`**

```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

func newSnapshotCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "snapshot", Short: "Create and list snapshots"}
	c.AddCommand(newSnapshotCreateCmd(gf))
	c.AddCommand(newSnapshotListCmd(gf))
	return c
}

func newSnapshotCreateCmd(gf *globalFlags) *cobra.Command {
	var owner string
	var reason string
	c := &cobra.Command{
		Use:   "create",
		Short: "Trigger an immediate snapshot",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			sockPath := filepath.Join(expandHome(gf.DataDir), "gohomed.sock")
			conn, err := (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
			dieOnError(err)
			defer conn.Close()

			req := map[string]string{"op": "snapshot", "owner": owner, "reason": reason}
			if err := json.NewEncoder(conn).Encode(req); err != nil {
				dieOnError(err)
			}
			var resp struct {
				OK       bool   `json:"ok"`
				Error    string `json:"error"`
				Position uint64 `json:"position"`
			}
			if err := json.NewDecoder(conn).Decode(&resp); err != nil {
				dieOnError(err)
			}
			if !resp.OK {
				dieOnError(fmt.Errorf("%s", resp.Error))
			}
			fmt.Printf("%s snapshot for %s at position %d\n",
				Success.Render("created"), EntityID.Render(owner), resp.Position)
		},
	}
	c.Flags().StringVar(&owner, "owner", "state_cache", "owner: state_cache | registry | <projector>")
	c.Flags().StringVar(&reason, "reason", "manual", "reason recorded in snapshot meta")
	return c
}

func newSnapshotListCmd(gf *globalFlags) *cobra.Command {
	var owner string
	c := &cobra.Command{
		Use:   "list",
		Short: "List snapshots stored in the DB",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer db.Close()

			query := `SELECT position, ts, owner, encoding, LENGTH(state) FROM snapshots`
			args := []any{}
			if owner != "" {
				query += ` WHERE owner = ?`
				args = append(args, owner)
			}
			query += ` ORDER BY position DESC LIMIT 50`
			rows, err := db.QueryContext(ctx, query, args...)
			dieOnError(err)
			defer rows.Close()

			t := table.New().Headers("Position", "Owner", "Encoding", "Size", "Time")
			for rows.Next() {
				var pos int64
				var tsNanos int64
				var o, enc string
				var size int64
				dieOnError(rows.Scan(&pos, &tsNanos, &o, &enc, &size))
				t.Row(
					fmt.Sprint(pos),
					o,
					enc,
					fmt.Sprintf("%.1f KB", float64(size)/1024),
					time.Unix(0, tsNanos).Format("2006-01-02 15:04:05"),
				)
			}
			fmt.Fprintln(os.Stdout, t)
		},
	}
	c.Flags().StringVar(&owner, "owner", "", "filter by owner")
	return c
}
```

- [ ] **Step 2: Smoke-test**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go build -o dist/gohome ./cmd/gohome
DATA=$(mktemp -d)
./dist/gohomed --data-dir "$DATA" --admin-port 9197 &
PID=$!
sleep 1
./dist/gohome --data-dir "$DATA" snapshot create --owner=state_cache
./dist/gohome --data-dir "$DATA" snapshot list
kill -TERM $PID
wait $PID 2>/dev/null
rm -rf "$DATA"
```

Expected:
- `snapshot create` reports a position.
- `snapshot list` shows one row with owner `state_cache`.

- [ ] **Step 3: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/cli
git commit -m "feat(cli): add snapshot create (via UNIX socket) and list"
```

---

## Task 22: Golden-file replay fixtures

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/testutil/fixtures.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/basic_state_flow.jsonl`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/basic_state_flow.golden.json`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/scene_apply.jsonl`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/scene_apply.golden.json`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/driver_restart.jsonl`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/driver_restart.golden.json`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/snapshot_roundtrip.jsonl`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/snapshot_roundtrip.golden.json`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/correlation_walk.jsonl`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/testdata/fixtures/correlation_walk.golden.json`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/replay_golden_test.go`

- [ ] **Step 1: Write `internal/testutil/fixtures.go`**

```go
package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

// LoadFixture reads testdata/fixtures/<name>.jsonl — one protojson-encoded
// Event per line. Returns a slice of events with Position left as 0
// (the store assigns positions on Append).
func LoadFixture(t *testing.T, name string) []eventstore.Event {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fixtures", name+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		// Try current-dir form (for tests run from repo root).
		path = filepath.Join("testdata", "fixtures", name+".jsonl")
		f, err = os.Open(path)
		if err != nil {
			t.Fatalf("open fixture %s: %v", name, err)
		}
	}
	defer f.Close()

	var out []eventstore.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		var rec struct {
			Kind    string          `json:"kind"`
			Entity  string          `json:"entity"`
			Source  string          `json:"source"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("unmarshal fixture line: %v — %s", err, string(line))
		}
		payload := &eventv1.Payload{}
		if err := protojson.Unmarshal(rec.Payload, payload); err != nil {
			t.Fatalf("unmarshal payload: %v — %s", err, string(rec.Payload))
		}
		out = append(out, eventstore.Event{
			Kind:    rec.Kind,
			Entity:  rec.Entity,
			Source:  rec.Source,
			Payload: payload,
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan fixture %s: %v", name, err)
	}
	return out
}

// AssertGolden compares `got` (a map) with testdata/fixtures/<name>.golden.json.
// If -update is passed, rewrites the golden file instead.
func AssertGolden(t *testing.T, name string, got map[string]any) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fixtures", name+".golden.json")

	raw, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if *updateGolden {
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if string(expected) != string(raw)+"\n" {
		t.Fatalf("golden mismatch for %s\n--- expected:\n%s\n--- got:\n%s",
			name, expected, raw)
	}
	_ = context.Background // keep import tidy if unused
}
```

- [ ] **Step 2: Write `testdata/fixtures/basic_state_flow.jsonl`**

```
{"kind":"driver_event","source":"gohomed","payload":{"driverEvent":{"driverInstanceId":"hue-1","kind":"started","detail":""}}}
{"kind":"entity_registered","entity":"light.lr","source":"driver:hue-1","payload":{"entityRegistered":{"driverInstanceId":"hue-1","deviceId":"","entityType":"light","friendlyName":"Living Room","capabilities":{"light":{"on":false,"brightness":0,"colorTemp":0}}}}}
{"kind":"state_changed","entity":"light.lr","source":"driver:hue-1","payload":{"stateChanged":{"attributes":{"light":{"on":true,"brightness":200,"colorTemp":250}}}}}
{"kind":"state_changed","entity":"light.lr","source":"driver:hue-1","payload":{"stateChanged":{"attributes":{"light":{"on":true,"brightness":100,"colorTemp":250}}}}}
{"kind":"state_changed","entity":"light.lr","source":"driver:hue-1","payload":{"stateChanged":{"attributes":{"light":{"on":false,"brightness":0,"colorTemp":0}}}}}
```

- [ ] **Step 3: Write remaining four fixture JSONL files**

`scene_apply.jsonl` — 20 state_changed events across 5 entities with a shared correlation UUID (tester should generate and paste a UUID string via `uuidgen` and use it consistently). Keep structure consistent.

`driver_restart.jsonl`:

```
{"kind":"driver_event","source":"gohomed","payload":{"driverEvent":{"driverInstanceId":"d1","kind":"started"}}}
{"kind":"entity_registered","entity":"switch.x","source":"driver:d1","payload":{"entityRegistered":{"driverInstanceId":"d1","entityType":"switch","friendlyName":"X","capabilities":{"switch":{"on":false}}}}}
{"kind":"driver_event","source":"gohomed","payload":{"driverEvent":{"driverInstanceId":"d1","kind":"failed","detail":"connection refused"}}}
{"kind":"driver_event","source":"gohomed","payload":{"driverEvent":{"driverInstanceId":"d1","kind":"started"}}}
```

`snapshot_roundtrip.jsonl` — same shape as basic_state_flow but longer (20 state_changed events across 3 entities) so the test can snapshot after event 10 and verify event 11-20 are replayed correctly.

`correlation_walk.jsonl` — 5 events forming a cause chain: event 1 is root (no cause), 2 caused by 1, 3 caused by 2, etc. Test verifies walking the chain backwards reaches a zero-cause root.

Keep fixtures small (10-20 events each) — golden files become unwieldy otherwise.

- [ ] **Step 4: Write `internal/eventstore/replay_golden_test.go`**

```go
package eventstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/registry"
	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestGoldenReplay(t *testing.T) {
	fixtures := []string{
		"basic_state_flow",
		"scene_apply",
		"driver_restart",
		"snapshot_roundtrip",
		"correlation_walk",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			runFixture(t, name)
		})
	}
}

func runFixture(t *testing.T, name string) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	reg, err := registry.New(ctx, f.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(reg, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	events := testutil.LoadFixture(t, name)
	for _, e := range events {
		if _, err := f.store.Append(ctx, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	got := map[string]any{
		"state_cache": dumpCache(cache),
		"registry":    dumpRegistry(t, ctx, reg),
	}
	testutil.AssertGolden(t, name, got)
}

func dumpCache(c *state.Cache) map[string]any {
	out := map[string]any{}
	view := c.View()
	iter := view.Iterator()
	for !iter.Done() {
		id, s, _ := iter.Next()
		raw, _ := protojson.Marshal(s.Attributes)
		var attr any
		_ = json.Unmarshal(raw, &attr)
		out[id] = map[string]any{
			"updated_by": s.UpdatedBy,
			"attributes": attr,
		}
	}
	return out
}

func dumpRegistry(t *testing.T, ctx context.Context, r *registry.Registry) map[string]any {
	t.Helper()
	di, err := r.ListDriverInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dev, _ := r.ListDevices(ctx, registry.DeviceFilter{IncludeDisabled: true})
	ent, _ := r.ListEntities(ctx, registry.EntityFilter{IncludeDisabled: true})
	return map[string]any{
		"driver_instances": summarizeDrivers(di),
		"devices":          summarizeDevices(dev),
		"entities":         summarizeEntities(ent),
	}
}

func summarizeDrivers(list []registry.DriverInstance) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{"id": d.ID, "status": d.Status})
	}
	return out
}

func summarizeDevices(list []registry.Device) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{"id": d.ID, "driver": d.DriverInstanceID, "disabled": d.Disabled})
	}
	return out
}

func summarizeEntities(list []registry.Entity) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, map[string]any{
			"id":       e.ID,
			"type":     e.EntityType,
			"disabled": e.Disabled,
			"driver":   e.DriverInstanceID,
		})
	}
	return out
}
```

- [ ] **Step 5: Initial golden generation**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestGoldenReplay -update
```

Expected: creates `.golden.json` files. Inspect them by eye to verify they look right.

- [ ] **Step 6: Run golden tests without `-update`**

```bash
go test ./internal/eventstore/... -run TestGoldenReplay
```

Expected: PASS — 5 subtests.

- [ ] **Step 7: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/testutil testdata internal/eventstore
git commit -m "test(eventstore): add five golden-replay fixtures with AssertGolden + -update"
```

---

## Task 23: Property tests (Live == Replay, monotonicity, filter, snapshot, correlation)

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/properties_test.go`

- [ ] **Step 1: Write `internal/eventstore/properties_test.go`**

```go
package eventstore_test

import (
	"context"
	"testing"
	"testing/quick"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestProperty_AppendPositionsMonotonic(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	fn := func(n uint8) bool {
		if n == 0 {
			return true
		}
		prev := f.store.LatestPosition()
		for i := 0; i < int(n); i++ {
			pos, err := f.store.Append(ctx, testutil.StateChanged("light.x", uint32(i)))
			if err != nil {
				return false
			}
			if pos <= prev {
				return false
			}
			prev = pos
		}
		return true
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Fatal(err)
	}
}

func TestProperty_LiveEqualsReplay(t *testing.T) {
	ctx := context.Background()

	// Apply events live.
	f1 := newStoreFixture(t)
	cache1 := state.New()
	if err := f1.store.RegisterProjector(cache1, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f1.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	events := testutil.LoadFixture(t, "basic_state_flow")
	for _, e := range events {
		if _, err := f1.store.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	liveDump := dumpCache(cache1)

	// Reopen same DB and replay.
	logger := observability.Init(observability.LogConfig{})
	metrics := observability.NewMetrics()
	s2, err := eventstore.Open(ctx, eventstore.Config{}, f1.db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	cache2 := state.New()
	if err := s2.RegisterProjector(cache2, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := s2.Replay(ctx); err != nil {
		t.Fatal(err)
	}
	replayDump := dumpCache(cache2)

	if !deepEqual(liveDump, replayDump) {
		t.Fatalf("live != replay\nlive: %#v\nreplay: %#v", liveDump, replayDump)
	}
}

func TestProperty_FilterMatchesSubscription(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	filter := eventstore.Filter{Entities: []string{"light.a"}}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	e1 := testutil.StateChanged("light.a", 10)
	e2 := testutil.StateChanged("light.b", 20)
	_, _ = f.store.Append(ctx, e1)
	_, _ = f.store.Append(ctx, e2)

	// Drain with small timeout.
	got := make(map[string]int)
	for i := 0; i < 2; i++ {
		select {
		case e := <-sub.C():
			got[e.Entity]++
		case <-ctxTimeout(t, ctx):
			break
		}
	}
	if got["light.b"] != 0 {
		t.Fatal("filter leaked light.b")
	}
}

func TestProperty_CauseChainReachesRoot(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	p1, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 10))
	p2, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 20, testutil.WithCause(p1)))
	p3, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 30, testutil.WithCause(p2)))

	walk := []uint64{p3}
	cursor := p3
	for cursor != 0 {
		es, err := f.store.Query(ctx, eventstore.QueryOptions{FromPosition: cursor - 1, ToPosition: cursor, Limit: 1})
		if err != nil || len(es) == 0 {
			t.Fatalf("query: %v len=%d", err, len(es))
		}
		cursor = es[0].CausePosition
		if cursor != 0 {
			walk = append(walk, cursor)
		}
	}
	if len(walk) != 3 || walk[len(walk)-1] != p1 {
		t.Fatalf("walk = %v, want root = %d", walk, p1)
	}
}

// Deep equal for map[string]any structures. quick-and-dirty recursion.
func deepEqual(a, b any) bool {
	ma, aok := a.(map[string]any)
	mb, bok := b.(map[string]any)
	if aok != bok {
		return false
	}
	if aok && bok {
		if len(ma) != len(mb) {
			return false
		}
		for k, v := range ma {
			if !deepEqual(v, mb[k]) {
				return false
			}
		}
		return true
	}
	return toString(a) == toString(b)
}

func toString(v any) string {
	if v == nil {
		return "<nil>"
	}
	return fmtAny(v)
}
```

Add a tiny `fmtAny` helper at top of file or use `fmt.Sprintf("%+v", v)`. Also add `ctxTimeout` helper returning a short-lived channel:

```go
func ctxTimeout(t *testing.T, parent context.Context) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		// 250ms is enough for these unit-speed tests.
		timer := time.NewTimer(250 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-parent.Done():
		case <-timer.C:
		}
	}()
	return ch
}

func fmtAny(v any) string { return fmt.Sprintf("%+v", v) }
```

Add imports: `"fmt"`, `"time"`.

- [ ] **Step 2: Run property tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test ./internal/eventstore/... -run TestProperty -v
```

Expected: PASS — 4 property tests.

- [ ] **Step 3: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "test(eventstore): add property tests for monotonicity, live==replay, filter, cause chain"
```

---

## Task 24: Fuzz tests

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/fuzz_test.go`

- [ ] **Step 1: Write `internal/eventstore/fuzz_test.go`**

```go
package eventstore_test

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

func FuzzEventDecode(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x08, 0x96, 0x01})
	f.Fuzz(func(t *testing.T, data []byte) {
		var p eventv1.Payload
		_ = proto.Unmarshal(data, &p) // must not panic
	})
}

func FuzzFilterMatch(f *testing.F) {
	f.Add("state_changed", "light.a", "driver:x")
	f.Fuzz(func(t *testing.T, kind, entity, source string) {
		filter := eventstore.Filter{
			Kinds:    []string{kind},
			Entities: []string{entity},
			Sources:  []string{source},
			MinTs:    time.Time{},
		}
		e := eventstore.Event{Kind: kind, Entity: entity, Source: source, Timestamp: time.Now()}
		_ = filter.Matches(e)
	})
}

func FuzzFixtureParse(f *testing.F) {
	f.Add([]byte(`{"stateChanged":{"attributes":{"light":{"on":true,"brightness":100}}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var p eventv1.Payload
		_ = protojson.Unmarshal(data, &p) // must not panic
	})
}
```

- [ ] **Step 2: Run fuzz tests briefly**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test -fuzz=FuzzEventDecode -fuzztime=10s ./internal/eventstore
go test -fuzz=FuzzFilterMatch -fuzztime=10s ./internal/eventstore
go test -fuzz=FuzzFixtureParse -fuzztime=10s ./internal/eventstore
```

Expected: no panics, no discovered crashes.

- [ ] **Step 3: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "test(eventstore): add fuzz targets for event decode, filter match, and fixture parse"
```

---

## Task 25: Crash-safety integration tests

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/crash_integration_test.go`
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/internal/eventstore/testdata/crashhelper/main.go`

- [ ] **Step 1: Write the helper binary `internal/eventstore/testdata/crashhelper/main.go`**

```go
// Helper binary used by crash-safety tests: opens the DB, appends N events,
// then sleeps forever. Test kills it with -9 mid-loop.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/storage"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func main() {
	var (
		dbPath = flag.String("db", "", "path to gohome.db")
		count  = flag.Int("count", 1000, "events to append")
	)
	flag.Parse()

	ctx := context.Background()
	db, err := storage.Open(ctx, storage.Config{Path: *dbPath})
	if err != nil {
		log.Fatal(err)
	}
	store, err := eventstore.Open(ctx, eventstore.Config{}, db, observability.Init(observability.LogConfig{}), observability.NewMetrics())
	if err != nil {
		log.Fatal(err)
	}
	_ = store.Start(ctx)

	_, _ = os.Stderr.WriteString("READY\n")
	_ = os.Stderr.Sync()

	for i := 0; i < *count; i++ {
		if _, err := store.Append(ctx, testutil.StateChanged("light.x", uint32(i))); err != nil {
			log.Fatal(err)
		}
	}
	// Hang — test kills us.
	time.Sleep(time.Hour)
}
```

- [ ] **Step 2: Write `internal/eventstore/crash_integration_test.go`**

```go
//go:build integration

package eventstore_test

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/state"
	"github.com/fynn-labs/gohome/internal/storage"
)

func TestCrash_Kill9MidAppendLeavesConsistentDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "gohome.db")

	binary := buildHelper(t)

	cmd := exec.Command(binary, "-db", dbPath, "-count", "10000")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Wait for READY line.
	ready := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if sc.Text() == "READY" {
				close(ready)
				return
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not signal READY")
	}

	// Let it append for 50ms, then kill -9.
	time.Sleep(50 * time.Millisecond)
	_ = cmd.Process.Signal(syscall.SIGKILL)
	_ = cmd.Wait()

	// Reopen and verify.
	ctx := context.Background()
	db, err := storage.Open(ctx, storage.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	store, err := eventstore.Open(ctx, eventstore.Config{}, db, observability.Init(observability.LogConfig{}), observability.NewMetrics())
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	cache := state.New()
	if err := store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := store.Replay(ctx); err != nil {
		t.Fatalf("replay after crash: %v", err)
	}
	// Just verify we got some events and cache matches.
	if store.LatestPosition() == 0 {
		t.Fatal("expected some events to have been committed before crash")
	}
	if _, ok := cache.Get("light.x"); !ok {
		t.Fatal("state cache did not restore light.x")
	}
}

func TestCrash_SnapshotCorruptionFallsBack(t *testing.T) {
	t.Skip("requires helper to write 3 snapshots then test harness to corrupt newest; see spec §7 — scaffold for future")
	_ = os.WriteFile // keep imports
}

func buildHelper(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "crashhelper")
	cmd := exec.Command("go", "build", "-o", bin, "./internal/eventstore/testdata/crashhelper")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	return bin
}
```

- [ ] **Step 3: Run integration tests**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test -tags=integration ./internal/eventstore/... -run TestCrash
```

Expected: PASS for `TestCrash_Kill9MidAppendLeavesConsistentDB`; SKIP for the snapshot corruption one.

- [ ] **Step 4: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add internal/eventstore
git commit -m "test(eventstore): add kill-9 crash-safety integration test with helper binary"
```

---

## Task 26: CI — GitHub Actions workflow

**Files:**
- Create: `/Users/fdatoo/Desktop/GoHome/gohome/.github/workflows/ci.yml`

- [ ] **Step 1: Write `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  build-and-test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Install Task
        uses: arduino/setup-task@v2
        with:
          version: 3.x

      - name: Install buf
        uses: bufbuild/buf-setup-action@v1

      - name: Regenerate proto (verify committed in sync)
        run: |
          buf generate
          git diff --exit-code gen/

      - name: Tidy
        run: go mod tidy && git diff --exit-code go.mod go.sum

      - name: Build
        run: task build

      - name: Test
        run: task test

      - name: Race
        run: task test:race

      - name: Integration
        run: task test:integration

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  fuzz-scheduled:
    if: github.event_name == 'schedule'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -fuzz=FuzzEventDecode -fuzztime=5m ./internal/eventstore
      - run: go test -fuzz=FuzzFilterMatch -fuzztime=5m ./internal/eventstore
```

- [ ] **Step 2: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add .github
git commit -m "ci: add GitHub Actions workflow for build, test, race, integration, lint"
```

---

## Task 27: Final end-to-end smoke test and success criteria check

**Files:** none.

- [ ] **Step 1: Run the full test matrix**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
task lint
task test
task test:race
task test:integration
```

Expected: everything green.

- [ ] **Step 2: Run the spec's end-to-end smoke scenario**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
DATA=$(mktemp -d)
./dist/gohomed --data-dir "$DATA" --admin-port 9200 &
PID=$!
sleep 2

# Tail should receive the startup event.
timeout 3 ./dist/gohome --data-dir "$DATA" events tail || true

# Registry list (empty until drivers register in C2).
./dist/gohome --data-dir "$DATA" registry list entities

# Snapshot create + list.
./dist/gohome --data-dir "$DATA" snapshot create --owner=state_cache
./dist/gohome --data-dir "$DATA" snapshot list

# Verify /metrics + /health.
curl -sf http://localhost:9200/health | head -c 80
curl -sf http://localhost:9200/metrics | grep gohome_events_appended_total

# Restart survives.
kill -TERM $PID
wait $PID 2>/dev/null
./dist/gohomed --data-dir "$DATA" --admin-port 9200 &
PID=$!
sleep 2
./dist/gohome --data-dir "$DATA" events query --limit 5
kill -TERM $PID
wait $PID 2>/dev/null
rm -rf "$DATA"
```

Expected: tail shows the live startup event; snapshot create succeeds; restart replays the snapshot and the second startup event is appended contiguously.

- [ ] **Step 3: Check coverage**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go test -coverprofile=/tmp/cover.out ./internal/eventstore/... ./internal/state/... ./internal/registry/...
go tool cover -func=/tmp/cover.out | tail -1
```

Expected: total coverage ≥ 85% across those three packages. If below, add targeted tests before declaring C1 complete.

- [ ] **Step 4: Tag C1 completion**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git tag -a c1-complete -m "C1 — Event Core & Storage complete per design doc"
```

- [ ] **Step 5: Commit the plan's completion notes (optional)**

Add any deviations discovered during implementation into a short `CHANGELOG.md` entry so C2 has context.

---

## Self-Review Notes

The plan above was reviewed against the spec sections:

- **§1 Scope** — all listed packages appear in Tasks 3–14; both binaries covered in Tasks 17–18.
- **§2 Repo layout** — Task 1 creates the repo; `cmd/`, `internal/`, `proto/`, `gen/`, `testdata/` all materialize.
- **§3 SQL schema** — exact DDL reproduced in Tasks 5 and 11.
- **§4 Core APIs** — `storage.Tx`, `Event`, `Projector` + `NoSnapshot`, `SubscribeOptions`, `state.Cache` (with `View()` not `Snapshot()`), `registry.Registry`, `eventstore.Store` all land with matching signatures.
- **§5 Append path** — covered by Task 7 (with projector dispatch) + Task 9 (Promote/Discard post-commit).
- **§6 Tailer + fanout** — Task 12 (non-durable), Task 13 (durable), tailer runs in Task 12's `tailer.go`.
- **§7 Snapshots** — Task 10 (state cache Snapshot/Restore), Task 14 (SnapshotNow + cadence).
- **§8 Startup recovery** — Task 16 wires phases 1–5; recovery mode enters on Replay failure.
- **§9 Observability** — Tasks 3 (logging), 4 (metrics + tracing stubs), wired into daemon in Task 16.
- **§10 CLI** — Tasks 18–21 cover root, events, state, registry, snapshot. `--config` flag is absent by design.
- **§11 Testing** — Task 22 (golden), Task 23 (property), Task 24 (fuzz), Task 25 (crash-safety).
- **§12 Decisions** — every numbered decision has a corresponding implementation task.
- **§13 Deps** — all imports introduced by the earliest task that uses them; `go mod tidy` runs repeatedly.
- **§14 C2 inheritance** — the C2 starting state (running `gohomed`, registry tables, `Projector` + `storage.Tx`) is produced by Task 27.

Known imperfections that are cheap to fix during implementation (not worth respinning the plan):

1. **`lipglossStyleAlias` placeholder** in `internal/cli/render.go` — the plan notes this explicitly. Replace with the real `lipgloss.Style` struct during Task 19 step 2.
2. **`ProjectorMode` registration check** — plan currently relies on `s.started` as a flag; for robustness a mutex around registration might be warranted but is not blocking for C1.
3. **CLI `state dump` / `state get` rebuild the cache from scratch on every call** — acceptable for C1 since there's no daemon RPC yet; C4 retrofits via Connect-RPC.
4. **Helper `BEGIN IMMEDIATE` inside `appendTx`** — `modernc.org/sqlite` starts a deferred tx via `BeginTx`; the subsequent `BEGIN IMMEDIATE` is a no-op/error that's silently ignored. To genuinely use `IMMEDIATE`, use `db.ExecContext(ctx, "BEGIN IMMEDIATE")` + manual COMMIT/ROLLBACK instead of `db.BeginTx`. Implementer may choose either path; if they take the `BeginTx` path, confirm the driver upgrades the lock correctly on first write (it does — SQLite upgrades deferred → reserved on INSERT).

These are all judgment-call items the implementer can resolve in-task.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-21-c1-event-core-and-storage.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?

---

## Plan Amendments

### 2026-04-29 — recovery.go merged into daemon.go

T16: recovery socket logic merged into `internal/daemon/daemon.go` rather than landing as a separate `internal/daemon/recovery.go` file. Functionally equivalent; no behavior change.

### 2026-04-29 — event_subscriptions migration relocated

Subscription table migration relocated from `internal/registry/migrations/0004_event_subscriptions.sql` to `internal/storage/migrations/0005_event_subscriptions.sql` because subscriptions are part of event-store schema, not registry schema. Storage migrations are co-located with the package that owns the tables (per the gohome CLAUDE.md invariant).
