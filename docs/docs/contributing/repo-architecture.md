# Repository Architecture

This page is the contributor map for Switchyard's source tree. It complements
the user-facing architecture overview and the deeper internals notes by
answering a narrower question: where does code belong, and which packages own
which contracts?

## Top-level layout

| Path | Owner | Notes |
|------|-------|-------|
| `app/` | Admin UI | Vue application, generated API clients, tests, and static assets that are embedded into `switchyardd`. |
| `cmd/switchyard/` | CLI | Thin command-line adapter over daemon APIs. Domain behavior should not live here. |
| `cmd/switchyardd/` | Daemon entrypoint | Startup, shutdown, config flags, and top-level wiring only. |
| `cmd/testdriver/` | Test binary | Scenario driver used by integration tests. |
| `docs/` | Documentation | Zensical site, design history, ADRs, and agent-authored specs/plans. |
| `drivers/` | First-party drivers | Out-of-process Carport drivers shipped with this repo. |
| `examples/` | Example config | Sample Pkl configs and fragments. Runtime config does not read this directory by default. |
| `gen/` | Generated Go | Committed protobuf output. Edit `proto/`, then run `task proto`. |
| `internal/` | Daemon internals | Main domain packages and API adapters. External modules must not import these. |
| `proto/` | Wire contracts | Protobuf API and event schema definitions. Preserve field numbers and reserve removed fields. |
| `switchyard-driverkit/` | Public Go SDK | Separate Go module for driver authors. Keep dependencies and APIs suitable for third-party drivers. |
| `testdata/` | Fixtures | Golden CLI output, integration fixtures, and other stable test inputs. |

## Runtime shape

Switchyard is built around an event-sourced daemon:

1. `internal/config` evaluates Pkl config into a protobuf snapshot.
2. `internal/daemon` wires that snapshot into the event store, registry, state
   cache, automation engine, API listener, MCP server, and web server.
3. `internal/eventstore` serializes durable events into SQLite.
4. `internal/registry` and `internal/state` project those events into query
   models.
5. `internal/carport` supervises out-of-process drivers and translates driver
   messages into events.
6. `internal/api` adapts internal interfaces to Connect-RPC services.
7. `internal/mcp` calls the same daemon/API seams for agent workflows.
8. `internal/web` serves the embedded UI bundle; API traffic goes through the
   Connect listener.

The event log is the durability boundary. Registry rows, state cache entries,
activity stories, and UI views are derived state.

## Internal package ownership

| Package | Responsibility | Should not own |
|---------|----------------|----------------|
| `internal/activity` | Activity feed summaries and related command-catalog verbs. | Event storage or registry projection rules. |
| `internal/api` | Connect-RPC adapters, pagination, streaming helpers, and error mapping. | Business logic that should be shared with CLI/MCP. |
| `internal/api/listener` | HTTP, h2c, Unix socket, interceptors, peer credentials, and route mounting. | Service implementation logic. |
| `internal/auth` | Transport-neutral auth contracts and simple auth composition. | Credential persistence details. |
| `internal/auth/credentials` | Password hashes, API tokens, passkeys, and enrollment tokens. | Request middleware or policy decisions. |
| `internal/auth/identity` | Configured user and role projection. | Password/passkey/token verification. |
| `internal/auth/sessions` | Cookie sessions and refresh-token rotation. | WebAuthn or API token storage. |
| `internal/auth/throttle` | Failed-auth throttling buckets. | Login flow orchestration. |
| `internal/automation` | Automation runtime, triggers, condition/action compilation, and scene invocation support. | Config parsing or API response shaping. |
| `internal/carport` | Driver process lifecycle, Carport gRPC streams, command dispatch, and event ingest. | Driver-specific protocol details. |
| `internal/config` | Pkl evaluation, validation, diffs, config apply events, and live snapshot ownership. | Driver supervision or API formatting. |
| `internal/daemon` | Dependency construction and lifecycle orchestration. | Domain rules that can live in a subsystem. |
| `internal/display` | Ambient display pairing, display registry, and recommendations. | Web route rendering. |
| `internal/driver/management` | Driver settings/read-model API for the UI. | Carport process control internals. |
| `internal/editsession` | Pkl edit-session locks and conflict handling. | Pkl language-server features. |
| `internal/eventstore` | Append-only event log, projectors, subscriptions, snapshots, and replay cursors. | Domain-specific read models. |
| `internal/mcp` | MCP transport, resources, tools, and audit hooks. | Direct database access. |
| `internal/observability` | Logging, metrics, tracing shims, and recovery HTTP endpoints. | Subsystem health decisions. |
| `internal/page` | Custom page domain types, service, catalog, scaffolding, and layout storage seams. | UI rendering. |
| `internal/pkllsp` | Pkl language-server subprocess lifecycle and request translation. | Config validation. |
| `internal/policy` | Compiled authorization policy and evaluation. | Authentication. |
| `internal/registry` | SQL-backed projection of drivers, devices, entities, and subscriptions. | Event append ownership. |
| `internal/replay` | Historical state and causation-chain queries. | Live state cache mutation. |
| `internal/script` | Named Starlark script runtime and test execution. | Automation scheduling. |
| `internal/starlark` | Sandboxed Starlark VM, builtins, module loading, and limits. | Script catalog or config discovery. |
| `internal/state` | Copy-on-write live entity state cache. | Durable storage. |
| `internal/storage` | SQLite open, PRAGMAs, lockfile, and migrations. | Event schema semantics. |
| `internal/web` | Embedded SPA and widget asset serving. | Connect-RPC API handling. |
| `internal/widgetpack` | Widget-pack install and metadata pipeline. | Page layout rendering. |

## Dependency direction

Keep dependencies pointing inward toward stable domain seams:

- `cmd/*` imports `internal/cli` or top-level wiring, not subsystem internals
  opportunistically.
- `internal/api` depends on small interfaces from `deps.go`; daemon adapters
  satisfy those interfaces.
- `internal/mcp` should call daemon/API seams rather than reaching into
  eventstore, registry, or config storage.
- Driver implementations under `drivers/` use `switchyard-driverkit`; they
  should not import daemon internals.
- `switchyard-driverkit/` can import generated protocol types, but not
  `internal/`.

When a package starts importing too many sibling internals, add a small
interface at the call site or move the behavior to the package that owns the
data invariant.

## Comment and doc policy

Every Go package should have a package comment. Use `doc.go` when the package
needs more than one sentence or when the first source file would otherwise
start with implementation details.

Exported comments should describe the contract, not restate the identifier.
Good comments answer at least one of:

- what invariant the type or function owns
- what caller is responsible for
- what error or concurrency behavior callers can rely on
- why the symbol is exported inside `internal/`

Avoid comments that narrate obvious control flow. Inline comments are for
surprising constraints, ordering requirements, concurrency fences, security
boundaries, or dependency quirks.

## Generated and derived files

- Protobuf schemas live in `proto/`; generated Go lives in `gen/`.
- `gen/` is committed. Do not hand-edit it.
- The web bundle under `dist/` and `internal/web/dist` is derived by app build
  tasks and should not be treated as source.
- Pkl-generated layout fragments belong in config or examples, not root-level
  directories.

## Adding new code

Before adding a top-level directory, update `AGENTS.md` and this page. Before
adding a new internal package, check whether an existing package already owns
the invariant. New packages should make ownership sharper; they should not be a
holding area for miscellaneous helpers.
