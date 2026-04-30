# GoHome

GoHome is a self-hosted home automation platform. `gohomed` runs on your local network and connects to devices via a driver model; `gohome` is the CLI for managing it.

## Modules

| Module | Path | Description |
|--------|------|-------------|
| `github.com/fdatoo/gohome` | `.` | Daemon (`gohomed`) + CLI (`gohome`) |
| `github.com/fdatoo/gohome-driverkit` | `./gohome-driverkit` | SDK for building device drivers |

Both modules are linked by a Go workspace (`go.work`), so `go build ./...` and `go test ./...` work across the full tree from the repo root.

## Prerequisites

- Go 1.25+
- [Task](https://taskfile.dev) — `brew install go-task`
- [buf](https://buf.build) — `brew install bufbuild/buf/buf` (proto codegen)
- [Pkl](https://pkl-lang.org) — `brew install pkl` (config schema)
- Node.js 20+ — for the web UI

## Building

```bash
task build          # builds gohomed + gohome binaries into dist/
task web:build      # builds the web UI (required before task build)
```

## Testing

```bash
task test           # unit tests
task test:race      # race detector
task test:integration   # integration tests (real disk I/O)
```

## Drivers

Drivers are out-of-process binaries that implement the driver gRPC protocol. Use the [gohome-driverkit](./gohome-driverkit) to build one:

```bash
cd gohome-driverkit
go build ./...
```

## Documentation

Full documentation lives in [`docs/`](./docs) and is published via Zensical. Design specs and implementation plans live in [`docs/design/`](./docs/design).

## License

[MIT](./LICENSE)
