# gohome Documentation Site — Design Document

**Status:** Approved
**Date:** 2026-04-27
**Scope:** Comprehensive documentation for the gohome project, published via the `gohome-docs` Zensical site.

---

## 1. Goals

Produce a complete, live documentation site for gohome covering all three audiences:

1. **Homelab operators** — install, configure, and run gohome.
2. **Driver developers** — build Carport-speaking drivers using the Go SDK.
3. **Contributors** — understand internal architecture and contribute to the project.

All audiences are served from a single topic-based navigation tree. No audience-gated sections. Reference and guide content coexist under clear headings.

---

## 2. Site Configuration

Update `gohome-docs/zensical.toml`:

- `site_name = "gohome"`
- `site_description = "Go-native home automation. Event-sourced, typed, agent-friendly."`
- `site_author = "Fynn Datoo"`
- Explicit `nav` block defining full page order (prevents filesystem-derived ordering).
- Custom CSS for three status badge admonition variants: `status-alpha`, `status-planned`, `status-wip`.
- Enable `content.code.copy`, `navigation.sections`, `navigation.instant`, `navigation.footer`, `navigation.path`, `search.highlight`.

**Status badge convention:** Any page or section covering an unshipped feature opens with a standard admonition:

```
!!! status-planned "Planned — not yet implemented"
    This feature is designed but not yet shipped.
```

Variants:
- `status-alpha` — shipped, API unstable
- `status-planned` — designed, not started
- `status-wip` — in active development

---

## 3. File Structure

Full directory layout under `docs/`:

```
docs/
├── index.md
├── introduction/
│   ├── index.md
│   ├── architecture.md
│   ├── vs-home-assistant.md
│   └── changelog.md
├── installation/
│   ├── index.md
│   ├── binary.md
│   ├── docker.md
│   ├── systemd.md
│   └── first-run.md
├── concepts/
│   ├── index.md
│   ├── domain-model.md
│   ├── event-sourcing.md
│   └── config-model.md
├── configuration/
│   ├── index.md
│   ├── entities.md
│   ├── areas-zones.md
│   ├── drivers.md
│   ├── scenes.md
│   ├── dashboards.md
│   ├── auth.md
│   └── secrets.md
├── automations/
│   ├── index.md
│   ├── triggers.md
│   ├── conditions.md
│   ├── actions.md
│   ├── scripts.md
│   ├── starlark.md
│   └── computed-entities.md
├── drivers/
│   ├── index.md
│   ├── first-party.md
│   └── building/
│       ├── index.md
│       ├── manifest.md
│       ├── go-sdk.md
│       ├── lifecycle.md
│       └── testing.md
├── ai-agents/
│   ├── index.md
│   ├── tool-catalog.md
│   ├── resources.md
│   └── workflows.md
├── api-reference/
│   ├── pkl-modules.md
│   ├── cli.md
│   ├── connect-rpc.md
│   └── event-types.md
├── operations/
│   ├── index.md
│   ├── deployment.md
│   ├── backup-restore.md
│   ├── updates.md
│   └── observability.md
├── migration/
│   ├── index.md
│   ├── what-transfers.md
│   ├── jinja-to-starlark.md
│   └── post-migration.md
├── edge-agents/
│   ├── index.md
│   ├── pairing.md
│   └── resilience.md
└── contributing/
    ├── index.md
    ├── dev-setup.md
    └── architecture-internals.md
```

~52 pages total.

---

## 4. Page-by-Page Content Design

### 4.1 Landing page (`index.md`)

- Three-sentence pitch: what gohome is, who it's for, what distinguishes it.
- Three entry-point cards: "Install gohome", "Migrate from Home Assistant", "Build a driver".
- Feature grid: event sourcing, Pkl config, Starlark automations, MCP-native, typed entities.
- No wall of text. Minimal prose on the landing page itself.

### 4.2 Introduction

**`index.md`** — What is gohome. The three architectural bets (event sourcing, Pkl+Starlark, agent-native). Target audience (prosumer / homelab). Explicit non-goals. ~500 words.

**`architecture.md`** — The Mermaid component diagram from the master design. Binaries table (`gohomed`, `gohome`, `gohome-edge`). Internal module inventory. Public contracts (the five hardest-to-change surfaces: Carport, event schema, Connect-RPC API, MCP tool surface, Pkl module schemas).

