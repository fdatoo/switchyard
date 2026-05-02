# Tool Catalog

!!! status-alpha "Alpha — shipped, interface evolving"

switchyard exposes 12 MCP tools. Every tool name is prefixed with `switchyard__` to avoid collisions with tools from other servers. Each tool carries a **verb** that indicates the category of operation:

| Verb | Meaning |
|------|---------|
| `READ` | Read-only — does not modify state |
| `CALL` | Invokes an action or script — modifies runtime state |
| `ADMIN` | Mutates persistent configuration or applies config bundles |

List the tools exposed by your installed version of switchyard:

```sh
switchyard mcp tools
switchyard mcp tools --json  # machine-readable
```

---

### `switchyard__get_state`

**Verb:** READ

Get the current state of a single entity by its dotted ID.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `entity_id` | string | yes | Dotted entity ID, e.g. `light.living_room` |

**Output**

Protojson of the `Entity` message, including `entity_id`, `friendly_name` (returned as `name`), `class`, `state`, `attributes`, `area`, and `last_changed`.

```json
{
  "entity_id": "light.living_room",
  "name": "Living Room",
  "class": "light",
  "state": "on",
  "attributes": { "brightness": 80, "color_temp": 370 },
  "area": "living_room",
  "last_changed": "2026-04-27T21:04:11Z"
}
```

**Example invocation**

```json
{
  "name": "switchyard__get_state",
  "arguments": {
    "entity_id": "light.living_room"
  }
}
```

---

### `switchyard__list_entities`

**Verb:** READ

Browse all entities with optional filters. Returns a paginated list.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `areas` | array of string | no | Filter to entities in any of these area slugs |
| `zones` | array of string | no | Filter to entities in any of these zone slugs |
| `classes` | array of string | no | Filter by entity class, e.g. `light`, `sensor`, `binary_sensor` |
| `device_id` | string | no | Filter to entities belonging to a specific device |
| `limit` | integer | no | Maximum results to return (1–1000, default 100) |
| `cursor` | string | no | Opaque pagination token from a previous call |

**Output**

```json
{
  "entities": [
    {
      "entity_id": "light.living_room",
      "name": "Living Room",
      "class": "light",
      "state": "on",
      "area": "living_room",
      "subscribe_uri": "switchyard://entities/light.living_room"
    }
  ],
  "next_cursor": "eyJvZmZzZXQiOjEwMH0"
}
```

Each entity entry includes a `subscribe_uri` pointing at the corresponding MCP resource URI.

**Example invocation**

```json
{
  "name": "switchyard__list_entities",
  "arguments": {
    "classes": ["light"],
    "areas": ["living_room"],
    "limit": 20
  }
}
```

---

### `switchyard__call_capability`

**Verb:** CALL

Invoke a named capability on an entity. Capabilities are class-specific (e.g. `turn_on`, `set_brightness` for lights; `lock`, `unlock` for locks). The call is synchronous — it returns once the driver has acknowledged the command, or returns `UNAVAILABLE` if the driver is unreachable.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `entity_id` | string | yes | Dotted entity ID |
| `capability` | string | yes | Capability name, e.g. `turn_on`, `set_brightness`, `set_color` |
| `params` | object | no | Capability-specific parameters; shape depends on the capability |

**Output**

```json
{
  "accepted": true,
  "command_id": "01JSZQ8KXWABCD1234567890AB"
}
```

**Example invocation**

```json
{
  "name": "switchyard__call_capability",
  "arguments": {
    "entity_id": "light.living_room",
    "capability": "set_brightness",
    "params": { "brightness": 60 }
  }
}
```

---

### `switchyard__query_events`

**Verb:** READ

