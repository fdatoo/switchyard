# Updates

!!! status-alpha "Alpha — shipped, interface evolving"
    `switchyard self-update` and `switchyard driver upgrade` are shipped. apt/brew packages and auto-update opt-in are still being finalized.

## Updating switchyardd

### Bare-metal: `switchyard self-update`

```bash
switchyard self-update
```

This command:

1. Queries the GitHub Releases API for the latest version.
2. Downloads the new binary for the current platform.
3. Verifies the sigstore/cosign signature against the release manifest.
4. Atomically replaces the running binary (write to a temp file, then rename).
5. If switchyardd is running under systemd, restarts the unit automatically.

The binary is never replaced if signature verification fails. There is no silent auto-update — `switchyard self-update` must be invoked explicitly or via a user-configured cron trigger.

```bash
# Check what version would be installed without installing it
switchyard self-update --dry-run
```

### Docker / OCI

```bash
docker pull ghcr.io/fynn-labs/switchyardd:latest
# or pin to a specific version:
docker pull ghcr.io/fynn-labs/switchyardd:v0.2.0
```

Then restart your container. Compose users:

```bash
docker compose pull && docker compose up -d
```

### apt

```bash
sudo apt update && sudo apt upgrade switchyardd switchyard
```

### Homebrew (macOS)

```bash
brew upgrade switchyard
```

## Schema migrations

switchyardd runs database migrations at startup using [golang-migrate](https://github.com/golang-migrate/migrate). You do not need to do anything manually — migrations are embedded in the binary and applied automatically when a new version starts.

Migration safety:

- **Pre-migration copy** — before running any migration, switchyardd copies `switchyard.db` to `switchyard.db.pre-migrate.<version>` in the data directory. If something goes wrong you have a rollback point.
- **Only-forward** — migrations are never reversed automatically. To downgrade you must restore from backup.
- **Additive only** — schema changes only add columns or tables. Existing columns are never dropped or renamed within a major version.

If a migration fails, switchyardd exits with a clear error before opening any connections. Fix the underlying issue (disk space, permissions) and restart.

## Event schema backward compatibility

Old events are valid forever. The event log uses an append-only model:

- **New event kinds** are additive. Projectors that do not recognise a kind log it and skip it. Older versions of switchyardd are safe to run against a database written by a newer version, as long as the schema version is compatible.
- **New fields on existing events** are additive — serialised using protobuf field numbers, so old readers ignore unknown fields.
- **No field removal or renaming** without a major version bump. This is enforced by the project's breaking-change policy.

## Driver updates

Drivers version independently of switchyardd. Each driver advertises its supported Carport protocol version on handshake. switchyardd refuses to start a driver whose Carport version falls outside the compatible range.

To upgrade a single driver without restarting the daemon:

```bash
switchyard driver upgrade zigbee2mqtt
```

This:

1. Downloads the new driver binary and verifies its signature.
2. Stops the running driver instance gracefully.
3. Replaces the binary.
4. Restarts the driver instance.

Only the upgraded driver restarts — all other driver instances continue running uninterrupted.

To see available upgrades:

```bash
switchyard driver list
```

The output includes the installed version, the latest available version, and whether the Carport protocol version is compatible.

## Pkl module version pinning

The `switchyard:*` Pkl modules are embedded in the daemon binary. The minimum supported Pkl module version is validated at config load time. If your config imports an older module signature the daemon will print a clear error:

```
error: config imports switchyard:automations@0.1; minimum supported is 0.2
       run `switchyard config migrate-pkl` to update import paths
```

To pin to a specific module version in your imports (useful for CI validation against a known version):

```pkl
import "switchyard:automations@0.2" as automations
```

Omitting the version always resolves to the version embedded in the running daemon.
