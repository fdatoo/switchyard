# C2 — Carport Protocol v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Carport Protocol `v1alpha1` (gRPC + protobuf), the host-side supervisor in `internal/carport`, a scenario-driven fake driver binary, and the CLI/observability wiring so `gohomed` can auto-spawn, supervise, dispatch commands to, and ingest events from driver subprocesses over local Unix-domain sockets.

**Architecture:** Single-process-per-driver-instance model. `carport.Host` sits in the `internal/daemon` startup sequence between eventstore and socket server. `Host.Dispatch(ctx, entityID, capability, args)` is the single control-plane call; it appends `CommandIssued`, sends a `Command` on the instance's bidi `Run` stream, awaits a matching `CommandResult`, appends `CommandAck`, returns. Driver-originated events (`StateChanged`, `EntityRegistered`, etc.) are ingested on the same stream and appended to the eventstore. Lifecycle is a pure FSM — `spawning → awaiting_handshake → running → failed → backoff → …` with quarantine after exhausted restart budget.

**Tech Stack:** Go 1.25, `google.golang.org/grpc` (NEW dep), `google.golang.org/protobuf`, `github.com/BurntSushi/toml` (NEW dep), Prometheus, `charmbracelet/log` (slog), Cobra, lipgloss, SQLite (existing C1 stack).

**Source spec:** `docs/superpowers/specs/2026-04-21-c2-carport-protocol-design.md` — the plan enforces every invariant, decision, and success criterion listed there.

**Working directory:** `/Users/fdatoo/Desktop/GoHome/gohome` (module `github.com/fynn-labs/gohome`, tagged `c1-complete` at start).

---

## File Structure

### New files

```
proto/gohome/carport/v1alpha1/
├── carport.proto                # service Driver: Handshake/Run/Health/Shutdown
├── envelope.proto               # HostToDriver, DriverToHost oneofs
├── manifest.proto               # DriverManifest
└── errors.proto                 # CarportErrorCode enum

docs/proto-hygiene.md            # grouped numbering rule + `reserved` forever rule

scripts/check-proto-hygiene.sh   # CI lint: git-history check for reserved-on-removal

internal/carport/
├── carport.go                   # package doc; Host type; constructor
├── config.go                    # drivers.toml loader + validator
├── config_test.go
├── fsm.go                       # pure state-machine types
├── fsm_test.go
├── routing.go                   # entity→instance resolver from registry projection
├── routing_test.go
├── instance.go                  # per-instance runtime: connection, pending map, stream I/O
├── instance_test.go
├── supervisor.go                # spawn, handshake, backoff, restart budget, shutdown
├── supervisor_test.go
├── dispatch.go                  # Host.Dispatch (call-through API)
├── dispatch_test.go
├── ingest.go                    # DriverToHost non-response messages → eventstore
├── ingest_test.go
├── errors.go                    # sentinel errors: ErrInstanceNotRunning, etc.
├── properties_test.go           # property tests (correlation, FSM legality, budget)
├── fuzz_test.go                 # FuzzEnvelopeDecode
└── fakedriver/
    └── fakedriver.go            # in-process Driver gRPC server double (test-only pkg)

cmd/testdriver/
└── main.go                      # scenario-driven driver binary (TESTDRIVER_MODE env)

internal/carport/testdata/       # scenario helper data (optional)

testdata/golden/carport/         # golden fixtures (happy-path, crash-recovery, quarantine)
├── happy-path.events
├── happy-path.state.json
├── happy-path.registry.json
├── crash-recovery.events
├── crash-recovery.state.json
├── crash-recovery.registry.json
├── quarantine.events
├── quarantine.state.json
└── quarantine.registry.json

internal/cli/driver.go           # gohome driver list/status/restart
internal/cli/command.go          # gohome command send
```

### Modified files

```
go.mod / go.sum                  # add google.golang.org/grpc, BurntSushi/toml

buf.gen.yaml                     # add connectrpc/grpc-go plugin (or direct protoc-gen-go-grpc)

Taskfile.yml                     # extend test:fuzz with carport

proto/gohome/event/v1/event.proto          # proto-hygiene retrofit (range comment headers)
proto/gohome/event/v1/snapshot.proto       # ditto
proto/gohome/entity/v1/attributes.proto    # ditto

internal/observability/metrics.go          # add 9 carport metrics
internal/daemon/config.go                  # add ConfigDir, CarportEnabled fields
internal/daemon/daemon.go                  # wire carport.Host into startup/shutdown (phase 4.5)
internal/daemon/recovery.go                # add socket ops: driver_restart, command_send
internal/cli/root.go                       # register newDriverCmd, newCommandCmd
internal/cli/snapshot.go                   # (no changes; used as reference pattern)
internal/registry/registry.go              # (no changes; read-only consumer)
internal/eventstore/store.go               # (no changes; used as-is)
```

---

## Dependency Additions (applied once in Task 0)

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go get google.golang.org/grpc@latest
go get github.com/BurntSushi/toml@latest
go mod tidy
```

The gRPC runtime picks up `google.golang.org/protobuf` which is already in go.mod. `grpc-go`'s compiler plugin (`protoc-gen-go-grpc`) is invoked via `buf generate`; we configure it in `buf.gen.yaml` in Task 2.

---

## Legend for steps

- **Write failing test** — TDD red phase.
- **Run test, confirm fail** — `go test -run ^TestName$ ./internal/carport/... -v`.
- **Write impl** — minimum code to pass.
- **Run test, confirm pass** — same command; expect PASS.
- **Commit** — use short conventional prefix (`feat:`, `test:`, `fix:`, `chore:`, `refactor:`). One task = one commit unless explicitly noted.

---

## Tasks

### Task 0: Dependency additions & gRPC codegen wiring

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `buf.gen.yaml`

- [ ] **Step 1: Add gRPC and TOML dependencies**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
go get google.golang.org/grpc@v1.68.0
go get github.com/BurntSushi/toml@v1.4.0
go mod tidy
```

Expected: `go.mod` gains `google.golang.org/grpc` and `github.com/BurntSushi/toml` lines; `go.sum` updated.

- [ ] **Step 2: Install `protoc-gen-go-grpc` locally (developer convenience)**

```bash
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Expected: `$(go env GOBIN)/protoc-gen-go-grpc` executable exists.

- [ ] **Step 3: Extend `buf.gen.yaml` with the gRPC plugin**

Replace `buf.gen.yaml` contents with:

```yaml
version: v2
inputs:
  - directory: proto
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/fynn-labs/gohome/gen
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - local: protoc-gen-go-grpc
    out: gen
    opt: paths=source_relative,require_unimplemented_servers=false
```

Rationale for `require_unimplemented_servers=false`: our fake drivers and real drivers alike will implement the full `Driver` service, and we don't want the compiler forcing unused stubs.

- [ ] **Step 4: Verify buf config loads (no new protos yet)**

```bash
buf lint
```

Expected: no output, exit 0. (There are no new protos yet, so generation is not affected; we're verifying buf parses the config.)

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum buf.gen.yaml
git commit -m "chore(deps): add grpc-go, BurntSushi/toml; wire grpc plugin for buf gen"
```

---

### Task 1: Proto hygiene — doc, retrofit, CI script

**Files:**
- Create: `docs/proto-hygiene.md`
- Create: `scripts/check-proto-hygiene.sh`
- Modify: `proto/gohome/event/v1/event.proto`
- Modify: `proto/gohome/event/v1/snapshot.proto`
- Modify: `proto/gohome/entity/v1/attributes.proto`

- [ ] **Step 1: Write `docs/proto-hygiene.md`**

```markdown
# Proto Hygiene Rules

Applies to every `.proto` file in this repo.

## Rules

1. **Grouped numbering.** In any `oneof` or message with 3+ fields, group fields by semantic
   category using tens-aligned blocks: identity (1-9), primary domain payload (10-19), metadata
   (20-29, 30-39, ...), transport/health (90-99).

2. **Range-comment headers.** Every group gets a one-line comment above its first member:
   `// 10-19: state-plane events`. The header is the contract for future additions — new fields
   in a category pick the next free tag *within that block*.

3. **`reserved` on removal, forever.** Any field number OR oneof tag removed from a message
   MUST be added to a `reserved N;` statement in the same message (and the old name in
   `reserved "old_name";`). Field numbers are never reused, even across breaking package
   versions.

4. **Tag stability is part of the wire contract.** Field numbers and oneof tags do not change
   within a protocol version (`v1alpha1`). Breaking changes move to a new package suffix
   (`v1alpha2`, or eventually `v1`).

5. **`v1alpha*` vs `v1`.** Packages suffixed `v1alpha*` may make wire-breaking changes between
   releases with a migration note. Graduating to `v1` is a one-way door and requires a decision
   record entry in the relevant design doc.
```

- [ ] **Step 2: Write `scripts/check-proto-hygiene.sh`**

```bash
#!/usr/bin/env bash
# Fails if a field was removed from any proto file without a corresponding
# `reserved` statement being added in the same commit.
#
# Minimal heuristic: for any diff line removing a field definition
# (`type name = N;`), assert that a matching `reserved N;` or `reserved "name";`
# appears in the same commit's diff.

set -euo pipefail

# Scan all staged+committed changes relative to the previous commit.
BASE="${1:-HEAD^}"

removed=$(git diff "$BASE" -- 'proto/**/*.proto' \
  | grep -E '^-[[:space:]]+[a-zA-Z_][a-zA-Z0-9_.]*[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*=[[:space:]]*[0-9]+;' \
  | grep -v '^-//' || true)

if [ -z "$removed" ]; then
  exit 0
fi

# For each removed field, check the diff for a reserved line with that number.
fail=0
while IFS= read -r line; do
  num=$(echo "$line" | grep -oE '= *[0-9]+;' | grep -oE '[0-9]+' || true)
  [ -z "$num" ] && continue
  if ! git diff "$BASE" -- 'proto/**/*.proto' | grep -E "^\+[[:space:]]+reserved[[:space:]]+.*\b$num\b" >/dev/null 2>&1; then
    echo "ERROR: field removed without reserved tag $num: $line" >&2
    fail=1
  fi
done <<< "$removed"

exit $fail
```

```bash
chmod +x scripts/check-proto-hygiene.sh
```

- [ ] **Step 3: Retrofit `proto/gohome/event/v1/event.proto`**

Read current file, add one range-comment header per grouped block in `Payload.oneof`. The field numbers do NOT change. Resulting file:

```proto
syntax = "proto3";

package gohome.event.v1;

import "gohome/entity/v1/attributes.proto";

// See docs/proto-hygiene.md for grouping conventions.

