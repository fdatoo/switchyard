# C7 — Connect-RPC API Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Date:** 2026-04-23
**Status:** Draft
**Depends on:** C1 (Event Core), C2 (Carport Protocol), C3 (Driver SDK), C4 (Pkl Config), C5 (Starlark Runtime), C6 (Automation & Script Engine)
**Unblocks:** C8 (MCP Server), C9 (Auth & Policy), C10 (Web UI), C11 (HA Import Tool), C12 (Edge Agent)

---

## 1. Scope

C7 delivers the first user-facing wire surface of `gohomed`: a Connect-RPC API defined as `gohome.v1alpha1.*` protobuf services, served over both a Unix domain socket and a TCP listener, consumed by the rewritten `gohome` CLI today and by MCP, the web UI, third-party Python/Go/TS clients, and the edge agent tomorrow. C7 also closes C6's webhook deferral by adding the HTTP webhook endpoint and the corresponding `WebhookReceived` event.

### 1.1 In scope

- **Proto sources** for all 13 services from master §7.1 (`Entity`, `Device`, `Area`, `Zone`, `Driver`, `Event`, `Scene`, `Automation`, `Script`, `Config`, `Dashboard`, `Auth`, `System`) plus shared `common.proto` and a separate `gohome.error.v1alpha1.ErrorDetail` payload for Connect error details.
- **Implemented handlers** for the ten services whose backing logic exists today (`Entity`, `Device`, `Area`, `Zone`, `Driver`, `Event`, `Automation`, `Script`, `Config`, `System`).
- **`UNIMPLEMENTED` handlers** for `Scene`, `Dashboard`, `Auth` — proto shape is final from C7, handlers return `connect.CodeUnimplemented` until their own specs ship.
- **Wire conventions** binding every service: error taxonomy, pagination cursor codec, streaming heartbeats and backpressure, timestamp and ID conventions, versioning policy.
- **Listener stack**: single `http.Server` bound to a UDS listener and a TCP listener, serving Connect, gRPC, and gRPC-Web on the same handlers; a non-Connect `/webhooks/{slug}` handler; a no-auth `/healthz` liveness probe.
- **Auth seam**: `internal/auth` package with `Principal`, `Authenticator`, `Authorizer` interfaces. Stub implementation grants a synthetic `system:local` principal on UDS via peer-cred and rejects all TCP requests until C9 lands.
- **Interceptor stack**: request-id, slog, metrics, tracing, recover, authenticate, authorize.
- **Webhook ingress**: `/webhooks/{slug}` handler with HMAC validation against a Pkl-declared secret, emitting a new `WebhookReceived` event that drives C6's `WebhookTrigger` matcher.
- **`gohome` CLI rewrite**: every existing subcommand ported to the generated `connectgo` client; the JSON-op socket surface in `internal/daemon/recovery.go` is deleted in the same change.

### 1.2 Explicit non-goals

- **C9 work** — passkey, OIDC, password auth, real API tokens, policy compilation and enforcement beyond the seam. C7 ships interfaces and a stub.
- **TLS lifecycle** — ACME, self-signed generation, cert rotation, mTLS. C7 accepts operator-supplied cert/key files via Pkl; everything else is C13.
- **Scene engine and dashboard rendering** — protos only.
- **Generated client publishing** to PyPI, npm, Packagist, crates.io — Buf module and `gen/` tree exist; distribution is C13.
- **`EventService.ReplayAt`** — requires point-in-time projector rebuild (C1 §5.5 snapshot + replay path) not implemented in C1. Deferred to a small follow-up once there is a consumer.
- **Webhook response bodies** — webhooks always respond `202 Accepted` with no body. Rich responses (returning data from an automation run) are out of scope for v1.
- **MCP tool shim** — handled in C8. C7 only provides the Connect surface that C8 will wrap.

---

## 2. Background

Master design §7.1 names 13 `gohome.v1.*` services, a Connect-RPC transport decision, and a Buf-based generated-client strategy (Go, TypeScript, Python). C1–C6 built the backing machinery — event store, registry, state cache, Carport transport, Pkl config, Starlark runtime, automation and script engines — plus a temporary Unix-socket JSON-op surface (`{"op":"automation_list"}`, `{"op":"script_run"}`, `{"op":"snapshot"}`, `{"op":"starlark_eval"}`, and about a dozen others) that the `gohome` CLI currently speaks. That socket was always scaffolding for C1-C6 testability; it has no versioning, no typed wire schema, no auth, and no streaming.

C7 retires that scaffolding and establishes the real API. Three downstream specs are waiting on it: C8 wraps the Connect surface as MCP tools, C10 generates a TypeScript client against it, C11 uses `ConfigService.Apply` as its HA-import landing pad.

C6 shipped `WebhookTrigger` as a Pkl-validated but inert trigger. C7 is the single owner of everything `gohomed` exposes over HTTP, so the webhook endpoint lands here and completes that loop.

---

## 3. Architecture Overview

### 3.1 Service inventory and implementation status

| Service | RPCs | Status in C7 | Backing |
|---|---|---|---|
| `EntityService` | `List`, `Get`, `CallCapability`, `Subscribe` (server-stream) | Live | `registry`, `state`, `carport` |
| `DeviceService` | `List`, `Get`, `Rename`, `Reassign` | Live | `registry` |
| `AreaService` | `List`, `Get` | Live | `registry` |
| `ZoneService` | `List`, `Get` | Live | `registry` |
| `DriverService` | `ListDrivers`, `ListInstances`, `InstanceHealth`, `RestartInstance` | Live | `carport` |
| `EventService` | `Query`, `Tail` (server-stream) | Live (no `ReplayAt`) | `eventstore` |
| `SceneService` | `List`, `Apply`, `Preview` | UNIMPLEMENTED | — |
| `AutomationService` | `List`, `Get`, `Enable`, `Disable`, `Trigger`, `Trace` (server-stream) | Live | `automation` |
| `ScriptService` | `List`, `Run`, `Cancel` | Live | `script` |
| `ConfigService` | `Validate`, `Apply`, `Reload`, `GetArtifact` | Live | `config` |
| `DashboardService` | `List`, `Get`, `SaveLayout` | UNIMPLEMENTED | — |
| `AuthService` | `Login`, `Logout`, `CurrentUser`, `CreateToken`, `RevokeToken`, `ListUsers`, `RegisterPasskey`, `StartWebAuthnChallenge` | UNIMPLEMENTED | C9 |
| `SystemService` | `Version`, `Health`, `Metrics`, `Diagnostics` | Live | direct |

