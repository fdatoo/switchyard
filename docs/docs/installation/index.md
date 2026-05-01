# Installation

!!! status-alpha "Alpha — shipped, interface evolving"
    switchyard is in early development. The binaries and container image are available; package distribution (.deb/.rpm) is still being finalized. APIs and config schema may change between releases.

switchyard ships as a single daemon binary (`switchyardd`) and a CLI client (`switchyard`). There is no runtime dependency — both are statically linked Go binaries. Pick an install method that fits your environment.

## Platform support matrix

| Platform | Static binary | Docker / OCI | `.deb` / `.rpm` | Homebrew |
|---|---|---|---|---|
| Linux x86-64 (amd64) | Yes | Yes | Yes (pending) | — |
| Linux arm64 | Yes | Yes | Yes (pending) | — |
| Linux armv7 | Yes | — | — | — |
| macOS arm64 (Apple Silicon) | Yes | — | — | Yes (pending) |
| macOS x86-64 | Yes | — | — | Yes (pending) |
| Windows x86-64 | Yes | — | — | — |

**Linux is the primary supported platform.** macOS and Windows binaries are provided for development and CLI use; running `switchyardd` as a persistent service on those platforms is possible but not documented as a production configuration.

## Choosing an install method

| Situation | Recommended method |
|---|---|
| You want the fastest path to a running daemon | [Static binary](binary.md) |
| You run Docker, Portainer, Unraid, or similar | [Docker](docker.md) |
| You run a Linux server and want the daemon managed as a service | [systemd / packages](systemd.md) |
| You're on macOS and just need the CLI (`switchyard`) | [Static binary](binary.md) or [Homebrew](systemd.md#homebrew) |

All methods produce the same daemon. After installation, continue to [First run](first-run.md) regardless of which method you used.

## What you get

After installation you will have:

- **`switchyardd`** — the daemon. Run this on your server. It contains the event store, driver manager, automation engine, Connect-RPC API, MCP server, and the embedded web UI.
- **`switchyard`** — the CLI. Run this anywhere you can reach the daemon. Used for config management, diagnostics, and ad-hoc commands.

[Edge agents (`switchyard-edge`)](../edge-agents/index.md) are a separate topic and not required for a basic install.

## Before you begin

You will need:

- A host to run `switchyardd` on (Linux recommended; Raspberry Pi 4 or newer works, a NAS or small server is typical)
- Approximately 512 MB RAM available for `switchyardd` at rest (more for large event replays)
- Persistent storage for the config directory and the event database — an SSD is preferred, though an SD card or spinning disk will work
- Outbound internet access during installation (for pulling artifacts); `switchyardd` itself does not require internet at runtime

## Next steps

1. Follow one of the install guides:
    - [Static binary](binary.md)
    - [Docker](docker.md)
    - [systemd / packages](systemd.md)
2. [First run](first-run.md) — create a config, validate it, and start the daemon
3. [Concepts](../concepts/index.md) — understand how switchyard models your home before writing config
