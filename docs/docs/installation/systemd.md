# systemd / packages

!!! status-wip "In development"
    Package distribution (`.deb`, `.rpm`, Homebrew) is not yet available in the current alpha. Use the [static binary](binary.md) or [Docker](docker.md) install method for now. The systemd unit template described here works with the static binary today.

On production Linux servers, running `switchyardd` as a systemd service is the recommended approach. systemd handles process supervision, log collection, restart on failure, and startup ordering.

## Option A: Install from package (Debian/Ubuntu)

```bash
# Add the switchyard apt repository
curl -fsSL https://packages.fynn-labs.dev/gpg.key \
  | sudo gpg --dearmor -o /usr/share/keyrings/fynn-labs.gpg

echo "deb [arch=amd64 signed-by=/usr/share/keyrings/fynn-labs.gpg] \
  https://packages.fynn-labs.dev/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/fynn-labs.list

sudo apt update
sudo apt install switchyard
```

The package installs:

- `/usr/bin/switchyardd` — the daemon
- `/usr/bin/switchyard` — the CLI
- `/etc/systemd/system/switchyardd.service` — the systemd unit
- `/etc/switchyard/` — default config directory (empty; populate before starting)

## Option B: Install from package (Fedora / RHEL / Rocky)

```bash
# Add the switchyard RPM repository
sudo tee /etc/yum.repos.d/fynn-labs.repo <<'EOF'
[fynn-labs]
name=Fynn Labs – switchyard
baseurl=https://packages.fynn-labs.dev/rpm/stable/$basearch
enabled=1
gpgcheck=1
gpgkey=https://packages.fynn-labs.dev/gpg.key
EOF

sudo dnf install switchyard
```

## Option C: Install via Homebrew (macOS) {#homebrew}

```bash
brew tap fynn-labs/tap
brew install fynn-labs/tap/switchyard
```

This installs both `switchyardd` and `switchyard`. On macOS, `switchyardd` can be run as a LaunchAgent rather than systemd — see the formula's caveats after install:

```bash
brew info fynn-labs/tap/switchyard
```

## Option D: systemd unit with the static binary

If you installed via the [static binary](binary.md) method and want systemd supervision, create the unit manually.

Create a dedicated user:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin switchyardd
```

Create the config and data directories:

```bash
sudo mkdir -p /etc/switchyard /var/lib/switchyard
sudo chown switchyardd:switchyardd /etc/switchyard /var/lib/switchyard
sudo chmod 750 /etc/switchyard /var/lib/switchyard
```

Create the systemd unit at `/etc/systemd/system/switchyardd.service`:

```ini
[Unit]
Description=switchyard daemon
Documentation=https://switchyard.dev/docs/installation/
After=network-online.target
Wants=network-online.target
StartLimitBurst=5
StartLimitIntervalSec=60s

[Service]
Type=simple
User=switchyardd
Group=switchyardd

ExecStart=/usr/local/bin/switchyardd \
  --config /etc/switchyard/main.pkl \
  --data-dir /var/lib/switchyard

ExecReload=/bin/kill -HUP $MAINPID

Restart=on-failure
RestartSec=5s

# Logging — all output goes to the journal
StandardOutput=journal
StandardError=journal
SyslogIdentifier=switchyardd

# Hardening (optional but recommended)
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/switchyard /etc/switchyard
ProtectHome=true
CapabilityBoundingSet=

[Install]
WantedBy=multi-user.target
```

## Enable and start the service

These steps apply whether you installed via a package or created the unit manually.

Populate your config before starting (see [First run](first-run.md)):

```bash
# Minimal smoke-check: validate config before the daemon tries to load it
switchyard config validate --config /etc/switchyard/main.pkl
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable switchyardd
sudo systemctl start switchyardd
```

Check status:

```bash
sudo systemctl status switchyardd
```

## View logs

```bash
# Follow live logs
journalctl -u switchyardd -f

# Show the last 100 lines
journalctl -u switchyardd -n 100

# Show logs since last boot
journalctl -u switchyardd -b

# Show logs between two timestamps
journalctl -u switchyardd --since "2026-04-27 08:00" --until "2026-04-27 10:00"
```

## Reload config without a restart

switchyardd supports SIGHUP-triggered config reloads. Only changed driver instances and automations are re-initialized; the event store and existing connections are not interrupted.

```bash
# Via systemctl
sudo systemctl reload switchyardd

# Or via the CLI (sends the reload RPC)
switchyard config apply
```

## Uninstall

=== "apt"

    ```bash
    sudo apt remove switchyard
    # Config and data directories are preserved; remove manually if desired:
    # sudo rm -rf /etc/switchyard /var/lib/switchyard
    ```

=== "dnf"

    ```bash
    sudo dnf remove switchyard
    ```

=== "Manual"

    ```bash
    sudo systemctl stop switchyardd
    sudo systemctl disable switchyardd
    sudo rm /etc/systemd/system/switchyardd.service
    sudo systemctl daemon-reload
    sudo rm /usr/local/bin/switchyardd /usr/local/bin/switchyard
    ```

## Next step

Continue to [First run](first-run.md) to create your Pkl config and confirm the daemon is healthy.