### 3.2 Package map

```
proto/gohome/
├── v1alpha1/
│   ├── common.proto         # PageRequest, PageResponse, Cursor, Heartbeat, EntitySelector
│   ├── entity.proto         # EntityService + request/response types
│   ├── device.proto
│   ├── area.proto
│   ├── zone.proto
│   ├── driver.proto
│   ├── event.proto          # EventService (distinct from gohome.event.v1)
│   ├── scene.proto
│   ├── automation.proto
│   ├── script.proto
│   ├── config.proto
│   ├── dashboard.proto
│   ├── auth.proto
│   └── system.proto
└── error/v1alpha1/
    └── error.proto          # gohome.error.v1alpha1.ErrorDetail

gen/gohome/
├── v1alpha1/                # generated Go types
│   └── v1alpha1connect/     # generated Connect-Go service interfaces
└── error/v1alpha1/

internal/api/
├── service_entity.go        # EntityService impl (interface-typed deps)
├── service_device.go
├── service_area.go
├── service_zone.go
├── service_driver.go
├── service_event.go
├── service_automation.go
├── service_script.go
├── service_config.go
├── service_system.go
├── service_unimplemented.go # Scene, Dashboard, Auth stubs
├── deps.go                  # dependency interfaces (EntityReader, EventSource, ...)
├── errors.go                # domain-error → connect.Error mapping
├── pagination.go            # opaque cursor codec (encode/decode)
├── streaming.go             # heartbeat ticker + backpressure helpers
├── webhook.go               # /webhooks/{slug} handler + WebhookReceived emit
├── time.go                  # timestamp conversion helpers
└── api_test.go              # table-driven handler tests over fakes

internal/api/listener/
├── listener.go              # http.Server build, mux, UDS + TCP binds, graceful shutdown
├── h2c.go                   # plaintext HTTP/2 server setup
└── interceptors.go          # auth, request-id, slog, metrics, tracing, recover

internal/auth/
├── auth.go                  # Principal, Authenticator, Authorizer interfaces, action constants
├── local.go                 # peer-cred → system:local stub for UDS
└── reject.go                # default-reject Authenticator for TCP until C9
```

### 3.3 Deployment topology

```
                 UDS (@data/gohomed.sock, 0600)   TCP (127.0.0.1:8080)
                            │                              │
                            ▼                              ▼
                         ┌─────────────────────────────────────┐
                         │  http.Server (h2c for plaintext)    │
                         └──────────────────┬──────────────────┘
                                            │
                                  ┌─────────▼─────────┐
                                  │   http.ServeMux   │
                                  └───┬──────┬────────┘
                                      │      │      │
                           /connect.* │      │      │ /healthz
                                      │   /webhooks │
                                      │      │      │
                                      ▼      ▼      ▼
                           Connect handlers   webhook   liveness
                              │  (chain)      handler   probe
                              ▼
                       interceptors: recover →
                                     request-id →
                                     slog →
                                     metrics →
                                     tracing →
                                     authenticate →
                                     authorize →
                              ▼
                        internal/api services
                              │
                              ▼
                        engine interfaces
                        (EntityReader, EventSource,
                         AutomationControl, ScriptRunner,
                         ConfigApplier, DriverControl, ...)
                              │
                              ▼
                        C1-C6 implementations
                        (registry, state, eventstore,
                         automation, script, config, carport)
```

One `http.Server`, one `ServeMux`, two listeners. The Connect handler set is served on both listeners identically — the only per-listener difference is what the auth interceptor decides about the request.

### 3.4 Dependency interfaces (in `internal/api/deps.go`)

The API package depends on backing engines via **narrow read-only / read-write interfaces** owned by `internal/api`, never imports of the concrete engine packages beyond what's needed to build the interface values at daemon construction time. Each service handler accepts one or more of these interfaces, which means:

- Tests in `internal/api` use fakes, not real engines.
- C9's auth backend can slot in without a handler rewrite.
- Engines stay in their C1-C6 packages as the source of truth; `internal/api` is a pure translation layer.

Sketch:

```go
// internal/api/deps.go
type EntityReader interface {
    ListEntities(ctx context.Context, sel EntitySelector, page PageReq) ([]Entity, Cursor, error)
    GetEntity(ctx context.Context, id string) (Entity, error)
}

type CapabilityCaller interface {
    Call(ctx context.Context, target, capability string, params map[string]any) error
}

type EventSource interface {
    Query(ctx context.Context, filter EventFilter, page PageReq) ([]Event, Cursor, error)
    Subscribe(ctx context.Context, filter EventFilter, fromCursor uint64) (<-chan Event, error)
}

type AutomationControl interface {
    List(ctx context.Context) ([]Automation, error)
    Get(ctx context.Context, id string) (Automation, error)
    SetEnabled(ctx context.Context, id string, enabled bool) error
    Trigger(ctx context.Context, id string) (runID string, err error)
    Trace(ctx context.Context, runID string) (<-chan TraceEvent, error)
}

// ... ScriptRunner, ConfigApplier, DriverControl, RegistryReader, ...
```

The daemon wires these interfaces to concrete implementations in `internal/daemon/daemon.go`. The engines already expose methods of roughly this shape; C7 introduces the interfaces as the wiring contract.

---

## 4. Wire Conventions

Every service in `gohome.v1alpha1.*` MUST follow these conventions. They are the contract C8, C10, and every third-party client depends on.

### 4.1 Versioning policy

- **Initial package: `gohome.v1alpha1.*`.** Follows `gohome/docs/proto-hygiene.md`: `v1alpha*` may make wire-breaking changes between releases with a migration note.
- **Graduation to `gohome.v1`** is a one-way door. It requires:
  - C8 (MCP), C10 (Web UI), and at least one external Python client have been built against `v1alpha1` and agree the surface is stable.
  - A decision-record entry in this spec (§13) or a follow-up graduation spec.
  - A deprecation window where both `v1alpha1` and `v1` are served; `v1alpha1` is removed one minor release after.
- **Existing `v1` data messages stay `v1`.** `gohome.event.v1`, `gohome.entity.v1`, `gohome.config.v1` are referenced by `v1alpha1` services as-is. Data message stability was already earned in C1-C4.
- **Proto hygiene** (grouped numbering, `reserved` on removal, tens-aligned blocks) applies to every file in this surface.

### 4.2 Naming conventions