Query the historical event log with filters and cursor-based pagination. Returns a bounded slice of events; use `next_cursor` to page through large results.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kinds` | array of string | no | Filter by event kind, e.g. `state_changed`, `capability_called` |
| `entity_prefix` | string | no | Filter by entity ID prefix, e.g. `light.` to match all lights |
| `sources` | array of string | no | Filter by source, e.g. `mcp`, `cli`, `automation` |
| `from_cursor` | integer | no | Start from this event cursor (exclusive) |
| `to_cursor` | integer | no | End at this event cursor (inclusive) |
| `from_time` | string | no | Start time, RFC3339 format |
| `to_time` | string | no | End time, RFC3339 format |
| `limit` | integer | no | Maximum events to return (1–1000) |
| `cursor` | string | no | Opaque pagination token from a previous call |

**Output**

```json
{
  "events": [
    {
      "cursor": 48291,
      "kind": "state_changed",
      "entity_id": "light.living_room",
      "occurred_at": "2026-04-27T21:04:11Z",
      "payload": { "from": "off", "to": "on" }
    }
  ],
  "next_cursor": "eyJjdXJzb3IiOjQ4MjkxfQ"
}
```

**Example invocation**

```json
{
  "name": "switchyard__query_events",
  "arguments": {
    "entity_prefix": "light.",
    "kinds": ["state_changed"],
    "from_time": "2026-04-27T20:00:00Z",
    "to_time": "2026-04-27T22:00:00Z",
    "limit": 50
  }
}
```

---

### `switchyard__tail_events`

**Verb:** READ

Return a buffered window of recent or incoming events without holding an open subscription. Useful for short-lived "what just happened" checks without the overhead of a resource subscription.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kinds` | array of string | no | Filter by event kind |
| `entity_prefix` | string | no | Filter by entity ID prefix |
| `sources` | array of string | no | Filter by source |
| `from_cursor` | integer | no | Resume from this cursor; 0 means from now |
| `max_events` | integer | no | Maximum events to buffer (1–1000, default 100) |
| `wait_seconds` | integer | no | Block up to this many seconds waiting for new events (0–60, default 0 = return immediately) |

**Output**

```json
{
  "events": [ /* same shape as query_events */ ],
  "next_cursor": "eyJjdXJzb3IiOjQ4MzEyfQ"
}
```

Use the returned `next_cursor` in a subsequent `tail_events` call to continue from where you left off.

**Example invocation**

```json
{
  "name": "switchyard__tail_events",
  "arguments": {
    "from_cursor": 0,
    "max_events": 20,
    "wait_seconds": 5
  }
}
```

---

### `switchyard__apply_scene`

**Verb:** CALL

Apply a named scene by ID. All entities in the scene are set to their recorded state simultaneously.

!!! note "Not yet implemented"
    `switchyard__apply_scene` is registered in the tool catalog so the schema is discoverable, but it currently returns `UNIMPLEMENTED`. It will become functional when the Scene spec ships.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `scene_id` | string | yes | Scene ID as declared in Pkl config |

**Output** *(once implemented)*

```json
{
  "applied": true,
  "scene_id": "evening_movie",
  "applied_at": "2026-04-27T21:10:00Z"
}
```

**Example invocation**

```json
{
  "name": "switchyard__apply_scene",
  "arguments": {
    "scene_id": "evening_movie"
  }
}
```

---

### `switchyard__run_script`

**Verb:** CALL

Invoke a named Starlark script declared in config. The call is synchronous — it blocks until the script completes or its deadline is reached. Use `query_events` with the returned `run_id` to inspect events emitted by the script.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | yes | Script name as declared in `scripts.pkl` |
| `args` | object | no | Named arguments to pass to the script |
| `timeout_seconds` | integer | no | Override the script's default deadline (1–300) |

**Output**

```json
{
  "run_id": "01JSZQ8KXWABCD1234567890AB",
  "result": "notified 2 residents",
  "duration_ms": 143
}
```

**Example invocation**

```json
{
  "name": "switchyard__run_script",
  "arguments": {
    "name": "notify_residents",
    "args": {
      "message": "Motion detected in garage",
      "priority": "high"
    }
  }
}
```

---

### `switchyard__eval_starlark`

**Verb:** CALL

Evaluate an ad-hoc Starlark expression or short script body. The result is returned as the Starlark `repr()` of the expression's value.

**Guardrails:**

- Read-only stdlib only: `state`, `now`, `log`, `repr`. No `call_capability`, no entity writes.
- 30-second wall-clock deadline, 10 million step limit.
- Output capped at 64 KiB; larger results are truncated with a marker.
- Every call emits an `MCPEvalRequested` audit event in the daemon event log.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `source` | string | yes | Starlark expression or short script body |

**Output**

```json
{
  "result": "{'state': 'on', 'brightness': 80}",
  "duration_ms": 12,
  "truncated": false
}
```

**Example invocation**

