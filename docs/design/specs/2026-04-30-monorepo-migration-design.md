# Monorepo Migration Design

**Date:** 2026-04-30
**Status:** Approved

## Context

GoHome is currently a container directory holding three git submodules:

| Submodule path | Remote | Purpose |
|---|---|---|
| `gohome/` | `fynn-labs/gohome` | Go module: `gohomed` daemon + `gohome` CLI |
| `gohome-driverkit/` | `fynn-labs/gohome-driverkit` | Driver development kit |
| `docs/` | `fynn-labs/gohome-docs` | Documentation site |

`gohome-driverkit` already references `gohome` via a `replace` directive (`replace github.com/fynn-labs/gohome => ../gohome`), and its CI must check out `gohome` as a sibling repo to satisfy it. The submodule model provides little practical isolation benefit given this coupling.

## Goal

Collapse all three submodules into a single monorepo at `~/Developer/GoHome`, backed by `fdatoo/gohome` on GitHub. Old repos (`fynn-labs/gohome`, `fynn-labs/gohome-driverkit`, `fynn-labs/gohome-docs`) are deleted after migration. Git history is not preserved (fresh start).

## Directory Structure

```
~/Developer/GoHome/
├── .github/
│   └── workflows/
│       ├── ci.yml          # unified Go CI (gohome + driverkit, path-filtered)
│       └── docs.yml        # docs deploy
├── go.work                 # Go workspace linking ./gohome and ./gohome-driverkit
├── gohome/                 # module: github.com/fdatoo/gohome
│   └── ... (internals unchanged)
├── gohome-driverkit/       # module: github.com/fdatoo/gohome-driverkit
│   └── ... (internals unchanged)
└── docs/                   # was gohome-docs/ submodule
    ├── design/             # was docs/ inside gohome-docs/
    │   ├── ai-agents/
    │   ├── automations/
    │   └── ...
    ├── site/
    └── zensical.toml
```

The container-level `CLAUDE.md` and `README.md` carry over. `.gitmodules` is dropped entirely.

## Go Module Changes

### Module path renames

All occurrences of the old import paths are rewritten:

- `github.com/fynn-labs/gohome` → `github.com/fdatoo/gohome`
- `github.com/fynn-labs/gohome-driverkit` → `github.com/fdatoo/gohome-driverkit`

This touches every `.go` file that imports from either module, plus both `go.mod` files.

### Replace directive removal

The `replace github.com/fynn-labs/gohome => ../gohome` directive in `gohome-driverkit/go.mod` is removed. The `go.work` file makes it redundant.

### go.work

```
go 1.25.0

use (
    ./gohome
    ./gohome-driverkit
)
```

## CI Consolidation

### Before

| Workflow | Repo | Notes |
|---|---|---|
| `gohome/.github/workflows/ci.yml` | `fynn-labs/gohome` | Full pipeline: build, test, race, integration, lint, coverage, proto, fuzz |
| `gohome-driverkit/.github/workflows/ci.yml` | `fynn-labs/gohome-driverkit` | Checks out `fynn-labs/gohome` as sibling using `GOHOME_REPO_TOKEN` |
| `gohome-docs/.github/workflows/docs.yml` | `fynn-labs/gohome-docs` | Zensical build + GitHub Pages deploy |

### After

**`.github/workflows/ci.yml`** — unified Go CI with path filters:

- `gohome/**` or `go.work` changed → runs all existing gohome jobs unchanged (build, test, race, integration, lint, coverage, proto hygiene, fuzz-scheduled)
- `gohome-driverkit/**` or `go.work` changed → runs driverkit jobs (build, test, race, integration, lint); sibling-checkout step and `GOHOME_REPO_TOKEN` removed

**`.github/workflows/docs.yml`** — unchanged logic, triggered on `docs/**` changes.

## Migration Phases

Steps 1–6 are fully local and reversible. GitHub operations begin only after a clean local build.

1. **Create `~/Developer/GoHome/`** — copy files from each submodule into the target structure; nothing deleted yet
2. **Rename module paths** — bulk find/replace across all `.go` and `go.mod` files
3. **Set up `go.work`** — create workspace file, run `go work sync`
4. **Verify locally** — `go build ./...` and `go test ./...` in both modules; `zensical build` in `docs/`
5. **Write CI** — new `ci.yml` and `docs.yml` into `.github/workflows/`
6. **`git init` + first commit** — single commit with everything
7. **GitHub: repurpose `fdatoo/gohome`** — inspect current contents, then push monorepo as new remote
8. **GitHub: delete old repos** — `fynn-labs/gohome`, `fynn-labs/gohome-driverkit`, `fynn-labs/gohome-docs`
9. **Update `CLAUDE.md`** — revise submodule map to reflect monorepo layout
