# CLI Reference

!!! status-alpha "Alpha — shipped, interface evolving"

switchyard ships two binaries:

- **`switchyardd`** — the daemon process
- **`switchyard`** — the operator CLI

The CLI connects to the daemon over Connect-RPC. By default it dials the Unix domain socket at `~/.local/share/switchyard/switchyardd.sock`. Override with `--endpoint` or `SWITCHYARD_ENDPOINT`.

---

## Global flags

These flags are accepted by every `switchyard` subcommand.

| Flag | Default | Description |
|------|---------|-------------|
| `--data-dir <path>` | `~/.local/share/switchyard` | Path to the switchyard data directory (where the UDS socket lives) |
| `--endpoint <uri>` | _(UDS from data-dir)_ | Explicit API endpoint: `unix:///path/to/sock`, `tcp://host:port`, or `https://host:port` |
| `--format <fmt>` | `auto` | Output format: `auto`, `table`, `json`, or `yaml` |
| `--no-color` | `false` | Disable ANSI colour output |
| `--log-level <level>` | `warn` | Log verbosity: `error`, `warn`, `info`, `debug` |
| `-v`, `--verbose` | `false` | Shorthand for `--log-level=debug` |

`SWITCHYARD_ENDPOINT` environment variable overrides `--endpoint`.

---

## `switchyardd`

The daemon process. Run directly or managed by systemd.

### `switchyardd start`

```
switchyardd [flags]
```

Starts the daemon. Loads config from the config directory, binds the UDS and TCP listeners, and enters the event loop. Fatal on invalid config.

| Flag | Default | Description |
|------|---------|-------------|
| `--config-dir <path>` | `~/.config/switchyard` | Directory containing `main.pkl` and supporting Pkl files |
| `--data-dir <path>` | `~/.local/share/switchyard` | Directory for the SQLite database, UDS socket, and snapshots |
| `--bind <addr>` | `127.0.0.1:8080` | TCP listener bind address |
| `--log-level <level>` | `info` | Log verbosity |

Exit codes: `0` = clean shutdown, `1` = fatal error (invalid config, port conflict, etc.).

### `switchyardd version`

```
switchyardd version
```

Prints the daemon binary version and git commit.

---

## `switchyard version`

```
switchyard version
```

Prints the CLI binary version and git commit. Does not connect to the daemon.

---

## Config commands

### `switchyard config validate`

```
switchyard config validate [global flags]
```

Asks the daemon to evaluate and validate the on-disk Pkl config without applying it. Prints a summary of what would change.

**Output on success:**

```
✓ Config valid
Driver instances added  +2
Driver instances removed -0
Driver instances changed ~1
Automations changed     ~3
```

**Exit codes:** `0` = valid, `1` = invalid (errors printed to stderr).

### `switchyard config apply`

```
switchyard config apply [--dry-run] [--message <msg>] [global flags]
```

Evaluates, validates, and applies the current on-disk config. Prints a diff table. On success, emits a `ConfigApplied` event.

| Flag | Description |
|------|-------------|
| `--dry-run` | Validate and print the diff, but do not persist or restart any driver instances |
| `--message <msg>` | Free-form change message recorded with the apply (audit trail) |

**Exit codes:** `0` = applied (or dry-run completed), `1` = validation failure or apply error.

### `switchyard config reload`

```
switchyard config reload [global flags]
```

Tells the daemon to re-read config from its configured directory without sending a new bundle. Use this after editing files in place. Equivalent to `apply` but reads from the daemon's config dir rather than the CLI's working directory.

---

## Status commands

### `switchyard system health`

```
switchyard system health [global flags]
```

Displays daemon health: overall status plus per-subsystem breakdown (event store, registry, each driver instance, automation engine).

**Example output:**

```
OK
  ✓ eventstore — healthy, 142 893 events
  ✓ registry   — 12 entities, 2 drivers
  ✗ hue_bridge — last heartbeat 47s ago (threshold 30s)
```

### `switchyard system version`

```
switchyard system version [global flags]
```

Returns the daemon's version, git commit, build date, and schema version.

### `switchyard state get <entity-id>`

```
switchyard state get <entity-id> [global flags]
```

Prints the current state of a single entity by its dotted ID (e.g. `light.living_room`). State is returned as protojson.

### `switchyard state dump`

```
switchyard state dump [global flags]
```

Dumps all entity states as a JSON object keyed by entity ID. Paginates automatically.

---

## Event commands

### `switchyard events query`

```
switchyard events query [--kind <kind>] [--entity <prefix>] [--limit <n>] [global flags]
```

Historical query against the event log. Returns events sorted by cursor ascending.

| Flag | Default | Description |
|------|---------|-------------|
| `--kind <kind>` | — | Filter to a single event kind, e.g. `state_changed` |
| `--entity <prefix>` | — | Filter by entity ID prefix, e.g. `light.` |
| `--limit <n>` | `100` | Maximum number of events to return |