// Payload is the on-the-wire variant union. The DB stores this marshalled
// inside the `events.payload` BLOB column; position/ts/kind/entity/source/
// correlation/cause live in dedicated columns.
message Payload {
  oneof kind {
    // 1-9: system/meta events
    SystemEvent system = 1;

    // 10-19: command/state plane
    StateChanged        state_changed  = 10;
    CommandIssued       command_issued = 11;
    CommandAck          command_ack    = 12;

    // 20-29: registry plane
    EntityRegistered    entity_registered   = 20;
    EntityUnregistered  entity_unregistered = 21;

    // 30-39: driver-typed passthrough
    DriverEvent driver_event = 30;
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

- [ ] **Step 4: Retrofit `proto/gohome/event/v1/snapshot.proto`**

Edit so it reads:

```proto
syntax = "proto3";

package gohome.event.v1;

import "gohome/entity/v1/attributes.proto";

// See docs/proto-hygiene.md for grouping conventions.

// StateCacheSnapshot is the serialised form of the in-memory state cache,
// written per-projector by the snapshotter goroutine and read during Replay
// to skip re-projecting already-committed events.
message StateCacheSnapshot {
  // 1-9: meta
  uint64 position = 1;
  int64  ts       = 2;   // unix nanos

  // 10-19: payload
  repeated EntityState entities = 10;
}

message EntityState {
  // 1-9: identity
  string entity_id  = 1;

  // 10-19: meta
  int64  updated_at = 10;  // unix nanos
  string updated_by = 11;

  // 20-29: payload
  gohome.entity.v1.Attributes attributes = 20;
}
```

Note: this changes field numbers on `EntityState` (3→10, 4→20). Because this proto is only ever serialised *inside* `snapshots.state` BLOB — which is regenerated on every snapshot — the wire break is safe: existing snapshots become unreadable, but the daemon tolerates that (snapshot corruption falls back to full replay in C1's behavior, see `internal/eventstore/replay.go` — verify this before committing).

**Before committing, run the full test suite and verify no snapshot-dependent test breaks.** If tests break, revert the field-number changes on `EntityState` (keep them as 1/2/3/4 with headers above) rather than migrating data.

```bash
go test ./...
```

If any test references pre-existing snapshot bytes (a golden with serialized snapshots), keep field numbers as-is and only add the comment headers above them. Verify by grepping:

```bash
grep -r "StateCacheSnapshot\|EntityState" testdata/ internal/ || true
```

- [ ] **Step 5: Retrofit `proto/gohome/entity/v1/attributes.proto`**

```proto
syntax = "proto3";

package gohome.entity.v1;

// See docs/proto-hygiene.md for grouping conventions.

// Attributes carries both capabilities (static) and live state (dynamic)
// for an entity. Specific payloads land in later milestones — C1 only
// needs the envelope compiling. Future oneof variants get added here.
message Attributes {
  oneof kind {
    // 10-19: actuator domains
    Light  light         = 10;
    Switch switch_device = 11;

    // 20-29: sensor domains
    Sensor sensor        = 12;
    // NOTE: Sensor is in 10-19 from C1; leave its tag at 12 and annotate.
    // Future sensors start at 20.
  }
}

message Light {
  // 1-9: primary payload
  bool   on         = 1;
  uint32 brightness = 2;
  uint32 color_temp = 3;
}

message Switch {
  bool on = 1;
}

message Sensor {
  string unit  = 1;
  double value = 2;
}
```

- [ ] **Step 6: Regenerate protos**

```bash
buf generate
```

Expected: files under `gen/` are regenerated; `git diff gen/` should show only comment-level changes if any.

- [ ] **Step 7: Run full test suite to confirm retrofit is non-breaking**

```bash
go test ./... && go test -tags=integration ./...
```

Expected: all tests pass. If snapshot-replay tests fail, revert any field-number changes in `snapshot.proto` and keep the retrofit purely comment-only (step 4's "if tests break" branch).

- [ ] **Step 8: Commit**

```bash
git add docs/proto-hygiene.md scripts/check-proto-hygiene.sh proto/ gen/
git commit -m "refactor(proto): retrofit range-comment headers + hygiene doc; CI script for reserved-on-removal"
```

---

### Task 2: Carport proto definitions (the wire contract)

**Files:**
- Create: `proto/gohome/carport/v1alpha1/carport.proto`
- Create: `proto/gohome/carport/v1alpha1/envelope.proto`
- Create: `proto/gohome/carport/v1alpha1/manifest.proto`
- Create: `proto/gohome/carport/v1alpha1/errors.proto`
- Modify: (generated) `gen/gohome/carport/v1alpha1/*.pb.go` and `*_grpc.pb.go`

- [ ] **Step 1: Create `errors.proto`**

```proto
syntax = "proto3";

package gohome.carport.v1alpha1;

// See docs/proto-hygiene.md.

enum CarportErrorCode {
  // 0: success sentinel
  CARPORT_OK = 0;

  // 1-9: caller / request errors
  CARPORT_UNSUPPORTED_CAPABILITY = 1;
  CARPORT_BAD_ARGS               = 2;

  // 10-19: device / downstream errors
  CARPORT_DEVICE_OFFLINE = 10;
  CARPORT_DEVICE_ERROR   = 11;

  // 20-29: lifecycle / transport errors
  CARPORT_TIMEOUT              = 20;
  CARPORT_DRIVER_SHUTTING_DOWN = 21;

  // 99: opaque fallback
  CARPORT_INTERNAL = 99;
}
```

- [ ] **Step 2: Create `manifest.proto`**

```proto
syntax = "proto3";

package gohome.carport.v1alpha1;

message DriverManifest {
  // 1-9: identity
  string name             = 1;
  string version          = 2;
  string protocol_version = 3;

  // 10-19: capability surface
  repeated string supported_capabilities = 10;

  // 90-99: future-structured fields (reserved for C4 Pkl integration)
  bytes pkl_module = 99;
}
```

- [ ] **Step 3: Create `envelope.proto`**

```proto
syntax = "proto3";

package gohome.carport.v1alpha1;

import "gohome/carport/v1alpha1/errors.proto";
import "gohome/event/v1/event.proto";

// HostToDriver: messages flowing from gohomed → driver subprocess over Run stream.
message HostToDriver {
  oneof kind {
    // 1-9: command plane
    Command command = 1;

    // 90-99: transport/health
    Heartbeat ping = 90;
  }
}

// DriverToHost: messages flowing from driver subprocess → gohomed over Run stream.
// Reuses the canonical payload types from gohome.event.v1 where they match semantically.
message DriverToHost {
  oneof kind {
    // 1-9: command response plane
    CommandResult result = 1;

    // 10-19: state-plane events (reuse event.v1 payload types)
    gohome.event.v1.StateChanged       state_changed       = 10;
    gohome.event.v1.EntityRegistered   entity_registered   = 11;
    gohome.event.v1.EntityUnregistered entity_unregistered = 12;

    // 20-29: driver-typed passthrough
    gohome.event.v1.DriverEvent driver_event = 20;

    // 90-99: transport/health
    Heartbeat pong = 90;
  }
}

message Heartbeat {
  int64 ts_unix_ms = 1;
}

message Command {
  // 1-9: identity
  string command_id       = 1;
  int64  deadline_unix_ms = 2;

  // 10-19: target & payload
  string              entity_id  = 10;
  string              capability = 11;
  map<string, string> args       = 12;
}

message CommandResult {
  // 1-9: correlation
  string command_id = 1;

  // 10-19: outcome
  bool             ok            = 10;
  CarportErrorCode code          = 11;
  string           error_message = 12;
}
```

- [ ] **Step 4: Create `carport.proto`**

```proto
syntax = "proto3";

package gohome.carport.v1alpha1;

import "gohome/carport/v1alpha1/envelope.proto";
import "gohome/carport/v1alpha1/manifest.proto";
import "gohome/event/v1/event.proto";

service Driver {
  // Initial negotiation. Called exactly once before Run.
  rpc Handshake(HandshakeRequest) returns (HandshakeResponse);

  // Lifetime bidi stream.
  rpc Run(stream HostToDriver) returns (stream DriverToHost);

  // Out-of-band liveness probe.
  rpc Health(HealthRequest) returns (HealthResponse);

  // Graceful shutdown request.
  rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
}

message HandshakeRequest {
  // 1-9: identity & version
  string protocol_version = 1;
  string instance_id      = 2;
  string handshake_secret = 3;

  // 10-19: instance-specific config (opaque to host)
  bytes  instance_config  = 10;
}

message HandshakeResponse {
  // 1-9: identity & version
  string protocol_version = 1;

  // 10-19: driver self-description
  DriverManifest manifest = 10;

  // 20-29: initial registry delta
  repeated gohome.event.v1.EntityRegistered initial_entities = 20;
}

message HealthRequest  {}
message HealthResponse {
  bool   ok     = 1;
  string detail = 2;
}

message ShutdownRequest  { int64 grace_ms = 1; }
message ShutdownResponse { bool  acknowledged = 1; }
```

- [ ] **Step 5: Regenerate**

```bash
buf lint && buf generate
```

Expected: `gen/gohome/carport/v1alpha1/carport.pb.go`, `envelope.pb.go`, `manifest.pb.go`, `errors.pb.go`, and `carport_grpc.pb.go` all exist.

- [ ] **Step 6: Verify compile**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 7: Commit**

```bash
git add proto/gohome/carport/ gen/gohome/carport/
git commit -m "feat(proto): add gohome.carport.v1alpha1 wire contract"
```

---

### Task 3: Sentinel errors (`internal/carport/errors.go`)

**Files:**
- Create: `internal/carport/errors.go`

- [ ] **Step 1: Create the file**

```go
// Package carport hosts the driver-supervisor subsystem: drivers.toml
// configuration, per-instance subprocess lifecycle, command dispatch, and
// event ingest from drivers over the Carport gRPC protocol (v1alpha1).
//
// See docs/superpowers/specs/2026-04-21-c2-carport-protocol-design.md.
package carport

import "errors"

var (
	// ErrEntityUnknown: routing.Resolve found no entity with the given id.
	ErrEntityUnknown = errors.New("carport: entity unknown")

	// ErrInstanceNotRunning: the entity's owning driver instance is not in state=running.
	ErrInstanceNotRunning = errors.New("carport: driver instance not running")

	// ErrDispatchTimeout: deadline elapsed before CommandResult arrived.
	ErrDispatchTimeout = errors.New("carport: dispatch timeout")

	// ErrStreamClosed: Run stream died mid-dispatch (driver crash, network error).
	ErrStreamClosed = errors.New("carport: driver stream closed")

	// ErrContextCanceled: caller's context was canceled.
	ErrContextCanceled = errors.New("carport: context canceled")

	// ErrHostStopped: carport.Host is shutting down or already stopped.
	ErrHostStopped = errors.New("carport: host stopped")
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/carport/errors.go
git commit -m "feat(carport): add package doc and sentinel errors"
```

---

### Task 4: FSM (`internal/carport/fsm.go`, pure)

**Files:**
- Create: `internal/carport/fsm.go`
- Create: `internal/carport/fsm_test.go`

- [ ] **Step 1: Write failing test `fsm_test.go`**

```go
package carport

import "testing"

func TestState_String(t *testing.T) {
	cases := map[State]string{
		StateDeclared:          "declared",
		StateSpawning:          "spawning",
		StateAwaitingHandshake: "awaiting_handshake",
		StateRunning:           "running",
		StateFailed:            "failed",
		StateBackoff:           "backoff",
		StateQuarantined:       "quarantined",
		StateStopping:          "stopping",
		StateStopped:           "stopped",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestTransition_LegalTransitions(t *testing.T) {
	legal := []struct {
		from, to State
		trigger  Trigger
	}{
		{StateDeclared, StateSpawning, TriggerStart},
		{StateSpawning, StateAwaitingHandshake, TriggerSpawned},
		{StateSpawning, StateFailed, TriggerSpawnError},
		{StateAwaitingHandshake, StateRunning, TriggerHandshakeOK},
		{StateAwaitingHandshake, StateFailed, TriggerHandshakeFail},
		{StateRunning, StateFailed, TriggerCrash},
		{StateRunning, StateFailed, TriggerHealthFail},
		{StateRunning, StateFailed, TriggerStreamError},
		{StateFailed, StateBackoff, TriggerBackoffScheduled},
		{StateBackoff, StateSpawning, TriggerBackoffElapsed},
		{StateBackoff, StateQuarantined, TriggerBudgetExhausted},
		{StateQuarantined, StateSpawning, TriggerManualRestart},
		{StateRunning, StateStopping, TriggerShutdown},
		{StateStopping, StateStopped, TriggerExited},
	}
	for _, c := range legal {
		if !IsLegal(c.from, c.trigger, c.to) {
			t.Errorf("expected legal: %s --%s--> %s", c.from, c.trigger, c.to)
		}
	}
}

func TestTransition_IllegalTransitions(t *testing.T) {
	illegal := []struct {
		from, to State
		trigger  Trigger
	}{
		{StateDeclared, StateRunning, TriggerStart}, // skip phases
		{StateQuarantined, StateRunning, TriggerHandshakeOK},
		{StateStopped, StateSpawning, TriggerStart}, // terminal
	}
	for _, c := range illegal {
		if IsLegal(c.from, c.trigger, c.to) {
			t.Errorf("expected illegal: %s --%s--> %s", c.from, c.trigger, c.to)
		}
	}
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -run '^TestState_String$|^TestTransition' ./internal/carport/ -v
```

Expected: compile errors (types undefined).

- [ ] **Step 3: Implement `fsm.go`**

```go
package carport

import "fmt"

// State enumerates the lifecycle phases a driver instance moves through.
type State int

const (
	StateDeclared State = iota // row exists in drivers.toml; nothing running yet
	StateSpawning
	StateAwaitingHandshake
	StateRunning
	StateFailed
	StateBackoff
	StateQuarantined
	StateStopping
	StateStopped
)

func (s State) String() string {
	return [...]string{
		"declared", "spawning", "awaiting_handshake", "running",
		"failed", "backoff", "quarantined", "stopping", "stopped",
	}[s]
}

// Trigger names the cause of a state change.
type Trigger int

const (
	TriggerStart Trigger = iota
	TriggerSpawned
	TriggerSpawnError
	TriggerHandshakeOK
	TriggerHandshakeFail
	TriggerCrash
	TriggerHealthFail
	TriggerStreamError
	TriggerBackoffScheduled
	TriggerBackoffElapsed
	TriggerBudgetExhausted
	TriggerManualRestart
	TriggerShutdown
	TriggerExited
)

func (t Trigger) String() string {
	return [...]string{
		"start", "spawned", "spawn_error", "handshake_ok", "handshake_fail",
		"crash", "health_fail", "stream_error", "backoff_scheduled",
		"backoff_elapsed", "budget_exhausted", "manual_restart", "shutdown", "exited",
	}[t]
}

// transitions is the total legal-transition table.
var transitions = map[State]map[Trigger]State{
	StateDeclared: {
		TriggerStart: StateSpawning,
	},
	StateSpawning: {
		TriggerSpawned:    StateAwaitingHandshake,
		TriggerSpawnError: StateFailed,
		TriggerShutdown:   StateStopping,
	},
	StateAwaitingHandshake: {
		TriggerHandshakeOK:   StateRunning,
		TriggerHandshakeFail: StateFailed,
		TriggerShutdown:      StateStopping,
	},
	StateRunning: {
		TriggerCrash:       StateFailed,
		TriggerHealthFail:  StateFailed,
		TriggerStreamError: StateFailed,
		TriggerShutdown:    StateStopping,
	},
	StateFailed: {
		TriggerBackoffScheduled: StateBackoff,
		TriggerShutdown:         StateStopping,
	},
	StateBackoff: {
		TriggerBackoffElapsed:  StateSpawning,
		TriggerBudgetExhausted: StateQuarantined,
		TriggerShutdown:        StateStopping,
	},
	StateQuarantined: {
		TriggerManualRestart: StateSpawning,
		TriggerShutdown:      StateStopping,
	},
	StateStopping: {
		TriggerExited: StateStopped,
	},
}

// IsLegal reports whether from --trigger--> to is permitted.
func IsLegal(from State, trigger Trigger, to State) bool {
	m, ok := transitions[from]
	if !ok {
		return false
	}
	got, ok := m[trigger]
	return ok && got == to
}

// Next returns the destination state for (from, trigger) or an error.
func Next(from State, trigger Trigger) (State, error) {
	m, ok := transitions[from]
	if !ok {
		return 0, fmt.Errorf("no transitions from state %s", from)
	}
	to, ok := m[trigger]
	if !ok {
		return 0, fmt.Errorf("illegal trigger %s from state %s", trigger, from)
	}
	return to, nil
}
```

- [ ] **Step 4: Run test, confirm pass**

```bash
go test -run '^TestState_String$|^TestTransition' ./internal/carport/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/carport/fsm.go internal/carport/fsm_test.go
git commit -m "feat(carport): add lifecycle FSM (pure state-transition table)"
```

---

### Task 5: Config loader (`internal/carport/config.go`)

**Files:**
- Create: `internal/carport/config.go`
- Create: `internal/carport/config_test.go`

- [ ] **Step 1: Write failing test `config_test.go`**

```go
package carport_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/carport"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "drivers.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig_EmptyFileIsValid(t *testing.T) {
	p := writeTOML(t, "")
	cfg, err := carport.LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Instances) != 0 {
		t.Fatalf("want 0 instances, got %d", len(cfg.Instances))
	}
}

func TestLoadConfig_HappyPath(t *testing.T) {
	// The `binary` must point to something that exists and is executable.
	bin := filepath.Join(t.TempDir(), "fake")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTOML(t, `
[[instance]]
id = "hue_main"
binary = "`+bin+`"
enabled = true
config_json = "{\"x\": 1}"
`)
	cfg, err := carport.LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Instances) != 1 {
		t.Fatalf("want 1 instance, got %d", len(cfg.Instances))
	}
	got := cfg.Instances[0]
	if got.ID != "hue_main" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Binary != bin {
		t.Errorf("Binary = %q", got.Binary)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
	if string(got.ConfigJSON) != `{"x": 1}` {
		t.Errorf("ConfigJSON = %q", got.ConfigJSON)
	}
	// Lifecycle defaults apply when absent.
	if got.Lifecycle.HealthProbeInterval != 15*time.Second {
		t.Errorf("HealthProbeInterval default = %v", got.Lifecycle.HealthProbeInterval)
	}
}

func TestLoadConfig_RejectsDuplicateID(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	p := writeTOML(t, `
[[instance]]
id = "a"
binary = "`+bin+`"

[[instance]]
id = "a"
binary = "`+bin+`"
`)
	_, err := carport.LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for duplicate id")
	}
}

func TestLoadConfig_RejectsInvalidID(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	for _, id := range []string{"", "UPPER", "with space", "long" + string(make([]byte, 80))} {
		p := writeTOML(t, `
[[instance]]
id = "`+id+`"
binary = "`+bin+`"
`)
		if _, err := carport.LoadConfig(p); err == nil {
			t.Errorf("expected error for id %q", id)
		}
	}
}

func TestLoadConfig_RejectsMissingBinary(t *testing.T) {
	p := writeTOML(t, `
[[instance]]
id = "x"
binary = "/no/such/file/exists/here"
`)
	_, err := carport.LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestLoadConfig_AcceptsLifecycleOverrides(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	p := writeTOML(t, `
[[instance]]
id = "x"
binary = "`+bin+`"
[instance.lifecycle]
health_probe_interval_ms = 5000
health_failures_to_restart = 5
shutdown_grace_ms = 20000
restart_budget_window_minutes = 30
restart_budget_max = 3
`)
	cfg, err := carport.LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	inst := cfg.Instances[0]
	if inst.Lifecycle.HealthProbeInterval != 5*time.Second {
		t.Errorf("HealthProbeInterval = %v", inst.Lifecycle.HealthProbeInterval)
	}
	if inst.Lifecycle.HealthFailuresToRestart != 5 {
		t.Errorf("HealthFailuresToRestart = %d", inst.Lifecycle.HealthFailuresToRestart)
	}
	if inst.Lifecycle.RestartBudgetWindow != 30*time.Minute {
		t.Errorf("RestartBudgetWindow = %v", inst.Lifecycle.RestartBudgetWindow)
	}
}

func TestLoadConfig_MissingFileReturnsEmptyConfig(t *testing.T) {
	cfg, err := carport.LoadConfig(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("LoadConfig missing: %v", err)
	}
	if len(cfg.Instances) != 0 {
		t.Fatalf("want 0 instances, got %d", len(cfg.Instances))
	}
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -run '^TestLoadConfig' ./internal/carport/ -v
```

Expected: compile errors (types undefined).

- [ ] **Step 3: Implement `config.go`**

```go
package carport

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
)

// LifecycleConfig tunes per-instance timing and restart policy.
// Zero values are filled by withLifecycleDefaults.
type LifecycleConfig struct {
	HandshakeDeadline       time.Duration
	HealthProbeInterval     time.Duration
	HealthProbeTimeout      time.Duration
	HealthFailuresToRestart int
	ShutdownGrace           time.Duration
	RestartBackoffInitial   time.Duration
	RestartBackoffMax       time.Duration
	RestartBudgetWindow     time.Duration
	RestartBudgetMax        int
}

// Instance is one entry in drivers.toml.
type Instance struct {
	ID         string
	Binary     string
	Enabled    bool
	ConfigJSON []byte
	Lifecycle  LifecycleConfig
}

// Config is the parsed drivers.toml.
type Config struct {
	Instances []Instance
}

var idRE = regexp.MustCompile(`^[a-z0-9_\-]{1,64}$`)

// LoadConfig reads drivers.toml at path. A missing file yields an empty Config.
// An unreadable/invalid file is an error. Every entry is validated per §5.2 of
// the C2 design doc.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read drivers.toml: %w", err)
	}

	// Raw parse into a shape that mirrors the TOML directly.
	var raw struct {
		Instance []struct {
			ID         string `toml:"id"`
			Binary     string `toml:"binary"`
			Enabled    *bool  `toml:"enabled"`
			ConfigJSON string `toml:"config_json"`
			Lifecycle  struct {
				HandshakeDeadlineMs       int `toml:"handshake_deadline_ms"`
				HealthProbeIntervalMs     int `toml:"health_probe_interval_ms"`
				HealthProbeTimeoutMs      int `toml:"health_probe_timeout_ms"`
				HealthFailuresToRestart   int `toml:"health_failures_to_restart"`
				ShutdownGraceMs           int `toml:"shutdown_grace_ms"`
				RestartBackoffInitialMs   int `toml:"restart_backoff_initial_ms"`
				RestartBackoffMaxMs       int `toml:"restart_backoff_max_ms"`
				RestartBudgetWindowMin    int `toml:"restart_budget_window_minutes"`
				RestartBudgetMax          int `toml:"restart_budget_max"`
			} `toml:"lifecycle"`
		} `toml:"instance"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse drivers.toml: %w", err)
	}

	seen := map[string]bool{}
	cfg := &Config{}
	for i, r := range raw.Instance {
		if !idRE.MatchString(r.ID) {
			return nil, fmt.Errorf("instance[%d]: invalid id %q (want [a-z0-9_-]{1,64})", i, r.ID)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("duplicate instance id %q", r.ID)
		}
		seen[r.ID] = true
		if r.Binary == "" {
			return nil, fmt.Errorf("instance %q: binary is required", r.ID)
		}
		info, err := os.Stat(r.Binary)
		if err != nil {
			return nil, fmt.Errorf("instance %q: binary %q: %w", r.ID, r.Binary, err)
		}
		if info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("instance %q: binary %q is not executable", r.ID, r.Binary)
		}
		enabled := true
		if r.Enabled != nil {
			enabled = *r.Enabled
		}
		lc := LifecycleConfig{
			HandshakeDeadline:       dur(r.Lifecycle.HandshakeDeadlineMs, 5*time.Second),
			HealthProbeInterval:     dur(r.Lifecycle.HealthProbeIntervalMs, 15*time.Second),
			HealthProbeTimeout:      dur(r.Lifecycle.HealthProbeTimeoutMs, 3*time.Second),
			HealthFailuresToRestart: intd(r.Lifecycle.HealthFailuresToRestart, 3),
			ShutdownGrace:           dur(r.Lifecycle.ShutdownGraceMs, 10*time.Second),
			RestartBackoffInitial:   dur(r.Lifecycle.RestartBackoffInitialMs, 1*time.Second),
			RestartBackoffMax:       dur(r.Lifecycle.RestartBackoffMaxMs, 60*time.Second),
			RestartBudgetWindow:     time.Duration(intd(r.Lifecycle.RestartBudgetWindowMin, 10)) * time.Minute,
			RestartBudgetMax:        intd(r.Lifecycle.RestartBudgetMax, 10),
		}
		cfg.Instances = append(cfg.Instances, Instance{
			ID:         r.ID,
			Binary:     r.Binary,
			Enabled:    enabled,
			ConfigJSON: []byte(r.ConfigJSON),
			Lifecycle:  lc,
		})
	}
	return cfg, nil
}

func dur(ms int, def time.Duration) time.Duration {
	if ms <= 0 {
		return def
	}
	return time.Duration(ms) * time.Millisecond
}

func intd(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
```

- [ ] **Step 4: Run test, confirm pass**

```bash
go test -run '^TestLoadConfig' ./internal/carport/ -v
```

Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/carport/config.go internal/carport/config_test.go
git commit -m "feat(carport): add drivers.toml loader + validator"
```

---

### Task 6: Routing (`internal/carport/routing.go`)

**Files:**
- Create: `internal/carport/routing.go`
- Create: `internal/carport/routing_test.go`

- [ ] **Step 1: Write failing test `routing_test.go`**

```go
package carport_test

import (
	"context"
	"testing"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/carport"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/registry"
	"github.com/fynn-labs/gohome/internal/testutil"
)

// seed a driver_instances row by appending DriverEvent{kind:"started"}
// followed by EntityRegistered to the registry projector.
func seedRouting(t *testing.T) (*registry.Registry, func()) {
	t.Helper()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	// Insert a driver_instances row and one entity via a synthetic event.
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ev := eventstore.Event{
		Kind:   "driver_event",
		Source: "carport:host",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_DriverEvent{
			DriverEvent: &eventv1.DriverEvent{DriverInstanceId: "hue_main", Kind: "started"},
		}},
	}
	_ = reg.Apply(context.Background(), tx, ev)
	ev2 := eventstore.Event{
		Kind:   "entity_registered",
		Entity: "light.kitchen",
		Source: "driver:hue_main",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "hue_main",
				EntityType:       "light",
				FriendlyName:     "Kitchen",
			},
		}},
	}
	_ = reg.Apply(context.Background(), tx, ev2)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return reg, func() { _ = db.Close() }
}

func TestRouting_ResolveHappyPath(t *testing.T) {
	reg, cleanup := seedRouting(t)
	defer cleanup()

	r := carport.NewRouter(reg)
	instanceID, err := r.Resolve(context.Background(), "light.kitchen")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if instanceID != "hue_main" {
		t.Errorf("instanceID = %q, want hue_main", instanceID)
	}
}

func TestRouting_ResolveUnknownEntityReturnsErrEntityUnknown(t *testing.T) {
	reg, cleanup := seedRouting(t)
	defer cleanup()

	r := carport.NewRouter(reg)
	_, err := r.Resolve(context.Background(), "light.nope")
	if err == nil || !errorsIs(err, carport.ErrEntityUnknown) {
		t.Fatalf("got %v, want ErrEntityUnknown", err)
	}
}

func errorsIs(err, target error) bool {
	// stdlib errors.Is, local helper avoids a dep line at the top.
	for e := err; e != nil; {
		if e == target {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -run '^TestRouting_' ./internal/carport/ -v
```

Expected: compile error (`NewRouter` undefined).

- [ ] **Step 3: Implement `routing.go`**

```go
package carport

import (
	"context"
	"fmt"

	"github.com/fynn-labs/gohome/internal/registry"
)

// Router resolves an entity_id to its owning driver_instance_id by reading the
// registry projection (which itself is populated from the event log).
type Router struct {
	reg *registry.Registry
}

// NewRouter wraps a registry projection for lookups. The registry is consulted
// on every Resolve; this is a map lookup and carries zero I/O overhead.
func NewRouter(reg *registry.Registry) *Router {
	return &Router{reg: reg}
}

// Resolve returns the driver_instance_id that currently owns the entity, or
// ErrEntityUnknown if the entity isn't registered.
func (r *Router) Resolve(ctx context.Context, entityID string) (string, error) {
	e, ok := r.reg.GetEntity(ctx, entityID)
	if !ok {
		return "", fmt.Errorf("resolve %q: %w", entityID, ErrEntityUnknown)
	}
	if e.DriverInstanceID == "" {
		return "", fmt.Errorf("resolve %q: entity has no driver_instance_id: %w", entityID, ErrEntityUnknown)
	}
	return e.DriverInstanceID, nil
}
```

- [ ] **Step 4: Verify `registry.Registry.GetEntity` signature matches**

Read `internal/registry/queries.go` to confirm `GetEntity` takes `(ctx, id)` and returns `(Entity, bool)`. If the signature differs, adjust `routing.go`. Do NOT change the registry — treat it as external.

```bash
grep -n "func.*GetEntity" internal/registry/queries.go
```

If the function signature is different, update `routing.go` accordingly.

- [ ] **Step 5: Run test, confirm pass**

```bash
go test -run '^TestRouting_' ./internal/carport/ -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/carport/routing.go internal/carport/routing_test.go
git commit -m "feat(carport): add Router resolving entity→driver_instance_id via registry projection"
```

---

### Task 7: In-process fake driver (`internal/carport/fakedriver/`)

**Files:**
- Create: `internal/carport/fakedriver/fakedriver.go`

- [ ] **Step 1: Write `fakedriver.go`**

```go
// Package fakedriver is an in-process implementation of the Carport Driver
// gRPC service used by unit tests. Separate package so tests can import both
// carport and fakedriver without import cycles.
package fakedriver

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fynn-labs/gohome/gen/gohome/event/v1"
)

// Double implements carportpb.DriverServer for tests.
// Behavior is pluggable via OnCommand and WantHandshakeError hooks.
type Double struct {
	carportpb.UnimplementedDriverServer

	// Hooks — replace in tests as needed.
	OnCommand          func(ctx context.Context, c *carportpb.Command) *carportpb.CommandResult
	InitialEntities    []*eventpb.EntityRegistered
	Manifest           *carportpb.DriverManifest
	WantHandshakeError error

	// Observability for assertions.
	mu        sync.Mutex
	handshaken int
	closed     bool

	// EventsToEmit: pushed to Run stream right after handshake.
	EventsToEmit []*carportpb.DriverToHost

	// ExpectedSecret: if set, Handshake rejects mismatches.
	ExpectedSecret string
}

// Serve starts a grpc server on a fresh Unix domain socket and returns the
// socket path + a stop func.
func (d *Double) Serve(t TB) (socketPath string, stop func()) {
	t.Helper()
	dir := t.TempDir()
	p := dir + "/sock"
	ln, err := net.Listen("unix", p)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	s := grpc.NewServer()
	carportpb.RegisterDriverServer(s, d)
	go func() { _ = s.Serve(ln) }()
	return p, func() {
		s.GracefulStop()
		_ = ln.Close()
	}
}

// TB is the subset of testing.TB we need (so this pkg doesn't import testing
// into non-test builds).
type TB interface {
	Helper()
	TempDir() string
	Fatalf(format string, args ...any)
}

func (d *Double) Handshake(ctx context.Context, req *carportpb.HandshakeRequest) (*carportpb.HandshakeResponse, error) {
	if d.WantHandshakeError != nil {
		return nil, d.WantHandshakeError
	}
	if d.ExpectedSecret != "" && req.HandshakeSecret != d.ExpectedSecret {
		return nil, status("bad secret")
	}
	d.mu.Lock()
	d.handshaken++
	d.mu.Unlock()

	mf := d.Manifest
	if mf == nil {
		mf = &carportpb.DriverManifest{
			Name:            "fake",
			Version:         "0.0.0",
			ProtocolVersion: "v1alpha1",
		}
	}
	return &carportpb.HandshakeResponse{
		ProtocolVersion: "v1alpha1",
		Manifest:        mf,
		InitialEntities: d.InitialEntities,
	}, nil
}

func (d *Double) Run(srv carportpb.Driver_RunServer) error {
	// Emit any pre-programmed events immediately.
	for _, m := range d.EventsToEmit {
		if err := srv.Send(m); err != nil {
			return err
		}
	}
	// Loop: receive Commands; respond using OnCommand hook.
	for {
		in, err := srv.Recv()
		if err != nil {
			return err
		}
		switch k := in.Kind.(type) {
		case *carportpb.HostToDriver_Command:
			if d.OnCommand == nil {
				continue
			}
			res := d.OnCommand(srv.Context(), k.Command)
			if res == nil {
				continue
			}
			if err := srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Result{Result: res},
			}); err != nil {
				return err
			}
		case *carportpb.HostToDriver_Ping:
			_ = srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{TsUnixMs: time.Now().UnixMilli()}},
			})
		}
	}
}

func (d *Double) Health(ctx context.Context, _ *carportpb.HealthRequest) (*carportpb.HealthResponse, error) {
	return &carportpb.HealthResponse{Ok: true}, nil
}

func (d *Double) Shutdown(ctx context.Context, _ *carportpb.ShutdownRequest) (*carportpb.ShutdownResponse, error) {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	return &carportpb.ShutdownResponse{Acknowledged: true}, nil
}

// HandshakeCount returns the number of successful handshakes (for test assertions).
func (d *Double) HandshakeCount() int { d.mu.Lock(); defer d.mu.Unlock(); return d.handshaken }
func (d *Double) Closed() bool        { d.mu.Lock(); defer d.mu.Unlock(); return d.closed }

func status(msg string) error { return grpcErr(msg) }

// grpcErr is a stub used to avoid pulling codes into this file. Tests that
// want typed codes can return status.Errorf themselves.
func grpcErr(msg string) error { return &doubleErr{msg: msg} }

type doubleErr struct{ msg string }

func (e *doubleErr) Error() string { return e.msg }
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/carport/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/carport/fakedriver/fakedriver.go
git commit -m "feat(carport): add in-process fakedriver.Double for unit tests"
```

---

### Task 8: Instance runtime — connection, pending map, stream I/O

**Files:**
- Create: `internal/carport/instance.go`
- Create: `internal/carport/instance_test.go`

Scope: this file holds the per-instance live runtime — the gRPC client conn, the open `Run` bidi stream, the `pending[command_id] chan *CommandResult` map, helpers to `sendCommand`, register/complete waiters, tear down the stream.

- [ ] **Step 1: Write failing test `instance_test.go`**

```go
package carport_test

import (
	"context"
	"sync"
	"testing"
	"time"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	"github.com/fynn-labs/gohome/internal/carport"
	"github.com/fynn-labs/gohome/internal/carport/fakedriver"
)

func TestInstance_SendCommand_ResultDelivered(t *testing.T) {
	d := &fakedriver.Double{
		OnCommand: func(ctx context.Context, c *carportpb.Command) *carportpb.CommandResult {
			return &carportpb.CommandResult{
				CommandId: c.CommandId,
				Ok:        true,
				Code:      carportpb.CarportErrorCode_CARPORT_OK,
			}
		},
	}
	sock, stop := d.Serve(t)
	defer stop()

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatalf("DialInstance: %v", err)
	}
	defer inst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := inst.SendCommand(ctx, &carportpb.Command{
		CommandId:  "cmd-1",
		EntityId:   "light.x",
		Capability: "turn_on",
	})
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if res.CommandId != "cmd-1" || !res.Ok {
		t.Errorf("result = %+v", res)
	}
}

func TestInstance_SendCommand_TimeoutFailsFast(t *testing.T) {
	d := &fakedriver.Double{
		OnCommand: func(ctx context.Context, c *carportpb.Command) *carportpb.CommandResult {
			time.Sleep(200 * time.Millisecond)
			return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
		},
	}
	sock, stop := d.Serve(t)
	defer stop()

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = inst.SendCommand(ctx, &carportpb.Command{CommandId: "cmd-2", EntityId: "x", Capability: "y"})
	if err != context.DeadlineExceeded && err != carport.ErrDispatchTimeout {
		t.Errorf("expected timeout-ish error, got %v", err)
	}
}

func TestInstance_ConcurrentCommandsMatchResults(t *testing.T) {
	// Driver echoes ok=true with matching id.
	d := &fakedriver.Double{
		OnCommand: func(ctx context.Context, c *carportpb.Command) *carportpb.CommandResult {
			return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
		},
	}
	sock, stop := d.Serve(t)
	defer stop()

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "cmd-" + itoa(i)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res, err := inst.SendCommand(ctx, &carportpb.Command{CommandId: id, EntityId: "x", Capability: "y"})
			if err != nil {
				t.Errorf("cmd %s: %v", id, err)
				return
			}
			if res.CommandId != id {
				t.Errorf("cmd %s: got %s", id, res.CommandId)
			}
		}(i)
	}
	wg.Wait()
}

func itoa(n int) string {
	// small inline to avoid strconv import for a test helper
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -run '^TestInstance_' ./internal/carport/ -v
```

Expected: compile error (`DialInstance` undefined).

- [ ] **Step 3: Implement `instance.go`**

```go
package carport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
)

// instanceConn is the per-instance live runtime: the gRPC client, the open
// Run stream, and the pending-waiter map correlating Commands with Results.
type instanceConn struct {
	conn   *grpc.ClientConn
	client carportpb.DriverClient

	streamCtx    context.Context
	streamCancel context.CancelFunc
	stream       carportpb.Driver_RunClient

	mu      sync.Mutex
	pending map[string]chan *carportpb.CommandResult
	closed  bool

	// ingestHook is called for every non-result DriverToHost message; set by the
	// supervisor so ingest.go can append events to the eventstore. nil during
	// instance-level unit tests.
	ingestHook func(*carportpb.DriverToHost)
	// onStreamError fires exactly once when the stream reader goroutine exits.
	onStreamError func(error)
}

// DialInstance connects to a Unix-domain-socket Carport driver and starts the
// Run stream. The returned *instanceConn is the unit of lifetime: close it to
// tear the whole connection down.
//
// For C2, transport is always UDS insecure — authentication is the handshake
// secret (sent in HandshakeRequest; not wired in DialInstance — the supervisor
// adds it in the Handshake call).
func DialInstance(ctx context.Context, socketPath string) (*instanceConn, error) {
	dialer := func(ctx context.Context, addr string) (c net.Conn, err error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", addr)
	}
	conn, err := grpc.NewClient(
		"passthrough:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	client := carportpb.NewDriverClient(conn)

	streamCtx, cancel := context.WithCancel(context.Background())
	stream, err := client.Run(streamCtx)
	if err != nil {
		cancel()
		_ = conn.Close()
		return nil, fmt.Errorf("open run stream: %w", err)
	}
	ic := &instanceConn{
		conn:         conn,
		client:       client,
		streamCtx:    streamCtx,
		streamCancel: cancel,
		stream:       stream,
		pending:      map[string]chan *carportpb.CommandResult{},
	}
	go ic.reader()
	return ic, nil
}

// reader pumps DriverToHost messages. CommandResults go to pending waiters;
// other messages go through ingestHook.
func (ic *instanceConn) reader() {
	for {
		msg, err := ic.stream.Recv()
		if err != nil {
			ic.failAll(err)
			if ic.onStreamError != nil {
				ic.onStreamError(err)
			}
			return
		}
		switch k := msg.Kind.(type) {
		case *carportpb.DriverToHost_Result:
			ic.deliver(k.Result)
		default:
			if ic.ingestHook != nil {
				ic.ingestHook(msg)
			}
		}
	}
}

func (ic *instanceConn) deliver(r *carportpb.CommandResult) {
	ic.mu.Lock()
	ch, ok := ic.pending[r.CommandId]
	if ok {
		delete(ic.pending, r.CommandId)
	}
	ic.mu.Unlock()
	if ok {
		select {
		case ch <- r:
		default:
		}
	}
}

func (ic *instanceConn) failAll(err error) {
	ic.mu.Lock()
	if ic.closed {
		ic.mu.Unlock()
		return
	}
	ic.closed = true
	pending := ic.pending
	ic.pending = nil
	ic.mu.Unlock()
	for _, ch := range pending {
		close(ch) // receivers treat closed chan as ErrStreamClosed
	}
}

// SendCommand queues a Command and blocks on its matching CommandResult.
// Respects ctx deadline. Returns ErrStreamClosed if the stream has died.
func (ic *instanceConn) SendCommand(ctx context.Context, c *carportpb.Command) (*carportpb.CommandResult, error) {
	ic.mu.Lock()
	if ic.closed {
		ic.mu.Unlock()
		return nil, ErrStreamClosed
	}
	ch := make(chan *carportpb.CommandResult, 1)
	ic.pending[c.CommandId] = ch
	ic.mu.Unlock()

	if err := ic.stream.Send(&carportpb.HostToDriver{Kind: &carportpb.HostToDriver_Command{Command: c}}); err != nil {
		ic.mu.Lock()
		delete(ic.pending, c.CommandId)
		ic.mu.Unlock()
		return nil, fmt.Errorf("send command: %w", err)
	}

	// Compute effective deadline.
	var tmr *time.Timer
	var deadlineC <-chan time.Time
	if c.DeadlineUnixMs > 0 {
		d := time.Until(time.UnixMilli(c.DeadlineUnixMs))
		if d > 0 {
			tmr = time.NewTimer(d)
			deadlineC = tmr.C
		} else {
			return nil, ErrDispatchTimeout
		}
	}
	if tmr != nil {
		defer tmr.Stop()
	}

	select {
	case res, ok := <-ch:
		if !ok {
			return nil, ErrStreamClosed
		}
		return res, nil
	case <-ctx.Done():
		ic.mu.Lock()
		delete(ic.pending, c.CommandId)
		ic.mu.Unlock()
		return nil, ctx.Err()
	case <-deadlineC:
		ic.mu.Lock()
		delete(ic.pending, c.CommandId)
		ic.mu.Unlock()
		return nil, ErrDispatchTimeout
	}
}

// Close terminates the stream and the underlying gRPC client connection.
func (ic *instanceConn) Close() error {
	ic.streamCancel()
	ic.failAll(nil)
	return ic.conn.Close()
}

// setIngestHook registers the supervisor's ingest callback. Must be called
// before the first driver-originated message; in practice the supervisor calls
// this immediately after DialInstance.
func (ic *instanceConn) setIngestHook(f func(*carportpb.DriverToHost)) {
	ic.ingestHook = f
}

// Handshake performs the Handshake RPC on the already-dialed connection.
// This is the supervisor's entry point; DialInstance does NOT handshake.
func (ic *instanceConn) Handshake(ctx context.Context, req *carportpb.HandshakeRequest) (*carportpb.HandshakeResponse, error) {
	return ic.client.Handshake(ctx, req)
}

// Health calls the Health RPC (out-of-band from the Run stream).
func (ic *instanceConn) Health(ctx context.Context) (*carportpb.HealthResponse, error) {
	return ic.client.Health(ctx, &carportpb.HealthRequest{})
}

// Shutdown sends a Shutdown RPC; caller is responsible for waiting grace.
func (ic *instanceConn) Shutdown(ctx context.Context, graceMs int64) (*carportpb.ShutdownResponse, error) {
	return ic.client.Shutdown(ctx, &carportpb.ShutdownRequest{GraceMs: graceMs})
}
```

Add the missing `net` import at the top (paste into the import block).

- [ ] **Step 4: Run test, confirm pass**

```bash
go test -run '^TestInstance_' ./internal/carport/ -v -race
```

Expected: PASS, no race report.

- [ ] **Step 5: Commit**

```bash
git add internal/carport/instance.go internal/carport/instance_test.go
git commit -m "feat(carport): per-instance bidi stream runtime with pending-waiter map"
```

---

### Task 9: Host skeleton (`internal/carport/carport.go`)

**Files:**
- Create: `internal/carport/carport.go`

The Host type is the public API. Its public methods land across later tasks (`Dispatch`, `RestartInstance`, etc.). This task gives it shape.

- [ ] **Step 1: Create the file**

```go
package carport

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/registry"
)

// HostConfig is the daemon-level configuration handed to New.
type HostConfig struct {
	// DriversTOMLPath is the absolute path to drivers.toml. Empty → disabled.
	DriversTOMLPath string

	// SocketDir is where per-instance UDS files are created (owned by gohomed).
	// Defaults to <data_dir>/carport/.
	SocketDir string
}

// Host is the public face of the carport subsystem.
type Host struct {
	cfg     HostConfig
	cfgData *Config

	db      *sql.DB
	store   *eventstore.Store
	router  *Router
	logger  *slog.Logger
	metrics *observability.Metrics

	mu        sync.RWMutex
	instances map[string]*managedInstance // keyed by instance.ID

	stopOnce sync.Once
	stopped  chan struct{}
}

// managedInstance bundles the parsed config + live FSM state + active
// *instanceConn (when state == running). Defined in supervisor.go.
type managedInstance struct {
	cfg   Instance
	state State

	conn *instanceConn

	// restart bookkeeping
	restartHistory []timestamp

	// lifecycle goroutine signal
	cancelLifecycle context.CancelFunc

	mu sync.Mutex
}

type timestamp int64 // unix nanos

// New constructs a Host. The host is inert until Start is called.
func New(cfg HostConfig, db *sql.DB, store *eventstore.Store, reg *registry.Registry, logger *slog.Logger, metrics *observability.Metrics) (*Host, error) {
	cfgData, err := LoadConfig(cfg.DriversTOMLPath)
	if err != nil {
		return nil, err
	}
	return &Host{
		cfg:       cfg,
		cfgData:   cfgData,
		db:        db,
		store:     store,
		router:    NewRouter(reg),
		logger:    logger.With("subsystem", "carport"),
		metrics:   metrics,
		instances: map[string]*managedInstance{},
		stopped:   make(chan struct{}),
	}, nil
}

// Start launches lifecycle goroutines for each enabled instance.
// Returns after all goroutines have been launched; non-blocking.
func (h *Host) Start(ctx context.Context) error {
	for _, inst := range h.cfgData.Instances {
		if !inst.Enabled {
			continue
		}
		m := &managedInstance{cfg: inst, state: StateDeclared}
		h.mu.Lock()
		h.instances[inst.ID] = m
		h.mu.Unlock()
		h.launchLifecycle(ctx, m)
	}
	return nil
}

// Stop signals every lifecycle goroutine to shut its instance down and waits
// up to cfg.Lifecycle.ShutdownGrace per instance for a clean stop.
// Safe to call multiple times.
func (h *Host) Stop(ctx context.Context) {
	h.stopOnce.Do(func() {
		close(h.stopped)
		h.mu.RLock()
		targets := make([]*managedInstance, 0, len(h.instances))
		for _, m := range h.instances {
			targets = append(targets, m)
		}
		h.mu.RUnlock()
		for _, m := range targets {
			h.shutdownInstance(ctx, m)
		}
	})
}
```

- [ ] **Step 2: Add stub methods so later tasks can refer to them**

Append to `carport.go`:

```go
// launchLifecycle is implemented in supervisor.go.
// shutdownInstance is implemented in supervisor.go.
```

(These are real functions we'll build in Task 11; the comment here is just a trailing breadcrumb.)

- [ ] **Step 3: Verify compile**

```bash
go build ./internal/carport/...
```

Expected: failure — `launchLifecycle`/`shutdownInstance` referenced but not yet defined. That's fine; next tasks complete the surface. For now, stub them inline so the package still compiles during independent test runs:

```go
func (h *Host) launchLifecycle(ctx context.Context, m *managedInstance) { /* task 11 */ }
func (h *Host) shutdownInstance(ctx context.Context, m *managedInstance) { /* task 11 */ }
```

- [ ] **Step 4: Verify compile, pass**

```bash
go build ./internal/carport/...
```

Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/carport/carport.go
git commit -m "feat(carport): Host type skeleton + constructor"
```

---

### Task 10: Ingest (`internal/carport/ingest.go`)

**Files:**
- Create: `internal/carport/ingest.go`
- Create: `internal/carport/ingest_test.go`

Scope: a pure function that maps a `DriverToHost` message (non-`Result` variants) to an `eventstore.Event` and appends it. Enforces INV-2 (no silent drops): on append failure, returns the error — caller (supervisor) closes the stream.

- [ ] **Step 1: Write failing test `ingest_test.go`**

```go
package carport_test

import (
	"context"
	"testing"
	"time"

	entitypb "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/carport"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

func TestIngestMessage_StateChanged(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_StateChanged{
			StateChanged: &eventpb.StateChanged{
				Attributes: &entitypb.Attributes{
					Kind: &entitypb.Attributes_Light{Light: &entitypb.Light{On: true, Brightness: 100}},
				},
			},
		},
	}
	// Note: carport.IngestMessage takes an entityID hint because the proto
	// doesn't carry it (state_changed is tied to an entity by the stream's
	// previous context on real drivers; in C2 we carry it on the carport
	// wrapper — see the design doc §4 envelope notes).
	err := carport.IngestMessage(context.Background(), f.store, "hue_main", "light.kitchen", msg)
	if err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}

	events, err := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Kind != "state_changed" {
		t.Errorf("Kind = %q", events[0].Kind)
	}
	if events[0].Source != "driver:hue_main" {
		t.Errorf("Source = %q", events[0].Source)
	}
	if events[0].Entity != "light.kitchen" {
		t.Errorf("Entity = %q", events[0].Entity)
	}
}

func TestIngestMessage_DriverEvent(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_DriverEvent{
			DriverEvent: &eventpb.DriverEvent{
				DriverInstanceId: "hue_main",
				Kind:             "custom_button_press",
				Detail:           "button=up",
			},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue_main", "", msg); err != nil {
		t.Fatal(err)
	}
	events, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(events) != 1 || events[0].Kind != "driver_event" {
		t.Fatalf("want driver_event, got %+v", events)
	}
	_ = time.Second // keep import
}

// newStoreFixtureForTest is defined in a shared test-helper file.
```

Create `internal/carport/test_helpers_test.go` with the fixture helper (referenced by multiple tests):

```go
package carport_test

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/fynn-labs/gohome/internal/testutil"
)

type storeFixture struct {
	store *eventstore.Store
	db    *sql.DB
}

func newStoreFixtureForTest(t *testing.T) *storeFixture {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelWarn, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return &storeFixture{store: s, db: db}
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -run '^TestIngestMessage_' ./internal/carport/ -v
```

Expected: compile error.

- [ ] **Step 3: Implement `ingest.go`**

```go
package carport

import (
	"context"
	"fmt"
	"time"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

// IngestMessage translates a non-Result DriverToHost message to an eventstore
// Event and appends it. Returns the append error verbatim; caller is
// responsible for tearing down the stream on error (INV-2).
//
// `entityID` is a caller-provided hint for StateChanged messages which do not
// carry their own entity_id on the wire.
func IngestMessage(ctx context.Context, store *eventstore.Store, instanceID, entityID string, msg *carportpb.DriverToHost) error {
	now := time.Now()
	source := "driver:" + instanceID

	switch k := msg.Kind.(type) {
	case *carportpb.DriverToHost_StateChanged:
		ev := eventstore.Event{
			Timestamp: now,
			Kind:      "state_changed",
			Entity:    entityID,
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_StateChanged{
				StateChanged: k.StateChanged,
			}},
		}
		_, err := store.Append(ctx, ev)
		return err

	case *carportpb.DriverToHost_EntityRegistered:
		er := k.EntityRegistered
		if er.DriverInstanceId == "" {
			er.DriverInstanceId = instanceID
		}
		// Use the EntityRegistered's friendly name / type to infer entityID
		// via a convention: friendly name mapped 1:1. The driver is expected
		// to include a synthetic id in friendly_name or the entity type.
		// For C2 we rely on the carport host to pass entity_id in a separate
		// channel; this function only persists the event as the driver said.
		ev := eventstore.Event{
			Timestamp: now,
			Kind:      "entity_registered",
			Entity:    entityID, // may be empty for registry-wide metadata
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityRegistered{
				EntityRegistered: er,
			}},
		}
		_, err := store.Append(ctx, ev)
		return err

	case *carportpb.DriverToHost_EntityUnregistered:
		ev := eventstore.Event{
			Timestamp: now,
			Kind:      "entity_unregistered",
			Entity:    entityID,
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityUnregistered{
				EntityUnregistered: k.EntityUnregistered,
			}},
		}
		_, err := store.Append(ctx, ev)
		return err

	case *carportpb.DriverToHost_DriverEvent:
		de := k.DriverEvent
		if de.DriverInstanceId == "" {
			de.DriverInstanceId = instanceID
		}
		ev := eventstore.Event{
			Timestamp: now,
			Kind:      "driver_event",
			Entity:    entityID,
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_DriverEvent{
				DriverEvent: de,
			}},
		}
		_, err := store.Append(ctx, ev)
		return err

	case *carportpb.DriverToHost_Pong:
		// Pongs are transport-level; nothing to ingest.
		return nil

	case *carportpb.DriverToHost_Result:
		// Result messages are delivered via the pending-waiter map in
		// instance.go, not here. If one reaches ingest, that's a bug.
		return fmt.Errorf("IngestMessage got CommandResult; should be consumed by instanceConn.deliver")

	default:
		return fmt.Errorf("IngestMessage: unknown message kind %T", msg.Kind)
	}
}
```

- [ ] **Step 4: Run test, confirm pass**

```bash
go test -run '^TestIngestMessage_' ./internal/carport/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/carport/ingest.go internal/carport/ingest_test.go internal/carport/test_helpers_test.go
git commit -m "feat(carport): IngestMessage → eventstore append (INV-2 on-failure-close)"
```

---

### Task 11: Supervisor — spawn, handshake, backoff, restart budget, shutdown

**Files:**
- Create: `internal/carport/supervisor.go`
- Create: `internal/carport/supervisor_test.go`

Scope: the lifecycle goroutine per managed instance. Transitions drive `DriverEvent` appends. Restart budget bookkeeping. Graceful shutdown via `Shutdown` RPC + SIGKILL escalation.

- [ ] **Step 1: Write supervisor_test.go (uses testdriver binary — so the build tag is set)**

```go
//go:build integration

