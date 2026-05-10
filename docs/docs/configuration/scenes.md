# Scenes

!!! status-planned "Planned — scene engine not implemented"

A scene is a named snapshot of desired entity states. The scene engine is not shipped yet: current automation `SceneAction` and Starlark `scene.apply()` calls append bookkeeping scene events but do not resolve target states or dispatch entity commands. Scene declarations, `switchyard scene apply <scene-id>`, and web UI scene application are planned for the Scene engine spec.

## Declaring scenes

```pkl
// scenes.pkl
import "switchyard:scenes" as scenes

scenes: Listing<scenes.Scene> = new {
  new scenes.Scene {
    id          = "night_mode"
    displayName = "Night Mode"
    states      = new {
      // Each entry is an entity id → desired state
      ["light.kitchen_ceiling"] {
        on         = true
        brightness = 30
        color_temp = 2700
      }
      ["light.living_room_main"] {
        on         = true
        brightness = 20
        color_temp = 2400
      }
      ["light.entrance"] {
        on         = false
      }
    }
  }

  new scenes.Scene {
    id          = "movie_mode"
    displayName = "Movie Mode"
    states      = new {
      ["light.living_room_main"] {
        on         = true
        brightness = 10
        color_temp = 2200
      }
      ["light.kitchen_ceiling"] {
        on = false
      }
    }
  }

  new scenes.Scene {
    id          = "all_off"
    displayName = "All Off"
    states      = new {
      ["light.kitchen_ceiling"]   { on = false }
      ["light.living_room_main"]  { on = false }
      ["light.entrance"]          { on = false }
      ["light.upstairs_landing"]  { on = false }
    }
  }
}
```

Import scenes from `main.pkl`:

```pkl
scenes = import("scenes.pkl").scenes
```

## Planned CLI application

```
$ switchyard scene apply night_mode
✓ Scene "Night Mode" applied (3 entities updated)
```

When implemented, the command will set each entity in the scene to the declared state. Entities not listed in the scene are unchanged.

## The `SceneApplied` event

Every successful scene application will append a `SceneApplied` event to the event store. The current stubs append a minimal bookkeeping `scene_applied` event without changing entity state.

```
cursor: 5102
kind:   SceneApplied
scene:  night_mode
by:     user:fdatoo
at:     2026-04-27T22:15:00Z
entities_updated: 3
```

This means scene activations show up in the event timeline alongside state changes, automation firings, and config applies. You can query when a scene was last applied, or trace what was happening in the house around the time someone turned on night mode.

## Using scenes in automations

Scenes can be the action of an automation:

```pkl
new automations.Action {
  kind = "scene"
  body = "night_mode"   // scene id
}
```

Or triggered from a Starlark script via the `scene.apply(id)` built-in, e.g. `scene.apply("scene.night_mode")`.

## Scenes vs. automations

A scene declares a static target state — it has no trigger, condition, or logic. An automation declares triggers and conditions, and can apply a scene as an action. Use scenes for named presets you apply manually or on a fixed schedule; use automations for dynamic behavior that reacts to events.
