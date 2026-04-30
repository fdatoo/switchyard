# gohome MCP Setup

gohome ships a stdio-transport MCP server. Connect any MCP-compatible AI client to control your home.

## Setup

### Claude Code (CLI)

```sh
claude mcp add gohome -- gohome mcp serve
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or equivalent:

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

Add to `.cursor/mcp.json` in your project root (or `~/.cursor/mcp.json` globally):

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

## Tool catalog

List all tools without a daemon:

```sh
gohome mcp tools
gohome mcp tools --json
```

| Tool | Verb | Summary |
|------|------|---------|
| `gohome__get_state` | READ | Get current state of one entity |
| `gohome__list_entities` | READ | Browse entities with optional filters (area, zone, class, device) |
| `gohome__call_capability` | CALL | Invoke a capability on one entity (turn on, set brightness, etc.) |
| `gohome__query_events` | READ | Query the event log with cursor-based pagination |
| `gohome__tail_events` | READ | Stream recent events with a configurable deadline |
| `gohome__apply_scene` | CALL | Apply a named scene *(not yet implemented)* |
| `gohome__run_script` | CALL | Run a named Starlark script |
| `gohome__eval_starlark` | CALL | Evaluate a Starlark expression (output capped at 64 KiB) |
| `gohome__validate_config` | READ | Validate a Pkl config bundle without applying |
| `gohome__apply_config` | ADMIN | Apply a Pkl config bundle to the running daemon |
| `gohome__read_config_file` | READ | Read a file from the config directory |
| `gohome__write_config_file` | ADMIN | Write a file to the config directory (syntax-checked) |

## Resource subscriptions

Subscribe to live updates:

| URI pattern | Description |
|-------------|-------------|
| `gohome://entities/` | All entities (list) |
| `gohome://entities/{id}` | Single entity by ID |
| `gohome://automations/{automation_id}/runs/{run_id}/trace` | Automation run trace events |

## Security notes

- **Local-only (C8):** The MCP server connects to `gohomed` over a Unix-domain socket. Only processes on the same machine can reach it. Network exposure and token-based auth are planned for C9.
- The MCP server runs as the same user as the `gohome` CLI. Daemon-side capabilities are enforced by the daemon regardless of the MCP layer.
- `gohome__apply_config` and `gohome__write_config_file` are ADMIN-class tools — they mutate persistent state. Future releases will allow disabling individual tools via config.

## Troubleshooting

### `gohomed not running` or connection refused

The MCP server requires `gohomed` to be running. Start it:

```sh
gohomed --data-dir ~/.local/share/gohome
```

Or check its status:

```sh
gohome status
```

### Check what's happening

Tail the event log to see what the daemon is processing:

```sh
gohome events tail
```

### Find the daemon socket

The default socket path is `~/.local/share/gohome/gohomed.sock`. Override with:

```sh
gohome mcp serve --endpoint unix:///path/to/gohomed.sock
```

Or set `GOHOME_ENDPOINT=unix:///path/to/gohomed.sock` in your environment.