### `switchyard events tail`

```
switchyard events tail [--kind <kind>] [--entity <prefix>] [global flags]
```

Streams live events from the daemon. Press `Ctrl-C` to stop.

| Flag | Description |
|------|-------------|
| `--kind <kind>` | Filter to a single event kind |
| `--entity <prefix>` | Filter by entity ID prefix |

### `switchyard events inspect <position>`

```
switchyard events inspect <position> [global flags]
```

Shows a single event in full detail, including raw payload fields. Reads the local SQLite database directly (no daemon required).

### `switchyard events export`

```
switchyard events export [--from <pos>] [--to <pos>] [-o <file>] [global flags]
```

Exports events as JSONL (newline-delimited JSON). Reads the local database directly.

| Flag | Default | Description |
|------|---------|-------------|
| `--from <pos>` | `0` | Start position (exclusive) |
| `--to <pos>` | `0` | End position (inclusive); `0` means unbounded |
| `-o <file>` | `-` | Output file; `-` for stdout |

---

## Driver commands

### `switchyard driver list`

```
switchyard driver list [global flags]
```

Lists all configured driver instances with their status, entity count, and last handshake time.

### `switchyard driver status <instance>`

```
switchyard driver status <instance> [global flags]
```

Shows the out-of-band health status for a single driver instance (calls the Carport `Health` RPC).

**Exit codes:** `0` = healthy, `1` = unhealthy or not found.

### `switchyard driver restart <instance>`

```
switchyard driver restart <instance> [--reason <msg>] [global flags]
```

Signals the Carport supervisor to restart a driver instance. Returns immediately; the restart is asynchronous.

| Flag | Default | Description |
|------|---------|-------------|
| `--reason <msg>` | `"manual"` | Reason string recorded in the `DriverInstanceRestarted` event |

---

## Automation commands

### `switchyard automation list`

```
switchyard automation list [global flags]
```

Lists all registered automations with their enabled state and trigger mode.

### `switchyard automation get <id>`

```
switchyard automation get <id> [global flags]
```

Shows details for a single automation.

### `switchyard automation enable <id>`

```
switchyard automation enable <id> [global flags]
```

Enables an automation in-memory. This override reverts on daemon restart. For a durable change, edit the Pkl config and run `switchyard config apply`.

### `switchyard automation disable <id>`

```
switchyard automation disable <id> [global flags]
```

Disables an automation in-memory. Same caveats as `enable`.

### `switchyard automation trigger <id>`

```
switchyard automation trigger <id> [global flags]
```

Manually fires an automation. Returns an error (`FAILED_PRECONDITION`) if the automation is disabled.

### `switchyard automation trace <id> [run-id]`

```
switchyard automation trace <id> [run-id] [global flags]
```

Streams the run timeline for an automation. If `run-id` is omitted, streams the next run. Each event is printed with timestamp, kind, outcome, and elapsed time.

### `switchyard automation watch`

```
switchyard automation watch [global flags]
```

Streams all automation and script lifecycle events (`automation_triggered`, `automation_finished`, `script_invoked`, `script_finished`) until interrupted.

---

## Script commands

### `switchyard script list`

```
switchyard script list [global flags]
```

Lists all registered scripts by name.

### `switchyard script run <name>`

```
switchyard script run <name> [--arg key=value ...] [global flags]
```

Runs a named script synchronously. Blocks until the script returns or the deadline expires.

| Flag | Description |
|------|-------------|
| `--arg key=value` | Script argument (repeatable). Values are strings; the script receives them as a `dict`. |

Prints the return value (if non-None) and the `run_id` for log cross-reference.

!!! note
    `script cancel` and `script eval` are RPC-only operations (`ScriptService.Cancel` and `ScriptService.Eval`); they are not exposed as CLI subcommands.

---

## Snapshot commands

### `switchyard snapshot create`

```
switchyard snapshot create [--owner <owner>] [--reason <msg>] [global flags]
```

Triggers an immediate projector snapshot via the daemon. Snapshots speed up daemon restarts by reducing the number of events that must be replayed.

| Flag | Default | Description |
|------|---------|-------------|
| `--owner <owner>` | `"state_cache"` | Projector owner to snapshot |
| `--reason <msg>` | `"manual"` | Reason recorded in snapshot metadata |

### `switchyard snapshot list`

```
switchyard snapshot list [--owner <owner>] [global flags]
```

Lists snapshots stored in the local database (reads directly, no daemon required).

---

## Command (capability) invocation

### `switchyard command send <entity> <capability>`

```
switchyard command send <entity> <capability> [--arg k=v ...] [global flags]
```

Invokes a capability on an entity via the daemon. Equivalent to calling `EntityService.CallCapability` over the RPC API.

| Flag | Description |
|------|-------------|
| `--arg k=v` | Capability argument key=value pair (repeatable) |

