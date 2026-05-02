# Switchyard — Monorepo

## Directory map

| Path | Module | Purpose |
|------|--------|---------|
| `.` | `github.com/fdatoo/switchyard` | `switchyardd` daemon + `switchyard` CLI |
| `switchyard-driverkit/` | `github.com/fdatoo/switchyard-driverkit` | Driver development kit |
| `docs/` | — | Documentation site (Zensical) |
| `docs/design/` | — | Design specs and implementation plans |
| `dev/` | — | Internal developer notes (proto hygiene, setup guides) |

## Go workspace

`go.work` at the repo root links `.` and `./switchyard-driverkit`. Use standard `go` commands from anywhere in the tree; the workspace resolves both modules locally.

## Rules

- **Documentation and design specs live in `docs/design/`**, not scattered in the source tree.
- **Never create a new top-level directory** without checking with the user first.
- The `github.com/fdatoo/switchyard` module root is the repo root (`.`), not a subdirectory.
