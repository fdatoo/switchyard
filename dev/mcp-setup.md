# switchyard MCP Setup

switchyard ships a stdio-transport MCP server. Connect any MCP-compatible AI client to control your home.

## Setup

### Claude Code (CLI)

```sh
claude mcp add switchyard -- switchyard mcp serve
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or equivalent:

```json
{
  "mcpServers": {
    "switchyard": {
      "command": "switchyard",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json` in your project root (or `~/.cursor/mcp.json` globally):

```json
{
  "mcpServers": {
    "switchyard": {
      "command": "switchyard",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Tool catalog

List all tools without a daemon:

```sh
switchyard mcp tools
switchyard mcp tools --json
```

| Tool | Verb | Summary |
|------|------|---------|
| `switchyard__get_state` | READ | Get current state of one entity |
| `switchyard__list_entities` | READ | Browse entities with optional filters (area, zone, class, device) |
| `switchyard__call_capability` | CALL | Invoke a capability on one entity (turn on, set brightness, etc.) |
| `switchyard__query_events` | READ | Query the event log with cursor-based pagination |
| `switchyard__tail_events` | READ | Stream recent events with a configurable deadline |
| `switchyard__apply_scene` | CALL | Apply a named scene *(not yet implemented)* |
| `switchyard__run_script` | CALL | Run a named Starlark script |
| `switchyard__eval_starlark` | CALL | Evaluate a Starlark expression (output capped at 64 KiB) |
| `switchyard__validate_config` | READ | Validate a Pkl config bundle without applying |
| `switchyard__apply_config` | ADMIN | Apply a Pkl config bundle to the running daemon |
| `switchyard__read_config_file` | READ | Read a file from the config directory |
| `switchyard__write_config_file` | ADMIN | Write a file to the config directory (syntax-checked) |

## Resource subscriptions

Subscribe to live updates:

| URI pattern | Description |
|-------------|-------------|
| `switchyard://entities/` | All entities (list) |
| `switchyard://entities/{id}` | Single entity by ID |
| `switchyard://automations/{automation_id}/runs/{run_id}/trace` | Automation run trace events |

## Security notes

- **Local-only (C8):** The MCP server connects to `switchyardd` over a Unix-domain socket. Only processes on the same machine can reach it. Network exposure and token-based auth are planned for C9.
- The MCP server runs as the same user as the `switchyard` CLI. Daemon-side capabilities are enforced by the daemon regardless of the MCP layer.
- `switchyard__apply_config` and `switchyard__write_config_file` are ADMIN-class tools — they mutate persistent state. Future releases will allow disabling individual tools via config.

## Troubleshooting

### `switchyardd not running` or connection refused

The MCP server requires `switchyardd` to be running. Start it:

```sh
switchyardd --data-dir ~/.local/share/switchyard
```

Or check its status:

```sh
switchyard status
```

### Check what's happening

Tail the event log to see what the daemon is processing:

```sh
switchyard events tail
```

### Find the daemon socket

The default socket path is `~/.local/share/switchyard/switchyardd.sock`. Override with:

```sh
switchyard mcp serve --endpoint unix:///path/to/switchyardd.sock
```

Or set `SWITCHYARD_ENDPOINT=unix:///path/to/switchyardd.sock` in your environment.
