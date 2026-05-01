# Hue Driver — Robustness Pass — Design

**Status:** draft
**Date:** 2026-04-30
**Branch:** `feat/hue-driver` (continues from the v0.1 driver work)

## Summary

A focused robustness pass on `drivers/hue/` plus one foundational proto
change shared by all drivers. After this lands the driver behaves
correctly under real-world failure modes: unreachable bulbs, bridge
restarts, revoked API keys, command bursts that hit Hue's rate limit,
and bulbs added or removed from the bridge while the driver is running.

No new entity types. Lights only. Color (RGB/XY) is still out of scope —
the gohome `Light` proto doesn't carry color, and that's a separate
workstream. Groups, scenes, and sensors are also deferred.

## Goals

- Surface per-bulb reachability through the platform — every driver, not
  just Hue, can express "this entity is currently reachable".
- Filter advertised capabilities per-entity so commands the bulb can't
  honour aren't reachable from gohome.
- Pace command issuance to fit Hue bridge limits (~10 req/sec on `light`).
- Detect revoked API keys and exit cleanly so the supervisor can quarantine.
- Track bulbs added or removed from the bridge across the driver's
  lifetime; reflect both as gohome registrations.
- Smooth transitions on every command via an optional `duration_ms` arg.
- `set_brightness 0` means off; `set_brightness N>0` on an off-bulb turns it on.
- Bounded command latency — no command waits forever on a stuck bridge.
- Useful logs and operator events for diagnosing problems in the field.

## Non-goals

- Color control (RGB/XY/hue/saturation) — blocked on a separate proto change.
- Groups, rooms, zones, scenes — separate milestone.
- Motion sensors, dimmer switches, temperature sensors.
- CLIP v1 fallback for pre-2021 bridge firmware.
- Automated key rotation or re-pairing — Hue keys don't expire and
  re-pairing requires physical access to the bridge button.

## Architecture

### Cross-cutting proto change

Add `bool available = 90;` to `gohome.entity.v1.Attributes`. The number
90 sits above the entity-type oneof range (10-19) so the field reads as
a metadata bit alongside the typed payload. Drivers populate it; default
zero (`false`) means "unknown / unreachable". The daemon's state cache
serializes it through the existing `Attributes` proto round-trip — no
cache code change needed beyond not stripping the field.

The `gohome` CLI's `state list` and `state get` formatters get an
"AVAIL" column reflecting `Attributes.GetAvailable()`. Automations gain
the ability to condition on it via the existing entity Starlark
bindings (`entity.attributes.available`).

### Driver-internal layout

The existing layout stands. New code lands in:

```
drivers/hue/
├── cmd/hue-driver/main.go             # rate-limit wiring, slog setup, signals
├── internal/bridge/
│   ├── client.go                      # SetLight gains duration arg; auth detection
│   ├── client_test.go
│   ├── ratelimit.go                   # NEW — token bucket wrapper
│   ├── ratelimit_test.go
│   ├── devices.go                     # NEW — /clip/v2/resource/device fetch + zigbee_connectivity correlation
│   └── devices_test.go
└── internal/state/
    ├── mapping.go                     # CommandToUpdate gains duration_ms arg + brightness=0 + auto-on
    ├── mapping_test.go
    ├── capabilities.go                # NEW — per-bulb capability inference from bridge.Light
    └── capabilities_test.go
```

## Components

### 1. Per-bulb capability filtering

`internal/state/capabilities.go` exposes:

```go
func Capabilities(l bridge.Light) []string
```

Logic:
- Always includes `turn_on`, `turn_off` (every Hue light supports these).
- Includes `set_brightness` if `l.Dimming != nil`.
- Includes `set_color_temp` if `l.ColorTemperature != nil` (the field itself, not whether `Mirek` is currently set).

`cmd/hue-driver/main.go` swaps the package-level `var capabilities`
constant for a per-bulb call:

```go
caps := state.Capabilities(l)
d.AddEntity(entityID, driver.EntitySpec{..., Capabilities: caps})
for _, c := range caps {
    d.OnCapability(entityID, c, ...)
}
```

