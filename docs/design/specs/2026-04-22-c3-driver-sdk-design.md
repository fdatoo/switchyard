# C3 — Driver SDK (Go) Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Predecessor:** [C2 — Carport Protocol v1](./2026-04-21-c2-carport-protocol-design.md)
**Date:** 2026-04-22
**Status:** Draft → Implementation-ready once approved

---

## Table of Contents

1. [Scope & Deliverables](#1-scope--deliverables)
2. [Repository & Module Layout](#2-repository--module-layout)
3. [Architecture Overview](#3-architecture-overview)
4. [Package: `protocol`](#4-package-protocol)
5. [Package: `driver`](#5-package-driver)
6. [Package: `drivertest`](#6-package-drivertest)
7. [Example Driver: `fakedevice`](#7-example-driver-fakedevice)
8. [Documentation](#8-documentation)
9. [SDK Testing Strategy](#9-sdk-testing-strategy)
10. [Dependencies & Versioning](#10-dependencies--versioning)
11. [Success Criteria](#11-success-criteria)
12. [Explicit Deferrals](#12-explicit-deferrals)
13. [Decision Record](#13-decision-record)
14. [Task Breakdown](#14-task-breakdown)

---

## 1. Scope & Deliverables

C3 delivers the Go library that driver authors use to write Carport-speaking gohome drivers. It lives in a **separate repository** (`github.com/fynn-labs/gohome-driverkit`) with its own module, version, and release cadence. The main `gohome` repo is a dependency (for generated proto types), not the other way around.

### 1.1 What C3 delivers

- **`protocol` package** — low-level Carport protocol implementation: env var ingestion, Unix domain socket setup, gRPC server lifecycle, handshake secret verification, heartbeat pong, Health RPC, Shutdown RPC, Run stream dispatch.
- **`driver` package** — high-level driver builder: entity registry with state tracking, capability handler routing, `EmitState` for background goroutines, reconnect loop.
- **`drivertest` package** — in-process `Harness` (fake Carport host for unit tests) and a `drivertest` CLI binary (compiled driver integration / acceptance harness).
- **`examples/fakedevice`** — realistic dimmable-light example driver with its own `_test.go`.
- **Documentation** — `README.md` quick start, `docs/getting-started.md`, `docs/testing.md`, `docs/protocol.md`, Go package doc comments throughout.

### 1.2 What C3 does NOT include

| Scope | Doc / milestone |
|---|---|
| Pkl-typed driver manifests (`DriverManifest.pkl_module`) | C4 |
| Remote/edge TLS transport | C12 |
| Signed manifest verification | C13 |
| WASM driver tier | v1.x |
| Entity class type system (typed `Attributes` constructors per domain) | C4 (Pkl) |
| First-party production drivers (Hue, Z2M, Matter, …) | Post-C4 |

---

## 2. Repository & Module Layout

```
github.com/fynn-labs/gohome-driverkit   # repo + Go module root

gohome-driverkit/
├── go.mod                               # module github.com/fynn-labs/gohome-driverkit
├── go.sum
├── doc.go                               # package driverkit (top-level orientation comment)
├── protocol/
│   ├── doc.go
│   └── conn.go                          # Conn, Handler, Emitter, FromEnv, Serve
├── driver/
│   ├── doc.go
│   ├── driver.go                        # Driver, New, AddEntity, OnCapability, EmitState, Run
│   └── entity.go                        # EntitySpec
├── drivertest/
│   ├── doc.go
│   ├── harness.go                       # Harness, New, SendCommand, StateChanges, AssertState
│   └── cmd/
│       └── drivertest/
│           └── main.go                  # CLI binary harness
├── examples/
│   └── fakedevice/
│       ├── main.go                      # Dimmable-light example driver
│       └── fakedevice_test.go           # Uses drivertest.New
└── docs/
    ├── getting-started.md
    ├── testing.md
    └── protocol.md
```

### 2.1 Public vs internal

All three packages (`protocol`, `driver`, `drivertest`) are public — this is the SDK's entire point. There is no `internal/` in this repo. The `drivertest/cmd/drivertest` binary is published as a standalone installable artifact.

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────┐
│  driver author code                         │
│                                             │
│  d := driver.New("my-driver", "1.0.0")     │
│  d.AddEntity(...)                           │
│  d.OnCapability(...)                        │
│  d.Run(ctx)                                 │
└───────────────┬─────────────────────────────┘
                │ implements protocol.Handler
                ▼
┌─────────────────────────────────────────────┐
│  driver package                             │
│  - entity registry + state tracking         │
│  - capability handler routing               │
│  - reconnect loop (calls Conn.Serve loop)  │
└───────────────┬─────────────────────────────┘
                │ calls
                ▼
┌─────────────────────────────────────────────┐
│  protocol package                           │
│  - reads env vars                           │
│  - opens UDS, starts gRPC server            │
│  - Handshake (secret verify, version check) │
│  - Run stream (commands in, events out)     │
│  - Health, Shutdown, Heartbeat pong         │
└───────────────┬─────────────────────────────┘
                │ Unix domain socket
                ▼
         gohomed / drivertest.Harness
```

**Dependency direction:** `driver` imports `protocol`; `drivertest` imports both; `examples/fakedevice` imports `driver` only.

---

## 4. Package: `protocol`

The `protocol` package is the complete low-level Carport implementation. A power user who needs fine-grained control (custom Health behaviour, wrapping an existing gRPC server, etc.) uses this package directly. Most authors never touch it.

### 4.1 `Conn` and construction

```go
// Conn holds the parameters for a single Carport connection.
type Conn struct {
    SocketPath string // GOHOME_CARPORT_SOCKET
    Secret     string // GOHOME_CARPORT_SECRET
    InstanceID string // GOHOME_CARPORT_INSTANCE_ID
    Config     []byte // GOHOME_CARPORT_INSTANCE_CONFIG (raw; driver parses its own)

    // HealthChecker, if non-nil, is called during Health RPCs.
    // Defaults to always returning ok=true.
    HealthChecker func(ctx context.Context) (ok bool, detail string)
}

// FromEnv constructs a Conn from Carport environment variables.
// Returns an error if GOHOME_CARPORT_SOCKET is unset.
func FromEnv() (*Conn, error)
```

### 4.2 `Emitter`

```go
// Emitter sends messages on the active Run stream.
// Safe to call from any goroutine while the stream is open.
type Emitter interface {
    Send(msg *carportpb.DriverToHost) error
}
```

### 4.3 `Handler` interface

```go
// Handler is implemented by the driver layer (or directly by power users).
type Handler interface {
    // OnHandshake is called once during the Handshake RPC, after the handshake
    // secret is verified. config is the raw instance config bytes (Conn.Config).
    // Returns the driver manifest and the initial entity list.
    OnHandshake(config []byte) (*carportpb.DriverManifest, []*eventpb.EntityRegistered, error)

    // OnRunStart is called when the Run stream opens. emit is valid for the
    // stream's lifetime. Implementations typically store emit for use in
    // background goroutines (e.g. polling a device).
    OnRunStart(ctx context.Context, emit Emitter)

    // OnCommand is called for each Command received on the Run stream.
    // Returns the CommandResult to send back. A non-nil error maps to
    // CARPORT_INTERNAL with the error message surfaced.
    OnCommand(ctx context.Context, cmd *carportpb.Command, emit Emitter) (*carportpb.CommandResult, error)

    // OnShutdown is called when the host sends a graceful Shutdown RPC.
    // Implementations should flush any in-flight work.
    OnShutdown(ctx context.Context) error
}
```

### 4.4 `Serve`

```go
// Serve starts the gRPC server on c.SocketPath, drives the Carport protocol
// for one connection lifetime, and returns when the Run stream closes or ctx
// is cancelled. It does NOT reconnect — callers manage reconnect loops.
func (c *Conn) Serve(ctx context.Context, h Handler) error
```

**What `Serve` owns internally:**

- Starts a `net.Listen("unix", c.SocketPath)` and a gRPC server.
- Implements `carportpb.DriverServer` internally; delegates to `h` after protocol machinery.
- Handshake: verifies `HandshakeRequest.HandshakeSecret == c.Secret`; checks protocol version is `"v1alpha1"`; calls `h.OnHandshake`.
- Health: calls `c.HealthChecker` (or returns `ok: true` by default).
- Shutdown: calls `h.OnShutdown`, then returns `ShutdownResponse{Acknowledged: true}`.
- Run stream: calls `h.OnRunStart` with an `Emitter`; reads the stream in a loop; responds to `Heartbeat.Ping` automatically (no handler involvement); routes `Command` messages to `h.OnCommand`.

---

## 5. Package: `driver`

The `driver` package is what most authors use. It implements `protocol.Handler` and adds entity registration, state tracking, capability handler routing, and the reconnect loop.

### 5.1 `EntitySpec`

```go
// EntitySpec describes a driver-owned entity.
type EntitySpec struct {
    EntityType   string   // closed enum: "light", "sensor", "switch", etc.
    FriendlyName string
    Capabilities []string // advertised in the manifest and returned on handshake
}
```

### 5.2 `CapabilityHandler`

```go
// CapabilityHandler handles a single capability invocation.
// The returned *entitypb.Attributes becomes the entity's new tracked state
// and is automatically emitted as a StateChanged event.
// Return (nil, nil) to send a successful CommandResult without updating state.
type CapabilityHandler func(
    ctx context.Context,
    entityID string,
    args map[string]string,
) (*entitypb.Attributes, error)
```

### 5.3 `Driver`

```go
// Driver is the high-level SDK entry point for writing Carport drivers.
type Driver struct { /* unexported fields */ }

// New creates a Driver with the given name and version.
func New(name, version string) *Driver

// AddEntity registers an entity. Must be called before Run.
// entityID format: "<type>.<name>" e.g. "light.kitchen"
// Returns ErrEntityAlreadyRegistered if the id is already taken.
func (d *Driver) AddEntity(entityID string, spec EntitySpec) error

// OnCapability registers a handler for a specific entity+capability pair.
// Panics if entityID was not registered via AddEntity.
func (d *Driver) OnCapability(entityID, capability string, h CapabilityHandler)

// EmitState updates the tracked state for entityID and sends a StateChanged
// event on the current Run stream. Safe to call from any goroutine.
// Returns ErrNotConnected if no stream is currently active.
func (d *Driver) EmitState(entityID string, attrs *entitypb.Attributes) error

// Run reads Conn from env vars and calls Conn.Serve in a reconnect loop
// until ctx is cancelled. Backoff: 1s initial, 30s max, exponential.
func (d *Driver) Run(ctx context.Context) error
```

### 5.4 State tracking semantics

- `AddEntity` initialises the entity's tracked `*entitypb.Attributes` to `nil`.
- When a `CapabilityHandler` returns non-nil attrs: tracked state is updated atomically, then a `StateChanged` is sent on the active stream.
- `EmitState` updates tracked state atomically, then sends `StateChanged`.
- On reconnect, `OnHandshake` returns all registered entities as `initial_entities`, each carrying its current tracked attrs as initial state. A driver that was polling before a host reconnect presents fresh state immediately without re-querying the device.
- Tracked state is protected by a `sync.RWMutex`; all reads and writes are safe across goroutines.

### 5.5 Error variables

```go
var (
    ErrEntityAlreadyRegistered = errors.New("entity already registered")
    ErrEntityUnknown           = errors.New("entity unknown")
    ErrCapabilityUnknown       = errors.New("capability unknown") // → CARPORT_UNSUPPORTED_CAPABILITY
    ErrNotConnected            = errors.New("no active run stream")
)
```

### 5.6 Reconnect loop

`Run` calls `protocol.FromEnv()` once, then calls `conn.Serve` in a loop:

```
for {
    err := conn.Serve(ctx, d)
    if ctx.Err() != nil { return nil }
    log (warn): reconnecting in <backoff>
    sleep(backoff); backoff = min(backoff*2, 30s)
}
```

The backoff resets to 1s after a successful `Serve` that lasted longer than 5s (indicating a healthy session, not a crash loop).

---

## 6. Package: `drivertest`

### 6.1 In-process `Harness`

The `Harness` acts as a fake Carport host. It creates a temp UDS, sets the Carport env vars, starts the driver's `Run` in a background goroutine, dials in as a gRPC client, and performs the handshake — mirroring exactly what `gohomed` does, but in-process and without subprocess overhead.

```go
// New creates a Harness for d, starts d.Run() in the background, performs
// the Carport handshake, and opens the Run stream.
// The harness shuts down automatically via t.Cleanup.
func New(t testing.TB, d *driver.Driver) *Harness

type Harness struct { /* unexported */ }

// Entities returns the initial entity list from the handshake response.
func (h *Harness) Entities() []*eventpb.EntityRegistered

// SendCommand delivers a command to the driver and blocks for the
// CommandResult. Returns an error if the stream is closed or ctx expires.
func (h *Harness) SendCommand(
    ctx context.Context,
    entityID, capability string,
    args map[string]string,
) (*carportpb.CommandResult, error)

// StateChanges returns a channel that receives every StateChanged event
// emitted by the driver (both command-triggered and background).
// The channel is closed when the harness shuts down.
func (h *Harness) StateChanges() <-chan *eventpb.StateChanged

// AssertState fails the test if the last StateChanged received for entityID
// does not match want. Waits up to 1s for an event before failing.
func (h *Harness) AssertState(t testing.TB, entityID string, want *entitypb.Attributes)
```

Typical test shape:

```go
func TestTurnOn(t *testing.T) {
    d := driver.New("fakedevice", "0.1.0")
    d.AddEntity("light.fake", driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Fake Light",
        Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
    })
    d.OnCapability("light.fake", "turn_on", handleTurnOn)

    h := drivertest.New(t, d)

    res, err := h.SendCommand(context.Background(), "light.fake", "turn_on", nil)
    require.NoError(t, err)
    require.True(t, res.Ok)
    h.AssertState(t, "light.fake", wantOnAttrs)
}
```

### 6.2 CLI binary harness

Exercises a compiled driver binary end-to-end. Useful in CI for integration/acceptance testing without importing the SDK.

**Usage:**

```
drivertest run ./my-driver \
    --instance-id my_instance \
    --config '{"key":"value"}' \
    --scenario happy-path \
    --timeout 30s \
    [--json]
```

**v1 built-in scenarios:**

| Scenario | What it asserts |
|---|---|
| `happy-path` | Handshake succeeds; at least one entity registered; one `SendCommand` per declared capability returns `ok: true`; clean `Shutdown`. |
| `reconnect` | Handshake, close stream, reconnect, assert entity list matches first handshake exactly (state tracking on reconnect). |

Output is human-readable by default; `--json` produces structured output for CI reporting. Exit code 0 = all assertions passed; non-zero = failure with reason.

**Install:**

```
go install github.com/fynn-labs/gohome-driverkit/drivertest/cmd/drivertest@latest
```

---

## 7. Example Driver: `fakedevice`

A dimmable light driver that tracks on/off and brightness state. Shows: initial state setup, multi-capability routing, arg validation, concurrent state management, and background `EmitState`.

```go
// examples/fakedevice/main.go
package main

import (
    "context"
    "fmt"
    "log"
    "strconv"
    "sync"

    "github.com/fynn-labs/gohome-driverkit/driver"
    entitypb "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
)

func main() {
    d := driver.New("fakedevice", "0.1.0")

    var mu sync.Mutex
    var on bool
    var brightness int32 = 100

    d.AddEntity("light.fake_light", driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Fake Light",
        Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
    })

    d.OnCapability("light.fake_light", "turn_on",
        func(_ context.Context, _ string, _ map[string]string) (*entitypb.Attributes, error) {
            mu.Lock(); on = true; b := brightness; mu.Unlock()
            return lightAttrs(true, b), nil
        })

    d.OnCapability("light.fake_light", "turn_off",
        func(_ context.Context, _ string, _ map[string]string) (*entitypb.Attributes, error) {
            mu.Lock(); on = false; mu.Unlock()
            return lightAttrs(false, 0), nil
        })

    d.OnCapability("light.fake_light", "set_brightness",
        func(_ context.Context, _ string, args map[string]string) (*entitypb.Attributes, error) {
            v, err := strconv.Atoi(args["brightness"])
            if err != nil || v < 0 || v > 100 {
                return nil, fmt.Errorf("brightness must be 0–100")
            }
            mu.Lock(); brightness = int32(v); isOn := on; mu.Unlock()
            return lightAttrs(isOn, int32(v)), nil
        })

    log.Fatal(d.Run(context.Background()))
}
```

`examples/fakedevice/fakedevice_test.go` tests all three capabilities using `drivertest.New`, and also exercises the `reconnect` scenario to verify state is preserved across reconnects.

---

## 8. Documentation

### 8.1 `README.md`

- One-paragraph orientation: what the SDK is, who it's for.
- 15-line "hello world" using `driver.New` + `d.Run`.
- Install: `go get github.com/fynn-labs/gohome-driverkit`.
- Links to the three docs pages and the `fakedevice` example.
- Carport protocol version compatibility table:

  | driverkit | Carport protocol | gohome |
  |---|---|---|
  | v0.x | v1alpha1 | C2+ |

### 8.2 `docs/getting-started.md`

The primary resource for driver authors. Covers end-to-end:

1. Create a new Go module (`go mod init`).
2. Add dependency on `gohome-driverkit`.
3. Declare entities with `driver.AddEntity`.
4. Register capability handlers with `driver.OnCapability` — including arg validation patterns.
5. Call `d.Run(context.Background())` in `main`.
6. Write tests with `drivertest.New`.
7. Build the binary and add it to `drivers.toml`.
8. Run `gohome driver status` to verify the driver is healthy.

Uses `fakedevice` as the running example throughout.

### 8.3 `docs/testing.md`

Two sections:

**In-process harness** — `drivertest.New`, `SendCommand`, `AssertState`, the `StateChanges()` channel (how to consume background events), how `t.Cleanup` handles shutdown.

**CLI harness** — `drivertest run` flags, scenario descriptions, JSON output format, how to wire into GitHub Actions / CI.

### 8.4 `docs/protocol.md`

For power users using `protocol.Conn` directly. Covers: when to choose this layer over `driver.Driver`, `FromEnv`, the `Handler` interface (method-by-method), `Emitter`, `HealthChecker`, reconnect responsibility. Includes a minimal working example.

### 8.5 Go package doc comments

Every exported type and function carries a one-line doc comment. Each package has a `doc.go` with a two-sentence orientation:

- `protocol`: *"Package protocol is the low-level Carport driver implementation. Most authors use the driver package instead."*
- `driver`: *"Package driver is the high-level SDK for writing gohome Carport drivers. Start with New."*
- `drivertest`: *"Package drivertest provides test helpers for Carport drivers. Use New for in-process unit tests and the drivertest CLI for binary integration tests."*

---

## 9. SDK Testing Strategy

The SDK tests itself using its own abstractions where possible.

| Layer | Approach |
|---|---|
| `protocol` | Unit tests use `fakedriver.Double` from `gohome` (test-only import) as a gRPC client; verifies handshake secret, version rejection, heartbeat pong, Health/Shutdown RPCs, and Run stream dispatch. |
| `driver` | Unit tests use `drivertest.New` (dogfoods the harness); covers entity registration, capability routing, state tracking on reconnect, `EmitState` from a goroutine, error variables. |
| `drivertest/harness` | Tested against the `fakedevice` example driver in-process. |
| `drivertest/cmd` | Integration test: compiles `fakedevice`, runs `drivertest run` against the binary for both scenarios, asserts exit code and JSON output. |

CI matrix: `linux/amd64`, `linux/arm64`, `darwin/arm64` — matching the main repo.

---

## 10. Dependencies & Versioning

### 10.1 `go.mod` dependencies

| Dependency | Why |
|---|---|
| `github.com/fynn-labs/gohome` | Generated proto types (`gen/gohome/carport/v1alpha1`, `gen/gohome/event/v1`, `gen/gohome/entity/v1`) |
| `google.golang.org/grpc` | gRPC server and client |
| `google.golang.org/protobuf` | Protobuf runtime |

No other runtime dependencies. `fakedriver.Double` from `gohome/internal/carport/fakedriver` is a **test-only** import — it does not appear in the runtime dependency graph.

### 10.2 Version policy

- The module tracks `gohome`'s Carport version via a pinned `go.mod` dependency.
- While the protocol is `v1alpha1`, this module is `v0.x` — no backward-compat promises.
- When Carport graduates to `v1`, this module cuts `v1.0.0`. That is a one-way door and requires a decision record entry (mirroring C2's DR-11).

---

## 11. Success Criteria

C3 is complete when:

1. `go build ./...` and `go test ./...` pass with no errors in the `gohome-driverkit` repo.
2. `protocol.Conn.Serve` correctly handles: handshake secret mismatch, protocol version mismatch, heartbeat pong, Health RPC (default and custom), Shutdown RPC, and Run stream command/event dispatch.
3. `driver.Driver.Run` reconnects after a stream close with exponential backoff; backoff resets after a healthy session.
4. State tracked via `CapabilityHandler` return and `EmitState` is consistent with `initial_entities` returned on reconnect.
5. `drivertest.New` can exercise all three `fakedevice` capabilities and the reconnect scenario in-process.
6. `drivertest run` CLI passes both `happy-path` and `reconnect` scenarios against the compiled `fakedevice` binary; exit code and JSON output are correct.
7. All four documentation files (`README.md`, `docs/getting-started.md`, `docs/testing.md`, `docs/protocol.md`) are written and accurate relative to the implemented API.
8. Every exported symbol has a Go doc comment.
9. CI (`lint`, `test`, `test:race`, integration test) is green on `linux/{amd64,arm64}` and `darwin/arm64`.
10. Git tag `c3-complete` applied.

---

## 12. Explicit Deferrals

| Deferred | To | Note |
|---|---|---|
| Pkl-typed driver manifests | C4 | `DriverManifest.pkl_module` field is reserved; SDK returns it empty. |
| Entity class typed constructors (e.g. `lightAttrs` helpers) | C4 | Authors construct `*entitypb.Attributes` directly in C3. |
| Remote/edge TLS transport | C12 | `protocol.Conn` uses UDS only; TLS is additive in C12. |
| Signed manifest verification | C13 | No signing in SDK v0.x. |
| WASM driver tier | v1.x | SDK is binary-only in v0.x. |
| Additional `drivertest` scenarios (stress, malformed args, …) | Post-C3 | `happy-path` and `reconnect` are sufficient for v0.x. |

---

## 13. Decision Record

| # | Decision | Alternatives considered | Reason |
|---|---|---|---|
| DR-1 | **Separate repo (`gohome-driverkit`), not a package in the main repo.** | (A) `pkg/driver` in `gohome`; (B) inline in `internal/` | Driver authors should not depend on `gohome` daemon internals. A separate module gives clean versioning, independent release cadence, and a minimal dependency surface. |
| DR-2 | **Layered architecture: `protocol` (low-level) + `driver` (high-level).** | (A) single flat package; (B) interface-only thin wrapper | Power users occasionally need raw protocol control (custom Health, wrapping existing servers). Two layers gives them an escape hatch without exposing gRPC internals to the majority of authors. |
| DR-3 | **`driver.Driver` tracks entity state; reconnect returns current tracked attrs as `initial_entities`.** | (A) driver author manages state themselves; (B) no state tracking | The most common driver bug is presenting stale state after a host reconnect. Tracking state in the SDK eliminates this class of bug without restricting author flexibility. |
| DR-4 | **`CapabilityHandler` returns `*entitypb.Attributes`; `nil` return means no state update.** | (A) separate `EmitState` call required; (B) always require a return | Most capabilities naturally produce new state. Making the return value the update avoids a separate `EmitState` call in the common case; `nil` is the clean escape for commands that don't produce state changes. |
| DR-5 | **Reconnect loop lives in `driver.Run`, not in `protocol.Conn.Serve`.** | (A) `Serve` reconnects internally; (B) author manages loop | Power users of `protocol.Conn` expect single-connection semantics. Putting the loop in `driver` keeps the `protocol` package predictable and composable. |
| DR-6 | **`drivertest.New` starts the driver in-process and dials as a gRPC client.** | (A) mock the protocol layer; (B) always use subprocess | In-process means tests run fast and don't require a compiled binary. The harness exercises the real gRPC path (including heartbeat and reconnect), unlike a mock. |
| DR-7 | **Two test harness tiers: in-process (`drivertest.New`) + CLI binary (`drivertest run`).** | (A) in-process only; (B) binary only | Disjoint coverage: in-process is fast and precise for unit/integration tests in the driver's own repo; binary harness tests the compiled artifact in CI, catches build-tag and linking issues the in-process harness misses. |
| DR-8 | **`fakedevice` example uses a realistic dimmable light (not a stub).** | (A) minimal stub; (B) both | A realistic example shows all patterns authors need: state management, multi-capability routing, arg validation, concurrent access. A stub adds little beyond `docs/getting-started.md`'s inline snippets. |
| DR-9 | **Module is `v0.x` until Carport graduates to `v1`.** | (A) ship `v1.0.0` immediately | We haven't exercised the SDK with real non-fake drivers yet. `v0.x` reserves the right to adjust the API if C3–C6 surface issues. Graduation to `v1` is its own decision, mirroring C2's DR-11. |

---

## 14. Task Breakdown

High-level tasks in dependency order. The implementation plan (produced by `writing-plans` after spec approval) will decompose these into TDD steps.

1. **Repo bootstrap** — `git init`, `go mod init github.com/fynn-labs/gohome-driverkit`, CI workflow (lint, test, test:race, integration matrix), `golangci.yml`.
2. **`protocol/conn.go`** — `Conn`, `FromEnv`, `Emitter`, `Handler` interface, `Serve` (gRPC server, UDS, handshake, health, shutdown, run stream).
3. **`protocol` unit tests** — using `fakedriver.Double` as the gRPC client side.
4. **`driver/entity.go`** — `EntitySpec`.
5. **`driver/driver.go`** — `Driver`, `New`, `AddEntity`, `OnCapability`, `EmitState`, state tracking, reconnect loop.
6. **`drivertest/harness.go`** — `Harness`, `New`, `SendCommand`, `StateChanges`, `AssertState`.
7. **`driver` unit tests** — using `drivertest.New`.
8. **`examples/fakedevice/main.go`** — dimmable light example.
9. **`examples/fakedevice/fakedevice_test.go`** — capability tests + reconnect test using `drivertest.New`.
10. **`drivertest/cmd/drivertest/main.go`** — CLI binary: `run` command, `happy-path` and `reconnect` scenarios, JSON output.
11. **CLI integration test** — compiles `fakedevice`, runs `drivertest run` against it for both scenarios.
12. **Documentation** — `README.md`, `docs/getting-started.md`, `docs/testing.md`, `docs/protocol.md`, `doc.go` files, all exported symbol doc comments.
13. **Coverage gate + CI green** — enforce lint/test/race/integration on the full matrix.
14. **End-to-end smoke + tag** — manual walkthrough: write a trivial driver against the SDK, run `drivertest run`, verify health in `gohome driver status`; apply `c3-complete`.

---

*End of C3 design document.*
