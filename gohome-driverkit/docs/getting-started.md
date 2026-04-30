# Getting started

This guide walks you from zero to a working Carport driver connected to `gohomed`.

## Prerequisites

- Go 1.25+
- A running `gohomed` instance (C2 or later)

## 1. Create your module

```bash
mkdir my-driver && cd my-driver
go mod init github.com/you/my-driver
go get github.com/fdatoo/gohome-driverkit@latest
```

## 2. Declare entities

Create `main.go`:

```go
package main

import (
    "context"
    "log"

    entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
    "github.com/fdatoo/gohome-driverkit/driver"
)

const entityID = "light.ceiling"

func main() {
    d := driver.New("my-driver", "0.1.0")

    if err := d.AddEntity(entityID, driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Ceiling Light",
        Capabilities: []string{"turn_on", "turn_off"},
    }); err != nil {
        log.Fatal(err)
    }
```

Entity IDs follow the format `<type>.<name>`, matching gohome's domain conventions.

## 3. Register capability handlers

```go
    d.OnCapability(entityID, "turn_on", func(ctx context.Context, id string, args map[string]string) (*entityv1.Attributes, error) {
        // Talk to your device here.
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: true, Brightness: 255},
        }}, nil
    })

    d.OnCapability(entityID, "turn_off", func(ctx context.Context, id string, args map[string]string) (*entityv1.Attributes, error) {
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: false},
        }}, nil
    })
```

The returned `*entityv1.Attributes` becomes the new tracked state and is automatically emitted as a `StateChanged` event. Return `(nil, nil)` to acknowledge success without updating state.

## 4. Handle arg validation

```go
    d.OnCapability(entityID, "set_brightness", func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
        v, err := strconv.Atoi(args["brightness"])
        if err != nil || v < 0 || v > 255 {
            return nil, fmt.Errorf("brightness must be 0-255, got %q", args["brightness"])
        }
        // ... apply to device
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: true, Brightness: uint32(v)},
        }}, nil
    })
```

## 5. Start the driver

```go
    log.Fatal(d.Run(context.Background()))
}
```

`d.Run` reads env vars set by `gohomed` (`GOHOME_CARPORT_SOCKET`, `GOHOME_CARPORT_SECRET`, etc.) and serves in a reconnect loop until the process exits.

## 6. Background state emission

For drivers that poll a device independently of commands:

```go
go func() {
    for range time.Tick(30 * time.Second) {
        state := pollMyDevice()
        if err := d.EmitState(entityID, state); err != nil && err != driver.ErrNotConnected {
            log.Printf("EmitState: %v", err)
        }
    }
}()
```

`ErrNotConnected` is expected when `gohomed` hasn't connected yet; log other errors.

## 7. Build and register

```bash
go build -o my-driver .
```

Add to `$GOHOME_CONFIG_DIR/drivers.toml`:

```toml
[[instance]]
id      = "my_driver"
binary  = "/path/to/my-driver"
enabled = true
```

Restart `gohomed`. Check health:

```bash
gohome driver status my_driver
```

## 8. Write tests

See [testing.md](./testing.md).