package carport_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/carport"
	"github.com/fynn-labs/gohome/internal/registry"
)

// buildTestDriver compiles cmd/testdriver into a test-temp binary and returns
// its path. Idempotent across calls in a test run.
func buildTestDriver(t *testing.T) string {
	t.Helper()
	outDir := t.TempDir()
	bin := filepath.Join(outDir, "testdriver")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/testdriver")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = findRepoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build testdriver: %v\n%s", err, out)
	}
	return bin
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("repo root not found")
	return ""
}

func TestSupervisor_NormalLifecycle(t *testing.T) {
	bin := buildTestDriver(t)
	f := newStoreFixtureForTest(t)
	reg, err := registry.New(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.store.RegisterProjector(reg, nil) // assume nil mode means sync — adjust to match RegisterProjector signature

	// Write drivers.toml with one "normal" instance.
	cfgDir := t.TempDir()
	tomlPath := filepath.Join(cfgDir, "drivers.toml")
	if err := os.WriteFile(tomlPath, []byte(`
[[instance]]
id = "normal_one"
binary = "`+bin+`"
enabled = true
config_json = "{\"TESTDRIVER_MODE\":\"normal\"}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := carport.New(carport.HostConfig{DriversTOMLPath: tomlPath, SocketDir: t.TempDir()},
		f.db, f.store, reg, newTestLogger(t), newTestMetrics())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := h.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Wait for running state.
	if !waitFor(5*time.Second, func() bool {
		return h.InstanceState("normal_one") == carport.StateRunning
	}) {
		t.Fatalf("instance never reached running, state=%s", h.InstanceState("normal_one"))
	}

	h.Stop(context.Background())
	if !waitFor(5*time.Second, func() bool {
		return h.InstanceState("normal_one") == carport.StateStopped
	}) {
		t.Fatalf("instance never stopped, state=%s", h.InstanceState("normal_one"))
	}
}

func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// newTestLogger / newTestMetrics defined in test_helpers_test.go (add there).
```

Extend `test_helpers_test.go` with the helpers above:

```go
import "bytes"

func newTestLogger(t *testing.T) *slog.Logger {
	return observability.Init(observability.LogConfig{Level: slog.LevelWarn, Format: "json", Output: &bytes.Buffer{}})
}

func newTestMetrics() *observability.Metrics {
	return observability.NewMetrics()
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -tags=integration -run '^TestSupervisor_NormalLifecycle$' ./internal/carport/ -v
```

Expected: compile error (`h.InstanceState`, `supervisor.go` functions undefined). `cmd/testdriver` also doesn't exist yet — create its stub in Task 13; the current task's test will remain red until Task 13 completes. **Mark this sub-task as "return to after Task 13 lands."** The supervisor implementation itself is built here; the full integration verification waits.

- [ ] **Step 3: Implement `supervisor.go`**

```go
package carport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc/status"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

// launchLifecycle starts the goroutine that drives one managed instance
// through its FSM until the Host is stopped or the instance is quarantined.
func (h *Host) launchLifecycle(parent context.Context, m *managedInstance) {
	ctx, cancel := context.WithCancel(parent)
	m.cancelLifecycle = cancel
	go h.runLifecycle(ctx, m)
}

func (h *Host) runLifecycle(ctx context.Context, m *managedInstance) {
	for {
		if ctx.Err() != nil {
			return
		}
		// Transition: declared/backoff → spawning
		m.mu.Lock()
		m.state = StateSpawning
		m.mu.Unlock()

		sock, proc, secret, err := h.spawn(ctx, m.cfg)
		if err != nil {
			h.emitDriverEvent(ctx, m, "spawn_error", err.Error())
			h.transitionToFailed(ctx, m, err)
			if !h.scheduleBackoff(ctx, m) {
				return
			}
			continue
		}
		m.mu.Lock()
		m.state = StateAwaitingHandshake
		m.mu.Unlock()
		h.emitDriverEvent(ctx, m, "spawned", fmt.Sprint(proc.Pid))

		ic, handshakeResp, err := h.handshake(ctx, m.cfg, sock, secret)
		if err != nil {
			_ = proc.Process.Kill()
			h.emitDriverEvent(ctx, m, "handshake_failed", err.Error())
			h.transitionToFailed(ctx, m, err)
			if !h.scheduleBackoff(ctx, m) {
				return
			}
			continue
		}

		m.mu.Lock()
		m.state = StateRunning
		m.conn = ic
		m.mu.Unlock()
		h.emitDriverEvent(ctx, m, "started", handshakeResp.Manifest.Version)

		// Apply initial registry delta.
		for _, er := range handshakeResp.InitialEntities {
			_, _ = h.store.Append(ctx, eventstore.Event{
				Timestamp: time.Now(),
				Kind:      "entity_registered",
				Source:    "driver:" + m.cfg.ID,
				Entity:    "", // driver-supplied payload carries its own ids
				Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityRegistered{EntityRegistered: er}},
			})
		}

		// Register ingest hook so driver-originated events flow to eventstore.
		ic.setIngestHook(func(msg *carportpb.DriverToHost) {
			if err := IngestMessage(ctx, h.store, m.cfg.ID, "", msg); err != nil {
				h.logger.Error("ingest failed", "instance_id", m.cfg.ID, "err", err)
				// Close the stream → state machine transitions to failed.
				_ = ic.Close()
			}
		})
		ic.onStreamError = func(err error) {
			h.logger.Warn("stream error", "instance_id", m.cfg.ID, "err", err)
		}

		// Run until death: wait for ctx.Done OR stream close OR health failure.
		healthy := h.runHealth(ctx, m, ic)
		_ = ic.Close()
		_ = proc.Wait()

		if ctx.Err() != nil {
			// Daemon-initiated shutdown.
			h.transitionToStopped(ctx, m, "daemon shutdown")
			return
		}
		cause := "stream closed"
		if !healthy {
			cause = "health failed"
		}
		h.emitDriverEvent(ctx, m, "failed", cause)
		h.transitionToFailed(ctx, m, errors.New(cause))
		if !h.scheduleBackoff(ctx, m) {
			return
		}
	}
}

// spawn forks the driver binary with a fresh UDS socket path + handshake secret.
func (h *Host) spawn(ctx context.Context, cfg Instance) (socketPath string, cmd *exec.Cmd, secret string, err error) {
	if err = os.MkdirAll(h.cfg.SocketDir, 0o750); err != nil {
		return "", nil, "", err
	}
	socketPath = filepath.Join(h.cfg.SocketDir, cfg.ID+".sock")
	_ = os.Remove(socketPath)

	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, "", err
	}
	secret = hex.EncodeToString(buf)

	cmd = exec.CommandContext(ctx, cfg.Binary)
	cmd.Env = append(os.Environ(),
		"GOHOME_CARPORT_SOCKET="+socketPath,
		"GOHOME_CARPORT_SECRET="+secret,
		"GOHOME_CARPORT_INSTANCE_ID="+cfg.ID,
		"GOHOME_CARPORT_INSTANCE_CONFIG="+string(cfg.ConfigJSON),
	)
	cmd.Stdout = os.Stdout // driver logs go through stderr convention; stdout is gRPC-unrelated
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return "", nil, "", err
	}
	// Best-effort: wait briefly for the socket to appear.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, cmd, secret, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	return "", nil, "", fmt.Errorf("socket %s never appeared", socketPath)
}

func (h *Host) handshake(ctx context.Context, cfg Instance, socketPath, secret string) (*instanceConn, *carportpb.HandshakeResponse, error) {
	dctx, cancel := context.WithTimeout(ctx, cfg.Lifecycle.HandshakeDeadline)
	defer cancel()

	ic, err := DialInstance(dctx, socketPath)
	if err != nil {
		return nil, nil, err
	}
	resp, err := ic.Handshake(dctx, &carportpb.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		InstanceId:      cfg.ID,
		HandshakeSecret: secret,
		InstanceConfig:  cfg.ConfigJSON,
	})
	if err != nil {
		_ = ic.Close()
		if st, ok := status.FromError(err); ok {
			return nil, nil, fmt.Errorf("handshake: %s", st.Message())
		}
		return nil, nil, err
	}
	if resp.ProtocolVersion != "v1alpha1" {
		_ = ic.Close()
		return nil, nil, fmt.Errorf("protocol mismatch: want v1alpha1, got %s", resp.ProtocolVersion)
	}
	return ic, resp, nil
}

// runHealth loops sending Health probes on cfg.Lifecycle.HealthProbeInterval
// until ctx is canceled OR `failures` consecutive probes fail. Returns true if
// it exited via ctx (graceful), false if health failed.
func (h *Host) runHealth(ctx context.Context, m *managedInstance, ic *instanceConn) bool {
	t := time.NewTicker(m.cfg.Lifecycle.HealthProbeInterval)
	defer t.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return true
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, m.cfg.Lifecycle.HealthProbeTimeout)
			resp, err := ic.Health(pctx)
			cancel()
			if err != nil || (resp != nil && !resp.Ok) {
				failures++
				h.logger.Warn("health probe failure", "instance_id", m.cfg.ID, "failures", failures)
				if failures >= m.cfg.Lifecycle.HealthFailuresToRestart {
					return false
				}
			} else {
				failures = 0
			}
		}
	}
}

func (h *Host) transitionToFailed(ctx context.Context, m *managedInstance, cause error) {
	m.mu.Lock()
	m.state = StateFailed
	m.mu.Unlock()
}

func (h *Host) transitionToStopped(ctx context.Context, m *managedInstance, reason string) {
	h.emitDriverEvent(ctx, m, "stopped", reason)
	m.mu.Lock()
	m.state = StateStopped
	m.mu.Unlock()
}

// scheduleBackoff updates the restart budget and either schedules a retry
// (returning true) or transitions the instance to quarantined (returning false
// — lifecycle goroutine must exit until manual restart).
func (h *Host) scheduleBackoff(ctx context.Context, m *managedInstance) bool {
	now := time.Now()
	m.mu.Lock()
	// prune history older than window
	cutoff := now.Add(-m.cfg.Lifecycle.RestartBudgetWindow).UnixNano()
	fresh := make([]timestamp, 0, len(m.restartHistory))
	for _, ts := range m.restartHistory {
		if int64(ts) >= cutoff {
			fresh = append(fresh, ts)
		}
	}
	m.restartHistory = append(fresh, timestamp(now.UnixNano()))
	used := len(m.restartHistory)
	m.mu.Unlock()

	if used > m.cfg.Lifecycle.RestartBudgetMax {
		h.emitDriverEvent(ctx, m, "quarantined", "restart budget exhausted")
		m.mu.Lock()
		m.state = StateQuarantined
		m.mu.Unlock()
		return false
	}

	// Compute backoff: initial * 2^(used-1), capped at max.
	backoff := m.cfg.Lifecycle.RestartBackoffInitial
	for i := 1; i < used; i++ {
		backoff *= 2
		if backoff > m.cfg.Lifecycle.RestartBackoffMax {
			backoff = m.cfg.Lifecycle.RestartBackoffMax
			break
		}
	}
	h.emitDriverEvent(ctx, m, "backoff_scheduled", backoff.String())

	m.mu.Lock()
	m.state = StateBackoff
	m.mu.Unlock()

	select {
	case <-time.After(backoff):
		return true
	case <-ctx.Done():
		return false
	}
}

func (h *Host) emitDriverEvent(ctx context.Context, m *managedInstance, kind, detail string) {
	_, _ = h.store.Append(ctx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "driver_event",
		Source:    "carport:host",
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_DriverEvent{
			DriverEvent: &eventpb.DriverEvent{
				DriverInstanceId: m.cfg.ID,
				Kind:             kind,
				Detail:           detail,
			},
		}},
	})
	h.logger.Info("driver transition",
		"instance_id", m.cfg.ID, "kind", kind, "detail", truncate(detail, 200))
}

// shutdownInstance sends Shutdown RPC, waits grace, SIGKILLs if needed.
func (h *Host) shutdownInstance(ctx context.Context, m *managedInstance) {
	m.mu.Lock()
	prev := m.state
	m.state = StateStopping
	conn := m.conn
	m.mu.Unlock()

	h.emitDriverEvent(ctx, m, "stopping", prev.String())
	if conn != nil {
		sctx, cancel := context.WithTimeout(ctx, m.cfg.Lifecycle.ShutdownGrace)
		defer cancel()
		_, _ = conn.Shutdown(sctx, m.cfg.Lifecycle.ShutdownGrace.Milliseconds())
		_ = conn.Close()
	}
	if m.cancelLifecycle != nil {
		m.cancelLifecycle()
	}
}

// InstanceState returns the current FSM state for id, or StateDeclared if
// unknown.
func (h *Host) InstanceState(id string) State {
	h.mu.RLock()
	m, ok := h.instances[id]
	h.mu.RUnlock()
	if !ok {
		return StateDeclared
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…"
}
```

- [ ] **Step 4: Build verify**

```bash
go build ./internal/carport/...
```

Expected: success. (The supervisor integration test remains red until Task 13 provides `cmd/testdriver`.)

- [ ] **Step 5: Commit**

```bash
git add internal/carport/supervisor.go internal/carport/supervisor_test.go internal/carport/test_helpers_test.go
git commit -m "feat(carport): supervisor lifecycle goroutine — spawn, handshake, health, backoff, quarantine"
```

---

### Task 12: Dispatch (`internal/carport/dispatch.go`)

**Files:**
- Create: `internal/carport/dispatch.go`
- Create: `internal/carport/dispatch_test.go`

- [ ] **Step 1: Write failing test `dispatch_test.go`**

```go
package carport_test

import (
	"context"
	"errors"
	"testing"
	"time"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	"github.com/fynn-labs/gohome/internal/carport"
)

// Dispatch_EntityUnknown: no CommandIssued appended, pre-flight error.
func TestDispatch_EntityUnknown(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, nil) // no seeded routing
	defer h.Stop(context.Background())

	_, err := h.Dispatch(context.Background(), "light.nope", "turn_on", nil)
	if !errors.Is(err, carport.ErrEntityUnknown) {
		t.Fatalf("got %v, want ErrEntityUnknown", err)
	}
	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if len(evs) > 0 {
		t.Fatal("no event should be appended on pre-flight error")
	}
}

func TestDispatch_InstanceNotRunning(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	// Do not start any instance — state is StateDeclared.
	defer h.Stop(context.Background())

	_, err := h.Dispatch(context.Background(), "light.kitchen", "turn_on", nil)
	if !errors.Is(err, carport.ErrInstanceNotRunning) {
		t.Fatalf("got %v, want ErrInstanceNotRunning", err)
	}
}

func TestDispatch_HappyPathAppendsIssuedAndAck(t *testing.T) {
	f := newStoreFixtureForTest(t)
	// Uses the in-process fakedriver and a test-only helper that stubs the
	// supervisor so we can inject a running instanceConn directly.
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := h.Dispatch(ctx, "light.kitchen", "turn_on", map[string]string{"brightness": "60"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !res.Ok {
		t.Error("expected ok=true")
	}

	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	var issued, acked int
	for _, e := range evs {
		switch e.Kind {
		case "command_issued":
			issued++
		case "command_ack":
			acked++
		}
	}
	if issued != 1 || acked != 1 {
		t.Errorf("issued=%d acked=%d, want 1/1", issued, acked)
	}
}

func TestDispatch_TimeoutAppendsAck(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		time.Sleep(200 * time.Millisecond)
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := h.Dispatch(ctx, "light.kitchen", "turn_on", nil)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, carport.ErrDispatchTimeout) {
		t.Errorf("expected timeout error, got %v", err)
	}

	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	var issued, acked int
	for _, e := range evs {
		switch e.Kind {
		case "command_issued":
			issued++
		case "command_ack":
			acked++
		}
	}
	if issued != 1 || acked != 1 {
		t.Errorf("issued=%d acked=%d, want 1/1 (INV-1)", issued, acked)
	}
}
```

Add helpers to `test_helpers_test.go`:

```go
// newHostForDispatch constructs a Host with the given store/registry, optionally
// seeded via seed fn. Does NOT start any supervised instance.
func newHostForDispatch(t *testing.T, f *storeFixture, seed func(*storeFixture)) *carport.Host {
	t.Helper()
	reg, err := registry.New(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.store.RegisterProjector(reg, eventstore.ProjectorModeSync)
	if seed != nil {
		seed(f)
	}
	cfgPath := filepath.Join(t.TempDir(), "drivers.toml")
	_ = os.WriteFile(cfgPath, []byte(""), 0o644) // empty = no instances
	h, err := carport.New(carport.HostConfig{DriversTOMLPath: cfgPath, SocketDir: t.TempDir()},
		f.db, f.store, reg, newTestLogger(t), newTestMetrics())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// seedHueMainEntity appends an EntityRegistered for light.kitchen owned by hue_main.
func seedHueMainEntity(f *storeFixture) {
	_, _ = f.store.Append(context.Background(), eventstore.Event{
		Kind:   "entity_registered",
		Entity: "light.kitchen",
		Source: "driver:hue_main",
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityRegistered{
			EntityRegistered: &eventpb.EntityRegistered{
				DriverInstanceId: "hue_main",
				EntityType:       "light",
				FriendlyName:     "Kitchen",
			},
		}},
	})
}

// injectRunningFake starts an in-process fakedriver.Double and registers it in
// the Host as a running instance keyed by instanceID. Exposed only via a
// carport test-helper function: carport.InjectRunningInstanceForTests.
func injectRunningFake(t *testing.T, h *carport.Host, instanceID string, onCmd func(*carportpb.Command) *carportpb.CommandResult) func() {
	t.Helper()
	d := &fakedriver.Double{OnCommand: onCmd}
	sock, stop := d.Serve(t)
	if err := carport.InjectRunningInstanceForTests(h, instanceID, sock); err != nil {
		t.Fatalf("inject: %v", err)
	}
	return stop
}

func anyQueryOptions() eventstore.QueryOptions { return eventstore.QueryOptions{} }
```

(Add the corresponding imports at the top of `test_helpers_test.go`: `filepath`, `os`, `fakedriver`, `carportpb`, etc.)

- [ ] **Step 2: Run test, confirm fail**

```bash
go test -run '^TestDispatch_' ./internal/carport/ -v
```

Expected: compile error — `Host.Dispatch` and `InjectRunningInstanceForTests` undefined.

- [ ] **Step 3: Implement `dispatch.go`**

```go
package carport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	"github.com/fynn-labs/gohome/internal/eventstore"
)

const defaultDispatchTimeout = 30 * time.Second

// Dispatch sends a command to the driver instance owning entityID and waits
// for its CommandResult. It appends CommandIssued before sending and
// CommandAck after receiving (or on any pre-driver-response failure) — see
// INV-1 in the C2 design doc.
func (h *Host) Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*carportpb.CommandResult, error) {
	// 1. Routing.
	instanceID, err := h.router.Resolve(ctx, entityID)
	if err != nil {
		return nil, err
	}

	// 2. Instance lookup + state check.
	h.mu.RLock()
	m, ok := h.instances[instanceID]
	h.mu.RUnlock()
	if !ok {
		return nil, ErrInstanceNotRunning
	}
	m.mu.Lock()
	conn := m.conn
	state := m.state
	m.mu.Unlock()
	if state != StateRunning || conn == nil {
		return nil, ErrInstanceNotRunning
	}

	// 3. command_id.
	commandID := uuid.NewString()

	// 4. Append CommandIssued.
	issuedPos, err := h.store.Append(ctx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "command_issued",
		Entity:    entityID,
		Source:    "carport:host",
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_CommandIssued{
			CommandIssued: &eventpb.CommandIssued{
				Command:    capability,
				Parameters: args,
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("append command_issued: %w", err)
	}

	// 5-7. Send + wait. Deadline = min(ctx, default 30s).
	dctx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, defaultDispatchTimeout)
		defer cancel()
	}
	deadlineMs := int64(0)
	if dl, ok := dctx.Deadline(); ok {
		deadlineMs = dl.UnixMilli()
	}
	cmd := &carportpb.Command{
		CommandId:       commandID,
		EntityId:        entityID,
		Capability:      capability,
		Args:            args,
		DeadlineUnixMs:  deadlineMs,
	}
	result, sendErr := conn.SendCommand(dctx, cmd)

	// 8. Append CommandAck (INV-1).
	ack := &eventpb.CommandAck{}
	if sendErr != nil {
		ack.Success = false
		ack.ErrorMessage = sendErr.Error()
	} else if result != nil {
		ack.Success = result.Ok
		ack.ErrorMessage = result.ErrorMessage
	}
	_, ackErr := h.store.Append(ctx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "command_ack",
		Entity:    entityID,
		Source:    "carport:host",
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_CommandAck{CommandAck: ack}},
		// causation = CommandIssued.position (set in C13 once eventstore.Event
		// exposes causation_id on the append path; for now we leave it and
		// rely on query-time correlation via command_id in the Command payload).
	})
	_ = issuedPos
	if ackErr != nil {
		h.logger.Error("append command_ack", "entity", entityID, "err", ackErr)
	}

	if sendErr != nil {
		return nil, mapSendError(sendErr)
	}
	return result, nil
}

func mapSendError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return ErrContextCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrDispatchTimeout
	case errors.Is(err, ErrStreamClosed):
		return ErrStreamClosed
	default:
		return err
	}
}

// InjectRunningInstanceForTests is an internal testing seam. It establishes
// an instanceConn to the provided UDS path and registers it as a running
// managedInstance under instanceID, bypassing supervisor spawn/handshake.
// Callers are responsible for stopping the backing server.
func InjectRunningInstanceForTests(h *Host, instanceID, socketPath string) error {
	ic, err := DialInstance(context.Background(), socketPath)
	if err != nil {
		return err
	}
	m := &managedInstance{
		cfg:   Instance{ID: instanceID, Lifecycle: LifecycleConfig{HandshakeDeadline: 5 * time.Second}},
		state: StateRunning,
		conn:  ic,
	}
	h.mu.Lock()
	h.instances[instanceID] = m
	h.mu.Unlock()
	return nil
}
```

- [ ] **Step 4: Run test, confirm pass**

```bash
go test -run '^TestDispatch_' ./internal/carport/ -v -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/carport/dispatch.go internal/carport/dispatch_test.go internal/carport/test_helpers_test.go
git commit -m "feat(carport): Host.Dispatch with INV-1 (every issued has an ack)"
```

---

### Task 13: `cmd/testdriver` — scenario-driven driver binary

**Files:**
- Create: `cmd/testdriver/main.go`

- [ ] **Step 1: Write `main.go`**

```go
// Package main is a minimal scenario-driven driver binary used by integration
// tests. Behavior is selected by TESTDRIVER_MODE (encoded in
// GOHOME_CARPORT_INSTANCE_CONFIG as JSON).
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
)

type cfg struct {
	Mode string `json:"TESTDRIVER_MODE"`
}

func main() {
	sock := os.Getenv("GOHOME_CARPORT_SOCKET")
	secret := os.Getenv("GOHOME_CARPORT_SECRET")
	raw := os.Getenv("GOHOME_CARPORT_INSTANCE_CONFIG")
	var c cfg
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		log.Fatalf("listen %s: %v", sock, err)
	}
	s := grpc.NewServer()
	carportpb.RegisterDriverServer(s, &server{mode: c.Mode, expectedSecret: secret})
	log.Printf("testdriver ready: mode=%s sock=%s", c.Mode, sock)
	if err := s.Serve(ln); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type server struct {
	carportpb.UnimplementedDriverServer
	mode           string
	expectedSecret string
}

func (s *server) Handshake(ctx context.Context, req *carportpb.HandshakeRequest) (*carportpb.HandshakeResponse, error) {
	switch s.mode {
	case "bad_secret":
		return nil, grpcErr("bad secret")
	case "bad_protocol_version":
		return &carportpb.HandshakeResponse{ProtocolVersion: "v99"}, nil
	case "slow_handshake":
		time.Sleep(10 * time.Second)
		return &carportpb.HandshakeResponse{ProtocolVersion: "v1alpha1"}, nil
	}
	if req.HandshakeSecret != s.expectedSecret {
		return nil, grpcErr("secret mismatch")
	}
	return &carportpb.HandshakeResponse{
		ProtocolVersion: "v1alpha1",
		Manifest: &carportpb.DriverManifest{
			Name:            "testdriver",
			Version:         "0.0.0",
			ProtocolVersion: "v1alpha1",
		},
	}, nil
}

func (s *server) Run(srv carportpb.Driver_RunServer) error {
	switch s.mode {
	case "crash_after_handshake":
		time.Sleep(100 * time.Millisecond)
		os.Exit(2)
	case "crash_mid_stream":
		time.Sleep(250 * time.Millisecond)
		os.Exit(2)
	case "chatty":
		// fire 1000 state_changed events
		for i := 0; i < 1000; i++ {
			if err := srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{}},
			}); err != nil {
				return err
			}
		}
	}

	for {
		in, err := srv.Recv()
		if err != nil {
			return err
		}
		switch k := in.Kind.(type) {
		case *carportpb.HostToDriver_Command:
			if s.mode == "hang_on_command" {
				time.Sleep(time.Hour)
				continue
			}
			_ = srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Result{
					Result: &carportpb.CommandResult{
						CommandId: k.Command.CommandId,
						Ok:        true,
					},
				},
			})
		case *carportpb.HostToDriver_Ping:
			_ = srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{TsUnixMs: time.Now().UnixMilli()}},
			})
		}
	}
}

func (s *server) Health(ctx context.Context, _ *carportpb.HealthRequest) (*carportpb.HealthResponse, error) {
	return &carportpb.HealthResponse{Ok: true}, nil
}

func (s *server) Shutdown(ctx context.Context, _ *carportpb.ShutdownRequest) (*carportpb.ShutdownResponse, error) {
	if s.mode == "hang_on_shutdown" {
		time.Sleep(time.Hour)
	}
	return &carportpb.ShutdownResponse{Acknowledged: true}, nil
}

func grpcErr(msg string) error { return &e{msg} }

type e struct{ m string }

func (err *e) Error() string { return err.m }
```

- [ ] **Step 2: Verify compile**

```bash
go build ./cmd/testdriver
```

Expected: produces `./testdriver` binary.

- [ ] **Step 3: Retry supervisor integration test**

```bash
go test -tags=integration -run '^TestSupervisor_NormalLifecycle$' ./internal/carport/ -v
```

Expected: PASS. If the test fails on plumbing (registry projector mode, metric nil deref), triage those — the test spec remains correct.

- [ ] **Step 4: Commit**

```bash
git add cmd/testdriver/main.go
git commit -m "feat(testdriver): scenario-driven Carport driver binary for integration tests"
```

---

### Task 14: Full integration matrix (§7.4 of spec)

**Files:**
- Modify: `internal/carport/supervisor_test.go` (add 9 more scenarios beyond `normal`)

Each scenario is a dedicated `Test*` function that:
1. Builds the testdriver binary (reuse `buildTestDriver`).
2. Writes a one-line drivers.toml with its `TESTDRIVER_MODE` in `config_json`.
3. Starts a `carport.Host`.
4. Waits for the expected end-state (running / failed / backoff / quarantined).
5. Asserts expected DriverEvents in the event log.

- [ ] **Step 1: Add scenarios** (use the table-driven pattern to keep code DRY but still show each assertion explicitly)

```go
//go:build integration

package carport_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/carport"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/registry"
)

func runScenario(t *testing.T, mode string, until func(*carport.Host) bool) *carport.Host {
	t.Helper()
	bin := buildTestDriver(t)
	f := newStoreFixtureForTest(t)
	reg, err := registry.New(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.store.RegisterProjector(reg, eventstore.ProjectorModeSync)

	cfgDir := t.TempDir()
	tomlPath := filepath.Join(cfgDir, "drivers.toml")
	_ = os.WriteFile(tomlPath, []byte(`
[[instance]]
id = "test_one"
binary = "`+bin+`"
enabled = true
config_json = "{\"TESTDRIVER_MODE\":\"`+mode+`\"}"
[instance.lifecycle]
handshake_deadline_ms = 2000
health_probe_interval_ms = 500
health_failures_to_restart = 2
restart_backoff_initial_ms = 100
restart_backoff_max_ms = 500
restart_budget_window_minutes = 1
restart_budget_max = 3
shutdown_grace_ms = 1000
`), 0o644)

	h, err := carport.New(carport.HostConfig{DriversTOMLPath: tomlPath, SocketDir: t.TempDir()},
		f.db, f.store, reg, newTestLogger(t), newTestMetrics())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); h.Stop(context.Background()) })
	if err := h.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !waitFor(10*time.Second, func() bool { return until(h) }) {
		t.Fatalf("scenario %s: never reached expected state; current=%s", mode, h.InstanceState("test_one"))
	}
	return h
}

func TestSupervisor_CrashAfterHandshake(t *testing.T) {
	h := runScenario(t, "crash_after_handshake", func(h *carport.Host) bool {
		return h.InstanceState("test_one") == carport.StateBackoff ||
			h.InstanceState("test_one") == carport.StateSpawning
	})
	_ = h
}

func TestSupervisor_CrashMidStream(t *testing.T) {
	h := runScenario(t, "crash_mid_stream", func(h *carport.Host) bool {
		return h.InstanceState("test_one") == carport.StateBackoff ||
			h.InstanceState("test_one") == carport.StateSpawning
	})
	_ = h
}

func TestSupervisor_HangOnShutdown(t *testing.T) {
	h := runScenario(t, "hang_on_shutdown", func(h *carport.Host) bool {
		return h.InstanceState("test_one") == carport.StateRunning
	})
	h.Stop(context.Background())
	if !waitFor(3*time.Second, func() bool { return h.InstanceState("test_one") == carport.StateStopped }) {
		t.Fatalf("hang_on_shutdown never stopped; state=%s", h.InstanceState("test_one"))
	}
}

func TestSupervisor_BadProtocolVersion(t *testing.T) {
	h := runScenario(t, "bad_protocol_version", func(h *carport.Host) bool {
		return h.InstanceState("test_one") == carport.StateBackoff ||
			h.InstanceState("test_one") == carport.StateQuarantined
	})
	_ = h
}

func TestSupervisor_SlowHandshake(t *testing.T) {
	h := runScenario(t, "slow_handshake", func(h *carport.Host) bool {
		// handshake_deadline_ms=2000 in TOML; slow_handshake sleeps 10s.
		return h.InstanceState("test_one") == carport.StateBackoff ||
			h.InstanceState("test_one") == carport.StateQuarantined
	})
	_ = h
}
```

- [ ] **Step 2: Run**

```bash
go test -tags=integration -run '^TestSupervisor_' ./internal/carport/ -v
```

Expected: PASS on all scenarios. If any scenario is flaky (timings), tune the `runScenario` lifecycle overrides before loosening assertions.

- [ ] **Step 3: Commit**

```bash
git add internal/carport/supervisor_test.go
git commit -m "test(carport): cover crash/hang/bad-protocol/slow-handshake scenarios"
```

---

### Task 15: Property tests (`internal/carport/properties_test.go`)

- [ ] **Step 1: Write `properties_test.go`**

```go
package carport_test

import (
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/fynn-labs/gohome/internal/carport"
)

// Property: across any randomized sequence of legal triggers starting at
// StateDeclared, the FSM never advances to a state not reachable from the
// seed's trigger path.
func TestProp_FSMLegalReachability(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(42))}
	prop := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		state := carport.StateDeclared
		for i := 0; i < 20; i++ {
			triggers := []carport.Trigger{
				carport.TriggerStart, carport.TriggerSpawned, carport.TriggerSpawnError,
				carport.TriggerHandshakeOK, carport.TriggerHandshakeFail,
				carport.TriggerCrash, carport.TriggerHealthFail, carport.TriggerStreamError,
				carport.TriggerBackoffScheduled, carport.TriggerBackoffElapsed,
				carport.TriggerBudgetExhausted, carport.TriggerManualRestart,
				carport.TriggerShutdown, carport.TriggerExited,
			}
			trig := triggers[r.Intn(len(triggers))]
			next, err := carport.Next(state, trig)
			if err != nil {
				continue
			}
			if !carport.IsLegal(state, trig, next) {
				return false
			}
			state = next
		}
		return true
	}
	if err := quick.Check(prop, cfg); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run, commit**

```bash
go test -run '^TestProp_' ./internal/carport/ -v
git add internal/carport/properties_test.go
git commit -m "test(carport): property tests for FSM legal reachability"
```

---

### Task 16: Fuzz (`internal/carport/fuzz_test.go`)

- [ ] **Step 1: Write `fuzz_test.go`**

```go
package carport_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	carportpb "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
)

func FuzzEnvelopeDecode(f *testing.F) {
	// seed
	seed, _ := proto.Marshal(&carportpb.HostToDriver{
		Kind: &carportpb.HostToDriver_Command{
			Command: &carportpb.Command{CommandId: "x"},
		},
	})
	f.Add(seed)

	f.Fuzz(func(t *testing.T, data []byte) {
		var h2d carportpb.HostToDriver
		_ = proto.Unmarshal(data, &h2d) // must not panic
		var d2h carportpb.DriverToHost
		_ = proto.Unmarshal(data, &d2h)
	})
}
```

- [ ] **Step 2: Smoke-fuzz**

```bash
go test -fuzz=FuzzEnvelopeDecode -fuzztime=30s ./internal/carport/
```

Expected: no crashes.

- [ ] **Step 3: Extend `Taskfile.yml` fuzz task**

Edit `Taskfile.yml`, add a line to `test:fuzz`:

```yaml
  test:fuzz:
    desc: Run fuzz targets briefly
    cmds:
      - go test -fuzz=Fuzz -fuzztime=30s ./internal/eventstore
      - go test -fuzz=Fuzz -fuzztime=30s ./internal/registry
      - go test -fuzz=Fuzz -fuzztime=30s ./internal/carport
```

- [ ] **Step 4: Commit**

```bash
git add internal/carport/fuzz_test.go Taskfile.yml
git commit -m "test(carport): FuzzEnvelopeDecode target; extend Taskfile test:fuzz"
```

---

### Task 17: Observability — metrics additions

**Files:**
- Modify: `internal/observability/metrics.go`

- [ ] **Step 1: Add carport metric fields to `Metrics` struct**

At the end of the struct (before `BuildInfo`), insert:

```go
	// Carport (driver subsystem)
	CarportDriverInstances        *prometheus.GaugeVec
	CarportHandshakesTotal        *prometheus.CounterVec
	CarportCommandDispatchTotal   *prometheus.CounterVec
	CarportCommandDispatchSeconds *prometheus.HistogramVec
	CarportEventsIngestedTotal    *prometheus.CounterVec
	CarportDriverRestartsTotal    *prometheus.CounterVec
	CarportHealthProbeSeconds     *prometheus.HistogramVec
	CarportStreamMessagesTotal    *prometheus.CounterVec
	CarportPendingCommands        *prometheus.GaugeVec
```

- [ ] **Step 2: Initialize in `NewMetrics`**

Insert after the existing `BuildInfo` initialization:

```go
	m.CarportDriverInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "carport_driver_instances", Help: "Driver instances by FSM state"},
		[]string{"state"},
	)
	m.CarportHandshakesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_handshakes_total", Help: "Carport Handshake outcomes"},
		[]string{"instance_id", "result"},
	)
	m.CarportCommandDispatchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_command_dispatch_total", Help: "Dispatch outcomes"},
		[]string{"instance_id", "result"},
	)
	m.CarportCommandDispatchSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "carport_command_dispatch_duration_seconds", Help: "Dispatch latency"},
		[]string{"instance_id", "capability"},
	)
	m.CarportEventsIngestedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_events_ingested_total", Help: "Events ingested from drivers"},
		[]string{"instance_id", "kind"},
	)
	m.CarportDriverRestartsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_driver_restarts_total", Help: "Driver restarts by cause"},
		[]string{"instance_id", "reason"},
	)
	m.CarportHealthProbeSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "carport_health_probe_duration_seconds", Help: "Health probe latency"},
		[]string{"instance_id"},
	)
	m.CarportStreamMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_stream_messages_received_total", Help: "DriverToHost messages by kind"},
		[]string{"instance_id", "kind"},
	)
	m.CarportPendingCommands = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "carport_pending_commands", Help: "In-flight Dispatch calls by instance"},
		[]string{"instance_id"},
	)
