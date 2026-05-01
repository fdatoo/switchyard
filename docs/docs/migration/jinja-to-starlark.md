# Jinja to Starlark

!!! status-wip "In development"
    This feature is in active development. The `switchyard import-ha` command is not yet shipped.

Home Assistant uses Jinja2 templates extensively: in automation conditions, action templates, computed (template) entities, and notification messages. switchyard uses [Starlark](../automations/starlark.md) — a deterministic, sandboxed subset of Python — for the same purposes.

The importer includes a Jinja → Starlark transpiler that automatically converts the most common Jinja patterns. This page documents exactly what the transpiler handles, what it cannot handle, and how to fix the `# FIXME` markers it leaves behind.

---

## What the transpiler converts automatically

### State access

| Jinja | Starlark |
|---|---|
| `states('light.kitchen')` | `state('light.kitchen')` |
| `states('x').state` | `state('x').state` |
| `state_attr('x', 'brightness')` | `state('x').attributes["brightness"]` |
| `is_state('x', 'on')` | `state('x').state == 'on'` |
| `is_state_attr('x', 'attr', v)` | `state('x').attributes["attr"] == v` |
| `has_value('x')` | `state('x').state != None` |

### Type coercion (filters)

| Jinja | Starlark |
|---|---|
| `value \| float` | `float(value)` |
| `value \| int` | `int(value)` |
| `value \| bool` | `bool(value)` |
| `value \| string` | `str(value)` |
| `value \| default(v)` | `(value if value != None else v)` |

### Math filters

| Jinja | Starlark |
|---|---|
| `value \| min` | `min(value)` |
| `value \| max` | `max(value)` |
| `value \| sum` | `sum(value)` |
| `value \| round(n)` | `round(value, n)` |
| `value \| abs` | `abs(value)` |
| `value \| average` | `avg(value)` |

### String helpers

| Jinja | Starlark |
|---|---|
| `value \| length` | `len(value)` |
| `"fmt" \| format(a, b)` | `"fmt" % (a, b)` |

### Time helpers

| Jinja | Starlark |
|---|---|
| `now()` | `time.now()` |
| `utcnow()` | `time.utcnow()` |
| `as_datetime(v)` | `time.parse(v)` |
| `as_timestamp(v)` | `time.timestamp(v)` |
| `today_at('HH:MM')` | `time.today_at(HH, MM)` |

### Logical operators

Logical operators (`not`, `and`, `or`), comparisons (`==`, `!=`, `<`, `>`, `<=`, `>=`, `in`), and the ternary `iif` helper all translate directly:

| Jinja | Starlark |
|---|---|
| `iif(cond, t, f)` | `(t if cond else f)` |
| `x and y` | `x and y` |
| `not x` | `not x` |
| `x in list` | `x in list` |

### List operations

| Jinja | Starlark |
|---|---|
| `list \| selectattr('field', 'eq', v)` | `[x for x in list if x.field == v]` |
| `list \| rejectattr('field', 'eq', v)` | `[x for x in list if x.field != v]` |

Only the basic `eq` and `in` operator forms are supported. Complex `selectattr` predicates emit `# FIXME`.

### Control flow

| Jinja | Starlark |
|---|---|
| `{% if c %}...{% else %}...{% endif %}` | `if c: ...\nelse: ...` (statement) or `(... if c else ...)` (expression) |
| `{% elif c %}` | `elif c:` |
| `{% for x in y %}...{% endfor %}` | `for x in y: ...` |
| `{% set x = v %}` | `x = v` |

---

## What emits `# FIXME`

These Jinja constructs are outside the transpiler's supported set. When the transpiler encounters them, it emits a placeholder and adds a `# FIXME(jinja-import)` comment with the original Jinja preserved so you have full context for the manual fix.

### State access via property syntax

```jinja
{# Property-style access — not transpiled #}
{{ states.light.kitchen.state }}
{{ states['sensor.temperature'].attributes.unit_of_measurement }}
```

The transpiler handles `states('entity_id')` (function call form) but not `states.domain.name` (attribute traversal form). Replace with the function call equivalent:

```python
# After fixing:
state('light.kitchen').state
state('sensor.temperature').attributes["unit_of_measurement"]
```

### Area and device expansion helpers

```jinja
{{ expand(group.all_lights) }}
{{ area_entities('kitchen') }}
{{ device_entities('abc123') }}
{{ area_id('light.kitchen') }}
{{ area_name('abc123') }}
```

