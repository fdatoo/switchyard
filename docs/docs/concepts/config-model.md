# Config model

!!! status-alpha "Alpha — shipped, interface evolving"
    The Pkl and Starlark layers described here are shipped. The specific module APIs (`gohome:entities`, `gohome:automations`, etc.) may evolve before v1.0.

gohome uses two languages for configuration:

- **Pkl** for structure: driver instances, entity declarations, dashboards, automations shape, users, roles, policies.
- **Starlark** for logic: automation bodies, trigger conditions, computed entity expressions, scripts.

They solve different problems and are intentionally kept separate. Pkl is validated at load time and round-trips through git and AI editors cleanly. Starlark is a real programming language that executes safely in a sandbox.

## Pkl for structure

[Pkl](https://pkl-lang.org) is Apple's open-source configuration language. It looks like a typed, composable config file. It has classes, inheritance, generics, computed properties, and a module system.

### Why Pkl instead of YAML

YAML has no types. The following is valid YAML:

```yaml
brightness: "80"   # a string
brightness: 80     # an integer
```

Nothing in a YAML parser tells you which one is correct. Errors surface at runtime, not at load time.

Pkl has types:

```pkl
brightness: Int = 80   // validated at evaluation time
```

If you write `brightness: "80"`, the Pkl evaluator rejects the config before gohomed even starts.

### Why Pkl instead of YAML+Jinja

Home Assistant uses YAML with Jinja templates for dynamic values. The combination is awkward: Jinja is a templating language designed for HTML, not configuration. It has no types, no way to validate whether a template is syntactically correct without rendering it, and errors only appear at runtime when the template is evaluated.

Pkl holds Starlark expressions directly (see [The seam](#the-seam) below). Those expressions are syntax-validated at `gohome config validate` time — before the daemon loads the config — not at runtime.

### Typed, git-friendly, AI-editable

A gohome config tree is a directory of `.pkl` files. It can be committed to git like any other code. Diffs are readable. Changes can be reviewed in a pull request.

Because Pkl is typed and has a documented module schema, AI assistants can edit it reliably. Claude can read `gohome:entities`, understand the `Light` class, and generate a correct entity declaration. It can also run `gohome config validate` and interpret the structured error output to fix mistakes.

### An example config file

```pkl
import "gohome:base"      as base
import "gohome:carport"   as carport
import "gohome:entities"  as entities
import "gohome:automations" as auto

// Driver instance — binds the hue driver to a specific bridge
hueMain = new carport.HueDriverInstance {
  id         = "hue_main"
  driverName = "hue"
  bridgeHost = "10.0.0.42"
  apiToken   = "env:HUE_TOKEN"
}

// Entity declaration
kitchenLight = new entities.Light {
  id                 = "light.kitchen_ceiling"
  friendlyName       = "Kitchen ceiling"
  area               = "kitchen"
  supportsBrightness = true
  supportsColorTemp  = true
}

// Automation — structure in Pkl, logic in Starlark
lightsOff = new auto.Automation {
  id       = "auto.lights_off_at_midnight"
  triggers = new Listing<auto.Trigger> {
    new auto.TimeTrigger { at = "00:00" }
  }
  actions = new Listing<auto.Action> {
    new auto.StarlarkAction { body = """
      for e in entities(domain="light"):
        e.turn_off()
    """ }
  }
}
```

## Starlark for logic

[Starlark](https://github.com/bazelbuild/starlark) is a Python-dialect designed for sandboxed scripting. It is deterministic, has no I/O by default, and runs in a sandbox with explicit resource limits.

gohome uses `go.starlark.net`, the Google-maintained canonical Go implementation.

### Why Starlark instead of Jinja

Jinja is a templating language. Its conditionals, loops, and filters are designed for rendering text, not for expressing control flow in a home automation context. It has no proper scoping, no functions, no way to call a capability from within a template. Complex HA automations written in YAML+Jinja become unreadable quickly.

Starlark is a real programming language. It has functions, loops, conditionals, variables, lists, dicts, and closures. It is deterministic and sandboxed: no file I/O, no network access, no threads, no global state. An automation that calls `turn_off` on a list of lights is just a for-loop. A computed entity is just an expression. A script is just a function.

```starlark
# Starlark automation body — turn off all lights, then wait, then set a scene
for e in entities(domain="light"):
    e.turn_off()
sleep(5)
scene.apply("scene.night_mode")
```

### Execution contexts

Starlark runs in different **contexts** depending on where it appears in config. Each context has different available functions and resource limits:

| Context | Where used | Wall-clock | Available functions |
|---|---|---|---|
| `automation` | Automation action bodies | 30s | `state`, `call_service`, `sleep`, `notify`, `scene`, `event`, `now`, `log` |
| `computed` | Computed entity expressions | 100ms | `state` (read-only), `now`, `entities`, `avg` |
| `condition` | Trigger condition guards | 50ms | `state` (read-only), `event` (read-only), `now` |
| `script` | Named scripts | 30s | Same as `automation` + `params` |
| `mcp_eval` | AI agent evaluation | 30s | Configurable; read-only by default |

The `computed` context is read-only: it can read state but cannot call capabilities or fire events. This prevents infinite loops: a computed entity cannot trigger a state change that re-triggers itself.

In `automation` and `script` contexts, both the typed capability form (`e.turn_on()`) and the string-based `call_service(e.id, "turn_on")` are available. The typed form is preferred — it provides IDE completion and catches typos at config-validate time.

### Syntax validation at config load time

Starlark expressions inside Pkl config files are syntax-validated when you run `gohome config validate`. If a Starlark expression has a syntax error, the config is rejected with a structured error pointing to the file, line, and column:

```
$ gohome config validate
✗ Config invalid

  automations/lights.pkl:14:9
    StarlarkScript syntax error: unexpected token ','
    body = "for e in entities(domain='light',: ..."
```

This is the key improvement over Jinja: syntax errors are caught at load time, not when the automation fires at midnight.

## The seam

The practical question is: when does logic stay inline in Pkl, and when does it move to a `.star` file?

**Keep inline** when the logic is a one-liner or a simple expression:

```pkl
// Computed entity — simple inline expression
compute = "avg(s.state for s in entities(class='Temperature', area='interior'))"

// Trigger condition — single predicate
condition = "event.data.get('from_state') == 'off'"
```

**Move to a `.star` file** when the logic is multi-line, reusable, or testable:

```pkl
// Automation referencing an external script
new auto.Action {
  kind = "starlark"
  body = """
    load("//automations/lib/lighting.star", "good_night_sequence")
    good_night_sequence(delay=params.delay_secs)
  """
}
```

```starlark
# automations/lib/lighting.star
def good_night_sequence(delay):
    scene.apply("scene.evening")
    sleep(delay)
    for e in entities(domain="light", area="bedroom"):
        e.turn_off()
```

The `load("//...")` syntax resolves paths relative to the config directory root. Any path that escapes the config directory is rejected. Module results are cached in memory and invalidated when `gohome config apply` runs.

The rule of thumb: if you would write a test for it, move it to a `.star` file. If it fits on one line and you would never need to debug it, keep it inline.

## Secret handling

Secrets — API tokens, passwords, bridge credentials — are **never written in Pkl source**. They are referenced by URI:

| URI prefix | Source | Example |
|---|---|---|
| `env:` | Environment variable | `env:HUE_TOKEN` |
| `file:` | File on disk (e.g. Docker secret) | `file:/run/secrets/hue_token` |
| `keyring:` | OS keyring | `keyring:gohome/hue_token` |

In Pkl, declare a secret value using the prefix-string format:

```pkl
# Option A — environment variable
apiToken = "env:HUE_TOKEN"
```

```pkl
# Option B — file secret (e.g. Docker secret mount)
apiToken = "file:/run/secrets/hue"
```

```pkl
# Option C — system keyring (service/account)
apiToken = "keyring:gohome/hue"
```

Secrets are resolved at `Apply` time — after the config is validated but before it is passed to drivers. Resolved secret values are **never written to the event log**. The `ConfigApplied` event records that a config was applied and what changed (number of driver instances added/removed/changed), not the contents of the config itself.

If `gohome config validate` succeeds, secrets are not resolved — validation is side-effect-free. Only `gohome config apply` resolves secrets.

## Diff-based reload

When you run `gohome config apply`, gohomed computes the diff between the current running config and the new config. Only the things that actually changed are updated:

- **Unchanged driver instances** are not restarted. Their connections to hardware stay live.
- **Changed driver instances** (new config hash) are gracefully stopped, the new config is sent, and the instance resumes.
- **Removed driver instances** are shut down.
- **New driver instances** are started.
- **Automations** are recompiled and the engine reloads its ruleset.
- **Dashboards** are updated in the registry; connected web UI clients are notified.

This means applying a config change that only touches one driver instance does not disrupt any other driver. Your Zigbee network stays up while you update the Hue bridge token.

You can preview what will change before applying:

```
gohome config apply --dry-run
```

```
Dry-run — no changes applied

  Driver instances changed : 1  (hue_main: apiToken updated)
  Automations changed      : 0
  Dashboards changed       : 0
```

## Config validation workflow

```
gohome config validate          # evaluate Pkl, check cross-references, validate Starlark syntax
gohome config apply --dry-run   # same, plus show what would change
gohome config apply             # validate, resolve secrets, apply diff, record ConfigApplied event
```

The daemon also validates config on startup. If the config is invalid, the daemon exits with a non-zero status code and prints structured errors. It does not start with a partially-valid config.
