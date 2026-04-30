# Building Drivers

!!! status-alpha "Alpha — shipped, interface evolving"

A gohome driver is a **standalone binary** that speaks the Carport protocol — gohome's gRPC-based driver IPC contract. The binary connects to `gohomed` over a Unix domain socket, performs a handshake, and then enters a long-lived bidirectional stream where it receives commands and emits state changes.

Drivers run as supervised subprocesses. `gohomed` spawns them, monitors their health, and restarts them on failure. A crashing driver does not affect the daemon or other drivers.

---

## The Carport protocol

Carport (`gohome.carport.v1alpha1`) is the wire contract between `gohomed` and every driver. It defines four RPCs:

| RPC | Direction | Purpose |
|---|---|---|
| `Handshake` | host → driver | Protocol version check, secret verification, manifest exchange, initial entity list |
| `Run` | bidirectional stream | Commands in, state events out, periodic heartbeats |
| `Health` | host → driver | Out-of-band liveness probe |
| `Shutdown` | host → driver | Graceful shutdown request |

The `Run` stream is the main channel. The host sends `Command` messages (e.g. "turn on `light.kitchen` with brightness 80"); the driver responds with `CommandResult` and optionally emits `StateChanged` events as device state updates.

You do not need to implement this protocol from scratch. The Go SDK handles it.

---

## The Go SDK

The [`github.com/fynn-labs/gohome-driverkit`](https://github.com/fynn-labs/gohome-driverkit) module provides everything you need:

| Package | Purpose |
|---|---|
| `driver` | High-level entry point: entity registration, capability handlers, state tracking, reconnect loop |
| `protocol` | Low-level Carport implementation: env ingestion, gRPC server, handshake, stream dispatch |
| `drivertest` | Test harness: in-process fake host, `SendCommand`, `AssertState`, reconnect simulation |

Most driver authors use only the `driver` package. The `protocol` package is available for advanced use cases (custom Health behaviour, wrapping an existing gRPC server).

Install:

```
go get github.com/fynn-labs/gohome-driverkit
```

---

## Local subprocess vs. edge transport

**Local subprocess** (the default) is for drivers running on the same host as `gohomed`. The daemon spawns the binary, passes a Unix socket path and a per-launch secret via environment variables, and connects over that socket. This is the standard deployment for most drivers.

**Edge transport** (C12, planned) is for drivers running on a separate host — for example, a Raspberry Pi with a Z-Wave USB stick in a detached garage, or a Zigbee coordinator on a different subnet. In this case, the driver connects to a `gohome-edge` agent running on the remote host, which forwards the Carport stream over TLS to the primary daemon. Edge transport requires the same driver binary structure; only the connection path changes.

For v0.x, all drivers use the local subprocess transport. The edge transport is additive and will not require changes to driver code.

---

## Basic driver structure

A minimal gohome driver in Go:

```go
package main

import (
    "context"
    "log"

    entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
    "github.com/fynn-labs/gohome-driverkit/driver"
)

func main() {
    d := driver.New("my-driver", "0.1.0")

    if err := d.AddEntity("switch.my_switch", driver.EntitySpec{
        EntityType:   "switch",
        FriendlyName: "My Switch",
        Capabilities: []string{"turn_on", "turn_off"},
    }); err != nil {
        log.Fatal(err)
    }

    d.OnCapability("switch.my_switch", "turn_on",
        func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
            // Send the "on" command to the physical device here.
            return &entityv1.Attributes{
                Kind: &entityv1.Attributes_Switch{Switch: &entityv1.Switch{On: true}},
            }, nil
        })

    d.OnCapability("switch.my_switch", "turn_off",
        func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
            // Send the "off" command to the physical device here.
            return &entityv1.Attributes{
                Kind: &entityv1.Attributes_Switch{Switch: &entityv1.Switch{On: false}},
            }, nil
        })

    log.Fatal(d.Run(context.Background()))
}
```

`d.Run` reads the Carport environment variables injected by `gohomed`, starts the gRPC server, and enters the reconnect loop. It blocks until the context is cancelled.

---

## Next steps

- [Driver manifest](manifest.md) — declaring entities and config schema in Pkl
- [Go SDK walkthrough](go-sdk.md) — detailed API reference with a complete example
- [Lifecycle](lifecycle.md) — how `gohomed` spawns, supervises, and restarts drivers
- [Testing](testing.md) — using `drivertest` to write fast, reliable driver tests
