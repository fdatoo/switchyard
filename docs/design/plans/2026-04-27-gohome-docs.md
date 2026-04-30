# gohome Documentation Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete 52-page Zensical documentation site for gohome covering operators, driver developers, and contributors.

**Architecture:** Single topic-based Markdown site built with Zensical (MkDocs Material fork). Config in `zensical.toml`. Custom CSS admonitions for status badges (`status-planned`, `status-wip`, `status-alpha`). All content drawn from design specs at `/Users/fdatoo/Desktop/GoHome/docs/superpowers/specs/` and source code at `/Users/fdatoo/Desktop/GoHome/gohome/`.

**Tech Stack:** Zensical, Markdown, Mermaid diagrams. Working directory: `/Users/fdatoo/Desktop/GoHome/gohome-docs`.

---

## File Structure

**Modify:**
- `zensical.toml` — site config, nav, extra_css

**Create:**
- `docs/stylesheets/extra.css`
- `docs/index.md` (replace placeholder)
- `docs/introduction/index.md`, `architecture.md`, `vs-home-assistant.md`, `changelog.md`
- `docs/installation/index.md`, `binary.md`, `docker.md`, `systemd.md`, `first-run.md`
- `docs/concepts/index.md`, `domain-model.md`, `event-sourcing.md`, `config-model.md`
- `docs/configuration/index.md`, `entities.md`, `areas-zones.md`, `drivers.md`, `scenes.md`, `dashboards.md`, `auth.md`, `secrets.md`
- `docs/automations/index.md`, `triggers.md`, `conditions.md`, `actions.md`, `scripts.md`, `starlark.md`, `computed-entities.md`
- `docs/drivers/index.md`, `first-party.md`, `building/index.md`, `building/manifest.md`, `building/go-sdk.md`, `building/lifecycle.md`, `building/testing.md`
- `docs/ai-agents/index.md`, `tool-catalog.md`, `resources.md`, `workflows.md`
- `docs/api-reference/pkl-modules.md`, `cli.md`, `connect-rpc.md`, `event-types.md`
- `docs/operations/index.md`, `deployment.md`, `backup-restore.md`, `updates.md`, `observability.md`
- `docs/migration/index.md`, `what-transfers.md`, `jinja-to-starlark.md`, `post-migration.md`
- `docs/edge-agents/index.md`, `pairing.md`, `resilience.md`
- `docs/contributing/index.md`, `dev-setup.md`, `architecture-internals.md`

**Delete:**
- `docs/markdown.md`

---

### Task 1: Site Foundation

**Files:**
- Modify: `zensical.toml`
- Create: `docs/stylesheets/extra.css`
- Delete: `docs/markdown.md`

- [ ] **Step 1: Update `zensical.toml`**

Replace the entire file with:

```toml
[project]
site_name = "gohome"
site_description = "Go-native home automation. Event-sourced, typed, agent-friendly."
site_author = "Fynn Datoo"
copyright = "Copyright &copy; 2026 Fynn Datoo"

extra_css = ["stylesheets/extra.css"]

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

[project.theme]
language = "en"
features = [
  "announce.dismiss",
  "content.code.annotate",
  "content.code.copy",
  "content.code.select",
  "content.footnote.tooltips",
  "content.tabs.link",
  "content.tooltips",
  "navigation.footer",
  "navigation.indexes",
  "navigation.instant",
  "navigation.instant.prefetch",
  "navigation.path",
  "navigation.sections",
  "navigation.top",
  "navigation.tracking",
  "search.highlight",
]

[[project.theme.palette]]
scheme = "default"
toggle.icon = "lucide/sun"
toggle.name = "Switch to dark mode"

[[project.theme.palette]]
scheme = "slate"
toggle.icon = "lucide/moon"
toggle.name = "Switch to light mode"

[project.markdown_extensions.abbr]
[project.markdown_extensions.admonition]
[project.markdown_extensions.attr_list]
[project.markdown_extensions.def_list]
[project.markdown_extensions.footnotes]
[project.markdown_extensions.md_in_html]
[project.markdown_extensions.toc]
permalink = true
[project.markdown_extensions.pymdownx.betterem]
[project.markdown_extensions.pymdownx.caret]
[project.markdown_extensions.pymdownx.details]
[project.markdown_extensions.pymdownx.emoji]
emoji_generator = "zensical.extensions.emoji.to_svg"
emoji_index = "zensical.extensions.emoji.twemoji"
[project.markdown_extensions.pymdownx.highlight]
anchor_linenums = true
line_spans = "__span"
pygments_lang_class = true
[project.markdown_extensions.pymdownx.inlinehilite]
[project.markdown_extensions.pymdownx.keys]
[project.markdown_extensions.pymdownx.magiclink]
[project.markdown_extensions.pymdownx.mark]
[project.markdown_extensions.pymdownx.smartsymbols]
[project.markdown_extensions.pymdownx.superfences]
custom_fences = [
  { name = "mermaid", class = "mermaid", format = "pymdownx.superfences.fence_code_format" }
]
[project.markdown_extensions.pymdownx.tabbed]
alternate_style = true
combine_header_slug = true
[project.markdown_extensions.pymdownx.tasklist]
custom_checkbox = true
[project.markdown_extensions.pymdownx.tilde]
```

- [ ] **Step 2: Create `docs/stylesheets/extra.css`**

```css
/* ── Status badge admonitions ───────────────────────────────── */

/* status-planned: orange — designed but not yet implemented */
.md-typeset .admonition.status-planned,
.md-typeset details.status-planned {
  border-color: rgb(255, 145, 0);
}
.md-typeset .status-planned > .admonition-title,
.md-typeset .status-planned > summary {
  background-color: rgba(255, 145, 0, 0.1);
  border-color: rgb(255, 145, 0);
}
.md-typeset .status-planned > .admonition-title::before,
.md-typeset .status-planned > summary::before {
  background-color: rgb(255, 145, 0);
  -webkit-mask-image: var(--md-admonition-icon--warning);
          mask-image: var(--md-admonition-icon--warning);
}

/* status-wip: blue — in active development */
.md-typeset .admonition.status-wip,
.md-typeset details.status-wip {
  border-color: rgb(0, 105, 218);
}
.md-typeset .status-wip > .admonition-title,
.md-typeset .status-wip > summary {
  background-color: rgba(0, 105, 218, 0.1);
  border-color: rgb(0, 105, 218);
}
.md-typeset .status-wip > .admonition-title::before,
.md-typeset .status-wip > summary::before {
  background-color: rgb(0, 105, 218);
  -webkit-mask-image: var(--md-admonition-icon--info);
          mask-image: var(--md-admonition-icon--info);
}

/* status-alpha: purple — shipped but API unstable */
.md-typeset .admonition.status-alpha,
.md-typeset details.status-alpha {
  border-color: rgb(148, 0, 211);
}
.md-typeset .status-alpha > .admonition-title,
.md-typeset .status-alpha > summary {
  background-color: rgba(148, 0, 211, 0.1);
  border-color: rgb(148, 0, 211);
}
.md-typeset .status-alpha > .admonition-title::before,
.md-typeset .status-alpha > summary::before {
  background-color: rgb(148, 0, 211);
  -webkit-mask-image: var(--md-admonition-icon--abstract);
          mask-image: var(--md-admonition-icon--abstract);
}
```

- [ ] **Step 3: Delete `docs/markdown.md`**

```bash
rm /Users/fdatoo/Desktop/GoHome/gohome-docs/docs/markdown.md
```

- [ ] **Step 4: Verify site builds**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
zensical build
```

Expected: build succeeds (the nav references pages that don't exist yet, so warnings are expected — but no Python errors).

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add zensical.toml docs/stylesheets/extra.css
git rm docs/markdown.md
git commit -m "docs: configure site, add status badge CSS, remove placeholder"
```

---

### Task 2: Introduction Section

**Source:** `/Users/fdatoo/Desktop/GoHome/docs/superpowers/specs/2026-04-21-gohome-master-design.md` §1–3, §4.4, §4.5

**Files:**
- Create: `docs/introduction/index.md`
- Create: `docs/introduction/architecture.md`
- Create: `docs/introduction/vs-home-assistant.md`
- Create: `docs/introduction/changelog.md`

- [ ] **Step 1: Write `docs/introduction/index.md`**

```markdown
---
icon: lucide/home
---

# What is gohome?

**gohome** is a Go-native home automation platform built for the prosumer and homelab audience — comfortable editing a config file, running Proxmox or Docker, sometimes managing things through an AI agent.

Three architectural bets distinguish it from Home Assistant as it exists today:

1. **Event sourcing as the architectural spine.** The event log is the source of truth; current state is a materialized view. This gives you time-travel debugging, lossless history, free audit, and clean remote-agent replication — all from one design decision.
2. **Declarative config (Pkl) with sandboxed dynamic logic (Starlark).** No YAML+Jinja. Config is typed, git-versioned, AI-editable, and validated at load time. Logic is a real programming language, not a templating kludge.
3. **Agent-native from day one.** An MCP server is a first-class API surface, not a bolted-on integration. AI agents can inspect state, call services, edit Pkl config with validation feedback, and evaluate Starlark — out of the box.

## Who is gohome for?

Prosumers and homelab users. You probably run Proxmox, Docker, or bare-metal Linux. You have kids or guests on the home network. You're comfortable in a config file, maybe in git. You might use Claude or another AI agent to help manage your home.

If you're currently running Home Assistant and finding yourself fighting its YAML+Jinja automation model, the lack of typed entities, or the inability to reason about what happened and when — gohome is designed for you.

## What gohome is not (v1.0)

- **Not a high-availability cluster.** Single-primary only.
- **Not a drop-in HA replacement.** No HA API compatibility shim. [Migration tooling](../migration/index.md) imports your config, but existing HA apps won't talk to gohome unmodified.
- **Not an appliance OS.** No HAOS equivalent. You run the binary or container on your own OS.
- **Not a voice-assistant platform.** Voice integrations are drivers or agents against the API.
- **No commercial plugin marketplace.** A signed driver directory, not a store.

## Getting started

- [Install gohome](../installation/index.md)
- [Migrate from Home Assistant](../migration/index.md)
- [Build a driver](../drivers/building/index.md)
```

