# gohome vs. Home Assistant

Home Assistant is a mature, widely-deployed project with a large ecosystem. This page is not a dismissal of it. If HA works well for you, there is no compelling reason to switch.

This page is for people who have outgrown HA's model and want to understand what gohome does differently, what it deliberately keeps from HA, and what you give up by switching.

## What gohome fixes

These are design decisions in HA that gohome makes differently. In most cases the HA approach reflects reasonable constraints from when that design was established; gohome is newer and can make different tradeoffs.

| Home Assistant | gohome |
|---|---|
| **Integration** — a single concept for both driver code and per-instance configuration | **Driver + driver instance** — separate binary (code) and config declaration. Multiple instances of the same driver are supported naturally. |
| **String state** — entity state is always a string (`"on"`, `"off"`, `"22.5"`) | **Typed state** — `bool`, `float64`, enum, struct, as declared by the entity class |
| **Untyped attributes** — a dict blob attached to an entity, no schema | **Typed attributes** — declared as fields on the Pkl entity class, validated at config load |
| **Services** — a global verb-space dispatched by string (`light.turn_on`, `climate.set_temperature`) | **Capabilities** — typed methods on entity classes, discovered from the class definition |
| **Jinja templates** — templating embedded in YAML, limited language, runtime errors | **Starlark** — a real (if sandboxed) programming language, syntax-validated at `config validate` time |
| **Template entities** — second-class, awkward to define, hard to test | **Computed entities** — first-class, Starlark-backed, reactively re-evaluated |
| **Non-hierarchical areas** — string labels attached to entities, no hierarchy | **First-class geometry** — hierarchical areas (rooms within floors) and geofenced zones as distinct concepts |

The driver/driver-instance split is the most structural difference. In HA, "integration" conflates the code (the Python module) with the config (the credentials and options you enter). In gohome, a driver is just a binary; a driver instance is a Pkl declaration that binds that binary to specific parameters. You can run three Hue instances against three bridges using one driver binary. The concepts are cleanly separate.

The typed entity model flows from this. Because entity classes are declared in Pkl (the driver manifest specifies which entity classes it produces), the types are machine-checkable at config load time rather than discovered at runtime by inspecting dict blobs.

## What gohome keeps from HA

gohome deliberately preserves the parts of HA's model where familiarity helps and the model is sound. If you have used HA, these will feel familiar:

- **Entity domains** — `light`, `switch`, `sensor`, `binary_sensor`, `climate`, `cover`, `media_player`, `camera`, `lock`, `person`, `vacuum`, `fan`, and the `input_*` helpers. The domain set is a closed, versioned enum; HA domains that have become archaic are omitted.
- **Entity id format** — `domain.name`, e.g. `light.kitchen`, `sensor.outdoor_temp`. Same convention, same intuition.
- **Areas** — spatial groupings of entities. The concept is the same; gohome adds hierarchy.
- **Scenes** — declarative target state. Applying a scene computes the diff against current state and dispatches the needed commands.
- **Scripts** — callable named procedures. In gohome they are Starlark functions registered under a name with typed parameters.
- **Automation trigger / condition / action shape** — the three-part structure of automations is preserved. The implementation underneath is different (Pkl for structure, Starlark for logic) but the mental model transfers.
- **Persons** — user-associated presence tracking entities.
- **Zones as geofences** — lat/lon + radius zones used for presence-driven automations.

If you have been thinking in HA's vocabulary for years, you do not need to relearn the basic concepts. The renames and reshapings are targeted at specific places where the model has caused pain, not a wholesale reimagining.

## What you give up

This list is honest. These are real limitations of gohome v1.0.

**No HA API compatibility shim.** gohome does not implement the HA REST or WebSocket API. Anything that talks to HA as a backend — HA's mobile app, third-party dashboards that use the HA API, HA Cloud services — will not work with gohome without modification. This is explicitly deferred and may be addressed in a later version if there is sufficient demand, but it is not planned for v1.0.

**HACS integrations do not transfer.** HACS (Home Assistant Community Store) custom integrations are Python code that runs inside HA's runtime. They have no equivalent in gohome. If you depend on a HACS integration for a device that has no first-party gohome driver, you cannot use gohome for that device yet.

**Supervisor add-ons do not transfer.** HA's Supervisor add-on ecosystem runs containers managed by the HA OS. gohome has no equivalent. If you use Supervisor add-ons (Zigbee2MQTT-as-add-on, for example), those run independently of gohome — which is actually fine, because gohome can talk to Zigbee2MQTT over MQTT or the Z2M bridge driver.

**The HA mobile app will not work unmodified.** The HA Companion app is built against the HA API. It will not connect to gohome. gohome's web UI is a PWA installable on mobile; a native wrapper is deferred.

**Smaller ecosystem at launch.** HA has been developed since 2013 with thousands of contributors. gohome has a fraction of that history and a fraction of the driver coverage. First-party drivers (v0.1.0 alpha ships: MQTT, Zigbee2MQTT bridge, Hue, ESPHome native, generic REST, generic webhook; v1.0 target adds: Matter, HomeKit bridge, Z-Wave JS bridge, Nest). Devices outside that set require a custom driver or waiting for the ecosystem to grow.
