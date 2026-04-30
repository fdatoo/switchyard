# Domain model

gohome's vocabulary is close to Home Assistant's where it helps — entity ids, domains, areas, scenes, scripts — and diverges where HA's model has caused pain. This page defines every noun the system uses, with Pkl examples and a quick-reference comparison table.

## Quick reference: gohome vs. Home Assistant

| Home Assistant | gohome | Why it changed |
|---|---|---|
| Integration | **Driver** + **driver instance** | HA conflates code and config; gohome separates them |
| String state (`"on"`, `"22.5"`) | **Typed state** (`bool`, `float64`, enum) | Strings require runtime parsing; types are validated at load |
| Untyped attribute dict | **Typed attributes** (Pkl class fields) | Dicts have no schema; Pkl fields are validated |
| Services (global verb-space) | **Capabilities** (methods on entity classes) | Global strings are hard to discover and type-check |
| Template entities (second-class) | **Computed entities** (first-class, Starlark-backed) | Template entities are awkward to define and impossible to test |
| Non-hierarchical areas | **Hierarchical areas** (rooms → floors → home) | Flat labels limit spatial reasoning in automations |

## Driver

A **driver** is a separate Go binary that knows how to talk to a category of hardware or cloud services. Examples: `hue-driver`, `z2m-driver`, `matter-driver`.

A driver is just code. It has no opinion about your specific setup — which bridge IP address, which credentials, which rooms. All of that lives in a driver instance.

Drivers communicate with `gohomed` over the **Carport** protocol (gRPC over a unix socket for local drivers, mTLS over TCP for remote edge deployments). The daemon supervises driver processes: launches them, health-checks them, restarts them on failure.

## Driver instance

A **driver instance** is a user-declared binding of a driver binary to specific parameters. You declare one in Pkl:

```pkl
import "gohome:carport" as carport

new HueDriverInstance {
  id        = "hue_main"
  driverName = "hue"
  bridgeHost = "10.0.0.42"
  apiToken   = "env:HUE_TOKEN"
  area       = "living_room"
}
```

The driver/instance split is the most structural difference from HA. In HA, "integration" means both the code (the Python module) and the config (the options you entered in the UI). In gohome, these are separate. You can declare three Hue instances against three different bridges, all using the same `hue` driver binary.

Driver instance IDs are user-chosen slugs (e.g. `hue_main`, `hue_upstairs`). They appear in the event log as the `source` on every event from that instance.

## Device

A **device** is a logical unit surfaced by a driver instance — usually corresponding to a physical thing or a cloud-side resource. Examples: a specific Zigbee bulb, a Hue bridge, a thermostat.

A device belongs to exactly one driver instance. The system assigns an opaque ULID as the internal ID; devices also carry a human-readable `slug` attribute.

Devices are not directly addressed in automations. Their entities are.

## Entity

An **entity** is an addressable, typed property or capability of a device. It is the primary unit of state in gohome.

Every entity has:

- **A unique id** in `domain.name` format (e.g. `light.kitchen_ceiling`, `sensor.outdoor_temp`).
- **Typed state** — not a string, but a real type as declared by the entity class: `bool`, `Float`, an enum, a struct.
- **Typed attributes** — declared as fields on the entity class, not an arbitrary dict blob.
- **Capabilities** — the operations you can invoke on it (e.g. `turn_on`, `set_temperature`).

```pkl
import "gohome:entities" as entities

new entities.Light {
  id                 = "light.kitchen_ceiling"
  friendlyName       = "Kitchen ceiling"
  area               = "kitchen"
  supportsBrightness = true
  supportsColorTemp  = true
}
```

Entity domains are a closed, versioned enum shipped with gohomed: `light`, `switch`, `sensor`, `binary_sensor`, `climate`, `cover`, `media_player`, `camera`, `lock`, `person`, `vacuum`, `fan`, and the `input_*` helpers. The domain set is deliberately close to HA's; HA domains that have become archaic are omitted.

### Typed state

In HA, every entity's state is a string. `light.kitchen` has state `"on"` or `"off"`. `sensor.outdoor_temp` has state `"22.5"`. Parsing and type-checking happens at runtime in every automation and template.

In gohome, state is typed by the entity class. A `Light` entity has state `bool`. A `Sensor` entity has state `Float`. An enum-valued entity has a proper enum type. Automations work with the actual type:

