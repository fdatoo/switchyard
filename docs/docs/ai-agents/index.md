# AI Agents & MCP

!!! status-alpha "Alpha — MCP server shipped; token-based auth is in development (C9)"

switchyard ships a built-in [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server. Any MCP-compatible AI client — Claude Code, Claude Desktop, Cursor, or any other — can connect to it and control your home directly: querying entity state, invoking capabilities, reading and editing config, running scripts, and following live events.

---

## What MCP is and how switchyard uses it

MCP is a standard wire protocol that lets AI agents call tools and subscribe to data sources on a remote server. Tools are named functions with typed inputs and outputs. Resources are URIs the agent can read or subscribe to for live updates.

switchyard exposes:

- **12 MCP tools** covering entity state, capability invocation, event history, automations, scripts, config editing, and Starlark evaluation.
- **2 MCP resource types** for live entity state and automation run traces.

The MCP server runs as `switchyard mcp serve` — a subprocess launched by the MCP client. It speaks MCP JSON-RPC on stdin/stdout and dials your running `switchyardd` daemon over a local Unix-domain socket. There is no network listener and no credential to configure in v1.

```
MCP client (Claude Desktop, Cursor, ...)
    │  MCP JSON-RPC on stdin/stdout
    ▼
switchyard mcp serve  (subprocess)
    │  Connect-RPC over Unix socket
    ▼
switchyardd  (daemon)
```

---

## Prerequisites

`switchyardd` must be running before the MCP client launches `switchyard mcp serve`. If the daemon is not reachable, the subprocess exits immediately and the MCP client surfaces an error.

Start the daemon:

```sh
switchyardd --data-dir ~/.local/share/switchyard
```

Or check its status:

```sh
switchyard status
```

---

## Setup

### Claude Code

Run once from your terminal:

```sh
claude mcp add switchyard -- switchyard mcp serve
```

This writes the server entry to your Claude Code MCP config (`~/.claude/mcp-servers.json` or the project-local `.claude/mcp-servers.json`). Restart Claude Code and the `switchyard` server will be available.

To add it manually, edit `~/.claude/mcp-servers.json`:

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

### Claude Desktop

Edit the Claude Desktop config file:

- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Linux:** `~/.config/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

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

Restart Claude Desktop. The switchyard tools will appear in the tool palette.

### Cursor

Add to `.cursor/mcp.json` in your project root for a project-scoped server, or `~/.cursor/mcp.json` globally:

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

Restart Cursor. switchyard will appear in the MCP server list under **Settings → MCP**.

### Other clients

Any MCP client that supports stdio subprocess launch works. Point it at:

```
command: switchyard
args: ["mcp", "serve"]
```

---

## Verifying the setup

Lists all MCP tools exposed by the running switchyard daemon. Requires a running daemon to fetch current state for tool descriptions.

```sh
switchyard mcp tools
```

For machine-readable output:

```sh
switchyard mcp tools --json
```

---

## Security model

**Local-only Unix socket.** In v1, the MCP server connects to `switchyardd` exclusively over a Unix-domain socket. Only processes running on the same machine and as the same user can reach it. There is no network listener, no port to expose, and no firewall rule to configure.

**Do not expose the MCP socket over the network.** The MCP server's stdio transport is not designed for network access. Do not tunnel it with `socat`, `netcat`, or similar tools. Network-accessible MCP with bearer-token auth is planned for a future release (C9).

**`system:local` principal.** Because the subprocess dials a local UDS, the daemon treats it as the `system:local` principal — equivalent to the logged-in user running any other `switchyard` CLI command. All daemon-side enforcement applies normally.

**ADMIN-class tools.** `switchyard__apply_config` and `switchyard__write_config_file` are admin-verb tools — they mutate persistent state. They are subject to the same daemon-side policy as any other caller. Per-tool policy gating is planned for C9.

---

## Getting an API token (C9, coming soon)

!!! status-wip "In development"

Token-based auth is planned as part of C9. Once shipped, you will be able to create a named token scoped to MCP:

```sh
switchyard auth tokens create --name "claude" --scope mcp
```

The token will be passed to `switchyard mcp serve` via an environment variable and will enable the HTTP transport for remote MCP access. Until C9 ships, the stdio transport with `system:local` auth covers local agent use.

---

## Troubleshooting

### `switchyardd not running` or subprocess exits immediately

The MCP subprocess prints a message to stderr and exits if it cannot reach the daemon:

```
switchyard mcp: cannot reach switchyardd at unix://@data/switchyardd.sock: connection refused
```

Start the daemon and try again.

### The daemon socket path

The default socket is `~/.local/share/switchyard/switchyardd.sock`. To use a different path:

```sh
switchyard mcp serve --endpoint unix:///path/to/switchyardd.sock
```

Or set the environment variable:

```sh
export SWITCHYARD_ENDPOINT=unix:///path/to/switchyardd.sock
```

### Inspecting MCP activity

All MCP tool calls appear in the daemon event log with `source=mcp`. Tail the event log to see what the agent is doing:

```sh
switchyard events tail
```
