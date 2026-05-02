# Computed entities

!!! status-alpha "Alpha — shipped, interface evolving"

A computed entity is an entity whose state is derived from other entities rather than reported by a driver. When any of its dependencies change state, the computed entity re-evaluates and publishes a new `StateChanged` event. Computed entities are first-class — they appear in the entity registry, can be used in automation conditions and triggers, and show up in dashboards just like hardware-backed entities.

---

## `ComputedEntity` Pkl class

Computed entities are declared in your entities config using `ComputedEntity`:

```pkl
class ComputedEntity extends Entity {
  entityClass: String?   // fully-qualified entity class, e.g. "switchyard.entities.Temperature"
  handler:     String    // Starlark expression evaluated in the `computed` context
}
```

```pkl
// entities/computed.pkl
import "switchyard:entities" as entities

computed: Listing<entities.ComputedEntity> = new {
  new entities.ComputedEntity {
    id           = "sensor.avg_interior_temp"
    friendlyName = "Average Interior Temperature"
    area         = null   // spans all rooms; no single area
    handler      = """
      temps = [
          state(e.id).attributes["value"]
          for e in entities("sensor")
          if e.area in ("kitchen", "living_room", "bedroom", "office")
          and "value" in state(e.id).attributes
      ]
      if len(temps) == 0:
          None
      else:
          avg(temps)
    """
  }
}
```

Register computed entities from `main.pkl`:

```pkl
computedEntities = import("entities/computed.pkl").computed
```

The `handler` runs in the `KindComputedEntity` context — read-only access to `state()`, `entities()`, `avg()`, and `now()`. Resource limits: 100ms wall-clock, 500 000 steps.

---

## Reactive re-evaluation

The daemon tracks which entities each computed entity reads during evaluation. When any of those source entities publishes a `StateChanged` event, the computed entity re-evaluates automatically. The dependency set is updated on each evaluation — if a handler conditionally reads different entities, the tracker follows.

This means:

- No polling. Computed entities update in reaction to real state changes.
- No explicit dependency declaration. The runtime discovers dependencies by observing which `state()` calls the handler makes.
- If the handler returns `None`, the computed entity's state is cleared (marked unavailable).

---

## Examples

### Average temperature across interior rooms

```pkl
new entities.ComputedEntity {
  id           = "sensor.avg_interior_temp"
  friendlyName = "Average Interior Temperature"
  handler      = """
    sensors = [
        state(e.id).attributes["value"]
        for e in entities("sensor")
        if e.area in ("kitchen", "living_room", "bedroom", "office")
        and state(e.id).attributes.get("unit") == "°C"
    ]
    avg(sensors) if len(sensors) > 0 else None
  """
}
```

Use this in an automation to turn on a fan when the average temperature exceeds a threshold:

```pkl
new automations.StateChangeTrigger {
  entities = new { "sensor.avg_interior_temp" }
}
// Condition: numeric threshold
new automations.NumericCondition {
  entity    = "sensor.avg_interior_temp"
  attribute = "value"
  op        = "gte"
  value     = 26.0
}
```

---

### Presence: someone is home

Derives a binary `home` / `away` state from whether any tracked person is in the home zone.

```pkl
new entities.ComputedEntity {
  id           = "binary_sensor.someone_home"
  friendlyName = "Someone Home"
  handler      = """
    persons = [e for e in entities("person") if state(e.id).state == "home"]
    "on" if len(persons) > 0 else "off"
  """
}
```

Automations can now use a single entity to express presence, regardless of how many people are tracked:

```pkl
new automations.StateChangeTrigger {
  entities = new { "binary_sensor.someone_home" }
  from     = "off"
  to       = "on"
}
```

Apply a welcome scene when someone arrives:

```pkl
actions = new {
  new automations.SceneAction { slug = "arrive_home" }
}
```

---

### Energy total

Sums consumption across all power sensors in the home.

```pkl
new entities.ComputedEntity {
  id           = "sensor.total_power_w"
  friendlyName = "Total Power Consumption"
  handler      = """
    load("//lib/helpers.star", "safe_float")
    readings = [
        safe_float(state(e.id).attributes.get("power_w", 0))
        for e in entities("sensor")
        if state(e.id).attributes.get("unit") == "W"
    ]
    sum(readings)
  """
}
```

```python
# lib/helpers.star
def safe_float(v):
    if type(v) == "int" or type(v) == "float":
        return float(v)
    return 0.0
```

Alert when total draw exceeds a threshold:

```pkl
new automations.NumericCondition {
  entity    = "sensor.total_power_w"
  attribute = "value"
  op        = "gt"
  value     = 5000.0
}
```

---

### Time-aware state

Computed entities can use `now()` to incorporate the current time. Note that time-based computed entities do **not** auto-update — they re-evaluate only when a dependency entity changes. Combine with a `TimeTrigger` on an automation if you need time-driven updates.

```pkl
new entities.ComputedEntity {
  id           = "input_boolean.business_hours"
  friendlyName = "Business Hours Active"
  handler      = """
    t = now()
    weekday = t.weekday()   # 0=Monday, 6=Sunday
    hour    = t.hour
    "on" if weekday < 5 and 9 <= hour < 18 else "off"
  """
}
```

---

## Available built-ins in computed entity context

| Built-in | Description |
|---|---|
| `state(entity_id)` | Current state of an entity (read-only) |
| `entities(domain)` | All registered entities in a domain |
| `avg(values)` | Arithmetic mean of a list of numbers |
| `sum(iterable)` | Sum of a list of numbers (standard Starlark built-in) |
| `now()` | Current UTC time |

No `call_service`, no `notify`, no `sleep` — computed entities are pure functions of current state.

---

## Computed entities vs. automations

| | Computed entity | Automation |
|---|---|---|
| **Trigger** | Any dependency state change | Explicit trigger configuration |
| **Side-effects** | None — pure computation | Yes — calls services, applies scenes |
| **Output** | A new entity state | Event log entries + service calls |
| **Reuse** | Used as input to automations and dashboards | Not directly composable |

Use computed entities to derive clean, reusable state from raw sensor data. Use automations to react to that derived state with actions.
