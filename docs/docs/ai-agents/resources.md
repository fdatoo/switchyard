# MCP Resources

!!! status-alpha "Alpha — shipped, API may evolve"

MCP resources are URI-addressed data sources that agents can read once or subscribe to for live updates. switchyard exposes two resource types:

1. **Entity state** — the current state of one entity or a filtered set of entities.
2. **Automation run traces** — the step-by-step trace of a single automation run.

Unlike tools, resources support the `resources/subscribe` flow: the agent subscribes once and receives a `notifications/resources/updated` notification whenever the data changes, then calls `resources/read` to fetch the new value.

---

## Resource catalog

### `switchyard://entities/{entity_id}`

Live state of a single entity.

**URI pattern:** `switchyard://entities/{entity_id}`

**Examples:**
- `switchyard://entities/light.living_room`
- `switchyard://entities/binary_sensor.front_door_motion`

**What it returns**

A `resources/read` call returns the current entity state — the same shape as `switchyard__get_state`:

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

**Discovering URIs**

Entity resource URIs are returned in `switchyard__list_entities` results as the `subscribe_uri` field on each entity entry. You can also construct them directly from a known entity ID.

**How to subscribe**

Send a `resources/subscribe` request for the URI:

```json
{
  "method": "resources/subscribe",
  "params": { "uri": "switchyard://entities/light.living_room" }
}
```

**What updates look like**

When the entity's state changes, the server sends:

```json
{
  "method": "notifications/resources/updated",
  "params": { "uri": "switchyard://entities/light.living_room" }
}
```

Receive the notification, then call `resources/read` on the same URI to get the new state. The server caches the latest snapshot, so the read does not require an additional daemon roundtrip.

**Backpressure**

If the agent processes updates slowly, the server coalesces intermediate snapshots: you receive a single `updated` notification instead of many, and the next `resources/read` returns the latest state. Intermediate states are not replayed. Coalescing begins after 256 pending updates are queued — matching the behaviour described in the backpressure section.

---

### `switchyard://entities?selector={selector}`

Live state of a filtered set of entities.

**URI pattern:** `switchyard://entities?selector={selector}`

The `{selector}` is a base64url-encoded JSON object with the same fields as `switchyard__list_entities` filters:

```json
{
  "areas": ["living_room", "kitchen"],
  "classes": ["light"]
}
```

Base64url-encode this JSON to form the selector query parameter.

**What it returns**

A `resources/read` call returns all matching entities as a snapshot:

```json
{
  "entities": [
    {
      "entity_id": "light.living_room",
      "name": "Living Room",
      "state": "on",
      "attributes": { "brightness": 80 }
    },
    {
      "entity_id": "light.kitchen_ceiling",
      "name": "Kitchen Ceiling",
      "state": "off",
      "attributes": {}
    }
  ],
  "generated_at": "2026-04-27T21:10:00Z"
}
```

Selector reads are bounded by a hard cap of 1000 entities. If your selector matches more, the server returns `selector_too_broad` with a suggested narrower filter.

**How to subscribe**

```json
{
  "method": "resources/subscribe",
  "params": { "uri": "switchyard://entities?selector=eyJhcmVhcyI6WyJsaXZpbmdfcm9vbSJdfQ" }
}
```

**What updates look like**

Same pattern as single-entity resources — a `notifications/resources/updated` notification when any entity in the set changes, followed by a `resources/read` to fetch the updated snapshot.

**Resource listing**

`resources/list` enumerates one entry per known entity (for single-entity URIs) and includes a URI template for the selector form:

```
uriTemplate: "switchyard://entities?selector={selector}"
```

---

### `switchyard://automations/{automation_id}/runs/{run_id}/trace`

Step-by-step trace of a single automation run.

**URI pattern:** `switchyard://automations/{automation_id}/runs/{run_id}/trace`

**Example:**
- `switchyard://automations/motion_hall_light/runs/01JSZQ8KXWABCD1234567890AB/trace`

**What it returns**

A `resources/read` call drains up to 5 seconds of trace events from the run, returning them as a list plus a completion flag:

```json
{
  "trace_events": [
    {
      "cursor": 1,
      "kind": "trigger_matched",
      "occurred_at": "2026-04-27T22:01:00Z",
      "detail": { "trigger": "StateChangeTrigger", "entity_id": "binary_sensor.hall_motion", "to": "on" }
    },
    {
      "cursor": 2,
      "kind": "condition_evaluated",
      "occurred_at": "2026-04-27T22:01:00Z",
      "detail": { "condition": "TimeCondition", "result": true }
    },
    {
      "cursor": 3,
      "kind": "action_dispatched",
      "occurred_at": "2026-04-27T22:01:00Z",
      "detail": { "entity_id": "light.hall", "capability": "turn_on" }
    },
    {
      "cursor": 4,
      "kind": "run_completed",
      "occurred_at": "2026-04-27T22:01:01Z",
      "detail": { "status": "success", "duration_ms": 312 }
    }
  ],
  "complete": true,
  "next_cursor": 4
}
```

If the run is still in progress when you call `resources/read`, `complete` is `false`. Subscribe to receive the remaining events as they arrive.

**Discovering trace URIs**

Trace URIs are included in:

- `switchyard__run_script` results (the `run_id` field)
- `query_events` results for automation-triggered runs
- Daemon event log entries for `AutomationTriggered` events

**How to subscribe**

```json
{
  "method": "resources/subscribe",
  "params": {
    "uri": "switchyard://automations/motion_hall_light/runs/01JSZQ8KXWABCD1234567890AB/trace"
  }
}
```

**What updates look like**

```json
{
  "method": "notifications/resources/updated",
  "params": {
    "uri": "switchyard://automations/motion_hall_light/runs/01JSZQ8KXWABCD1234567890AB/trace"
  }
}
```

Call `resources/read` after each notification to fetch new trace events from the server's cache. The server advances the cursor with each update so you always receive fresh events.

**Backpressure**

Trace subscriptions use a larger internal buffer (1024 pending updates). If the buffer overflows, the server closes the subscription with a `trace_overflow` notification. Resubscribe using the `next_cursor` from the last successful read to catch up.

**Trace resources are not enumerated**

`resources/list` does not enumerate trace URIs — there can be thousands of historical runs. Discover them from tool results and event log entries as described above.

---

## Subscribing and unsubscribing

### Subscribing

Send `resources/subscribe` with the URI. The server opens the underlying Connect stream from `switchyardd` and starts forwarding notifications.

### Reading after a notification

When you receive `notifications/resources/updated`, call `resources/read` with the same URI. The server returns the cached latest snapshot without an additional daemon roundtrip.

### Unsubscribing

Send `resources/unsubscribe` with the URI. The server closes the underlying Connect stream and drops the subscription.

### Session close

When the MCP client disconnects (stdin EOF), all subscriptions are closed cleanly. To resume, re-subscribe after reconnecting.

### Stream closed by daemon

If the daemon shuts down or the underlying Connect stream closes unexpectedly, the server fires one final `notifications/resources/updated` notification tagged with `error: "stream_closed"`. Re-subscribe to resume once the daemon is back.
