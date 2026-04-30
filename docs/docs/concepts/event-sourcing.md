# Event sourcing

!!! status-alpha "Alpha — shipped, interface evolving"
    The event store is shipped and the core behaviour described here is stable. The `gohome events` CLI surface may change before v1.0.

## Every state change is an event

In most home automation systems, when a light turns on, the system updates a value in a database: `light.kitchen = "on"`. The previous value is gone. You can see what things are right now, but you cannot ask what happened.

gohome works differently. Every state change — a light turns on, a door opens, an automation fires, a command is sent — is written as an **event** appended to an immutable log. Nothing is ever updated or deleted. Current state is computed from the log. "What is the kitchen light right now?" is answered by replaying the events that affected it and returning the final value.

This design is called **event sourcing**.

The event log is stored in SQLite, inside the gohome data directory. It grows over time. The daemon reads it on startup, loads the most recent snapshot, replays events from there, and arrives at current state. All of this happens before the daemon starts accepting connections.

## What you get

### Time-travel debugging

Because the full history is in the log, you can ask what the system looked like at any point in the past:

```
gohome events replay --at "2026-01-15T02:00:00"
```

This replays the event log up to that timestamp and prints the state of all entities at that moment. Useful when you wake up and find that the lights came on at 2am and you want to know why.

You can filter to a specific entity:

```
gohome events replay --at "2026-01-15T02:00:00" --entity light.kitchen_ceiling
```

### A free audit log

Every action in the system — not just state changes, but commands sent to drivers, automations triggered, scenes applied, users logging in — becomes an event. The `source` field on each event records what caused it: `driver:hue_main`, `user:alice`, `automation:auto.lights_off_at_midnight`, `system`.

```
gohome events list --since "2026-01-15T00:00:00" --until "2026-01-15T06:00:00"
```

```
2026-01-15T00:00:01Z  automation_triggered  auto.lights_off_at_midnight        source=automation:auto.lights_off_at_midnight
2026-01-15T00:00:01Z  command_issued        light.kitchen_ceiling turn_off      source=automation:auto.lights_off_at_midnight
2026-01-15T00:00:02Z  command_acknowledged  light.kitchen_ceiling turn_off      source=driver:hue_main
2026-01-15T00:00:02Z  state_changed         light.kitchen_ceiling on=false      source=driver:hue_main
2026-01-15T01:58:43Z  state_changed         light.kitchen_ceiling on=true       source=driver:hue_main
```

That fifth event answers the question immediately: the light turned on at 01:58:43 and the source was the Hue driver directly — not an automation, not a user. Someone physically pressed the switch.

### Startup recovery

If `gohomed` crashes or is restarted, it does not lose any state. It loads the nearest snapshot, replays events forward from that point, and arrives at exactly the correct current state before accepting connections. No stale cache, no inconsistency.

### Commands are in the log too

When gohome sends a command to a driver — `turn_on`, `set_temperature` — a `CommandIssued` event is written. When the driver responds, a `CommandAcknowledged` or `CommandFailed` event is written. This means you can answer questions like: "Did my midnight lights-off automation actually reach every light, or did some commands time out?"

```
gohome events list \
  --kind command_issued,command_failed \
  --since "2026-01-15T00:00:00" \
  --until "2026-01-15T00:05:00"
```

If you see `command_failed` events, you know the commands did not reach the driver. If you see `command_issued` but no `command_acknowledged`, the driver accepted the command but never confirmed — possibly a network issue or a driver bug.

## Worked example: lights on at 2am

You wake up and find the living room lights are on. You check the gohome events log to figure out what happened.

**Step 1: Find when the light turned on.**

```
gohome events list \
  --entity light.living_room_main \
  --kind state_changed \
  --since "2026-01-15T00:00:00" \
  --until "2026-01-15T03:00:00"
```

```
2026-01-15T02:12:07Z  state_changed  light.living_room_main  on=true  brightness=100  source=driver:hue_main
```

The light turned on at 02:12:07. The source is `driver:hue_main`, meaning the Hue driver reported the change — something physically happened (a switch, an app, or a Hue routine), not an automation in gohome.

**Step 2: Confirm no gohome automation was involved.**

```
gohome events list \
  --kind automation_triggered,command_issued \
  --since "2026-01-15T02:10:00" \
  --until "2026-01-15T02:15:00"
```

```
(no results)
```

No automation fired in that window. No commands were issued by gohome. The Hue driver simply reported a state change that originated on the Hue side.

**Step 3: If you want to see what state everything was in at that moment.**

```
gohome events replay --at "2026-01-15T02:12:06"
```

This shows the complete entity state one second before the light turned on — useful if you want to verify whether any other devices changed around the same time.

**Conclusion:** The lights came on because someone (or something) changed the light through the Hue app or a physical switch, not because of a gohome automation. The gohome event log let you reach that conclusion in under two minutes without guessing.

## What it costs

### Disk space

The event log grows over time. gohome writes an event only when entity state actually changes — not on every poll cycle. A driver may poll a device every 30 seconds, but if the device hasn't changed state, no event is written.

A typical prosumer install with ~200 entities, each changing state ~10 times per day on average, produces roughly 730,000 events per year. At ~500 bytes per event in SQLite, that is around **365 MB per year** — well under 1 GB. A realistic range for such an install, accounting for busier periods and larger event payloads, is **300 MB–2 GB/year**.

High-frequency installs — energy monitoring, HVAC systems that cycle frequently, temperature sensors configured to write on every small change — can reach **10+ GB/year** as events accumulate rapidly when entities change state many times per minute.

Those setups should either configure per-kind retention limits or use an external time-series database sink driver.

By default, gohome keeps everything indefinitely. You can configure per-kind retention in Pkl:

```pkl
import "gohome:base" as base

retention = new base.RetentionPolicy {
  maxAgeDays = 365   // keep one year
}
```

Or per event kind (to keep high-frequency sensor data for 30 days but everything else forever):

```pkl
retentionOverrides = new Mapping {
  ["state_changed/sensor"] = new base.RetentionPolicy { maxAgeDays = 30 }
}
```

Rollup / downsampling of high-frequency numeric events into lower-frequency summaries is a planned v1.x feature — the event schema has room for it, but it is not shipped in v1.0.

### Startup time

On startup, gohomed loads the nearest snapshot and replays events forward. At typical scale (200 entities, a few hundred MB to 2 GB/year), this takes a few seconds. Snapshots are taken every 10,000 events or every hour (whichever comes first), so the replay window is bounded.

If you have a very old installation with hundreds of millions of events and no recent snapshot, startup can be slower. The `gohome snapshot create` command forces a new snapshot to be written, capping future replay windows.

## The event log is the source of truth

The in-memory state cache is a derived view of the event log — not the other way around. If the cache and the log disagree, the log wins. The cache is discarded and rebuilt.

This is what makes gohome's state trustworthy: there is one authoritative record, it never goes away, and everything else is derived from it.
