# Architecture Internals

This page is a deep reference for contributors working on the gohome daemon. It explains module boundaries, concurrency invariants, and the two most complex subsystems (the Carport FSM and config diff-based reload). Each section points to the relevant child design spec for further reading.

---

## Module boundaries

The daemon is structured around a small set of internal packages with strict ownership of their domains. The `internal/daemon` package wires them together; no package imports another package's internals.

| Package | Responsibility |
|---------|---------------|
| `internal/eventstore` | SQLite-backed append-only event log. Single writer. All state flows through here. Exposes `Append`, `AppendBatch`, subscription, snapshot, and projector dispatch. |
| `internal/carport` | Carport protocol finite state machine. Manages driver process lifecycle: spawn, handshake, run, stop, restart, backoff, quarantine. |
| `internal/config` | Pkl config loading and diff engine. Evaluates `main.pkl`, validates the result, and produces a `ConfigDelta` for reload processing. |
| `internal/automation` | Automation engine. One Starlark VM per trigger; sandboxed execution with gohome built-ins. |
| `internal/rpc` | Connect-RPC service layer. Thin adapter over the internal domain — translates protobuf requests/responses to internal types and delegates to the appropriate package. |
| `internal/mcp` | MCP server. Calls into the RPC layer; does not talk to eventstore or registry directly. |
| `internal/web` | Web UI server. Serves a statically embedded frontend and proxies API calls to the RPC layer. |
| `internal/daemon` | Top-level coordinator. Constructs all subsystems, wires channels, manages startup and shutdown ordering. |
| `internal/registry` | Registry projector. Device/entity/driver-instance query API. Kept consistent by eventstore projectors. |
| `internal/state` | Copy-on-write state cache (HAMT-based). Fast concurrent reads, single-writer updates. |
| `internal/storage` | SQLite open, PRAGMAs, `storage.Tx` abstraction, and goose migrations. Used by eventstore and registry. |
| `internal/observability` | `slog` setup, Prometheus registry, metric helpers, OpenTelemetry tracing stubs. |

---

## Single-writer eventstore discipline

The event store is the source of truth for all state in the system. To maintain consistency, only one goroutine ever appends events:

- The eventstore writer goroutine is the sole caller of `db.Exec("INSERT ...")` on the events table.
- All other code submits events via a submit channel and receives a confirmation when the append completes.
- Concurrent state reads (via registry queries or state cache lookups) are always safe — they do not hold write locks.

This means: **never call `Append` or `AppendBatch` directly from a goroutine other than the writer**. Submit through the channel.

---

## Concurrency invariants

Three named invariants govern how the daemon handles concurrency. Code changes must not violate them.

### 1. Event append is single-writer

Only the eventstore goroutine calls `db.Exec("INSERT ...")` on the events table. All callers submit events through the submit channel and block until they receive a confirmation. This eliminates the need for write locks on the event log and makes serialisation straightforward.

### 2. Driver FSM is goroutine-per-driver

Each driver instance has exactly one dedicated goroutine managing its lifecycle. State transitions for a given driver always happen in that goroutine. There are no cross-driver locks: driver A's goroutine cannot block on driver B's state. This means a hanging or slow driver cannot stall other drivers.

### 3. Config reload is copy-on-write

The active config is held behind an atomic pointer. When a reload is processed, the new config is built in full and the pointer is swapped atomically. In-flight requests that read the old config pointer continue to use the old config until they complete. No request ever sees a partially-applied config.

---

## The Carport FSM

Each driver instance moves through the following states:

```
declared → spawning → awaiting_handshake → running → stopping → stopped
                                                            ↓
                                                         failed → backoff → quarantined
```

State descriptions:

| State | Meaning |
|-------|---------|
| `declared` | Driver instance exists in config; not yet started. |
| `spawning` | `gohomed` is launching the driver binary as a subprocess. |
| `awaiting_handshake` | Process is running; waiting for the Carport `Handshake` RPC to complete. |
| `running` | Handshake succeeded; the `Run` bidirectional stream is active. |
| `stopping` | Graceful shutdown requested; waiting for the driver to exit. |
| `stopped` | Driver exited cleanly. |
| `failed` | Driver exited with an error or the process crashed. |
| `backoff` | Waiting before a restart attempt (exponential backoff). |
| `quarantined` | Driver has failed too many times in too short a window; restarts suspended. |

Each transition appends an event to the event log. The registry projector consumes these events to maintain the visible driver-instance status that `gohome status` and the web UI display.

Design spec: [C2 — Carport Protocol Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-21-c2-carport-protocol-design.md)

---

## Config diff-based reload

Config reload does not restart the world. The config loader:

1. Evaluates the new `main.pkl` against the Pkl schema.
2. Compares the new config against the currently active config.
3. Produces a `ConfigDelta` — a structured diff describing which driver instances were added, removed, or changed.

Delta processing order is fixed:

1. **Removes** — stop and clean up driver instances that no longer appear in config.
2. **Updates** — restart driver instances whose config changed (e.g. a new environment variable or config key).
3. **Adds** — start new driver instances that appear in config for the first time.

This ordering ensures no two instances of the same driver are ever running simultaneously during a reload. Driver instances not referenced in the delta are not touched — a reload that adds one new driver does not restart any existing drivers.

Design spec: [C4 — Pkl Config Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-22-c4-pkl-config-design.md)

---

## Design specs

Each subsystem has a corresponding child design spec with full detail on the protocol, data structures, and design decisions:

| Subsystem | Spec |
|-----------|------|
| Event store | [C1 — Event Core and Storage Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-21-c1-event-core-and-storage-design.md) |
| Carport protocol | [C2 — Carport Protocol Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-21-c2-carport-protocol-design.md) |
| Driver SDK | [C3 — Driver SDK Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-22-c3-driver-sdk-design.md) |
| Pkl config | [C4 — Pkl Config Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-22-c4-pkl-config-design.md) |
| Starlark runtime | [C5 — Starlark Runtime Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-22-c5-starlark-runtime-design.md) |
| Automation engine | [C6 — Automation Engine Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-23-c6-automation-engine-design.md) |
| Connect-RPC API | [C7 — Connect-RPC API Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-23-c7-connect-rpc-api-design.md) |
| MCP server | [C8 — MCP Server Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-24-c8-mcp-server-design.md) |
| Auth & Policy | [C9 — Auth and Policy Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-25-c9-auth-and-policy-design.md) |
| Web UI | [C10 — Web UI Architecture Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-26-c10-web-ui-architecture-design.md) |
| HA Import Tool | [C11 — HA Import Tool Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-26-c11-ha-import-tool-design.md) |
| Edge Agent | [C12 — Edge Agent Design](https://github.com/fynn-labs/gohome-docs/blob/main/superpowers/specs/2026-04-27-c12-edge-agent-design.md) |
