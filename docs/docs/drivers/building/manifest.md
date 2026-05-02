# Driver Manifest

!!! status-alpha "Alpha — shipped, interface evolving"

Each installed driver ships a `manifest.pkl` next to its binary. The manifest tells `switchyardd`:

- The driver's name (must match the directory name) and version.
- The typed Pkl class operators use to declare instances of this driver.
- Lifecycle defaults (handshake deadlines, restart budget, etc.).
- Optional metadata: description, what entity types the driver produces, what driver-event kinds it emits.

## Installation layout

```
<data-dir>/drivers/
└── hue/
    ├── hue-driver       # the binary
    └── manifest.pkl     # this file
```

- The directory name is the import key. `import "driver:hue"` resolves to `<drivers-root>/hue/manifest.pkl`. switchyardd refuses the manifest if its `name` field doesn't match the directory name.
- The default binary name is `<dir>/<name>-driver` (e.g. `hue/hue-driver`). Override via the manifest's `binary` field — absolute paths stay absolute, relative paths resolve against the driver dir.

## Hue example

```pkl
// drivers/hue/manifest.pkl
extends "switchyard:driver"

const name        = "hue"
const version     = "0.4.2"
description       = "Philips Hue bridge — local CLIP API."
produces          = new { "light"; "scene"; "sensor" }

lifecycleDefaults {
  restartBudgetMax = 10                  // only fields you write override Go defaults
}

class HueInstance extends Instance {
  driverName = name                      // one-line boilerplate; auto-derives the driver name
  bridgeHost: String
  apiToken:   String
}
```

Operators then write:

```pkl
import "driver:hue" as hue

driverInstances = new {
  new hue.HueInstance {
    id         = "hue_main"
    bridgeHost = "10.0.0.42"
    apiToken   = "env:HUE_TOKEN"
  }
}
```

The `driverName = "hue"` field is auto-set on the instance; operators never write it.

## Module-level fields

The manifest amends `switchyard:driver`'s typed surface. All fields are checked at `switchyard config validate` time.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `const String` | yes | Must equal the containing directory name. `const` so per-driver classes can use it in defaults. |
| `version` | `const String` | yes | SemVer driver version. |
| `description` | `String?` | no | Human-readable summary. |
| `produces` | `Listing<String>` | yes | Entity domain types this driver registers (`"light"`, `"sensor"`, …). |
| `driverEventTypes` | `Listing<String>` | no | `DriverEvent.kind` values this driver may emit. Defaults to empty. |
| `binary` | `String?` | no | Override the default binary name. Absolute paths stay absolute; relative paths resolve against the driver dir. |
| `lifecycleDefaults` | `carport.LifecycleOverride` | no | Per-driver overrides for handshake/health/restart timing. Only fields you set override switchyardd's `DefaultLifecycleConfig`. |

## Why `extends` (not `amends`) and `const`

Two Pkl semantics constraints shape the manifest's outer form:

- **`open module` + `extends`.** The manifest needs to declare its own classes (like `HueInstance`). Pkl rejects non-local class declarations under `amends` — `Class needs a 'local' modifier. To define a non-local class, extend rather than amend the parent module (which must be 'open' for extension)`. So `switchyard:driver` is `open module` and manifests use `extends`.

- **`const name` and `const version`.** Pkl class-body identifier resolution uses the lexical scope of the class declaration. Each driver's instance class writes `driverName = name` to auto-derive the driver name from the module — and Pkl requires that `name` be declared `const` to be referenceable from a class default.

These constraints are enforced by the type system, so you'll get a clear Pkl error if you forget either.

## Defining the instance class

`Instance` is the abstract base inherited from `switchyard:driver`. Each driver extends it with its own typed fields:

```pkl
class HueInstance extends Instance {
  driverName = name                                              // boilerplate

  // Required fields
  bridgeHost: String
  apiToken:   String

  // Optional fields with defaults
  pollIntervalSeconds: Int(this >= 1 && this <= 300) = 10
  useClipV2:            Boolean = true
}
```

Pkl's type system gives you:

- **Constrained integers** — `Int(this >= 1 && this <= 300)` rejects out-of-range values at validate time.
- **Typed fields** — `Boolean`, `String`, `Int`, `Duration`, custom types — all validated before the driver binary is spawned.
- **Optional fields with defaults** — omitted fields use the declared default; no runtime nil-checks needed.

The validated config is serialised to JSON and delivered to the driver in `HandshakeRequest.instance_config`. The driver deserialises it with `encoding/json` or any JSON library.

## Lifecycle defaults

All fields are optional; unset fields fall through to switchyardd's `DefaultLifecycleConfig` (5s handshake, 15s health probe, 10/10min restart budget, etc.). Set only the fields where your driver's hardware or protocol genuinely diverges from those defaults — operators can still override per-instance.

```pkl
lifecycleDefaults {
  handshakeDeadline = 30.s     // CLIP v2 first-time discovery is slow
  restartBudgetMax  = 5
}
```

## Embedding the manifest in the binary (follow-on)

The Carport handshake's `DriverManifest.pkl_module` field is reserved for shipping the manifest bytes back to switchyardd at runtime, for cross-checking the on-disk file against what the driver was built with. This isn't wired yet — the on-disk `manifest.pkl` is the only authoritative source today. When the cross-check ships, the embedding pattern will look like:

```go
//go:embed manifest.pkl
var manifestBytes []byte
```

with the driver passing `manifestBytes` back via `HandshakeResponse.manifest.pkl_module`.