The driver still rejects unknown capabilities at the handler layer
(`state.CommandToUpdate` returns "unknown capability"), but that becomes
a defence-in-depth path; gohome won't dispatch commands the entity
didn't advertise.

### 2. Reachability tracking

Hue v2 splits per-bulb reachability across two resources:
- `/clip/v2/resource/light/{id}` — the bulb's lighting state.
- `/clip/v2/resource/device/{id}` — the physical Zigbee device. Its
  `services` array points at a `zigbee_connectivity` resource whose
  `status` field is `connected` / `connectivity_issue` / `unreachable`.

`internal/bridge/devices.go` exposes:

```go
type DeviceStatus struct {
    DeviceID string  // Hue device UUID
    LightID  string  // the light service UUID this device owns (if any)
    Status   string  // "connected" | "connectivity_issue" | "unreachable"
}

func (c *Client) ListDevices(ctx) ([]DeviceStatus, error)
```

Implementation: `GET /clip/v2/resource/device` to enumerate, then for
each device, walk `services` to find the `zigbee_connectivity` resource
ref, then `GET /clip/v2/resource/zigbee_connectivity/{id}` to read
`status`. The response can be batched (the `zigbee_connectivity`
resource collection is a single GET).

`state.LightToAttrs` and `state.MergeEvent` learn an extra `available
bool` argument: callers (in `cmd/hue-driver`) pass the current
reachability for that bulb. The mapping populates `Attributes.Available`.

The SSE stream emits `zigbee_connectivity` updates when a bulb becomes
reachable or unreachable; the driver's event loop handles those alongside
`light` updates. On disconnect resync, both `ListLights` and `ListDevices`
run; `state.LightToAttrs` is called with the freshest status.

### 3. Auth-failure detection

`internal/bridge/client.go` tracks consecutive `401 Unauthorized`
responses with timestamps. After three within 60 seconds, the next
caller (typically the SSE goroutine or a command handler) gets a
sentinel error `bridge.ErrAuthRevoked`. Main translates that into a
fatal exit:

```go
if errors.Is(err, bridge.ErrAuthRevoked) {
    log.Fatalf("hue-driver: bridge rejected API key — re-pair via button press")
}
```

Exit code is non-zero; the carport supervisor's restart-budget logic
quarantines after a few failed restarts. A `DriverEvent{kind:
"auth_failed", detail: "<bridge_address>"}` lands in the event log
before exit so operators can see the cause via `gohome events tail`.

### 4. Rate limiting

`internal/bridge/ratelimit.go` wraps `*Client` with a
`golang.org/x/time/rate.Limiter` configured for 10 req/sec, burst=10.
Every `SetLight` and `ListLights`/`ListDevices` call awaits a token
before issuing the HTTP request; the limiter is shared across all
goroutines that use the client.

The limiter is a field on `Client`, populated by `bridge.New`:

```go
type Client struct {
    ...
    limiter *rate.Limiter // 10 events/sec, burst 10
}
```

When a request is throttled (`limiter.Wait` blocks > 0ms), the driver
emits a `DriverEvent{kind: "rate_limited", detail: "<ms>"}` once per
window so the operator can see the bridge is being saturated.

### 5. Per-call command timeouts

Every `Client.SetLight` derives a 5-second timeout from its caller
context:

```go
func (c *Client) SetLight(ctx context.Context, id string, u LightUpdate) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    ...
}
```

`ListLights` / `ListDevices` get 10-second timeouts (heavier responses).

The SSE stream context is unchanged — that's a long-lived stream, not a
discrete request.

### 6. EntityRegistered/EntityUnregistered diff on resync

The existing `cmd/hue-driver.resync` function gains a diff step:

```go
func resync(ctx, client, d, cache) error {
    lights, err := client.ListLights(ctx)
    if err != nil { return err }

    // Compute add/remove against cache.hueToID.
    seen := map[string]bool{}
    for _, l := range lights {
        seen[l.ID] = true
        if _, known := cache.hueToID[l.ID]; !known {
            // New bulb. Register and emit StateChanged.
            registerBulb(d, cache, l)
        } else {
            // Existing bulb. Refresh state.
            refreshBulb(d, cache, l)
        }
    }
    for hueID, entityID := range cache.hueToID {
        if !seen[hueID] {
            // Bulb removed.
            d.UnregisterEntity(entityID) // see driverkit change below
            delete(cache.hueToID, hueID)
            delete(cache.byEntID, entityID)
        }
    }
    return nil
}
```

