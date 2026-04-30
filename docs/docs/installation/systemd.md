# systemd / packages

!!! status-wip "In development"
    Package distribution (`.deb`, `.rpm`, Homebrew) is not yet available in the current alpha. Use the [static binary](binary.md) or [Docker](docker.md) install method for now. The systemd unit template described here works with the static binary today.

On production Linux servers, running `gohomed` as a systemd service is the recommended approach. systemd handles process supervision, log collection, restart on failure, and startup ordering.

## Option A: Install from package (Debian/Ubuntu)

```bash
# Add the gohome apt repository
curl -fsSL https://packages.fynn-labs.dev/gpg.key \
  | sudo gpg --dearmor -o /usr/share/keyrings/fynn-labs.gpg

echo "deb [arch=amd64 signed-by=/usr/share/keyrings/fynn-labs.gpg] \
  https://packages.fynn-labs.dev/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/fynn-labs.list

sudo apt update
sudo apt install gohome
```

The package installs:

- `/usr/bin/gohomed` — the daemon
- `/usr/bin/gohome` — the CLI
- `/etc/systemd/system/gohomed.service` — the systemd unit
- `/etc/gohome/` — default config directory (empty; populate before starting)

## Option B: Install from package (Fedora / RHEL / Rocky)

```bash
# Add the gohome RPM repository
sudo tee /etc/yum.repos.d/fynn-labs.repo <<'EOF'
[fynn-labs]
name=Fynn Labs – gohome
baseurl=https://packages.fynn-labs.dev/rpm/stable/$basearch
enabled=1
gpgcheck=1
gpgkey=https://packages.fynn-labs.dev/gpg.key
EOF

sudo dnf install gohome
```

## Option C: Install via Homebrew (macOS) {#homebrew}

```bash
brew tap fynn-labs/tap
brew install fynn-labs/tap/gohome
```

This installs both `gohomed` and `gohome`. On macOS, `gohomed` can be run as a LaunchAgent rather than systemd — see the formula's caveats after install:

```bash
brew info fynn-labs/tap/gohome
```

## Option D: systemd unit with the static binary

If you installed via the [static binary](binary.md) method and want systemd supervision, create the unit manually.

Create a dedicated user:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin gohomed
```

Create the config and data directories:

```bash
sudo mkdir -p /etc/gohome /var/lib/gohome
sudo chown gohomed:gohomed /etc/gohome /var/lib/gohome
sudo chmod 750 /etc/gohome /var/lib/gohome
```

Create the systemd unit at `/etc/systemd/system/gohomed.service`:

```ini
[Unit]
Description=gohome daemon
Documentation=https://gohome.dev/docs/installation/
After=network-online.target
Wants=network-online.target
StartLimitBurst=5
StartLimitIntervalSec=60s

[Service]
Type=simple
User=gohomed
Group=gohomed

ExecStart=/usr/local/bin/gohomed \
  --config /etc/gohome/main.pkl \
  --data-dir /var/lib/gohome

ExecReload=/bin/kill -HUP $MAINPID

Restart=on-failure
RestartSec=5s

# Logging — all output goes to the journal
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gohomed

# Hardening (optional but recommended)
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/gohome /etc/gohome
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
gohome config validate --config /etc/gohome/main.pkl
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable gohomed
sudo systemctl start gohomed
```

Check status:

```bash
sudo systemctl status gohomed
```

## View logs

```bash
# Follow live logs
journalctl -u gohomed -f

# Show the last 100 lines
journalctl -u gohomed -n 100

# Show logs since last boot
journalctl -u gohomed -b

# Show logs between two timestamps
journalctl -u gohomed --since "2026-04-27 08:00" --until "2026-04-27 10:00"
```

## Reload config without a restart

gohomed supports SIGHUP-triggered config reloads. Only changed driver instances and automations are re-initialized; the event store and existing connections are not interrupted.

```bash
# Via systemctl
sudo systemctl reload gohomed

# Or via the CLI (sends the reload RPC)
gohome config apply
```

## Uninstall

=== "apt"

    ```bash
    sudo apt remove gohome
    # Config and data directories are preserved; remove manually if desired:
    # sudo rm -rf /etc/gohome /var/lib/gohome
    ```

=== "dnf"

    ```bash
    sudo dnf remove gohome
    ```

=== "Manual"

    ```bash
    sudo systemctl stop gohomed
    sudo systemctl disable gohomed
    sudo rm /etc/systemd/system/gohomed.service
    sudo systemctl daemon-reload
    sudo rm /usr/local/bin/gohomed /usr/local/bin/gohome
    ```

## Next step

Continue to [First run](first-run.md) to create your Pkl config and confirm the daemon is healthy.
