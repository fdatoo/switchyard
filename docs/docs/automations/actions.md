# Actions

!!! status-alpha "Alpha — shipped, interface evolving"

Actions are executed after all conditions pass. The top-level `actions` list is an implicit sequence — each action runs in order, and an error aborts the remainder unless you opt in to `continueOnError`.

All action types extend `automations.Action`:

```pkl
abstract class Action {
  continueOnError: Boolean = false
}
```

---

## Call a capability

`CallServiceAction` dispatches a typed command to an entity's driver.

```pkl
class CallServiceAction extends Action {
  entity:     String(!isEmpty)
  capability: String(!isEmpty)
  args:       Mapping<String, String>?
}
```

**Examples**

Turn a light on at a specific brightness:

```pkl
new automations.CallServiceAction {
  entity     = "light.kitchen"
  capability = "turn_on"
  args       = new { ["brightness"] = "200" ["color_temp"] = "3000" }
}
```

Lock the front door:

```pkl
new automations.CallServiceAction {
  entity     = "lock.front_door"
  capability = "lock"
}
```

Set thermostat target temperature:

```pkl
new automations.CallServiceAction {
  entity     = "thermostat.living_room"
  capability = "set_temperature"
  args       = new { ["temperature"] = "21.5" }
}
```

All `args` values are strings at the Pkl level; the driver's capability schema coerces them to the correct types.

---

## Apply a scene

`SceneAction` applies a named scene, setting all of its entities to their declared states in one operation.

```pkl
class SceneAction extends Action {
  slug: String(!isEmpty)   // scene id
}
```

**Example**

```pkl
new automations.SceneAction { slug = "night_mode" }
```

This is equivalent to `gohome scene apply night_mode`. The scene engine resolves which entities to update; the automation does not need to know them individually.

---

## Run a named script

`ScriptAction` calls a named script. The script runs under the same correlation ID as the automation, so `gohome automation trace <id>` shows the full chain.

```pkl
class ScriptAction extends Action {
  name: String(!isEmpty)
  args: Mapping<String, String>?
}
```

**Example**

```pkl
new automations.ScriptAction {
  name = "notify_residents"
  args = new { ["message"] = "Front door left open" ["priority"] = "high" }
}
```

The `name` must resolve to a declared script (see [Scripts](scripts.md)). An unresolved name is a compile error.

---

## Wait

`WaitAction` pauses the automation for a fixed duration without holding a Starlark thread or blocking other automations.

```pkl
class WaitAction extends Action {
  duration: Duration
}
```

**Examples**

```pkl
new automations.WaitAction { duration = 30.s }
new automations.WaitAction { duration = 5.min }
new automations.WaitAction { duration = 1.h }
```

In `mode = "restart"`, a `WaitAction` is cancelled when a new trigger pre-empts the current run — the `ctx.Done()` channel fires and the wait returns immediately.

---

## Starlark action blocks

`StarlarkAction` runs inline Starlark in the `KindAutomation` context. Use it when you need logic between typed actions — computing a value, branching on runtime state, or using multiple built-ins in sequence.

```pkl
class StarlarkAction extends Action {
  body: starlark.StarlarkScript
}
```

**Examples**

Compute brightness based on time of day and set it:

```pkl
new automations.StarlarkAction {
  body = """
    hour = now().hour
    if hour < 8 or hour > 21:
        brightness = 40
    else:
        brightness = 200
    call_service("light.hall", "turn_on", brightness=brightness)
  """
}
```

Notify and log:

```pkl
new automations.StarlarkAction {
  body = """
    msg = "Motion at " + str(now())
    log(msg)
    notify("user:alice", msg)
  """
}
```

**Available built-ins:** `state()`, `call_service()`, `sleep()`, `now()`, `log()`, `notify()`, `scene.apply()`, `event.fire()`, `random()`, `time`.

**Resource limits:** 30s wall-clock, 10 000 000 steps.

---

## Sequence blocks

`SequenceBlock` groups actions into an explicit named sequence. Useful when building `ParallelBlock` branches that are themselves sequences.

```pkl
class SequenceBlock extends Action {
  actions: Listing<Action>
}
```

**Example**

```pkl
new automations.ParallelBlock {
  actions = new {
    // Branch 1: hall
    new automations.SequenceBlock {
      actions = new {
        new automations.CallServiceAction { entity = "light.hall" capability = "turn_on" }
        new automations.WaitAction { duration = 10.min }
        new automations.CallServiceAction { entity = "light.hall" capability = "turn_off" }
      }
    }
    // Branch 2: porch
    new automations.SequenceBlock {
      actions = new {
        new automations.CallServiceAction { entity = "light.porch" capability = "turn_on" }
        new automations.WaitAction { duration = 10.min }
        new automations.CallServiceAction { entity = "light.porch" capability = "turn_off" }
      }
    }
  }
}
```

---

## Parallel blocks

`ParallelBlock` runs all child actions concurrently. All branches start simultaneously; the block completes when all branches finish (or one fails hard).

```pkl
class ParallelBlock extends Action {
  actions: Listing<Action>
}
```

**Example**

Lock all doors and arm the alarm simultaneously:

```pkl
new automations.ParallelBlock {
  actions = new {
    new automations.CallServiceAction { entity = "lock.front_door"  capability = "lock" }
    new automations.CallServiceAction { entity = "lock.garage_door" capability = "lock" }
    new automations.CallServiceAction { entity = "alarm.home"       capability = "arm_away" }
  }
}
```

**Error handling in parallel blocks:**

- If a child action fails and `continueOnError = false` (default), the parallel block cancels all remaining branches and returns the error.
- If `continueOnError = true`, the branch logs a warning and does not cancel siblings.

---

## Error handling

By default, any action error aborts the rest of the automation and produces `AutomationFinished{OUTCOME_ACTION_ERROR}`. Set `continueOnError = true` on actions where partial failure is acceptable:

```pkl
new automations.CallServiceAction {
  entity          = "light.garden"
  capability      = "turn_on"
  continueOnError = true   // garden light may be offline; continue regardless
}
new automations.CallServiceAction {
  entity = "light.porch"
  capability = "turn_on"
}
```

---

## Actions at a glance

| Pkl class | Effect |
|---|---|
| `CallServiceAction` | Dispatch a typed command to an entity's driver |
| `SceneAction` | Apply a named scene |
| `ScriptAction` | Call a named script (shares correlation ID) |
| `StarlarkAction` | Run Starlark inline (30s / 10M steps) |
| `WaitAction` | Pause for a duration (no Starlark thread held) |
| `SequenceBlock` | Run child actions in order |
| `ParallelBlock` | Run child actions concurrently |
