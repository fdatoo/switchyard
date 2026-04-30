# C2 — Carport Protocol v1 Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Predecessor:** [C1 — Event Core & Storage](./2026-04-21-c1-event-core-and-storage-design.md)
**Date:** 2026-04-21
**Status:** Draft → Implementation-ready once approved

---

## Table of Contents

1. [Scope & Deliverables](#1-scope--deliverables)
2. [Architecture Overview](#2-architecture-overview)
3. [Package & Proto Layout](#3-package--proto-layout)
4. [Protocol Wire Spec](#4-protocol-wire-spec)
5. [Driver Instance Configuration](#5-driver-instance-configuration)
6. [Lifecycle State Machine](#6-lifecycle-state-machine)
7. [Data Flow](#7-data-flow)
8. [Error Model](#8-error-model)
9. [Observability](#9-observability)
10. [CLI Additions](#10-cli-additions)
11. [Testing Strategy](#11-testing-strategy)
12. [Proto Hygiene Rules](#12-proto-hygiene-rules)
13. [Success Criteria](#13-success-criteria)
14. [Explicit Deferrals](#14-explicit-deferrals)
15. [Decision Record](#15-decision-record)
16. [Task Breakdown](#16-task-breakdown)

---

## 1. Scope & Deliverables

C2 delivers the first half of gohome's extensibility spine: the **Carport driver protocol** (`gohome.carport.v1alpha1`) and the **host-side supervisor** (`internal/carport`) that spawns, supervises, dispatches commands to, and ingests events from driver subprocesses over local Unix-domain sockets.

### 1.1 Packages (in `github.com/fynn-labs/gohome`)

- `proto/gohome/carport/v1alpha1/` — public wire contract (protobuf)
- `internal/carport/host` — supervisor, lifecycle state machine, `Host.Dispatch`
- `internal/carport/routing` — entity→driver-instance resolution (reads the registry projection)
- `internal/carport/stream` — bidi envelope codec, pending-waiter correlation map
- `internal/carport/config` — `drivers.toml` loader + validator
- `internal/carport/fakedriver` — in-process `Driver` server double for fast unit tests
- `cmd/testdriver/` — scenario-driven real driver binary for integration tests
- `cmd/gohomed/main.go` — wires `carport.Host` into the existing supervisord startup/shutdown sequence
- `cmd/gohome/` — gains `driver list/status/restart` and `command send` subcommands

### 1.2 What C2 does NOT include

Deferred to later child docs (per master doc roadmap, with this doc's renumbering):

| Scope | Doc |
|-------|-----|
| Remote/edge TLS transport for Carport | C12 (`gohome-edge`) |
| mTLS pairing ceremony (CA mint, cert issuance, `gohome edge pair`) | C12 |
| Pkl-typed driver manifests & instance config schemas | C4 (Pkl loader) |
| Connect-RPC `EntityService.CallCapability` (caller of `Dispatch`) | C7 |
| Automation-engine retry policy on command failure | C6 |
| Signed driver manifest verification | C13 |
| Surgical repair CLI for orphan events | C13 |
| WASM driver tier | v1.x |

Each of these leaves **additive** hooks in C2's protocol; no graduation to `v1` happens until those hooks are exercised end-to-end.

### 1.3 Deliverable binaries (changes from C1)

- `gohomed` — now auto-spawns declared driver instances on startup; waits for health; supervises through the daemon's lifetime.
- `gohome` — gains `driver list / status / restart` and `command send` subcommands.
- `testdriver` (**new**, `cmd/testdriver/`) — scenario-driven driver binary, shipped **only** in this repo for C2's integration tests. Not a user-facing artifact.

---

## 2. Architecture Overview

`gohomed` gains a new internal subsystem, **`carport.Host`**, wired into supervisord between `eventstore` and `api`:

```
startup:  storage → eventstore → state → registry → carport.Host → (future: api) → metrics/http
shutdown: reverse
```

`carport.Host` is responsible for:

1. **Config load.** Read `$GOHOME_CONFIG_DIR/drivers.toml` at startup. Validate. Construct an in-memory instance table.
2. **Spawn.** For each enabled instance, `exec.Cmd.Start` the declared binary with a freshly-generated Unix socket path (`GOHOME_CARPORT_SOCKET`) and a per-launch handshake secret (`GOHOME_CARPORT_SECRET`) passed through env vars.
3. **Connect & handshake.** gRPC-dial the subprocess over its UDS, call `Handshake` (protocol-version check, secret verification, manifest exchange, instance-config delivery), apply the driver's initial registry delta (`EntityRegistered` events).
4. **Run.** Hold the `Run` bidi stream open for the driver's lifetime:
   - host → driver: `Command` (from `Dispatch` callers), periodic `Heartbeat` ping.
   - driver → host: `CommandResult`, `StateChanged`, `EntityRegistered`, `EntityUnregistered`, `DriverEvent` (driver-typed passthrough), `Heartbeat` pong.
5. **Health probe.** Periodic `Health` RPC out-of-band from the `Run` stream; N consecutive failures trigger restart.
6. **Crash detect.** Subprocess death, stream error, or health failure all funnel into the same restart path.
7. **Backoff & quarantine.** Exponential backoff on restart; quarantine instances that exhaust their restart budget.
8. **Shutdown.** On daemon `SIGTERM`, call `Shutdown` RPC on every running instance, wait grace period, hard-kill stragglers.

Callers (in C2: the CLI and integration tests; in C7: the Connect-RPC API layer) use one entry point:

```go
Host.Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*CommandResult, error)
```

which atomically:

1. Resolves the entity to an instance.
2. Appends `CommandIssued` to the eventstore.
3. Sends the `Command` on the instance's `Run` stream.
4. Waits (bounded by context deadline) for the matching `CommandResult`.
5. Appends `CommandAck` with causation pointing at the `CommandIssued` position.
6. Returns to caller.

Events are the durable audit trail; `Dispatch` is the control-plane call. See [§7 Data Flow](#7-data-flow) and [§15 Decision Record, DR-4](#15-decision-record).

### 2.1 Component map

```
 ┌────────────────────────────────────────────────────────────────────┐
 │ gohomed process                                                    │
 │                                                                    │
 │   ┌─────────────┐        ┌────────────────────────┐                │
 │   │ CLI/API     │──Dispatch──▶ carport.Host       │                │
 │   └─────────────┘        │  ├─ routing            │                │
 │                          │  ├─ stream (bidi + wait)│               │
 │                          │  ├─ config (TOML)      │                │
 │                          │  └─ supervisor (FSM)   │                │
 │                          └──┬─────────────────┬───┘                │
 │                             │ Append          │ spawn + UDS dial   │
 │                             ▼                 │                    │
 │                        ┌─────────┐            │                    │
 │                        │eventstore│           │                    │
 │                        │ +state   │           │                    │
 │                        │ +registry│◀──events──┘                    │
 │                        └─────────┘                                 │
 └──────────────────────────────────┬─────────────────────────────────┘
                                    │ Unix domain socket
                                    ▼
                          ┌───────────────────────┐
                          │ driver subprocess     │
                          │ (one per instance)    │
                          └───────────────────────┘
```

---

## 3. Package & Proto Layout

### 3.1 Proto files

```
proto/gohome/carport/v1alpha1/
├── carport.proto        # service Driver (RPCs)
├── envelope.proto       # HostToDriver, DriverToHost (Run-stream oneofs)
├── manifest.proto       # DriverManifest
└── errors.proto         # CarportErrorCode enum
```

Generated into `gen/gohome/carport/v1alpha1/` by the existing `buf generate` pipeline. `buf.gen.yaml` gains one line — no new plugin.

### 3.2 Go packages

```
internal/carport/
├── host/             # Host type, Dispatch, state machine driver, spawn logic
├── routing/          # Resolve(entity_id) (driver_instance_id, error)
├── stream/           # bidi envelope codec, pending-command waiter map
├── config/           # drivers.toml loader + validator
└── fakedriver/       # in-process Driver gRPC server for unit tests
cmd/
├── testdriver/       # real driver binary, scenario via TESTDRIVER_MODE env
├── gohomed/main.go   # adds carport.Host to supervisord
└── gohome/           # adds driver{list,status,restart} and command{send}
```

### 3.3 Public vs internal

The **protobuf** package is public — driver authors (C3, community) will import `gen/gohome/carport/v1alpha1/`. The **Go supervisor** is internal; it is not API to driver authors.

---

## 4. Protocol Wire Spec

### 4.1 Service definition (`carport.proto`)

```proto
syntax = "proto3";

package gohome.carport.v1alpha1;

import "gohome/carport/v1alpha1/envelope.proto";
import "gohome/carport/v1alpha1/manifest.proto";

service Driver {
  // Initial negotiation. Must be called exactly once, before Run.
  rpc Handshake(HandshakeRequest) returns (HandshakeResponse);

  // The lifetime bidi stream. Host sends Commands; driver sends events,
  // results, and heartbeat pongs.
  rpc Run(stream HostToDriver) returns (stream DriverToHost);

  // Out-of-band liveness probe. Independent of the Run stream.
  rpc Health(HealthRequest) returns (HealthResponse);

  // Graceful shutdown request. Driver should flush in-flight state,
  // close the Run stream, then exit.
  rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
}

message HandshakeRequest {
  // 1-9: identity & version
  string protocol_version = 1;    // "v1alpha1"
  string instance_id      = 2;    // slug from drivers.toml
  string handshake_secret = 3;    // must match GOHOME_CARPORT_SECRET env

  // 10-19: instance-specific config
  bytes  instance_config  = 10;   // opaque to host; driver parses its own
}

message HandshakeResponse {
  // 1-9: identity & version (must match request where applicable)
  string protocol_version = 1;

  // 10-19: driver self-description
  DriverManifest manifest = 10;

  // 20-29: initial registry delta
  repeated EntityRegistered initial_entities = 20;
}

message HealthRequest  {}
message HealthResponse {
  bool   ok     = 1;
  string detail = 2;    // free-form; human-readable when ok=false
}

message ShutdownRequest  { int64 grace_ms = 1; }
message ShutdownResponse { bool  acknowledged = 1; }
```

### 4.2 Manifest (`manifest.proto`)

```proto
message DriverManifest {
  // 1-9: identity
  string name             = 1;
  string version          = 2;
  string protocol_version = 3;

  // 10-19: capability surface (structured, queryable)
  repeated string supported_capabilities = 10;

  // 90-99: future-structured fields (reserved for C4 Pkl integration)
  bytes pkl_module = 99;   // reserved; empty in C2
}
```

### 4.3 Run-stream envelope (`envelope.proto`)

```proto
message HostToDriver {
  oneof kind {
    // 1-9: command plane
    Command command = 1;

    // 90-99: transport/health
    Heartbeat ping = 90;
  }
}

message DriverToHost {
  oneof kind {
    // 1-9: command response plane
    CommandResult result = 1;

    // 10-19: state-plane events
    StateChanged       state_changed       = 10;
    EntityRegistered   entity_registered   = 11;
    EntityUnregistered entity_unregistered = 12;

    // 20-29: driver-typed passthrough
    DriverEvent driver_event = 20;

    // 90-99: transport/health
    Heartbeat pong = 90;
  }
}

message Heartbeat { int64 ts_unix_ms = 1; }

message Command {
  // 1-9: identity
  string command_id       = 1;   // UUIDv4, host-assigned
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
  bool   ok            = 10;
  CarportErrorCode code = 11;   // CARPORT_OK on success
  string error_message = 12;
}

message StateChanged       { string entity_id = 1; /* attributes in payload */ }
message EntityRegistered   { /* see §4.5 */ }
message EntityUnregistered { string entity_id = 1; string reason = 2; }
message DriverEvent        { string kind = 1; bytes payload = 2; string detail = 3; }
```

> **Type reuse.** `StateChanged`, `EntityRegistered`, `EntityUnregistered`, and `DriverEvent` inside the envelope are **the same protobuf messages** declared in `gohome.event.v1` (C1), imported via `import "gohome/event/v1/event.proto";`. One source of truth for these payload shapes. The envelope adds `Command`, `CommandResult`, and `Heartbeat` on top — types that are Carport-specific and live in the `v1alpha1` package.

### 4.4 Error enum (`errors.proto`)

```proto
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

### 4.5 Protocol semantics (normative)

- **Ordering on `Run`.** The stream is strictly ordered in each direction. A `StateChanged` emitted *because* of a `Command` MUST arrive after the `CommandResult` for that command. Violating drivers will surface as flaky tests; the host does not sort or dedupe.
- **Correlation.** Every `Command` carries a host-assigned UUIDv4 `command_id`. The matching `CommandResult` MUST echo it verbatim.
- **At-most-once.** Host never resends a `Command` on reconnect. The caller of `Dispatch` gets `CARPORT_DRIVER_SHUTTING_DOWN` or `CARPORT_TIMEOUT` for commands in flight when a stream dies. Retry policy is caller's concern (C6).
- **Initial registry delta.** `HandshakeResponse.initial_entities` is the authoritative view of what the driver *owns*. The host:
  - Applies `EntityRegistered` events for each (appended with `source = "driver:<instance_id>"`).
  - For entities the registry *already* shows owned by this instance but NOT in `initial_entities`: marks them `last_seen_ts = handshake_ts` but does **not** immediately unregister — deferred TTL-based cleanup lives in a later child doc (C6 / registry maintenance).
- **Heartbeat.** Host sends `Heartbeat.ping` every 10s; driver echoes as `Heartbeat.pong`. Absence for 30s (three missed intervals) counts as a health-probe failure equivalent.
- **Shutdown.** Host sends `Shutdown{grace_ms: 10000}` on SIGTERM. Driver SHOULD close its `Run` send-direction, flush in-flight work, return `ShutdownResponse{acknowledged:true}`, and exit. Host waits `grace_ms`; on timeout, `SIGTERM` to the process; after another 3s, `SIGKILL`.

---

## 5. Driver Instance Configuration

### 5.1 `drivers.toml` format

```toml
# $GOHOME_CONFIG_DIR/drivers.toml

[[instance]]
id         = "hue_main"
binary     = "/usr/local/bin/hue-driver"
enabled    = true
config_json = """
{
  "bridge_address": "10.0.0.42",
  "api_key_env":    "HUE_API_KEY",
  "poll_interval_seconds": 30
}
"""

# optional per-instance lifecycle tuning (sensible defaults otherwise)
[instance.lifecycle]
health_probe_interval_ms  = 15000
health_probe_timeout_ms   = 3000
health_failures_to_restart = 3
handshake_deadline_ms     = 5000
shutdown_grace_ms         = 10000
restart_backoff_initial_ms = 1000
restart_backoff_max_ms     = 60000
restart_budget_window_minutes  = 10
restart_budget_max         = 10
```

### 5.2 Validation (at load time, before any spawn)

- `id`: non-empty, `[a-z0-9_-]{1,64}`, unique across all instances.
- `binary`: non-empty path, file exists, mode bits include owner-executable.
- `config_json`: parses as valid JSON or is empty (`""`). Contents are NOT validated by the host — driver parses its own.
- Duplicate IDs → hard error at startup.
- Missing binary → hard error at startup (no silent skip).

### 5.3 Reload

In C2, `drivers.toml` is loaded **only at process start**. `gohomed` reload / `gohome config apply` semantics arrive with C4's Pkl loader. For C2, to reconfigure drivers: edit file, restart daemon.

### 5.4 Ownership note

`drivers.toml` is the C2-only control surface for driver instances. C4's Pkl loader will supersede it wholesale — the Pkl pipeline will emit a new `drivers.toml`-equivalent (or a typed protobuf `DriverInstanceSet`) and the loader here will be deprecated. No migration drama: the replacement is additive in C4; the loader shrinks to a fallback-only path, and is removed when C4 ships.

---

## 6. Lifecycle State Machine

### 6.1 States & transitions

```
      [declared]                                # row in drivers.toml, not yet touched
          │ start (startup or manual restart)
          ▼
      [spawning]  ─spawn error──────────────────▶ [failed]
          │ subprocess started, UDS created
          ▼
   [awaiting_handshake] ─timeout / bad version / bad secret / RPC err──▶ [failed]
          │ handshake ok
          ▼
       [running] ◀──────────────────────┐
          │                             │
          │ process exit / stream err / │
          │ N health probe failures     │
          ▼                             │
       [failed]                         │
          │ emit DriverEvent{failed},   │
          │ compute backoff             │
          ▼                             │
      [backoff]                         │
          │ budget ok?                  │
          ├──yes─────────spawn──────────┘
          │
          │ budget exhausted
          ▼
     [quarantined] ──`gohome driver restart`──▶ [spawning]
                      (manual intervention only)
          │
          │ daemon SIGTERM (from any state)
          ▼
       [stopping] ── grace or SIGKILL ─▶ [stopped]
```

### 6.2 State transition events (appended to eventstore)

Every meaningful transition produces a `DriverEvent` in the event log:

| From → To | `DriverEvent.kind` | `detail` |
|---|---|---|
| spawning → awaiting_handshake | `spawned` | pid |
| awaiting_handshake → running | `started` | manifest version |
| awaiting_handshake → failed | `handshake_failed` | reason |
| running → failed | `failed` | cause (crash / health / stream) |
| failed → backoff | `backoff_scheduled` | next_attempt_ts |
| backoff → quarantined | `quarantined` | reason |
| quarantined → spawning | `restart_manual` | actor |
| any → stopping | `stopping` | initiator |
| stopping → stopped | `stopped` | exit code |

All emitted with `source = "carport:host"` and `entity_id = ""` (these are about the instance, not an entity).

### 6.3 Policy defaults

| Tunable | Default | Rationale |
|---|---|---|
| `handshake_deadline_ms`        | 5000 | Fresh subprocesses come up fast; 5s is ample on a Pi. |
| `health_probe_interval_ms`     | 15000 | Low-cost, frequent enough to catch wedges in ≤45s worst-case. |
| `health_probe_timeout_ms`      | 3000 | 3×RTT on a loaded homelab network is generous. |
| `health_failures_to_restart`   | 3 | Avoids spurious restarts on a single missed probe. |
| `shutdown_grace_ms`            | 10000 | Matches gRPC connection-close norms; lets drivers flush. |
| `restart_backoff_initial_ms`   | 1000 | Fast first retry for transient startup races. |
| `restart_backoff_max_ms`       | 60000 | Upper bound prevents hot-loops. |
| `restart_budget_window_minutes`    | 10 | Rolling window for "too many failures." |
| `restart_budget_max`           | 10 | Tolerate flaps; quarantine the truly broken. |

All overridable per-instance in `drivers.toml`'s `[instance.lifecycle]` block.

### 6.4 Restart side effects

- Entities registered by the instance in its previous incarnation: kept in the registry projection with `last_seen_ts` unchanged until the new handshake. Reconciled from `HandshakeResponse.initial_entities`.
- In-flight `Dispatch` calls on that instance: released with `ErrStreamClosed` (`CARPORT_DRIVER_SHUTTING_DOWN`).
- Pending commands queued for delivery but not yet on the wire: dropped, same error to caller.
- No retry on the host side — C6 owns retry policy.

---

## 7. Data Flow

### 7.1 Dispatch end-to-end

```
caller ──► Host.Dispatch(ctx, "light.kitchen", "turn_on", {brightness:60})
              │
              ├─1. routing.Resolve("light.kitchen") → "hue_main"
              │       (err: ErrEntityUnknown if not in registry)
              ├─2. host.instances["hue_main"] → *instanceConn
              │       (err: ErrInstanceNotRunning if state ≠ running)
              ├─3. command_id = uuid.NewV4()
              ├─4. tx := db.Begin();
              │    eventstore.AppendTx(tx, CommandIssued{entity,capability,args,command_id})
              │    tx.Commit()
              ├─5. conn.pending[command_id] = chan *CommandResult
              ├─6. conn.send <- HostToDriver{Command{command_id, entity, capability, args, deadline}}
              ├─7. select {
              │      case r := <-waiter: ...
              │      case <-ctx.Done(): err=ErrContextCanceled
              │      case <-time.After(deadline): err=ErrDispatchTimeout
              │      case <-conn.closed: err=ErrStreamClosed
              │    }
              ├─8. eventstore.Append(CommandAck{
              │         success:       r.ok,
              │         error_message: r.error_message,
              │         causation_id:  <CommandIssued.position>,
              │    })
              ├─9. delete(conn.pending, command_id)
              └─10. return r to caller (or (nil, err) on a non-protocol failure)
```

**Invariant INV-1:** for every `CommandIssued` appended in step 4, a matching `CommandAck` is appended in step 8. No exceptions. If the driver dies, step 8 appends `CommandAck{ok=false, error_message="<cause>"}`. If the daemon itself crashes between steps 4 and 8, a bounded startup-time reconciler (deferred to C13's surgical-repair work) closes the gap; for C2, the invariant is enforced only while `carport.Host` is alive and is surfaced as a documented post-crash repair case.

### 7.2 Driver-originated events

Non-response messages on `DriverToHost` (`StateChanged`, `EntityRegistered`, `EntityUnregistered`, `DriverEvent`) are ingested by the host's `Run`-stream reader goroutine. For each message:

1. Validate (`entity_id` non-empty where required; capability on `EntityRegistered.capabilities` is known or opaque-accepted for driver-typed classes).
2. Build an `eventstore.Event` with:
   - `source = "driver:<instance_id>"`
   - `entity = msg.entity_id` (or `""` for instance-scoped `DriverEvent`)
   - payload = the corresponding `gohome.event.v1.Payload` variant
3. `eventstore.Append(event)`. No batching in C2 — keeps ingest latency predictable; batching is a perf tune-up for later.
4. On append failure: log `ERROR`, emit `DriverEvent{kind:"ingest_failed"}` (best-effort; if *that* append also fails, log only), then **close the `Run` stream** so the state machine transitions to `failed`. Preserves the invariant "events the driver sent us either made it to the log, or we lost the driver" — no silent drops.

### 7.3 Heartbeat

A dedicated goroutine per running instance:

```
every 10s: send HostToDriver{ping}
            record send_ts
on   DriverToHost{pong}: record rtt
if time.Since(last pong) > 30s: count as health failure
```

Heartbeats count toward the same `health_failures_to_restart` budget as explicit `Health` RPC failures.

---

## 8. Error Model

### 8.1 `Dispatch` errors

Every `Dispatch` returns exactly one of:

| Return shape | Meaning | Event emitted |
|---|---|---|
| `(*CommandResult{ok:true}, nil)` | Driver reported success. | `CommandAck{success:true}` |
| `(*CommandResult{ok:false, code, msg}, nil)` | Driver reported a typed failure. | `CommandAck{success:false, error_message:msg}` |
| `(nil, ErrEntityUnknown)`      | Entity not in registry. | None (no `CommandIssued` appended). |
| `(nil, ErrInstanceNotRunning)` | Entity's owning instance not in `running` state. | None. |
| `(nil, ErrDispatchTimeout)`    | Deadline elapsed before `CommandResult`. | `CommandAck{success:false, error_message:"dispatch timeout"}` |
| `(nil, ErrStreamClosed)`       | Stream died before `CommandResult`. | `CommandAck{success:false, error_message:"driver stream closed"}` |
| `(nil, ErrContextCanceled)`    | Caller's context canceled. | `CommandAck{success:false, error_message:"context canceled"}` |

**Key rule:** the existence or non-existence of a `CommandIssued` event is the branch: either it was appended (→ `CommandAck` is mandatory) or it wasn't (→ caller gets a pre-flight error and no events are produced at all).

### 8.2 Invariants (spec-level, not just test-level)

1. **INV-1 — Issued ⇒ Acked.** Every appended `CommandIssued` has a corresponding `CommandAck` with matching correlation, within the lifetime of the daemon that appended it. (C13 closes this across daemon crashes.)
2. **INV-2 — No silent ingest drops.** If a driver sends an event and the host accepts the stream message (i.e. gRPC delivery succeeded), the event is appended to the log OR the driver is torn down.
3. **INV-3 — No out-of-order acks per command.** For a given `command_id`, there is at most one `CommandResult` and at most one `CommandAck`.
4. **INV-4 — Registry consistency on handshake.** After a handshake, every entity the driver claims to own appears in the registry with a consistent `driver_instance_id`.
5. **INV-5 — Restart budget is honest.** An instance that enters `quarantined` can only leave via explicit manual restart.

Each invariant has a named test in §11.

---

## 9. Observability

### 9.1 Structured logging (slog)

Every log line from the carport subsystem carries:

```
subsystem   = "carport"
instance_id = <slug>
driver_name = <from manifest, "" before handshake>
```

Critical log points:

| Event | Level | Additional fields |
|---|---|---|
| spawn | INFO | pid, binary, socket_path |
| handshake-start | DEBUG | protocol_version |
| handshake-ok | INFO | manifest.version, n_initial_entities |
| handshake-fail | WARN | reason |
| health-probe-fail | WARN | failures_in_window |
| crash-detected | WARN | exit_code, signal |
| backoff-scheduled | INFO | next_attempt_ts, backoff_ms, budget_used/budget_max |
| quarantined | ERROR | reason |
| dispatch-start | DEBUG | command_id, entity_id, capability |
| dispatch-ok | DEBUG | command_id, duration_ms |
| dispatch-error | WARN | command_id, err |
| ingest-failed | ERROR | msg_kind, err (INV-2 breach) |
| stream-closed | INFO | reason |
| shutdown-start / shutdown-done | INFO | grace_ms |

### 9.2 Prometheus metrics

Additions to the existing `obs.Metrics` registry:

| Metric | Type | Labels |
|---|---|---|
| `carport_driver_instances`                    | gauge     | `state` |
| `carport_handshakes_total`                    | counter   | `instance_id`, `result` |
| `carport_command_dispatch_total`              | counter   | `instance_id`, `result` |
| `carport_command_dispatch_duration_seconds`   | histogram | `instance_id`, `capability` |
| `carport_events_ingested_total`               | counter   | `instance_id`, `kind` |
| `carport_driver_restarts_total`               | counter   | `instance_id`, `reason` |
| `carport_health_probe_duration_seconds`       | histogram | `instance_id` |
| `carport_stream_messages_received_total`      | counter   | `instance_id`, `kind` |
| `carport_pending_commands`                    | gauge     | `instance_id` |

`result` label values for dispatch: `ok`, `timeout`, `stream_closed`, `device_error`, `device_offline`, `bad_args`, `unsupported_capability`, `driver_shutting_down`, `entity_unknown`, `instance_not_running`, `context_canceled`, `internal`.

### 9.3 Tracing (OTel stub)

`Dispatch` opens a span `carport.Dispatch` with attrs `instance_id`, `entity_id`, `capability`, `command_id`. Span events: `CommandIssued appended`, `sent on stream`, `CommandResult received`, `CommandAck appended`. OTLP exporter remains stubbed from C1; full wiring in C13.

---

## 10. CLI Additions

### 10.1 Read-only (direct SQLite, no running daemon required)

- `gohome driver list` — reads `driver_instances` registry projection; prints table.
- `gohome driver status <instance>` — reads registry + recent `DriverEvent`s; prints a detail block (state, last-handshake, last-N-restarts, pending-commands, last-heartbeat-rtt).

### 10.2 Mutative (UNIX-socket RPC to `gohomed`, same transport as C1's `snapshot create`)

- `gohome driver restart <instance>` — forces the state machine to `quarantined → spawning` (resets restart budget).
- `gohome command send <entity> <capability> [--arg k=v]...` — test/debug escape hatch for `Host.Dispatch`. Subject to the same error surface as §8.1.

### 10.3 UNIX-socket protocol additions

Extend C1's socket message envelope with two new request kinds:

```go
type RequestKind string
const (
    RequestSnapshotCreate RequestKind = "snapshot_create"     // existing (C1)
    RequestDriverRestart  RequestKind = "driver_restart"      // NEW
    RequestCommandSend    RequestKind = "command_send"        // NEW
)
```

Encoded as length-prefixed JSON on the UDS, consistent with C1's style.

---

## 11. Testing Strategy

### 11.1 Unit tests (in-process, `fakedriver.Double`)

Coverage target: ≥85% line coverage in `internal/carport/*`.

Key tests:

| Test | Covers |
|---|---|
| `TestDispatch_HappyPath` | full Dispatch lifecycle, CommandIssued + CommandAck appended |
| `TestDispatch_Timeout` | context deadline, `ErrDispatchTimeout`, CommandAck{success:false} |
| `TestDispatch_CallerCancel` | ctx.Done, `ErrContextCanceled`, CommandAck appended |
| `TestDispatch_StreamClosedMidFlight` | driver stream dies, `ErrStreamClosed`, CommandAck appended |
| `TestDispatch_EntityUnknown` | no event appended, pure pre-flight error |
| `TestDispatch_InstanceNotRunning` | ditto |
| `TestDispatch_DriverErrorCodeSurfaces` | `ok=false` with every `CarportErrorCode` variant |
| `TestRouting_ResolveHappy` | entity → instance from registry |
| `TestRouting_ResolveStale` | entity with `last_seen_ts` older than TTL |
| `TestStream_EnvelopeCodecRoundTrip` | HostToDriver / DriverToHost proto round-trip |
| `TestStream_PendingMapLeak` | every add has a matching delete under random interleavings |
| `TestConfig_TOMLParseAndValidate` | all drivers.toml validation rules |

### 11.2 Property tests (`testing/quick`)

- **`TestProp_CommandResultCorrelation`** — randomize N in-flight commands, deliver CommandResults out of order, assert every caller receives its matching result and pending map is empty.
- **`TestProp_StateTransitionsLegal`** — randomize fail/recover sequences; every emitted `DriverEvent.kind` corresponds to a legal transition; no impossible transitions are ever logged.
- **`TestProp_RestartBudget`** — randomized fail/recover timings; invariant `observed_restarts_in_window ≤ restart_budget_max` always holds.

### 11.3 Golden replay (extends C1 `LoadFixture` / `AssertGolden`)

Three fixtures in `testdata/carport/`:

- `happy-path.events` — spawn → handshake → 10 StateChanged + 3 commands → clean shutdown. Goldens: `state.json`, `registry.json`.
- `crash-recovery.events` — running → driver SIGSEGV → restart → re-handshake → resume. Goldens match pre-crash state exactly.
- `quarantine.events` — 11 crashes inside 10 minutes → `quarantined` DriverEvent. Golden: registry shows the instance's state as quarantined; no further events from it.

### 11.4 Integration tests (real subprocess, `cmd/testdriver`)

Scenarios (each a `//go:build integration` test in `internal/carport/host/integration_test.go`):

| `TESTDRIVER_MODE` | Asserts |
|---|---|
| `normal` | handshake ok, 10 state changes ingested, 3 Dispatch round-trips succeed, clean shutdown |
| `crash_after_handshake` | state goes running→failed→backoff→running after restart |
| `crash_mid_stream` | running → ECONNRESET → backoff → running; no INV-1 breach |
| `hang_on_command` | `ErrDispatchTimeout` returned; CommandAck{ok:false} appended |
| `hang_on_shutdown` | SIGTERM → Shutdown RPC → grace timeout → SIGKILL; exit code recorded |
| `bad_protocol_version` | handshake fails; instance enters `failed`, backs off |
| `bad_secret` | handshake fails with explicit mismatch log; no retry bypass |
| `slow_handshake` | exceeds handshake deadline; transitions to `failed` |
| `chatty` | emits 1000 StateChanged rapidly; all ingested; metrics counter matches |
| `repeat_register` | after restart, re-registers the same entity; registry has no duplicate |

Each test bootstraps a fresh in-process `gohomed` fixture (pattern from C1), points it at a one-line `drivers.toml`, runs to completion, asserts state + events + exit status.

### 11.5 Fuzz targets

- **`FuzzEnvelopeDecode`** — randomized bytes into `HostToDriver` and `DriverToHost` proto decode; must never panic; decode-error cases are benign.

CI runs a 60s smoke-fuzz per PR (matches C1's `FuzzEventDecode` setup).

### 11.6 Coverage gate

CI enforces ≥85% line coverage in `internal/carport/*` as a hard gate, via the same `go tool cover -func` script used in C1. Below that, the workflow fails.

---

## 12. Proto Hygiene Rules

A house convention applied to **all** protobuf files in this repo, including a retrofit of the C1-vintage protos.

### 12.1 Rules

1. **Grouped numbering.** Within a `oneof` or message, group fields by semantic category: identity (1-9), domain payload (10-19), metadata/meta (20-29, 30-39, ...), reserved-for-extension (90-99).
2. **Range headers in every non-trivial oneof / message.** A one-line comment above each group: `// 10-19: state-plane events`. The header is the contract — next-additions pick the next free number *in that group's block*.
3. **`reserved` on removal, forever.** Any field number or oneof tag removed MUST be added to a `reserved X;` statement in the same message, and the old name appended to `reserved "old_name";`. Field numbers are never reused.
4. **Tag-stability boundary.** Field numbers and oneof tags are part of the wire contract. They do not change within a protocol version (`v1alpha1`); breaking changes move to a new package (`v1alpha2`).
5. **`v1alpha*` vs `v1`.** Packages suffixed `alpha*` may make wire-breaking changes across releases with a migration note. Graduating to `v1` is a one-way door and requires a decision record entry.

### 12.2 Retrofit scope (Task 1 of the plan)

Apply Rule 2 range headers to the three C1 protos without changing any field numbers:

- `proto/gohome/event/v1/event.proto` — `Payload.oneof kind` already uses grouped numbering informally; add the explicit headers.
- `proto/gohome/event/v1/snapshot.proto` — add headers where messages have ≥3 fields.
- `proto/gohome/entity/v1/attributes.proto` — `Attributes.oneof kind` ditto.

A top-level `docs/proto-hygiene.md` states the rules in one page and is referenced from each `.proto`'s top-of-file comment.

### 12.3 Enforcement

Lint the rules via `buf breaking` in CI (already present for wire-compat) plus a new `scripts/check-proto-hygiene.sh` that greps for `reserved` lines on every removed field via git history. Minimal, imperfect, sufficient.

---

## 13. Success Criteria

C2 is complete when:

1. Protobuf contract `gohome.carport.v1alpha1` is defined, generated, committed, and `buf lint` is green.
2. Proto-hygiene retrofit applied to C1 protos; `docs/proto-hygiene.md` shipped.
3. `internal/carport/host` implements the §6 state machine with all transitions exercised by tests.
4. `Host.Dispatch(ctx, entityID, capability, args)` returns correct results / errors across all of §8.1 and §11.4.
5. `drivers.toml` parsing and validation passes every case in §5.2; `gohomed` auto-spawns declared instances on startup.
6. `gohome driver list / status / restart` and `gohome command send` work against a running daemon.
7. `cmd/testdriver` binary ships with every scenario in §11.4.
8. All golden-replay fixtures in §11.3 pass; `--update` is idempotent.
9. `kill -9 gohomed` mid-dispatch leaves the eventstore consistent on next start; any orphan `CommandIssued` without a matching `CommandAck` is bounded (documented as a C13 repair path) and the daemon comes up healthy.
10. Line coverage in `internal/carport/*` ≥ 85%.
11. CI (`lint`, `test`, `test:integration`, `test:race`, `fuzz-smoke`) is green on `linux/{amd64,arm64}` and `darwin/arm64`.
12. Git tag `c2-complete` applied.

---

## 14. Explicit Deferrals

Each deferred item names what's missing and what protocol hook (if any) C2 leaves behind so the later doc doesn't have to break wire compat.

| Deferred | To | Protocol hook |
|---|---|---|
| Remote / edge TLS transport | C12 | `Run` stream is transport-agnostic; adding a TLS listener is additive. |
| mTLS pairing ceremony | C12 | Manifest's `protocol_version` string lets us bump to `v1alpha2` with pairing fields without breaking C2 drivers. |
| Pkl-typed driver manifests | C4 | `DriverManifest.pkl_module` (field 99) is reserved empty in C2; C4 populates it and layers type-checking above. |
| Connect-RPC `EntityService.CallCapability` | C7 | `Host.Dispatch` is the Go API; the RPC handler is a thin wrapper. |
| Automation-engine retry policy | C6 | `Dispatch` fails fast; retry is policy, not protocol. |
| Signed manifest verification | C13 | `DriverManifest` is bytes; signature is an additive field. |
| Surgical repair of orphan `CommandIssued` | C13 | Events carry enough correlation metadata; repair is read-only. |
| WASM driver tier | v1.x | WASM shim will speak the same gRPC against an in-process host-function bridge. |
| Reload of `drivers.toml` without restart | C4 (Pkl reload) | N/A — the file is throwaway. |

---

## 15. Decision Record

Every architectural decision made during the brainstorming session, preserved for future readers. Format follows the master doc's Decision Record.

| # | Decision | Alternatives considered | Reason |
|---|---|---|---|
| DR-1 | **Minimum viable Carport scope.** Implement: protobuf contract, host-side supervisor, local UDS transport, lifecycle state machine, fake drivers. Specify-but-defer: mTLS, edge transport, Pkl manifests, WASM. | (B) add edge/TLS + pairing in C2; (C) haul all of C3/C4/C12 forward. | The roadmap deliberately separates edge (C12) and Pkl (C4). Building the TLS path without a consumer yields write-only code. Locking the wire in the *spec* is enough to keep C12 honest; implementation stays focused on what we can exercise today. |
| DR-2 | **Driver instances declared in `drivers.toml`.** Placeholder format; C4's Pkl loader supersedes it wholesale. | (A) flags/env only; (C) SQLite control table + `gohome driver add`; (D) hybrid. | TOML is the cheapest throwaway. Flags make repeatable deployment miserable; a SQLite control table creates migration drag we'd regret. When Pkl arrives in C4 the loader collapses without touching the event log. |
| DR-3 | **One subprocess per driver instance.** Drop `RegisterInstance` RPC; `Handshake` carries the instance config directly. | (B) one process per driver binary with multi-instance via `RegisterInstance` (master doc's literal sketch). | Driver instances almost never share resources worth pooling. Crash isolation is free with per-instance processes. Driver authors write single-tenant binaries (vastly simpler). Extra RSS cost is negligible. If a future driver truly needs multi-instance, `HandshakeResponse.manifest` can advertise `multi_instance_capable` and we add `RegisterInstance` back additively. |
| DR-4 | **Call-through `Dispatch` API; events are the audit trail, not the trigger.** `Host.Dispatch(ctx, ...)` blocks for the result; events are appended as a side effect. | (A) pure event-driven: anyone appends `CommandIssued`, host tails events to dispatch. (C) hybrid with reconciler. | Matches the master doc's described control flow ("emits a CommandIssued event. The owning driver receives the command"). Callers (API, automations) want synchronous semantics. Event log gives us lossless history without having to *drive* the command path. A reconciler can be layered in C13 if a failure mode demands it. |
| DR-5 | **Both in-process fake driver AND `cmd/testdriver` real binary for tests.** | (A) in-process only; (B) binary only. | Disjoint coverage: in-process is fast and precise for protocol/routing/envelope logic; the binary exercises spawn/signal/crash/restart/UDS-cleanup paths that an in-process double can't. Proven pattern from C1's in-memory DB + kill-9 helper. |
| DR-6 | **Grouped numbering + range-comment headers for all protos.** `// 10-19: state-plane events` style; retrofit to C1 protos. `// NEXT:` comments rejected. | (A) `// NEXT: N`; (B) strict linear numbering; (C) no convention. | Grouped numbering is already in C1's `event.proto` informally. Range headers document the grouping; new additions know which block to pick a tag from. `// NEXT:` implies linearity we explicitly don't have. |
| DR-7 | **`reserved` every removed field, forever.** Rule documented in `docs/proto-hygiene.md`. | (A) trust author discipline; (B) checker-only. | Field-number reuse is the one proto landmine that silently corrupts the wire. A hard rule plus a CI lint is cheap insurance. |
| DR-8 | **Invariant INV-1 (every `CommandIssued` gets a `CommandAck`) enforced at the host level in C2; cross-crash reconciliation deferred to C13.** | (A) full reconciler now; (B) no invariant. | Within a running daemon INV-1 is enforceable with a single `defer AppendAck(...)` pattern. Across crashes it requires a startup scanner and is a natural fit for C13's surgical-repair CLI work. Partial now, total later. |
| DR-9 | **Per-instance handshake secret via env var.** Random 32 bytes generated per spawn, sent in `HandshakeRequest.handshake_secret`. | (A) Unix-socket perm-bits only; (B) PID-based check; (C) none (trust the OS). | Defends against a rogue process binding to the socket path in the window between unlink and bind. Cheap, real. Perm-bits alone are fine on single-user boxes but fail open on shared hosts. |
| DR-10 | **Host closes the `Run` stream on ingest-append failure** rather than silently dropping events. | (A) log-and-drop; (B) block the driver. | Preserves INV-2. Dropping silently corrupts audit; blocking the driver would cascade into timeouts on every concurrent command. Forced reconnect is the surgical failure mode. |
| DR-11 | **`v1alpha1` as the initial protocol version; `v1` is a one-way door.** | (A) ship `v1` immediately; (B) go straight to `v1` after C3. | We haven't yet exercised the wire with a real non-fake driver. `v1alpha*` gives us permission to break things during C3-C6 if reality teaches us something. Graduation to `v1` is its own decision, documented when it happens. |

---

## 16. Task Breakdown

High-level tasks, in dependency order. The detailed implementation plan (produced by `writing-plans` after this spec is approved) will decompose these into bite-sized TDD steps.

1. **Proto hygiene retrofit** — range headers on C1 protos; write `docs/proto-hygiene.md`; wire a `scripts/check-proto-hygiene.sh`.
2. **Carport proto definitions** — author the four `.proto` files; generate; commit `gen/`.
3. **`internal/carport/config`** — TOML loader + validation.
4. **`internal/carport/routing`** — entity→instance resolver; handles stale/missing.
5. **`internal/carport/stream`** — envelope codec; pending-waiter map; property tests.
6. **`internal/carport/fakedriver`** — in-process `Driver` server; scenario hooks.
7. **`internal/carport/host` skeleton** — lifecycle FSM; spawn/handshake; state transitions as `DriverEvent`s.
8. **Dispatch** — append/send/wait/ack; all §8.1 error paths.
9. **Ingest loop** — driver-originated events ingested to eventstore; INV-2 enforcement.
10. **Health & heartbeat** — probe goroutine; failure budget; restart trigger.
11. **Backoff & quarantine** — restart budget; quarantine transition.
12. **Shutdown choreography** — SIGTERM → Shutdown RPC → grace → SIGKILL.
13. **`cmd/testdriver`** — scenario-driven binary.
14. **Integration tests** — §11.4 matrix.
15. **Golden replay fixtures** — §11.3 three fixtures; extend C1 `LoadFixture`.
16. **CLI additions** — `gohome driver list/status/restart`, `gohome command send`; UDS request kinds.
17. **`gohomed` wiring** — add `carport.Host` to supervisord startup/shutdown.
18. **Observability** — metrics, logging fields, tracing spans.
19. **Fuzz target** — `FuzzEnvelopeDecode`.
20. **Coverage gate + CI matrix** — enforce ≥85%; add carport scenarios to `test:integration`.
21. **End-to-end smoke + tag** — manual acceptance walkthrough; apply `c2-complete`.

---

*End of C2 design document.*
