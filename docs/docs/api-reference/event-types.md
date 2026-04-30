# Event Types Reference

!!! status-alpha "Alpha — shipped, interface evolving"

The event schema is a **public contract**. Payload field names and types will not change without a major version bump. New optional fields may be added; existing fields are never removed or renumbered within a major version.

Every event stored in the event log has an outer envelope (position, timestamp, kind, entity, source, correlation ID, cause ID) and an inner `Payload` that carries the event-specific fields. The `Payload` is a proto `oneof`; the active variant is identified by the `kind` string column, which mirrors the proto field name.

Events are retrieved via [`EventService.Query`](./connect-rpc.md#query) (history) or [`EventService.Tail`](./connect-rpc.md#tail) (live stream).

---

## Payload variant map

```proto
message Payload {
  oneof kind {
    // 1-9: system/meta
    SystemEvent           system                    = 1;
    // 10-19: command/state plane
    StateChanged          state_changed             = 10;
    CommandIssued         command_issued            = 11;
    CommandAck            command_ack               = 12;
    // 20-29: registry plane
    EntityRegistered      entity_registered         = 20;
    EntityUnregistered    entity_unregistered       = 21;
    // 30-39: driver passthrough
    DriverEvent           driver_event              = 30;
    // 40-49: config plane
    ConfigApplied         config_applied            = 40;
    // 50-59: automation/script plane
    AutomationTriggered   automation_triggered      = 50;
    AutomationFinished    automation_finished       = 51;
    ScriptInvoked         script_invoked            = 52;
    ScriptFinished        script_finished           = 53;
    // 60-69: external ingress
    WebhookReceived       webhook_received          = 60;
    MCPEvalRequested      mcp_eval_requested        = 61;
    ConfigFileEdited      config_file_edited        = 62;
    // 70-79: registry mutations
    DeviceRenamed         device_renamed            = 70;
    DeviceReassigned      device_reassigned         = 71;
    // 80-89: driver control
    DriverInstanceRestarted driver_instance_restarted = 80;
  }
}
```

---

## Run outcome enum

Several automation and script events share a `RunOutcome` enum:

| Value | Meaning |
|-------|---------|
| `OUTCOME_UNSPECIFIED` | Default / unknown |
| `OUTCOME_OK` | Run completed successfully |
| `OUTCOME_CONDITION_FAIL` | Automation condition evaluated to false (automations only) |
| `OUTCOME_ACTION_ERROR` | An action raised an error |
| `OUTCOME_LIMIT_EXCEEDED` | Execution step limit hit |
| `OUTCOME_CANCELLED` | Run was cancelled (e.g. client deadline, `ScriptService.Cancel`) |
| `OUTCOME_SKIPPED` | Automation skipped by the admission gate (automations only) |

---

## Event reference

### `SystemEvent` (`system`)

**Kind string:** `system`

**When emitted:** On daemon startup, shutdown, or other system-level lifecycle transitions.

```proto
message SystemEvent {
  string              kind = 1;   // "startup" | "shutdown" | custom
  map<string, string> data = 2;   // free-form context
}
```

| Field | Description |
|-------|-------------|
| `kind` | Lifecycle stage: `"startup"`, `"shutdown"`, or a daemon-defined custom string |
| `data` | Free-form key/value context for the event (e.g. version, config hash on startup) |

**Automation trigger writers:** Use this event to react to daemon restarts. The `state_changed` trigger with `kind = "system"` is the conventional pattern for on-startup automations.

**MCP users:** `SystemEvent` is informational; most workflows use `state_changed` and `automation_finished` instead.

---

### `StateChanged` (`state_changed`)

**Kind string:** `state_changed`

**When emitted:** Whenever a driver reports a new attribute set for an entity. This is the most common event type; it drives the majority of automation triggers.

```proto
message StateChanged {
  gohome.entity.v1.Attributes attributes = 1;
}
```

| Field | Description |
|-------|-------------|
| `attributes` | Full attribute snapshot for the entity at the time of the state change |

The outer envelope's `entity` column carries the dotted entity ID (e.g. `"light.living_room"`). The `attributes` map is the complete current state — not just the delta.

**Automation trigger writers:** This is the primary trigger event. Use `StateChangeTrigger` in Pkl to match on specific entity+attribute transitions.

**MCP users:** Subscribe to `state_changed` via `EventService.Tail` with an entity prefix filter to watch a room or device class in real time.

---

### `CommandIssued` (`command_issued`)

**Kind string:** `command_issued`

**When emitted:** Immediately when `EntityService.CallCapability` is called (or when an automation issues a `call_service` action). Emitted before the driver has processed the command.

```proto
message CommandIssued {
  string              command    = 1;   // capability name, e.g. "turn_on"
  map<string, string> parameters = 2;  // capability arguments
}
```

| Field | Description |
|-------|-------------|
| `command` | The capability invoked, e.g. `"turn_on"`, `"set_brightness"` |
| `parameters` | Arguments passed to the capability |

The outer envelope `entity` column identifies the target entity.

**Automation trigger writers:** Use `CommandIssued` as a trigger to chain automations that react to commands sent to an entity (e.g. log every brightness change).

---

### `CommandAck` (`command_ack`)

**Kind string:** `command_ack`

**When emitted:** After the driver has processed (or failed to process) the command identified by the corresponding `CommandIssued` event. The `cause_id` outer field links this event back to the `CommandIssued`.

```proto
message CommandAck {
  bool   success       = 1;
  string error_message = 2;  // non-empty on failure
}
```

| Field | Description |
|-------|-------------|
| `success` | Whether the driver accepted and applied the command |
| `error_message` | Error detail from the driver if `success = false` |

**Automation trigger writers:** Use `CommandAck` with `success = false` to build alerting automations that notify when commands fail.

---

### `EntityRegistered` (`entity_registered`)

**Kind string:** `entity_registered`

**When emitted:** When a driver instance registers a new entity with the daemon (typically at driver startup or device discovery).

```proto
message EntityRegistered {
  string                      driver_instance_id = 1;
  string                      device_id          = 2;
  string                      entity_type        = 3;   // "light" | "switch" | ...
  string                      friendly_name      = 4;
  gohome.entity.v1.Attributes capabilities       = 5;
}
```

| Field | Description |
|-------|-------------|
| `driver_instance_id` | The driver instance that owns this entity |
| `device_id` | The physical device this entity belongs to |
| `entity_type` | Entity class: `"light"`, `"switch"`, `"sensor"`, etc. |
| `friendly_name` | Display name at registration time |
| `capabilities` | Attribute map describing what the entity can do |

The outer envelope `entity` field carries the assigned dotted entity ID.

**Automation trigger writers:** Use `EntityRegistered` to react to new devices coming online (e.g. auto-assign to a room, send a notification).

---

### `EntityUnregistered` (`entity_unregistered`)

**Kind string:** `entity_unregistered`

**When emitted:** When a driver removes an entity (e.g. device removed, driver stopped).

```proto
message EntityUnregistered {
  string reason = 1;  // e.g. "driver_stopped", "device_removed"
}
```

| Field | Description |
|-------|-------------|
| `reason` | Why the entity was unregistered |

The outer envelope `entity` field identifies which entity was removed.

---

### `DriverEvent` (`driver_event`)

**Kind string:** `driver_event`

**When emitted:** For driver lifecycle events that don't map to a specific entity: driver instance started, stopped, failed, or sent a heartbeat.

```proto
message DriverEvent {
  string driver_instance_id = 1;
  string kind               = 2;   // "started" | "stopped" | "failed" | "heartbeat"
  string detail             = 3;   // human-readable context
}
```

| Field | Description |
|-------|-------------|
| `driver_instance_id` | The driver instance this event pertains to |
| `kind` | Lifecycle event type |
| `detail` | Free-form detail (error message for `failed`, version for `started`, etc.) |

**Automation trigger writers:** Use `kind = "failed"` to alert on driver crashes. Use `kind = "started"` to re-run initialisation automations after a driver restart.

**MCP users:** Monitor this event to know when a device hub goes offline or comes back online.

---

### `ConfigApplied` (`config_applied`)

**Kind string:** `config_applied`

**When emitted:** After a successful `ConfigService.Apply` (with `dry_run = false`). Records the diff applied to the running config.

```proto
message ConfigApplied {
  int64 applied_at_unix_ms       = 1;
  int32 driver_instances_added   = 2;
  int32 driver_instances_removed = 3;
  int32 driver_instances_changed = 4;
  int32 automations_changed      = 5;
  bool  dry_run                  = 6;  // always false when emitted
}
```

| Field | Description |
|-------|-------------|
| `applied_at_unix_ms` | Unix timestamp in milliseconds of the apply |
| `driver_instances_added` | Number of new driver instances started |
| `driver_instances_removed` | Number of driver instances stopped |
| `driver_instances_changed` | Number of driver instances restarted with new config |
| `automations_changed` | Number of automations reloaded |
| `dry_run` | Always `false` for events committed to the log (dry-run runs do not emit this event) |

---

### `AutomationTriggered` (`automation_triggered`)

**Kind string:** `automation_triggered`

**When emitted:** At the start of each automation run, immediately after the trigger matched and before conditions are evaluated.

```proto
message AutomationTriggered {
  string automation_id          = 1;
  string correlation_id         = 2;   // UUID per run
  uint64 trigger_event_position = 10;  // 0 for time/manual triggers
  string trigger_kind           = 11;  // "state_changed" | "event" | "time" | "manual"
  string invoked_by             = 12;  // "cli:<user>" | "api:<token>" | ""
}
```

| Field | Description |
|-------|-------------|
| `automation_id` | The automation's slug |
| `correlation_id` | Unique run ID; links this event to `AutomationFinished` and any actions taken during the run |
| `trigger_event_position` | Cursor of the event that fired the trigger (0 for time and manual triggers) |
| `trigger_kind` | What kind of trigger fired |
| `invoked_by` | Who triggered this run manually (empty for subscription-driven runs) |

**Automation trigger writers:** Use `correlation_id` to correlate `AutomationTriggered` with the subsequent `AutomationFinished`.

---

### `AutomationFinished` (`automation_finished`)

**Kind string:** `automation_finished`

**When emitted:** When an automation run completes (success, skip, or error).

```proto
message AutomationFinished {
  string     automation_id  = 1;
  string     correlation_id = 2;
  RunOutcome outcome        = 10;
  string     error          = 11;   // non-empty on error outcomes
  int64      elapsed_ms     = 20;
  uint64     starlark_steps = 21;
  repeated string log_lines = 22;
}
```

| Field | Description |
|-------|-------------|
| `automation_id` | The automation's slug |
| `correlation_id` | Matches the `AutomationTriggered` for this run |
| `outcome` | `RunOutcome` enum value |
| `error` | Error message if `outcome = OUTCOME_ACTION_ERROR` |
| `elapsed_ms` | Total wall-clock time for the run in milliseconds |
| `starlark_steps` | Number of Starlark evaluation steps consumed |
| `log_lines` | Lines emitted by `print()` calls inside the automation's Starlark blocks |

**MCP users:** Tail `automation_finished` to know when an automation run has completed and check its outcome.

---

### `ScriptInvoked` (`script_invoked`)

**Kind string:** `script_invoked`

**When emitted:** When a script run starts (via `ScriptService.Run`, an automation `script` action, or the CLI).

```proto
message ScriptInvoked {
  string              script_name    = 1;
  string              correlation_id = 2;
  string              invoked_by     = 10;  // "cli:<user>" | "automation:<id>" | "api:<token>"
  map<string, string> args           = 11;
}
```

| Field | Description |
|-------|-------------|
| `script_name` | Registered script name |
| `correlation_id` | Unique run ID; links to `ScriptFinished` |
| `invoked_by` | What or who started the script |
| `args` | Arguments passed to the script (string-coerced) |

---

### `ScriptFinished` (`script_finished`)

**Kind string:** `script_finished`

**When emitted:** When a script run completes.

```proto
message ScriptFinished {
  string     script_name    = 1;
  string     correlation_id = 2;
  RunOutcome outcome        = 10;
  string     error          = 11;
  int64      elapsed_ms     = 20;
  uint64     starlark_steps = 21;
  repeated string log_lines = 22;
  string     return_value   = 23;  // Starlark repr() of the return value
}
```

| Field | Description |
|-------|-------------|
| `script_name` | Registered script name |
| `correlation_id` | Matches `ScriptInvoked` for this run |
| `outcome` | `RunOutcome` enum |
| `error` | Error message on failure |
| `elapsed_ms` | Wall-clock time in milliseconds |
| `starlark_steps` | Starlark evaluation step count |
| `log_lines` | `print()` output from the script |
| `return_value` | Starlark `repr()` of the script's return value (empty if `None`) |

---

### `WebhookReceived` (`webhook_received`)

**Kind string:** `webhook_received`

**When emitted:** When a valid (HMAC-authenticated) `POST /webhooks/{slug}` request arrives. Emitted before the `202 Accepted` response is sent.

```proto
message WebhookReceived {
  string              slug      = 1;
  bytes               body      = 10;
  map<string, string> headers   = 11;  // lowercased: content-type, user-agent, x-forwarded-for
  string              source_ip = 20;
}
```

| Field | Description |
|-------|-------------|
| `slug` | The webhook slug from the URL path |
| `body` | Raw request body bytes (up to `listener.webhooks.max_body_bytes`, default 1 MiB) |
| `headers` | Selected request headers with lowercased keys |
| `source_ip` | Peer IP, or first hop from `X-Forwarded-For` if behind a configured trusted proxy |

**Automation trigger writers:** This event drives `WebhookTrigger`. Declare a `WebhookTrigger{slug: "my-hook", secret: ...}` in Pkl and the automation fires on each valid webhook delivery.

---

### `MCPEvalRequested` (`mcp_eval_requested`)

**Kind string:** `mcp_eval_requested`

**When emitted:** When the MCP server processes a Starlark eval request from an AI agent. Provides an audit trail of AI-initiated evaluations.

```proto
message MCPEvalRequested {
  string principal_id      = 1;
  string session_id        = 2;
  string source            = 10;   // Starlark source (may be truncated)
  string result_sha256_hex = 11;
  bool   truncated         = 12;
  uint32 result_bytes      = 13;
  uint32 duration_ms       = 20;
  string error             = 21;
}
```

| Field | Description |
|-------|-------------|
| `principal_id` | The MCP principal that made the request |
| `session_id` | MCP session identifier |
| `source` | Starlark source (truncated in the event if long) |
| `result_sha256_hex` | SHA-256 hex digest of the full result |
| `truncated` | Whether the `source` field was truncated |
| `result_bytes` | Byte length of the full result |
| `duration_ms` | Evaluation duration in milliseconds |
| `error` | Error string if evaluation failed |

---

### `ConfigFileEdited` (`config_file_edited`)

**Kind string:** `config_file_edited`

**When emitted:** When the MCP server writes a Pkl config file on behalf of an AI agent.

```proto
message ConfigFileEdited {
  string principal_id = 1;
  string session_id   = 2;
  string path         = 10;   // absolute path within the config directory
  string sha256_hex   = 11;
  uint32 size_bytes   = 12;
}
```

| Field | Description |
|-------|-------------|
| `principal_id` | The MCP principal that made the edit |
| `session_id` | MCP session identifier |
| `path` | Absolute path to the file that was written |
| `sha256_hex` | SHA-256 hex digest of the written content |
| `size_bytes` | File size in bytes |

---

### `DeviceRenamed` (`device_renamed`)

**Kind string:** `device_renamed`

**When emitted:** After `DeviceService.Rename` succeeds.

```proto
message DeviceRenamed {
  string device_id          = 1;
  string old_friendly_name  = 10;
  string new_friendly_name  = 11;
}
```

| Field | Description |
|-------|-------------|
| `device_id` | The device that was renamed |
| `old_friendly_name` | Previous display name |
| `new_friendly_name` | New display name |

---

### `DeviceReassigned` (`device_reassigned`)

**Kind string:** `device_reassigned`

**When emitted:** After `DeviceService.Reassign` succeeds.

```proto
message DeviceReassigned {
  string device_id    = 1;
  string old_area_id  = 10;
  string new_area_id  = 11;
}
```

| Field | Description |
|-------|-------------|
| `device_id` | The device that was moved |
| `old_area_id` | Previous area slug |
| `new_area_id` | New area slug |

---

### `DriverInstanceRestarted` (`driver_instance_restarted`)

**Kind string:** `driver_instance_restarted`

**When emitted:** After `DriverService.RestartInstance` triggers a restart cycle in the Carport supervisor.

```proto
message DriverInstanceRestarted {
  string driver_instance_id = 1;
  string reason             = 10;
  string actor              = 11;  // "cli:<user>" | "api:<token>" | "system"
}
```

| Field | Description |
|-------|-------------|
| `driver_instance_id` | The instance that was restarted |
| `reason` | The reason string supplied in the request |
| `actor` | What or who initiated the restart |

**Automation trigger writers:** Use this event to run a post-restart initialisation automation (e.g. re-apply a scene after a driver comes back up).

---

### `SceneApplied` (`scene_applied`)

**Kind string:** `scene_applied`

!!! status-planned "Planned — SceneService not yet implemented"
    SceneApplied events will be emitted when the SceneService ships.

**When emitted:** When a scene is applied via `gohome scene apply` or from an automation.

| Field | Type | Description |
|---|---|---|
| `scene_id` | string | The scene entity ID that was applied |
| `applied_by` | string | Who applied it: `automation:<id>`, `user:<id>`, `mcp`, `cli` |
| `entity_count` | int32 | Number of entity states set by this scene |

---

### `UserLoggedIn` (`user_logged_in`)

**Kind string:** `user_logged_in`

!!! status-planned "Planned — AuthService not yet implemented"
    User auth events will be emitted when the AuthService ships.

**When emitted:** When a user successfully authenticates.

| Field | Type | Description |
|---|---|---|
| `user_id` | string | The authenticated user's ID |
| `method` | string | Auth method used: `passkey`, `password`, `token` |
| `client_addr` | string | Client IP address |

### `UserLoggedOut` (`user_logged_out`)

**Kind string:** `user_logged_out`

**When emitted:** When a user session ends (explicit logout or session expiry).

| Field | Type | Description |
|---|---|---|
| `user_id` | string | The user ID |
| `reason` | string | `logout`, `expired`, `revoked` |

---

## Using events in automations and MCP

### Automation triggers

Declare a `StateChangeTrigger` or `EventTrigger` in Pkl to react to any of the above event kinds. The `trigger_event_position` field on `AutomationTriggered` tells you which specific event fired the automation — use this to correlate in the `EventService.Query` history.

### MCP event tailing

Use the `gohome__tail_events` MCP tool or `EventService.Tail` directly to stream events into an AI agent's context. Filter by `kinds` to limit to the relevant event types. Use `from_cursor` to resume after a disconnect without missing events.

### At-least-once delivery on resume

Resuming a stream with `from_cursor` guarantees at-least-once delivery at the seam. Your consumer should track `last_seen_cursor` and discard events with `cursor <= last_seen_cursor` on reconnect.