**`vs-home-assistant.md`** — Dedicated HA comparison page covering:
- HA warts and gohome fixes (full table: integration→driver+instance, string state→typed state, untyped attributes→typed attributes, services→capabilities, Jinja→Starlark, template entities→computed entities, floppy areas→first-class geometry).
- What gohome deliberately keeps from HA (entity domains, `domain.name` id format, areas, scenes, scripts, automation trigger/condition/action shape, persons, zones as geofences).
- What you give up (no HA API compatibility shim, HACS integrations don't transfer, supervisor add-ons don't transfer, HA mobile app won't work unmodified).
- Tone: matter-of-fact, not disparaging of HA.

**`changelog.md`** — Version history stub.

### 4.3 Installation

**`index.md`** — Prerequisites (Linux/macOS/Windows support matrix), deciding which install path to use. Links to sub-pages.

**`binary.md`** — Download static binary, verify sigstore signature, place in `$PATH`. Covers all supported platforms.

**`docker.md`** — OCI image, Docker Compose example with volume mounts for config dir and data dir. Environment variables. Port mapping.

**`systemd.md`** — `.deb`/`.rpm` install via apt/rpm. Homebrew formula for macOS. systemd unit template: install, enable, start, view logs via `journalctl`.

**`first-run.md`** — Shared "now what" page for all install paths: create config directory, copy `minimal-main.pkl`, run `gohome config validate`, start daemon, confirm with `gohome status`.

### 4.4 Concepts

**`index.md`** — Brief map of the three concept pages and why they matter.

**`domain-model.md`** — Every noun defined with a Pkl example: driver, driver instance, device, entity (typed state, typed attributes, capabilities), entity class, computed entity, area, zone, automation, script, scene, dashboard, widget, user/role/policy. The HA wart comparison table repeated here for quick reference. Identity conventions (`domain.name`, ULID slugs).

**`event-sourcing.md`** — Written for operators, not engineers. "Every state change is an event" in plain terms. What this gives you: time-travel debugging (`gohome events replay --at <timestamp>`), free audit log, answering "what happened at 2am?". What it costs: SQLite grows over time (~40 GB/year at typical prosumer scale). One worked example.

**`config-model.md`** — Pkl for structure (typed, validated, git-friendly, AI-editable). Starlark for logic (sandboxed, real language, not Jinja). The seam: where Pkl holds Starlark one-liners inline vs. where logic migrates to `.star` files. Secret sources: never in Pkl source. Why this beats YAML+Jinja.

### 4.5 Configuration

**`index.md`** — Full config directory layout diagram. `main.pkl` as root import. `gohome config validate` / `apply` / `apply --dry-run`. Diff-based reload explained: unchanged driver instances are not restarted.

**`entities.md`** — Standard entity classes (`gohome.entities.*`). Entity id format. Typed state and attributes. Overrides (`entities/overrides.pkl`). Custom entity classes.

**`areas-zones.md`** — Hierarchical areas. Zones as geofences (lat/lon + radius). Assigning entities to areas.

**`drivers.md`** — Declaring driver instances in `drivers.pkl`. Typed config per driver class. Multiple instances of the same driver. Secret references in driver config.

**`scenes.md`** — Declaring scenes in `scenes.pkl`. `gohome scene apply`. The `SceneApplied` event and what it records.

**`dashboards.md`** — Dashboard declaration in Pkl. Pages, grids, widget instances. The WYSIWYG round-trip (edit → Pkl write-back). Widget packs.

**`auth.md`** — Users (Pkl-declared, auth methods). Roles (built-in: admin/member/guest; custom). Policies (the kids-policy example from the master design). Enforcement points. Auth events on the log.

**`secrets.md`** — All secret source types: `env:`, `file:`, `keyring:`. How secrets resolve at runtime. Community modules (`vault:`, `1password:`, `bitwarden:`). What never to do (commit secrets in Pkl source).

### 4.6 Automations & Scripts

**`index.md`** — Full trigger→conditions→actions shape in Pkl. The Pkl+Starlark seam for automations. How automations are compiled and registered.

**`triggers.md`** — State change triggers, time triggers, event triggers. Typed Pkl wrappers. Examples.

**`conditions.md`** — Typed conditions (state comparisons, time windows). Starlark conditions. `and`/`or` composition.

**`actions.md`** — Call capability, run script, apply scene, wait, block. Typed wrappers. Starlark action blocks.

**`scripts.md`** — Declaring named scripts. Typed parameters. Calling scripts from automations, UI, CLI, MCP. The `scripts/*.star` convention.

**`starlark.md`** — Full language guide. Per-context stdlib tables (automation, computed entity, trigger condition, script, widget compute, MCP eval). Resource limits per context. User-defined `load()` for shared `.star` files. `gohome eval` scratch tool. Debugging tips.

**`computed-entities.md`** — First-class computed entities. `ComputedEntity` Pkl class. Reactive re-evaluation. Examples: average temperature, presence-derived state.

### 4.7 Drivers

**`index.md`** — What drivers are. Install/remove workflow. `gohome driver upgrade <name>`. Driver health via `gohome driver status`. How drivers are versioned independently.

**`first-party.md`** — Catalog of all v1.0 first-party drivers. One entry per driver: what it does, config fields table, known caveats, link to driver repo. Drivers: MQTT, Zigbee2MQTT, Matter, HomeKit bridge, ESPHome native, Z-Wave JS, generic REST, generic webhook, Nest, Hue.

**`building/index.md`** — Overview: what a driver is (a binary that speaks Carport), what the Go SDK provides, when to use local subprocess vs. edge transport.

**`building/manifest.md`** — Writing the Pkl manifest. `DriverManifest` fields. `instanceConfig` typed class. `produces` entity list. `driverEventTypes`. Embedding the manifest in the binary.

**`building/go-sdk.md`** — Go library walkthrough. Entity class registration. Emitting `StateChanged` events. Handling `Command` messages. Sending `CommandResult`. Typed `DriverEvent` payloads.

**`building/lifecycle.md`** — Full lifecycle state machine: launch, handshake, register instances, run, health probe, graceful shutdown, crash+restart with exponential backoff. What "stateless from gohomed's perspective" means in practice.

**`building/testing.md`** — Driver test harness. Using `fakedriver` as a reference. Integration test patterns. Running `gohome test` against a driver.

### 4.8 AI Agents & MCP

**`index.md`** — MCP setup for Claude Code, Claude Desktop, Cursor. Expanded from `gohome/docs/mcp-setup.md`. Security model (local-only Unix socket in C8; token-based auth in C9).

**`tool-catalog.md`** — One entry per tool: name, verb (READ/CALL/ADMIN), description, full input schema, output shape, example invocation. All 12 tools.

**`resources.md`** — Three MCP resource subscriptions: `gohome://entities/`, `gohome://entities/{id}`, `gohome://automations/{id}/runs/{run_id}/trace`. How to subscribe. What updates look like.

**`workflows.md`** — Three worked end-to-end examples:
1. Ask Claude to create an automation (the garage lights example from the master design).
2. Ask Claude to debug why a light didn't turn on (querying events, reading state, tracing a command).
3. Ask Claude to add a new driver instance (reading config, writing config, validating, applying).

### 4.9 API Reference

**`pkl-modules.md`** — All `gohome.*` Pkl modules with type signatures, fields, constraints, and examples: `base`, `carport`, `entities`, `automations`, `dashboards`, `widgets`, `auth`, `starlark`.

**`cli.md`** — Full `gohome` and `gohomed` command reference. Every subcommand, flags, exit codes, environment variables. Organized as a reference table then per-command detail.

**`connect-rpc.md`** — All 13 services with each RPC's request/response types, streaming behavior, pagination, and error taxonomy. Versioning policy (`v1alpha1` → `v1` promotion criteria).

**`event-types.md`** — Every event payload type in the protobuf `Payload.kind` oneof. Fields, when it's emitted, what it means. Useful for automation trigger writers and MCP users.

### 4.10 Operations

**`index.md`** — Overview of operational concerns.

**`deployment.md`** — Default ports, data directory layout (SQLite DB, config dir, driver binaries, lock file), environment variables, the lock file behavior.

**`backup-restore.md`** — Total persistent state: config dir (Pkl) + SQLite DB + driver binaries. `gohome backup` (SQLite online backup API, consistent, no downtime, optional encryption). `gohome restore`. The one-liner for moving to a new server.

**`updates.md`** — All update paths: OCI tag bump, apt/brew, `gohome self-update` (download, verify sigstore, atomic replace, systemd restart). Schema migration behavior (golang-migrate at startup, pre-migration DB copy). Event schema backward compatibility. Driver update lifecycle. Pkl module version pinning.

**`observability.md`** — Structured logs (stdlib `slog`, JSON, configurable level). Prometheus metrics endpoint (`/metrics`). OpenTelemetry tracing with OTLP export. The `gohome diag` bundle (redacted support bundle: versions, driver versions, recent errors, health snapshots).

### 4.11 Migration from Home Assistant

**`index.md`** — Overview of `gohome import-ha`. Prerequisites. What the importer does and doesn't do. Point to `vs-home-assistant.md` first. The command: `gohome import-ha ~/.homeassistant -o ./my-gohome`.

**`what-transfers.md`** — The full HA construct → gohome target mapping table with confidence levels (High/Medium/Low) and notes. Covers: configuration.yaml, areas, zones, device/entity registries, integrations, automations, scripts, scenes, template sensors, Lovelace dashboards, users/persons, secrets. Integration coverage: MQTT, Z2M, ESPHome, HomeKit, Matter, Hue, Nest, Z-Wave JS, generic REST/webhook, template platform.

**`jinja-to-starlark.md`** — Transpiler rules. What converts automatically (`states('x')` → `state('x')`, arithmetic, control flow, common filters). What emits `# FIXME`. Common patterns and their Starlark equivalents. How to handle unmapped constructs.

**`post-migration.md`** — Checklist: validate config, re-register passkeys (passwords not migrated), verify each driver instance, check computed entities, review automations with `# FIXME` markers, verify scenes and dashboards, test a few automations.

### 4.12 Edge Agents

**`index.md`** — What an edge agent is: `gohome-edge` runs on a remote host (e.g. Raspberry Pi), hosts drivers, forwards Carport over TLS back to the primary daemon. When to use it: remote Z-Wave radio, garage Pi, basement switch. Status: `status-wip`.

**`pairing.md`** — The mTLS pairing flow: gohomed issues a CA certificate at pairing time; edge agent presents its certificate on connection; CA is locked to that pair. Step-by-step pairing command sequence.

**`resilience.md`** — Local event buffering when the edge agent loses connection to the primary. Reconnection behavior. Multi-edge scenarios (multiple edge agents on one primary).

### 4.13 Contributing

**`index.md`** — Contribution overview. Code of conduct. Types of contributions welcome (drivers, Pkl modules, bug fixes, docs). PR process.

**`dev-setup.md`** — Prerequisites: Go 1.22+, Pkl CLI, buf, task (Taskfile). Clone and build. Running tests (`task test`). Pre-commit hooks. Running the daemon locally with a test config.

**`architecture-internals.md`** — Deep reference for contributors. Module boundaries and why they exist. The single-writer eventstore discipline. Concurrency invariants (three named invariants). The carport FSM. Config diff-based reload internals. Pointers to the child design specs (C1–C13) in the `docs` submodule for each subsystem.

---

## 5. Navigation Structure (zensical.toml `nav` block)

```toml
nav = [
  { "Home" = "index.md" },
  { "Introduction" = [
    { "What is gohome" = "introduction/index.md" },
    { "Architecture" = "introduction/architecture.md" },
    { "vs. Home Assistant" = "introduction/vs-home-assistant.md" },
    { "Changelog" = "introduction/changelog.md" },
  ]},
  { "Installation" = [
    { "Overview" = "installation/index.md" },
    { "Static binary" = "installation/binary.md" },
    { "Docker" = "installation/docker.md" },
    { "systemd / packages" = "installation/systemd.md" },
    { "First run" = "installation/first-run.md" },
  ]},
  { "Concepts" = [
    { "Overview" = "concepts/index.md" },
    { "Domain model" = "concepts/domain-model.md" },
    { "Event sourcing" = "concepts/event-sourcing.md" },
    { "Config model" = "concepts/config-model.md" },
  ]},
  { "Configuration" = [
    { "Config directory" = "configuration/index.md" },
    { "Entities" = "configuration/entities.md" },
    { "Areas & Zones" = "configuration/areas-zones.md" },
    { "Drivers" = "configuration/drivers.md" },
    { "Scenes" = "configuration/scenes.md" },
    { "Dashboards" = "configuration/dashboards.md" },
    { "Auth & Policies" = "configuration/auth.md" },
    { "Secrets" = "configuration/secrets.md" },
  ]},
  { "Automations & Scripts" = [
    { "Overview" = "automations/index.md" },
    { "Triggers" = "automations/triggers.md" },
    { "Conditions" = "automations/conditions.md" },
    { "Actions" = "automations/actions.md" },
    { "Scripts" = "automations/scripts.md" },
    { "Starlark guide" = "automations/starlark.md" },
    { "Computed entities" = "automations/computed-entities.md" },
  ]},
  { "Drivers" = [
    { "Using drivers" = "drivers/index.md" },
    { "First-party drivers" = "drivers/first-party.md" },
    { "Building drivers" = [
      { "Overview" = "drivers/building/index.md" },
      { "Driver manifest" = "drivers/building/manifest.md" },
      { "Go SDK" = "drivers/building/go-sdk.md" },
      { "Lifecycle" = "drivers/building/lifecycle.md" },
      { "Testing" = "drivers/building/testing.md" },
    ]},
  ]},
  { "AI Agents & MCP" = [
    { "Setup" = "ai-agents/index.md" },
    { "Tool catalog" = "ai-agents/tool-catalog.md" },
    { "Resources" = "ai-agents/resources.md" },
    { "Example workflows" = "ai-agents/workflows.md" },
  ]},
  { "API Reference" = [
    { "Pkl modules" = "api-reference/pkl-modules.md" },
    { "CLI reference" = "api-reference/cli.md" },
    { "Connect-RPC API" = "api-reference/connect-rpc.md" },
    { "Event types" = "api-reference/event-types.md" },
  ]},
  { "Operations" = [
    { "Overview" = "operations/index.md" },
    { "Deployment" = "operations/deployment.md" },
    { "Backup & Restore" = "operations/backup-restore.md" },
    { "Updates" = "operations/updates.md" },
    { "Observability" = "operations/observability.md" },
  ]},
  { "Migration from HA" = [
    { "Overview" = "migration/index.md" },
    { "What transfers" = "migration/what-transfers.md" },
    { "Jinja → Starlark" = "migration/jinja-to-starlark.md" },
    { "Post-migration checklist" = "migration/post-migration.md" },
  ]},
  { "Edge Agents" = [
    { "Overview" = "edge-agents/index.md" },
    { "Pairing" = "edge-agents/pairing.md" },
    { "Resilience" = "edge-agents/resilience.md" },
  ]},
  { "Contributing" = [
    { "Overview" = "contributing/index.md" },
    { "Dev setup" = "contributing/dev-setup.md" },
    { "Architecture internals" = "contributing/architecture-internals.md" },
  ]},
]
```

---

## 6. Source Mapping

Each doc page draws primarily from the following sources:

| Page(s) | Primary source |
|---|---|
| Introduction, Architecture | Master design §1–3 |
| vs-home-assistant | Master design §4.4, §4.5 |
| Domain model | Master design §4 |
| Event sourcing | Master design §5 |
| Config model | Master design §6.5–6.9 |
| Configuration/* | Master design §6, child spec C4 |
| Automations/* | Master design §6.8, child spec C5, C6 |
| Drivers (using) | Master design §6.1–6.3 |
| Drivers (building) | Child spec C3 |
| AI Agents & MCP | Child spec C8, `gohome/docs/mcp-setup.md` |
| Connect-RPC API | Child spec C7 |
| Pkl modules | Child spec C4, Pkl source files |
| CLI reference | Source: `internal/cli/` |
| Event types | Source: `proto/gohome/event/v1/event.proto` |
| Operations | Master design §8, child spec C13 |
| Migration | Child spec C11 |
| Edge Agents | Child spec C12 |
| Auth | Child spec C9 |
| Web UI / Dashboards | Child spec C10 |

---

## 7. Implementation Notes

- Pages that document unimplemented features open with the appropriate status badge admonition.
- The existing `gohome/docs/mcp-setup.md` is superseded by `ai-agents/index.md` — the gohome repo README should link to the gohome-docs site instead.
- Config examples in `gohome/examples/` are referenced (not duplicated) from the configuration and first-run pages.
- The `markdown.md` placeholder in `gohome-docs/docs/` is removed.
- The `index.md` placeholder is replaced with the proper landing page.
- All Mermaid diagrams from the spec docs are reused as-is.

---

*End of design document.*
