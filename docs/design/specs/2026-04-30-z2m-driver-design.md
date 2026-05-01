# Zigbee2MQTT Driver — Design

**Status:** draft
**Date:** 2026-04-30
**Branch:** `feat/z2m-driver`

## Summary

`drivers/z2m/` is a GoHome Carport driver that mirrors a single Zigbee2MQTT
deployment's devices into gohome. One driver instance = one Z2M instance
(reached via its MQTT broker). Surfaces three device classes in v0.1: lights
(white, tunable-white, color), numeric sensors (temperature, humidity, etc.),
and binary sensors (occupancy, contact, leak, etc.). Devices added or removed
on the Zigbee network at runtime are reconciled live via `bridge/devices`.

This spec depends on the **entity-binary-sensor** proto change, which lands
first.

## Goals

- Discover all Z2M-paired devices at startup (and after every Z2M network
  change) and register them as gohome entities.
- Surface lights as `light.*` entities with `turn_on`, `turn_off`,
  `set_brightness`, `set_color_temp`, `set_color_rgb` capabilities.
- Surface numeric properties (temperature, humidity, illuminance, battery,
  pressure, ...) as `numeric_sensor.*` entities (read-only).
- Surface binary properties (occupancy, contact, water_leak, smoke, tamper,
  ...) as `binary_sensor.*` entities (read-only).
- Hot add/remove: register entities for newly-paired devices and unregister
  those for removed devices, without driver restart.
- Reflect external state changes (Z2M dashboard, Zigbee remote, etc.) in
  gohome with sub-second latency.
- Stay up across MQTT broker restarts; resync state on reconnect via Z2M's
  retained topics.

## Non-goals (v0.1)

- Z2M network management (pairing, removal, OTA updates, name changes from
  gohome). Documented as Z2M-side recipes instead.
- Scenes, groups, automations defined in Z2M.
- Action sensors (`action: "single"`, `action: "double"`) — these are
  ephemeral events, not state, and want a different proto shape.
- Climate, cover, lock, fan device classes (no proto support yet).
- Switch / smart-plug actuators (writable `state` properties). The `Switch`
  proto variant exists, but plumbing it through this driver is held back
  to v0.2 to keep v0.1 focused. Z2M devices that expose only a writable
  `state` (and no other properties) will register zero entities in v0.1
  with an INFO log. Mixed plugs (state + power) will surface their
  read-only properties only.
- MQTT 5.0 features (request/response correlation). Z2M speaks 3.1.1 anyway.
- Composite-state entities (one entity holding multiple properties). One
  property → one entity is the contract; multi-property devices fan out.

## Architecture

### Layout

In the root `github.com/fdatoo/gohome` module:

```
drivers/z2m/
  cmd/z2m-driver/main.go          # wiring: env → mqtt + driver → Run
  internal/mqtt/                  # paho.mqtt.golang wrapper
    client.go                     #   Connect, Subscribe, Publish, Close, callbacks
  internal/z2m/                   # Z2M topic + payload model (typed)
    topics.go                     #   Topic constructors: bridge/*, <base>/<name>, /set, /availability
    payload.go                    #   Device, Expose, BridgeEvent, BridgeState, StatePayload structs
  internal/state/                 # PURE: Z2M ↔ gohome translation, no I/O
    mapping.go                    #   Z2MDevice → []EntitySpec; per-property mappers; type-driven + blocklist
    merge.go                      #   StatePayload + previous Attributes → new Attributes
    reconcile.go                  #   Reconcile(prev, next []Z2MDevice) []Action
    command.go                    #   capability + args → /set payload
    ids.go                        #   EntityID(ieee, prop) string
  README.md
```

A new shared package `gohome-driverkit/colorconv/` holds the CIE-xy ↔ RGB and
HSV ↔ RGB conversions extracted from the Hue driver. Both Hue and Z2M consume
it. Living in driverkit (not `drivers/internal/`) signals "this is how
drivers — including third-party — are expected to handle color."

The driver depends on `gohome-driverkit/driver`, `gohome-driverkit/colorconv`,
and `gohome/gen/gohome/entity/v1`. It does not import any `internal/` package
from the gohome root module.

### Components

**`internal/mqtt.Client`** — thin wrapper around `eclipse/paho.mqtt.golang`.