---

## Starlark eval

### `switchyard eval <file.star>`

```
switchyard eval <file.star> [global flags]
```

Evaluates a Starlark file against the running daemon. Useful for ad-hoc inspection and one-off operations. The script has access to the full switchyard Starlark API (state, event, capabilities).

---

## MCP commands

### `switchyard mcp serve`

```
switchyard mcp serve [global flags]
```

Starts the MCP server on stdio. Used by MCP clients (e.g. Claude Code) to connect switchyard as an MCP server. The MCP server connects to `switchyardd` via the normal Connect-RPC endpoint.

### `switchyard mcp tools`

```
switchyard mcp tools [--json] [global flags]
```

Prints the MCP tool catalog. No daemon connection required.

| Flag | Description |
|------|-------------|
| `--json` | Emit machine-readable JSON instead of a styled table |

---

## Auth commands

!!! status-planned "Planned"
    All `switchyard auth *` subcommands depend on `AuthService`, which is UNIMPLEMENTED in the current release. The interface below is the planned API.

### `switchyard auth login`

```
switchyard auth login [global flags]
```

Log in to switchyard. Opens a browser window for passkey or password authentication. Stores the resulting session token for subsequent CLI calls.

### `switchyard auth logout`

```
switchyard auth logout [global flags]
```

Log out of the current session. Revokes the active session token.

### `switchyard auth whoami`

```
switchyard auth whoami [global flags]
```

Show the currently authenticated user identity (user ID, display name, roles).

### `switchyard auth tokens list`

```
switchyard auth tokens list [global flags]
```

List all API tokens for the current user.

### `switchyard auth tokens create`

```
switchyard auth tokens create --name <name> [--scope <scope>] [global flags]
```

Create a new API token.

| Flag | Description |
|------|-------------|
| `--name <name>` | Human-readable token name (required) |
| `--scope <scope>` | Comma-separated permission scopes to grant (defaults to the current user's full scope) |

### `switchyard auth tokens revoke`

```
switchyard auth tokens revoke <token-id> [global flags]
```

Revoke an API token by its ID. Revocation is immediate.

### `switchyard auth users list`

```
switchyard auth users list [global flags]
```

List all users in the system.

### `switchyard auth passkeys list`

```
switchyard auth passkeys list [global flags]
```

List passkeys registered for the current user.

### `switchyard auth set-password`

```
switchyard auth set-password [global flags]
```

Change the password for the current user. Prompts for the current and new passwords interactively.

### `switchyard auth explain`

```
switchyard auth explain [global flags]
```

Explain what the current user's role and policy allows. Prints a human-readable summary of effective permissions.

---

## Backup commands

!!! status-planned "Planned"
    `switchyard backup` and `switchyard restore` are not yet implemented. The interface below is the planned API.

### `switchyard backup`

```
switchyard backup <output-path> [--encrypt] [global flags]
```

Create a backup archive containing an SQLite online backup and a tarball of the config directory. Writes to `<output-path>`.

| Flag | Description |
|------|-------------|
| `--encrypt` | Encrypt the backup archive (prompts for a passphrase) |

### `switchyard restore`

```
switchyard restore <backup-path> [global flags]
```

Restore switchyard from a backup file created by `switchyard backup`. The daemon must not be running when this command is issued.

---

## Diagnostics

!!! status-planned "Planned"
    `switchyard diag` is not yet implemented. The interface below is the planned API.

### `switchyard diag`

```
switchyard diag [global flags]
```

Generate a diagnostic bundle for support. Collects and redacts: binary versions, driver versions, recent errors, and health snapshots. Writes the bundle to `switchyard-diag-<timestamp>.tar.gz` in the current directory.

---

## Self-update

!!! status-planned "Planned"
    `switchyard self-update` is not yet implemented. The interface below is the planned API.

### `switchyard self-update`

```
switchyard self-update [global flags]
```

Download the latest `switchyardd` and `switchyard` binaries, verify the Sigstore signature, and atomically replace the running binaries. If the daemon is managed by systemd, restarts the service after the update.

---

## Registry commands

### `switchyard registry list (devices|entities|drivers)`

```
switchyard registry list <collection> [global flags]
```

Lists registry rows. The `<collection>` argument is required and must be one of:

| Collection | Columns shown |
|------------|---------------|
| `devices` | ID, Driver, Name |
| `entities` | ID, Type, Name, Driver |
| `drivers` | ID, Driver, Status, Endpoint |

Reads the local SQLite database directly — no daemon connection required.

### `switchyard registry show <id>`

```
switchyard registry show <id> [global flags]
```

Shows details for a single entity, device, or driver instance by its ID. Tries entity, device, and driver lookups in order and prints the first match. Returns an error if no match is found.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (printed to stderr). Includes validation failures, RPC errors, and not-found results. |
| `2` | Bad arguments (wrong number of positional args, unrecognised flag) |
