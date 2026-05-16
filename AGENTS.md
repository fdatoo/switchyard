# Working in Switchyard

This is the canonical repo-onboarding doc. `CLAUDE.md` is a symlink to it, so
tools that look for `AGENTS.md` and tools that look for `CLAUDE.md` read the
same content.

If you are a human, this doc still applies.

## Setup

First run `task setup`. Skipping this means your commits will likely be
rejected. This activates the mandatory Git hooks via `core.hooksPath`.

A clean clone also needs Go 1.25+, Node.js 20+, Task, Buf, Pkl, and
golangci-lint. Standard commands:

| Action       | Command                 |
|--------------|-------------------------|
| Setup        | `task setup`            |
| Build        | `task build`            |
| Test         | `task test`             |
| Race test    | `task test:race`        |
| Integration  | `task test:integration` |
| Format       | `task fmt`              |
| Format check | `task fmt-check`        |
| Lint         | `task lint`             |
| Check        | `task check`            |
| Full check   | `task check:full`       |

Run `task check` before claiming broad repo work is done. Run
`task check:full` when touching runtime concurrency, integration surfaces, or
release-critical paths. For narrow changes, run the smallest relevant subset
and say exactly what passed.

## Repo Layout

```
Switchyard/
|-- AGENTS.md                  # this file (CLAUDE.md is a symlink to it)
|-- Taskfile.yml               # setup, build, test, fmt, lint, check
|-- app/                       # Vue admin UI
|-- cmd/                       # switchyardd, switchyard, and test binaries
|-- docs/                      # documentation site and long-lived notes
|-- docs/agents/               # agent-authored working docs
|   |-- specs/                 # design specs: what and why
|   `-- plans/                 # implementation plans: how
|-- docs/adrs/                 # architecture decision records
|-- drivers/                   # first-party out-of-process drivers
|-- examples/                  # sample config files and config fragments
|-- gen/                       # committed generated protobuf code
|-- internal/                  # main daemon and CLI internals
|-- proto/                     # protobuf API and event schemas
|-- switchyard-driverkit/      # public Go driver SDK module
`-- testdata/                  # golden fixtures and integration data
```

Rules:

- Product docs and long-lived architecture notes go in `docs/`.
- Agent-authored specs and plans go in `docs/agents/`.
- Cross-cutting durable decisions go in `docs/adrs/`.
- The root Go module stays at `.`. Do not move daemon code under a new top-level
  module directory.
- Do not add a new top-level directory without first updating this layout and
  explaining why the existing boundaries do not fit.

## Code Conventions

- **Go:** use Go 1.25+. Keep package APIs explicit and small. Prefer standard
  library types unless an existing local abstraction already owns the invariant.
- **Errors:** do not silence errors. Return explicit errors with useful context.
  Use sentinel or typed errors when callers need to branch. Do not string-match
  errors unless the dependency leaves no better option, and document that case.
- **Logging:** use structured logging. Log and return/propagate; logging is not
  a substitute for handling an error.
- **Config:** Pkl is the source config language. Generated config snapshots are
  protobuf artifacts. Config validation errors should include a stable code
  where practical, plus file/line/field context when known.
- **Proto:** preserve field numbers, reserve removed fields, and run
  `task proto` after schema changes. Generated code under `gen/` is committed.
- **Frontend:** keep UI code in `app/`. Use `task app:typecheck`,
  `task app:test`, and `task app:build` for app-only verification.
- **Comments:** explain constraints and surprising decisions. Do not narrate
  obvious control flow.

## Semantic Messages

Commit messages are semantic, one line maximum. No body. No trailers.

```
feat(config): add semantic apply messages
fix(app): preserve scene filter on reload
chore(ci): cache app dependencies
docs(agents): document repo workflow
```

- **Allowed prefixes:** `feat`, `fix`, `chore`, `refactor`, `test`, `docs`,
  `perf`, `build`.
- **Allowed scopes:** `app`, `api`, `auth`, `carport`, `cli`, `config`,
  `daemon`, `driverkit`, `drivers`, `eventstore`, `policy`, `proto`, `repo`,
  `ci`, `docs`, `agents`.
- **Subject:** imperative, lowercase, no trailing period.

Config apply messages use the same one-line shape and the `config` prefix:

```
config(scene): add evening kitchen scene
config(area): move office devices
config(driver): rotate hue bridge credentials
```

Allowed config scopes are `area`, `automation`, `driver`, `entity`, `page`,
`policy`, `scene`, `script`, `auth`, `mcp`, and `repo`.

Never include `Co-Authored-By:` trailers, generated-by footers, tool
watermarks, or agent attribution.

## Workflow Expectations

1. Read the issue or local spec first.
2. Write or update a spec under `docs/agents/specs/` before non-trivial design
   work.
3. Write or update a plan under `docs/agents/plans/` for multi-step execution.
4. Keep patches surgical. Do not refactor unrelated code.
5. Verify locally before claiming done. Treat local failures the same way CI
   will.
