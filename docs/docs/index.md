---
hide:
  - navigation
  - toc
---

# switchyard

**switchyard** is a Go-native home automation daemon built for people who want their home to be as reliable as production infrastructure. Every state change is an immutable event stored in SQLite, every entity has a typed schema, and the entire system is designed to be queried and controlled by AI agents via MCP — no hacks, no YAML glue.

If you're coming from Home Assistant, you'll find the concepts familiar but the implementation dramatically simpler: typed entities instead of string states, Pkl configuration instead of sprawling YAML, Starlark instead of Jinja templates.

---

<div class="grid cards" markdown>

-   :material-download-circle:{ .lg .middle } **Install switchyard**

    ---

    Get the daemon running on Linux — static binary, Docker, or systemd package.

    [:octicons-arrow-right-24: Installation](installation/index.md)

-   :material-home-import-outline:{ .lg .middle } **Migrate from Home Assistant**

    ---

    Understand what transfers automatically and what needs rewriting before you cut over.

    [:octicons-arrow-right-24: Migration guide](migration/index.md)

-   :material-puzzle-edit:{ .lg .middle } **Build a driver**

    ---

    Use the Carport protocol and Go SDK to connect any device or service.

    [:octicons-arrow-right-24: Driver development](drivers/building/index.md)

</div>

---

## Why switchyard?

<div class="grid" markdown>

<div markdown>

**Event sourcing**

Every state change — light on, door opened, temperature update — is appended as an immutable event. Replay history, audit anything, time-travel debug automations.

</div>

<div markdown>

**Pkl configuration**

No more sprawling YAML. Pkl gives you a typed, composable config language with first-class IDE support. Bad configs are caught before they ship.

</div>

<div markdown>

**Starlark automations**

Automations are deterministic Python-subset scripts, not ad-hoc trigger/condition/action blobs. Full testability, no runtime surprises.

</div>

<div markdown>

**MCP-native**

The daemon exposes a Model Context Protocol server out of the box. Ask Claude to turn off the lights, query energy usage, or set a scene — no prompt engineering required.

</div>

<div markdown>

**Typed entities**

Entities have schemas. A `light` has `brightness: Int` and `color_temp: Kelvin` — not `state: "on"` with opaque attributes. Your automations know what they're working with.

</div>

<div markdown>

**Single binary**

`switchyardd` is a single statically linked binary with no runtime dependencies. Drop it on a Raspberry Pi, point it at a config directory, and it runs.

</div>

</div>
