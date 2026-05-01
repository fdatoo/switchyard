# Triggers

!!! status-alpha "Alpha — shipped, interface evolving"

A trigger is the condition that causes an automation to fire. Every automation has one or more triggers; if **any** trigger matches, the automation is admitted to run (subject to conditions and the mode gate).

All trigger types are declared as typed Pkl classes extending `automations.Trigger`.

---

## State change triggers

`StateChangeTrigger` fires when an entity transitions between states.

```pkl
class StateChangeTrigger extends Trigger {
  entities: Listing<String(!isEmpty)>  // one or more entity IDs
  from:     String?                    // required prior state; null = any
  to:       String?                    // required new state; null = any
  forDur:   Duration?                  // must hold for this long before firing
}
```

**Examples**

Motion sensor turns on:

```pkl
new automations.StateChangeTrigger {
  entities = new { "binary_sensor.hall_motion" }
  from     = "off"
  to       = "on"
}
```

Any light in the living room turns off (any prior state):

```pkl
new automations.StateChangeTrigger {
  entities = new {
    "light.living_room_main"
    "light.living_room_corner"
  }
  to = "off"
}
```

Door left open for more than 5 minutes (hold duration):

```pkl
new automations.StateChangeTrigger {
  entities = new { "binary_sensor.front_door" }
  to     = "on"    // "on" = open for binary sensors
  forDur = 5.min
}
```

When `forDur` is set, the trigger fires only after the entity has held the target state continuously for the specified duration. Any intervening state change cancels the hold timer and resets the clock.

**Tips**

- Leave `from` and `to` null to trigger on any state change for the entity.
- Multiple entity IDs in one trigger mean "any of these entities changes" — useful for grouping related sensors.
- Unknown entity IDs produce a compile **warning** (not an error) because drivers may register entities after config apply.

---

## Time triggers

`TimeTrigger` fires on a schedule. Exactly one of `at`, `cron`, or `every` must be set.

```pkl
class TimeTrigger extends Trigger {
  at:    String?    // "HH:MM" local time, fires daily
  cron:  String?    // standard 5-field cron expression
  every: Duration?  // repeating interval
}
```

**Daily at a fixed time**

```pkl
new automations.TimeTrigger { at = "07:30" }
```

Fires every day at 07:30 local time.

**Cron expression**

```pkl
new automations.TimeTrigger { cron = "0 8 * * mon-fri" }
```

Fires at 08:00 on weekdays. Uses standard 5-field cron syntax (`minute hour day-of-month month day-of-week`).

```pkl
new automations.TimeTrigger { cron = "*/15 * * * *" }
```

Fires every 15 minutes.

**Repeating interval**

```pkl
new automations.TimeTrigger { every = 30.min }
```

Fires every 30 minutes from daemon startup.

**Tips**

- Time triggers use the daemon's system timezone. Per-trigger timezone override is planned for a future release.
- Combine a `TimeTrigger` with a `TimeCondition` to fire at an interval but only within a time window.
- Sunrise/sunset scheduling is not a `TimeTrigger` variant — a `sun` driver will expose `sun.sunrise` and `sun.sunset` as entity state changes, making a `StateChangeTrigger` the natural fit.

---

## Event triggers

`EventTrigger` fires when a specific event kind appears on the event log. Use this for driver-specific events (button presses, lock codes, alarms) that are not expressed as entity state changes.

```pkl
class EventTrigger extends Trigger {
  kind: String(!isEmpty)          // exact event kind string
  data: Mapping<String, String>?  // optional equality filters on event data
}
```

**Button press from a Hue remote:**

```pkl
new automations.EventTrigger {
  kind = "driver_event.hue.button_press"
  data = new {
    ["button_id"] = "1"
    ["action"]    = "short_release"
  }
}
```

The `data` filter requires **all** listed key/value pairs to match the event data. Omit `data` to fire on any event of that kind regardless of payload.

**Custom event fired from Starlark:**

```pkl
new automations.EventTrigger {
  kind = "home.presence_changed"
}
```

Your Starlark code can fire custom events with `event.fire("home.presence_changed", {"who": "alice"})`. Automations subscribed to that kind fire on the next delivery.

**Tips**

- `kind` is an exact string match. For prefix matching or patterns, write a `StarlarkCondition` that evaluates `event.kind.startswith("driver_event.hue.")`.
- Event triggers receive `event = None` when fired manually via `switchyard automation trigger`; guard with `if event != None:` if you access event data in conditions or actions.

---

## Multiple triggers

List multiple triggers on one automation. Any match fires:

```pkl
triggers = new {
  new automations.StateChangeTrigger {
    entities = new { "binary_sensor.hall_motion" }
    to = "on"
  }
  new automations.EventTrigger {
    kind = "driver_event.doorbell.press"
  }
}
```

This automation fires when the hall motion sensor activates **or** when the doorbell is pressed.

---

## Trigger kinds at a glance

| Pkl class | Fires when |
|---|---|
| `StateChangeTrigger` | Entity transitions state (optionally held for `forDur`) |
| `TimeTrigger` | Fixed time (`at`), cron schedule (`cron`), or interval (`every`) |
| `EventTrigger` | Named event kind appears on the event log |
| `WebhookTrigger` | HTTP POST to a configured path (validated in config; HTTP wiring arrives in a later release) |
