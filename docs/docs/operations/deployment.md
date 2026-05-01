# Deployment

!!! status-alpha "Alpha — shipped, interface evolving"
    The data directory layout, lock file, and listen address flags are stable. Environment variable aliases for all flags are designed but not yet wired in the current binary — use CLI flags directly for now.

## Default ports

| Service | Default | Notes |
|---|---|---|
| Connect-RPC API | `8080` | HTTP/2 with gRPC-compatible framing |
| Web UI | `8080` | Same port as the API; served under a different path prefix (`/app/`) |
| Admin (`/metrics`, `/health`) | `9190` | Separate HTTP port, not authenticated |
| MCP Unix socket | `$GOHOME_DATA/switchyardd.sock` | Local only; no TCP exposure by default |

The API and web UI share a single port. Route separation is by path: `/api/` routes to the Connect-RPC handlers; `/app/` routes to the embedded static web UI assets.

## Data directory layout

The data directory (`$GOHOME_DATA`) holds all runtime state written by the daemon:

```
$GOHOME_DATA/                         # default: ~/.local/share/switchyard/ (Linux)
│                                     #          ~/Library/Application Support/switchyard/ (macOS)
├── switchyard.db                         # SQLite event store — the source of truth
├── switchyardd.sock                      # Connect-RPC Unix domain socket (API + MCP)
├── switchyardd.lock                      # PID lock file — prevents double-start
└── drivers/                          # Downloaded driver binaries
```

The data directory is created on first start if it does not exist.

## Config directory

The config directory (`$GOHOME_CONFIG`) holds your Pkl source files — everything you edit:

```
$GOHOME_CONFIG/                       # default: ~/.config/switchyard/
├── main.pkl
├── drivers.pkl
├── areas.pkl
├── automations/
│   └── lights.pkl
└── ...
```

See [Config directory](../configuration/index.md) for the full layout and how `main.pkl` wires together the rest.

## Environment variables

These environment variables mirror the daemon's CLI flags. When both are set, the CLI flag takes precedence.

| Variable | Equivalent flag | Description |
|---|---|---|
| `GOHOME_DATA` | `--data-dir` | Path to the data directory |
| `GOHOME_CONFIG` | `--config-dir` | Path to the config directory (overrides the default inside `$GOHOME_DATA`) |
| `GOHOME_LISTEN` | `--listen` | Listen address for the main HTTP server (default: `:8080`) |
| `GOHOME_LOG_LEVEL` | `--log-level` | Log verbosity: `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `GOHOME_LOG_FORMAT` | `--log-format` | Log format: `auto`, `json`, or `tty` |
| `GOHOME_MCP_SOCKET` | `--mcp-socket` | Override the MCP Unix socket path |

The `switchyard` CLI uses `GOHOME_ENDPOINT` to locate the daemon:

```bash
# Connect to a remote daemon over TCP instead of the local Unix socket
export GOHOME_ENDPOINT="tcp://192.168.1.10:8080"
switchyard state list
```

Precedence for endpoint resolution: `--endpoint` flag → `GOHOME_ENDPOINT` env → `unix://$GOHOME_DATA/switchyardd.sock`.

## Lock file

`switchyardd.lock` in the data directory contains the PID of the running daemon. On startup:

1. switchyardd reads the lock file if it exists.
2. If a live process with that PID is found, switchyardd exits immediately with:
   ```
   error: switchyardd already running (pid 12345)
   ```
3. If the PID is stale (process no longer exists), the lock file is overwritten.
4. On clean shutdown, the lock file is removed.

This prevents two daemon instances from writing to the same SQLite database simultaneously, which would corrupt the event log.

If switchyardd was killed without a chance to clean up (power loss, `kill -9`), the lock file will be stale. You can remove it manually and restart:

```bash
rm "$GOHOME_DATA/switchyardd.lock"
switchyardd --data-dir "$GOHOME_DATA"
```

## Running as a service

See [systemd / packages](../installation/systemd.md) for the recommended service unit that handles restart-on-failure, data directory ownership, and the lock file lifecycle correctly.

For Docker deployments, see [Docker](../installation/docker.md).
