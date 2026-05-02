# Drivers

!!! status-alpha "Alpha — shipped, interface evolving"

Driver instances are declared in `main.pkl` under the `driverInstances` listing. Each instance binds an installed driver to a specific set of parameters — credentials, host addresses, anything else the driver needs to connect to its hardware or cloud service.

## Where drivers live

Each installed driver lives in its own subdirectory under `<data-dir>/drivers/`:

```
~/.local/share/switchyard/drivers/
├── hue/
│   ├── hue-driver       # the binary
│   └── manifest.pkl     # typed config schema, lifecycle defaults, version
└── zigbee2mqtt/
    ├── z2m-driver
    └── manifest.pkl
```

The directory name is the import key: `import "driver:hue"` resolves to `~/.local/share/switchyard/drivers/hue/manifest.pkl`. The directory name **must** match the manifest's `name` field; switchyardd refuses to start otherwise.

By default, the binary is `<dir>/<name>-driver`. The manifest may override this with a `binary` field (absolute or relative to the driver directory).

The drivers root defaults to `<data-dir>/drivers/`. Override with `--drivers-dir <path>` on `switchyardd` and on `switchyard config validate`.

## Declaring driver instances

The `switchyard:carport` module provides the base `DriverInstance` class. Driver authors extend it (transitively, via `switchyard:driver.Instance`) with their own typed fields, declared in the manifest. The driver's typed instance class is what your config imports.

## Hue example

```pkl
// main.pkl
amends "switchyard:config"

import "switchyard:carport" as carport
import "driver:hue"          as hue

driverInstances = new {
  new hue.HueInstance {
    id         = "hue_main"
    bridgeHost = "10.0.0.42"
    apiToken   = "env:HUE_TOKEN"
  }
}
```

The Pkl evaluator type-checks every field. If `bridgeHost` is missing or not a `String`, `switchyard config validate` fails with a clear error pointing at the line. You don't write `driverName = "hue"` — the manifest's `HueInstance` class auto-derives it.

## Zigbee2MQTT example

```pkl
import "driver:zigbee2mqtt" as z2m

new z2m.Zigbee2MQTTInstance {
  id             = "z2m_main"
  mqttHost       = "10.0.0.10"
  mqttPort       = 1883
  mqttUsername   = "switchyard"
  mqttPassword   = "env:Z2M_MQTT_PASSWORD"
  baseTopic      = "zigbee2mqtt"
  permitJoinOnStart = false
}
```

## Multiple instances of the same driver

You can declare as many instances of any driver as you like — for example, two Hue bridges in a large home:

```pkl
driverInstances = new {
  new hue.HueInstance {
    id         = "hue_ground_floor"
    bridgeHost = "10.0.0.42"
    apiToken   = "env:HUE_GROUND_TOKEN"
  }
  new hue.HueInstance {
    id         = "hue_upstairs"
    bridgeHost = "10.0.0.43"
    apiToken   = "env:HUE_UPSTAIRS_TOKEN"
  }
}
```

Each instance gets its own supervisor slot. Restarting one does not affect the other. The event log records the `source` on every event, so `hue_ground_floor` and `hue_upstairs` events are distinguishable in history.

## Disabling an instance

Set `enabled = false` on any instance to keep it in your config without spawning the driver. switchyardd evaluates and tracks the instance but never registers it with the supervisor.

```pkl
new hue.HueInstance {
  id         = "hue_upstairs"
  enabled    = false
  bridgeHost = "10.0.0.43"
  apiToken   = "env:HUE_UPSTAIRS_TOKEN"
}
```

## Per-instance lifecycle overrides

Each driver's `manifest.pkl` ships sensible lifecycle defaults (handshake deadline, restart budget, etc.). Operators may override individual fields per-instance with a `lifecycle` block:

```pkl
new hue.HueInstance {
  id         = "hue_main"
  bridgeHost = "10.0.0.42"
  apiToken   = "env:HUE_TOKEN"
  lifecycle  = new carport.LifecycleOverride {
    restartBudgetMax = 20      // bump from manifest default
  }
}
```

Only fields you explicitly set override the manifest default; everything else falls through to either the manifest's `lifecycleDefaults` (set by the driver author) or, if those don't set it either, to switchyardd's hard-coded `DefaultLifecycleConfig`.

## Secret references in driver config

Never put secrets directly in Pkl source. Use one of the three secret reference types from `switchyard:base`:

```pkl
// From an environment variable
apiToken = "env:HUE_TOKEN"

// From a file on disk (e.g. written by a secrets manager)
apiToken = "file:/run/secrets/hue_token"

// From the system keyring (macOS Keychain, Linux Secret Service)
apiToken = "keyring:switchyard/hue_token"
```

Secret references are opaque to Pkl — the evaluator serializes them as tagged strings. The Go runtime resolves them to plaintext at apply time, immediately before handing config to the carport supervisor. Resolved secret values are never written to the event store or printed in diff output.

See [Secrets](secrets.md) for the full reference on all secret source types.

## Diff-based restart

When you run `switchyard config apply`, only driver instances whose configuration hash has changed are restarted. The hash covers the entire serialized config of the instance, including resolved secret values (but the hash is computed before any secret resolution, on the tagged-string form — so a secret rotation that keeps the same `env:VAR_NAME` reference does not trigger a restart).

If you add a new driver instance, only the new instance is started. If you remove one, only that one is stopped. Unchanged instances are untouched.