| Method | Purpose |
|---|---|
| `Connect(ctx) error` | Connects to broker; configures auto-reconnect. |
| `Subscribe(topic string, h func(topic string, payload []byte))` | Subscribe with handler. Idempotent across reconnects. |
| `Unsubscribe(topic string)` | Drop subscription (used when device removed). |
| `Publish(topic string, payload []byte, retained bool) error` | One-shot publish. |
| `OnConnect(func())`, `OnDisconnect(func(error))` | Callbacks for `main` to (re)assert subscriptions and emit driver events. |
| `Close()` | Disconnect cleanly. |

`paho.mqtt.golang`'s built-in auto-reconnect handles broker churn; the
`OnConnect` callback fires on every successful (re)connect, which `main` uses
to re-subscribe and to emit `EmitDriverEvent("broker_reconnected", ...)`.

**`internal/z2m`** — typed payload model. `Device` decodes the elements of
`zigbee2mqtt/bridge/devices` (a JSON array): `ieee_address`, `friendly_name`,
`type`, `definition.exposes` (recursive tree). `BridgeState` is `online` /
`offline`. `StatePayload` is `map[string]json.RawMessage` because each
device's state shape differs.

**`internal/state.Mapper`** — pure translation, no I/O.

| Function | Purpose |
|---|---|
| `EntitiesFor(d z2m.Device) []EntitySpec` | One device → ≥0 entities. Lights collapse all light properties (state, brightness, color_temp, color) into a single `light.*` entity. **Read-only** numeric properties (`temperature`, `humidity`, `illuminance`, `battery`, `pressure`) become `numeric_sensor.*` entities. **Read-only** binary properties (`occupancy`, `contact`, `water_leak`, `smoke`, `tamper`) become `binary_sensor.*` entities. Mixed devices fan out accordingly — a multi-sensor that exposes occupancy + temperature + humidity + battery yields four entities. Type-driven: walks the `exposes` tree, dispatches on the leaf node `type` (`numeric`, `binary`, `light`), with a blocklist of properties never surfaced (`linkquality`, `voltage`, `update_available`, `last_seen`). Unknown leaf types are skipped with a debug log. **Writable non-light properties (e.g. a smart plug's `state`, identified by the `access` bit) are skipped in v0.1**: they belong as `Switch` actuator entities, but `Switch` is not in the v0.1 device-class scope — see Non-goals. A skipped writable property is logged once at INFO with device + property name so users can see what they're missing. |
| `MergeState(prev *Attributes, prop string, raw json.RawMessage) (*Attributes, error)` | Single property update from a state-topic payload → new Attributes. For lights, multiple property updates accumulate into one Attributes update (the caller iterates the payload's keys). |
| `Reconcile(prev, next []z2m.Device) []Action` | Diff the device lists. Actions are `AddEntity`, `UnregisterEntity`, `UpdateAttrs` (e.g. friendly_name changed → just relabel). Pure function; takes only the device descriptors, returns only the action list. |
| `CommandToPayload(cap string, args map[string]string) (jsonPayload []byte, err error)` | Carport command → Z2M `/set` JSON. e.g. `set_brightness brightness=128` → `{"brightness":128}`. Validates ranges; bad input returns error before the network. |
| `EntityID(ieee string, prop string) string` | Lights: `light.z2m_<last8hex>`. Sensors: `<kind>.z2m_<last8hex>_<prop>`. The last 8 hex of the IEEE address (e.g. `00158d0001234abc` → `01234abc`) is short, stable across friendly_name changes, and human-tractable in logs. |

**Action types** in `state.Reconcile`'s output:

```go
type Action interface{ isAction() }

type AddEntity      struct{ Spec EntitySpec; FriendlyName string; Topics SubscribeTopics }
type UnregisterEntity struct{ EntityID string; Topics UnsubscribeTopics }
type UpdateAttrs    struct{ EntityID string; Attrs *entityv1.Attributes } // for renames, etc.
```

The reconciler emits `Topics` so `main` knows which MQTT topics to
subscribe / unsubscribe alongside the entity registration. The reconciler
itself never touches MQTT.

**State cache.** `main` keeps:

- `entities map[entityID]*entityv1.Attributes` — last-known per-entity state.
- `devices map[ieee]z2m.Device` — last-known device list, fed to the next
  `Reconcile`.
- `entityByTopic map[string][]string` — which entity IDs receive updates from
  a given state topic (multi-property devices fan one topic out to N entities).

Single `sync.Mutex` covers all three. The maps are small (hundreds of
entries) and updates are infrequent.

**`cmd/z2m-driver/main.go`** — wiring only. Reads env, builds the mqtt
client and driver, registers MQTT handlers for `bridge/devices` (drives
reconciliation), `bridge/state` (bridge availability), `bridge/event` (logged
only — informational), `<base>/<name>` (state push), `<base>/<name>/availability`
(per-device availability). Applies reconciler actions via `driver.AddEntity` /
`driver.UnregisterEntity` / `driver.EmitState` plus `mqtt.Subscribe` /
`mqtt.Unsubscribe`. Calls `driver.Run`. Target ~250 lines (more than Hue
because of reconciliation orchestration).

## Configuration

Read from environment variables passed by `gohomed`:

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `Z2M_BROKER_URL` | yes | — | `tcp://host:1883` or `ssl://host:8883` |
| `Z2M_USERNAME` | no | — | MQTT auth username (broker-dependent) |
| `Z2M_PASSWORD` | no | — | MQTT auth password |
| `Z2M_BASE_TOPIC` | no | `zigbee2mqtt` | Z2M's `mqtt.base_topic` setting |
| `Z2M_CLIENT_ID` | no | `gohome-z2m-<random8>` | MQTT client identifier |
| `Z2M_TLS_SKIP_VERIFY` | no | `false` | For self-signed brokers |

The catalog page at `docs/docs/drivers/first-party.md` is updated to add
`z2m` alongside `hue`.

## Data flow

**Startup.** `main` connects MQTT → subscribes `<base>/bridge/devices`
(Z2M publishes this topic retained — the current device list arrives
immediately on subscribe) → subscribes `<base>/bridge/state`,
`<base>/bridge/event`. The first `bridge/devices` payload triggers
`Reconcile(empty, devices)` → all actions are `AddEntity` → for each:
`driver.AddEntity` + register capability handlers + `mqtt.Subscribe` to
the per-device state and availability topics. If Z2M is configured to
publish state retained (config-dependent — recent Z2M versions default
to non-retained), `EmitState` fires automatically as retained payloads
arrive on subscribe; otherwise an entity's `Attributes` stay at the
mapper-assigned defaults until the device's next state change. Either
case is acceptable for v0.1. After the first reconciliation completes,
`main` calls `driver.Run`, which hands off to the driverkit's reconnect
loop and serves the Carport stream to `gohomed`.

**Command (gohome → Z2M).** `OnCapability` handler for a light receives
`(entityID, args)` → `Mapper.CommandToPayload(capability, args)` →
`mqtt.Publish` to `<base>/<friendly_name>/set` with `retained=false`. Handler
returns `nil, nil` — success without state update. The new state echoes back
within ~100ms via the normal state-push path, which emits `StateChanged`.

**State push (Z2M → gohome).** Subscriber on `<base>/<friendly_name>` receives
JSON → look up the device (and its entities) by topic → for each property in
the payload that maps to one of our entities, call `Mapper.MergeState(prev,
prop, raw)` → write back to cache → `driver.EmitState(entityID, attrs)`.

**Hot add/remove.** Z2M republishes the full retained `bridge/devices` array
on every network change. Subscriber receives → decode → `Reconcile(prev_devices,
new_devices)` → apply actions in order:

- `AddEntity`: `driver.AddEntity` + register handlers + `mqtt.Subscribe` per-device.
- `UnregisterEntity`: `mqtt.Unsubscribe` + `driver.UnregisterEntity` (which emits an `EntityUnregistered` event).
- `UpdateAttrs`: `driver.EmitState` with the relabeled attrs (rename case).

Order matters within a single reconcile cycle: subscribe before registering
(so retained state delivery doesn't race the registration). The action list
preserves this ordering.

**Per-device availability.** `<base>/<friendly_name>/availability` (`online` /
`offline`, retained) → look up the device's entities → set `Attributes.Available`
on each → `EmitState`. Z2M only publishes availability if the user enables
it server-side. If we never receive an availability message for a device, we
leave `Available=true` (assuming reachable) — documented as a soft caveat
in the README.

**Bridge state.** `<base>/bridge/state` (`online` / `offline`) → on `offline`,
mark every entity `Available=false` and `EmitDriverEvent("bridge_offline", ...)`.
On return-to-`online`, the next retained `bridge/devices` reconciliation will
restore correct availability per-entity.

**MQTT disconnect.** paho's auto-reconnect runs in the background. On
disconnect: log + `EmitDriverEvent("broker_disconnected", ...)`. On reconnect:
`OnConnect` callback re-asserts subscriptions; retained payloads re-deliver
state automatically; `EmitDriverEvent("broker_reconnected", ...)`.

## Error handling

**Startup.** Missing `Z2M_BROKER_URL`, unreachable broker, auth failure → log
and exit non-zero. `gohomed`'s supervisor handles backoff.

**Command errors.** `Mapper.CommandToPayload` validates args (e.g.
`brightness=999`) before the network and returns Go error → driverkit
translates to `CommandResult{Ok: false, Code: CARPORT_INTERNAL}`. Publish
errors at the broker level are similarly surfaced. **Caveat:** if Z2M ignores
a command silently (invalid friendly_name on the bridge side, device
unreachable), gohome sees a successful publish and reports `Ok: true`. Real
fix is request/response on top of MQTT 5; out of scope for v0.1. Documented
in the README.

**Unparseable `bridge/devices` JSON.** Skip the reconcile cycle, log loudly,
keep prior registrations. Do not wipe the registry on a transient parse
error — Z2M's republish is idempotent and the next valid payload heals.

**Unknown property type in `exposes`.** Type-driven mapper falls through with
a debug log; no entity created for that property. The rest of the device's
properties still register normally.

**State payload references unknown property.** Ignored with a debug log.
Common cause: device firmware update added a property the user's Z2M version
already exposes but our mapper doesn't recognize.

**Per-device availability never seen.** Treat `Available` as `true`
(Z2M-side feature is opt-in). Documented caveat.

## Lifecycle

The `ctx` passed to `driver.Run` is the root. `main` passes it to the MQTT
client and reconciliation goroutine; cancellation tears down both cleanly.
The driverkit's reconnect loop handles gRPC-level disconnects from `gohomed`
independently of the MQTT broker connection.

## Testing

**Unit (`internal/state`).** Table-driven, pure functions, no I/O.

- `EntitiesFor`: canned `Device` fixtures → assert exact `EntitySpec` slice:
  - color light → one `light.*` entity with all four light capabilities.
  - motion multi-sensor (occupancy + temperature + humidity + battery) →
    one `binary_sensor.*` + three `numeric_sensor.*` entities.
  - contact sensor (contact + battery) → one `binary_sensor.*` + one
    `numeric_sensor.*`.
  - smart plug (writable `state` + read-only `power`) → one
    `numeric_sensor.*` for power; the writable `state` is skipped per
    v0.1 scope (verify the INFO log was emitted).
  - device with only `linkquality` + `voltage` (blocked properties) →
    zero entities.
- `MergeState`: per-property merging into a previous `Attributes`.
- `Reconcile`: empty→N (initial), N→N+1 (add), N→N-1 (remove), N→N rename,
  N→N no-op.
- `CommandToPayload`: every capability × happy + invalid-args.
- `EntityID`: known-IEEE → known-output; collision-free across realistic
  fixtures.

**Unit (`internal/z2m`).** Decode captured `bridge/devices` JSON fixtures
from a real Z2M instance (committed to repo, sourced once). Decode
representative state payloads.

**Integration (`cmd/z2m-driver`).** One test using an embedded Go MQTT broker
(`mochi-mqtt/server`) plus the driverkit's `drivertest` harness.

- Seed `bridge/devices` retained → assert correct entities registered.
- Send a `turn_on` command → assert correct `/set` publish → echo state back
  → assert `EmitState` reflected.
- Republish `bridge/devices` with one new device → assert hot-add.
- Republish without a device → assert hot-remove + unsubscribe.
- `bridge/state` `offline` → assert all entities marked unavailable.
- Per-device `availability` `offline` → assert that entity unavailable, others
  unaffected.

**Out of scope.** Real Z2M instance in CI. Real broker in CI. Real Zigbee
hardware. Pairing flow simulation.

## Rollout

This is a separate driver binary (`cmd/z2m-driver`) listed in the catalog
alongside `hue`. No interaction with the Hue driver beyond shared use of
the new `gohome-driverkit/colorconv` package. Lands after the
**entity-binary-sensor** proto spec.

## Open items

None blocking. Action-sensor (`action`) support, climate / cover / lock /
fan device classes, MQTT 5 request/response, and Z2M-side network management
all become candidates as the proto and catalog mature.