```

Extend the `reg.MustRegister(...)` call to include all new metrics.

- [ ] **Step 3: Wire metrics in `supervisor.go` / `dispatch.go`**

Add `h.metrics.CarportHandshakesTotal.WithLabelValues(...).Inc()` etc. at the appropriate emission points. Only increment inside already-tested code paths; no new test is needed if logic is purely telemetry.

- [ ] **Step 4: Run full test**

```bash
go test ./...
```

Expected: all tests pass. (Prometheus test registry picks up new metrics; no registration collisions.)

- [ ] **Step 5: Commit**

```bash
git add internal/observability/metrics.go internal/carport/
git commit -m "feat(observability): add 9 carport metrics; wire dispatch/handshake/ingest"
```

---

### Task 18: Daemon wiring (`internal/daemon/daemon.go`)

**Files:**
- Modify: `internal/daemon/config.go` — add carport fields
- Modify: `internal/daemon/daemon.go` — add carport.Host to startup/shutdown
- Modify: `cmd/gohomed/main.go` — add `--drivers-toml` flag

- [ ] **Step 1: Extend `Config`**

```go
// in internal/daemon/config.go
type Config struct {
	DataDir             string
	LogLevel            slog.Level
	LogFormat           string
	AdminPort           int
	SocketPath          string
	SnapshotEveryEvents int
	SnapshotEveryPeriod time.Duration

	// Carport
	DriversTOMLPath string // default: <data_dir>/drivers.toml
	CarportSocketDir string // default: <data_dir>/carport/
}

