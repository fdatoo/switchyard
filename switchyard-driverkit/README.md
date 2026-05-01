# switchyard-driverkit

Go SDK for writing [switchyard](https://github.com/fdatoo/switchyard) Carport drivers.

## Quick start

```go
package main

import (
    "context"
    "log"

    entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
    "github.com/fdatoo/switchyard-driverkit/driver"
)

func main() {
    d := driver.New("my-driver", "0.1.0")

    d.AddEntity("light.main", driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Main Light",
        Capabilities: []string{"turn_on", "turn_off"},
    })

    d.OnCapability("light.main", "turn_on", func(ctx context.Context, id string, args map[string]string) (*entityv1.Attributes, error) {
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{On: true}}}, nil
    })

    log.Fatal(d.Run(context.Background()))
}
```

Point `switchyardd` at the compiled binary via `drivers.toml`, and it will spawn and supervise your driver automatically.

## Install

```bash
go get github.com/fdatoo/switchyard-driverkit@latest
```

## Packages

| Package | Purpose |
|---|---|
| [`driver`](./driver/) | High-level SDK — start here |
| [`protocol`](./protocol/) | Low-level Carport protocol — for power users |
| [`drivertest`](./drivertest/) | Test helpers: in-process harness and CLI |

## Docs

- [Getting started](./docs/getting-started.md)
- [Testing your driver](./docs/testing.md)
- [Low-level protocol](./docs/protocol.md)

## CLI harness

```bash
go install github.com/fdatoo/switchyard-driverkit/drivertest/cmd/drivertest@latest
drivertest run ./my-driver --scenario happy-path
```

## Carport protocol compatibility

| driverkit | Carport protocol | switchyard |
|---|---|---|
| v0.x | v1alpha1 | v0.1.1+ |