Driverkit gains an `UnregisterEntity(entityID string) error` method that
removes the entry from `d.entities` and emits an `EntityUnregistered`
message on the Run stream. Daemon-side this works without changes — the
state cache already deletes on `EntityUnregistered`.

`registerBulb` is a small helper that does what `buildDriver` does for
each bulb today: `d.AddEntity` + handler registration + cache seeding.
Hot-add is a goroutine path now, which means thread-safety on `d.entities`
matters; the driverkit's existing `d.mu` already guards that.

### 7. Transition time as `duration_ms` arg

`state.CommandToUpdate` gains a uniform parser:

```go
func parseDuration(args map[string]string) (uint32, error) {
    raw, ok := args["duration_ms"]
    if !ok { return 0, nil }
    v, err := strconv.Atoi(raw)
    if err != nil || v < 0 || v > 6_000_000 {
        return 0, fmt.Errorf("duration_ms must be integer 0-6000000, got %q", raw)
    }
    return uint32(v), nil
}
```

When non-zero, `CommandToUpdate` includes `dynamics: {duration: ms}` in
the `LightUpdate`. The Hue v2 API caps transitions at ~6,000,000 ms
(100 minutes), which we mirror.

`bridge.LightUpdate` adds:

```go
type LightUpdate struct {
    On               *OnState
    Dimming          *Dimming
    ColorTemperature *ColorTemperature
    Dynamics         *Dynamics `json:"dynamics,omitempty"`
}

type Dynamics struct {
    Duration uint32 `json:"duration"`
}
```

### 8. Brightness/on semantics

`state.CommandToUpdate` for `set_brightness`:

- If parsed brightness is `0`: return `LightUpdate{On: {On: false}}`,
  no dimming. Bulb turns off; subsequent state shows `On=false,
  Brightness=0`.
- If parsed brightness is `>0`: return `LightUpdate{On: {On: true},
  Dimming: {Brightness: hue}}`. The explicit `On: true` covers the case
  where the bulb was off; the bridge accepts both fields in the same
  PUT.

`set_color_temp` similarly auto-includes `On: true` so a colour-temp
change wakes a bulb that was off. This matches the Hue app's behaviour.

`turn_on` and `turn_off` are unchanged — they only set `On`.

### 9. Structured logging

`cmd/hue-driver/main.go` initializes a `*slog.Logger` with a JSON
handler at startup, using log level from `HUE_LOG_LEVEL` env var
(defaults `info`). All log call sites switch from `log.Printf` to slog.

Standard fields injected on every log line:
- `instance_id` — from `GOHOME_CARPORT_INSTANCE_ID` env (the carport
  protocol already passes this).
- `bridge_address` — from `HUE_BRIDGE_ADDRESS`.

Per-event fields:
- `entity_id` on command/state log lines.
- `hue_id` on bridge-resource log lines.
- `error` on failures (just the error string, not stack traces).

Format defaults to JSON for parseability; `HUE_LOG_FORMAT=text` switches
to a human-readable format for local development.

### 10. DriverEvent emissions

The driver explicitly emits `DriverEvent` messages for ops visibility:

| `kind`              | When                                                                | `detail`                    |
|---------------------|---------------------------------------------------------------------|-----------------------------|
| `bridge_unreachable`| `ListLights` or `SetLight` fails N HTTP-network errors in a row    | `"<addr>: <err>"`           |
| `bridge_recovered`  | First success after `bridge_unreachable`                            | `"<addr>"`                  |
| `sse_reconnected`   | SSE stream reconnects after a drop                                  | `"<downtime ms>"`           |
| `rate_limited`      | Token bucket made the next call wait > 50ms (debounced 1/sec)       | `"<wait ms>"`               |
| `auth_failed`       | Three consecutive 401s within 60s; emitted before fatal exit        | `"<addr>"`                  |
| `bulb_added`        | Resync surfaces a bulb not previously known                         | `"<entity_id>"`             |
| `bulb_removed`      | Resync surfaces a previously-known bulb that has disappeared        | `"<entity_id>"`             |