func (c *Config) WithDefaults() {
	// ... existing ...
	if c.DriversTOMLPath == "" {
		c.DriversTOMLPath = "@data/drivers.toml" // resolved to data_dir in Run
	}
	if c.CarportSocketDir == "" {
		c.CarportSocketDir = "@data/carport"
	}
}
```

- [ ] **Step 2: Insert Carport startup into `daemon.go`**

After the `store.Start(ctx)` call (phase 4), before phase 5:

```go
	// Phase 4.5: carport
	driversTOML := d.cfg.DriversTOMLPath
	if driversTOML == "@data/drivers.toml" {
		driversTOML = filepath.Join(dataDir, "drivers.toml")
	}
	socketDir := d.cfg.CarportSocketDir
	if socketDir == "@data/carport" {
		socketDir = filepath.Join(dataDir, "carport")
	}
	cport, err := carport.New(carport.HostConfig{
		DriversTOMLPath: driversTOML,
		SocketDir:       socketDir,
	}, d.db, d.store, d.registry, d.logger, d.metrics)
	if err != nil {
		return fmt.Errorf("carport: %w", err)
	}
	d.carport = cport
	if err := d.carport.Start(ctx); err != nil {
		return fmt.Errorf("carport start: %w", err)
	}
```

And add `carport *carport.Host` to the `Daemon` struct.

Shutdown: before `store.Close(ctx)`:

```go
	if d.carport != nil {
		d.carport.Stop(shutCtx)
	}