These require switchyard stdlib helpers that do not exist in v1.0. Use `entities(domain)` with an area filter as a starting point, or list the entities explicitly.

### Geographic helpers

```jinja
{{ closest('zone.home').name }}
{{ distance('zone.home', 'device_tracker.phone') }}
```

No equivalent in switchyard v1.0. These require a `switchyard.geo` stdlib that is deferred to a future release.

### Jinja macros

```jinja
{% macro brightness_pct(val) %}{{ (val / 255 * 100) | round }}{% endmacro %}
{{ brightness_pct(brightness) }}
```

Macros are transpiled as a `# FIXME`. Extract the logic into a helper function in a shared `.star` file instead.

### Custom Jinja filters and tests

Any filter or test not in the standard Jinja2 set, or not in HA's documented template helpers, emits `# FIXME`. This includes custom filters installed via HACS template extensions.

### Jinja includes and imports

```jinja
{% include 'templates/common.jinja' %}
{% import 'templates/macros.jinja' as m %}
```

Not supported. Extract the logic to a `.star` module and use Starlark's `load()` instead.

### HA blueprint inputs

```yaml
value_template: "{{ states(config.entity_id) }}"
```

Blueprint `!input` references embedded in templates are flagged with `# FIXME(blueprint-input)`. Blueprints are not a v1.0 switchyard concept.

---

## FIXME format

Every unfixable construct is emitted as a clearly-marked placeholder that produces valid Starlark (the file parses cleanly; only the semantics are missing):

```python
# FIXME(jinja-import): unmapped construct
#   Original Jinja: {{ closest('zone.home').name }}
#   At: automations/handlers/arrival_lights.star:12 (automation 'arrival_lights')
result = None  # placeholder — replace with equivalent Starlark
```

For statement contexts (e.g. inside a loop body):

```python
# FIXME(jinja-import): unmapped construct
#   Original Jinja: {% macro format_temp(v) %}...{% endmacro %}
pass  # FIXME — replace with Starlark equivalent
```

The placeholder is always syntactically valid. The file will load; the automation will run; the `None` or `pass` will produce a no-op or an error at runtime depending on how the value is used. Fix the FIXMEs before relying on those automations in production.

---

## Common patterns and their Starlark equivalents

The following are complete before/after examples for common HA templates.

| Jinja | Starlark |
|---|---|
| `{{ states('sensor.temp') \| float \| round(1) }}` | `round(float(state('sensor.temp').state), 1)` |
| `{{ is_state('light.kitchen', 'on') }}` | `state('light.kitchen').state == 'on'` |
| `{{ state_attr('climate.living', 'current_temperature') > 22 }}` | `state('climate.living').attributes["current_temperature"] > 22` |
| `{{ (now() - as_datetime(states.sensor.motion.last_changed)).seconds > 300 }}` | `# FIXME — property-style access on states.sensor.motion` |
| `{{ states('sensor.a') \| float + states('sensor.b') \| float }}` | `float(state('sensor.a').state) + float(state('sensor.b').state)` |
| `{% if is_state('binary_sensor.door', 'on') %}unlocked{% else %}locked{% endif %}` | `('unlocked' if state('binary_sensor.door').state == 'on' else 'locked')` |
| `{{ [1, 2, 3] \| max }}` | `max([1, 2, 3])` |
| `{{ as_timestamp(now()) \| int }}` | `int(time.timestamp(time.now()))` |
| `{{ expand(group.motion_sensors) \| selectattr('state', 'eq', 'on') \| list \| count > 0 }}` | `# FIXME — expand() not supported` |

---

## How to handle unmapped constructs

After running `switchyard import-ha`, search the output directory for all items that need attention:

```
$ grep -r 'FIXME(' ./my-switchyard/
```

Or read the summary in `IMPORT_REPORT.md` — the **Open FIXMEs** section lists every FIXME with its file, line, and the original Jinja that triggered it.

For each FIXME:

1. Read the original Jinja in the comment.
2. Understand what it computes.
3. Write the Starlark equivalent using the [Starlark guide](../automations/starlark.md) and the built-ins available in the relevant context.
4. Replace the `result = None` placeholder (or `pass`) with the new Starlark expression.
5. Run `switchyard config validate` to confirm the file is valid.
6. Test the automation with `switchyard automation trigger <id>` or `switchyard eval`.

Most FIXMEs arise from `states.entity_id` property-style access or from area expansion helpers. The fix is usually a small, mechanical rewrite.
