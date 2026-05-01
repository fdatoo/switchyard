# Hue Driver — Design

**Status:** draft
**Date:** 2026-04-30
**Branch:** `feat/hue-driver`

## Summary

`drivers/hue/` is a GoHome Carport driver that mirrors a single Philips Hue
bridge's lights into gohome as `light.*` entities. One driver instance = one
bridge. Talks to the bridge over CLIP v2 (HTTPS + server-sent events). Ships
white and tunable-white control only — color is out of scope until the
`entityv1.Light` proto carries an RGB/XY field.

## Goals

- Discover all lights on a configured Hue bridge at startup and register them
  as `light.*` entities.
- Support `turn_on`, `turn_off`, `set_brightness`, `set_color_temp`.
- Reflect external state changes (wall switch, Hue app) in gohome with
  sub-second latency via SSE.
- Stay up across SSE drops and bridge restarts; resync state on reconnect.

## Non-goals (v0.1)

- Color control (RGB/XY/hue/saturation).
- Groups, rooms, zones, scenes.
- Motion sensors, dimmer switches, temperature sensors.
- CLIP v1 fallback for pre-2021 bridge firmware.
- Hot add/remove of bulbs paired after driver startup.
- Bridge discovery and pairing flows in code (documented as curl recipes
  instead).

## Architecture

### Layout

In the root `github.com/fdatoo/gohome` module:

```
drivers/hue/
  cmd/hue-driver/main.go      # wiring: read config, build driver, register handlers, Run
  internal/bridge/             # HTTP+SSE client to the Hue CLIP v2 API
    client.go                  #   GET/PUT /clip/v2/resource/light[/{id}]
    events.go                  #   SSE reader on /eventstream/clip/v2
  internal/state/              # Hue→gohome mapping (pure, no I/O)
    mapping.go
  README.md
```

The driver depends on `gohome-driverkit/driver` and `gohome/gen/gohome/entity/v1`.
It does not import any `internal/` package from the gohome root module — it
consumes the same public surface a third-party driver would.

### Components

**`internal/bridge.Client`** — HTTPS client for CLIP v2. Holds one `*http.Client`
with the `hue-application-key` header and `InsecureSkipVerify` (configurable,
defaults true since the bridge uses a self-signed cert).

| Method | Purpose |
|---|---|
| `ListLights(ctx) ([]Light, error)` | `GET /clip/v2/resource/light` — startup enumeration and resync. |
| `SetLight(ctx, id, LightUpdate) error` | `PUT /clip/v2/resource/light/{id}` — apply commands. |
| `Events(ctx) (<-chan Event, error)` | Opens `GET /eventstream/clip/v2` (SSE), parses `data:` frames, closes the channel on disconnect or `ctx.Done()`. |

**`internal/state.Mapper`** — pure translation, no I/O.

| Function | Purpose |
|---|---|
| `LightToAttrs(bridge.Light) *entityv1.Attributes` | Full Hue light → `entityv1.Light`. Hue brightness 0-100 float → gohome 0-255 uint32; Hue mirek → `color_temp`. Used for startup enumeration and post-resync. |
| `MergeEvent(prev *entityv1.Light, ev bridge.Event) *entityv1.Attributes` | Merge a partial SSE event into the cached previous state and return the new attributes. Hue v2 events carry only changed fields. |
| `EntityID(bridge.Light) string` | `light.hue_<short_uuid>` from the Hue v2 stable resource UUID (first 8 chars). Survives bulb renames in the Hue app. |
| `CommandToUpdate(capability string, args map[string]string) (bridge.LightUpdate, error)` | Carport command → bridge JSON; validates ranges. |

**State cache.** `main` keeps a `map[entityID]*entityv1.Light` of last-known
state, populated at startup and updated on every command result and SSE event.
The cache exists because SSE events are partial (only changed fields) — without
the previous state, we'd lose unchanged fields on every push. Guarded by a
single mutex.

