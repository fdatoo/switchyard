# Drivers

!!! status-alpha "Alpha — shipped, interface evolving"

Driver instances are declared in `drivers.pkl`. Each instance binds a driver binary to a specific set of parameters — credentials, host addresses, poll intervals, and anything else the driver needs to connect to its hardware or cloud service.

## Declaring driver instances

All driver instances live under the `drivers` listing in your config. The `switchyard:carport` module provides the base `DriverInstance` class that every driver extends:

```pkl
module switchyard.carport

abstract class DriverInstance {
  driverName: String   // must match an installed driver binary
  id: String           // user-chosen slug, unique across all instances
}
```

Drivers extend this with their own typed fields. The driver's Pkl class is published in its manifest and imported directly into your config.

## Hue example

```pkl
// drivers.pkl
import "switchyard:base"    as base
import "switchyard:carport" as carport

// Import the Hue driver's typed config class.
// This comes from the driver's installed manifest.
import "driver:hue" as hue

drivers: Listing<carport.DriverInstance> = new {
  new hue.HueInstance {
    id         = "hue_main"
    driverName = "hue"
    bridgeHost = "10.0.0.42"
    apiToken   = "env:HUE_TOKEN"
    area       = "living_room"    // default area for entities from this instance
  }
}
```

The Pkl evaluator will type-check every field. If `bridgeHost` is missing or not a `String`, `switchyard config validate` fails with a clear error pointing at the line.

## Zigbee2MQTT example

```pkl
import "driver:zigbee2mqtt" as z2m

new z2m.Zigbee2MQTTInstance {
  id             = "z2m_main"
  driverName     = "zigbee2mqtt"
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
drivers: Listing<carport.DriverInstance> = new {
  new hue.HueInstance {
    id         = "hue_ground_floor"
    driverName = "hue"
    bridgeHost = "10.0.0.42"
    apiToken   = "env:HUE_GROUND_TOKEN"
    area       = "ground_floor"
  }
  new hue.HueInstance {
    id         = "hue_upstairs"
    driverName = "hue"
    bridgeHost = "10.0.0.43"
    apiToken   = "env:HUE_UPSTAIRS_TOKEN"
    area       = "upstairs"
  }
}
```

Each instance gets its own supervisor slot. Restarting one does not affect the other. The event log records the `source` on every event, so `hue_ground_floor` and `hue_upstairs` events are distinguishable in history.

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
