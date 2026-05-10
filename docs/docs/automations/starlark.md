# Starlark guide

!!! status-alpha "Alpha — shipped, interface evolving"

Starlark is switchyard's embedded scripting language. It is a deterministic, sandboxed subset of Python with no I/O except through the built-ins provided by each execution context. If you can write Python, you can write Starlark — the syntax is nearly identical.

---

## Language overview

Starlark is Python-like but deliberately simpler:

- **Deterministic.** No implicit global state, no mutable closures across calls.
- **No I/O.** No `open()`, no `import`, no `os`, no network. All external access goes through context-provided built-ins.
- **No `while True:`.** Iteration must terminate; unbounded loops are caught by the step counter.
- **No classes.** Structs and dicts instead.
- **No exceptions.** Errors propagate as Starlark errors; handle them with `fail()` if needed.

**What carries over from Python:**

```python
# Basic types
x = 42
name = "hall"
active = True
nums = [1, 2, 3]
d = {"key": "value"}

# Control flow
if x > 10:
    log("big")
elif x > 5:
    log("medium")
else:
    log("small")

for n in nums:
    log(str(n))

# Functions
def clamp(val, lo, hi):
    return max(lo, min(hi, val))

# String formatting (f-strings not available; use + or %)
msg = "brightness=" + str(brightness)
msg = "entity=%s brightness=%d" % (entity, brightness)

# List comprehensions
lights_on = [e for e in lights if state(e).state == "on"]
```

