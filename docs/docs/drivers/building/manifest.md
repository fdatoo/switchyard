# Driver Manifest

!!! status-alpha "Alpha — shipped, interface evolving"

The driver manifest is a Pkl file that describes the driver to `switchyardd`: its name, version, what entity types it produces, and the typed schema for per-instance configuration. The manifest is embedded in the driver binary at build time.

!!! note "Pkl manifests are a C4 deliverable"
    In v0.x, the Carport handshake carries manifest fields directly from Go code (name, version, supported capabilities). The Pkl-typed manifest (`DriverManifest.pkl_module`) is reserved in the protocol but not yet populated — it ships with C4 (the Pkl config loader milestone). This page documents the intended C4 manifest format so you can design your driver's config schema ahead of time.

---

## `DriverManifest` fields

A `DriverManifest.pkl` file at the root of your driver repository:

```pkl
// DriverManifest.pkl
import "switchyard:driver" as driver

manifest: driver.DriverManifest = new {
  name    = "driver.hue"
  version = "0.4.2"

  description = "Philips Hue bridge integration using the local CLIP API."

  // The typed Pkl class that switchyardd will use to validate per-instance config.
  instanceConfig = HueInstanceConfig

  // Entity types this driver can register.
  produces = new {
    "light"
    "scene"
    "sensor"
  }

  // Driver-typed event kinds emitted via DriverEvent on the Run stream.
  driverEventTypes = new {
    "bridge_unavailable"
    "bridge_reconnected"
    "scene_activated"
  }
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `String` | yes | Reverse-DNS driver identifier, e.g. `driver.hue` |
| `version` | `String` | yes | SemVer driver version |
| `description` | `String` | no | Human-readable driver description |
| `instanceConfig` | `Class` | yes | Pkl class used to validate per-instance configuration |
| `produces` | `Listing<String>` | yes | Entity domain types this driver registers (e.g. `"light"`, `"sensor"`) |
| `driverEventTypes` | `Listing<String>` | no | `DriverEvent.kind` values this driver may emit |

---

## Defining `instanceConfig` in Pkl

The `instanceConfig` field references a typed Pkl class. `switchyardd` will validate each instance's config block against this class before spawning the driver, and will report type errors at `switchyard config validate` time rather than at runtime.

```pkl
// DriverManifest.pkl (continued)

class HueInstanceConfig {
  // Required fields
  bridgeAddress: String
  apiKeyEnv: String

  // Optional fields with defaults
  pollIntervalSeconds: Int(this >= 1 && this <= 300) = 10
  useClipV2: Boolean = true
  tlsSkipVerify: Boolean = true
}
```

Pkl's type system gives you:

- **Constrained integers** — `Int(this >= 1 && this <= 300)` rejects out-of-range values at load time.
- **Typed fields** — `Boolean`, `String`, `Int`, `Duration`, custom types — all validated before the driver binary is spawned.
- **Optional fields with defaults** — omitted fields use the declared default; no runtime nil-checks needed.

The validated config is serialised to JSON and delivered to the driver in `HandshakeRequest.instance_config`. The driver deserialises it with `encoding/json` or any JSON library.

---

## Embedding the manifest in the binary

The manifest Pkl file is compiled to a binary module and embedded in the driver binary using Go's `//go:embed` directive:

```go
// manifest.go
package main

import _ "embed"

//go:embed DriverManifest.pkl.bin
var manifestBytes []byte
```

The compiled Pkl module (`DriverManifest.pkl.bin`) is produced by `pkl compile` as part of your build script or Makefile:

```makefile
.PHONY: build
build: DriverManifest.pkl.bin
    go build -o ./bin/hue-driver ./cmd/hue-driver

DriverManifest.pkl.bin: DriverManifest.pkl
    pkl compile --format binary -o DriverManifest.pkl.bin DriverManifest.pkl
```

At runtime, the driver passes `manifestBytes` back to `switchyardd` via `HandshakeResponse.manifest.pkl_module`. `switchyardd` uses it to validate configs for future instances without spawning the binary.

!!! note
    In v0.x (C3), `pkl_module` is reserved but empty. The Go SDK's `driver.Driver` returns a `DriverManifest` proto with `Name`, `Version`, `ProtocolVersion`, and `SupportedCapabilities` populated from code. The embedding pattern is documented here so you can prepare your build pipeline for C4.

---

## Full manifest example

A complete `DriverManifest.pkl` for the Hue driver:

```pkl
import "switchyard:driver" as driver

class HueInstanceConfig {
  bridgeAddress: String
  apiKeyEnv: String
  pollIntervalSeconds: Int(this >= 1 && this <= 300) = 10
  useClipV2: Boolean = true
  tlsSkipVerify: Boolean = true
}

manifest: driver.DriverManifest = new {
  name        = "driver.hue"
  version     = "0.4.2"
  description = "Philips Hue bridge — local CLIP API, no cloud required."

  instanceConfig   = HueInstanceConfig
  produces         = new { "light"; "scene"; "sensor" }
  driverEventTypes = new { "bridge_unavailable"; "bridge_reconnected" }
}
```

Instance config in `switchyard.pkl` (after C4):

```pkl
import "switchyard:drivers" as drivers

drivers: Listing<drivers.DriverInstance> = new {
  new drivers.DriverInstance {
    id     = "hue_main"
    driver = "driver.hue"
    config = new HueInstanceConfig {
      bridgeAddress    = "10.0.0.42"
      apiKeyEnv        = "HUE_API_KEY"
      pollIntervalSeconds = 30
    }
  }
}
```

Type errors in `config` are caught at `switchyard config validate`, before the driver is ever spawned.
