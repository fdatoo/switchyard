# Pkl Module Reference

!!! status-alpha "Alpha — shipped, interface evolving"

switchyard ships eight Pkl modules embedded directly in the `switchyardd` binary. Users import them under the `switchyard:` URI scheme. The LSP resolves them from the local `pkl/switchyard/` directory when `pkl.projectDir` points at `pkl/`.

```pkl
import "switchyard:base"       as base
import "switchyard:entities"   as entities
import "switchyard:automations" as automations
import "switchyard:scripts"    as scripts
import "switchyard:dashboards" as dashboards
import "switchyard:widgets"    as widgets
import "switchyard:auth"       as auth
import "switchyard:starlark"   as starlark
```

---

## `switchyard:base`

Foundational types used by every other module. Import this when you need to declare secrets or metadata inline.

```pkl
module switchyard.base

// Secrets are tagged strings. Go's ResolveSecrets walks the evaluated JSON
// and replaces these with resolved values before applying side-effects.
// Secrets are NEVER written to the event log.
typealias EnvSecret     = String(matches(Regex("env:[A-Z_][A-Z0-9_]*")))
typealias FileSecret    = String(matches(Regex("file:/.+")))
typealias KeyringSecret = String(matches(Regex("keyring:[^/]+/.+")))
typealias Secret        = String(matches(Regex("(env:[A-Z_]|file:/|keyring:).+")))

class Metadata {
  name:   String
  labels: Mapping<String, String> = new {}
}

class RetentionPolicy {
  maxAgeDays: Int?
  maxBytes:   Int?
}
```

### Fields

**`Secret`** — a typealias for `String` constrained to one of three prefix patterns. Use the appropriate prefix directly:

| Typealias | Pattern | Example |
|-----------|---------|---------|
| `EnvSecret` | `env:<UPPER_CASE_VAR>` | `"env:HUE_API_KEY"` |
| `FileSecret` | `file:<absolute-path>` | `"file:/run/secrets/hue_key"` |
| `KeyringSecret` | `keyring:<service>/<account>` | `"keyring:switchyard/hue_key"` |
| `Secret` | any of the above | accepts all three forms |

At config apply time the Go runtime walks the evaluated JSON and replaces every matching tagged string with its resolved plaintext value. Resolved values are never written to the event log.

**`Metadata`**

| Field | Type | Description |
|-------|------|-------------|
| `name` | `String` | Human-readable name |
| `labels` | `Mapping<String, String>` | Arbitrary key/value labels for filtering |

**`RetentionPolicy`**

| Field | Type | Description |
|-------|------|-------------|
| `maxAgeDays` | `Int?` | Retain events for at most this many days (null = unlimited) |
| `maxBytes` | `Int?` | Cap the store at this many bytes (null = unlimited) |

### Example

```pkl
import "switchyard:base" as base

apiKey: base.Secret = "env:PHILIPS_HUE_KEY"
// or using a file:
apiKey: base.Secret = "file:/run/secrets/philips_hue_key"
// or the system keyring:
apiKey: base.Secret = "keyring:switchyard/philips_hue_key"
```

---

## `switchyard:carport`

Driver instance base class. Driver authors extend `DriverInstance` with their own typed configuration fields.

```pkl
module switchyard.carport

abstract class DriverInstance {
  driverName: String
  id:         String
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `driverName` | `String` | Name of the installed driver plugin, e.g. `"switchyard-hue"` |
| `id` | `String` | Unique instance identifier, e.g. `"hue_bridge_1"` |

Drivers publish a Pkl class that extends `DriverInstance`. Users import that class and add their own typed fields (API key, host, port, etc.).

### Example

```pkl
import "switchyard:carport" as carport

// Hypothetical Hue driver class (defined by the driver, not switchyard core):
class HueInstance extends carport.DriverInstance {
  host:   String
  apiKey: String
}
```

---

## `switchyard:entities`

Entity declarations that link physical devices to the switchyard domain model.

```pkl
module switchyard.entities

abstract class Entity {
  id:           String    // dotted-path, e.g. "light.living_room"
  friendlyName: String
  area:         String?   // area slug; null = unassigned
}

class Light extends Entity {
  supportsBrightness: Boolean = false
  supportsColorTemp:  Boolean = false
}

class Thermostat extends Entity {
  minTemp: Float = 10.0
  maxTemp: Float = 35.0
}