For the full language reference, see the [Starlark specification](https://github.com/bazelbuild/starlark/blob/master/spec.md).

---

## Per-context built-ins

Every Starlark snippet runs inside an **execution context**. The context determines which built-ins are available and what resource limits apply.

### `automation` context

Used for `StarlarkAction` bodies and `StarlarkScript` handlers invoked from an automation.

| Built-in | Signature | Description |
|---|---|---|
| `state` | `state(entity_id) → EntityState` | Read current entity state |
| `call_service` | `call_service(entity_id, capability, **kwargs)` | Dispatch a command to a driver |
| `sleep` | `sleep(seconds)` | Pause execution (ctx-cancellable) |
| `now` | `now() → Time` | Current UTC time |
| `log` | `log(msg, level="info")` | Emit a log line (captured in run record) |
| `notify` | `notify(target, message)` | Send a notification |
| `scene` | `scene.apply(slug)` | Record a scene application stub |
| `event` | `event.fire(kind, data)` | Fire a custom event; `.kind`, `.entity_id`, `.data` from trigger |
| `random` | `random() → float` | Random float in [0, 1) |
| `time` | module | `go.starlark.net/lib/time` — `time.now()`, durations, parsing |

**Example:**

```python
lux = state("sensor.outdoor_lux").attributes["value"]
if lux < 100:
    brightness = max(30, min(255, 255 - int(lux * 1.5)))
    call_service("light.hall", "turn_on", brightness=brightness)
    log("Hall light on at brightness=%d (lux=%d)" % (brightness, lux))
else:
    log("Enough daylight, skipping")
```

### `script` context

Identical to `automation` context, plus:

| Built-in | Signature | Description |
|---|---|---|
| `params` | dict | Typed parameters declared in `ScriptParam` |

```python
# In a Script handler
entity = params["entity"]
min_b  = params["min_pct"] * 255 // 100
call_service(entity, "turn_on", brightness=min_b)
```

### `condition` context

Used for `StarlarkCondition.expr`. Read-only — no side-effects.

| Built-in | Signature | Description |
|---|---|---|
| `state` | `state(entity_id) → EntityState` | Read current entity state (read-only) |
| `event` | read-only struct | `.kind`, `.entity_id`, `.data` from the triggering event; `None` for time/manual triggers |
| `now` | `now() → Time` | Current UTC time |

Must evaluate to a truthy or falsy value. Example:

```python
state("binary_sensor.hall_motion").state == "on"
and state("sensor.outdoor_lux").attributes["value"] < 150
```

### `computed` entity context

Used for computed entity handlers. Reactive — re-evaluated when dependencies change.

| Built-in | Signature | Description |
|---|---|---|
| `state` | `state(entity_id) → EntityState` | Read current entity state (read-only) |
| `entities` | `entities(domain) → list[EntityState]` | All entities in a domain |
| `avg` | `avg(values) → float` | Average of a list of numbers |
| `sum` | `sum(iterable) → number` | Sum of a list of numbers (standard Starlark built-in) |
| `now` | `now() → Time` | Current UTC time |

```python
# Average temperature across interior rooms
temps = [state(e.id).attributes["value"]
         for e in entities("sensor")
         if e.area in ("kitchen", "living_room", "bedroom")]
avg(temps)
```

### `widget_compute` context

Used to compute display values for dashboard widgets.

| Built-in | Signature | Description |
|---|---|---|
| `state` | `state(entity_id) → EntityState` | Read current entity state (read-only, cached snapshot) |
| `now` | `now() → Time` | Current UTC time |
| `entities` | `entities(domain) → list[EntityState]` | All entities in a domain |

### `mcp_eval` context

Used for MCP `eval_starlark` tool calls. Read-only by default; can be promoted to write-capable based on MCP auth policy.

| Built-in (read-only default) | Description |
|---|---|
| `state(entity_id)` | Read entity state |
| `now()` | Current time |
| `entities(domain)` | List entities |
| `log(msg)` | Emit a log line |

When the MCP policy grants write access:

| Additional built-in | Description |
|---|---|
| `call_service(entity_id, capability, **kwargs)` | Dispatch a command |
| `scene.apply(slug)` | Record a scene application stub |
| `notify(target, message)` | Send a notification |

---

## Resource limits

| Context | Wall-clock | Max steps | Network | File I/O |
|---|---|---|---|---|
| `automation` | 30s | 10 000 000 | No | No |
| `script` | 30s | 10 000 000 | No | No |
| `condition` | 50ms | 100 000 | No | No |
| `computed` entity | 100ms | 500 000 | No | No |
| `widget_compute` | 50ms | 100 000 | No | No |
| `mcp_eval` | 30s | 10 000 000 | No | No |

Breaching a limit produces a `LimitError`:

- In `automation` and `script` contexts: the run ends with `OUTCOME_LIMIT_EXCEEDED`.
- In `condition` context: treated as `false` with a warning log.
- In `computed` context: the previous computed value is retained.

The step counter increments once per Starlark evaluation step — function calls, loop iterations, comparisons. A tight loop over 10M items hits the limit; practical automation logic stays well below it.

---

## User-defined `load()` for shared `.star` files

Split reusable logic into `.star` modules under your config directory and `load()` them:

```python
load("//lib/helpers.star", "clamp", "brightness_for_lux")

lux = state("sensor.outdoor_lux").attributes["value"]
call_service("light.hall", "turn_on", brightness=brightness_for_lux(lux))
```

**Resolution rules:**

- Paths must start with `//` — resolved relative to your config directory (`~/.config/switchyard/`).
- Path traversal (`..`) is rejected.
- Circular loads are detected and reported as errors.
- Modules are cached per `config apply`; editing a `.star` file takes effect after the next `switchyard config apply`.
- Non-`//` schemes (`http:`, bare relative paths, `file:`) are rejected.

**Defining a shared module:**

```python
# lib/helpers.star

def clamp(val, lo, hi):
    """Clamp val to [lo, hi]."""
    return max(lo, min(hi, val))

def brightness_for_lux(lux):
    """Compute brightness (0-255) inversely proportional to ambient lux."""
    return clamp(255 - int(lux * 0.8), 20, 255)
```

Only names explicitly listed in the `load()` call are imported:

```python
load("//lib/helpers.star", "clamp")   # only clamp is available
```

---

## `switchyard eval` — scratch tool

Run a Starlark snippet against the live daemon without writing a file:

```
$ switchyard eval 'state("light.kitchen").state'
→ "off"  (3ms / 12 steps)
```

Pass a file:

```
$ switchyard eval my_script.star
[log] brightness=180
→ 180  (8ms / 423 steps)
```

**Flags:**

| Flag | Default | Options |
|---|---|---|
| `--context` | `automation` | `automation`, `computed`, `condition`, `script`, `mcp` |

**Output format:**

- `[log]` prefix: lines from `log()` calls.
- `→ value`: the final expression value (omitted for `None`).
- `elapsed` + `steps` on the last line.
- Errors go to stderr; exit code 1.

Use `switchyard eval` to prototype snippets before putting them in an automation, or to query live state interactively.

---

## Debugging tips

**Print state to console:**

```python
s = state("light.kitchen")
log("state=" + s.state + " attrs=" + str(s.attributes))
```

**Check what you're working with:**

```python
ents = entities("sensor")
for e in ents:
    log(e.id + " = " + str(state(e.id).attributes))
```

**Test conditions without running actions:**

```
$ switchyard eval --context condition \
  'state("binary_sensor.hall_motion").state == "on"'
→ False  (2ms / 8 steps)
```

**Write unit tests with `switchyard test`:**

```python
# automations/test_lights.star
load("//lib/helpers.star", "clamp")

def test_brightness_clamp():
    assert(clamp(300, 0, 255) == 255, "upper clamp failed")
    assert(clamp(-10, 0, 255) == 0,   "lower clamp failed")
    assert(clamp(128, 0, 255) == 128, "passthrough failed")
```

```
$ switchyard test automations/test_lights.star
--- PASS: test_brightness_clamp  (4ms / 63 steps)
```

**Trace a full run:**

```
$ switchyard automation trigger hall_motion_night
▶ triggered hall_motion_night (manual) corr=a3f2

$ switchyard automation trace a3f2
automation.run          hall_motion_night  corr=a3f2
 ├─ conditions          PASS (2 checked)
 ├─ action[0]  call_service  light.hall.turn_on    ✓  2ms
 ├─ action[1]  wait          5min                  ✓  300s
 └─ action[2]  call_service  light.hall.turn_off   ✓  3ms
AutomationFinished  OK  300.005s
```

**Common pitfalls:**

- `event` is `None` when an automation is triggered manually or by a time trigger. Guard: `if event != None:`.
- `state(entity_id).attributes` is a dict with string keys; numeric values arrive as `int` or `float` depending on the driver.
- `sleep()` in a condition context is not available — conditions have a 50ms wall-clock limit and run synchronously.
- `load()` paths must start with `//`; bare `"helpers.star"` is rejected.
