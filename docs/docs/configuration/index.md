# Config directory

All of switchyard's user-editable configuration lives under a single directory — by default `~/.config/switchyard/`. Everything in that directory is Pkl source. The daemon evaluates the entire tree on startup and on `switchyard config apply`, producing a typed `ConfigSnapshot` that feeds every subsystem.

## Directory layout

```
~/.config/switchyard/
├── main.pkl           # root import — switchyard reads this file first
├── drivers.pkl        # driver instances
├── areas.pkl          # area hierarchy (rooms, floors, home)
├── zones.pkl          # geographic zones for presence detection
├── entities/
│   └── overrides.pkl  # entity overrides (rename, re-room, disable)
├── automations/
│   ├── lights.pkl     # automation definitions
│   └── *.star         # Starlark logic files referenced by automations
├── scenes.pkl
├── dashboards.pkl
├── auth.pkl
└── secrets/           # NOT Pkl source — runtime secret refs live in driver config
```

The `secrets/` directory is a convention for file-backed secrets (e.g. tokens written by a secrets manager). It is not evaluated by Pkl. Secret _references_ are constrained strings with prefix conventions: `"env:VAR_NAME"` (typed as `base.Secret`), `"file:/path"`, or `"keyring:service/account"`. They are never literal credentials. Resolution happens in Go at apply time, never in the Pkl evaluator.

## `main.pkl` — the root import

Every other file is reachable from `main.pkl`. A minimal working example:

```pkl
import "switchyard:base"        as base
import "switchyard:entities"    as entities
import "switchyard:automations" as automations
import "switchyard:dashboards"  as dashboards
import "switchyard:auth"        as auth

amends "switchyard:config"

drivers     = import("drivers.pkl").drivers
areas       = import("areas.pkl").areas
zones       = import("zones.pkl").zones
entities    = import("entities/overrides.pkl").overrides
automations = import("automations/lights.pkl").automations
scenes      = import("scenes.pkl").scenes
dashboards  = import("dashboards.pkl").dashboards
users       = import("auth.pkl").users
roles       = import("auth.pkl").roles
policies    = import("auth.pkl").policies
```

You are free to split files however you like — switchyard only cares about `main.pkl` as the entry point. Large installs often have one `.pkl` file per area, or separate files for lights, climate, and security.

## Validating config

Run `switchyard config validate` to evaluate and cross-reference-check your config without touching any running driver instances. This is safe to run at any time — it has no side-effects.

```
$ switchyard config validate

✓ Config valid
  Driver instances : 2
  Entities         : 14
  Automations      : 3
  Dashboards       : 1
```

On failure each error is printed with the file path and line number:

```
✗ Config invalid

  drivers.pkl:12  driverName "zigbee2mqtt_v2" is not installed
  entities/overrides.pkl:8  entity id "light.kitchen" not found in registry
```

Exit code 1 on any validation error. This makes `switchyard config validate` safe to run in CI.

## Applying config

```
$ switchyard config apply
```

Validate → resolve secrets → diff against the currently-running snapshot → apply the diff → append a `ConfigApplied` event to the event store.

```
Driver instances   +2  -0  ~1
Automations        +0  -0  ~3
Dashboards         +1  -0  ~0
```

### Dry run

```
$ switchyard config apply --dry-run
```

Produces the diff table and exits. No secrets are resolved, no driver instances are restarted, no events are appended.

## Diff-based reload

switchyard compares the new `ConfigSnapshot` against the currently-running one using a content hash per driver instance. Only driver instances whose config hash changed are restarted. A driver instance that appears identically in the old and new snapshot is not touched.

This means:

- Editing `scenes.pkl` or `dashboards.pkl` does not restart any driver.
- Adding a new driver instance starts only that instance.
- Changing one driver instance's credentials restarts only that instance, not the others.

The diff metadata is recorded in the `ConfigApplied` event:

```proto
message ConfigApplied {
  int32 driver_instances_added   = 2;
  int32 driver_instances_removed = 3;
  int32 driver_instances_changed = 4;
  int32 automations_changed      = 5;
  bool  dry_run                  = 6;
}
```

## LSP support

The `switchyard:*` Pkl modules are embedded in the daemon binary and also shipped as a `PklProject.pkl` in the source repo. Point your editor's `pkl.projectDir` setting at the `pkl/` directory in the switchyard source tree. The Pkl LSP then resolves `switchyard:entities`, `switchyard:auth`, etc. and provides autocomplete and type checking while you edit your config files.