class Switch       extends Entity {}
class Sensor       extends Entity { unit: String? }
class BinarySensor extends Entity {}
```

### Fields

**`Entity`** (abstract base for all entity types)

| Field | Type | Description |
|-------|------|-------------|
| `id` | `String` | Dotted-path entity ID: `"<type>.<name>"`. Must be unique across the config. |
| `friendlyName` | `String` | Human-readable display name |
| `area` | `String?` | Area slug this entity belongs to (optional) |

**`Light`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `supportsBrightness` | `Boolean` | `false` | Whether the light supports brightness control |
| `supportsColorTemp` | `Boolean` | `false` | Whether the light supports colour temperature |

**`Thermostat`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `minTemp` | `Float` | `10.0` | Minimum setpoint (Celsius) |
| `maxTemp` | `Float` | `35.0` | Maximum setpoint (Celsius) |

**`Sensor`**

| Field | Type | Description |
|-------|------|-------------|
| `unit` | `String?` | Unit of measure, e.g. `"°C"`, `"lux"`, `"ppm"` |

`Switch` and `BinarySensor` carry no extra fields beyond the `Entity` base.

### Example

```pkl
import "switchyard:entities" as entities

entities: Listing<entities.Entity> = new {
  new entities.Light {
    id           = "light.living_room_ceiling"
    friendlyName = "Living Room Ceiling"
    area         = "living_room"
    supportsBrightness = true
    supportsColorTemp  = true
  }
  new entities.BinarySensor {
    id           = "binary_sensor.front_door"
    friendlyName = "Front Door"
    area         = "entrance"
  }
}
```

---

## `switchyard:automations`

Automation, trigger, condition, and action types. Trigger and Action are abstract base classes with concrete typed subtypes. Starlark fields resolve to `String` at evaluation time; the daemon validates Starlark syntax during `config apply`.

```pkl
module switchyard.automations
import "switchyard:starlark" as starlark

// ── Triggers ─────────────────────────────────────────────────
abstract class Trigger { _type: String }