- **Services** end in `Service`: `EntityService`, `EventService`. (The master design's table already does this; we keep it.)
- **RPC verbs** are imperative: `List`, `Get`, `Create`, `Update`, `Delete`, `Apply`, `Trigger`, `Cancel`, `Subscribe`, `Tail`, `Trace`.
- **Request / response types** are `<Verb><Resource>Request` / `<Verb><Resource>Response` (e.g. `ListEntitiesRequest`). This collides with proto-hygiene's `RPC_REQUEST_STANDARD_NAME` lint, which we've already excluded in `buf.yaml`.
- **Entity IDs** are dotted-path strings, carried as `string` (e.g. `"light.living_room_ceiling"`) — consistent with C1.
- **Timestamps** are `google.protobuf.Timestamp`.
- **Durations** are `google.protobuf.Duration`.
- **Free-form key/value metadata** is `map<string, string>` — anything stronger belongs in a typed field.

### 4.3 Pagination

Every `List*` RPC takes a `PageRequest` and returns a `PageResponse`. Cursors are opaque base64-encoded bytes that decode to a service-specific internal state (typically just the last-seen primary key or cursor from C1).

```proto
// common.proto
message PageRequest {
  // 1-9: paging
  uint32 page_size     = 1;   // max 1000; default 100 if 0
  string page_token    = 2;   // opaque; empty means "first page"
}

message PageResponse {
  // 1-9: next-page info
  string next_page_token = 1; // empty means "no more pages"
  uint32 total_size      = 2; // optional; 0 means "unknown"
}
```

Rules:
- **Cap enforced server-side:** any `page_size` > 1000 is clamped to 1000 with a log line, not an error. (A client asking for "all" gets paginated, not rejected.)
- **`page_token`** is opaque. The server encodes internal state (typically a C1 cursor or a registry row ID); the client passes it back verbatim. Callers MUST NOT parse it.
- **Stable ordering**: every `List*` RPC documents a stable sort order. Default for registry-backed lists is by slug ascending; default for event queries is by cursor ascending.
- **Cross-page consistency** is best-effort. A resource appearing / disappearing mid-pagination is tolerated; we do not hold snapshots.

### 4.4 Error taxonomy

Handlers map domain errors to Connect codes through `internal/api/errors.go`. We map our errors into the canonical Connect code set and attach a structured `gohome.error.v1alpha1.ErrorDetail` for the cause chain.

| Connect code | When we use it |
|---|---|
| `INVALID_ARGUMENT` | Malformed request, unknown entity ID format, bad cursor token, unknown capability, invalid Pkl syntax passed to `ConfigService.Validate` |
| `NOT_FOUND` | Entity / device / automation / script / driver ID does not exist |
| `ALREADY_EXISTS` | `ConfigService.Apply` with a slug that collides in strict mode (if we add one); not used in v1 |
| `FAILED_PRECONDITION` | `AutomationService.Trigger` on a disabled automation; `ScriptService.Cancel` on a completed run; `ConfigService.Apply` without a prior successful `Validate` in strict mode |
| `PERMISSION_DENIED` | Authorizer rejected the call. Never returned before C9 (stub authorizer only produces `UNAUTHENTICATED`) |
| `UNAUTHENTICATED` | No / bad credentials. Default-reject authenticator on TCP returns this until C9 |
| `RESOURCE_EXHAUSTED` | Streaming backpressure kick; rate limit (future); page_size over hard cap (not used — we clamp instead) |
| `CANCELED` | Client aborted; we do not synthesize it |
| `DEADLINE_EXCEEDED` | Context deadline hit inside a handler |
| `UNIMPLEMENTED` | `Scene`, `Dashboard`, `Auth` services; `EventService.ReplayAt` |
| `INTERNAL` | Unmapped errors. Logged with stack; not exposed to client beyond a correlation ID |
| `UNAVAILABLE` | Daemon is shutting down; driver instance for a `CallCapability` request is down |

`ErrorDetail` proto:

```proto
// gohome/error/v1alpha1/error.proto
message ErrorDetail {
  // 1-9: classification
  string reason = 1;          // stable constant, e.g. "entity_not_found", "automation_disabled"
  string domain = 2;          // subsystem, e.g. "automation", "eventstore", "carport"

  // 10-19: correlation
  string request_id    = 10;  // always set by the request-id interceptor
  string correlation_id = 11; // present if the originating action produced one (automation run, config apply, ...)

  // 20-29: detail
  map<string, string> metadata = 20; // free-form extra context
}
```

Handler rules:
- Every returned `connect.Error` carries exactly one `ErrorDetail`.
- `reason` is drawn from a constant catalog in `internal/api/errors.go`. New reasons need a catalog entry.
- Internal error messages (stack traces, driver-side errors) NEVER leak to the client; they're logged with `request_id` and the client sees a generic message + the correlation ID.

### 4.5 Streaming

All streaming RPCs are **server-streaming**. No bidi, no client-streaming — the use cases we have (event tail, subscription, automation trace) are one-way firehoses.

**Resume.** Every streaming RPC whose payload maps to the event log takes an optional `from_cursor` (uint64, C1 cursor). Empty means "live from now." When set, the server replays from that cursor forward, seamlessly transitioning into live delivery. Delivery is **at-least-once on the seam**; clients deduplicate by discarding anything with `cursor <= last_seen_cursor`.

**Heartbeats.** Every open stream emits a `Heartbeat` envelope at most every 30 seconds of silence (configurable via `listener.stream_heartbeat_interval` in Pkl). Heartbeats carry the latest cursor even when no real events have fired, so idle clients can keep their resume point current. Heartbeats let us detect dead peers and keep idle streams past HTTP/2 idle timeouts and any intervening middlebox.

**Backpressure.** The server-side fan-out buffer per subscription is bounded (default 10,000 events). If a slow client would cause the buffer to exceed the bound, the server closes the stream with `RESOURCE_EXHAUSTED` + `ErrorDetail.reason = "subscription_overflow"` + `metadata["buffered"]` = the overflow count. The client reconnects with `from_cursor = last_seen_cursor` and resumes. No unbounded server-side buffering, ever.

**Message shape.** Streaming RPCs return a oneof message so heartbeats and payloads share a stream:

```proto
message TailResponse {
  oneof kind {
    Event     event     = 1;
    Heartbeat heartbeat = 2;
  }
}

message Heartbeat {
  uint64 latest_cursor = 1;
  google.protobuf.Timestamp server_time = 2;
}
```

### 4.6 Request IDs, correlation, trace propagation

- **Request ID.** Every incoming request gets a ULID-shaped request ID minted by the `request-id` interceptor if absent, echoed in a response header (`x-request-id`). The ID goes into the `slog` context for the duration of the handler and is the `ErrorDetail.request_id` on failure.
- **Correlation ID.** Actions that originate a run (`AutomationService.Trigger`, `ScriptService.Run`, `ConfigService.Apply`) return the correlation ID they assigned internally, so the caller can watch for the run in `EventService.Tail` or `AutomationService.Trace`.
- **Tracing.** OTel W3C traceparent headers are honored via `internal/observability/tracing.go`. Each handler starts a span named `<service>.<method>`.

### 4.7 Timeouts and deadlines

- No default server-side timeout. Streaming RPCs are long-lived by design; unary RPCs rely on client-supplied deadlines.
- `ConfigService.Apply`'s real-mode RPC is bounded internally by the config engine's apply ceiling (from C4); if the client deadline is shorter, the apply is aborted and rolled back at the config layer.

### 4.8 Backward compatibility

Within `v1alpha1` we may rename fields, renumber carefully following proto-hygiene rules, or drop RPCs between releases **with a migration note committed alongside**. Migration notes go in `docs/` (or `gohome/docs/` if they concern internals). Clients that pin `v1alpha1` read the migration notes; we don't promise seamless upgrades until graduation.

---

## 5. Service Catalog

For each service: proto sketch (not full — the full `.proto` files live in the implementation), handler responsibilities, dependencies, notable validation / error mapping. Live services are fully specified; UNIMPLEMENTED services are called out with the final-shape proto only.

### 5.1 `EntityService` — live

```proto
service EntityService {
  rpc List(ListEntitiesRequest) returns (ListEntitiesResponse);
  rpc Get(GetEntityRequest) returns (GetEntityResponse);
  rpc CallCapability(CallCapabilityRequest) returns (CallCapabilityResponse);
  rpc Subscribe(SubscribeEntitiesRequest) returns (stream SubscribeEntitiesResponse);
}

message ListEntitiesRequest {
  PageRequest page = 1;
  EntitySelector selector = 2;  // optional: filter by areas/zones/classes/device
}

message Entity {
  string id          = 1;  // "light.living_room"
  string type        = 2;  // "light"
  string device_id   = 3;
  string area_id     = 4;
  string zone_id     = 5;
  string friendly_name = 6;
  gohome.entity.v1.Attributes state        = 10;
  gohome.entity.v1.Attributes capabilities = 11;
}
```

- Handler deps: `EntityReader` (list, get), `CapabilityCaller` (call).
- `List` filter `EntitySelector` supports: `entity_ids`, `areas`, `zones`, `classes`, `device_ids`. All fields are sets; empty means "no filter on this axis."
- `CallCapability` validates the target entity exists and the capability is in its capabilities map. On success, a `CommandIssued` event is appended by the `CapabilityCaller`; the RPC returns once the driver has ACK'd (or `UNAVAILABLE` if the driver instance is down).
- `Subscribe` is the unified live view: any `StateChanged`, `EntityRegistered`, or `EntityUnregistered` event mapped through the selector. Cursor-resumable per §4.5.

### 5.2 `DeviceService` — live

```proto
service DeviceService {
  rpc List(ListDevicesRequest) returns (ListDevicesResponse);
  rpc Get(GetDeviceRequest) returns (GetDeviceResponse);
  rpc Rename(RenameDeviceRequest) returns (RenameDeviceResponse);
  rpc Reassign(ReassignDeviceRequest) returns (ReassignDeviceResponse);
}
```

- `Rename` updates the friendly name. Emitted as a `DeviceRenamed` event (new event payload, tag 70 in `gohome.event.v1`).
- `Reassign` moves the device to a different area. Emitted as `DeviceReassigned` (tag 71). Note: "new event block 70-79 — device registry mutations" opened here.
- Handler deps: `RegistryReader`, `RegistryWriter`.

(New event payloads `DeviceRenamed`, `DeviceReassigned` are specified in §7.3 alongside `WebhookReceived`.)

### 5.3 `AreaService`, `ZoneService` — live

Thin read surfaces — `List` and `Get`. Handler deps: `RegistryReader`. No mutation RPCs in v1; areas and zones come from Pkl.

### 5.4 `DriverService` — live

```proto
service DriverService {
  rpc ListDrivers(ListDriversRequest) returns (ListDriversResponse);
  rpc ListInstances(ListInstancesRequest) returns (ListInstancesResponse);
  rpc InstanceHealth(InstanceHealthRequest) returns (InstanceHealthResponse);
  rpc RestartInstance(RestartInstanceRequest) returns (RestartInstanceResponse);
}
```

- `ListDrivers` enumerates installed driver plugins with their manifests (from C2).
- `ListInstances` enumerates configured driver instances with last-known handshake status, uptime, entity count.
- `InstanceHealth` calls the out-of-band Carport `Health` RPC (C2 §6.2) and returns the result.
- `RestartInstance` signals the Carport supervisor to cycle the instance. Emits `DriverInstanceRestarted` event (tag 80, new block 80-89 for driver-control events).
- Handler deps: `DriverControl`.

### 5.5 `EventService` — live (no `ReplayAt`)

```proto
service EventService {
  rpc Query(QueryEventsRequest) returns (QueryEventsResponse);
  rpc Tail (TailEventsRequest)  returns (stream TailEventsResponse);
  // rpc ReplayAt — UNIMPLEMENTED in v1alpha1; see §1.2.
}
```

- `Query` is history; takes an `EventFilter` (kinds, entity prefix, source, time range, cursor range) and a `PageRequest`. Results sorted by cursor ascending.
- `Tail` is live + optional resume. `from_cursor` as per §4.5. The returned stream multiplexes `Event` payloads and `Heartbeat`s.
- Handler deps: `EventSource` (wraps `eventstore.Store`).

### 5.6 `SceneService` — UNIMPLEMENTED

Protos finalized; handlers return `connect.CodeUnimplemented`. Scene implementation owns its own spec.

### 5.7 `AutomationService` — live

```proto
service AutomationService {
  rpc List    (ListAutomationsRequest)    returns (ListAutomationsResponse);
  rpc Get     (GetAutomationRequest)      returns (GetAutomationResponse);
  rpc Enable  (SetAutomationEnabledRequest) returns (SetAutomationEnabledResponse);
  rpc Disable (SetAutomationEnabledRequest) returns (SetAutomationEnabledResponse);
  rpc Trigger (TriggerAutomationRequest)  returns (TriggerAutomationResponse);
  rpc Trace   (TraceAutomationRequest)    returns (stream TraceAutomationResponse);
}
```

- `Enable`/`Disable` wrap C6's in-memory override (non-durable per C6 §1.1). Clients that need durable enable state rewrite the Pkl.
- `Trigger` kicks the automation manually; returns `run_id`. `FAILED_PRECONDITION` if disabled.
- `Trace` is a server-stream of the automation's run trace events (C6 already defines the trace events shape). Supports `from_cursor` like `Tail`.
- Handler deps: `AutomationControl`.

### 5.8 `ScriptService` — live

```proto
service ScriptService {
  rpc List  (ListScriptsRequest)  returns (ListScriptsResponse);
  rpc Run   (RunScriptRequest)    returns (RunScriptResponse);
  rpc Cancel(CancelScriptRequest) returns (CancelScriptResponse);
}
```

- `Run` is synchronous: the RPC blocks until the script returns. Returns the script result JSON plus the `run_id` (correlation ID) for log cross-reference. If the client deadline expires first, the handler cancels the Starlark run via context; the run is marked aborted.
- `Cancel` is best-effort: it signals the script's `context.Context` cancel. Classification: `FAILED_PRECONDITION` if the script has already completed; otherwise returns immediately and the caller tails `EventService` for `ScriptFinished{outcome: canceled}`.
- Handler deps: `ScriptRunner`.

### 5.9 `ConfigService` — live

```proto
service ConfigService {
  rpc Validate   (ValidateConfigRequest)   returns (ValidateConfigResponse);
  rpc Apply      (ApplyConfigRequest)      returns (ApplyConfigResponse);
  rpc Reload     (ReloadConfigRequest)     returns (ReloadConfigResponse);
  rpc GetArtifact(GetConfigArtifactRequest) returns (GetConfigArtifactResponse);
}

message ApplyConfigRequest {
  // 1-9: payload
  bytes pkl_bundle = 1;   // tarball of pkl files, or full bundle from UI/CLI
  string message   = 2;   // free-form commit message (audit trail)

  // 10-19: mode
  bool dry_run = 10;      // validate + diff, do not persist or reload
  bool strict  = 11;      // require a prior successful Validate with the same hash
}
```

- `Validate` runs the full Pkl evaluator pipeline from C4, returns the diff without applying.
- `Apply` with `dry_run=true` is equivalent to `Validate` + diff. With `dry_run=false`, the bundle is written to the config dir, the engine reloads (surgical diff per C4/C6), and a `ConfigApplied` event is appended.
- `Reload` re-reads the current on-disk config — for when someone edited files directly. Same hooks fire; surgical reload applies.
- `GetArtifact` returns the current `ConfigSnapshot` (from C4) for read-only tools (web UI config viewer).
- Handler deps: `ConfigApplier`.

### 5.10 `DashboardService` — UNIMPLEMENTED

Protos finalized from master §7.3; handlers `UNIMPLEMENTED`.

### 5.11 `AuthService` — UNIMPLEMENTED

Protos finalized; handlers `UNIMPLEMENTED`. C9 ships the real impl.

### 5.12 `SystemService` — live

```proto
service SystemService {
  rpc Version    (VersionRequest)     returns (VersionResponse);
  rpc Health     (HealthRequest)      returns (HealthResponse);
  rpc Metrics    (MetricsRequest)     returns (MetricsResponse);
  rpc Diagnostics(DiagnosticsRequest) returns (DiagnosticsResponse);
}
```

- `Version` returns binary version, build metadata, schema version.
- `Health` returns per-subsystem health: eventstore, registry, each driver instance, automation engine. Readable summary.
- `Metrics` returns Prometheus exposition format as a string field (duplication of the existing `/metrics` scrape endpoint; here for UI).
- `Diagnostics` returns a bundle (recent logs, goroutine dump, config hash) as an archive bytes field. Size-capped at 10 MiB; larger means the UI downloads a file, not an RPC response.
- `Health` is reachable pre-auth when requested on the `/healthz` HTTP path (see §7.3); the Connect RPC requires auth.

---

## 6. Webhooks

C6 shipped `WebhookTrigger{slug, secret, ...}` Pkl-validated but inert. C7 adds the HTTP receiver and the event that drives the trigger.

### 6.1 HTTP endpoint

- Route: `POST /webhooks/{slug}` on the same `http.Server` / `ServeMux` as Connect.
- Mounted by the automation engine at reload time: on `ConfigApplied`, the engine tells the webhook router which slugs are currently active. Unknown slugs → `404 Not Found`.
- Auth is **not** the standard auth interceptor. Webhooks are externally-originated; they authenticate via a per-slug HMAC-SHA256 signature header matched against the Pkl-declared `secret`.
- Missing / invalid signature → `401 Unauthorized`. Malformed body (non-UTF-8 headers, body size over 1 MiB default cap) → `400 Bad Request`.
- Success response: `202 Accepted`, empty body. Webhook triggers are fire-and-forget from the caller's perspective.

### 6.2 Signature scheme

- Header: `X-GoHome-Signature: v1=<hex-hmac-sha256>`.
- HMAC computed over raw body bytes with the secret as key. Comparison is constant-time.
- Header name and versioning prefix are documented in the webhook trigger's Pkl module docstring.

### 6.3 `WebhookReceived` event

New event payload in `gohome.event.v1`, slotted into a new range block **60-69 "external ingress"**, starting at tag 60:

```proto
// gohome/event/v1/event.proto
message Payload {
  oneof kind {
    // ... existing 1-53 ...
    // 60-69: external ingress
    WebhookReceived webhook_received = 60;
    // 70-79: device registry mutations
    DeviceRenamed     device_renamed      = 70;
    DeviceReassigned  device_reassigned   = 71;
    // 80-89: driver control
    DriverInstanceRestarted driver_instance_restarted = 80;
  }
}

message WebhookReceived {
  // 1-9: identity
  string slug = 1;

  // 10-19: payload
  bytes               body    = 10;   // raw body bytes, up to configured max
  map<string, string> headers = 11;   // selected headers: content-type, user-agent, x-forwarded-for (lowercased keys)

  // 20-29: source
  string source_ip = 20;               // peer IP (or X-Forwarded-For first hop if behind the Pkl-configured trusted proxy)
}
```

The webhook handler appends `WebhookReceived` to the event store before responding `202`. C6's `WebhookTrigger` matcher subscribes to this payload type and fires the corresponding automation.

### 6.4 Config

New `listener.webhooks` block in `gohome.core.pkl`:

```pkl
listener {
  webhooks {
    max_body_bytes = 1048576    // 1 MiB default
    trusted_proxies = List()    // CIDRs; if peer matches, trust X-Forwarded-For
  }
}
```

### 6.5 Security notes

- Per-webhook secret is a Pkl-level secret (C4's secret-source indirection). If no `secret` is declared in the trigger, the webhook is rejected by config validation — we do not ship unauthenticated webhooks.
- There is no rate limiting in v1. Operators concerned about webhook DoS put a rate limiter in front via their reverse proxy (or bind the TCP listener to a private interface and tunnel).

---

## 7. Auth Seam

C7 defines the contract. C9 provides the implementation.

### 7.1 Interfaces

```go
// internal/auth/auth.go
type Principal struct {
    ID          string            // "user:fdatoo", "token:xyz", "system:local"
    DisplayName string
    Kind        string            // "user", "token", "system"
    Metadata    map[string]string // free-form; roles, audit bits
}

type Authenticator interface {
    // Authenticate inspects the request and returns a Principal, or an error
    // mapped by the caller to UNAUTHENTICATED.
    Authenticate(ctx context.Context, req AuthRequest) (Principal, error)
}

type AuthRequest struct {
    Scheme      string            // "uds:peercred", "bearer", "cookie", ...
    Headers     http.Header
    PeerCred    *syscall.Ucred    // set only when Scheme == "uds:peercred"
    RemoteAddr  string
    Method      string            // connect method: "/gohome.v1alpha1.EntityService/List"
}

type Authorizer interface {
    // Authorize decides whether Principal may perform Action on Target.
    Authorize(ctx context.Context, p Principal, action Action, target Target) error
}

type Action struct {
    Service string  // "EntityService"
    Method  string  // "CallCapability"
    Verb    string  // canonical: "read" | "write" | "call" | "admin"
}

type Target struct {
    Kind string            // "entity" | "automation" | "script" | "config" | "driver" | ""
    ID   string            // dotted path / slug / empty
    Attr map[string]string // extra info used by policy
}
```

### 7.2 Stub implementations for v1

- `internal/auth/local.go`: `LocalPeerCredAuthenticator`.
  - On UDS requests (`Scheme == "uds:peercred"`), returns `Principal{ID: "system:local", Kind: "system", DisplayName: "local"}`.
  - On TCP requests, returns `ErrNotApplicable` — a chained-authenticator signal meaning "try the next one."
- `internal/auth/reject.go`: `RejectAllAuthenticator`. Always returns `ErrUnauthenticated`. Used as the fallback behind `LocalPeerCredAuthenticator` until C9 installs a real chain.
- `internal/auth/allow.go`: `AllowAllAuthorizer`. The C7 stub authorizer accepts every `Authorize` call. C9 replaces with a policy-backed one.

**Concretely** in the daemon:

```go
// internal/daemon/daemon.go (sketch)
authenticator := auth.Chain(
    auth.LocalPeerCred{},
    auth.RejectAll{}, // TCP dead-ends here until C9
)
authorizer := auth.AllowAll{}
```

### 7.3 Interceptor behavior

- On each request, the `authenticate` interceptor builds `AuthRequest`, calls `authenticator.Authenticate(ctx, req)`, and attaches the resulting `Principal` to the context via `auth.WithPrincipal(ctx, p)`.
- Failure → `connect.CodeUnauthenticated` + `ErrorDetail.reason = "unauthenticated"`.
- The `authorize` interceptor reads the `Principal` from context, looks up the action/target from a per-method table in `internal/api/actions.go`, calls `authorizer.Authorize`.
- Failure → `connect.CodePermissionDenied` + `ErrorDetail.reason = "forbidden"`.
- The `/healthz` endpoint and the webhook handler bypass the auth interceptor chain entirely; they live on separate mux paths.

### 7.4 Action catalog

Each service defines a table mapping RPC method name → `Action{Verb, ...}`. Example:

```go
// internal/api/service_entity.go
var entityActions = map[string]auth.Action{
    "List":           {Service: "EntityService", Method: "List",           Verb: "read"},
    "Get":            {Service: "EntityService", Method: "Get",            Verb: "read"},
    "CallCapability": {Service: "EntityService", Method: "CallCapability", Verb: "call"},
    "Subscribe":      {Service: "EntityService", Method: "Subscribe",      Verb: "read"},
}
```

`Target` is computed per-call from the request message (entity ID for `Get`/`CallCapability`/`Subscribe`; `{Kind: "entity", ID: ""}` for `List`). The authorize interceptor dispatches to a per-method "target extractor" registered alongside the action.

---

## 8. Listener, Transport, and Interceptors

### 8.1 Listener wiring

```go
// internal/api/listener/listener.go
func Build(deps Deps) (*Listener, error) {
    mux := http.NewServeMux()

    // Connect handlers, all three protocols via connect-go's default config.
    path, handler := entityv1alpha1connect.NewEntityServiceHandler(deps.EntityService, interceptors...)
    mux.Handle(path, handler)
    // ... one per service ...

    mux.Handle("/webhooks/", deps.WebhookHandler)
    mux.HandleFunc("/healthz", livenessHandler(deps.HealthProbe))

    // Plaintext HTTP/2 (h2c) so the TCP listener speaks both HTTP/1.1 and HTTP/2
    // without TLS.
    h2s := &http2.Server{}
    httpsrv := &http.Server{
        Handler: h2c.NewHandler(mux, h2s),
    }

    return &Listener{httpSrv: httpsrv, ...}, nil
}
```

Two `net.Listener`s are created:
- **UDS**: `net.Listen("unix", cfg.Listener.UDS.Path)`. After binding, `os.Chmod(path, cfg.Listener.UDS.Mode)`. On shutdown, remove the socket file (best-effort, log on failure).
- **TCP**: `net.Listen("tcp", cfg.Listener.TCP.Bind)`. If `cfg.Listener.TCP.TLS.CertFile != ""`, wrap with `tls.NewListener(..., loadTLSConfig(cfg))`.

Both listeners call `httpsrv.Serve(l)` in separate goroutines. `Shutdown(ctx)` drains both.

### 8.2 Interceptor stack

Connect interceptors are composed outermost-first in `internal/api/listener/interceptors.go`. Order (outermost → innermost):

1. **recover** — catches panics, logs, returns `INTERNAL` with `ErrorDetail.reason = "panic"`.
2. **request-id** — mints / echoes `x-request-id`, seeds slog.
3. **slog** — `slog.InfoContext` at entry/exit with method, request-id, status, duration.
4. **metrics** — Prometheus counters and histograms per `service/method/status`.
5. **tracing** — OTel span per RPC. Uses `internal/observability/tracing.go`.
6. **authenticate** — `AuthRequest` → `Principal` → context.
7. **authorize** — `Principal` + action table → allow / deny.

Webhook and `/healthz` handlers bypass this stack (they're non-Connect routes on the mux) and use their own minimal instrumentation: request-id, slog, metrics, recover — no auth.

### 8.3 Shutdown

On `SIGTERM` / context cancel:

1. Close the TCP listener first (stop accepting new connections).
2. `httpSrv.Shutdown(ctx)` with a 10s grace deadline — existing unary RPCs drain, streaming RPCs receive a `UNAVAILABLE` close + `ErrorDetail.reason = "shutdown"`.
3. Close the UDS listener, `os.Remove` the socket file.
4. Daemon continues its own shutdown (engines, storage).

Streaming clients handle `UNAVAILABLE + shutdown` by reconnecting once the daemon comes back up.

### 8.4 Configuration

Added to `gohome.core.pkl` (C4's core config module):

```pkl
module gohome.core

class Listener {
  uds: UDSListener = new UDSListener {}
  tcp: TCPListener = new TCPListener {}
  webhooks: WebhookConfig = new WebhookConfig {}
  stream_heartbeat_interval: Duration = 30.s
}

class UDSListener {
  path: String = "@data/gohomed.sock"
  mode: UInt  = 0o600
}

class TCPListener {
  bind: String = "127.0.0.1:8080"
  tls: TLSConfig? = null
}

class TLSConfig {
  cert_file: String
  key_file:  String
}

class WebhookConfig {
  max_body_bytes: UInt = 1048576
  trusted_proxies: List<String> = List()
}
```

---

## 9. CLI Rewrite

### 9.1 Scope

Every existing `gohome` subcommand is ported from `internal/cli/cliutil.go`'s `sendReq`/JSON-op shape to a generated Connect-Go client. `internal/daemon/recovery.go`'s JSON-op switch is deleted. `cliutil.go` gains a `Dialer()` helper that builds a Connect client over UDS by default or TCP when `--endpoint=tcp://...` is supplied.

### 9.2 Command → RPC map

| CLI command | Current JSON op | Replacement RPC |
|---|---|---|
| `gohome automation list` | `automation_list` | `AutomationService.List` |
| `gohome automation get <id>` | `automation_get` | `AutomationService.Get` |
| `gohome automation enable <id>` | `automation_enable` | `AutomationService.Enable` |
| `gohome automation disable <id>` | `automation_disable` | `AutomationService.Disable` |
| `gohome automation trigger <id>` | `automation_trigger` | `AutomationService.Trigger` |
| `gohome automation trace <run-id>` | `automation_trace` (stream) | `AutomationService.Trace` (server-stream) |
| `gohome automation watch` | (tails events) | `EventService.Tail` with filter |
| `gohome script list` | `script_list` | `ScriptService.List` |
| `gohome script run <name> [args]` | `script_run` | `ScriptService.Run` |
| `gohome snapshot create` | `snapshot` | `SystemService.Diagnostics` (snapshots are system-level) **or** a dedicated `AdminService.CreateSnapshot` RPC — see §9.3 |
| `gohome driver restart <instance>` | `driver_restart` | `DriverService.RestartInstance` |
| `gohome eval` | `starlark_eval` | `ScriptService.Run` on an anonymous script, OR an explicit `ScriptService.Eval` — see §9.3 |
| `gohome test` | `starlark_test` | unchanged shape; handled by `ScriptService.Run` variant — see §9.3 |
| `gohome state get <id>` | (state read) | `EntityService.Get` |
| `gohome events tail` | (event tail) | `EventService.Tail` |
| `gohome config validate <path>` | (config validate) | `ConfigService.Validate` |
| `gohome config apply <path>` | (config apply) | `ConfigService.Apply` |

### 9.3 Three ops that need a home: `snapshot`, `eval`, `test`

These were daemon-local diagnostics exposed on the UDS. They don't cleanly belong to any user-facing service. C7's resolution:

- **`snapshot`** — creates a projector snapshot per C1 §5.5. Lives in `SystemService` as `SystemService.CreateSnapshot(CreateSnapshotRequest) returns (CreateSnapshotResponse)`. The request takes `owner` and `reason` strings.
- **`starlark_eval`** — ad-hoc Starlark expression evaluation. Lives in `ScriptService` as `ScriptService.Eval(EvalScriptRequest) returns (EvalScriptResponse)`. Body is a Starlark expression string; response is the serialized result or a trace on error. This is the v1 predecessor to the MCP tool `gohome__eval_starlark` from master §7.2.
- **`starlark_test`** — runs the Starlark test harness against a named test file. Lives in `ScriptService.RunTests(RunStarlarkTestsRequest) returns (stream RunStarlarkTestsResponse)` — server-streams test outcomes so the CLI can render progress.

These RPCs are marked in their proto comments as **"internal-flavor"** — intended to be exposed over UDS and tightly policy-gated over TCP once C9 lands.

### 9.4 Rendering

Output rendering stays as-is — the existing `internal/cli/styles*.go` and `render.go` already do the lipgloss styling. They get *typed* protobuf messages instead of `map[string]any`, which makes them cleaner to maintain. Per user's feedback memory ("CLI output should be well-styled via lipgloss"), each command's output styling is preserved verbatim; only the data source changes.

### 9.5 Endpoint selection

```
gohome [--endpoint <uri>] <command> [args]
```

- Default: `unix://@data/gohomed.sock` (resolved against `XDG_DATA_HOME`/the daemon's data dir).
- Alternatives: `tcp://127.0.0.1:8080`, `https://gohome.my.local:8443`.
- `GOHOME_ENDPOINT` env var overrides the default.

---

## 10. Observability

### 10.1 Metrics

Registered in `internal/observability/metrics.go`, minted by the metrics interceptor:

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `gohome_api_requests_total` | counter | `service`, `method`, `code`, `transport` (`uds`/`tcp`) | completed RPC count |
| `gohome_api_request_duration_seconds` | histogram | `service`, `method`, `code` | latency |
| `gohome_api_stream_events_sent_total` | counter | `service`, `method` | streaming events emitted to clients (not heartbeats) |
| `gohome_api_stream_heartbeats_sent_total` | counter | `service`, `method` | heartbeats emitted |
| `gohome_api_stream_backpressure_closes_total` | counter | `service`, `method` | streams closed with `RESOURCE_EXHAUSTED` |
| `gohome_api_webhook_received_total` | counter | `slug`, `result` (`accepted`/`bad_signature`/`too_large`/`unknown_slug`) | webhook outcomes |
| `gohome_api_active_streams` | gauge | `service`, `method` | currently-open server streams |

### 10.2 Logging

`slog` interceptor records per-request: `request_id`, `principal_id` (after auth), `service`, `method`, `transport`, `status`, `duration_ms`. Streams log once at open and once at close.

### 10.3 Tracing

OTel span per RPC. Span name = `gohome.v1alpha1.<Service>/<Method>`. Streams are a single span covering the stream's lifetime.

---

## 11. Testing Strategy

Three layers.

### 11.1 Handler unit tests

Each `service_*.go` file has a `service_*_test.go` in the same package using fake implementations of the `deps.go` interfaces. Tests are table-driven, one row per (valid request, error case, edge case). Error-mapping is tested exhaustively: every `(engine error, expected Connect code, expected ErrorDetail.reason)` triple is a row.

### 11.2 Interceptor tests

Each interceptor tested in isolation: auth on UDS path, auth on TCP path, request-id echo, slog output capture, metrics emission, panic recovery. Composed-stack tests assert that a panic in the handler doesn't break request-id or metrics.

### 11.3 Integration tests (`//go:build integration`)

A fixture daemon (from `internal/testutil`) is started with an in-memory SQLite and a fake Carport driver. Tests:

- Spin up the daemon, dial UDS, run CLI-parity tests (run `AutomationService.List` and assert against a golden fixture).
- Stream test: subscribe, inject events via the fake driver, assert the client sees them in order; kill the stream, reconnect with `from_cursor`, assert resume works.
- Heartbeat test: with no events, assert a heartbeat arrives within heartbeat_interval * 1.5.
- Backpressure test: configure a 10-event buffer; inject 100 events without reading; assert the stream closes with `RESOURCE_EXHAUSTED`.
- Webhook test: `POST /webhooks/foo` with a valid HMAC; assert a `WebhookReceived` event lands in the store and the Pkl-declared automation fires.
- Webhook auth test: bad signature → `401`, no event.
- Auth-seam test: TCP request without creds → `UNAUTHENTICATED`. UDS request → succeeds as `system:local`.

### 11.4 CLI integration tests

`task test:integration` runs the `gohome` CLI against a real daemon and compares stdout against golden fixtures. Covers every command in §9.2. Replaces the current JSON-op integration path.

---

## 12. Implementation Order

Suggested sequencing (detailed in the C7 implementation plan):

1. Proto sources + generated code for `common` and one service (`SystemService`) — proves the Buf pipeline.
2. `internal/auth` stub.
3. `internal/api/listener` skeleton, UDS + TCP + `/healthz`.
4. `SystemService` handler + integration test.
5. Remaining live services in dependency order: `Entity` → `Device` / `Area` / `Zone` → `Event` → `Config` → `Driver` → `Automation` → `Script`.
6. Streaming RPCs (`Entity.Subscribe`, `Event.Tail`, `Automation.Trace`) + heartbeat/backpressure.
7. Webhook endpoint + new event payload + C6 `WebhookTrigger` matcher activation.
8. `UNIMPLEMENTED` stubs for `Scene`, `Dashboard`, `Auth`.
9. CLI rewrite, command by command.
10. Deletion of `internal/daemon/recovery.go`'s JSON-op switch; integration suite green.

---

## 13. Decision Record

| # | Decision | Rationale |
|---|---|---|
| D1 | Full IDL, partial impl | C8 / C10 code against the final wire surface from day one; `UNIMPLEMENTED` is cheap; scene/dashboard/auth backends are substantial enough to earn their own specs. |
| D2 | Ship as `gohome.v1alpha1` | External consumers will pin; breakage cost of `v1` mistakes is higher than the one-time migration cost of `alpha1 → v1` later. Graduation after C8 + C10 stress-test the surface. |
| D3 | Connect-RPC on both UDS and TCP; retire JSON-op surface | One wire protocol, two transports. UDS authenticates via peer-cred; TCP via C9 credentials. Two surfaces was always scaffolding. |
| D4 | Real `Authenticator`/`Authorizer` interfaces + stub impl | Handler code is written once, against the seam. C9 swaps the stub without touching handlers. |
| D5 | Connect + gRPC + gRPC-Web on the same handlers | One line of mux config. Lets standard gRPC ecosystem (grpcurl, Rust `tonic`, Python `grpc`) work without adapters. Connect's protocol remains the recommended path for first-party clients. |
| D6 | Webhooks live in C7 | C7 is the HTTP surface owner. C6 already deferred here. The receiver is small; not worth a separate spec. |
| D7 | Streams resumable via `from_cursor`, at-least-once on seam | Matches C1's cursor semantics. Heartbeats every 30s. Server-side fan-out bounded at 10k events; overflow closes with `RESOURCE_EXHAUSTED`. |
| D8 | Plaintext on `127.0.0.1` default; operator-supplied cert via Pkl only | ACME, self-signed, mTLS are all their own feature area (C13). v1 defers cert lifecycle cleanly. |
| D9 | CLI port happens inside C7 | Preserves "one wire surface" invariant. The port is mechanical; typed clients are strictly better to build render code against. |
| D10 | Thin handlers in `internal/api/`, backends untouched | Matches Go convention for glue layers. Keeps C1-C6 packages as bounded contexts. Interfaces make handler tests trivial. |
| D11 | `CreateSnapshot`, `Eval`, `RunTests` live on `SystemService` / `ScriptService` | These are internal diagnostics; "admin / diagnostic" flavor documented in proto. Alternative — a separate `AdminService` — rejected to avoid a second service with its own auth semantics. |

---

## 14. Explicit Deferrals

- **Auth implementation (C9)**: passkeys, OIDC, password, API tokens, policy compilation.
- **TLS lifecycle (C13)**: ACME, self-signed generation, cert rotation, mTLS.
- **Scene engine**: own spec (Cn).
- **Dashboard rendering and persistence**: own spec (C10 adjacent).
- **`EventService.ReplayAt`**: requires point-in-time projector rebuild; small follow-up once a consumer needs it.
- **Generated client publishing** (PyPI, npm): C13.
- **Webhook rate limiting**: operator responsibility via reverse proxy in v1.
- **Durable automation enable/disable overrides**: C6 §1.1 already deferred; stays deferred.
- **Admin service / separate admin transport**: the "internal-flavor" RPCs live on existing services for v1.

---

*End of C7 design document.*
