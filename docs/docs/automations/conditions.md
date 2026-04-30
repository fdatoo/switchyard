# Conditions

!!! status-alpha "Alpha — shipped, interface evolving"

Conditions are evaluated after a trigger fires and before any actions run. **All** conditions must pass — the default composition is AND. If any condition fails, the automation emits `AutomationFinished{CONDITION_FAIL}` and no actions execute.

All condition types extend `automations.Condition`. Place cheap typed conditions before `StarlarkCondition` entries to short-circuit evaluation early.

---

## State conditions

`StateCondition` checks the current state of an entity at the moment the trigger fired.

```pkl
class StateCondition extends Condition {
  entity: String(!isEmpty)    // entity ID to check
  equals: String?             // state equals this value
  oneOf:  Listing<String>?    // state is one of these values
  not:    String?             // state is NOT this value
}
```

Exactly one of `equals`, `oneOf`, or `not` must be set.

**Examples**

Light is currently off:

```pkl
new automations.StateCondition {
  entity = "light.kitchen"
  equals = "off"
}
```

Thermostat mode is heating or cooling:

```pkl
new automations.StateCondition {
  entity = "thermostat.living_room"
  oneOf  = new { "heat" "cool" }
}
```

Guest mode is not active:

```pkl
new automations.StateCondition {
  entity = "input_boolean.guest_mode"
  not    = "on"
}
```

---

## Numeric conditions

`NumericCondition` reads a numeric attribute of an entity and compares it to a threshold.

```pkl
class NumericCondition extends Condition {
  entity:    String(!isEmpty)
  attribute: String = "value"    // attribute key to read
  op:        String              // "lt" | "lte" | "eq" | "gte" | "gt"
  value:     Number
}
```

**Examples**

Temperature is below 18°C:

```pkl
new automations.NumericCondition {
  entity    = "sensor.outdoor_temp"
  attribute = "value"
  op        = "lt"
  value     = 18.0
}
```

Light brightness is 50% or higher:

```pkl
new automations.NumericCondition {
  entity    = "light.living_room_main"
  attribute = "brightness"
  op        = "gte"
  value     = 128
}
```

---

## Time window conditions

`TimeCondition` checks the current time and/or day of week.

```pkl
class TimeCondition extends Condition {
  after:    String?             // "HH:MM" local — current time is after this
  before:   String?             // "HH:MM" local — current time is before this
  weekdays: Listing<String>?    // ["mon","tue","wed","thu","fri","sat","sun"]
}
```

**Examples**

Nighttime window (after 22:00 or before 07:00 — overnight-aware):

```pkl
new automations.TimeCondition {
  after  = "22:00"
  before = "07:00"
}
```

Weekday mornings only:

```pkl
new automations.TimeCondition {
  after    = "06:00"
  before   = "09:00"
  weekdays = new { "mon" "tue" "wed" "thu" "fri" }
}
```

**Tips**

- When `after` is later than `before` in clock time, the window is treated as overnight (e.g. `after = "22:00"`, `before = "06:00"` means 10pm through 6am the next morning).
- Omit `weekdays` to match any day.

---

## Starlark conditions

`StarlarkCondition` evaluates an inline Starlark expression for cases that typed conditions cannot cover. The expression runs in the `condition` context — it has access to `state()`, `event`, and `now()`, but no side-effects (no `call_service`, no `notify`).

```pkl
class StarlarkCondition extends Condition {
  expr: starlark.StarlarkCondition   // single expression; must be truthy/falsy
}
```

**Examples**

Outdoor lux below threshold and hall motion active:

```pkl
new automations.StarlarkCondition {
  expr = """
    state("sensor.outdoor_lux").attributes["value"] < 100
    and state("binary_sensor.hall_motion").state == "on"
  """
}
```

Trigger event is from a specific button on a remote:

```pkl
new automations.StarlarkCondition {
  expr = """
    event != None and event.data.get("button_id") == "top_left"
  """
}
```

**Resource limits:** 50ms wall-clock, 100 000 Starlark steps. Errors and limit breaches are treated as `false` with a warning log — the run is skipped, not aborted.

---

## Composing conditions

### `AndCondition` — all must pass

```pkl
new automations.AndCondition {
  all = new {
    new automations.StateCondition {
      entity = "input_boolean.vacation_mode"
      equals = "off"
    }
    new automations.TimeCondition {
      after  = "08:00"
      before = "22:00"
    }
    new automations.StarlarkCondition {
      expr = "state(\"sensor.outdoor_temp\").attributes[\"value\"] > 15"
    }
  }
}
```

### `OrCondition` — at least one must pass

```pkl
new automations.OrCondition {
  any = new {
    new automations.StateCondition {
      entity = "binary_sensor.front_door"
      equals = "on"
    }
    new automations.StateCondition {
      entity = "binary_sensor.back_door"
      equals = "on"
    }
  }
}
```

### `NotCondition` — inverts the result

```pkl
new automations.NotCondition {
  not = new automations.StateCondition {
    entity = "input_boolean.sleeping"
    equals = "on"
  }
}
```

### Nesting

`AndCondition`, `OrCondition`, and `NotCondition` nest arbitrarily:

```pkl
new automations.AndCondition {
  all = new {
    new automations.TimeCondition { after = "22:00" before = "07:00" }
    new automations.OrCondition {
      any = new {
        new automations.StateCondition { entity = "binary_sensor.hall_motion" equals = "on" }
        new automations.StateCondition { entity = "binary_sensor.stair_motion" equals = "on" }
      }
    }
  }
}
```

---

## Evaluation order and short-circuiting

Conditions are evaluated in the order they are declared. `AndCondition` short-circuits on the first `false`; `OrCondition` short-circuits on the first `true`. Put cheap typed conditions before expensive Starlark ones:

```pkl
conditions = new {
  // Cheap: reads from in-memory state cache
  new automations.StateCondition { entity = "input_boolean.sleeping" not = "on" }
  // Only evaluated if the state check passes
  new automations.StarlarkCondition {
    expr = "state(\"sensor.co2\").attributes[\"value\"] > 800"
  }
}
```

---

## Conditions at a glance

| Pkl class | Checks |
|---|---|
| `StateCondition` | Entity state equals / is one of / is not a value |
| `NumericCondition` | Numeric attribute comparison (lt, lte, eq, gte, gt) |
| `TimeCondition` | Current time within a window, optionally restricted to weekdays |
| `StarlarkCondition` | Arbitrary Starlark expression (read-only, 50ms limit) |
| `AndCondition` | All child conditions pass |
| `OrCondition` | At least one child condition passes |
| `NotCondition` | Child condition does not pass |
