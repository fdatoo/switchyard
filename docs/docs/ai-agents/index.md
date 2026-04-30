# AI Agents & MCP

!!! status-alpha "Alpha — MCP server shipped; token-based auth is in development (C9)"

gohome ships a built-in [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server. Any MCP-compatible AI client — Claude Code, Claude Desktop, Cursor, or any other — can connect to it and control your home directly: querying entity state, invoking capabilities, reading and editing config, running scripts, and following live events.

---

## What MCP is and how gohome uses it

MCP is a standard wire protocol that lets AI agents call tools and subscribe to data sources on a remote server. Tools are named functions with typed inputs and outputs. Resources are URIs the agent can read or subscribe to for live updates.

gohome exposes:

- **12 MCP tools** covering entity state, capability invocation, event history, automations, scripts, config editing, and Starlark evaluation.
- **2 MCP resource types** for live entity state and automation run traces.

The MCP server runs as `gohome mcp serve` — a subprocess launched by the MCP client. It speaks MCP JSON-RPC on stdin/stdout and dials your running `gohomed` daemon over a local Unix-domain socket. There is no network listener and no credential to configure in v1.

```
MCP client (Claude Desktop, Cursor, ...)
    │  MCP JSON-RPC on stdin/stdout
    ▼
gohome mcp serve  (subprocess)
    │  Connect-RPC over Unix socket
    ▼
gohomed  (daemon)
```

---

## Prerequisites

`gohomed` must be running before the MCP client launches `gohome mcp serve`. If the daemon is not reachable, the subprocess exits immediately and the MCP client surfaces an error.

Start the daemon:

```sh
gohomed --data-dir ~/.local/share/gohome
```

Or check its status:

```sh
gohome status
```

---

## Setup

### Claude Code

Run once from your terminal:

```sh
claude mcp add gohome -- gohome mcp serve
```

This writes the server entry to your Claude Code MCP config (`~/.claude/mcp-servers.json` or the project-local `.claude/mcp-servers.json`). Restart Claude Code and the `gohome` server will be available.

To add it manually, edit `~/.claude/mcp-servers.json`:

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

### Claude Desktop

Edit the Claude Desktop config file:

- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Linux:** `~/.config/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

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

Restart Claude Desktop. The gohome tools will appear in the tool palette.

### Cursor

Add to `.cursor/mcp.json` in your project root for a project-scoped server, or `~/.cursor/mcp.json` globally:

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

Restart Cursor. gohome will appear in the MCP server list under **Settings → MCP**.

### Other clients

Any MCP client that supports stdio subprocess launch works. Point it at:

```
command: gohome
args: ["mcp", "serve"]
```

---

## Verifying the setup

Lists all MCP tools exposed by the running gohome daemon. Requires a running daemon to fetch current state for tool descriptions.

```sh
gohome mcp tools
```

For machine-readable output:

```sh
gohome mcp tools --json
```

---

## Security model

**Local-only Unix socket.** In v1, the MCP server connects to `gohomed` exclusively over a Unix-domain socket. Only processes running on the same machine and as the same user can reach it. There is no network listener, no port to expose, and no firewall rule to configure.

**Do not expose the MCP socket over the network.** The MCP server's stdio transport is not designed for network access. Do not tunnel it with `socat`, `netcat`, or similar tools. Network-accessible MCP with bearer-token auth is planned for a future release (C9).

**`system:local` principal.** Because the subprocess dials a local UDS, the daemon treats it as the `system:local` principal — equivalent to the logged-in user running any other `gohome` CLI command. All daemon-side enforcement applies normally.

**ADMIN-class tools.** `gohome__apply_config` and `gohome__write_config_file` are admin-verb tools — they mutate persistent state. They are subject to the same daemon-side policy as any other caller. Per-tool policy gating is planned for C9.

---

## Getting an API token (C9, coming soon)

!!! status-wip "In development"

Token-based auth is planned as part of C9. Once shipped, you will be able to create a named token scoped to MCP:

```sh
gohome auth tokens create --name "claude" --scope mcp
```

The token will be passed to `gohome mcp serve` via an environment variable and will enable the HTTP transport for remote MCP access. Until C9 ships, the stdio transport with `system:local` auth covers local agent use.

---

## Troubleshooting

### `gohomed not running` or subprocess exits immediately

The MCP subprocess prints a message to stderr and exits if it cannot reach the daemon:

```
gohome mcp: cannot reach gohomed at unix://@data/gohomed.sock: connection refused
```

Start the daemon and try again.

### The daemon socket path

The default socket is `~/.local/share/gohome/gohomed.sock`. To use a different path:

```sh
gohome mcp serve --endpoint unix:///path/to/gohomed.sock
```

Or set the environment variable:

```sh
export GOHOME_ENDPOINT=unix:///path/to/gohomed.sock
```

### Inspecting MCP activity

All MCP tool calls appear in the daemon event log with `source=mcp`. Tail the event log to see what the agent is doing:

```sh
gohome events tail
```
