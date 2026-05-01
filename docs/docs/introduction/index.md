# What is switchyard

!!! status-alpha "Alpha — shipped, interface evolving"
    switchyard is in early development. Core functionality works; APIs and config schema may change between releases.

**switchyard** is a Go-native home automation daemon built for prosumer and homelab operators. It positions itself as an opinionated replacement for Home Assistant, not a clone of it. The differences are architectural, not cosmetic.

## Three bets

switchyard is built on three design decisions that set it apart from existing home automation platforms.

### 1. Event sourcing as the architectural spine

The event log is the single source of truth. Current entity state is a materialized view derived from it, not a free-standing record. Every state change, command issued, automation triggered, and config applied is an immutable event appended to a SQLite database.

The practical consequences are significant: you get time-travel debugging (replay state to any past moment), a complete audit log at zero extra cost, clean replication for edge agents, and a uniform subscription model used by every internal consumer — the state cache, the automation engine, the UI, and MCP clients all tail the same event stream.

### 2. Declarative config (Pkl) with sandboxed dynamic logic (Starlark)

Configuration lives in [Pkl](https://pkl-lang.org/), a typed, composable configuration language. Automations and computed entities use Starlark, a deterministic subset of Python designed for embedding. The two are distinct by design: Pkl handles structure and types, Starlark handles logic.

This replaces the YAML + Jinja approach common in Home Assistant, where templating is bolted onto a format not designed for it. In switchyard, a bad config fails at `switchyard config validate` — before the daemon touches the network. Starlark scripts are sandboxed, resource-limited, and fully testable.

### 3. Agent-native from day one

The MCP (Model Context Protocol) server is a first-class API surface, not an afterthought. It exposes tools for reading state, calling entity capabilities, querying the event log, and editing Pkl config with validation feedback — out of the box, secured by the same policy system as human users.

An AI agent with a switchyard token can inspect your home's state, write an automation, validate it, and apply it — with a human in the loop at each step if you configure it that way.

## Target audience

switchyard is for **prosumer and homelab users** who are comfortable editing a config file, running Docker or bare-metal Linux, and managing infrastructure with some care. You may have a family on the home network, use AI tools in your workflow, or want your home automation to behave more like a production system than a hobby project.

switchyard is not designed to be plug-and-play for non-technical users. It does not have a first-run wizard that discovers devices automatically and configures itself. Configuration is explicit and version-controlled.

## What switchyard ships

Three binaries:

- **`switchyardd`** — the daemon. Runs on your server. Contains everything: event store, driver manager, automation engine, Connect-RPC API, MCP server, and an embedded web UI.
- **`switchyard`** — the CLI. A thin client for operators. Used for config management, diagnostics, and interacting with a running daemon.
- **`switchyard-edge`** — an optional edge agent. Runs on remote hosts (a Raspberry Pi near your Z-Wave controller, for example) and proxies drivers back to the primary daemon over mTLS.

Drivers are separate binaries that speak the Carport gRPC protocol (switchyard's internal driver IPC protocol). They are not bundled into `switchyardd`; they are installed explicitly and communicate over Unix domain sockets (local) or TLS (edge).

## What switchyard does not do

- **No HA API compatibility shim.** Home Assistant integrations, apps, and the HA mobile app will not talk to switchyard without modification. This is an explicit non-goal for v1.0.
- **No high-availability clustering.** Single primary only. Edge agents extend reach, not redundancy.
- **No appliance OS.** There is no switchyardOS analogous to HAOS. switchyard runs on whatever Linux you have.
- **No commercial plugin marketplace.** Drivers are distributed via a signed directory, not a store.
- **No native mobile apps.** The web UI is a PWA and works on mobile; native wrappers are deferred.
- **No HACS-style third-party ecosystem.** Widget packs exist but are nascent — not a replacement for HACS's breadth.

If you need those things today, Home Assistant may be the better choice. switchyard is for users who value the architectural tradeoffs it makes over the breadth of its current ecosystem.
