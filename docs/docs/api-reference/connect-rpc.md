# Connect-RPC API Reference

!!! status-alpha "Alpha — shipped, interface evolving"

All switchyard services are served under the `switchyard.v1alpha1.*` protobuf package over [Connect-RPC](https://connectrpc.com/). The same handlers speak Connect, gRPC, and gRPC-Web on both the Unix domain socket (UDS) and the TCP listener.

**Base URL (default):** `http://127.0.0.1:8080` (TCP) or `unix://$SWITCHYARD_DATA_DIR/switchyardd.sock` (UDS)

**Package prefix:** `switchyard.v1alpha1`

---

## Service overview

| # | Service | RPCs | Status |
|---|---------|------|--------|
| 1 | [`EntityService`](#entityservice) | `List`, `Get`, `CallCapability`, `Subscribe` | Live |
| 2 | [`DeviceService`](#deviceservice) | `List`, `Get`, `Rename`, `Reassign` | Live |
| 3 | [`AreaService`](#areaservice) | `List`, `Get` | Live |
| 4 | [`ZoneService`](#zoneservice) | `List`, `Get` | Live |
| 5 | [`DriverService`](#driverservice) | `ListDrivers`, `ListInstances`, `InstanceHealth`, `RestartInstance` | Live |
| 6 | [`EventService`](#eventservice) | `Query`, `Tail` | Live |
| 7 | [`SceneService`](#sceneservice) | `List`, `Apply`, `Preview` | UNIMPLEMENTED |
| 8 | [`AutomationService`](#automationservice) | `List`, `Get`, `Enable`, `Disable`, `Trigger`, `Trace` | Live |
| 9 | [`ScriptService`](#scriptservice) | `List`, `Run`, `Cancel`, `Eval` | Live |
| 10 | [`ConfigService`](#configservice) | `Validate`, `Apply`, `Reload`, `GetArtifact` | Live |
| 11 | [`DashboardService`](#dashboardservice) | `List`, `Get`, `SaveLayout` | UNIMPLEMENTED |
| 12 | [`AuthService`](#authservice) | `Login`, `Logout`, `CurrentUser`, `CreateToken`, `RevokeToken`, `ListUsers`, `RegisterPasskey`, `StartWebAuthnChallenge` | UNIMPLEMENTED |
| 13 | [`SystemService`](#systemservice) | `Version`, `Health`, `Metrics`, `Diagnostics`, `CreateSnapshot`, `GetConfigDir`, `RecordConfigFileEdit`, `GetMCPConfig` | Live |

---

## Wire conventions

### Pagination

Every `List*` RPC accepts a `PageRequest` and returns a `PageResponse` alongside its results.

```proto
message PageRequest {
  uint32 page_size  = 1;  // max 1000; default 100 if 0
  string page_token = 2;  // opaque; empty = first page
}

message PageResponse {
  string next_page_token = 1;  // empty = no more pages
  uint32 total_size      = 2;  // 0 = unknown
}
```

- `page_token` is opaque. Never parse it; pass it back verbatim.
- `page_size > 1000` is silently clamped to 1000.
- Stable default ordering: registry-backed lists sort by slug ascending; event queries sort by cursor ascending.

### Streaming

All streaming RPCs are **server-streaming** only. Streaming responses carry a `oneof kind` so heartbeats and payloads share the stream:

```proto
message TailEventsResponse {
  oneof kind {
    Event     event     = 1;
    Heartbeat heartbeat = 2;
  }
}

message Heartbeat {
  uint64                    latest_cursor = 1;
  google.protobuf.Timestamp server_time   = 2;
}
```

- **Heartbeats** are emitted at most every 30 seconds of silence (configurable in Pkl via `listener.stream_heartbeat_interval`). They carry the latest cursor so idle clients can keep their resume point current.
- **Resume.** Every streaming RPC that maps to the event log accepts an optional `from_cursor` (uint64). Empty means "live from now." When set, the server replays from that cursor and then transitions into live delivery. Delivery is at-least-once on the seam; clients should discard anything with `cursor <= last_seen_cursor`.
- **Backpressure.** The per-subscription fan-out buffer is bounded at 10,000 events. On overflow the stream closes with `RESOURCE_EXHAUSTED` + `ErrorDetail.reason = "subscription_overflow"`. Reconnect with `from_cursor = last_seen_cursor`.

### Error model

Every error carries a structured `switchyard.error.v1alpha1.ErrorDetail` in the Connect trailer:

```proto
message ErrorDetail {
  string reason         = 1;   // stable constant, e.g. "entity_not_found"
  string domain         = 2;   // subsystem, e.g. "eventstore"
  string request_id     = 10;  // always set
  string correlation_id = 11;  // set for action-originated errors
  map<string, string> metadata = 20;
}
```

| Connect code | When used |
|---|---|
| `INVALID_ARGUMENT` | Malformed request, unknown entity ID format, bad cursor, invalid Pkl syntax |
| `NOT_FOUND` | Entity, automation, script, or driver ID does not exist |
| `FAILED_PRECONDITION` | `Trigger` on a disabled automation; `Cancel` on a completed script run |
| `PERMISSION_DENIED` | Authorizer rejected the call (post-C9) |
| `UNAUTHENTICATED` | No or invalid credentials on the TCP listener |
| `RESOURCE_EXHAUSTED` | Streaming backpressure overflow |
| `UNIMPLEMENTED` | `SceneService`, `DashboardService`, `AuthService`; `EventService.ReplayAt` |
| `UNAVAILABLE` | Daemon shutting down; driver instance is down |
| `INTERNAL` | Unmapped internal error. Correlation ID in `ErrorDetail`; details are logged server-side only. |

### Correlation IDs

RPCs that originate a run (`AutomationService.Trigger`, `ScriptService.Run`, `ConfigService.Apply`) return a `correlation_id` in the response. Use it to filter `EventService.Tail` or `AutomationService.Trace` for that run's events.

### Request IDs

Every request receives a ULID-shaped `x-request-id` response header. On error this ID appears in `ErrorDetail.request_id` so you can correlate client-side errors with daemon logs.

---

## EntityService

Live. Backed by the registry and state cache.

```proto
service EntityService {
  rpc List           (ListEntitiesRequest)       returns (ListEntitiesResponse);
  rpc Get            (GetEntityRequest)          returns (GetEntityResponse);
  rpc CallCapability (CallCapabilityRequest)     returns (CallCapabilityResponse);
  rpc Subscribe      (SubscribeEntitiesRequest)  returns (stream SubscribeEntitiesResponse);
}
```

### `List`

Returns a paginated list of entities. Supports filtering via `EntitySelector`.

**Request**

| Field | Type | Description |
|-------|------|-------------|
| `page` | `PageRequest` | Pagination |
| `selector` | `EntitySelector` | Optional filter: `entity_ids`, `areas`, `zones`, `classes`, `device_ids` |

**Response**

| Field | Type | Description |
|-------|------|-------------|
| `entities` | `repeated Entity` | Matching entities |
| `page` | `PageResponse` | Next-page token |

### `Get`

Returns a single entity by its dotted ID.

**Error:** `NOT_FOUND` if the entity ID does not exist.

### `CallCapability`

Invokes a capability on an entity. Blocks until the driver ACKs or returns `UNAVAILABLE` if the driver instance is down. On success, a `CommandIssued` event is appended.

**Request**

| Field | Type | Description |
|-------|------|-------------|
| `entity_id` | `string` | Target entity, e.g. `"light.living_room"` |
| `capability` | `string` | Capability name, e.g. `"turn_on"` |
| `params` | `map<string, string>` | Capability parameters |

**Errors:** `NOT_FOUND` (entity), `INVALID_ARGUMENT` (unknown capability), `UNAVAILABLE` (driver down).

### `Subscribe`

Server-streaming. Emits `StateChanged`, `EntityRegistered`, and `EntityUnregistered` events filtered by `EntitySelector`. Resumable via `from_cursor`.

---

## DeviceService

Live. Read/rename/reassign physical devices.

```proto
service DeviceService {
  rpc List     (ListDevicesRequest)    returns (ListDevicesResponse);
  rpc Get      (GetDeviceRequest)      returns (GetDeviceResponse);
  rpc Rename   (RenameDeviceRequest)   returns (RenameDeviceResponse);
  rpc Reassign (ReassignDeviceRequest) returns (ReassignDeviceResponse);
}
```

- `Rename` — updates the device's friendly name; emits `DeviceRenamed` event.
- `Reassign` — moves the device to a different area; emits `DeviceReassigned` event.

---

## AreaService

Live. Read-only. Areas are sourced from Pkl config; no mutation RPCs.

```proto
service AreaService {
  rpc List (ListAreasRequest) returns (ListAreasResponse);
  rpc Get  (GetAreaRequest)   returns (GetAreaResponse);
}
```

---

## ZoneService

Live. Read-only. Zones are sourced from Pkl config; no mutation RPCs.

```proto
service ZoneService {
  rpc List (ListZonesRequest) returns (ListZonesResponse);
  rpc Get  (GetZoneRequest)   returns (GetZoneResponse);
}
```

---

## DriverService

Live. Backed by the Carport driver supervisor.

```proto
service DriverService {
  rpc ListDrivers     (ListDriversRequest)     returns (ListDriversResponse);
  rpc ListInstances   (ListInstancesRequest)   returns (ListInstancesResponse);
  rpc InstanceHealth  (InstanceHealthRequest)  returns (InstanceHealthResponse);
  rpc RestartInstance (RestartInstanceRequest) returns (RestartInstanceResponse);
}
```

### `ListDrivers`

Enumerates installed driver plugins and their manifests.

### `ListInstances`

Enumerates configured driver instances with:

| Field | Description |
|-------|-------------|
| `id` | Instance ID |
| `driver_name` | Driver plugin name |
| `status` | Last-known status string |
| `entity_count` | Number of entities managed by this instance |
| `last_handshake` | Timestamp of last successful Carport handshake |

### `InstanceHealth`

Calls the Carport out-of-band `Health` RPC for the given instance. Returns `ok: bool` and a `detail` string.

**Error:** `NOT_FOUND` if the instance ID does not exist.

### `RestartInstance`

Signals the Carport supervisor to cycle the instance. Returns immediately; the restart is asynchronous. Emits `DriverInstanceRestarted` event.

**Request**

| Field | Default | Description |
|-------|---------|-------------|
| `instance_id` | — | Target instance |
| `reason` | `"manual"` | Reason recorded in the event |

---

## EventService

Live. `ReplayAt` is deferred (UNIMPLEMENTED).

```proto
service EventService {
  rpc Query (QueryEventsRequest)  returns (QueryEventsResponse);
  rpc Tail  (TailEventsRequest)   returns (stream TailEventsResponse);
  // rpc ReplayAt — UNIMPLEMENTED
}
```

### `Query`

Historical query. Accepts an `EventFilter`:

| Field | Description |
|-------|-------------|
| `kinds` | `repeated string` — event kinds to include; empty = all |
| `entity_prefix` | String prefix match on the `entity` column |
| `sources` | `repeated string` — filter to specific source strings |
| `from_cursor` | Minimum cursor (exclusive) |
| `to_cursor` | Maximum cursor (inclusive); `0` = unbounded |
| `from_time` | Minimum timestamp |
| `to_time` | Maximum timestamp |

Results are sorted by cursor ascending. Pagination via `PageRequest`.

### `Tail`

Server-streaming. Live event stream with optional cursor-based resume. Multiplexes `Event` payloads and `Heartbeat`s. Same `EventFilter` as `Query`. Set `from_cursor` to replay from a past point and then continue live.

---

## SceneService

**UNIMPLEMENTED.** Proto shape is final; handlers return `connect.CodeUnimplemented`.

```proto
service SceneService {
  rpc List    (ListScenesRequest)   returns (ListScenesResponse);
  rpc Apply   (ApplySceneRequest)   returns (ApplySceneResponse);
  rpc Preview (PreviewSceneRequest) returns (PreviewSceneResponse);
}
```

---

## AutomationService

Live. Backed by the automation engine.

```proto
service AutomationService {
  rpc List    (ListAutomationsRequest)      returns (ListAutomationsResponse);
  rpc Get     (GetAutomationRequest)        returns (GetAutomationResponse);
  rpc Enable  (EnableAutomationRequest)     returns (EnableAutomationResponse);
  rpc Disable (DisableAutomationRequest)    returns (DisableAutomationResponse);
  rpc Trigger (TriggerAutomationRequest)    returns (TriggerAutomationResponse);
  rpc Trace   (TraceAutomationRequest)      returns (stream TraceAutomationResponse);
}
```

### `List` / `Get`

Return automation summaries. Each automation carries `id`, `enabled`, and `mode` fields.

### `Enable` / `Disable`

Toggle the automation's enabled state in-memory. This override is not durable; it reverts on daemon restart. For durable changes, update the Pkl config.

**Error:** `NOT_FOUND` if the automation ID does not exist.

### `Trigger`

Manually fires an automation and returns the `run_id` (correlation ID).

**Error:** `FAILED_PRECONDITION` if the automation is currently disabled.

### `Trace`

Server-streaming. Streams the run trace for an automation. Accepts an optional `run_id` to attach to a specific run; without it, streams the next run. Resumable via `from_cursor`. Multiplexes `TraceEvent` payloads and `Heartbeat`s.

---

## ScriptService

Live.

```proto
service ScriptService {
  rpc List   (ListScriptsRequest)  returns (ListScriptsResponse);
  rpc Run    (RunScriptRequest)    returns (RunScriptResponse);
  rpc Cancel (CancelScriptRequest) returns (CancelScriptResponse);
  rpc Eval   (EvalScriptRequest)   returns (EvalScriptResponse);
}
```

### `List`

Returns all registered scripts with name and parameter schema.

### `Run`

Runs a named script synchronously. Blocks until the script returns (or the client deadline expires, at which point the Starlark run is cancelled).

**Request**

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Script name |
| `args` | `google.protobuf.Struct` | Arguments as a JSON-compatible struct |

**Response**

| Field | Type | Description |
|-------|------|-------------|
| `run_id` | `string` | Correlation ID for event log cross-reference |
| `result` | `google.protobuf.Value` | Starlark `repr()` of the return value |

### `Cancel`

Signals cancellation of an in-flight script run. Best-effort: returns immediately. The script's context is cancelled; the caller watches `EventService.Tail` for `ScriptFinished{outcome: cancelled}`.

**Error:** `FAILED_PRECONDITION` if the script run has already completed.

### `Eval`

Ad-hoc Starlark expression evaluation. Intended for diagnostic use over UDS; tightly policy-gated over TCP (post-C9).

**Request**

| Field | Type | Description |
|-------|------|-------------|
| `expr` | `string` | Starlark expression or script body |

**Response**

| Field | Type | Description |
|-------|------|-------------|
| `result` | `google.protobuf.Value` | Evaluated result |
| `stdout` | `string` | Any `print()` output from the script |

---

## ConfigService

Live.

```proto
service ConfigService {
  rpc Validate    (ValidateConfigRequest)    returns (ValidateConfigResponse);
  rpc Apply       (ApplyConfigRequest)       returns (ApplyConfigResponse);
  rpc Reload      (ReloadConfigRequest)      returns (ReloadConfigResponse);
  rpc GetArtifact (GetConfigArtifactRequest) returns (GetConfigArtifactResponse);
}
```

### `Validate`

Runs the full Pkl evaluator pipeline and returns the diff without applying. Does not persist anything or emit events.

**Response**

| Field | Type | Description |
|-------|------|-------------|
| `valid` | `bool` | Whether the config is valid |
| `errors` | `repeated string` | Validation error messages if invalid |
| `diff` | `ConfigDiff` | What would change if applied |

### `Apply`

**Request**

| Field | Type | Description |
|-------|------|-------------|
| `pkl_bundle` | `bytes` | Tarball of Pkl files (optional; if empty, uses daemon's config dir) |
| `message` | `string` | Free-form change message for the audit trail |
| `dry_run` | `bool` | If true, validate + diff without persisting |
| `strict` | `bool` | Require a prior successful `Validate` with the same hash |

On `dry_run = false`: writes the bundle, triggers a surgical reload of the automation and script engines, and appends a `ConfigApplied` event.

**Response**

| Field | Type | Description |
|-------|------|-------------|
| `diff` | `ConfigDiff` | Changes applied |
| `correlation_id` | `string` | Correlation ID of the resulting `ConfigApplied` event |

### `Reload`

Re-reads the daemon's current on-disk config directory without sending a new bundle. Triggers the same hooks as `Apply`.

### `GetArtifact`

Returns the current `ConfigSnapshot` (the fully-evaluated and compiled config) for read-only tools.

---

## DashboardService

**UNIMPLEMENTED.** Proto shape is final.

```proto
service DashboardService {
  rpc List       (ListDashboardsRequest)  returns (ListDashboardsResponse);
  rpc Get        (GetDashboardRequest)    returns (GetDashboardResponse);
  rpc SaveLayout (SaveLayoutRequest)      returns (SaveLayoutResponse);
}
```

---

## AuthService

**UNIMPLEMENTED.** C9 ships the real implementation. Proto shape is final.

```proto
service AuthService {
  rpc Login                  (LoginRequest)                  returns (LoginResponse);
  rpc Logout                 (LogoutRequest)                 returns (LogoutResponse);
  rpc CurrentUser            (CurrentUserRequest)            returns (CurrentUserResponse);
  rpc CreateToken            (CreateTokenRequest)            returns (CreateTokenResponse);
  rpc RevokeToken            (RevokeTokenRequest)            returns (RevokeTokenResponse);
  rpc ListUsers              (ListUsersRequest)              returns (ListUsersResponse);
  rpc RegisterPasskey        (RegisterPasskeyRequest)        returns (RegisterPasskeyResponse);
  rpc StartWebAuthnChallenge (StartWebAuthnChallengeRequest) returns (StartWebAuthnChallengeResponse);
}
```

Until C9 lands: all TCP requests to any service return `UNAUTHENTICATED`. UDS requests are authenticated as `system:local`.

---

## SystemService

Live.

```proto
service SystemService {
  rpc Version              (VersionRequest)              returns (VersionResponse);
  rpc Health               (HealthRequest)               returns (HealthResponse);
  rpc Metrics              (MetricsRequest)              returns (MetricsResponse);
  rpc Diagnostics          (DiagnosticsRequest)          returns (DiagnosticsResponse);
  rpc CreateSnapshot       (CreateSnapshotRequest)       returns (CreateSnapshotResponse);
  rpc GetConfigDir         (GetConfigDirRequest)         returns (GetConfigDirResponse);
  rpc RecordConfigFileEdit (RecordConfigFileEditRequest)  returns (RecordConfigFileEditResponse);
  rpc GetMCPConfig         (GetMCPConfigRequest)         returns (GetMCPConfigResponse);
}
```

### `Version`

Returns binary version, git commit, build date, and schema version.

### `Health`

Returns per-subsystem health. Also accessible without auth at `GET /healthz` (liveness probe).

**Response**

| Field | Type | Description |
|-------|------|-------------|
| `ok` | `bool` | Overall health |
| `summary` | `string` | Human-readable summary |
| `subsystems` | `repeated SubsystemHealth` | Per-subsystem: `name`, `ok`, `detail` |

### `Metrics`

Returns Prometheus exposition-format metrics as a string. (Also available at `GET /metrics`.)

### `Diagnostics`

Returns a diagnostic bundle (recent logs, goroutine dump, config hash) as archive bytes. Capped at 10 MiB.

### `CreateSnapshot`

Triggers an immediate projector snapshot. Returns the cursor and timestamp of the created snapshot.

**Request**

| Field | Default | Description |
|-------|---------|-------------|
| `owner` | `"state_cache"` | Projector owner to snapshot |
| `reason` | `"manual"` | Reason recorded in snapshot metadata |

### `GetConfigDir`

Returns the daemon's current config directory path.

### `RecordConfigFileEdit`

Records a config file edit into the event log. Used by the MCP server and editor integrations to create an audit trail of in-place file edits without a full `config apply`.

**Request**

| Field | Description |
|-------|-------------|
| `session_id` | Editor session identifier |
| `path` | Absolute path to the edited file |
| `sha256_hex` | SHA-256 hex digest of the new file contents |
| `size_bytes` | File size in bytes |

**Response:** `event_cursor` — the cursor of the appended `ConfigFileEdited` event.

### `GetMCPConfig`

Returns daemon-side limits used by the MCP server (buffer sizes, eval result caps, tail timeouts).

---

## Non-RPC endpoints

| Path | Method | Auth | Description |
|------|--------|------|-------------|
| `/healthz` | `GET` | None | Liveness probe; returns `200 OK` when the daemon is running |
| `/metrics` | `GET` | None | Prometheus metrics scrape endpoint |
| `/webhooks/{slug}` | `POST` | HMAC-SHA256 signature | Webhook ingress; validates `X-Switchyard-Signature: v1=<hex>` against the per-slug Pkl secret |

---

## Versioning policy

### Current: `v1alpha1`

All services live in `switchyard.v1alpha1.*`. Wire-breaking changes between releases are permitted with a migration note in the docs.

### Graduation to `v1`

Graduation is a one-way door. Requirements:

1. C8 (MCP), C10 (Web UI), and at least one external client have been built against `v1alpha1` and agree the surface is stable.
2. A decision-record entry has been committed in this spec or a follow-up graduation spec.
3. Both `v1alpha1` and `v1` are served during a deprecation window (at least one minor release). `v1alpha1` is removed one minor release after `v1` ships.

### Data messages

`switchyard.event.v1`, `switchyard.entity.v1`, and `switchyard.config.v1` are stable data messages referenced by `v1alpha1` services as-is. Their stability was established in C1–C4.
