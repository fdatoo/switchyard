# Go SDK Walkthrough

!!! status-alpha "Alpha — shipped, interface evolving"

The [`github.com/fynn-labs/switchyard-driverkit`](https://github.com/fynn-labs/switchyard-driverkit) Go module is the primary SDK for building switchyard drivers. This page walks through the complete API using a realistic dimmable-light driver as the running example.

```
go get github.com/fynn-labs/switchyard-driverkit
```

---

## Package overview

```
github.com/fynn-labs/switchyard-driverkit
├── driver/       # High-level entry point — start here
├── protocol/     # Low-level Carport implementation (advanced use only)
└── drivertest/   # Test harness
```

**`driver`** implements `protocol.Handler` and adds entity registration, state tracking, capability handler routing, and the reconnect loop. This is what most authors use.

**`protocol`** owns the gRPC server, UDS setup, handshake secret verification, heartbeat, Health, Shutdown, and the Run stream. You use it directly only if you need fine-grained control (custom Health behaviour, wrapping an existing gRPC server).

---

## Creating a driver

```go
d := driver.New("my-driver", "0.1.0")
```

`driver.New` takes the driver name (used in the manifest) and the version string. The returned `*Driver` is the central object for the rest of the setup.

---

## Registering entities

Call `AddEntity` for every entity the driver owns. Do this before calling `Run`.

```go
err := d.AddEntity("light.kitchen", driver.EntitySpec{
    EntityType:   "light",
    FriendlyName: "Kitchen Light",
    Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
})
```

`EntitySpec` fields:

| Field | Type | Description |
|---|---|---|
| `EntityType` | `string` | Closed enum: `"light"`, `"switch"`, `"sensor"`, `"climate"`, etc. |
| `FriendlyName` | `string` | Human-readable name shown in the UI |
| `Capabilities` | `[]string` | Capabilities this entity supports (advertised in the manifest) |

`AddEntity` returns `ErrEntityAlreadyRegistered` if the ID is already taken. Entity IDs must be unique within the driver and follow the convention `<type>.<name>`.

---

## Handling commands with `OnCapability`

Register a handler for each entity+capability pair:

```go
d.OnCapability("light.kitchen", "set_brightness",
    func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error) {
        s := args["brightness"]
        v, err := strconv.Atoi(s)
        if err != nil || v < 0 || v > 255 {
            return nil, fmt.Errorf("brightness must be 0–255, got %q", s)
        }
        // Send to physical device...
        return &entityv1.Attributes{
            Kind: &entityv1.Attributes_Light{
                Light: &entityv1.Light{On: true, Brightness: uint32(v)},
            },
        }, nil
    })
```

`CapabilityHandler` signature:

```go
type CapabilityHandler func(
    ctx context.Context,
    entityID string,
    args map[string]string,
) (*entityv1.Attributes, error)
```

**Return semantics:**

- Return `(*entityv1.Attributes, nil)` — success, tracked state updated to the returned attrs, `StateChanged` event emitted before `CommandResult`.
- Return `(nil, nil)` — success, no state update (useful for write-only commands).
- Return `(nil, err)` — failure, `CommandResult{ok: false, code: CARPORT_INTERNAL, error_message: err.Error()}` sent.

`OnCapability` panics if `entityID` was not registered via `AddEntity`.

---

## Emitting state from background goroutines

For drivers that poll a device or receive push notifications, use `EmitState` to send state changes outside of a command handler:

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            brightness, on, err := queryDevice()
            if err != nil {
                log.Printf("poll error: %v", err)
                continue
            }
            _ = d.EmitState("light.kitchen", &entityv1.Attributes{
                Kind: &entityv1.Attributes_Light{
                    Light: &entityv1.Light{On: on, Brightness: brightness},
                },
            })
        }
    }
}()
```

`EmitState` is safe to call from any goroutine. It returns `ErrNotConnected` if no Run stream is currently active (e.g. during a reconnect). The usual pattern is to log and discard this error — the next poll cycle will retry, and the reconnect will deliver the current state via `initial_entities`.

---

## Sending `CommandResult`

You do not construct `CommandResult` directly. The SDK builds it from your handler's return value:

| Handler returns | CommandResult sent |
|---|---|
| `(attrs, nil)` | `{ok: true}` (after emitting `StateChanged`) |
| `(nil, nil)` | `{ok: true}` (no state update) |
| `(nil, err)` | `{ok: false, code: CARPORT_INTERNAL, error_message: err.Error()}` |

For typed error codes (e.g. `CARPORT_BAD_ARGS`, `CARPORT_DEVICE_OFFLINE`), use the `protocol` package directly or wrap your handler to return a typed error — a helper for this is planned for a future release.

---

## Typed `DriverEvent` payloads

`DriverEvent` is a driver-typed passthrough message for events that are not entity state changes — for example, "bridge went offline" or "pairing mode activated". Emit them via the low-level `protocol.Emitter` in `OnRunStart`:

```go
// Implement protocol.Handler directly, or call EmitRaw (planned helper):
emit.Send(&carportv1alpha1.DriverToHost{
    Kind: &carportv1alpha1.DriverToHost_DriverEvent{
        DriverEvent: &eventv1.DriverEvent{
            Kind:   "bridge_unavailable",
            Detail: "connection refused to 10.0.0.42:80",
        },
    },
})
```

`DriverEvent` kinds should be declared in the driver manifest's `driverEventTypes` list so `switchyardd` can route and display them correctly.

---

## Running the driver

```go
log.Fatal(d.Run(context.Background()))
```

`Run` reads the Carport environment variables injected by `switchyardd`:

| Variable | Description |
|---|---|
| `GOHOME_CARPORT_SOCKET` | Unix domain socket path to listen on |
| `GOHOME_CARPORT_SECRET` | Per-launch handshake secret |
| `GOHOME_CARPORT_INSTANCE_ID` | Instance ID from `drivers.toml` |
| `GOHOME_CARPORT_INSTANCE_CONFIG` | Raw instance config bytes (JSON in v0.x) |

`Run` enters a reconnect loop: if the stream closes (host restart, network glitch), it waits with exponential backoff (1s initial, 30s max) and reconnects. The backoff resets to 1s after a session longer than 5s (healthy session, not a crash loop).

---

## Complete driver skeleton

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strconv"
    "sync"

    entityv1 "github.com/fynn-labs/switchyard/gen/switchyard/entity/v1"
    "github.com/fynn-labs/switchyard-driverkit/driver"
)

const entityID = "light.fake_light"

func main() {
    d := driver.New("fakedevice", "0.1.0")

    var mu sync.Mutex
    var on bool
    var brightness uint32 = 100

    if err := d.AddEntity(entityID, driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Fake Light",
        Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
    }); err != nil {
        log.Fatalf("AddEntity: %v", err)
    }

    d.OnCapability(entityID, "turn_on",
        func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
            mu.Lock()
            on = true
            b := brightness
            mu.Unlock()
            return lightAttrs(true, b), nil
        })

    d.OnCapability(entityID, "turn_off",
        func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
            mu.Lock()
            on = false
            mu.Unlock()
            return lightAttrs(false, 0), nil
        })

    d.OnCapability(entityID, "set_brightness",
        func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
            mu.Lock()
            defer mu.Unlock()
            if s := args["brightness"]; s != "" {
                v, err := strconv.Atoi(s)
                if err != nil || v < 0 || v > 255 {
                    return nil, fmt.Errorf("brightness must be 0-255, got %q", s)
                }
                brightness = uint32(v)
            }
            return lightAttrs(on, brightness), nil
        })

    log.Fatal(d.Run(context.Background()))
}

func lightAttrs(isOn bool, b uint32) *entityv1.Attributes {
    return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
        Light: &entityv1.Light{On: isOn, Brightness: b},
    }}
}
```

This is the `examples/fakedevice` driver from the driverkit repo. It shows:

- Multi-capability routing with shared mutable state protected by a mutex
- Argument validation with a typed error returned (maps to `CARPORT_INTERNAL`)
- The `lightAttrs` helper pattern for constructing typed `Attributes`

---

## Error variables

```go
var (
    ErrEntityAlreadyRegistered = errors.New("entity already registered")
    ErrEntityUnknown           = errors.New("entity unknown")
    ErrNotConnected            = errors.New("no active run stream")
)
```

`ErrEntityAlreadyRegistered` — returned by `AddEntity` if the ID is already registered.

`ErrEntityUnknown` — returned by `EmitState` if the entity ID is not registered.

`ErrNotConnected` — returned by `EmitState` if no Run stream is active.

---

## Next steps

- [Testing drivers](testing.md) — using `drivertest.New` for fast in-process tests
- [Driver lifecycle](lifecycle.md) — how `switchyardd` manages driver processes
