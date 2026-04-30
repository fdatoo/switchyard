# Areas & Zones

!!! status-alpha "Alpha — shipped, interface evolving"

**Areas** are spatial containers for entities — rooms, floors, and the home itself arranged in a hierarchy. **Zones** are geographic fences used for person tracking (is anyone home? did the kids arrive at school?). Both are declared in Pkl.

## Areas — hierarchical rooms

gohome's area model is hierarchical: rooms live inside floors, floors live inside the home. This matters for policies and automations — a rule that applies to `upstairs` automatically covers `nora_room`, `milo_room`, and any other room nested under it.

A simple three-bedroom home:

```pkl
// areas.pkl
import "gohome:base" as base

areas: Listing<base.Area> = new {
  new base.Area {
    slug        = "home"
    displayName = "Home"
    parent      = null
  }
  new base.Area {
    slug        = "ground_floor"
    displayName = "Ground Floor"
    parent      = "home"
  }
  new base.Area {
    slug        = "upstairs"
    displayName = "Upstairs"
    parent      = "home"
  }
  new base.Area {
    slug        = "kitchen"
    displayName = "Kitchen"
    parent      = "ground_floor"
  }
  new base.Area {
    slug        = "living_room"
    displayName = "Living Room"
    parent      = "ground_floor"
  }
  new base.Area {
    slug        = "nora_room"
    displayName = "Nora's Room"
    parent      = "upstairs"
  }
  new base.Area {
    slug        = "milo_room"
    displayName = "Milo's Room"
    parent      = "upstairs"
  }
}
```

Import this from `main.pkl`:

```pkl
areas = import("areas.pkl").areas
```

### Assigning entities to areas

Assign entities to areas either via driver instance config (applies to all entities from that instance), or per-entity in `entities/overrides.pkl`:

> Imports omitted for brevity — see [Drivers](drivers.md) for the full import pattern.

```pkl
// In drivers.pkl — all entities from this instance go to "kitchen" by default
new hue.HueInstance {
  id         = "hue_kitchen"
  driverName = "hue"
  bridgeHost = "10.0.0.42"
  area       = "kitchen"       // default area for all entities from this instance
  apiToken   = "env:HUE_TOKEN"
}
```

> Imports omitted for brevity — see [Drivers](drivers.md) for the full import pattern.

```pkl
// In entities/overrides.pkl — override a specific entity into a different area
import "gohome:entities" as entities

new entities.Light {
  id   = "light.hue_kitchen_1"
  area = "kitchen"
}
```

Per-entity area assignment wins over the driver instance default.

### Area hierarchy in policies and automations

The area hierarchy is used by the policy compiler. A policy rule with `areas = List("upstairs")` covers all entities in `nora_room`, `milo_room`, and any future rooms added under `upstairs`. You do not need to update the policy when you add a room.

In Starlark automations, you can use `areas.children("upstairs")` to get a list of all areas nested under a given slug.

## Zones — geographic fences

Zones are geographic regions used for presence detection. A zone is a circle defined by a latitude, longitude, and radius. Drivers that track person location (e.g., a mobile app companion driver) emit `zone_entered` and `zone_left` events when a person crosses a zone boundary.

```pkl
// zones.pkl
import "gohome:base" as base

zones: Listing<base.Zone> = new {
  new base.Zone {
    slug        = "home"
    displayName = "Home"
    lat         = 51.5074
    lon         = -0.1278
    radiusM     = 100
  }
  new base.Zone {
    slug        = "work"
    displayName = "Office"
    lat         = 51.5155
    lon         = -0.0922
    radiusM     = 200
  }
  new base.Zone {
    slug        = "school"
    displayName = "Kids School"
    lat         = 51.4995
    lon         = -0.1245
    radiusM     = 150
  }
}
```

Import from `main.pkl`:

```pkl
zones = import("zones.pkl").zones
```

### Zone events

When a tracked person enters or leaves a zone, the event store records:

```
kind:   ZoneEntered
person: fdatoo
zone:   home
at:     2026-04-27T18:22:05Z
```

These events are available to automations as triggers (`kind: "zone_entered"`, `kind: "zone_left"`) and to conditions (`entities["person.fdatoo"].state.zone == "home"`).

### The `home` zone

By convention, the zone with slug `home` is used for presence automations like "turn on the porch light when anyone arrives home" or "arm the alarm when everyone leaves." The zone slug `home` has no special treatment in the engine — it is simply a naming convention that drivers and automations typically agree on.