```starlark
# Starlark — state is already a bool, no string comparison needed
if state("light.kitchen_ceiling").state:
    light.kitchen_ceiling.turn_off()

# Numeric state — already a float, no parsing needed
temp = state("sensor.outdoor_temp").state
if temp < 0.0:
    climate.heating.set_temp(target=20.0)
```

### Typed attributes

HA attributes are untyped dicts. You access them by string key and hope the value is what you expect. In gohome, attributes are declared as fields on the entity class and validated when config is loaded:

```pkl
class Light extends Entity {
  supportsBrightness: Boolean = false
  supportsColorTemp:  Boolean = false
}
```

A driver instance declares the entity; the Pkl config system validates that all declared fields match the class definition. Bad configs are caught at `gohome config validate` time, not at runtime at 2am.

### Capabilities

HA's "services" are a global verb-space. You call `light.turn_on` and pass a target entity by string. There is no type-checking; the service either works or it does not.

In gohome, **capabilities** are typed methods on entity classes. To invoke a capability:

```starlark
# Starlark inside an automation — typed arguments, no string dispatching
light.kitchen_ceiling.turn_on(brightness=80, transition=2)
climate.upstairs.set_temp(target=68, mode="heat")
```

Under the hood, the runtime looks up the entity's class, finds the `turn_on` capability, type-checks the arguments, emits a `CommandIssued` event, and the driver receives the command on its Carport stream. The driver acks with `CommandAcknowledged` or `CommandFailed` — both become events in the log.

Both the typed capability form (`e.turn_on()`) and the string-based `call_service(e.id, "turn_on")` work in Starlark contexts. The typed form is preferred — it provides IDE completion and catches typos at config-validate time.

## Entity class

An **entity class** is a Pkl class that defines the type signature of an entity domain. It declares the typed state shape, the typed attributes, and the available capabilities.

gohome ships built-in entity classes in `gohome:entities`: `Light`, `Thermostat`, `Switch`, `Sensor`, `BinarySensor`, and others. Custom entity classes are a first-class extension point — drivers can declare their own Pkl classes in their manifests.

```pkl
module gohome.entities

abstract class Entity {
  id:           String   // "light.kitchen_ceiling"
  friendlyName: String
  area:         String?
}

class Light extends Entity {
  supportsBrightness: Boolean = false
  supportsColorTemp:  Boolean = false
}

class Thermostat extends Entity {
  minTemp: Float = 10.0
  maxTemp: Float = 35.0
}

class Sensor extends Entity {
  unit: String?
}
```

## Computed entity

A **computed entity** is an entity whose state is derived by a Starlark expression over other entities' state. It is re-evaluated reactively whenever any of its inputs change.

Computed entities are first-class in gohome. HA's template entities were second-class — awkward to declare, impossible to unit-test, hidden from most tooling. In gohome, a computed entity is declared exactly like any other entity:

```pkl
import "gohome:entities" as entities

new entities.ComputedEntity {
  id          = "sensor.house_avg_temp"
  entityClass = "gohome.entities.Temperature"
  handler     = """
    avg(s.state for s in entities(class='Temperature', area='interior'))
  """
}
```

The short class name `Temperature` in the `entities()` call is an alias for the fully qualified `gohome.entities.Temperature`. Both forms are accepted.

The Starlark expression runs in a sandboxed context with read-only access to state and a 100ms / 500k-step budget. If it raises or times out, the entity retains its last known good value.

## Area

An **area** is a spatial grouping inside the home. Areas are hierarchical: a room is inside a floor, a floor is inside a home.

```
home
├── ground_floor
│   ├── kitchen
│   ├── living_room
│   └── hallway
└── first_floor
    ├── bedroom_main
    ├── bedroom_guest
    └── bathroom
```

Entities belong to zero or one area. Automations can address all entities in an area:

```starlark
# Turn off all lights in the living room
for e in entities(domain="light", area="living_room"):
    e.turn_off()
```

## Zone

A **zone** is a geographic geofence: a lat/lon centre point and a radius. Zones are used for presence-driven automations.

```pkl
new Zone {
  id     = "zone.home"
  name   = "Home"
  lat    = 51.5074
  lon    = -0.1278
  radius = 100  // metres
}
```

Zones are orthogonal to areas. Areas describe physical space inside a building; zones describe geographic space outside.

## Automation

An **automation** is a trigger + conditions + actions declaration. The structure is declared in Pkl; logic that requires a real language is written in Starlark.