- [ ] **Step 2: Write `docs/introduction/architecture.md`**

Draw from master design §3. Include the full Mermaid diagram (copy verbatim from `2026-04-21-gohome-master-design.md` §3.1). Include the binaries table, internal modules table, and public contracts list from §3.2–3.5.

```markdown
---
icon: lucide/boxes
---

# Architecture

## Component map

[paste Mermaid flowchart from master design §3.1 verbatim]

## Binaries

| Binary | Purpose | Distribution |
|---|---|---|
| `gohomed` | Daemon. All long-running logic. Embeds the web UI. | Static binary, OCI image, `.deb`/`.rpm`, Homebrew |
| `gohome` | CLI for humans and agents. Thin Connect-RPC client. | Static binary |
| `gohome-edge` | Optional edge supervisor for remote hosts. | Static binary, OCI image |

## Internal modules

Each module has a narrow Go-package interface. They communicate through well-defined boundaries, not direct imports.

| Module | Responsibility |
|---|---|
| `eventstore` | Append-only event log, snapshots, replay, tailing. SQLite-backed. Sole gatekeeper of event data. |
| `state` | In-memory materialized view over the event log. Fast reads. |
| `registry` | Device/entity/area/zone registry projection. SQLite tables. |
| `carport-host` | gRPC supervisor for local driver subprocesses and remote edge-agent connections. |
| `api` | Connect-RPC service implementations. |
| `mcp` | MCP server — thin shim over `api`. |
| `web` | Embedded static assets (React bundle) + HTTP mux. |
| `config` | Pkl loader, validator, diff-based reloader. |
| `automation` | Starlark sandbox + automation/script runtime. |
| `auth` | Users, roles, sessions, passkeys, API tokens, OIDC, policy enforcement. |
| `recorder` | Long-term retention: vacuum/checkpoint scheduling. |
| `supervisord` | Main orchestrator: wires modules, handles startup/shutdown/reload. |

## Public contracts

These five surfaces are the hardest to change — versioned and backward-compatibility-governed:

1. **Carport** — gRPC/protobuf between the Carport host and every driver.
2. **Event schema** — protobuf messages persisted in the event log.
3. **Connect-RPC API** — the user-facing RPC surface (`gohome.v1.*`).
4. **MCP tool surface** — generated from the Connect-RPC surface.
5. **Pkl module schemas** — `gohome.*` Pkl modules users import.

Everything else is an internal Go package boundary that can be rearranged freely.
```

- [ ] **Step 3: Write `docs/introduction/vs-home-assistant.md`**

```markdown
---
icon: lucide/git-compare
---

# gohome vs. Home Assistant

gohome's vocabulary is deliberately close to Home Assistant where muscle memory transfers, and deliberately diverges where HA's model has become confusing.

## What gohome fixes

| HA concept | HA problem | gohome fix |
|---|---|---|
| Integration | Means both the code and a configured instance — conflated | Split: **driver** (code artifact) + **driver instance** (user config) |
| Entity state | Always a string (`"on"`, `"23.5"`) | Typed state via entity class (`bool`, `int`, `float`, enum, struct) |
| Entity attributes | Untyped dict blobs | Typed attributes via entity class |
| Services | Global verb-space called against entity targets | **Capabilities** — typed methods on entity classes |
| Template entities | Second-class citizens, Jinja-backed | **Computed entities** are first-class, Starlark-backed |
| Automation logic | Jinja2 embedded in YAML | Starlark: a real sandboxed language with syntax checking at config-load time |
| Areas | Floppy groupings | Hierarchical first-class geometry |
| Zones | Bolted-on geofence feature | First-class, lat/lon + radius |
| Entity IDs | Free-form strings | Still `domain.name`, but domain is a closed versioned enum |

## What gohome keeps from HA

These HA concepts transfer cleanly and gohome uses the same terminology:

- Entity domain names (`light`, `switch`, `sensor`, `binary_sensor`, `climate`, `cover`, `media_player`, `camera`, `lock`, `person`, `vacuum`, `fan`, `input_*`, …)
- Entity ID format: `<domain>.<name>`
- Areas and zones
- Scenes and scripts
- Automation trigger → conditions → actions shape
- Persons
- State + attributes terminology

## What you give up

- **No HA API compatibility shim.** Existing HA apps (Companion app, custom integrations) won't talk to gohome unmodified. This is a deliberate non-goal for v1.0.
- **HACS integrations don't transfer.** The HA import tool maps known integrations; HACS-only ones produce TODO stubs.
- **Supervisor add-ons don't transfer.** gohome has no add-on system; equivalent capabilities are drivers or external services.
- **HA mobile app won't work.** The web UI is a PWA; native wrappers are a future deferred feature.
- **Passwords aren't migrated.** Users need to re-register passkeys after migration.

## Decision record

For the full reasoning behind these choices, see the [master design decision record](https://github.com/fynn-labs/gohome/blob/main/docs/) (§9 of the master design spec).
```

- [ ] **Step 4: Write `docs/introduction/changelog.md`**

```markdown
---
icon: lucide/history
---

# Changelog

## Unreleased

Active development. Tracking milestone completion:

- **C1** — Event core & storage: complete
- **C2** — Carport protocol: complete
- **C3** — Driver SDK: complete
- **C4** — Pkl config: complete
- **C5** — Starlark runtime: complete
- **C6** — Automation & script engine: complete
- **C7** — Connect-RPC API: complete
- **C8** — MCP server: complete
- **C9** — Auth & policy: complete
- **C10** — Web UI: in progress
- **C11** — HA import tool: in progress
- **C12** — Edge agent: in progress
- **C13** — Distribution & operations: planned

First stable release will be tagged `v1.0.0` when all C1–C13 milestones are shipped.
```

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/introduction/
git commit -m "docs: add introduction section (what is gohome, architecture, vs HA, changelog)"
```

---

### Task 3: Installation Section

**Source:** Master design §8.2; `gohome/README.md`; `gohome/Taskfile.yml`

**Files:** `docs/installation/index.md`, `binary.md`, `docker.md`, `systemd.md`, `first-run.md`

- [ ] **Step 1: Write `docs/installation/index.md`**

```markdown
---
icon: lucide/download
---

# Installation

gohome ships three binaries. You need at minimum `gohomed` (the daemon) and `gohome` (the CLI).

| Binary | What it is |
|---|---|
| `gohomed` | The daemon. Runs continuously, manages all state. |
| `gohome` | The CLI. Inspect state, manage config, run the MCP server. |
| `gohome-edge` | Optional. Run on remote hosts to host drivers over TLS. |

## Platform support

| Platform | `gohomed` | `gohome` | `gohome-edge` |
|---|---|---|---|
| linux/amd64 | ✓ | ✓ | ✓ |
| linux/arm64 | ✓ | ✓ | ✓ |
| linux/armv7 | ✓ | ✓ | ✓ |
| darwin/arm64 | ✓ | ✓ | — |
| darwin/amd64 | ✓ | ✓ | — |
| windows/amd64 | ✓ | ✓ | — |

## Choose your install path

=== "Static binary (Linux/macOS)"
    Fastest way to get started. [Static binary →](binary.md)

=== "Docker / OCI"
    Best for Unraid, Portainer, Docker Compose setups. [Docker →](docker.md)

=== "systemd / packages"
    Best for bare-metal Linux servers. `.deb`/`.rpm` + systemd unit. [systemd →](systemd.md)
```

- [ ] **Step 2: Write `docs/installation/binary.md`**

```markdown
# Static binary

## Download

Grab the latest release from GitHub Releases. Replace `<version>` and `<platform>`:

```bash
# Linux amd64
curl -L -o gohomed https://github.com/fynn-labs/gohome/releases/latest/download/gohomed-linux-amd64
curl -L -o gohome  https://github.com/fynn-labs/gohome/releases/latest/download/gohome-linux-amd64

# macOS arm64 (Apple Silicon)
curl -L -o gohomed https://github.com/fynn-labs/gohome/releases/latest/download/gohomed-darwin-arm64
curl -L -o gohome  https://github.com/fynn-labs/gohome/releases/latest/download/gohome-darwin-arm64
```

## Verify signature (recommended)

