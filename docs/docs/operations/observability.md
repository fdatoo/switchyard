# Observability

!!! status-alpha "Alpha — shipped, interface evolving"
    Structured logs and Prometheus metrics are shipped. OpenTelemetry tracing is in development — the OTLP exporter configuration is available but traces are sparse in the current release.

## Structured logs

gohomed writes structured logs using Go's standard `log/slog` package. By default logs are emitted to stderr.

| Setting | Default | Options |
|---|---|---|
| Format | `auto` (TTY-detected) | `auto`, `json`, `tty` |
| Level | `info` | `debug`, `info`, `warn`, `error` |

In `auto` mode, gohomed uses a human-readable format when stderr is a terminal and switches to JSON automatically when running under systemd or Docker.

Set log level at startup:

```bash
gohomed --log-level debug --log-format json
```

Or via environment variables (see [Deployment](deployment.md#environment-variables)):

```bash
GOHOME_LOG_LEVEL=debug GOHOME_LOG_FORMAT=json gohomed
```

JSON log lines include `time`, `level`, `msg`, and structured key-value pairs specific to each event. Example:

```json
{"time":"2026-04-27T14:32:01Z","level":"INFO","msg":"event appended","kind":"state_changed","entity_id":"light.living_room","cursor":4821}
```

## Prometheus metrics

gohomed exposes a Prometheus-compatible `/metrics` endpoint on the admin port (default `9190`):

```
http://localhost:9190/metrics
```

This port also serves `/health` which returns a JSON health summary.

!!! tip "Admin port vs API port"
    The `/metrics` endpoint is on port `9190` (admin), not `8080` (API). The admin port is unauthenticated — restrict access at the network level if needed.

### Key metrics

| Metric | Type | Description |
|---|---|---|
| `gohome_events_appended_total` | Counter | Events written to the event log, labelled by `kind` |
| `gohome_events_append_duration_seconds` | Histogram | End-to-end append latency |
| `gohome_automation_runs_total` | Counter | Automation run completions, labelled by `automation_id` and `outcome` |
| `gohome_automation_run_duration_seconds` | Histogram | Wall-clock duration of automation runs |
| `gohome_automation_triggers_total` | Counter | Trigger fires admitted to execution |
| `carport_driver_restarts_total` | Counter | Driver crashes and restarts, labelled by `instance_id` and `reason` |
| `gohome_api_requests_total` | Counter | Completed API RPCs, labelled by procedure and status code |
| `gohome_api_request_duration_seconds` | Histogram | RPC latency, labelled by procedure |
| `gohome_projector_lag_events` | Gauge | How many events each projector is behind head |
| `gohome_sqlite_wal_bytes` | Gauge | Current SQLite WAL file size |
| `gohome_build_info` | Gauge | Version, commit, and Go version — always 1 |

A full list of metrics is available at `/metrics` on a running daemon.

### Alerting recommendations

| Condition | Metric to watch |
|---|---|
| Driver keeps crashing | `carport_driver_restarts_total` rate > 0 sustained |
| Event store falling behind | `gohome_projector_lag_events` > 1000 for more than 60s |
| API latency spike | `gohome_api_request_duration_seconds` p99 > 500ms |
| Automation failures | `gohome_automation_runs_total{outcome="failed"}` rate > 0 |

### Grafana

A community Grafana dashboard is available in the `contrib/grafana/` directory of the gohome source repository. Import it by ID or upload the JSON directly.

## OpenTelemetry tracing

!!! status-wip "In development"
    Basic span instrumentation is in place. OTLP export is configurable but coverage is incomplete in the current release.

gohomed emits OpenTelemetry traces for the key request path: API call → event append → state update → driver dispatch. Configure the OTLP exporter with standard OpenTelemetry environment variables:

| Variable | Description |
|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP receiver endpoint (e.g. `http://localhost:4317`) |
| `OTEL_SERVICE_NAME` | Service name shown in traces (default: `gohomed`) |
| `OTEL_EXPORTER_OTLP_HEADERS` | Additional headers (e.g. for auth tokens) |

Example — send traces to a local Jaeger instance:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 \
OTEL_SERVICE_NAME=gohomed \
  gohomed
```

When `OTEL_EXPORTER_OTLP_ENDPOINT` is not set, tracing is a no-op with no performance overhead.

## `gohome diag`

`gohome diag` generates a redacted support bundle safe to share with maintainers or attach to a bug report:

```bash
gohome diag
# Writes: gohome-diag-20260427T143201.tar.gz
```

The bundle contains:

- gohomed and gohome version strings, Go version, commit hash
- Driver versions and Carport protocol versions for all installed drivers
- Last 500 log lines at `WARN` level and above (no `DEBUG` lines)
- Health snapshot from `/health`
- Recent error events from the event log (last 100, entity IDs preserved, attribute values redacted)
- `gohome config validate` output
- System info: OS, architecture, available memory and disk

The bundle does **not** contain:

- Pkl source files or any config content
- Event log payloads beyond the redacted error events
- Credentials, tokens, or secrets of any kind

Share the bundle by running:

```bash
gohome diag --upload
# Prints a short URL to the secure upload endpoint
```

Or attach the `.tar.gz` directly to a GitHub issue.