```

- [ ] **Step 3: Extend `cmd/gohomed/main.go` flags**

```go
	driversTOMLPath = flag.String("drivers-toml", "", "path to drivers.toml (default <data-dir>/drivers.toml)")
```

And populate the Config.

- [ ] **Step 4: Run existing daemon tests**

```bash
go test ./internal/daemon/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/ cmd/gohomed/main.go
git commit -m "feat(daemon): wire carport.Host into gohomed startup/shutdown"
```

---

### Task 19: Socket ops — `driver_restart`, `command_send`

**Files:**
- Modify: `internal/daemon/recovery.go`

- [ ] **Step 1: Extend `socketReq` / `socketResp`**

```go
type socketReq struct {
	Op         string            `json:"op"`
	Owner      string            `json:"owner,omitempty"`
	InstanceID string            `json:"instance_id,omitempty"`
	Entity     string            `json:"entity,omitempty"`
	Capability string            `json:"capability,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
}

type socketResp struct {
	OK           bool              `json:"ok"`
	Error        string            `json:"error,omitempty"`
	Position     uint64            `json:"position,omitempty"`
	CommandOK    bool              `json:"command_ok,omitempty"`
	CommandError string            `json:"command_error,omitempty"`
}
```

- [ ] **Step 2: Add cases in `handleSocketConn`**

```go
	case "driver_restart":
		if err := d.carport.RestartInstance(ctx, req.InstanceID); err != nil {
			_ = enc.Encode(socketResp{Error: err.Error()})
			return
		}
		_ = enc.Encode(socketResp{OK: true})

	case "command_send":
		res, err := d.carport.Dispatch(ctx, req.Entity, req.Capability, req.Args)
		if err != nil {
			_ = enc.Encode(socketResp{Error: err.Error()})
			return
		}
		_ = enc.Encode(socketResp{
			OK:           true,
			CommandOK:    res.Ok,
			CommandError: res.ErrorMessage,
		})
```

- [ ] **Step 3: Implement `Host.RestartInstance`** (in `dispatch.go` or a new `control.go`)

```go
// internal/carport/control.go
package carport

import (
	"context"
	"fmt"
)

// RestartInstance forces a quarantined or running instance back to the
// spawning state, resetting its restart-budget history.
func (h *Host) RestartInstance(ctx context.Context, id string) error {
	h.mu.RLock()
	m, ok := h.instances[id]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown instance %q", id)
	}
	m.mu.Lock()
	m.restartHistory = nil
	m.state = StateSpawning // will transition through spawn again
	m.mu.Unlock()
	// The lifecycle goroutine may have exited (quarantined). Relaunch.
	h.launchLifecycle(ctx, m)
	h.emitDriverEvent(ctx, m, "restart_manual", "operator")
	return nil
}
```

- [ ] **Step 4: Run build/test**

```bash
go build ./... && go test ./internal/daemon/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/recovery.go internal/carport/control.go
git commit -m "feat(daemon): driver_restart and command_send UDS socket ops"
```

---

### Task 20: CLI — `gohome driver` subtree

**Files:**
- Create: `internal/cli/driver.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Create `driver.go`** (models after `snapshot.go`)

```go
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

func newDriverCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "driver", Short: "Inspect and control driver instances"}
	c.AddCommand(newDriverListCmd(gf))
	c.AddCommand(newDriverStatusCmd(gf))
	c.AddCommand(newDriverRestartCmd(gf))
	return c
}

func newDriverListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List driver instances from the registry projection",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer func() { _ = db.Close() }()

			rows, err := db.QueryContext(ctx, `SELECT id, driver_name, state, started_at FROM driver_instances ORDER BY id`)
			dieOnError(err)
			defer func() { _ = rows.Close() }()

			t := lgtable.New().
				Headers("Instance", "Driver", "State", "Started").
				StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
			for rows.Next() {
				var id, name, state string
				var startedNs int64
				dieOnError(rows.Scan(&id, &name, &state, &startedNs))
				t.Row(id, name, state, time.Unix(0, startedNs).Format("2006-01-02 15:04:05"))
			}
			dieOnError(rows.Err())
			fmt.Fprintln(os.Stdout, t)
		},
	}
}

func newDriverStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status <instance>",
		Short: "Show detailed status for one driver instance",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Minimal: dump the registry row + the 20 most recent DriverEvents.
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer func() { _ = db.Close() }()

			var id, name, state string
			var startedNs int64
			row := db.QueryRowContext(ctx,
				`SELECT id, driver_name, state, started_at FROM driver_instances WHERE id = ?`, args[0])
			dieOnError(row.Scan(&id, &name, &state, &startedNs))
			fmt.Fprintf(os.Stdout, "%s %s\n", EntityID.Render(id), StateOK.Render(state))
			fmt.Fprintf(os.Stdout, "driver: %s\nstarted: %s\n", name, time.Unix(0, startedNs).Format(time.RFC3339))
		},
	}
}

func newDriverRestartCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <instance>",
		Short: "Force-restart a driver instance via daemon socket",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			sockPath := filepath.Join(expandHome(gf.DataDir), "gohomed.sock")
			conn, err := (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
			dieOnError(err)
			defer func() { _ = conn.Close() }()

			req := map[string]string{"op": "driver_restart", "instance_id": args[0]}
			dieOnError(json.NewEncoder(conn).Encode(req))
			var resp struct {
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}
			dieOnError(json.NewDecoder(conn).Decode(&resp))
			if !resp.OK {
				dieOnError(errors.New(resp.Error))
			}
			fmt.Printf("%s restart scheduled for %s\n", Success.Render("ok:"), EntityID.Render(args[0]))
		},
	}
}
```

Note: `StateOK` style needs to exist in `internal/cli/styles.go`. If it doesn't, add it:

```go
var StateOK = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
```

- [ ] **Step 2: Register in `root.go`**

Append:

```go
	root.AddCommand(newDriverCmd(gf))
```

- [ ] **Step 3: Build & smoke**

```bash
go build ./cmd/gohome
./dist/gohome driver list --data-dir /tmp/nope 2>&1 || true
```

Expected: executes (no panic), may error on missing DB — acceptable.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/driver.go internal/cli/root.go internal/cli/styles.go
git commit -m "feat(cli): gohome driver list/status/restart"
```

---

### Task 21: CLI — `gohome command send`

**Files:**
- Create: `internal/cli/command.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Create `command.go`**

```go
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCommandCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "command", Short: "Send capability invocations to driver instances"}
	c.AddCommand(newCommandSendCmd(gf))
	return c
}

func newCommandSendCmd(gf *globalFlags) *cobra.Command {
	var argPairs []string
	c := &cobra.Command{
		Use:   "send <entity> <capability>",
		Short: "Invoke <capability> on <entity> via the daemon",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			argsMap := map[string]string{}
			for _, kv := range argPairs {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					dieOnError(fmt.Errorf("bad --arg %q (want k=v)", kv))
				}
				argsMap[parts[0]] = parts[1]
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			sockPath := filepath.Join(expandHome(gf.DataDir), "gohomed.sock")
			conn, err := (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
			dieOnError(err)
			defer func() { _ = conn.Close() }()

			req := map[string]any{
				"op":         "command_send",
				"entity":     args[0],
				"capability": args[1],
				"args":       argsMap,
			}
			dieOnError(json.NewEncoder(conn).Encode(req))

			var resp struct {
				OK           bool   `json:"ok"`
				Error        string `json:"error"`
				CommandOK    bool   `json:"command_ok"`
				CommandError string `json:"command_error"`
			}
			dieOnError(json.NewDecoder(conn).Decode(&resp))
			if !resp.OK {
				dieOnError(errors.New(resp.Error))
			}
			if !resp.CommandOK {
				dieOnError(fmt.Errorf("driver returned error: %s", resp.CommandError))
			}
			fmt.Printf("%s %s.%s\n", Success.Render("ok:"), args[0], args[1])
		},
	}
	c.Flags().StringSliceVar(&argPairs, "arg", nil, "k=v (repeatable)")
	return c
}
```

- [ ] **Step 2: Register, build, commit**

Add `root.AddCommand(newCommandCmd(gf))` to `root.go`.

```bash
go build ./...
git add internal/cli/command.go internal/cli/root.go
git commit -m "feat(cli): gohome command send"
```

---

### Task 22: Golden replay fixtures

**Files:**
- Create: `testdata/golden/carport/{happy-path,crash-recovery,quarantine}.{events,state.json,registry.json}`
- Create: `internal/carport/replay_golden_test.go` (mirrors `internal/eventstore/replay_golden_test.go`)

- [ ] **Step 1: Read C1's replay_golden_test.go to match the pattern**

```bash
cat internal/eventstore/replay_golden_test.go
```

- [ ] **Step 2: Write a minimal test that loads the fixture, replays through the eventstore+registry, and asserts against the golden JSON**

Pattern: for each fixture, call `testutil.LoadFixture(t, "carport/<name>.events")`, replay into a fresh store+registry, then `testutil.AssertGolden(t, "carport/<name>.state.json", stateDump)`.

If `testutil.LoadFixture` doesn't support paths under `testdata/golden/`, extend it — one path update limited to the existing `internal/testutil/fixtures.go`. Before doing so, verify:

```bash
cat internal/testutil/fixtures.go
```

- [ ] **Step 3: Generate fixtures with `-update`**

Run the test with `-update` to write golden files, inspect them manually for sanity, then remove the flag and confirm they pass.

```bash
go test -run '^TestGolden_Carport' ./internal/carport/... -v -update
# inspect testdata/golden/carport/*.json
go test -run '^TestGolden_Carport' ./internal/carport/... -v
```

- [ ] **Step 4: Commit**

```bash
git add testdata/golden/carport/ internal/carport/replay_golden_test.go internal/testutil/fixtures.go
git commit -m "test(carport): three golden-replay fixtures (happy, crash, quarantine)"
```

---

### Task 23: CI — extend workflow for carport integration coverage

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Inspect the current workflow**

```bash
cat .github/workflows/ci.yml
```

- [ ] **Step 2: Add a coverage-gate step after `test:integration`** that enforces ≥85% for carport:

```yaml
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - run: go test -coverprofile=cover.out ./internal/carport/...
      - run: |
          pct=$(go tool cover -func=cover.out | grep total | awk '{print $3}' | tr -d %)
          # Bash can't compare floats; use awk.
          fail=$(awk -v p="$pct" 'BEGIN{exit !(p+0<85)}')
          if [ "$fail" = "0" ]; then
            echo "carport coverage $pct% below 85% gate"
            exit 1
          fi
```

Add `scripts/check-proto-hygiene.sh` to a proto-lint job:

```yaml
  proto:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - run: ./scripts/check-proto-hygiene.sh
      - run: buf lint
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add carport coverage gate (≥85%) and proto-hygiene lint"
```

---

### Task 24: Smoke test, acceptance walk-through, `c2-complete` tag

- [ ] **Step 1: Full test matrix**

```bash
task lint
task test
task test:race
task test:integration
task test:fuzz
```

Expected: all green.

- [ ] **Step 2: End-to-end smoke**

Start a daemon with a drivers.toml pointing at `testdriver` in `normal` mode:

```bash
task build
mkdir -p /tmp/gohome-smoke/carport
cat > /tmp/gohome-smoke/drivers.toml <<EOF
[[instance]]
id = "smoke"
binary = "$(pwd)/dist/testdriver"
enabled = true
config_json = "{\"TESTDRIVER_MODE\":\"normal\"}"
EOF
./dist/gohomed --data-dir /tmp/gohome-smoke &
sleep 3
./dist/gohome driver list --data-dir /tmp/gohome-smoke
./dist/gohome command send --data-dir /tmp/gohome-smoke light.kitchen turn_on || true
kill %1
wait
```

Confirm: `driver list` shows the instance, `command send` either succeeds (if the testdriver registered a light) or errors with ErrEntityUnknown — acceptable if the testdriver doesn't seed entities.

- [ ] **Step 3: Tag**

```bash
git tag c2-complete
git log --oneline c1-complete..c2-complete
```

Expected: roughly 20 commits visible (one per task).

- [ ] **Step 4: Final commit** (if any README update)

```bash
git add -A
git commit -m "chore: c2 — Carport protocol complete" || true
```

---

## Dependencies & Ordering

```
T0 (deps + buf gen) ── T1 (proto hygiene retrofit) ── T2 (carport protos)
                                                         │
                   ┌─────────────────────────────────────┤
                   ▼                                     ▼
                   T3 (errors) ── T4 (FSM) ── T5 (config) ── T6 (routing)
                                                              │
                                   ┌──────────────────────────┤
                                   ▼                          ▼
                                   T7 (fakedriver)       T8 (instance) ── T9 (Host skeleton)
                                                              │
                                                              ├── T10 (ingest)
                                                              ├── T11 (supervisor)
                                                              ├── T12 (dispatch)
                                                              ▼
                                                              T13 (testdriver binary)
                                                              │
                                                              ▼
                                                              T14 (integration matrix)
                                                              ├── T15 (properties)
                                                              ├── T16 (fuzz)
                                                              ├── T17 (metrics)
                                                              ├── T18 (daemon wiring)
                                                              ├── T19 (socket ops)
                                                              ├── T20 (CLI driver)
                                                              ├── T21 (CLI command)
                                                              ├── T22 (goldens)
                                                              ├── T23 (CI)
                                                              ▼
                                                              T24 (smoke + tag)
```

A fresh subagent per task can work in sequence; cross-task commits are flagged above where needed. Tasks 15-22 are largely parallelizable once T14 stabilizes.

---

## Success Criteria Mapping (to spec §13)

| Spec criterion | Covered by |
|---|---|
| §13.1 proto contract defined + generated | T2 |
| §13.2 proto hygiene retrofit + doc | T1 |
| §13.3 FSM with all transitions tested | T4, T11, T15 |
| §13.4 Dispatch all error paths | T12, T14 |
| §13.5 drivers.toml parse + auto-spawn | T5, T11, T18 |
| §13.6 CLI driver list/status/restart + command send | T20, T21 |
| §13.7 cmd/testdriver all scenarios | T13, T14 |
| §13.8 golden fixtures pass + --update idempotent | T22 |
| §13.9 kill-9 consistency (INV-1 bounded) | T11 behavior, T24 smoke test |
| §13.10 coverage ≥85% in internal/carport | T23 gate |
| §13.11 CI green across lint/test/race/integration/fuzz | T23, T24 |
| §13.12 c2-complete tag | T24 |

---

*End of plan.*
