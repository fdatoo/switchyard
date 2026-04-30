# Scripts

!!! status-alpha "Alpha — shipped, interface evolving"

A script is a named, callable Starlark function declared in Pkl. Scripts are independently invokable — from automations, the CLI, and MCP tools — and share the same `KindScript` execution context as automations. They are the natural home for logic that is reused across multiple automations, takes parameters, or is complex enough to deserve its own tests.

---

## Declaring scripts

Scripts are declared in a `gohome.scripts` Pkl module:

```pkl
// scripts.pkl
import "gohome:scripts" as scripts

scripts: Listing<scripts.Script> = new {
  new scripts.Script {
    name = "notify_residents"
    params = new {
      new scripts.ScriptParam {
        name     = "message"
        type     = "string"
        required = true
      }
      new scripts.ScriptParam {
        name     = "priority"
        type     = "string"
        required = false
        default  = "normal"
      }
    }
    handler = """
      msg = params["message"]
      pri = params["priority"]
      log("Notifying residents: " + msg + " (priority=" + pri + ")")
      notify("user:alice", msg)
      notify("user:bob", msg)
    """
  }
}
```

Register scripts from `main.pkl`:

```pkl
scripts = import("scripts.pkl").scripts
```

---

## Typed parameters

Parameters are declared with `ScriptParam`:

```pkl
class ScriptParam {
  name:     String(!isEmpty)
  type:     String(oneOf("string","int","float","bool","entity_id"))
  required: Boolean = true
  default:  String?    // stringified; parsed against type at compile
}
```

| Type | Go equivalent | Notes |
|---|---|---|
| `string` | `string` | No coercion |
| `int` | `int64` | Must parse as integer |
| `float` | `float64` | Accepts integer literals too |
| `bool` | `bool` | `"true"` / `"false"` |
| `entity_id` | `string` | Validated as `<domain>.<name>` format |

Inside the Starlark handler, parameters are available as a `params` dict:

```python
params["message"]   # string
params["count"]     # int
params["threshold"] # float
params["active"]    # bool
params["entity"]    # string (entity id)
```

Required params missing at call time → compile-time error from `gohome config validate`; type coercion failures at runtime → `ErrScriptArgs`.

---

## Calling scripts from automations

Use `ScriptAction` to call a script from an automation:

```pkl
new automations.ScriptAction {
  name = "notify_residents"
  args = new { ["message"] = "Front door left open" ["priority"] = "high" }
}
```

The script runs under the automation's correlation ID so `gohome automation trace <id>` shows the full chain including the nested `ScriptInvoked` and `ScriptFinished` events.

---

## Calling scripts from the CLI

```
$ gohome script run notify_residents --arg message="Test alert" --arg priority=high
[log] Notifying residents: Test alert (priority=high)
→ None  (12ms / 847 steps)
```

Streaming output: `log()` calls appear as they execute. The final line shows the return value, elapsed time, and step count. Exit code 0 on success, 1 on error.

**List registered scripts:**

```
$ gohome script list
NAME                 PARAMS
notify_residents     message:string  [priority:string]="normal"
adaptive_brightness  entity:entity_id  [min_pct:int]=20
```

Required params are shown without brackets; optional params are `[bracketed]` with their default.

---

## Calling scripts from the web UI

Scripts registered in your config appear in the web UI's **Scripts** panel. You can invoke them with a form-based parameter editor and see log output streamed in real time. (Web UI is a later release; CLI is available now.)

---

## Calling scripts via MCP

MCP tools can invoke scripts using the `run_script` tool:

```json
{
  "tool": "run_script",
  "name": "notify_residents",
  "args": { "message": "Motion detected at 02:00", "priority": "high" }
}
```

The MCP server returns the script's return value and logs after execution.

---

## The `scripts/*.star` convention

For longer scripts, move the Starlark body to a `.star` file and reference it via `load()`:

```
~/.config/gohome/
├── scripts.pkl
└── scripts/
    ├── notify_residents.star
    └── adaptive_brightness.star
```

```pkl
// scripts.pkl
new scripts.Script {
  name = "adaptive_brightness"
  params = new {
    new scripts.ScriptParam { name = "entity" type = "entity_id" }
    new scripts.ScriptParam { name = "min_pct" type = "int" required = false default = "20" }
  }
  handler = """
    load("//scripts/adaptive_brightness.star", "run")
    run(params)
  """
}
```

```python
# scripts/adaptive_brightness.star

def run(params):
    entity = params["entity"]
    min_b  = params["min_pct"] * 255 // 100
    lux    = state("sensor.outdoor_lux").attributes["value"]
    target = max(min_b, 255 - int(lux * 0.8))
    call_service(entity, "set_brightness", brightness=target)
    log("Set " + entity + " brightness to " + str(target))
```

The `//` prefix resolves relative to your config directory. The module cache means the file is only read and compiled once per `config apply`, not on every call.

---

## Concurrency

Scripts have no admission gate — every `Call` runs immediately in its own goroutine, parallel to any other in-flight scripts or automations. There is no `mode` for scripts. If you need serialization, use an entity as a lock (check its state before proceeding) or let driver idempotency handle concurrent commands.

---

## Return values

A script's return value is the last expression evaluated in the Starlark body, or `None` if the script ends without a value. The return value is:

- Streamed to the CLI as `→ <value>` on completion.
- Recorded in the `ScriptFinished` event as `return_value` (Starlark `repr()`).
- Returned to MCP tool callers as a string.

```python
# This script returns the computed brightness
lux = state("sensor.outdoor_lux").attributes["value"]
255 - int(lux * 0.8)   # last expression = return value
```

---

## Script events

Every script invocation appends two events to the event store:

```
ScriptInvoked   script=notify_residents  corr=a3f2  by=cli:alice  args={message: "Test"}
ScriptFinished  script=notify_residents  corr=a3f2  outcome=OK    elapsed=12ms  steps=847
```

This makes script execution auditable and traceable through `gohome automation trace`.
