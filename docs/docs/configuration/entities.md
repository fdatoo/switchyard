# Entities

!!! status-alpha "Alpha — shipped, interface evolving"

An entity is the primary unit of state in switchyard. Everything you control or monitor — a light, a thermostat, a door sensor — is exposed as one or more entities. Entities have typed state and typed attributes declared via Pkl classes, not string blobs.

## Entity id format

Every entity has a unique id in `domain.name` format:

```
light.kitchen_ceiling
sensor.outdoor_temp
binary_sensor.front_door
switch.living_room_fan
thermostat.upstairs
```

The domain is the entity class family (`light`, `sensor`, `binary_sensor`, `switch`, `thermostat`). The name is a user-readable slug chosen by the driver when it registers the entity, or overridden by you in `entities/overrides.pkl`.

## Standard entity classes

The `switchyard:entities` module defines the built-in classes:

```pkl
module switchyard.entities

abstract class Entity {
  id: String           // "light.kitchen_ceiling"
  friendlyName: String
  area: String?        // area slug, e.g. "kitchen"
}

class Light extends Entity {
  supportsBrightness: Boolean = false
  supportsColorTemp:  Boolean = false
}

class Thermostat extends Entity {
  minTemp: Float = 10.0
  maxTemp: Float = 35.0
}

class Switch extends Entity {}

class Sensor extends Entity {
  unit: String?        // "°C", "lx", "ppm"
}

class BinarySensor extends Entity {}
```

Drivers declare additional entity classes by extending `Entity` and publishing their schema via the Carport manifest. Your Pkl config can import those classes directly.

## Typed state and attributes

Unlike Home Assistant's string state, switchyard entity state is typed at the class level. A `Light`'s state is a structured object — `on: Bool`, `brightness: Int` (0–255), `color_temp: Int` (Kelvin). A `Sensor`'s state is a `Float`. The type is enforced when a driver reports a state change; the event store records a structured value, not a raw string.

This matters for automations. A Starlark condition can write:

```python
entities["light.kitchen_ceiling"].state.brightness > 128
```

Rather than:

```python
int(entities["light.kitchen_ceiling"].attributes["brightness"]) > 128
```

## Entity overrides via `entities/overrides.pkl`

Drivers register entities automatically. If a driver names an entity `light.hue_1` but you want it to appear as `light.kitchen_ceiling`, or if it places an entity in the wrong area, you fix that in `entities/overrides.pkl`:

```pkl
import "switchyard:entities" as entities

overrides: Listing<entities.Entity> = new {
  // Rename and re-room a light
  new entities.Light {
    id           = "light.hue_1"
    friendlyName = "Kitchen Ceiling"
    area         = "kitchen"
  }

  // Remove a noisy sensor from area-based queries.
  // Setting area = null does NOT disable the entity — it still reports state.
  // Filtering unwanted sensors out of dashboards and automations is done via
  // area assignment; a per-entity disable flag is planned.
  new entities.BinarySensor {
    id           = "binary_sensor.hue_connectivity_1"
    friendlyName = "Hue Bridge Connectivity"
    area         = null
  }
}
```

Overrides are merged on top of driver-registered entity metadata. You can override `friendlyName`, `area`, or any class-specific attribute default. You cannot change an entity's class — a `Sensor` cannot be overridden into a `Light`.

## Custom entity classes

If you have a hardware type that doesn't fit the built-in classes, extend `Entity` in a local Pkl module:

```pkl
// my_entities.pkl
import "switchyard:entities" as entities

class AirQualitySensor extends entities.Entity {
  unit: String = "AQI"
  warningThreshold: Int = 100
  dangerThreshold:  Int = 150
}
```

Import and use it in your overrides or in driver config. Custom classes participate in the same typed state and attribute system as built-ins.

## What the event store records

Every time an entity's state changes, the event store appends a `StateChanged` event:

```
cursor: 4821
kind:   StateChanged
source: hue_main
entity: light.kitchen_ceiling
state:  {on: true, brightness: 200, color_temp: 3000}
at:     2026-04-27T07:32:15.823Z
```

The full state value is stored, not a delta. This makes replay and time-travel queries cheap — no need to reconstruct state from patches.
