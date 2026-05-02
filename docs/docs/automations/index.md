# Automations & Scripts

!!! status-alpha "Alpha — shipped, interface evolving"

An automation is a rule that fires when a trigger matches, passes a set of conditions, and executes a sequence of actions. Automations are declared in Pkl, with optional Starlark for logic that cannot be expressed as typed fields. Scripts are named, callable Starlark functions that automations, the CLI, and MCP tools can all invoke.

---

## The trigger → conditions → actions shape

Every automation follows the same structure in Pkl:

```pkl
// automations/lights.pkl
import "switchyard:automations" as automations

automations: Listing<automations.Automation> = new {
  new automations.Automation {
    id = "motion_hall_light"

    triggers = new {
      new automations.StateChangeTrigger {
        entities = new { "binary_sensor.hall_motion" }
        from     = "off"
        to       = "on"
      }
    }

    conditions = new {
      new automations.TimeCondition {
        after  = "22:00"
        before = "07:00"
      }
    }

    actions = new {
      new automations.CallServiceAction {
        entity     = "light.hall"
        capability = "turn_on"
        args       = new { ["brightness"] = "80" }
      }
      new automations.WaitAction { duration = 5.min }
      new automations.CallServiceAction {
        entity     = "light.hall"
        capability = "turn_off"
      }
    }
  }
}
```

Import the automations from `main.pkl`:

```pkl
automations = import("automations/lights.pkl").automations
```

**Key properties:**

| Field | Default | Description |
|---|---|---|
| `triggers` | required | One or more triggers; **any** match fires the automation |
| `conditions` | `[]` | All must pass (AND semantics) |
| `actions` | required | Executed as an implicit sequence |
| `mode` | `"single"` | `single`, `queued`, `restart`, `parallel` |
| `maxQueued` | `10` | Queue depth when `mode = "queued"` |
| `enabled` | `true` | Can be toggled at runtime via CLI |

---

## The Pkl + Starlark seam

Typed Pkl fields cover the common cases without any scripting:

- State-change, time, and event triggers
- State, numeric, and time-window conditions
- Call a capability, apply a scene, run a named script, wait

**Inline Starlark** is used when you need logic — a condition that combines multiple entity states, or an action that computes a value before calling a capability:

```pkl
new automations.StarlarkCondition {
  expr = """
    state("sensor.outdoor_lux").attributes["value"] < 100
    and state("binary_sensor.hall_motion").state == "on"
  """
}
```

```pkl
new automations.StarlarkAction {
  body = """
    brightness = int(state("sensor.outdoor_lux").attributes["value"]) // 4
    call_service("light.hall", "turn_on", brightness=max(brightness, 20))
  """
}
```

**Longer Starlark logic** lives in `.star` files under `automations/` and is either `load()`ed inside a `StarlarkAction` body, or registered as a named `Script` and called via `ScriptAction`:

```pkl
// Reference a named script instead of inlining Starlark
new automations.ScriptAction {
  name = "adaptive_brightness"
  args = new { ["entity"] = "light.hall" }
}
```

The convention: if a Starlark block fits on a few lines, keep it inline. If it needs its own tests or is reused by multiple automations, make it a `Script`.

---

## How automations are compiled and registered

When you run `switchyard config apply`, the daemon:

1. Evaluates your Pkl files, producing a typed `ConfigSnapshot`.
2. Compiles the snapshot into runtime automations: each trigger becomes a `Matcher`, each condition an `Evaluator`, each action an `Executor`.
3. Registers triggers with the trigger registry (state-change and event matchers) or the time scheduler (cron/time triggers).
4. Starts listening — inbound events fan out to matched automations immediately.

Reload is surgical: unchanged automations keep their in-flight runs; changed or removed automations are re-registered or cancelled.

**Validation** happens at `switchyard config validate` time. All compile errors — unresolved script names, invalid cron expressions, missing required fields — are reported together:

```
$ switchyard config validate

  automations/lights.pkl:14  automation "motion_hall_light"
    actions[2].script.name "adaptive_brightness" not found in scripts registry
```

---

## Concurrency modes

| Mode | Behaviour |
|---|---|
| `single` (default) | Only one run at a time. New trigger while running → **skipped** |
| `queued` | Runs queue up to `maxQueued`. Overflow → skipped |
| `restart` | Incoming trigger cancels the current run, starts a fresh one |
| `parallel` | Every trigger starts an independent run |

---

## Complete example: light control on motion

This automation turns the hall light on when motion is detected at night, dims it after 2 minutes, and turns it off after another 3 minutes:

```pkl
import "switchyard:automations" as automations

automations: Listing<automations.Automation> = new {
  new automations.Automation {
    id   = "hall_motion_night"
    mode = "restart"    // re-trigger resets the timer

    triggers = new {
      new automations.StateChangeTrigger {
        entities = new { "binary_sensor.hall_motion" }
        to       = "on"
      }
    }

    conditions = new {
      new automations.TimeCondition {
        after  = "21:00"
        before = "08:00"
      }
      new automations.StateCondition {
        entity = "light.hall"
        equals = "off"
      }
    }

    actions = new {
      new automations.CallServiceAction {
        entity     = "light.hall"
        capability = "turn_on"
        args       = new { ["brightness"] = "120" ["color_temp"] = "3000" }
      }
      new automations.WaitAction { duration = 2.min }
      new automations.CallServiceAction {
        entity     = "light.hall"
        capability = "set_brightness"
        args       = new { ["brightness"] = "30" }
      }
      new automations.WaitAction { duration = 3.min }
      new automations.CallServiceAction {
        entity     = "light.hall"
        capability = "turn_off"
      }
    }
  }
}
```

---

## CLI commands

```
switchyard automation list                    # all registered automations
switchyard automation get hall_motion_night   # detail + last 10 runs
switchyard automation trigger hall_motion_night  # fire manually
switchyard automation watch                   # live stream of runs
switchyard automation trace <correlation-id>  # full causal chain for one run
switchyard automation enable|disable <id>     # runtime toggle (in-memory; edit Pkl for durable change)
```