Driverkit gains an `EmitDriverEvent(kind, detail string) error` method
since drivers currently have no way to emit these.

## Error handling

**Bridge HTTP errors.**
- 401 → counted toward the auth-failure tally; if `ErrAuthRevoked` is
  triggered, all current and future calls return it immediately.
- 429 → log + retry once with 1s sleep. If still 429, return error.
- 5xx → log + return error. Caller decides whether to retry; the SSE
  reconnect loop will, command handlers won't.
- Network errors (timeout, refused) → return error; treat the bridge as
  unreachable. After N=3 consecutive failures, emit `bridge_unreachable`.

**Reachability churn.**
- A bulb flapping between reachable and unreachable → emit
  `StateChanged` for each transition. No debouncing in the driver; the
  daemon's state cache and downstream automations decide what to do.

**Hot-add races.**
- A bulb registered mid-flight while a command for it arrives →
  `OnCommand` returns `entity unknown`. Acceptable; the next command
  will work after registration completes.

**Hot-remove races.**
- A command sent to an entity that just got removed → `entity unknown`.
- A `StateChanged` arriving for a removed bulb → silently dropped (the
  Hue ID is no longer in `hueToID`).

## Lifecycle

The existing `runEventLoop` + `streamOnce` + `resync` structure stands.
Additions:

- `runEventLoop` periodically fires resync even without an SSE drop —
  every 5 minutes — to catch bulbs that were added/removed without
  generating an SSE event we noticed. Cheap.
- On startup, the auth-failure path fails fast: a single 401 from the
  initial `ListLights` exits non-zero immediately. Three-strikes is for
  steady-state, where transient network blips shouldn't be confused
  with key revocation.

Graceful shutdown: on SIGTERM, `cancel()` propagates through the SSE
stream and any in-flight commands, then the driverkit's `Run` returns
when the gRPC stream closes.

## Testing

**Unit (state package).**
- `TestCapabilities` — table-driven over Light fixtures with/without
  `Dimming`, `ColorTemperature`, both, neither. Asserts the capability
  list.
- `TestCommandToUpdate_DurationMs` — adds duration arg cases for each
  capability; verifies `Dynamics.Duration` flows through.
- `TestCommandToUpdate_BrightnessZero` — `set_brightness 0` → off.
  `set_brightness 100` → on + bright.
- `TestLightToAttrs_Available` — passing `available=true/false` flows
  to `Attributes.Available`.

**Unit (bridge package).**
- `TestRateLimiter_Paces` — 11 calls in tight succession; assert the
  11th waited >= 100ms.
- `TestClient_AuthFailureCounting` — three 401s within 60s → next call
  returns `ErrAuthRevoked`. Two 401s outside the window → no
  `ErrAuthRevoked`.
- `TestClient_SetLightTimeout` — fake bridge sleeps 6s; assert
  `SetLight` returns context.DeadlineExceeded after 5s.
- `TestListDevices` — fixture JSON with reachable + unreachable bulbs;
  assert correct correlation between device → light → status.

**Integration (cmd/hue-driver).**
- `TestDriver_HotAddRemove` — fake bridge ListLights returns 1 bulb;
  driver registers it. Bridge swaps to return 0 bulbs; resync triggers;
  driver emits EntityUnregistered. Bridge swaps to return 2 bulbs;
  resync registers both. Verify via `drivertest.Harness`.
- `TestDriver_Brightness0Off` — send `set_brightness brightness=0` to a
  bulb; assert PUT body has `on: {on: false}` and no `dimming`.
- `TestDriver_DurationPassthrough` — send `set_brightness brightness=128
  duration_ms=5000`; assert PUT body has `dynamics: {duration: 5000}`.
- `TestDriver_AuthFailureExits` — fake bridge returns 401 thrice;
  driver process exits non-zero. (Tested via process exit code rather
  than in-process exit; uses the drivertest CLI harness.)

## Open items

None blocking.
