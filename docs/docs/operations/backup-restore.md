# Backup & Restore

!!! status-planned "Planned — not yet implemented"
    The `gohome backup` and `gohome restore` commands are fully designed and the backup strategy is documented here. The commands are not yet implemented in the current binary. In the meantime, use the manual procedure described at the bottom of this page.

## What constitutes the full state

gohome's persistent state lives in three places:

| Location | Contents | Size guidance |
|---|---|---|
| Config directory (`$GOHOME_CONFIG`) | All Pkl source files — your declarations | Typically < 1 MB |
| SQLite database (`$GOHOME_DATA/gohome.db`) | Event log, registry projections, snapshots | Grows over time; ~40 GB/year at 200 entities |
| Driver binaries (`$GOHOME_DATA/drivers/`) | Downloaded driver executables | Varies; reinstallable from release artifacts |

A complete backup captures all three. The config directory is already git-trackable — the recommended happy path is to commit it to a private repo so it has its own history independent of the daemon.

## `gohome backup`

```bash
gohome backup ~/backups/gohome-$(date +%Y%m%d).tar.gz
```

The backup command:

1. Uses the **SQLite online backup API** to snapshot the event database while the daemon continues running. No shutdown required. The snapshot is transactionally consistent.
2. Archives the config directory and the SQLite snapshot together into the output `.tar.gz`.
3. Driver binaries are **not** included by default (they are platform-specific and reinstallable). Pass `--include-drivers` to include them.

### Encryption

```bash
gohome backup --encrypt ~/backups/gohome-$(date +%Y%m%d).tar.gz
```

When `--encrypt` is passed, gohomed prompts for a passphrase and encrypts the archive with AES-256-GCM before writing it to disk. Keep the passphrase safe — there is no recovery path without it.

### Automation

For unattended backups, pipe the passphrase in or set it via `GOHOME_BACKUP_PASSPHRASE`:

```bash
GOHOME_BACKUP_PASSPHRASE="$(cat /run/secrets/backup_key)" \
  gohome backup --encrypt /mnt/nas/gohome-$(date +%Y%m%d).tar.gz
```

Schedule with `cron` or a systemd timer. A daily backup typically completes in under a minute for a typical install.

## `gohome restore`

Restoration requires the daemon to be **stopped first**. The restore command will refuse to run if it detects a live daemon.

```bash
# Stop the daemon
sudo systemctl stop gohomed

# Restore from backup
gohome restore ~/backups/gohome-20260427.tar.gz

# Start the daemon
sudo systemctl start gohomed
```

If the backup was encrypted:

```bash
gohome restore --decrypt ~/backups/gohome-20260427.tar.gz
# Prompts for the passphrase
```

The restore command:

1. Validates the archive integrity.
2. Decrypts if `--decrypt` is passed.
3. Writes the config directory, replacing the existing contents.
4. Replaces `gohome.db` with the archived snapshot.
5. Does not restore driver binaries unless they were included with `--include-drivers` during backup. Run `gohome driver install <name>` after restore to reinstall drivers.

## Moving to a new server

The one-liner pattern for migrating to a new host:

```bash
# On the old server
gohome backup /tmp/gohome-migration.tar.gz
scp /tmp/gohome-migration.tar.gz newhost:/tmp/

# On the new server — after installing gohomed
sudo systemctl stop gohomed
gohome restore /tmp/gohome-migration.tar.gz
gohome driver install zigbee2mqtt hue     # reinstall drivers for this platform
sudo systemctl start gohomed
```

!!! warning "Driver binaries are platform-specific"
    Driver binaries compiled for `linux/amd64` will not run on `linux/arm64` (e.g. a Raspberry Pi). After restoring on a different architecture, reinstall all drivers with `gohome driver install <name>` rather than restoring the binaries from the backup.

## Manual backup (current workaround)

Until `gohome backup` is implemented, use the SQLite CLI directly:

```bash
# Stop the daemon first, or use the online backup API directly
sqlite3 "$GOHOME_DATA/gohome.db" ".backup /tmp/gohome-backup.db"

# Archive config + db snapshot
tar -czf ~/backups/gohome-$(date +%Y%m%d).tar.gz \
  -C "$(dirname "$GOHOME_CONFIG")" "$(basename "$GOHOME_CONFIG")" \
  /tmp/gohome-backup.db

rm /tmp/gohome-backup.db
```

The online backup API used by the `sqlite3` `.backup` command is safe to run while the daemon is running — it produces a consistent, page-level snapshot without locking the writer.
