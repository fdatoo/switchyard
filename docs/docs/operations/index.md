# Operations

!!! status-alpha "Alpha — shipped, interface evolving"
    Core operational tooling (daemon startup, lock file, metrics, structured logs) is shipped. Backup commands and OTEL tracing are still in progress.

This section covers running switchyard in production: how the daemon is deployed, how state is backed up and restored, how you keep the software current, and how you observe what the system is doing.

## What is in this section

### [Deployment](deployment.md)

Ports, data directory layout, config directory location, environment variables that control daemon behaviour, and the lock file that prevents double-start.

### [Backup & Restore](backup-restore.md)

What constitutes the full persistent state, how to create a consistent backup without stopping the daemon, how to restore, and how to move switchyard to a new server in one round-trip.

### [Updates](updates.md)

Update paths for bare-metal, Docker, and package-managed installs. How schema migrations run at startup, how event schema backward compatibility is maintained, and how to upgrade individual drivers without restarting the daemon.

### [Observability](observability.md)

Structured logging with `slog`, the Prometheus `/metrics` endpoint and the key metrics to watch, OpenTelemetry tracing export, and the `switchyard diag` support bundle command.