```json
{
  "name": "switchyard__eval_starlark",
  "arguments": {
    "source": "state('light.living_room')"
  }
}
```

---

### `switchyard__validate_config`

**Verb:** READ

Validate a gzipped tarball of Pkl config files without applying it. Returns a diff against the running config and any validation errors. Use this before `apply_config` to check your changes.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pkl_bundle` | bytes (base64) | yes | Gzipped tarball of Pkl source files |

**Output**

```json
{
  "valid": true,
  "diff": [
    { "path": "automations/lights.pkl", "change": "modified" }
  ],
  "errors": []
}
```

**Example invocation**

```json
{
  "name": "switchyard__validate_config",
  "arguments": {
    "pkl_bundle": "<base64-encoded gzipped tarball>"
  }
}
```

---

### `switchyard__apply_config`

**Verb:** ADMIN

Apply a gzipped tarball of Pkl config files to the running daemon. The daemon validates, diffs, and atomically applies the bundle. Use `dry_run: true` to preview changes without persisting them.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pkl_bundle` | bytes (base64) | yes | Gzipped tarball of Pkl source files |
| `message` | string | no | Audit trail message describing the change |
| `dry_run` | boolean | no | Validate and diff without persisting (default false) |
| `strict` | boolean | no | Require a prior successful `validate_config` with the same bundle hash |

**Output**

```json
{
  "applied": true,
  "diff": [
    { "path": "automations/lights.pkl", "change": "modified" }
  ],
  "applied_at": "2026-04-27T21:15:00Z"
}
```

**Example invocation**

```json
{
  "name": "switchyard__apply_config",
  "arguments": {
    "pkl_bundle": "<base64-encoded gzipped tarball>",
    "message": "Add garage motion automation",
    "dry_run": false
  }
}
```

---

### `switchyard__read_config_file`

**Verb:** READ

Read a single UTF-8 file from the switchyard config directory. The path is relative to the config root. Files up to 1 MiB are supported.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `path` | string | yes | Path relative to the switchyard config directory, e.g. `automations/lights.pkl` |

**Output**

```json
{
  "path": "automations/lights.pkl",
  "content": "// automations/lights.pkl\nimport \"switchyard:automations\" ...",
  "size_bytes": 1024,
  "sha256_hex": "a3f2c1..."
}
```

Errors:

- `path_escape` — path resolves outside the config directory
- `file_not_found` — file does not exist
- `not_a_regular_file` — path is a directory
- `file_too_large` — file exceeds 1 MiB
- `not_utf8` — file is not valid UTF-8

**Example invocation**

```json
{
  "name": "switchyard__read_config_file",
  "arguments": {
    "path": "automations/lights.pkl"
  }
}
```

---

### `switchyard__write_config_file`

**Verb:** ADMIN

Write a UTF-8 file to the switchyard config directory. Only `.pkl` and `.star` files may be written. The file is syntax-checked before being committed to disk. Writing does **not** trigger a config reload — call `apply_config` separately when ready to deploy.

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `path` | string | yes | Path relative to the config directory; must end in `.pkl` or `.star` |
| `content` | string | yes | UTF-8 file contents (replaces any existing file) |

**Output**

```json
{
  "path": "automations/lights.pkl",
  "sha256_hex": "b7e3d2...",
  "size_bytes": 2048
}
```

Errors:

- `path_escape` — path resolves outside the config directory
- `unsupported_extension` — extension is not `.pkl` or `.star`
- `syntax_error` — Pkl or Starlark parse error (includes line and column)
- `permission_denied` — OS-level write permission denied

!!! note "Error code distinction"
    OS-level write permission errors surface as `isError: true, reason: 'permission_denied'` from an `internal` error code, not from a `PERMISSION_DENIED` (forbidden) code. Policy-based denials (after C9 auth ships) will use `forbidden` instead.

Every successful write emits a `ConfigFileEdited` audit event in the daemon event log.

**Example invocation**

```json
{
  "name": "switchyard__write_config_file",
  "arguments": {
    "path": "automations/garage.pkl",
    "content": "// automations/garage.pkl\nimport \"switchyard:automations\" as automations\n\nautomations: Listing<automations.Automation> = new {\n  new automations.Automation {\n    id = \"garage_motion_lights\"\n    // ...\n  }\n}\n"
  }
}
```
