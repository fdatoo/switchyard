# Docker

!!! status-alpha "Alpha — shipped, interface evolving"
    The OCI image is published to GitHub Container Registry. The image API (environment variables, volume paths) is stable for v0.x but may change at v1.0.

switchyard ships a minimal OCI image. It is a good fit if you run Docker, Docker Compose, Portainer, Unraid, or any other container host.

## Image

```
ghcr.io/fynn-labs/switchyard:latest
```

Versioned tags are preferred for production use:

```
ghcr.io/fynn-labs/switchyard:v0.1.0
```

The image contains only `switchyardd` and `switchyard`. It is built `FROM scratch` — no shell, no package manager, no OS layer. The binary handles everything.

Pull the image to verify it is accessible:

```bash
docker pull ghcr.io/fynn-labs/switchyard:latest
```

## Quick start (single container)

Create the host directories first — if Docker creates them, they will be owned by root:

```bash
mkdir -p ./config ./data
```

Then start the container:

```bash
docker run -d \
  --name switchyardd \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 9090:9090 \
  -v "$(pwd)/config:/config" \
  -v "$(pwd)/data:/data" \
  -e LOG_LEVEL=info \
  ghcr.io/fynn-labs/switchyard:latest
```

This binds:

- `./config` on your host → `/config` in the container (Pkl config files)
- `./data` on your host → `/data` in the container (SQLite event store, driver state)

## Docker Compose

Save this as `compose.yaml` (or `docker-compose.yml`):

```yaml
services:
  switchyardd:
    image: ghcr.io/fynn-labs/switchyard:v0.1.0
    container_name: switchyardd
    restart: unless-stopped
    ports:
      - "8080:8080"   # Connect-RPC API + embedded web UI
      - "9090:9090"   # MCP server (HTTP transport)
    volumes:
      - ./config:/config    # Pkl configuration files
      - ./data:/data        # SQLite event store, driver state, certs
    environment:
      LOG_LEVEL: info
      CONFIG_PATH: /config/main.pkl
      DATA_DIR: /data
    # Optional: run as a non-root user.
    # The UID/GID must have write access to ./config and ./data on the host.
    # user: "1000:1000"
```

Start the stack:

```bash
docker compose up -d
```

View logs:

```bash
docker compose logs -f switchyardd
```

Stop and remove the container (volumes are preserved):

```bash
docker compose down
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `CONFIG_PATH` | `/config/main.pkl` | Path to the root Pkl config file inside the container |
| `DATA_DIR` | `/data` | Directory for the SQLite event database, driver state, and generated TLS certificates |
| `LOG_LEVEL` | `info` | Log verbosity. One of `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `text` | Log format. `text` for human-readable, `json` for structured output |
| `API_LISTEN` | `:8080` | Listen address for the Connect-RPC API and embedded web UI |
| `MCP_LISTEN` | `:9090` | Listen address for the MCP server (HTTP transport) |
| `MCP_STDIO` | `false` | Set to `true` to run the MCP server on stdio instead of HTTP. Disables `MCP_LISTEN`. |

## Port reference

| Port | Protocol | Description |
|---|---|---|
| `8080` | HTTP/2 (Connect-RPC), HTTP/1.1 (SSE) | Connect-RPC API — used by the `switchyard` CLI, the embedded web UI, and third-party clients |
| `9090` | HTTP | MCP server — used by AI agents (Claude, etc.) over the HTTP MCP transport |

If you are running behind a reverse proxy (nginx, Caddy, Traefik), expose only `8080` externally and handle TLS termination at the proxy. The MCP server on `9090` should remain internal unless you are explicitly exposing it to an external agent runtime.

## Volume reference

| Container path | Purpose | Notes |
|---|---|---|
| `/config` | Pkl configuration files | Mount as read-write; switchyard reads and watches this directory. Consider mounting read-only if you manage config externally and reload via `switchyard config apply`. |
| `/data` | Event store and runtime data | Must be read-write and persistent. Contains `events.db` (SQLite), `drivers/` (per-driver state), and `certs/` (generated mTLS certificates for edge agents). |

!!! warning "Do not use a tmpfs for /data"
    The event store is the source of truth for all entity state. If `/data` is not persistent across container restarts, all history, entity registrations, and driver pairings are lost on every restart.

## Running the CLI against a Docker container

The `switchyard` CLI connects to the daemon over Connect-RPC. You can run it from your host (if `8080` is published), from inside the container, or from another container on the same network.

**From your host:**

```bash
switchyard --server http://localhost:8080 status
```

**From inside the running container:**

```bash
docker exec switchyardd switchyard status
```

**Set a default server** so you don't repeat the flag:

```bash
export GOHOME_SERVER=http://localhost:8080
switchyard status
```

## Next step

Continue to [First run](first-run.md) to create your Pkl config and confirm the daemon is healthy.