class StateChangeTrigger extends Trigger {
  _type    = "switchyard.automations#StateChangeTrigger"
  entities: Listing<String(!isEmpty)>
  from:     String?
  to:       String?
  forDur:   Duration?
}
class EventTrigger extends Trigger {
  _type = "switchyard.automations#EventTrigger"
  kind: String(!isEmpty)
  data: Mapping<String, String>?
}
class TimeTrigger extends Trigger {
  _type  = "switchyard.automations#TimeTrigger"
  at:    String?
  cron:  String?
  every: Duration?
}
class WebhookTrigger extends Trigger {
  _type   = "switchyard.automations#WebhookTrigger"
  path:    String(matches(Regex(#"^/[a-zA-Z0-9/_-]+$"#)))
  methods: Listing<String> = new { "POST" }
}

// ── Conditions ───────────────────────────────────────────────
abstract class Condition { _type: String }

class StateCondition extends Condition {
  _type  = "switchyard.automations#StateCondition"
  entity: String(!isEmpty)
  equals: String?
  oneOf:  Listing<String>?
  not:    String?
}
class NumericCondition extends Condition {
  _type      = "switchyard.automations#NumericCondition"
  entity:    String(!isEmpty)
  attribute: String = "value"
  op:        String   // "lt" | "lte" | "eq" | "gte" | "gt"
  value:     Number
}
class TimeCondition extends Condition {
  _type     = "switchyard.automations#TimeCondition"
  after:    String?
  before:   String?
  weekdays: Listing<String>?
}
class StarlarkCondition extends Condition {
  _type = "switchyard.automations#StarlarkCondition"
  expr: starlark.StarlarkCondition
}
class AndCondition extends Condition { _type = "switchyard.automations#AndCondition"; all: Listing<Condition> }
class OrCondition  extends Condition { _type = "switchyard.automations#OrCondition";  any: Listing<Condition> }
class NotCondition extends Condition { _type = "switchyard.automations#NotCondition"; not: Condition }

// ── Actions ──────────────────────────────────────────────────
abstract class Action { _type: String; continueOnError: Boolean = false }

class CallServiceAction extends Action {
  _type      = "switchyard.automations#CallServiceAction"
  entity:     String(!isEmpty)
  capability: String(!isEmpty)
  args:       Mapping<String, String>?
}
class SceneAction  extends Action { _type = "switchyard.automations#SceneAction";  slug: String(!isEmpty) }
class ScriptAction extends Action { _type = "switchyard.automations#ScriptAction"; name: String(!isEmpty); args: Mapping<String, String>? }
class StarlarkAction extends Action { _type = "switchyard.automations#StarlarkAction"; body: starlark.StarlarkScript }
class WaitAction   extends Action { _type = "switchyard.automations#WaitAction";   duration: Duration }
class SequenceBlock extends Action { _type = "switchyard.automations#SequenceBlock"; actions: Listing<Action> }
class ParallelBlock extends Action { _type = "switchyard.automations#ParallelBlock"; actions: Listing<Action> }

// ── Automation ───────────────────────────────────────────────
class Automation {
  id:         String(!isEmpty)
  triggers:   Listing<Trigger>
  conditions: Listing<Condition> = new {}
  actions:    Listing<Action>
  mode:       String = "single"   // "single" | "queued" | "restart" | "parallel"
  maxQueued:  Int = 10
  enabled:    Boolean = true
}
```

### Fields

**`Automation`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `id` | `String` | — | Unique automation identifier (slug) |
| `triggers` | `Listing<Trigger>` | — | One or more triggers that can start this automation |
| `conditions` | `Listing<Condition>` | `new {}` | Optional guards; all must pass before actions run |
| `actions` | `Listing<Action>` | — | Ordered list of actions to execute |
| `mode` | `String` | `"single"` | Concurrency mode: `single`, `queued`, `restart`, or `parallel` |
| `maxQueued` | `Int` | `10` | Maximum queued runs when `mode = "queued"` |
| `enabled` | `Boolean` | `true` | Whether the automation is active on startup |

**Trigger subtypes**

| Class | Key fields |
|-------|------------|
| `StateChangeTrigger` | `entities: Listing<String>`, optional `from`, `to`, `forDur` |
| `EventTrigger` | `kind: String`, optional `data: Mapping<String, String>` |
| `TimeTrigger` | `at` (time string), `cron` (cron expression), or `every: Duration` |
| `WebhookTrigger` | `path: String`, `methods: Listing<String>` (default `["POST"]`) |

**Condition subtypes**

| Class | Key fields |
|-------|------------|
| `StateCondition` | `entity`, optional `equals`, `oneOf`, `not` |
| `NumericCondition` | `entity`, `attribute`, `op` (`lt`/`lte`/`eq`/`gte`/`gt`), `value` |
| `TimeCondition` | optional `after`, `before`, `weekdays` |
| `StarlarkCondition` | `expr: StarlarkCondition` |
| `AndCondition` | `all: Listing<Condition>` |
| `OrCondition` | `any: Listing<Condition>` |
| `NotCondition` | `not: Condition` |

**Action subtypes**

| Class | Key fields |
|-------|------------|
| `CallServiceAction` | `entity`, `capability`, optional `args` |
| `SceneAction` | `slug` |
| `ScriptAction` | `name`, optional `args` |
| `StarlarkAction` | `body: StarlarkScript` |
| `WaitAction` | `duration: Duration` |
| `SequenceBlock` | `actions: Listing<Action>` (sequential) |
| `ParallelBlock` | `actions: Listing<Action>` (parallel) |

All `Action` subtypes inherit `continueOnError: Boolean = false`.

### Example

```pkl
import "switchyard:automations" as automations

automations: Listing<automations.Automation> = new {
  new automations.Automation {
    id = "motion_hall_light"
    triggers = new {
      new automations.StateChangeTrigger {
        entities = new { "binary_sensor.hall_motion" }
        to = "on"
      }
    }
    actions = new {
      new automations.CallServiceAction {
        entity     = "light.hall"
        capability = "turn_on"
      }
    }
  }
}
```

For full trigger/condition/action field sets see the [Automations section](../automations/triggers.md).

---

## `switchyard:dashboards`

Dashboard layout types. Dashboard rendering is not yet implemented (UNIMPLEMENTED in the current release); the Pkl schema is final.

```pkl
module switchyard.dashboards

class WidgetInstance {
  widgetClass: String           // widget class constant, e.g. "Gauge"
  props:       Mapping<String, Any>
  col: Int; row: Int; w: Int; h: Int
}

class Grid  { widgets: Listing<WidgetInstance> }
class Page  { title: String; grid: Grid }
class Dashboard {
  slug:  String
  pages: Listing<Page>
}
```

### Fields

**`WidgetInstance`**

| Field | Type | Description |
|-------|------|-------------|
| `widgetClass` | `String` | Widget class name. Use constants from `switchyard:widgets`. |
| `props` | `Mapping<String, Any>` | Widget-specific configuration (entity ID, label, colour, etc.) |
| `col`, `row` | `Int` | Grid position (zero-indexed column and row) |
| `w`, `h` | `Int` | Width and height in grid cells |

**`Dashboard`**

| Field | Type | Description |
|-------|------|-------------|
| `slug` | `String` | URL-safe identifier for this dashboard |
| `pages` | `Listing<Page>` | Ordered list of pages |

### Example

```pkl
import "switchyard:dashboards" as dashboards
import "switchyard:widgets"    as widgets

dashboards: Listing<dashboards.Dashboard> = new {
  new dashboards.Dashboard {
    slug = "home"
    pages = new {
      new dashboards.Page {
        title = "Overview"
        grid  = new dashboards.Grid {
          widgets = new {
            new dashboards.WidgetInstance {
              widgetClass = widgets.gauge
              props = new { ["entity"] = "sensor.living_room_temp" }
              col = 0; row = 0; w = 2; h = 1
            }
          }
        }
      }
    }
  }
}
```

---

## `switchyard:widgets`

String constants for widget class names. Use these instead of raw strings to catch typos at evaluation time.

```pkl
module switchyard.widgets

const gauge:        String = "Gauge"
const lineChart:    String = "LineChart"
const entityToggle: String = "EntityToggle"
const markdown:     String = "Markdown"
const scriptButton: String = "ScriptButton"
```

| Constant | Value | Description |
|----------|-------|-------------|
| `gauge` | `"Gauge"` | Single-value numeric gauge |
| `lineChart` | `"LineChart"` | Time-series chart |
| `entityToggle` | `"EntityToggle"` | On/off toggle linked to an entity |
| `markdown` | `"Markdown"` | Static Markdown text panel |
| `scriptButton` | `"ScriptButton"` | Button that invokes a named script |

---

## `switchyard:auth`

User, role, and policy declarations for the built-in auth system. Auth enforcement is implemented in C9; the Pkl schema is final.

```pkl
module switchyard.auth

class User {
  slug:        String
  displayName: String
  roles:       Listing<String>
  active:      Boolean = true
}

class Role {
  slug:        String
  permissions: Listing<String>
}

class Policy {
  role:     String
  resource: String
  actions:  Listing<String>
}
```

### Fields

**`User`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `slug` | `String` | — | URL-safe unique identifier, e.g. `"alice"` |
| `displayName` | `String` | — | Human-readable display name |
| `roles` | `Listing<String>` | — | List of role slugs assigned to this user |
| `active` | `Boolean` | `true` | Set to `false` to disable the user without deleting |

**`Role`**

| Field | Type | Description |
|-------|------|-------------|
| `slug` | `String` | Role identifier, e.g. `"admin"`, `"viewer"` |
| `permissions` | `Listing<String>` | Permission strings granted by this role |

**`Policy`**

| Field | Type | Description |
|-------|------|-------------|
| `role` | `String` | Role slug this policy applies to |
| `resource` | `String` | Resource pattern, e.g. `"entity:*"`, `"automation:lights.*"` |
| `actions` | `Listing<String>` | Allowed verbs: `"read"`, `"write"`, `"call"`, `"admin"` |

### Example

```pkl
import "switchyard:auth" as auth

users: Listing<auth.User> = new {
  new auth.User {
    slug        = "alice"
    displayName = "Alice"
    roles       = new { "admin" }
  }
}

roles: Listing<auth.Role> = new {
  new auth.Role {
    slug        = "admin"
    permissions = new { "entity:*:read" "entity:*:call" "config:apply" }
  }
}
```

---

## `switchyard:starlark`

Type aliases for Starlark code fields. All three resolve to `String` at evaluation time; the daemon validates Starlark syntax during `config apply`.

```pkl
module switchyard.starlark

typealias StarlarkExpr      = String   // single expression
typealias StarlarkScript    = String   // multi-statement script body
typealias StarlarkCondition = String   // boolean expression for conditions
```

These aliases are used internally by `switchyard:automations` trigger/condition/action fields. You do not need to import this module directly unless you are building a driver or extension that embeds Starlark code in its own Pkl class.
