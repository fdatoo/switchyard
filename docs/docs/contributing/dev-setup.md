# Dev Setup

This page covers everything needed to build, test, and run switchyard from source.

---

## Prerequisites

| Tool | Check | Notes |
|------|-------|-------|
| Go 1.22+ | `go version` | [golang.org/dl](https://golang.org/dl) |
| Pkl CLI | `pkl --version` | [pkl-lang.org](https://pkl-lang.org) — required for config evaluation |
| buf | `buf --version` | [buf.build/docs/installation](https://buf.build/docs/installation) — protobuf toolchain, only needed if editing `.proto` files |
| task | `task --version` | `go install github.com/go-task/task/v3/cmd/task@latest` |
| golangci-lint | `golangci-lint --version` | Optional but recommended — the pre-commit hook skips lint if it is not installed |

---

## Clone and build

```bash
git clone https://github.com/fynn-labs/switchyard
cd switchyard
task build      # compiles switchyardd and switchyard binaries to dist/
```

All task targets:

| Command | Purpose |
|---------|---------|
| `task build` | Build both binaries (`dist/switchyardd`, `dist/switchyard`) |
| `task test` | Run unit tests |
| `task test:race` | Run unit tests with the race detector |
| `task test:integration` | Run integration tests (real disk I/O, requires `-tags=integration`) |
| `task test:fuzz` | Run fuzz targets briefly (30 s each) |
| `task lint` | Run `golangci-lint` |
| `task proto` | Regenerate `gen/` from `.proto` files via `buf generate` |
| `task tidy` | Run `go mod tidy` |

---

## Running tests

```bash
task test               # unit tests — fast, no external requirements
task test:race          # same tests with race detector — slower but catches data races
task test:integration   # integration tests — reads/writes real disk, uses -tags=integration
```

Integration tests are isolated to temp directories and clean up after themselves. They do not require network access or a running daemon.

---

## Pre-commit hooks

Install the git hooks once after cloning:

```bash
scripts/install-hooks.sh
```

This symlinks `scripts/hooks/pre-commit` and `scripts/hooks/pre-push` into `.git/hooks/`. The pre-commit hook runs:

1. `go mod tidy` — fails if `go.mod`/`go.sum` drift
2. `go build ./...` — native build
3. `go build ./...` for `darwin/arm64` — cross-compile check
4. `golangci-lint run ./...` — skipped if `golangci-lint` is not installed
5. `TestEvaluator_ValidConfig` — Pkl schema evaluation against the testdata config

---

## Running the daemon locally

Copy a starter config and launch the daemon:

```bash
cp examples/minimal-main.pkl ~/.config/switchyard/main.pkl
./dist/switchyardd
```

`switchyardd` reads its config from `~/.config/switchyard/` by default. Edit `main.pkl` and send `SIGHUP` to reload config without restarting:

```bash
kill -HUP "$(pidof switchyardd)"
```

---

## Regenerating protobuf code

Only needed if you edit `.proto` files under `proto/`:

```bash
task proto   # runs buf generate, writes output to gen/
```

The generated code in `gen/` is committed to git. Run `task proto` after any `.proto` change and commit the result alongside your source changes.

---

## Definition of done

Before opening a PR, all of the following must pass with no errors:

- [ ] `task build`
- [ ] `task test`
- [ ] `task test:race`
- [ ] `task test:integration`
- [ ] `task lint`
- [ ] `task tidy` — `go.mod` and `go.sum` must be clean
- [ ] If any `.proto` file changed: `task proto` was run and `gen/` changes are staged