**`cmd/hue-driver/main.go`** — wiring only. Reads env, constructs the bridge
client, calls `ListLights` once, registers each light with `driver.AddEntity`
plus four `OnCapability` handlers, launches the SSE reader as a background
goroutine, then calls `d.Run`. Target ~150 lines.

## Configuration

Read from environment variables passed by `gohomed`:

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `HUE_BRIDGE_ADDRESS` | yes | — | IP or hostname of the bridge. |
| `HUE_API_KEY` | yes | — | Application key for CLIP v2. User obtains via documented curl flow. |
| `HUE_TLS_SKIP_VERIFY` | no | `true` | Skip TLS verification (bridge ships a self-signed cert). |

The published catalog page at `docs/docs/drivers/first-party.md` is updated to
match: drop `poll_interval_s` and `use_clip_v2`; keep the rest.

## Data flow

**Startup.** `main` → `bridge.ListLights` → for each light,
`state.Mapper.EntityID` + `state.Mapper.LightToAttrs` → `driver.AddEntity` +
four `OnCapability` registrations → `driver.Run`. The handshake serves the
assembled entity list to `gohomed`.

**Command (gohome → bridge).** `OnCapability` handler receives
`(entityID, args)` → `state.Mapper.CommandToUpdate` → `bridge.SetLight` → handler
returns the new `Attributes`. The driverkit emits `StateChanged` automatically
before the `CommandResult`.

**External change (bridge → gohome).** SSE goroutine reads an event from the
`Events` channel → look up cached previous state for the entity →
`state.Mapper.MergeEvent(prev, event)` → write back to cache →
`driver.EmitState(entityID, attrs)`.

**SSE drop.** Goroutine logs, backs off (1s initial, 30s max, exponential),
calls `ListLights` to resync state for all known entities, reopens the stream.
This guards against drift if events were missed during the disconnect.

## Error handling

**Startup errors.** Missing required env vars, unreachable bridge, or bad API
key → log and exit non-zero. `gohomed`'s supervisor handles backoff.

**Command errors.** `bridge.SetLight` failures (HTTP 4xx/5xx, network) propagate
from the `OnCapability` handler as a Go error. The driverkit translates this
into a `CommandResult{Ok: false}` with `CARPORT_INTERNAL`. Bad arguments
(e.g. `brightness=999`) return from `Mapper.CommandToUpdate` and never hit the
network.

**SSE errors.** The reader goroutine never exits except on `ctx.Done()`. A
persistent SSE failure shows up in logs but does not take the driver down —
commands still work via the HTTP client; state just lags.

**State drift.** If a resync surfaces a light not known at startup, or one that
has disappeared, the driver logs a warning and ignores it. Hot entity
registration is out of scope. Documented in the README's "known caveats."

## Lifecycle

The `ctx` passed to `driver.Run` is the root. `main` passes it to the SSE
goroutine; cancellation tears down the stream cleanly. The driverkit's existing
reconnect loop handles gRPC-level disconnects from `gohomed`.

## Testing

**Unit.**

- `state` package: table-driven tests on `LightToAttrs`, `CommandToUpdate`,
  `EntityID`. Pure functions, no I/O.
- `bridge` package: tests use `httptest.NewTLSServer` to stand in for the
  bridge. Verify request shape (path, header, JSON body) for `SetLight`,
  verify SSE parsing on a canned `data:` stream, verify `ListLights` decoding
  against fixture JSON captured from a real bridge.

**Integration.** One test in `cmd/hue-driver/` using the driverkit's
`drivertest` harness plus an `httptest` bridge. Covers each of the four
capabilities on the happy path and one bridge-error case.

**Out of scope.** No real-bridge tests in CI. Fixture JSON is captured once
from a real bridge and checked in.

## Open items

None blocking. Color support, groups/scenes, sensor entities, and CLIP v1
fallback all become candidates once `entityv1` and the catalog mature.