```pkl
import "gohome:automations" as automations

new automations.Automation {
  id       = "auto.lights_off_at_midnight"
  triggers = new {
    new automations.TimeTrigger {
      at = "00:00"
    }
  }
  actions = new {
    new automations.StarlarkAction {
      body = """
        for e in entities(domain="light"):
            e.turn_off()
      """
    }
  }
}
```

Triggers can fire on state changes, time events, incoming webhooks, or arbitrary platform events. Conditions guard execution. Actions call capabilities, apply scenes, run scripts, or execute arbitrary Starlark.

## Script

A **script** is a named, parameterized Starlark procedure. Scripts are callable from automations, the CLI, the web UI, MCP agents, and other scripts.

```pkl
import "gohome:scripts" as scripts

new scripts.Script {
  id     = "script.good_night"
  params = new {
    new scripts.ScriptParam { name = "delay_secs"; type = "int"; required = false; default = "30" }
  }
  handler = """
    scene.apply("scene.night_mode")
    sleep(params["delay_secs"])
    for e in entities(domain="light"):
        e.turn_off()
  """
}
```

Call it from the CLI:

```
gohome script run good_night --param delay_secs=30
```

## Scene

A **scene** is a snapshot of entity states to be applied atomically. Applying a scene computes the diff against current state, dispatches only the needed commands, and emits a single `SceneApplied` event.

```pkl
new Scene {
  id     = "scene.evening"
  states = new Mapping {
    ["light.living_room_main"]  = new LightState { on = true; brightness = 60 }
    ["light.kitchen_ceiling"]   = new LightState { on = false }
    ["climate.living_room"]     = new ThermostatState { target = 20.0 }
  }
}
```

## Dashboard

A **dashboard** is a Pkl-declared collection of pages and widget instances. Dashboards round-trip through the WYSIWYG editor in the web UI: edits in the UI are reflected back to Pkl config, and config changes appear in the UI.

```pkl
import "gohome:dashboards" as dashboards

new dashboards.Dashboard {
  slug  = "main"
  pages = new Listing {
    new dashboards.Page {
      title = "Living room"
      grid  = new dashboards.Grid {
        widgets = new Listing {
          new dashboards.WidgetInstance {
            widgetClass = "EntityToggle"
            props       = new Mapping { ["entityId"] = "light.living_room_main" }
            col = 0; row = 0; w = 2; h = 1
          }
        }
      }
    }
  }
}
```

## Widget

A **widget** is a UI component instance with a typed Pkl config. Standard widget classes ship with gohomed: `Gauge`, `LineChart`, `EntityToggle`, `CameraStream`, `Markdown`, `ScriptButton`. Third-party widget packs are React component bundles paired with Pkl class definitions.

## User, role, and policy

**Users**, **roles**, and **policies** are all declared in Pkl and validated at config load time.

A **user** is a principal with one or more authentication methods (passkey, API token, OIDC). A **role** is a named permission group. Built-in roles are `admin`, `member`, and `guest`; custom roles are supported.

```pkl
import "gohome:auth" as auth

new auth.User {
  slug        = "alice"
  displayName = "Alice"
  roles       = new Listing { "admin" }
}

new auth.Role {
  slug        = "family"
  permissions = new Listing { "entity:read", "entity:write", "scene:apply" }
}
```

A **policy** is a rule that grants or restricts capabilities to roles. Policies are compiled from Pkl at config load time and enforced at the API boundary — every Connect-RPC handler checks policy before executing.

```pkl
new auth.Policy {
  role     = "guest"
  resource = "entity"
  actions  = new Listing { "read" }  // guests can read but not write
}
```

## Identity conventions

| Thing | ID format | Example |
|---|---|---|
| Entity | `domain.name` | `light.kitchen_ceiling` |
| Device | Opaque ULID + human `slug` | `01HQ3XYZABC…` / `hue_bulb_kitchen` |
| Driver instance | User-chosen slug | `hue_main`, `z2m_downstairs` |
| Area | Slug | `living_room`, `first_floor` |
| Zone | `zone.<slug>` | `zone.home`, `zone.office` |
| User | Slug | `alice`, `bob` |
| Automation | `auto.<slug>` | `auto.lights_off_at_midnight` |
| Script | `script.<slug>` | `script.good_night` |
| Scene | `scene.<slug>` | `scene.evening` |
