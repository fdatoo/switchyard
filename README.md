<picture>
  <source media="(prefers-color-scheme: dark)" srcset="media/banner-dark.png">
  <img src="media/banner-light.png" alt="FBI" width="460" />
</picture>

Switchyard is a self-hosted home automation platform. `switchyardd` runs on your local network and connects to devices via a driver model; `switchyard` is the CLI for managing it.

> **Warning**: This project is in early development. It's definitely not ready for production use, and the API is likely to change.

## Modules

| Module | Path | Description |
|--------|------|-------------|
| `github.com/fdatoo/switchyard` | `.` | Daemon (`switchyardd`) + CLI (`switchyard`) |
| `github.com/fdatoo/switchyard-driverkit` | `./switchyard-driverkit` | SDK for building device drivers |

Both modules are linked by a Go workspace (`go.work`), so `go build ./...` and `go test ./...` work across the full tree from the repo root.

## Prerequisites

- Go 1.25+
- [Task](https://taskfile.dev) — `brew install go-task`
- [buf](https://buf.build) — `brew install bufbuild/buf/buf` (proto codegen)
- [Pkl](https://pkl-lang.org) — `brew install pkl` (config schema)
- Node.js 20+ — for the web UI

## Setup

```bash
task setup          # activates repo Git hooks
```

## Building

```bash
task build          # builds switchyardd + switchyard binaries into dist/
task app:build      # builds the web UI
```

## Testing

```bash
task test           # unit tests
task test:race      # race detector
task test:integration   # integration tests (real disk I/O)
task check          # standard local verification suite
task check:full     # standard checks plus race and integration tests
```

## Drivers

Drivers are out-of-process binaries that implement the driver gRPC protocol. Use the [switchyard-driverkit](./switchyard-driverkit) to build one:

```bash
cd switchyard-driverkit
go build ./...
```

## Documentation

Full documentation lives in [`docs/`](./docs) and is published via Zensical.
Agent-authored specs and implementation plans live in
[`docs/agents/`](./docs/agents). Cross-cutting decisions live in
[`docs/adrs/`](./docs/adrs).

## License

[MIT](./LICENSE)