Each binary is signed with [sigstore/cosign](https://docs.sigstore.dev/). Verify before running:

```bash
cosign verify-blob \
  --certificate gohomed-linux-amd64.pem \
  --signature  gohomed-linux-amd64.sig \
  gohomed-linux-amd64
```

## Install

```bash
chmod +x gohomed gohome
sudo mv gohomed gohome /usr/local/bin/
gohome version
```

## Next step

[First run →](first-run.md)
```

- [ ] **Step 3: Write `docs/installation/docker.md`**

```markdown
# Docker

## Docker Compose

```yaml
services:
  gohomed:
    image: ghcr.io/fynn-labs/gohomed:latest
    container_name: gohomed
    restart: unless-stopped
    ports:
      - "8080:8080"     # Connect-RPC / web UI
      - "9090:9090"     # Prometheus metrics (optional)
    volumes:
      - ./config:/config        # Pkl config directory
      - gohome-data:/data       # SQLite DB, driver state
    environment:
      GOHOME_CONFIG_DIR: /config
      GOHOME_DATA_DIR: /data

volumes:
  gohome-data:
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `GOHOME_CONFIG_DIR` | `~/.config/gohome` | Pkl config directory |
| `GOHOME_DATA_DIR` | `~/.local/share/gohome` | SQLite DB and runtime data |
| `GOHOME_LISTEN` | `:8080` | HTTP listener address |
| `GOHOME_LOG_LEVEL` | `warn` | `error\|warn\|info\|debug` |
| `GOHOME_LOG_FORMAT` | `json` | `json\|text` |

## Accessing the CLI

The `gohome` CLI connects to the daemon over a Unix socket or TCP. In Docker, use the TCP endpoint:

```bash
docker exec -it gohomed gohome --endpoint tcp://localhost:8080 status
```

Or run the CLI container separately:

```bash
docker run --rm ghcr.io/fynn-labs/gohome:latest \
  gohome --endpoint tcp://gohomed:8080 status
```

## Next step

[First run →](first-run.md)
```

- [ ] **Step 4: Write `docs/installation/systemd.md`**

```markdown
# systemd / packages

## Debian / Ubuntu

```bash
curl -fsSL https://packages.gohome.dev/gpg | sudo gpg --dearmor -o /usr/share/keyrings/gohome.gpg
echo "deb [signed-by=/usr/share/keyrings/gohome.gpg] https://packages.gohome.dev/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/gohome.list
sudo apt update && sudo apt install gohomed gohome
```

## RHEL / Fedora

```bash
sudo dnf config-manager --add-repo https://packages.gohome.dev/rpm/gohome.repo
sudo dnf install gohomed gohome
```

## macOS (Homebrew)

```bash
brew tap fynn-labs/gohome
brew install gohomed gohome
```

## systemd unit

The packages install a systemd unit template. Enable and start:

```bash
# Create data and config directories
sudo mkdir -p /etc/gohome /var/lib/gohome
sudo chown gohome:gohome /var/lib/gohome

# Enable and start
sudo systemctl enable --now gohomed

# Check status
sudo systemctl status gohomed

# View logs
sudo journalctl -u gohomed -f
```

The unit file lives at `/lib/systemd/system/gohomed.service`. Key defaults:

```ini
[Service]
User=gohome
Environment=GOHOME_CONFIG_DIR=/etc/gohome
Environment=GOHOME_DATA_DIR=/var/lib/gohome
ExecStart=/usr/bin/gohomed
Restart=on-failure
RestartSec=5s
```

Override with a drop-in: `sudo systemctl edit gohomed`.

## Next step

[First run →](first-run.md)
```

- [ ] **Step 5: Write `docs/installation/first-run.md`**

```markdown
# First run

All install paths converge here.

## 1. Create a config directory

```bash
mkdir -p ~/.config/gohome
```

## 2. Copy the minimal config

```bash
# If you cloned the gohome repo:
cp /path/to/gohome/examples/minimal-main.pkl ~/.config/gohome/main.pkl

# Or download directly:
curl -L -o ~/.config/gohome/main.pkl \
  https://raw.githubusercontent.com/fynn-labs/gohome/main/examples/minimal-main.pkl
```

The minimal config sets up listeners and leaves all other sections empty. Edit it to match your setup before adding drivers.

## 3. Validate your config

```bash
gohome config validate --config-dir ~/.config/gohome
```

Expected output:

```
✓ Config valid. No changes pending.
```

Fix any reported errors before proceeding.

## 4. Start the daemon

=== "Direct"
    ```bash
    gohomed --config-dir ~/.config/gohome --data-dir ~/.local/share/gohome
    ```

=== "systemd"
    ```bash
    sudo systemctl start gohomed
    ```

=== "Docker Compose"
    ```bash
    docker compose up -d
    ```

## 5. Confirm it's running

```bash
gohome status
```

Expected:

```
gohomed  running  pid=12345  uptime=3s  events=1
```

## Next steps

- [Configure entities and drivers](../configuration/index.md)
- [Write your first automation](../automations/index.md)
- [Connect an AI agent via MCP](../ai-agents/index.md)
```

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/installation/
git commit -m "docs: add installation section (binary, docker, systemd, first-run)"
```

---

### Task 4: Concepts Section

**Source:** Master design §4 (domain model), §5 (event sourcing), §6.5–6.9 (config model)

**Files:** `docs/concepts/index.md`, `domain-model.md`, `event-sourcing.md`, `config-model.md`

- [ ] **Step 1: Write `docs/concepts/index.md`**

```markdown
---
icon: lucide/book-open
---

# Concepts

Three concepts underpin everything in gohome. Read them before diving into configuration.

| Concept | Why it matters |
|---|---|
| [Domain model](domain-model.md) | The vocabulary: entities, devices, drivers, areas, automations. What each thing is and how they relate. |
| [Event sourcing](event-sourcing.md) | Why gohome stores every state change as an immutable event, and what this gives you. |
| [Config model](config-model.md) | How Pkl (structure) and Starlark (logic) divide responsibilities, and why this beats YAML+Jinja. |
```

- [ ] **Step 2: Write `docs/concepts/domain-model.md`**

Draw from master design §4. Include every noun with a brief definition and a Pkl example where applicable. Include the capabilities vs. services comparison and the HA warts table.

Key sections to include:
- `## Driver` — code artifact (Go binary or WASM), speaks Carport, knows one protocol
- `## Driver instance` — user config binding driver to parameters (slug, typed config fields)
- `## Device` — logical unit surfaced by a driver instance; belongs to exactly one driver instance
- `## Entity` — addressable typed property/capability of a device. Show: unique ID `domain.name`, typed state, typed attributes, capabilities
- `## Entity class` — reusable Pkl-declared type (`gohome.entities.Light`, etc.)
- `## Computed entity` — state defined by Starlark expression. Show the `ComputedEntity` Pkl snippet from the master design
- `## Area` — spatial grouping, hierarchical
- `## Zone` — geographic geofence (lat/lon + radius)
- `## Automation` — trigger → conditions → actions
- `## Script` — callable named Starlark function
- `## Scene` — declarative target state
- `## Dashboard / Widget` — brief
- `## User, Role, Policy` — brief

Also include:
- Identity conventions table (`domain.name`, ULID slugs, user-chosen slugs)
- Capabilities vs. services comparison (show the Starlark example from master design §4.3)
- The HA warts table from master design §4.4

- [ ] **Step 3: Write `docs/concepts/event-sourcing.md`**

Written for operators, not engineers. Draw from master design §5.

Key sections:
- `## Every state change is an event` — plain-language explanation; events are immutable, append-only; current state is computed from events
- `## What this gives you` — time-travel debugging, audit log, answering "what happened at 2am?"; show `gohome events query` and `gohome events replay --at` commands
- `## What it costs` — SQLite grows over time; disk estimate (~40 GB/year at typical prosumer scale of 200 entities at 30s cadence); retention config via `RetentionPolicy`
- `## The subscription model` — brief: automations, UI, MCP all get live updates through the same `Subscribe` primitive
- `## Snapshots` — periodic snapshots compress startup time; transparent to the user

Include a worked example: "What happened to my kitchen lights at 2am?"

```bash
gohome events query \
  --entity light.kitchen \
  --after "2026-04-27T02:00:00" \
  --before "2026-04-27T03:00:00"
```

- [ ] **Step 4: Write `docs/concepts/config-model.md`**

Draw from master design §6.5–6.9.

Key sections:
- `## Pkl for structure` — typed, validated at load time, git-friendly, AI-editable; catches errors before the daemon touches the network; show `gohome config validate` workflow
- `## Starlark for logic` — sandboxed, real language, not a templating kludge; no unrestricted I/O; show a simple automation handler
- `## The seam` — Pkl holds Starlark inline (one-liners) or by path (`.star` files); syntax-checked at `config validate` time; show `StarlarkExpr` typealias
- `## Config directory layout` — show the directory tree from master design §6.5
- `## Secrets` — never in Pkl source; `env:`, `file:`, `keyring:` sources; show `Secret` typealias
- `## Diff-based reload` — `gohome config apply` diffs new vs applied snapshot; unchanged driver instances are not restarted; `--dry-run` shows the diff

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/concepts/
git commit -m "docs: add concepts section (domain model, event sourcing, config model)"
```

---

### Task 5: Configuration Section

**Source:** Master design §6; child spec `2026-04-22-c4-pkl-config-design.md`; Pkl source at `gohome/internal/config/pkl/gohome/`

**Files:** All 8 pages under `docs/configuration/`

- [ ] **Step 1: Write `docs/configuration/index.md`**

```markdown
---
icon: lucide/settings
---

# Configuration

gohome config is a directory of [Pkl](https://pkl-lang.org) files. `main.pkl` is the root; it imports the rest.

## Directory layout

```
~/.config/gohome/
├── main.pkl                   # root — imports everything else
├── drivers.pkl                # driver instances
├── areas.pkl                  # areas + hierarchy
├── zones.pkl                  # geofences
├── entities/
│   ├── computed.pkl           # computed entities
│   └── overrides.pkl          # entity name/area overrides
├── automations/
│   ├── lighting.pkl
│   └── handlers/
│       └── evening.star
├── scripts/
│   └── morning_routine.star
├── scenes.pkl
├── dashboards/
│   └── default.pkl
├── auth/
│   ├── users.pkl
│   ├── roles.pkl
│   └── policies.pkl
└── secrets.pkl
```

## Workflow

```bash
# Validate (no side-effects)
gohome config validate --config-dir ~/.config/gohome

# Preview what would change
gohome config apply --dry-run --config-dir ~/.config/gohome

# Apply
gohome config apply --config-dir ~/.config/gohome
```

`config apply` performs a **diff-based reload**: it compares the newly compiled config snapshot to the currently applied one and computes the minimal set of side-effects. Unchanged driver instances are not restarted.

After applying, a `ConfigApplied` event is written to the event log recording what changed.
```

- [ ] **Step 2: Write `docs/configuration/entities.md`**

Draw from `gohome/internal/config/pkl/gohome/entities.pkl`. Cover:
- Standard entity classes listing
- Entity ID format `domain.name`
- Typed state and attributes
- `entities/overrides.pkl` for renaming and area assignment
- Custom entity classes

Include a Pkl example of an entity override and a custom class definition.

- [ ] **Step 3: Write `docs/configuration/areas-zones.md`**

Cover:
- `areas.pkl` — hierarchical areas; parent assignment
- `zones.pkl` — geofence zones with lat/lon/radius
- Assigning entities to areas

Include Pkl examples for both.

- [ ] **Step 4: Write `docs/configuration/drivers.md`**

Draw from master design §6.4, §6.5. Cover:
- Declaring a driver instance in `drivers.pkl`
- The typed config per driver class (import the driver's Pkl module)
- Multiple instances of the same driver
- Secret references in driver config (using `Secret` typealias)

Include the Hue driver example from master design §6.4:

```pkl
import "gohome/drivers/hue.pkl" as hue

drivers = new Mapping<String, DriverInstance> {
  ["hue_main"] = new hue.Instance {
    bridgeAddress = "10.0.0.42"
    apiKey = read("env:HUE_API_KEY")
    pollIntervalSeconds = 15
  }
}
```

- [ ] **Step 5: Write `docs/configuration/scenes.md`**

Cover:
- Declaring scenes in `scenes.pkl` (Pkl class, list of entity targets and state)
- `gohome scene apply <slug>`
- The `SceneApplied` event (records what changed and what was already in the target state)

Include a Pkl example scene.

- [ ] **Step 6: Write `docs/configuration/dashboards.md`**

!!! status-wip badge at top.

Draw from child spec `2026-04-26-c10-web-ui-architecture-design.md`. Cover:
- `Dashboard` Pkl class structure (pages, grid, widget instances)
- Built-in widget classes
- WYSIWYG round-trip (edit → Pkl write-back preserving hand-edited Starlark)
- Widget packs (install as JS bundle + Pkl module)

- [ ] **Step 7: Write `docs/configuration/auth.md`**

Draw from child spec `2026-04-25-c9-auth-and-policy-design.md`. Cover:
- Users (Pkl-declared, auth methods: passkey, password, OIDC, API tokens)
- Roles (built-in: `admin`, `member`, `guest`; custom roles)
- Policies — include the full kids-policy example from master design §7.4:

```pkl
policies = List(
  new Policy {
    name = "kids_can_control_bedrooms_only"
    subjects = List(roles.kids)
    allow = new CapabilityAllow {
      capabilities = List("turn_on", "turn_off", "set_brightness")
      targets = new EntitySelector { areas = List(areas.kid_bedrooms) }
    }
    deny = new CapabilityDeny {
      capabilities = List("*")
      targets = new EntitySelector { classes = List(entities.Lock, entities.Alarm) }
    }
  },
)
```

- Enforcement points (every Connect-RPC handler, subscription stream filtering)
- Auth events on the log

- [ ] **Step 8: Write `docs/configuration/secrets.md`**

Draw from `gohome/internal/config/pkl/gohome/base.pkl`. Cover all secret source types with actual typealias definitions:

```pkl
// From gohome.base:
typealias EnvSecret     = String(matches(Regex("env:[A-Z_][A-Z0-9_]*")))
typealias FileSecret    = String(matches(Regex("file:/.+")))
typealias KeyringSecret = String(matches(Regex("keyring:[^/]+/.+")))
typealias Secret        = String(matches(Regex("(env:[A-Z_]|file:/|keyring:).+")))
```

Usage examples:

```pkl
apiKey: Secret = "env:HUE_API_KEY"           // from environment variable
password: Secret = "file:/run/secrets/db_pw"  // from file
token: Secret = "keyring:system/gohome-token" // from OS keyring
```

Cover: resolve-at-runtime semantics; secrets never written to event log; community modules (`vault:`, `1password:`, `bitwarden:`) as future extensions.

- [ ] **Step 9: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/configuration/
git commit -m "docs: add configuration section (entities, areas, drivers, scenes, dashboards, auth, secrets)"
```

---

### Task 6: Automations & Scripts Section

**Source:** Master design §6.8; child specs `c5-starlark-runtime-design.md`, `c6-automation-engine-design.md`; `gohome/internal/config/pkl/gohome/automations.pkl`

**Files:** All 7 pages under `docs/automations/`

- [ ] **Step 1: Write `docs/automations/index.md`**

Show the full automation Pkl shape. Draw from `automations.pkl` (which you've read):

```markdown
---
icon: lucide/zap
---

# Automations & Scripts

An automation is a trigger → conditions → actions rule. Declared in Pkl; dynamic logic written in Starlark.

## Anatomy of an automation

```pkl
import "gohome:automations" as auto

automations = List(
  new auto.Automation {
    id = "evening_lights"
    triggers = List(
      new auto.TimeTrigger { at = "sunset" }
    )
    conditions = List(
      new auto.StateCondition { entity = "binary_sensor.someone_home"; equals = "on" }
    )
    actions = List(
      new auto.CallServiceAction {
        entity = "light.living_room"
        capability = "turn_on"
        args = new { ["brightness"] = "80" ["transition"] = "2" }
      }
    )
    mode = "single"  // single | queued | restart | parallel
  }
)
```

## Execution mode

| Mode | Behaviour when triggered while already running |
|---|---|
| `single` | Ignore new trigger |
| `queued` | Queue up to `maxQueued` (default 10) |
| `restart` | Cancel current run, start fresh |
| `parallel` | Run a new instance concurrently |
```

- [ ] **Step 2: Write `docs/automations/triggers.md`**

Draw all four trigger classes from `automations.pkl`. For each: class name, fields with types, and a complete Pkl example.

| Trigger | Key fields |
|---|---|
| `StateChangeTrigger` | `entities: Listing<String>`, `from: String?`, `to: String?`, `forDur: Duration?` |
| `EventTrigger` | `kind: String`, `data: Mapping<String,String>?` |
| `TimeTrigger` | `at: String?` (e.g. `"sunset"`, `"06:30"`), `cron: String?`, `every: Duration?` |
| `WebhookTrigger` | `path: String` (e.g. `"/hooks/doorbell"`), `methods: Listing<String>` |

Include complete Pkl examples for each, plus a note that `WebhookTrigger` requires no auth by design but path should be treated as a secret.

- [ ] **Step 3: Write `docs/automations/conditions.md`**

Draw all condition classes from `automations.pkl`. Include:
- `StateCondition` — `entity`, `equals`, `oneOf`, `not`
- `NumericCondition` — `entity`, `attribute`, `op` (`lt|lte|eq|gte|gt`), `value`
- `TimeCondition` — `after`, `before`, `weekdays`
- `StarlarkCondition` — `expr: StarlarkCondition` (validated at config load)
- `AndCondition`, `OrCondition`, `NotCondition` — composition

Show a compound condition example using `AndCondition`.

- [ ] **Step 4: Write `docs/automations/actions.md`**

Draw all action classes from `automations.pkl`. Include:
- `CallServiceAction` — `entity`, `capability`, `args`
- `SceneAction` — `slug`
- `ScriptAction` — `name`, `args`
- `StarlarkAction` — `body: StarlarkScript`
- `WaitAction` — `duration`
- `SequenceBlock` — ordered list of actions
- `ParallelBlock` — concurrent actions

Show `continueOnError` field (common to all actions).

- [ ] **Step 5: Write `docs/automations/scripts.md`**

Cover:
- Declaring a script in `scripts.pkl` (name, typed parameters, body path)
- The `scripts/*.star` convention for multi-line bodies
- Calling scripts from automations (`ScriptAction`), CLI (`gohome script run`), MCP (`gohome__run_script`)
- Script return values (available to MCP callers)
- Typed parameters

Include a complete Pkl script declaration and the corresponding `.star` file.

- [ ] **Step 6: Write `docs/automations/starlark.md`**

Draw from child spec `c5-starlark-runtime-design.md`. Key sections:

`## The language` — Starlark is Python-like, sandboxed, deterministic. No file I/O, no network, no threads. Link to go.starlark.net.

`## Per-context stdlib` — Full table (draw from master design §6.8):

| Context | Available builtins |
|---|---|
| Automation handler | `state()`, `call_service()`, `sleep()`, `now()`, `log()`, `notify()`, `scene.apply()`, `event.fire()`, `random`, `time` |
| Computed entity | `state()` (read-only), `now()` — pure, no side-effects |
| Trigger condition | `state()`, `event`, `now()` — pure, bounded |
| Script | Same as automation + typed parameters |
| Widget compute | `state()` (read-only), cached |
| MCP `eval_starlark` | Scratch context, configurable scope, read-only by default |

`## state() builtin` — show the return struct: `.state` (string) and `.attributes` (dict). Example:

```python
kitchen = state("light.kitchen")
if kitchen.state == "on":
    brightness = kitchen.attributes["brightness"]
```

`## Resource limits` — wall-clock budgets (automation: 30s, computed: 100ms, condition: 50ms), step counter, memory cap. On breach: run cancelled, `AutomationFinished` event emitted with `OUTCOME_LIMIT_EXCEEDED`.

`## Shared modules` — `load("//lib/helpers.star", "compute_avg")` loads from a path relative to the config dir. No external network access.

`## gohome eval` — scratch evaluation for debugging:

```bash
gohome eval 'state("light.kitchen").state'
```

- [ ] **Step 7: Write `docs/automations/computed-entities.md`**

```markdown
# Computed entities

A computed entity's state is defined by a Starlark expression evaluated reactively over other entities' state. Re-evaluated whenever any dependency changes.

## Declaring a computed entity

```pkl
import "gohome:entities" as ent

computedEntities = List(
  new ent.ComputedEntity {
    id = "sensor.house_avg_temp"
    class = ent.Temperature
    compute = starlark"avg([state(e).attributes['value'] for e in entities(class='Temperature', area='interior')])"
  }
)
```

## Re-evaluation semantics

The runtime detects which entities are accessed during evaluation (via `state()` calls) and registers those as dependencies. When any dependency emits a `StateChanged` event, the computed entity re-evaluates. The new value is emitted as a `StateChanged` event for `sensor.house_avg_temp`.

## Limits

Computed entities run in the `computed entity` context: read-only `state()` and `now()`, 100ms wall-clock limit, no side-effects. Expressions that would call `call_service()` or `log()` fail at `config validate` time.

## Common patterns

```python
# Average of multiple sensors
avg([state(e).attributes["value"] for e in entities(class="Temperature", area="ground_floor")])

# Presence: anyone home?
any(state(e).state == "home" for e in entities(class="Person"))

# Combined state
"on" if state("switch.pump").state == "on" or state("switch.pump_backup").state == "on" else "off"
```
```

- [ ] **Step 8: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/automations/
git commit -m "docs: add automations section (triggers, conditions, actions, scripts, Starlark, computed entities)"
```

---

### Task 7: Drivers — Using

**Source:** Master design §6.1–6.3; child spec `c2-carport-protocol-design.md`

**Files:** `docs/drivers/index.md`, `docs/drivers/first-party.md`

- [ ] **Step 1: Write `docs/drivers/index.md`**

```markdown
---
icon: lucide/plug
---

# Drivers

A **driver** is a separate binary that knows how to talk to one kind of hardware or cloud system. It communicates with `gohomed` over the **Carport** gRPC protocol.

## Driver vs. driver instance

| Concept | What it is | Example |
|---|---|---|
| Driver | Code artifact — a binary | `hue-driver` binary |
| Driver instance | User config — a binding to specific parameters | `hue_main` pointing at `10.0.0.42` |

One driver binary can run as multiple instances (e.g., two Hue bridges).

## Installing a driver

```bash
gohome driver install hue
gohome driver install zigbee2mqtt
gohome driver list
```

Drivers are installed to `$GOHOME_DATA_DIR/drivers/`. Each driver is a signed binary; the signature is verified at install time.

## Driver health

```bash
gohome driver status           # all instances
gohome driver status hue_main  # one instance
```

## Updating a driver

Driver updates are independent of `gohomed` updates:

```bash
gohome driver upgrade hue      # upgrades all hue instances
gohome driver upgrade hue_main # upgrades one instance
```

No daemon restart required. The new binary goes through the Carport handshake and re-registers its instances.

## Removing a driver

```bash
gohome driver remove hue_main  # remove one instance
gohome driver uninstall hue    # uninstall binary (all instances must be removed first)
```

## Transport modes

| Mode | When used | Auth |
|---|---|---|
| Local subprocess | Default — driver on same host | Shared handshake secret via env |
| Remote (edge) | Driver on a remote host via `gohome-edge` | mTLS, CA issued by gohomed at pairing |
```

- [ ] **Step 2: Write `docs/drivers/first-party.md`**

One section per driver. For each: what it does, config fields (as a table), known caveats, and a Pkl config example. Drivers to cover:

**MQTT** — Generic MQTT broker bridge. Config: `brokerURL`, `username`, `password` (Secret), `clientID`. Produces entities from configurable topic subscriptions.

**Zigbee2MQTT** — Bridge to a running Zigbee2MQTT instance. Config: `mqttBrokerURL`, `baseTopic` (default: `zigbee2mqtt`). Produces Light, Switch, Sensor, BinarySensor entities per device.

**Matter** — Native Matter controller. Config: `storageDir`. Pairs and controls Matter devices.

**HomeKit bridge** — Exposes gohome entities to Apple HomeKit. Config: `pin`, `storagePath`. Acts as a HomeKit accessory bridge.

**ESPHome native** — Connects to ESPHome devices using the native API. Config: `host`, `port` (default: 6053), `password` (Secret).

**Z-Wave JS** — Bridge to a running Z-Wave JS server. Config: `serverURL`.

**Generic REST** — Polls a REST endpoint or handles webhooks. Config: `baseURL`, `pollIntervalSeconds`, entity mappings.

**Generic webhook** — Receives incoming webhooks and maps them to entity state. Config: entity mappings.

**Hue** (exemplar cloud integration) — Philips Hue bridge. Config: `bridgeAddress`, `apiKey` (Secret), `pollIntervalSeconds`.

**Nest** (exemplar cloud integration) — Google Nest via SDM API. Config: `projectID`, `clientID`, `clientSecret` (Secret), `refreshToken` (Secret).

Add a `!!! status-wip` badge at the top noting that first-party drivers ship as separate binaries and this catalog reflects the v1.0 target.

- [ ] **Step 3: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/drivers/index.md docs/drivers/first-party.md
git commit -m "docs: add driver usage section (overview, first-party catalog)"
```

---

### Task 8: Drivers — Building

**Source:** Child spec `2026-04-22-c3-driver-sdk-design.md`; master design §6.1–6.4; `gohome/internal/carport/fakedriver/fakedriver.go`

**Files:** All 5 pages under `docs/drivers/building/`

- [ ] **Step 1: Read the driver SDK spec**

```bash
cat /Users/fdatoo/Desktop/GoHome/docs/superpowers/specs/2026-04-22-c3-driver-sdk-design.md
```

Use its content as the primary source for the 5 building pages.

- [ ] **Step 2: Write `docs/drivers/building/index.md`**

Cover: what a driver is architecturally (a gRPC server speaking the Carport protocol), what the Go SDK provides (code-generates the gRPC stubs and provides helper types), when to use local subprocess vs edge transport, where to find the SDK (`github.com/fynn-labs/gohome/driverkit` — note: may be in `gohome-driverkit` submodule).

- [ ] **Step 3: Write `docs/drivers/building/manifest.md`**

Draw from master design §6.4. Show the full Pkl manifest example including `DriverManifest` fields, a typed `instanceConfig` class, `produces` entity list, `driverEventTypes`, and how to embed the manifest in the binary as a resource. Include the Hue manifest example from master design §6.4.

- [ ] **Step 4: Write `docs/drivers/building/go-sdk.md`**

Draw from child spec C3. Cover:
- Importing the SDK (`driverkit` package)
- Implementing the `Driver` interface
- Entity class registration
- Emitting `StateChanged` events
- Handling incoming `Command` messages
- Sending `CommandResult` (success or failure)
- Typed `DriverEvent` payloads for driver-specific events
- The `RegisterInstance` flow

Include a minimal but complete Go driver skeleton.

- [ ] **Step 5: Write `docs/drivers/building/lifecycle.md`**

Draw from master design §6.3. Show the full lifecycle state machine as a numbered list and a Mermaid state diagram:

States: `launched` → `handshaking` → `registering` → `running` ↔ `health_probe` → `shutting_down` → `stopped`. Error path: `running` → `crashed` → (backoff) → `launched`.

Cover: what happens at each stage, what gohomed does, what the driver must do, and error/timeout behaviour.

- [ ] **Step 6: Write `docs/drivers/building/testing.md`**

Draw from child spec C3. Cover:
- The `fakedriver` reference implementation at `gohome/internal/carport/fakedriver/`
- The driver test harness
- Writing integration tests that spin up a real Carport host
- `gohome test` command for end-to-end driver testing
- Common failure modes to test: crash recovery, malformed events, command timeouts

- [ ] **Step 7: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/drivers/building/
git commit -m "docs: add driver building section (Carport SDK, manifest, lifecycle, testing)"
```

---

### Task 9: AI Agents & MCP Section

**Source:** Child spec `2026-04-24-c8-mcp-server-design.md`; `gohome/docs/mcp-setup.md`

**Files:** All 4 pages under `docs/ai-agents/`

- [ ] **Step 1: Write `docs/ai-agents/index.md`**

Expand the content from `gohome/docs/mcp-setup.md` (which you've already read). Include all three client setups (Claude Code, Claude Desktop, Cursor), the full troubleshooting section, and the security model note.

```markdown
---
icon: lucide/bot
---

# AI Agents & MCP

gohome ships an MCP (Model Context Protocol) server out of the box. Any MCP-compatible AI client — Claude Code, Claude Desktop, Cursor, or your own agent — can control your home.

## Setup

### Claude Code

```bash
claude mcp add gohome -- gohome mcp serve
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "gohome": {
      "command": "gohome",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gohome": {
      "command": "gohome",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Security model

The MCP server connects to `gohomed` over a Unix-domain socket. Only processes on the same machine can reach it. The MCP server runs as the same user as the `gohome` CLI; daemon-side capability enforcement is independent of the MCP layer.

Token-based auth and network-accessible MCP will be added in a future release.

[Continue with troubleshooting section from mcp-setup.md]
```

- [ ] **Step 2: Write `docs/ai-agents/tool-catalog.md`**

Full reference table. For each tool: name, verb (READ/CALL/ADMIN), description, key input parameters, output shape, and a minimal example invocation. Include all 12 tools from the existing `mcp-setup.md` plus any additions from the C8 spec.

| Tool | Verb | Description |
|---|---|---|
| `gohome__get_state` | READ | Get current state of one entity |
| `gohome__list_entities` | READ | Browse with filters: area, zone, class, device |
| `gohome__call_capability` | CALL | Invoke a capability (turn on, set brightness, etc.) |
| `gohome__query_events` | READ | Query event log with cursor-based pagination |
| `gohome__tail_events` | READ | Stream recent events with configurable deadline |
| `gohome__apply_scene` | CALL | Apply a named scene |
| `gohome__run_script` | CALL | Run a named Starlark script |
| `gohome__eval_starlark` | CALL | Evaluate a Starlark expression (output capped 64 KiB) |
| `gohome__validate_config` | READ | Validate a Pkl bundle without applying |
| `gohome__apply_config` | ADMIN | Apply a Pkl bundle to the running daemon |
| `gohome__read_config_file` | READ | Read a file from the config directory |
| `gohome__write_config_file` | ADMIN | Write a file to the config directory (syntax-checked) |

For each tool, show the full input schema (parameters, types, optional/required) and a one-line example.

- [ ] **Step 3: Write `docs/ai-agents/resources.md`**

Cover the three MCP resource subscriptions with URI patterns, description, and example subscription output:

| URI pattern | Description |
|---|---|
| `gohome://entities/` | All entities (list), updated on any state change |
| `gohome://entities/{id}` | Single entity by ID |
| `gohome://automations/{automation_id}/runs/{run_id}/trace` | Automation run trace events |

- [ ] **Step 4: Write `docs/ai-agents/workflows.md`**

Three worked end-to-end examples with step-by-step tool calls:

**Example 1: Create an automation** — "Turn on garage lights when my car arrives home after sunset." Show the full sequence: `gohome__list_entities` (filter for device_tracker and light), draft Pkl snippet, `gohome__validate_config`, user approves, `gohome__apply_config`. This is the master-design §7.5 example.

**Example 2: Debug a missing event** — "Why didn't my kitchen lights turn off at midnight?" Show: `gohome__query_events` with entity and time filter, inspect the automation's `AutomationFinished` event, check `AutomationTriggered` → `AutomationFinished` correlation, identify that the condition failed.

**Example 3: Add a driver instance** — "Add my second Hue bridge." Show: `gohome__read_config_file` on `drivers.pkl`, draft the new instance block, `gohome__write_config_file`, `gohome__validate_config`, `gohome__apply_config`.

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/ai-agents/
git commit -m "docs: add AI agents & MCP section (setup, tool catalog, resources, workflows)"
```

---

### Task 10: API Reference — Pkl Modules + CLI

**Source:** `gohome/internal/config/pkl/gohome/` (all .pkl files); `gohome/internal/cli/` (all cmd_*.go and root.go)

**Files:** `docs/api-reference/pkl-modules.md`, `docs/api-reference/cli.md`

- [ ] **Step 1: Read all Pkl modules**

```bash
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/base.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/entities.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/automations.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/auth.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/carport.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/config.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/dashboards.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/scripts.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/starlark.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/widgets.pkl
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/config/pkl/gohome/mcp.pkl
```

- [ ] **Step 2: Write `docs/api-reference/pkl-modules.md`**

One section per module. For each class and typealias in the module: name, fields with types and defaults, constraints, and a usage example. Organize by module:

`## gohome.base` — `Secret`, `EnvSecret`, `FileSecret`, `KeyringSecret` typealiases (with regex patterns); `Metadata` class; `RetentionPolicy` class.

`## gohome.carport` — `DriverManifest` class fields; base `DriverInstance` class.

`## gohome.entities` — standard entity classes with their typed state and attribute fields. Include `ComputedEntity`.

`## gohome.automations` — all trigger, condition, and action classes (draw from the `automations.pkl` you've read); `Automation` class with all fields.

`## gohome.scripts` — Script declaration class.

`## gohome.dashboards` — `Dashboard`, `Page`, `Grid`, `WidgetInstance` classes.

`## gohome.widgets` — standard widget class definitions.

`## gohome.auth` — `User`, `Role`, `Policy`, `CapabilityAllow`, `CapabilityDeny`, `EntitySelector` classes.

`## gohome.starlark` — `StarlarkExpr`, `StarlarkScript`, `StarlarkCondition` typealiases; validation behaviour.

`## gohome.mcp` — MCP server config class.

- [ ] **Step 3: Read CLI command files**

```bash
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/cmd_automation.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/events.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/state.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/config.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/driver.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/eval.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/cmd_mcp.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/cmd_script.go
cat /Users/fdatoo/Desktop/GoHome/gohome/internal/cli/cmd_system.go
```

- [ ] **Step 4: Write `docs/api-reference/cli.md`**

Full reference for all `gohome` subcommands. Organize as a reference table first, then one subsection per top-level command. For each command: synopsis, flags, description, and an example.

Top-level commands from `root.go`: `version`, `system`, `events`, `state`, `registry`, `snapshot`, `driver`, `command`, `config`, `eval`, `test`, `automation`, `script`, `mcp`.

Global flags (from `root.go`):
- `--data-dir` (default: `~/.local/share/gohome`)
- `--format` (auto|table|json|yaml)
- `--no-color`
- `--log-level` (error|warn|info|debug)
- `--verbose` / `-v`
- `--endpoint` (unix:///path or tcp://host:port)

For each subcommand group, read the source file and document all subcommands with accurate flag names and descriptions.

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/api-reference/pkl-modules.md docs/api-reference/cli.md
git commit -m "docs: add API reference — Pkl modules and CLI"
```

---

### Task 11: API Reference — Connect-RPC + Event Types

**Source:** Child spec `2026-04-23-c7-connect-rpc-api-design.md`; `gohome/proto/gohome/v1alpha1/*.proto`; `gohome/proto/gohome/event/v1/event.proto`

**Files:** `docs/api-reference/connect-rpc.md`, `docs/api-reference/event-types.md`

- [ ] **Step 1: Read the API spec and proto files**

```bash
cat /Users/fdatoo/Desktop/GoHome/docs/superpowers/specs/2026-04-23-c7-connect-rpc-api-design.md
ls /Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/v1alpha1/
```

Read a few key proto files to get accurate RPC signatures:

```bash
cat /Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/v1alpha1/entity.proto
cat /Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/v1alpha1/event.proto
cat /Users/fdatoo/Desktop/GoHome/gohome/proto/gohome/v1alpha1/config.proto
```

- [ ] **Step 2: Write `docs/api-reference/connect-rpc.md`**

All 13 services. For each service: a brief description, then a table of RPCs with request type, response type, streaming flag, and description.

Services (from master design §7.1): `EntityService`, `DeviceService`, `AreaService`, `ZoneService`, `DriverService`, `EventService`, `SceneService`, `AutomationService`, `ScriptService`, `ConfigService`, `DashboardService`, `AuthService`, `SystemService`.

Include:
- How to connect (Connect-RPC over HTTP/2; browsers via SSE)
- Generated clients (Go via `gohomev1alpha1connect`, TypeScript via Connect-ES)
- Error taxonomy
- Versioning policy (current: `v1alpha1`; promotes to `v1` when backward-compat is committed)
- Pagination (cursor-based for list RPCs)
- Streaming (server-streaming for Subscribe/Tail RPCs)

- [ ] **Step 3: Write `docs/api-reference/event-types.md`**

Draw from `gohome/proto/gohome/event/v1/event.proto` (which you've read). One section per event type with fields, when it's emitted, and what it means for automation authors and MCP users.

Include the full `Payload.kind` oneof enumeration as an index table:

| Field | Type | Emitted when |
|---|---|---|
| `system` | `SystemEvent` | Daemon startup, shutdown, driver crash |
| `state_changed` | `StateChanged` | An entity's state or attributes changed |
| `command_issued` | `CommandIssued` | A capability was invoked on an entity |
| `command_ack` | `CommandAck` | Driver acknowledged or failed a command |
| `entity_registered` | `EntityRegistered` | A driver registered a new entity |
| `entity_unregistered` | `EntityUnregistered` | A driver removed an entity |
| `driver_event` | `DriverEvent` | Driver lifecycle: started, stopped, failed, heartbeat |
| `config_applied` | `ConfigApplied` | Config was validated and applied |
| `automation_triggered` | `AutomationTriggered` | An automation's trigger fired |
| `automation_finished` | `AutomationFinished` | An automation run completed (any outcome) |
| `script_invoked` | `ScriptInvoked` | A script was called |
| `script_finished` | `ScriptFinished` | A script run completed |
| `webhook_received` | `WebhookReceived` | A webhook was received |
| `mcp_eval_requested` | `MCPEvalRequested` | An MCP eval_starlark call was made |
| `config_file_edited` | `ConfigFileEdited` | A config file was written via MCP |
| `device_renamed` | `DeviceRenamed` | A device's friendly name was changed |
| `device_reassigned` | `DeviceReassigned` | A device was moved to a different area |
| `driver_instance_restarted` | `DriverInstanceRestarted` | A driver instance was restarted |

For each type, show the proto message fields and document when each field is populated.

Also document the `RunOutcome` enum values: `OUTCOME_OK`, `OUTCOME_CONDITION_FAIL`, `OUTCOME_ACTION_ERROR`, `OUTCOME_LIMIT_EXCEEDED`, `OUTCOME_CANCELLED`, `OUTCOME_SKIPPED`.

- [ ] **Step 4: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/api-reference/connect-rpc.md docs/api-reference/event-types.md
git commit -m "docs: add API reference — Connect-RPC services and event types"
```

---

### Task 12: Operations Section

**Source:** Master design §8; child spec `c13` (if written); `gohome/internal/observability/`

**Files:** All 5 pages under `docs/operations/`

- [ ] **Step 1: Write `docs/operations/index.md`**

Brief overview linking to the four sub-pages. Include a "persistent state inventory": config dir (Pkl) + SQLite DB + driver binaries = everything you need to backup and restore.

- [ ] **Step 2: Write `docs/operations/deployment.md`**

Cover:
- Default ports: `8080` (Connect-RPC / web UI), `9090` (Prometheus metrics, if enabled)
- Data directory layout: `gohomed.sock` (Unix socket), `events.db` (SQLite), `gohomed.lock` (lock file), `drivers/` (driver binaries), `snapshots/`
- Environment variables (full table — same as Docker page plus any additional)
- Lock file behavior: prevents multiple daemon instances; stale lock detection
- `gohomed --config-dir` and `--data-dir` flags

- [ ] **Step 3: Write `docs/operations/backup-restore.md`**

```markdown
# Backup & Restore

## What to back up

| Component | Where | Backup method |
|---|---|---|
| Config (Pkl) | `$GOHOME_CONFIG_DIR` | git (recommended) or file copy |
| Event database | `$GOHOME_DATA_DIR/events.db` | `gohome backup` |
| Driver binaries | `$GOHOME_DATA_DIR/drivers/` | Reinstallable — no backup needed |

## Backing up the database

```bash
# Create a backup (online, no downtime)
gohome backup --output /backup/gohome-$(date +%Y%m%d).db

# With encryption (AES-256, key from env)
gohome backup --output /backup/gohome.db.enc --encrypt env:GOHOME_BACKUP_KEY
```

`gohome backup` uses the SQLite online backup API — it creates a consistent copy while the daemon continues running.

## Restoring

```bash
# Stop the daemon
sudo systemctl stop gohomed

# Restore the database
gohome restore --from /backup/gohome-20260427.db --data-dir /var/lib/gohome

# Restart
sudo systemctl start gohomed
```

## Moving to a new server

```bash
# On old server
gohome backup --output gohome-backup.db
tar czf gohome-config.tar.gz ~/.config/gohome/

# On new server — install gohomed, then:
tar xzf gohome-config.tar.gz -C ~/.config/
gohome restore --from gohome-backup.db
gohome driver install hue zigbee2mqtt   # reinstall drivers
sudo systemctl start gohomed
```

## Auto-commit config

```bash
# Emit a git commit in the config directory after every ConfigApplied event
gohome config auto-commit --enable
```
```

- [ ] **Step 4: Write `docs/operations/updates.md`**

Cover all update paths: OCI tag bump (`docker compose pull && docker compose up -d`), apt/brew (`apt upgrade gohomed`), `gohome self-update`. Schema migration behavior (golang-migrate runs at startup; automatic pre-migration DB copy). Event schema backward compatibility (old events remain valid; new kinds are additive). Driver updates (independent, no daemon restart). Pkl module version pinning.

- [ ] **Step 5: Write `docs/operations/observability.md`**

```markdown
# Observability

## Structured logs

gohome uses Go's `slog` with JSON output by default.

```bash
# Change log level at runtime
gohome system log-level set debug

# Or set at startup
gohomed --log-level debug
```

Log format: `{"time":"...","level":"INFO","msg":"...","module":"carport","driver":"hue_main"}`

## Prometheus metrics

Expose metrics at `:9090/metrics` (disabled by default):

```pkl
// In main.pkl
metrics = new gohome.Metrics {
  enabled = true
  listenAddr = ":9090"
}
```

Key metrics:

| Metric | Type | Description |
|---|---|---|
| `gohome_events_appended_total` | Counter | Events appended to the log |
| `gohome_commands_dispatched_total` | Counter | Commands sent to drivers, by entity |
| `gohome_driver_restarts_total` | Counter | Driver process restarts, by instance |
| `gohome_automations_fired_total` | Counter | Automation runs by id and outcome |
| `gohome_api_request_duration_seconds` | Histogram | Connect-RPC handler latency |
| `gohome_eventstore_size_bytes` | Gauge | SQLite database file size |

## OpenTelemetry tracing

Export traces to any OTLP-compatible backend:

```pkl
tracing = new gohome.Tracing {
  enabled = true
  otlpEndpoint = "http://jaeger:4318"
}
```

Traces span the full request path: API call → event append → state update → driver dispatch.

## Diagnostics bundle

Produce a redacted support bundle safe to share:

```bash
gohome diag --output gohome-diag.tar.gz
```

Contains: versions, driver versions, recent errors, health snapshots, last 100 log lines. No event data, no secrets.
```

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/operations/
git commit -m "docs: add operations section (deployment, backup/restore, updates, observability)"
```

---

### Task 13: Migration from HA Section

**Source:** Child spec `2026-04-26-c11-ha-import-tool-design.md`; master design §8.1

**Files:** All 4 pages under `docs/migration/`

- [ ] **Step 1: Read the HA import spec**

```bash
cat /Users/fdatoo/Desktop/GoHome/docs/superpowers/specs/2026-04-26-c11-ha-import-tool-design.md
```

- [ ] **Step 2: Write `docs/migration/index.md`**

!!! status-wip badge.

Overview of `gohome import-ha`. What it does (reads HA config directory, produces gohome Pkl tree). What it doesn't do (migrate recorder DB, HACS integrations, supervisor add-ons). Point readers to `vs-home-assistant.md` first. The command:

```bash
gohome import-ha ~/.homeassistant --output ./my-gohome-config
cd my-gohome-config
gohome config validate
```

The output is a git-initializable directory the user reviews before pointing gohomed at it.

- [ ] **Step 3: Write `docs/migration/what-transfers.md`**

The full mapping table from master design §8.1 plus any additions from the C11 spec. Columns: HA construct, gohome target, confidence (High/Medium/Low), notes.

Include the v1.0 integration coverage list: MQTT, Zigbee2MQTT, ESPHome, HomeKit, Matter, Hue, Nest, Z-Wave JS, generic REST, generic webhook, `template` platform → ComputedEntity. Other integrations produce `# TODO: integration 'x' not yet mapped` stubs.

- [ ] **Step 4: Write `docs/migration/jinja-to-starlark.md`**

Transpiler rules. Table of HA Jinja patterns and their Starlark equivalents:

| Jinja | Starlark |
|---|---|
| `states('light.kitchen')` | `state("light.kitchen").state` |
| `state_attr('light.kitchen', 'brightness')` | `state("light.kitchen").attributes["brightness"]` |
| `is_state('switch.pump', 'on')` | `state("switch.pump").state == "on"` |
| `now()` | `now()` |
| `trigger.to_state.state` | `event.to_state` (in StateChangeTrigger context) |

What emits `# FIXME: unmapped Jinja: <original>` — complex filters, custom Jinja helpers, `{% set %}` blocks with side-effects. How to handle each manually.

- [ ] **Step 5: Write `docs/migration/post-migration.md`**

```markdown
# Post-migration checklist

After running `gohome import-ha`, work through this checklist before starting the daemon.

- [ ] Run `gohome config validate` and resolve all errors
- [ ] Search for `# TODO:` in the output — unmapped integrations need manual driver config
- [ ] Search for `# FIXME:` in `.star` files — unmapped Jinja needs manual Starlark rewrite
- [ ] Review `auth/users.pkl` — passwords are not migrated; users must re-register passkeys after first login
- [ ] Verify each driver instance config (bridge addresses, API keys, credentials)
- [ ] Review computed entities (`entities/computed.pkl`) for correctness
- [ ] Check automations for logic correctness — the transpiler handles syntax but not semantics
- [ ] Review scene definitions
- [ ] Check dashboard widget mappings — common cards translate; custom cards are marked TODO
- [ ] Start the daemon and check `gohome driver status` for each instance
- [ ] Trigger a few automations manually with `gohome automation trigger <id>` to verify
- [ ] Check `gohome events tail` for any errors during the first few minutes
```

- [ ] **Step 6: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/migration/
git commit -m "docs: add migration section (import-ha overview, what transfers, Jinja→Starlark, checklist)"
```

---

### Task 14: Edge Agents Section

**Source:** Child spec `2026-04-27-c12-edge-agent-design.md`

**Files:** All 3 pages under `docs/edge-agents/`

- [ ] **Step 1: Read the edge agent spec**

```bash
cat /Users/fdatoo/Desktop/GoHome/docs/superpowers/specs/2026-04-27-c12-edge-agent-design.md
```

- [ ] **Step 2: Write `docs/edge-agents/index.md`**

!!! status-wip badge.

What an edge agent is: `gohome-edge` runs on a remote host, hosts drivers, forwards Carport over TLS back to the primary daemon. When to use it: remote Z-Wave radio, garage Raspberry Pi, basement network switch. Topology diagram (text or Mermaid). The key design point: same Carport protocol, different transport (TLS instead of Unix socket).

- [ ] **Step 3: Write `docs/edge-agents/pairing.md`**

The mTLS pairing flow:
1. On the primary: `gohome edge pair --name garage-pi` — generates a pairing token
2. On the edge host: `gohome-edge start --pair-token <token> --primary tcp://gohomed.local:8443`
3. gohomed issues a CA certificate to the edge agent; the edge agent presents it on every future connection
4. After pairing, configure driver instances in `drivers.pkl` with `transport = "edge:garage-pi"`

Show the complete command sequence and the resulting `drivers.pkl` entry.

- [ ] **Step 4: Write `docs/edge-agents/resilience.md`**

Cover: local event buffering during disconnect (events held in edge agent's SQLite); reconnection with exponential backoff; cursor-based replay ensures no events are lost; multi-edge scenarios (each edge agent has its own cursor); offline-mode failover (drivers on edge continue operating; commands from the primary are queued).

- [ ] **Step 5: Commit**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/edge-agents/
git commit -m "docs: add edge agents section (overview, pairing, resilience)"
```

---

### Task 15: Contributing Section + Landing Page + Final Wiring

**Files:** `docs/contributing/index.md`, `dev-setup.md`, `architecture-internals.md`, `docs/index.md`

**Also:** Update `gohome/README.md` to link to the docs site.

- [ ] **Step 1: Write `docs/contributing/index.md`**

```markdown
---
icon: lucide/git-pull-request
---

# Contributing

Contributions are welcome — drivers, Pkl modules, bug fixes, documentation, and tooling.

## Types of contributions

- **Drivers** — new first-party or community drivers that speak Carport
- **Pkl modules** — entity classes, widget packs, utility modules
- **Core bug fixes** — against the `gohome` repo
- **Documentation** — against this `gohome-docs` repo
- **Tooling** — CLI improvements, build tooling, CI

## Before you start

Open an issue or discussion before starting large changes. The design specs in `docs/superpowers/specs/` (in the docs submodule) describe the intended architecture — diverging from them needs a design discussion.

## PR process

1. Fork the relevant repo
2. Create a feature branch
3. Ensure `task test` passes
4. Ensure `task lint` passes
5. Open a PR against `main`

Code review typically happens within a few days.
```

- [ ] **Step 2: Write `docs/contributing/dev-setup.md`**

Prerequisites and their install commands. Draw from `gohome/Taskfile.yml` for the accurate task names.

```bash
# Prerequisites
go install go.dev  # Go 1.22+
brew install pkl   # Pkl CLI (macOS) — or download from pkl-lang.org
brew install bufbuild/buf/buf  # buf (protobuf toolchain)
brew install go-task/tap/go-task  # Task (Taskfile runner)

# Clone
git clone https://github.com/fynn-labs/gohome
cd gohome

# Install git hooks
./scripts/install-hooks.sh

# Build
task build

# Run tests
task test

# Run linter
task lint

# Run the daemon locally with test config
gohomed --config-dir examples/ --data-dir /tmp/gohome-dev --log-level debug
```

Read `gohome/Taskfile.yml` to get accurate task names and commands.

- [ ] **Step 3: Write `docs/contributing/architecture-internals.md`**

Deep reference for contributors. Draw from master design §3.3 and §5.9.

Key sections:
- `## Module boundaries` — why each internal package has a narrow interface; the `eventstore` as the sole gatekeeper; the `carport-host` abstraction
- `## The single-writer invariant` — only `eventstore.Append` writes to the events table; why this matters
- `## Concurrency invariants` — the three named invariants from master design §5.9
- `## The Carport FSM` — the driver lifecycle state machine (reference `drivers/building/lifecycle.md` for the diagram)
- `## Config diff-based reload` — how `config.Diff` computes minimal side-effects; why unchanged driver instances are not restarted
- `## Design specs` — point to each child spec file (C1–C12) for deep dives on each subsystem

- [ ] **Step 4: Write the landing page `docs/index.md`**

Replace the Zensical placeholder:

```markdown
---
icon: lucide/home
---

# gohome

**Go-native home automation.** Event-sourced, typed, agent-friendly.

Built for the prosumer and homelab audience — comfortable in a config file, running Docker or bare metal, sometimes managing things through an AI agent.

---

!!! tip "New to gohome?"
    Start with [Installation](installation/index.md) and the [First run](installation/first-run.md) guide.

!!! note "Migrating from Home Assistant?"
    Read [gohome vs. Home Assistant](introduction/vs-home-assistant.md) first, then the [Migration guide](migration/index.md).

!!! info "Building a driver?"
    Jump to [Building drivers](drivers/building/index.md) for the Carport SDK.

---

## Why gohome?

| | Home Assistant | gohome |
|---|---|---|
| Config | YAML + Jinja | Pkl (typed, validated) |
| Automation logic | Jinja templates | Starlark (real language) |
| Entity state | Always a string | Typed (bool, int, float, …) |
| History | Recorder (lossy) | Event log (full-fidelity) |
| AI integration | Add-on | First-class MCP server |
| Driver model | Integration = code + config | Driver (code) + instance (config) |

## Get started

<div class="grid cards" markdown>

- :material-download: **[Install](installation/index.md)** — binary, Docker, or systemd
- :material-transfer: **[Migrate from HA](migration/index.md)** — import your existing setup
- :material-robot: **[Connect AI](ai-agents/index.md)** — wire up Claude or another MCP client
- :material-plug: **[Build a driver](drivers/building/index.md)** — extend gohome with Carport

</div>
```

- [ ] **Step 5: Verify full site builds**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
zensical build 2>&1 | grep -E "(ERROR|WARNING|error|warning)"
```

Expected: no errors. Warnings about missing pages should all be resolved at this point.

- [ ] **Step 6: Verify navigation and spot-check pages**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
zensical serve &
# Open http://localhost:8000 and verify:
# - Navigation renders all sections
# - Landing page cards display correctly
# - Status badges render with correct colors
# - Mermaid diagrams render in architecture page
# - Code blocks have copy buttons
# - Dark/light mode toggle works
```

- [ ] **Step 7: Update `gohome/README.md`**

Replace the existing "Status" section and the `docs/mcp-setup.md` link with a pointer to the hosted docs site:

```markdown
## Documentation

Full documentation at **https://gohome.dev/docs** (or run `zensical serve` in the `gohome-docs` repo for local preview).

- [Installation](https://gohome.dev/docs/installation/)
- [Configuration](https://gohome.dev/docs/configuration/)
- [MCP / AI Agents](https://gohome.dev/docs/ai-agents/)
```

This modifies `/Users/fdatoo/Desktop/GoHome/gohome/README.md`.

- [ ] **Step 8: Commit docs site**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome-docs
git add docs/contributing/ docs/index.md
git commit -m "docs: add contributing section and landing page, complete site"
```

- [ ] **Step 9: Commit gohome README update**

```bash
cd /Users/fdatoo/Desktop/GoHome/gohome
git add README.md
git commit -m "docs: link to gohome-docs site, supersede inline mcp-setup"
```

---

## Self-Review

**Spec coverage check:**

| Design doc section | Covered by task |
|---|---|
| §2 Site config + nav | Task 1 |
| §4.1 Landing page | Task 15 |
| §4.2 Introduction (4 pages) | Task 2 |
| §4.3 Installation (5 pages) | Task 3 |
| §4.4 Concepts (4 pages) | Task 4 |
| §4.5 Configuration (8 pages) | Task 5 |
| §4.6 Automations (7 pages) | Task 6 |
| §4.7 Drivers using (2 pages) | Task 7 |
| §4.7 Drivers building (5 pages) | Task 8 |
| §4.8 AI Agents (4 pages) | Task 9 |
| §4.9 API Reference — Pkl + CLI | Task 10 |
| §4.9 API Reference — RPC + Events | Task 11 |
| §4.10 Operations (5 pages) | Task 12 |
| §4.11 Migration (4 pages) | Task 13 |
| §4.12 Edge Agents (3 pages) | Task 14 |
| §4.13 Contributing (3 pages) | Task 15 |
| Status badge CSS | Task 1 |
| `markdown.md` deletion | Task 1 |
| `gohome/README.md` update | Task 15 |

All 52 pages accounted for. No gaps.

**Placeholder scan:** No TBDs or TODOs in the plan itself. Every step that writes content shows either complete Markdown or an explicit source file to read + section outline to follow.

**Type consistency:** All Pkl class names used in doc examples match the actual source: `StateChangeTrigger`, `CallServiceAction`, `StarlarkCondition`, `EnvSecret`, etc.
