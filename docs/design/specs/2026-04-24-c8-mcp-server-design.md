# C8 — MCP Server Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Date:** 2026-04-24
**Status:** Draft
**Depends on:** C1 (Event Core), C4 (Pkl Config), C5 (Starlark Runtime), C6 (Automation & Script Engine), C7 (Connect-RPC API)
**Unblocks:** C9 (Auth & Policy) — see §5.2 for the cross-spec dependency this introduces.

---

## 1. Scope

C8 delivers a Model Context Protocol server for `gohomed`, enabling AI agents (Claude Desktop, Claude Code, Cursor, and any other MCP-compatible client) to inspect entity state, invoke capabilities, query and follow events, run and edit scripts and config, and evaluate ad-hoc Starlark — out of the box, with no additional plumbing on the agent's side.

The server ships as a `gohome mcp serve` CLI subcommand using stdio transport. It runs as a subprocess launched by the MCP client, dials the local `gohomed` daemon over the C7 Unix-domain socket, and inherits the `system:local` peer-cred principal from C7's auth seam. There is no token, no policy, and no remote transport in v1 — those land in C9.

### 1.1 In scope

- **Stdio MCP server** delivered as `gohome mcp serve`. Launched by an MCP client per its config (e.g. `claude mcp add gohome -- gohome mcp serve`). Speaks MCP JSON-RPC on stdin/stdout via the official `modelcontextprotocol/go-sdk`.
- **12 tools** matching master §7.2 (the master design's bullet groups `read_config_file`/`write_config_file` together; counted separately they are 12):
  - `gohome__get_state`, `gohome__list_entities`, `gohome__call_capability` — entity reads + invocation.
  - `gohome__query_events`, `gohome__tail_events` — event history + buffered windowed tail.
  - `gohome__apply_scene` — registered, returns `UNIMPLEMENTED` until the Scene spec ships.
  - `gohome__run_script`, `gohome__eval_starlark` — script invocation and ad-hoc evaluation.
  - `gohome__validate_config`, `gohome__apply_config` — Pkl bundle validation and apply.
  - `gohome__read_config_file`, `gohome__write_config_file` — targeted file-level config edits with strict path containment.
- **2 MCP resources with subscription support**:
  - `gohome://entities/{id}` and `gohome://entities?selector=...` — live entity state.
  - `gohome://automations/{id}/runs/{run_id}/trace` — per-run automation trace.
- **Hand-written tool input schemas** (json-schema-tagged Go structs, agent-targeted), protojson outputs.
- **Two new audit events** in `gohome.event.v1`: `MCPEvalRequested` (tag 61) and `ConfigFileEdited` (tag 62), in C7's "external ingress" 60-69 block.
- **`x-gohome-source` Connect header** propagating the source (`mcp` / `cli` / `web`) end-to-end, picked up by C7's interceptor stack for metrics, slog, and per-handler branching.
- **`KindMCPEval` Starlark context enforcement** for `eval_starlark`: 30s wall-clock + 10M steps + read-only stdlib (already in C5), plus a 64 KiB result cap and an `MCPEvalRequested` audit event added in C8.
- **Action catalog stub** — every tool and resource maps to an `auth.Action` table; the C7 `AllowAllAuthorizer` stub continues to allow everything, but the seam is wired so C9 swaps the Authorizer without touching `internal/mcp` code.
- **Two daemon-side RPCs** added on `SystemService`: `GetConfigDir` (so the subprocess locates the config dir without re-evaluating Pkl) and `RecordConfigFileEdit` (so the daemon — not the subprocess — emits the audit event from a single source of truth).
- **Daemon-side metrics** under `gohome_mcp_*`, registered in `internal/observability/metrics.go`.
- **`gohome mcp tools`** debug subcommand: prints the tool catalog the server would expose and exits.

### 1.2 Explicit non-goals

- **HTTP transport.** The MCP-over-HTTP variant lands in C9 alongside real auth tokens. The same `internal/mcp` package is reused; only the transport changes.
- **API tokens, per-user attribution, per-tool policy enforcement.** C9 owns these. The action catalog and `Authorize` calls land here as a stub.
- **Scene tool implementation.** `gohome__apply_scene` ships as a registered tool that always returns the underlying `UNIMPLEMENTED` Connect error. The Scene spec is responsible for making it work.
- **Server-initiated sampling and prompts.** MCP supports both; v1 uses neither. Tools and resources only.
- **Resource templates beyond entities and traces.** Other live data surfaces (driver health, automation enable state, current config snapshot) stay tool-shaped for v1.
- **Multi-instance fan-in.** A single MCP client subprocess speaks to a single daemon. Aggregating across multiple gohomed instances is C12's territory (edge agent).
- **High-level / "workflow" tools.** No `create_automation_from_description` or similar. v1 ships the atomic primitives the master design names; agents compose them.
- **Subprocess-side metrics scrape.** Subprocess-per-session has no Prometheus target; metrics live entirely on the daemon side (driven by the `x-gohome-source: mcp` header).

---

## 2. Background

Master design §7.2 names MCP as a first-class API surface — "an MCP server is a first-class API surface, not a bolted-on integration." The named tool inventory and the agent-safety guardrails on `eval_starlark` are explicit. C8 takes the in-place wire surface (C7's Connect API), the in-place Starlark context (C5's `KindMCPEval`), and the in-place auth seam (C7's `Principal` / `Authenticator` / `Authorizer`), and assembles them into a stdio-transport MCP server that the major MCP clients can launch directly.

The master roadmap places C8 before C9 (auth & policy). C7's auth seam ships a `LocalPeerCredAuthenticator` that grants `system:local` on UDS and a `RejectAllAuthenticator` on TCP; until C9 ships real tokens and policy, the only sensible MCP transport is stdio invoked locally — which is exactly the most-used MCP integration pattern (Claude Desktop launching a local subprocess). C8 ships that subset cleanly and hands C9 the cross-spec requirements needed to close the loop (see §5.2).

C5 already created `KindMCPEval`. C7 already added `ScriptService.Eval`. C8 enforces the agent-specific guardrails on top of those foundations rather than duplicating them.

---

## 3. Architecture Overview

### 3.1 Process model

```
┌────────────────────┐                 ┌──────────────────────┐                ┌────────────────┐
│  MCP client        │   MCP JSON-RPC  │  gohome mcp serve    │  Connect-RPC   │   gohomed      │
│  (Claude Desktop,  │ ◄─── stdio ───► │  (CLI subprocess)    │ ◄── over UDS ──►   (daemon)     │
│   Claude Code,     │                 │  internal/mcp/server │                │  internal/api  │
│   Cursor, ...)     │                 └──────────────────────┘                └────────────────┘
└────────────────────┘                            │
                                                  └── reads/writes config dir directly
                                                      (filesystem tools only, with path containment)
```

- The MCP client launches `gohome mcp serve` per its config. (Example for Claude Desktop: `{"mcpServers": {"gohome": {"command": "gohome", "args": ["mcp", "serve"]}}}`.)
- The subprocess speaks MCP JSON-RPC over stdin/stdout via the official `modelcontextprotocol/go-sdk` server.
- Tool and resource handlers translate to Connect-RPC calls and dial `gohomed` over the UDS configured in C7 (`@data/gohomed.sock` by default, overridable via `--endpoint` / `GOHOME_ENDPOINT`).
- All Connect calls inherit the `system:local` peer-cred principal because the subprocess runs as the local user and dials a local UDS — C7's `LocalPeerCredAuthenticator` does the rest.
- Filesystem tools are the one exception that does not round-trip through the daemon: they read and write the config dir directly with the subprocess's own UID, after path containment. The daemon still emits the `ConfigFileEdited` audit event via a one-line RPC the subprocess calls after a successful write (so audit comes from a single source of truth).
- The subprocess exits on stdin EOF (clean MCP client shutdown) or on UDS dial failure (writes one line to stderr, exits 1).

### 3.2 Package map

```
gohome/
├── cmd/gohome/
│   └── main.go                    # registers the `mcp` subcommand
├── internal/cli/
│   └── cmd_mcp.go                 # `gohome mcp serve`, `gohome mcp tools`
└── internal/mcp/
    ├── server.go                  # builds the SDK server, registers tools/resources, runs stdio loop
    ├── deps.go                    # interface set the tools and resources depend on
    ├── client.go                  # Connect-go client wired to UDS, sets x-gohome-source: mcp
    ├── errors.go                  # Connect error → MCP tool error mapping
    ├── actions.go                 # action catalog stub for C9 hand-off
    ├── tools/
    │   ├── tool.go                # Tool registration helpers, common marshaling helpers
    │   ├── entities.go            # get_state, list_entities, call_capability
    │   ├── events.go              # query_events, tail_events
    │   ├── scenes.go              # apply_scene (UNIMPLEMENTED passthrough)
    │   ├── scripts.go             # run_script, eval_starlark
    │   ├── config.go              # validate_config, apply_config
    │   ├── files.go               # read_config_file, write_config_file
    │   └── *_test.go
    ├── resources/
    │   ├── resource.go            # resource registration + per-session subscription map
    │   ├── entities.go            # gohome://entities/... resources + subscribe
    │   ├── traces.go              # gohome://automations/.../trace resources + subscribe
    │   └── *_test.go
    ├── audit/
    │   └── recorder.go            # facade for emitting daemon-side audit (calls SystemService)
    ├── fs/
    │   ├── safepath.go            # path containment + symlink resolution
    │   ├── syntax.go              # best-effort .pkl / .star parse helpers
    │   └── *_test.go
    └── server_test.go             # integration: spawns server, drives MCP client, dials fake daemon
```

`internal/mcp` depends on `internal/auth` (interfaces and stubs only — no transitive daemon imports) and on the C7-generated Connect clients. It does NOT import `internal/api`, `internal/eventstore`, or any other engine package — the only contact point with the daemon is the Connect client.

### 3.3 Daemon-side changes

C8's daemon-side footprint is intentionally tiny:

1. **`SystemService.GetConfigDir`** — new RPC returning `{config_dir: string}`. The subprocess calls this once on startup to learn where to root filesystem-tool paths. (Trivial — daemon already has the resolved path in `internal/config`.)
2. **`SystemService.RecordConfigFileEdit`** — new RPC the subprocess calls after a successful `write_config_file`. Daemon validates the path is inside the config dir (defense in depth — the subprocess already checked), then appends `ConfigFileEdited` to the event store with the principal from the call's auth context. Returns `{event_cursor: uint64}`.
3. **`gohome.event.v1.Payload` extensions** — two new payload tags in C7's "external ingress" 60-69 block:
   - `MCPEvalRequested = 61`
   - `ConfigFileEdited = 62`
4. **`ScriptService.Eval` handler enhancement** — when the request's `x-gohome-source` header is `mcp`, the handler:
   - Enforces the 64 KiB result cap (truncates with marker `...[truncated; result was N bytes]`).
   - Emits `MCPEvalRequested{principal_id, session_id, source, result_sha256_hex, truncated, result_bytes, duration_ms, error}` to the event store before returning. The session_id is read from a header `x-gohome-mcp-session` set by the subprocess.
5. **`x-gohome-source` interceptor** — the C7 server-side interceptor stack reads the header (default `cli` if absent), tags the request context, and labels the existing `gohome_api_*` metrics with `source`.
6. **`gohome_mcp_*` metrics** registered in `internal/observability/metrics.go`.

That is the entire daemon footprint. Nothing else in `gohomed` changes.

### 3.4 The `x-gohome-source` Connect header

A small but load-bearing piece of plumbing. Without it, MCP-driven activity is invisible in metrics, audit, and slog — there's no way to ask "what is Claude doing right now" vs "what is the user doing at the CLI."

**Outgoing (subprocess):** `internal/mcp/client.go` installs a Connect-go interceptor that sets two headers on every outgoing unary or streaming call:
- `x-gohome-source: mcp`
- `x-gohome-mcp-session: <session-id>` — a ULID minted at subprocess startup, stable for the subprocess's lifetime.

**Incoming (daemon):** the C7 server-side interceptor stack reads `x-gohome-source` (defaulting to `cli` when absent), attaches it to the request context via `internal/api/source.go`, and adds a `source` label to the existing `gohome_api_*` metrics. Handlers branch on source where it matters — currently only `ScriptService.Eval`, where MCP origin triggers the 64 KiB cap and the `MCPEvalRequested` audit emit.

This header is plain HTTP — no auth significance, no trust assumption (a malicious local CLI process can lie about its source label, but it can already do everything that label might gate). It exists for observability and audit attribution only.

---

## 4. Tool Catalog

All tools sit under `internal/mcp/tools/`. Inputs are hand-written Go structs with json-schema tags (per Q9 decision). Outputs are protojson of the underlying Connect response message, with the per-tool exceptions noted.

### 4.1 `gohome__get_state`

Read current state of one entity.

```go
type GetStateInput struct {
    EntityID string `json:"entity_id" jsonschema:"required" jsonschema_description:"Dotted entity ID, e.g. 'light.living_room'"`
}
```

Backs `EntityService.Get`. Output: protojson of `Entity`, renaming `friendly_name` → `name` for terseness.

### 4.2 `gohome__list_entities`

Browse entities with optional filters.

```go
type ListEntitiesInput struct {
    Areas    []string `json:"areas,omitempty"   jsonschema_description:"Filter to entities in any of these area slugs"`
    Zones    []string `json:"zones,omitempty"   jsonschema_description:"Filter to entities in any of these zone slugs"`
    Classes  []string `json:"classes,omitempty" jsonschema_description:"Filter by entity class, e.g. 'light', 'sensor'"`
    DeviceID string   `json:"device_id,omitempty"`
    Limit    int      `json:"limit,omitempty" jsonschema:"minimum=1,maximum=1000" jsonschema_description:"Default 100"`
    Cursor   string   `json:"cursor,omitempty" jsonschema_description:"Opaque token from a previous call"`
}
```

Backs `EntityService.List`. Output: `{entities: [...], next_cursor: "..."}`.

### 4.3 `gohome__call_capability`

Invoke a capability on one entity.

```go
type CallCapabilityInput struct {
    EntityID   string         `json:"entity_id" jsonschema:"required"`
    Capability string         `json:"capability" jsonschema:"required" jsonschema_description:"e.g. 'turn_on', 'set_brightness'"`
    Params     map[string]any `json:"params,omitempty" jsonschema_description:"Capability-specific parameters; see entity capabilities for shape"`
}
```

Backs `EntityService.CallCapability`. Output: `{accepted: true, command_id: "..."}` or error. Synchronous — returns once the driver has ACK'd, or `UNAVAILABLE` if the driver is down.

### 4.4 `gohome__query_events`

Historical event slice.

```go
type QueryEventsInput struct {
    Kinds        []string  `json:"kinds,omitempty" jsonschema_description:"Filter by event kind, e.g. 'state_changed'"`
    EntityPrefix string    `json:"entity_prefix,omitempty" jsonschema_description:"Filter by entity ID prefix, e.g. 'light.'"`
    Sources      []string  `json:"sources,omitempty"`
    FromCursor   uint64    `json:"from_cursor,omitempty"`
    ToCursor     uint64    `json:"to_cursor,omitempty"`
    FromTime     string    `json:"from_time,omitempty" jsonschema_description:"RFC3339"`
    ToTime       string    `json:"to_time,omitempty"   jsonschema_description:"RFC3339"`
    Limit        int       `json:"limit,omitempty" jsonschema:"minimum=1,maximum=1000"`
    Cursor       string    `json:"cursor,omitempty"`
}
```

Backs `EventService.Query`. Output: `{events: [...], next_cursor}`.

### 4.5 `gohome__tail_events`

Buffered window of new events. **Tool, not resource** (per Q6 — high-frequency append-only firehoses fit poorly into resource subscription semantics).

```go
type TailEventsInput struct {
    Kinds        []string `json:"kinds,omitempty"`
    EntityPrefix string   `json:"entity_prefix,omitempty"`
    Sources      []string `json:"sources,omitempty"`
    FromCursor   uint64   `json:"from_cursor,omitempty" jsonschema_description:"Resume from this cursor; 0 = from now"`
    MaxEvents    int      `json:"max_events,omitempty" jsonschema:"minimum=1,maximum=1000" jsonschema_description:"Default 100"`
    WaitSeconds  int      `json:"wait_seconds,omitempty" jsonschema:"minimum=0,maximum=60" jsonschema_description:"Block up to this long for new events; 0 = return what's there now"`
}
```

Implementation: opens an `EventService.Tail` server-stream with the filter and `from_cursor`, reads up to `MaxEvents` or until `WaitSeconds` elapses (whichever first), closes the stream, returns the buffered slice + the next cursor. Heartbeats are consumed silently.

### 4.6 `gohome__apply_scene`

Backs `SceneService.Apply`. Returns MCP tool error `{reason: "unimplemented", message: "Scene service not yet available; planned in a follow-up spec."}` until the Scene spec ships. Tool is registered so that the schema is discoverable and agents that try to use it get a clear "not yet" rather than a missing-tool error.

### 4.7 `gohome__run_script`

Invoke a named Starlark script.

```go
type RunScriptInput struct {
    Name           string         `json:"name" jsonschema:"required"`
    Args           map[string]any `json:"args,omitempty"`
    TimeoutSeconds int            `json:"timeout_seconds,omitempty" jsonschema:"minimum=1,maximum=300"`
}
```

Backs `ScriptService.Run`. Synchronous — blocks until the script completes or its deadline hits. Output: `{run_id, result, duration_ms}`. The `run_id` is the C6 correlation ID; the agent can call `query_events` with it later to inspect the run's emitted events.

### 4.8 `gohome__validate_config`

```go
type ValidateConfigInput struct {
    PklBundle []byte `json:"pkl_bundle" jsonschema:"required" jsonschema_description:"Tarball (gzipped) of Pkl source files"`
}
```

Backs `ConfigService.Validate`. Output: `{valid: bool, diff: [...], errors: [...]}`.

In practice, agents will more often build incremental edits via `read_config_file` + `write_config_file`, then call `validate_config` against the current bundle to confirm the result is sane before `apply_config`.

### 4.9 `gohome__apply_config`

```go
type ApplyConfigInput struct {
    PklBundle []byte `json:"pkl_bundle" jsonschema:"required"`
    Message   string `json:"message" jsonschema_description:"Free-form audit-trail message"`
    DryRun    bool   `json:"dry_run,omitempty" jsonschema_description:"Validate + diff, do not persist"`
    Strict    bool   `json:"strict,omitempty"  jsonschema_description:"Require a prior successful Validate with the same hash"`
}
```

Backs `ConfigService.Apply`. Output: `{applied: bool, diff: [...], applied_at}` or error with diff context on failure.

### 4.10 `gohome__eval_starlark`

Per Q5: read-only, 64 KiB cap, emits `MCPEvalRequested`, no opt-in writes.

```go
type EvalStarlarkInput struct {
    Source string `json:"source" jsonschema:"required" jsonschema_description:"Starlark expression or short script body. Read-only stdlib only: state, now, log, repr."`
}
```

Backs `ScriptService.Eval` — the request context is tagged `from_mcp = true` via the `x-gohome-source: mcp` header on the underlying Connect call. Server enforces:

- `KindMCPEval` Starlark context (already in C5: 30s wall-clock, 10M steps, expression-only false, read-only stdlib).
- Result is serialized via Starlark `repr()`. If serialization exceeds 64 KiB, output is truncated with marker `...[truncated; result was N bytes]`.
- `MCPEvalRequested{principal_id, session_id, source, result_sha256_hex, truncated, result_bytes, duration_ms, error}` event emitted to the event store on every call (success or failure), at the daemon side (the subprocess merely makes the Connect call).

Output: `{result: "...", duration_ms, truncated: bool}`.

### 4.11 `gohome__read_config_file`

```go
type ReadConfigFileInput struct {
    Path string `json:"path" jsonschema:"required" jsonschema_description:"Path relative to the gohome config dir"`
}
```

Implementation: `safepath.Resolve(configDir, path)` → absolute path; reject with `invalid_argument` reason `path_escape` if the resolved path is outside `configDir` or traverses a symlink that does so. Open file if it is a regular file; reject with reason `not_a_regular_file` otherwise. **Read scope is the entire config dir** (any extension) — agents can inspect docs, READMEs, and other operator notes in the config dir.

Hard cap on file size: 1 MiB. Files larger return `invalid_argument` reason `file_too_large` with metadata `size_bytes`.

Output: `{path, content: string, size_bytes, sha256_hex}`. Content is UTF-8; non-UTF-8 files return `failed_precondition` reason `not_utf8`.

### 4.12 `gohome__write_config_file`

```go
type WriteConfigFileInput struct {
    Path    string `json:"path" jsonschema:"required" jsonschema_description:"Path relative to the gohome config dir; must end in .pkl or .star"`
    Content string `json:"content" jsonschema:"required" jsonschema_description:"UTF-8 file contents (will replace any existing file)"`
}
```

Implementation:

1. `safepath.Resolve(configDir, path)` (same as read).
2. Reject if extension is not `.pkl` or `.star` (`invalid_argument`, reason `unsupported_extension`).
3. Best-effort syntax check: Pkl parse for `.pkl`, Starlark parse for `.star`. Reject on syntax error (`invalid_argument`, reason `syntax_error`, message includes line and column).
4. `mkdir -p` the parent directory if it is inside `configDir` (creating new directories under config is allowed).
5. Write atomically: write to `<path>.tmp.<rand>`, fsync, rename over the destination.
6. Call daemon-side `SystemService.RecordConfigFileEdit(path, sha256_hex, size_bytes)` to emit the `ConfigFileEdited` audit event.
7. Return `{path, sha256_hex, size_bytes}`.

`write_config_file` does **not** trigger reload. The agent must call `apply_config` separately when ready to deploy — this conflates editing with deploying, which is exactly what we want to keep separate.

---

## 5. Resources

Per Q6: live entity state and per-run automation traces are MCP resources with subscription support; tail-events stays a tool.

### 5.1 URI scheme

| URI pattern | Purpose | Mime type |
|---|---|---|
| `gohome://entities/{entity_id}` | One entity's current state | `application/json` |
| `gohome://entities?selector=<base64-json>` | Filtered set of entities | `application/json` |
| `gohome://automations/{automation_id}/runs/{run_id}/trace` | One automation run's trace | `application/json` |

The selector form encodes the same `EntitySelector` shape as `list_entities` (areas/zones/classes/device_id) as a base64-url-encoded JSON object. Agents rarely construct these by hand — `list_entities` and similar tools include `subscribe_uri` in entity entries so agents can grab a URI directly.

### 5.2 `resources/list`

The MCP server responds to `resources/list` with:

- **One resource entry per known entity**, paginated via the MCP standard cursor (server returns up to 200 per page). Each entry includes `uri`, `name` (entity friendly name), `mimeType`, and a `description` of the form `"Live state for <name> (<entity_id>)"`.
- **One static resource template** for the selector form, with `uriTemplate = "gohome://entities?selector={selector}"` and a description explaining the selector encoding.
- **One static resource template** for traces, with `uriTemplate = "gohome://automations/{automation_id}/runs/{run_id}/trace"`.

Trace resources are **not** enumerated in `resources/list` — there can be thousands of historical runs and listing them serves no purpose. Agents discover trace URIs from `run_script` results, `query_events` results, or `automation.Trigger` results, then `resources/read` or `resources/subscribe` directly.

### 5.3 `resources/read`

- **Single entity:** dial `EntityService.Get`, return current state as JSON. Same shape as the `get_state` tool output.
- **Selector:** dial `EntityService.List` with the decoded selector, return `{entities: [...], generated_at}`. **No pagination on read** — selector results are bounded by the C7 hard cap (1000); reading a selector that would exceed it returns MCP error `{reason: "selector_too_broad", suggested_filter: "..."}`.
- **Trace:** dial `AutomationService.Trace` with `from_cursor=0`, drain events for up to 5 seconds or until the run completes, return `{trace_events: [...], complete: bool, next_cursor}`. If `complete=false`, the agent can `resources/subscribe` to receive the rest as updates.

### 5.4 `resources/subscribe`

When the MCP client sends `resources/subscribe` for a URI, the server:

1. Records the subscription in an in-memory map keyed by `(client_session_id, uri)`.
2. Opens the underlying Connect server-stream:
   - `gohome://entities/...` → `EntityService.Subscribe` with the matching selector.
   - `gohome://automations/.../trace` → `AutomationService.Trace` from cursor 0 (or last-delivered cursor on resume).
3. On each event from the Connect stream, fires an MCP `notifications/resources/updated` for the URI.
4. Caches the latest snapshot per subscription so that a follow-up `resources/read` is local (no extra Connect roundtrip).

Standard MCP semantics: agent receives `updated`, then calls `resources/read` if it wants the new state.

**Heartbeats.** The underlying Connect streams emit heartbeats per C7 §4.5. The subprocess consumes these silently — they do not produce MCP notifications.

**Backpressure.**

- **Entity subscriptions:** per-subscription buffer of 256 pending updates. On overflow, server **coalesces** — fires a single `notifications/resources/updated` and discards intermediate snapshots; the next `resources/read` returns latest state. This is safe for entity state (eventually consistent re-read).
- **Trace subscriptions:** per-subscription buffer of 1024 pending updates (traces are higher-fidelity). On overflow, the server unsubscribes and closes the resource with an error notification carrying `reason: "trace_overflow"`. The agent can re-subscribe with the cached `next_cursor` to catch up if it cares.

### 5.5 `resources/unsubscribe` and session close

- Explicit `resources/unsubscribe` → close the Connect stream, drop the subscription map entry.
- Stdio EOF (MCP client disconnects) → close all subscriptions, drain in-flight notifications, exit the subprocess.
- Connect stream closes unexpectedly (daemon shutdown, network error) → fire one final `notifications/resources/updated` carrying the cached snapshot tagged with `error: "stream_closed"`, drop the subscription. Agent reconnects by re-subscribing.

---

## 6. Auth, Audit, and Observability

### 6.1 Auth in v1

The MCP server has no auth surface of its own. Every Connect call to the daemon goes over UDS, so C7's `LocalPeerCredAuthenticator` populates the request context with `Principal{ID: "system:local", Kind: "system", DisplayName: "local"}`. Tools and resources inherit that principal transitively.

Concretely:

- **No tokens** in v1. The MCP client launches the subprocess on the same machine; the subprocess running implies "the user is local."
- **No per-tool gating.** Every tool is callable. Agent activity is contained by what the local user can do (which, via UDS, is everything).
- **Filesystem tools** still operate as `system:local` from the audit perspective, but they touch the filesystem directly with the subprocess's UID. The subprocess inherits the user's UID; if the user can't write the config dir, the OS error is mapped to MCP `internal_error` reason `permission_denied`.

### 6.2 Cross-spec dependency note for C9

C8 ships the seam; **C9 closes the loop**. The C9 spec MUST include, at minimum:

1. **MCP-scoped token issuance.** A `gohome auth tokens create --scope mcp [--allow-tool gohome__*]` flow producing a bearer token bound to a specific user and a list of allowed `gohome__*` tools / resource patterns.
2. **Per-tool policy enforcement.** The MCP server's `tools/call` dispatch already calls `auth.Authorize(principal, action, target)` before invoking each tool. C9 swaps the C7 `AllowAllAuthorizer` stub for the policy-backed Authorizer; no change to `internal/mcp` is required.
3. **Per-resource policy enforcement.** Resource subscription dispatch already calls `Authorize(principal, "read", target)` per entity in the selector. C9 must define how policy intersects with selector subscriptions — at minimum, the subscription is silently narrowed to the policy-allowed subset; ideally, the agent receives a notification listing entities denied by policy so it knows the view is filtered.
4. **HTTP transport.** The same `internal/mcp` package wired as an `/mcp` route on the C7 listener, with token auth via Connect's `Authorization: Bearer` header (or MCP-spec OAuth, if the SDK has stabilized that path by then).
5. **Token attribution in audit events.** `MCPEvalRequested.principal_id` and `ConfigFileEdited.principal_id` should carry the real user ID, not `system:local`, once tokens exist.

### 6.3 Audit events

Two new payloads in `gohome.event.v1`, slotted into C7's "external ingress" 60-69 block:

```proto
message Payload {
  oneof kind {
    // ... existing 1-53 ...
    // 60-69: external ingress
    WebhookReceived       webhook_received      = 60;  // C7
    MCPEvalRequested      mcp_eval_requested    = 61;  // C8
    ConfigFileEdited      config_file_edited    = 62;  // C8
    // 70-79: registry mutations (C7)
    // 80-89: driver control (C7)
  }
}

message MCPEvalRequested {
  // 1-9: identity
  string principal_id = 1;        // "system:local" in v1; real user id post-C9
  string session_id   = 2;        // ULID minted by the subprocess at startup; correlates calls within one MCP client session

  // 10-19: payload
  string source            = 10;  // raw Starlark source as submitted
  string result_sha256_hex = 11;  // sha256 of the (post-truncation) repr output
  bool   truncated         = 12;
  uint32 result_bytes      = 13;  // pre-truncation size in bytes

  // 20-29: outcome
  uint32 duration_ms = 20;
  string error       = 21;        // empty on success
}

message ConfigFileEdited {
  // 1-9: identity
  string principal_id = 1;
  string session_id   = 2;        // empty when source is not MCP

  // 10-19: change
  string path        = 10;        // relative to config dir, as supplied by the writer
  string sha256_hex  = 11;        // post-write sha256
  uint32 size_bytes  = 12;
}
```

Both events are emitted by the **daemon**, not the subprocess:

- `MCPEvalRequested` by `ScriptService.Eval` when `x-gohome-source` is `mcp`.
- `ConfigFileEdited` by `SystemService.RecordConfigFileEdit`, called by the subprocess after a successful write.

Centralizing emission in the daemon means audit events have one source of truth and are always cursor-ordered alongside other events.

### 6.4 Action catalog stub

C8 ships a per-tool and per-resource action table in `internal/mcp/actions.go`:

```go
var toolActions = map[string]auth.Action{
    "gohome__get_state":          {Service: "MCP", Method: "get_state",          Verb: "read"},
    "gohome__list_entities":      {Service: "MCP", Method: "list_entities",      Verb: "read"},
    "gohome__call_capability":    {Service: "MCP", Method: "call_capability",    Verb: "call"},
    "gohome__query_events":       {Service: "MCP", Method: "query_events",       Verb: "read"},
    "gohome__tail_events":        {Service: "MCP", Method: "tail_events",        Verb: "read"},
    "gohome__apply_scene":        {Service: "MCP", Method: "apply_scene",        Verb: "call"},
    "gohome__run_script":         {Service: "MCP", Method: "run_script",         Verb: "call"},
    "gohome__validate_config":    {Service: "MCP", Method: "validate_config",    Verb: "read"},
    "gohome__apply_config":       {Service: "MCP", Method: "apply_config",       Verb: "admin"},
    "gohome__eval_starlark":      {Service: "MCP", Method: "eval_starlark",      Verb: "call"},
    "gohome__read_config_file":   {Service: "MCP", Method: "read_config_file",   Verb: "read"},
    "gohome__write_config_file":  {Service: "MCP", Method: "write_config_file",  Verb: "admin"},
}

var resourceActions = map[string]auth.Action{
    "gohome://entities/":         {Service: "MCP", Method: "subscribe_entities", Verb: "read"},
    "gohome://automations/":      {Service: "MCP", Method: "trace_automation",   Verb: "read"},
}
```

Per-tool target extraction is registered alongside each tool (entity ID for `get_state` / `call_capability`; `{Kind: "config", ID: input.Path}` for filesystem tools; etc.). The dispatch loop calls `auth.Authorize(principal, action, target)` before invoking the handler; the C7 `AllowAllAuthorizer` makes this a no-op in v1.

### 6.5 Metrics

All MCP metrics are emitted **daemon-side**, driven by the `x-gohome-source: mcp` header (subprocess-per-session has no Prometheus scrape target).

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `gohome_api_requests_total` | counter (existing C7) | gains `source` label: `cli` / `mcp` / `web` | source-tagged RPC count |
| `gohome_mcp_tool_calls_total` | counter | `tool`, `result` (`ok` / `error` / `unimplemented`) | per-tool invocation count |
| `gohome_mcp_tool_call_duration_seconds` | histogram | `tool`, `result` | per-tool latency |
| `gohome_mcp_resource_subscriptions_active` | gauge | `kind` (`entity` / `trace`) | currently-open subscriptions |
| `gohome_mcp_resource_updates_sent_total` | counter | `kind` | `notifications/resources/updated` fired |
| `gohome_mcp_resource_overflow_closes_total` | counter | `kind`, `reason` (`coalesced` / `trace_overflow`) | subscriptions affected by backpressure |
| `gohome_mcp_eval_starlark_truncated_total` | counter | — | `eval_starlark` calls whose output exceeded 64 KiB |
| `gohome_mcp_config_file_writes_total` | counter | `extension` (`.pkl` / `.star`), `result` (`ok` / `syntax_error` / `path_escape` / `unsupported_extension`) | filesystem-tool writes |

Tool-call and resource-subscription metrics need the daemon to know the tool/resource being invoked; the subprocess passes that via two additional Connect headers per call:

- `x-gohome-mcp-tool: gohome__<name>` — set on every Connect call made on behalf of a tool dispatch.
- `x-gohome-mcp-resource: <uri>` — set on every Connect call made on behalf of a resource read or subscribe.

A daemon-side interceptor (`internal/api/mcp_interceptor.go`) reads these and emits the corresponding metrics. (No security significance — same trust boundary as `x-gohome-source`.)

### 6.6 Logging

- **Subprocess slog:** writes to **stderr only** (stdout is reserved for MCP JSON-RPC framing). One log line per tool dispatch (`level=info tool=... duration_ms=... ok=true`), one per subscription open / close / overflow, error lines on Connect failures.
- **Daemon slog:** unchanged from C7; the existing interceptor stack records principal, request_id, transport. With C8, every record gains a `source` field (`cli` / `mcp` / `web`).

### 6.7 Tracing

OTel propagation: the subprocess starts a span per tool call (`gohome.mcp.<tool>`) and per resource subscription (`gohome.mcp.<resource_kind>`), sets a `traceparent` header on outgoing Connect calls, and the daemon's C7 tracing interceptor continues the trace. End-to-end span coverage from "agent called tool X" through engine invocation.

---

## 7. Errors

### 7.1 Connect → MCP error mapping

MCP tool errors are returned as `CallToolResult` with `isError: true` and a single `content` entry of type `text` carrying a JSON-encoded error envelope:

```json
{
  "reason":  "entity_not_found",
  "message": "Entity 'light.foo' does not exist.",
  "metadata": {"entity_id": "light.foo"},
  "request_id": "01HZ...",
  "correlation_id": ""
}
```

`reason` and `metadata` mirror the `gohome.error.v1alpha1.ErrorDetail` returned by the underlying Connect call. `request_id` is taken from the Connect response headers; `correlation_id` is set when present (e.g. for failed `apply_config` returning an apply correlation ID).

The mapping table:

| Connect code | MCP behavior |
|---|---|
| `INVALID_ARGUMENT` | `isError: true`, reason from ErrorDetail (e.g. `bad_cursor`, `unknown_capability`) |
| `NOT_FOUND` | `isError: true`, reason from ErrorDetail (e.g. `entity_not_found`) |
| `FAILED_PRECONDITION` | `isError: true`, reason from ErrorDetail |
| `PERMISSION_DENIED` | `isError: true`, reason `forbidden` (will start firing post-C9) |
| `UNAUTHENTICATED` | `isError: true`, reason `unauthenticated`, message hints at C9 token scope (will fire post-C9 for HTTP transport) |
| `RESOURCE_EXHAUSTED` | `isError: true`, reason from ErrorDetail (`subscription_overflow`, etc.) |
| `DEADLINE_EXCEEDED` | `isError: true`, reason `deadline_exceeded` |
| `UNIMPLEMENTED` | `isError: true`, reason `unimplemented`, message names the underlying RPC and a planning hint |
| `INTERNAL` | `isError: true`, reason `internal`, generic message + `request_id` for operator log lookup; full error stays in daemon logs |
| `UNAVAILABLE` | `isError: true`, reason `unavailable` (driver instance down, daemon shutting down) |
| `CANCELED` | not surfaced to the MCP client; the JSON-RPC call simply returns its accumulated buffered result so far |

### 7.2 MCP-level errors (protocol)

Malformed `tools/call` requests, unknown tool names, unknown resource URIs, and similar protocol-layer issues are returned as MCP JSON-RPC errors with the standard error code set (`-32600` invalid request, `-32601` method not found, etc.). The official SDK handles these — `internal/mcp` only catches them when it needs to add context.

### 7.3 Filesystem-tool errors

Filesystem tools surface OS errors directly:

| OS situation | MCP error |
|---|---|
| Path resolution escapes config dir | `invalid_argument`, reason `path_escape` |
| Path is a symlink that escapes config dir | `invalid_argument`, reason `path_escape` |
| File does not exist (read) | `not_found`, reason `file_not_found` |
| Path is a directory (read) | `failed_precondition`, reason `not_a_regular_file` |
| File too large (>1 MiB read) | `invalid_argument`, reason `file_too_large` |
| Non-UTF-8 file (read) | `failed_precondition`, reason `not_utf8` |
| Unsupported extension (write) | `invalid_argument`, reason `unsupported_extension` |
| Pkl or Starlark syntax error (write) | `invalid_argument`, reason `syntax_error`, metadata includes `line` and `column` |
| OS-level write permission denied | `internal`, reason `permission_denied` |
| Disk full / I/O error (write) | `internal`, reason `io_error`, message includes errno |

---

## 8. Transports and CLI

### 8.1 Stdio transport

The only transport in v1. Implementation uses the official `modelcontextprotocol/go-sdk` server with a stdio transport adapter:

```go
// internal/mcp/server.go (sketch)
func Run(ctx context.Context, deps Deps) error {
    server := mcp.NewServer(mcp.ServerInfo{
        Name:    "gohome",
        Version: deps.Version,
    })

    tools.Register(server, deps)
    resources.Register(server, deps)

    return server.Serve(ctx, mcp.Stdio(os.Stdin, os.Stdout, os.Stderr))
}
```

Lifecycle:

- Subprocess started by the MCP client per its config.
- On startup, dial the daemon over UDS, call `SystemService.Version` (capability probe — also verifies the daemon is reachable and a compatible version) and `SystemService.GetConfigDir`. On dial failure, write a single line to stderr (`gohome mcp: cannot reach gohomed at <path>: <error>`) and exit 1.
- Mint a session ULID; set `x-gohome-source: mcp` and `x-gohome-mcp-session: <session>` interceptors on the Connect client.
- Register the 12 tools and 2 resource types with the SDK server.
- Run the SDK server's stdio loop until stdin EOF.
- On shutdown: close all open resource subscriptions (closing their Connect server-streams), flush stderr, exit 0.

### 8.2 CLI surface

Two new subcommands under `gohome`, both in `internal/cli/cmd_mcp.go`:

```
gohome mcp serve [--endpoint <uri>]
    Run the MCP server on stdio. Used by MCP clients via subprocess launch.
    --endpoint defaults to unix://@data/gohomed.sock (same as other gohome subcommands).

gohome mcp tools
    Print the MCP tool catalog (name, summary, input schema) and exit.
    Useful for verifying the surface a given gohome version exposes.
```

`gohome mcp tools` prints to stdout in human-friendly form, with a `--json` flag for machine consumption. It dials the daemon to fetch live state (which scenes / automations exist for the apply-scene and run-script tool descriptions) so the printed surface matches what the running server would expose.

**Styling (lipgloss).** The default human-friendly output uses the existing `internal/cli/styles.go` palette plus a small per-element extension, so it matches the rest of the CLI:

| Element | Style |
|---|---|
| Header bar (`MCP TOOLS — gohome <version>`) | `styles.HeaderBar` (bold, accent-colored background, padded) |
| Tool name (`gohome__get_state`) | `styles.Identifier` (bold, primary color) |
| One-line summary | `styles.SubtleText` (muted) |
| Verb badge (`read` / `call` / `admin`) | `styles.BadgeRead` / `BadgeCall` / `BadgeAdmin` (color-coded) — new in `styles_mcp.go` |
| `UNIMPLEMENTED` badge | `styles.BadgeWarn` (warning-yellow background) |
| Input schema field name | `styles.FieldName` (semibold) |
| Input schema field type | `styles.TypeName` (italic, muted) |
| Required field marker | `styles.Required` (red asterisk) |
| Section divider between tools | `styles.Divider` (subtle horizontal rule) |
| Footer summary line | `styles.SubtleText` |

`styles_mcp.go` houses the three new badge styles; everything else is reused from the existing CLI styles modules. `--json` output omits all styling and emits a JSON array of `{name, summary, verb, target_kind, status, input_schema}` objects.

### 8.3 MCP client setup snippets

Documented in the implementation README alongside the C8 work. Examples:

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):
```json
{
  "mcpServers": {
    "gohome": { "command": "gohome", "args": ["mcp", "serve"] }
  }
}
```

**Claude Code:**
```bash
claude mcp add gohome -- gohome mcp serve
```

**Cursor / other:** generic stdio MCP server pointed at `gohome mcp serve`.

If `gohomed` is not running, the subprocess exits 1 with a stderr message explaining how to start the daemon. The MCP client surfaces that message to the user.

---

## 9. Configuration

C8 adds a small `gohome.mcp` Pkl module to `internal/config/pkl/gohome/` for daemon-side knobs:

```pkl
module gohome.mcp

class MCPConfig {
  // Maximum bytes returned from gohome__eval_starlark before truncation.
  // Hard cap; agents that need more should write a named script.
  eval_result_max_bytes: UInt = 65536      // 64 KiB

  // Maximum file size readable via gohome__read_config_file.
  read_file_max_bytes:   UInt = 1048576    // 1 MiB

  // Per-subscription MCP `notifications/resources/updated` buffer.
  entity_subscription_buffer: UInt = 256
  trace_subscription_buffer:  UInt = 1024

  // Hold time on gohome__tail_events.
  tail_default_wait_seconds: UInt = 0
  tail_max_wait_seconds:     UInt = 60
}
```

These are daemon-side because the cap enforcement and buffer sizing happen daemon-side (eval cap in `ScriptService.Eval`, file cap in `RecordConfigFileEdit`'s preflight check, subscription buffers in the daemon's Connect server-stream). The subprocess reads these values via a dedicated `SystemService.GetMCPConfig` RPC at startup so its own input validation matches.

There is no `gohome.mcp.tools.disabled` knob in v1. Tool-set restriction is C9's policy job, not a separate config axis.

---

## 10. Testing Strategy

Three layers, mirroring C7's structure.

### 10.1 Tool / resource unit tests

Each `tools/*.go` and `resources/*.go` file has a `*_test.go` in the same package using fake implementations of the `deps.go` interfaces (mocked Connect client, in-memory FS for filesystem tools). Tests are table-driven, one row per (valid input, error case, edge case). Error mapping is tested exhaustively — every `(Connect code, ErrorDetail.reason) → (MCP error reason, message shape)` triple is a row.

Filesystem tool tests cover, at minimum:
- Path containment: `..` traversal, absolute paths, paths through symlinks pointing outside config dir.
- Extension policy: `.txt`, `.pkl.bak`, no extension, double-extension `.pkl.tmp`.
- Syntax check: deliberately malformed `.pkl` and `.star`.
- Atomic write: simulate failure between fsync and rename, assert original file unchanged.

Eval tests cover:
- Truncation at exactly the cap (64 KiB) and one byte over.
- All error paths (parse error, runtime error, deadline, step limit) emit `MCPEvalRequested` with correct shape.

### 10.2 SDK-level integration tests

`internal/mcp/server_test.go` spawns the MCP server in-process (using a pipe for stdio), drives it via the official SDK's MCP **client** library, and asserts behavior:

- `tools/list` returns exactly the 12 documented tools, in stable order (`get_state`, `list_entities`, `call_capability`, `query_events`, `tail_events`, `apply_scene`, `run_script`, `eval_starlark`, `validate_config`, `apply_config`, `read_config_file`, `write_config_file`).
- `tools/call` for each tool against a fake daemon — assert wire shape, not semantics (semantics are tested in 10.1).
- `resources/list` returns the registered entity resources + the two templates.
- `resources/subscribe` produces `notifications/resources/updated` when the fake daemon sends events on the underlying Connect stream.
- `resources/unsubscribe` and stdio EOF both close subscriptions cleanly (verified by subscription gauge dropping to zero).
- Backpressure: configure a 2-update buffer for entities; emit 100 underlying events without reading; assert exactly one `updated` notification fired (coalesced) and the metric incremented.
- Trace overflow: configure a 2-update buffer for traces; emit 100 events; assert subscription closed with `trace_overflow` notification.

### 10.3 End-to-end integration (`//go:build integration`)

A fixture daemon is started with an in-memory SQLite, a fake Carport driver, and Pkl config containing a few automations and entities. The test:

1. Starts `gohome mcp serve` as a real subprocess pointed at the fixture daemon's UDS.
2. Drives it via the official SDK MCP client.
3. Walks the catalog: list entities, get one, call `turn_on` on a fake light, assert the fake driver received the command, assert a `StateChanged` event arrives via `tail_events`.
4. Subscribes to `gohome://entities/light.x`; toggles state via the driver; asserts `notifications/resources/updated` fires; reads the resource; asserts new state is reflected.
5. Calls `eval_starlark` with `state("light.x")["brightness"]`; asserts result and an `MCPEvalRequested` event in the event store.
6. Calls `read_config_file`, modifies it via `write_config_file`, calls `validate_config`, calls `apply_config`; asserts `ConfigFileEdited` and `ConfigApplied` both appear in the event store, in that order.
7. Triggers an automation via `run_script` (or via a new entity event); subscribes to the trace resource; asserts trace events flow until completion.

### 10.4 Audit / metric assertions

Both 10.2 and 10.3 assert that:
- `gohome_mcp_tool_calls_total` increments for every tool dispatch with the right labels.
- `gohome_mcp_resource_subscriptions_active` reflects open subscription count.
- `gohome_api_requests_total{source="mcp"}` is non-zero after MCP activity.
- The action catalog table covers every registered tool (compile-time check via init: `if len(toolActions) != registeredToolCount { panic(...) }`).

---

## 11. Implementation Order

Suggested sequencing (detailed in the C8 implementation plan):

1. Add `modelcontextprotocol/go-sdk` Go dependency.
2. Extend `gohome.event.v1.Payload` with `MCPEvalRequested` (61) and `ConfigFileEdited` (62); regenerate.
3. Add `SystemService.GetConfigDir`, `SystemService.RecordConfigFileEdit`, `SystemService.GetMCPConfig` RPCs (proto + handlers).
4. Add `gohome.mcp` Pkl module + daemon plumbing to load it.
5. Add `x-gohome-source` server-side interceptor in `internal/api`; extend `gohome_api_requests_total` with the `source` label.
6. Add 64 KiB cap + `MCPEvalRequested` emission in `ScriptService.Eval` (gated on `x-gohome-source: mcp`).
7. Daemon-side `internal/observability/metrics.go`: register `gohome_mcp_*` metrics; add the small interceptor that reads `x-gohome-mcp-tool` / `x-gohome-mcp-resource` headers.
8. `internal/mcp/fs/safepath.go` + tests.
9. `internal/mcp/fs/syntax.go` + tests.
10. `internal/mcp/client.go` (Connect client wired to UDS, source / session / per-call header interceptors).
11. `internal/mcp/errors.go` (Connect → MCP error mapping).
12. `internal/mcp/actions.go` (action catalog).
13. `internal/mcp/audit/recorder.go` (calls `SystemService.RecordConfigFileEdit`).
14. `internal/mcp/server.go` skeleton (registers nothing yet; serves stdio loop).
15. `internal/mcp/tools/entities.go` (`get_state`, `list_entities`, `call_capability`) + tests.
16. `internal/mcp/tools/events.go` (`query_events`, `tail_events`) + tests.
17. `internal/mcp/tools/scenes.go` (`apply_scene` UNIMPLEMENTED passthrough) + tests.
18. `internal/mcp/tools/scripts.go` (`run_script`, `eval_starlark`) + tests.
19. `internal/mcp/tools/config.go` (`validate_config`, `apply_config`) + tests.
20. `internal/mcp/tools/files.go` (`read_config_file`, `write_config_file`) + tests.
21. `internal/mcp/resources/entities.go` + tests.
22. `internal/mcp/resources/traces.go` + tests.
23. `internal/cli/cmd_mcp.go` (`serve` and `tools` subcommands).
24. `internal/mcp/server_test.go` SDK-level integration tests.
25. End-to-end integration test under `//go:build integration`.
26. README / docs update with `claude mcp add gohome -- gohome mcp serve` instructions.

---

## 12. Decision Record

| # | Decision | Rationale |
|---|---|---|
| D1 | C8 ships before C9; local-only, stdio transport, system:local principal | Master roadmap order; closest fit to what C7's auth seam allows; "Claude Desktop launches local subprocess" is the most-used MCP pattern. C9 closes remote / token / policy story. |
| D2 | Use the official `modelcontextprotocol/go-sdk` (v1.5.0+) | Past 1.x GA, actively maintained by Anthropic + Google, spec-aligned. Community SDKs are still 0.x. Long project lifetime favors the official one. |
| D3 | Stdio transport only in v1; HTTP deferred to C9 | HTTP without real auth is dead code. C9 owns both at once. Same `internal/mcp` package serves both. |
| D4 | Ship all 12 master-design tools; `apply_scene` returns `UNIMPLEMENTED` | Schema is final from day one — agents and tooling code against the stable surface; UNIMPLEMENTED is a clear, discoverable signal. Reduces churn when Scene spec ships. |
| D5 | Live entity state and per-run traces are MCP **resources**; tail-events stays a **tool** | Resources fit "agent watches a thing change" semantics. Append-only firehoses are a poor fit; cursor-based windowed tool is more honest. Mixed model is more spec-correct than tools-only and more usable than resources-only. |
| D6 | Hand-written tool input schemas, protojson outputs | Agents are LLMs reading natural-language descriptions, not protobuf consumers. Inputs need agent-targeted shapes (`limit` not `page_size`). Outputs are structured data; protojson is the right shape for agents to consume programmatically. |
| D7 | `eval_starlark` is read-only with a 64 KiB result cap; emits `MCPEvalRequested` | Mutations through `eval_starlark` are bypass-prone and audit-invisible by their nature; force them through `call_capability` / `run_script` instead. Cap protects the agent's context window. Audit event makes agent activity a first-class log entry. |
| D8 | Filesystem tools: strict path containment, `.pkl` / `.star` write only, syntax check, `ConfigFileEdited` audit, no auto-apply | Editing-without-deploying matches how humans use git. Path containment is non-negotiable. Syntax check stops obvious garbage from landing on disk before validate. |
| D9 | `x-gohome-source` Connect header (plus `x-gohome-mcp-session` / `-tool` / `-resource`) for attribution and metrics | Without it, MCP activity is invisible. No security significance — same trust boundary as the rest of localhost. Cheap, clean, observability win. |
| D10 | Two daemon-side RPCs (`GetConfigDir`, `RecordConfigFileEdit`) instead of subprocess-direct event store writes | Daemon owns the event store. One source of truth for audit. Subprocess never touches storage. |
| D11 | Daemon-side metrics only; no Prometheus on the subprocess | Subprocess-per-session has no scrape target; metrics live where the long-lived process is. The header-based labelling is sufficient to attribute. |
| D12 | Action catalog stub ships in C8 even though Authorizer is no-op | The seam goes in once. C9 swaps the Authorizer; `internal/mcp` doesn't change. |
| D13 | `gohome mcp tools` debug subcommand alongside `gohome mcp serve` | Operators want to see "what tools does this gohome version expose" without launching an MCP client. Cheap to add, useful in support / diagnosis. |

---

## 13. Explicit Deferrals

- **HTTP transport** — C9 (with real auth tokens).
- **Per-tool / per-resource policy enforcement** — C9 swaps the Authorizer.
- **Token-attributed audit events** (`principal_id` ≠ `system:local`) — C9.
- **`apply_scene` real implementation** — Scene spec.
- **MCP server-initiated sampling** — not used in v1; revisit if a use case appears.
- **MCP prompts** — same as sampling.
- **Resource subscriptions for things other than entity state and run traces** — driver health, automation enable state, current config snapshot stay tool-shaped for v1.
- **Multi-instance fan-in** — C12.
- **High-level / workflow tools** — out of v1 scope; agents compose primitives.
- **Subprocess-side metrics scrape** — daemon-side metrics with source labels are sufficient.

---

*End of C8 design document.*
