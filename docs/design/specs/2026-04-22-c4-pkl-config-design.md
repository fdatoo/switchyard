# C4 — Pkl Schema & Config Lifecycle Design

**Milestone:** C4  
**Status:** Approved  
**Date:** 2026-04-22  
**Author:** Fynn Datoo

---

## 1. Scope

C4 adds the configuration system to gohome: the `gohome.*` Pkl schema modules users import into their config tree, and the `internal/config` Go pipeline that evaluates, validates, diffs, and applies those configs.

C4 lives entirely in the `github.com/fynn-labs/gohome` repo. No new repos.

**In scope:**
- Eight `gohome.*` Pkl modules embedded in the binary
- `PklProject.pkl` for LSP support
- `internal/config` package: evaluator, cross-reference compiler, diff engine, secret resolvers, manager
- New protos: `gohome.config.v1.ConfigSnapshot`, `gohome.event.v1.ConfigApplied`
- CLI: `gohome config validate`, `gohome config apply [--dry-run]`
- Daemon startup config load (fatal on invalid config)

**Out of scope (explicitly deferred):**
- Starlark runtime and syntax validation (C5) — see DR-6
- Driver manifest Pkl evaluation (post-C4) — see DR-7
- Daemon-proxied LSP for web editor (C10)
- Automation/script engine (C6)
- Connect-RPC `ConfigService` (C7)

---

## 2. Background

The master design (§6) establishes Pkl as the sole config language for gohome. Users declare driver instances, entities, automations, dashboards, auth, and policies in `.pkl` files under a config directory. The daemon evaluates this tree on startup and on `gohome config apply`, producing a typed `ConfigSnapshot`; changes are applied minimally (only affected driver instances are restarted) and recorded as `ConfigApplied` events.

C1 built the event store. C2 built the Carport protocol and driver supervisor. C3 built the driver SDK. C4 wires the config pipeline that feeds driver instance configuration into C2's carport supervisor.

---

## 3. Architecture

### 3.1 Evaluation pipeline

```
main.pkl  (user config dir)
    └─► pkl-go EvaluatorManager
            + gohome:* ExternalReader (embedded FS)
            └─► ConfigSnapshot proto
                    └─► config.Compile()   — cross-reference validation
                            └─► config.Diff()   — vs. current snapshot
                                    └─► ResolveSecrets()   — env/file/keyring
                                            └─► apply side-effects (carport)
                                                    └─► eventstore.Append(ConfigApplied)
```

### 3.2 `Manager`

`Manager` is a standalone struct owned by `gohomed`. It holds:
- `configDir string`
- `current *configpb.ConfigSnapshot` (guarded by `sync.RWMutex`)
- `store eventstore.Store`
- `carportMgr *carport.Manager`
- `evaluator *evaluator` (internal)

Public API:
```go
// Validate evaluates and cross-ref-validates config. No side-effects.
func (m *Manager) Validate(ctx context.Context) (*configpb.ConfigSnapshot, *ConfigDiff, error)

// Apply runs Validate, resolves secrets, applies the diff, and appends ConfigApplied.
// If dryRun is true, stops after diff — no secrets resolved, no events appended.
func (m *Manager) Apply(ctx context.Context, dryRun bool) error

// Current returns the most-recently-applied snapshot. Nil before first Apply.
func (m *Manager) Current() *configpb.ConfigSnapshot
```

### 3.3 Pkl module resolution

The `gohome.*` Pkl modules are embedded in the binary via `embed.FS`. pkl-go's `ExternalReader` API serves them under the `gohome` URI scheme. Users write:

```pkl
import "gohome:base" as base
import "gohome:entities" as entities
```

The ExternalReader resolves `gohome:base` → `pkl/gohome/base.pkl` from the embedded FS. No temp-dir extraction; no filesystem side-effects at startup.

### 3.4 Event integration

`ConfigApplied` is appended to the event store as the committed record of every successful `Apply`. Other components (C5, C6) react to it via their own eventstore subscriptions — consistent with how registry and carport work. The full `ConfigSnapshot` is never stored in the event; `Manager.Current()` is the read path.

---

## 4. File map

### New files

| Path | Responsibility |
|---|---|
| `pkl/PklProject.pkl` | LSP project descriptor; declares evaluator settings and module root |
| `pkl/gohome/base.pkl` | `Secret` interface + `env:`, `file:`, `keyring:` implementations; `Metadata`; `RetentionPolicy` |
| `pkl/gohome/carport.pkl` | `DriverManifest`, `DriverInstance` base config class |
| `pkl/gohome/entities.pkl` | `Entity`, `Light`, `Thermostat`, `Switch`, `Sensor`, `BinarySensor` |
| `pkl/gohome/automations.pkl` | `Automation`, `Trigger`, `Condition`, `Action`; Starlark fields are plain `String` stubs (DR-6) |
| `pkl/gohome/dashboards.pkl` | `Dashboard`, `Page`, `Grid`, `WidgetInstance` |
| `pkl/gohome/widgets.pkl` | `Gauge`, `LineChart`, `EntityToggle`, `Markdown`, `ScriptButton` |
| `pkl/gohome/auth.pkl` | `User`, `Role`, `Policy` |
| `pkl/gohome/starlark.pkl` | `StarlarkExpr`, `StarlarkScript`, `StarlarkCondition` — plain `String` type aliases (DR-6) |
| `proto/gohome/config/v1/snapshot.proto` | `ConfigSnapshot` and sub-messages |
| `internal/config/evaluator.go` | pkl-go wrapper; ExternalReader for `gohome:*`; `Evaluate(ctx, dir) (*configpb.ConfigSnapshot, error)` |
| `internal/config/compile.go` | `Compile(snapshot, querier) []ValidationError` — cross-reference checks |
| `internal/config/diff.go` | `Diff(old, new *configpb.ConfigSnapshot) *ConfigDiff` |
| `internal/config/secrets.go` | `ResolveSecrets(ctx, snapshot) error` — `env:`, `file:`, `keyring:` resolvers |
| `internal/config/manager.go` | `Manager` struct; `Validate`, `Apply`, `Current` |
| `internal/config/errors.go` | `EvalError`, `ValidationError` types |
| `internal/config/testdata/valid/main.pkl` | Fixture: minimal valid config (1 driver instance, 1 entity, 1 automation stub) |
| `internal/config/testdata/invalid-xref/main.pkl` | Fixture: config with a broken entity cross-reference |

### Modified files

| Path | Change |
|---|---|
| `proto/gohome/event/v1/event.proto` | Add `ConfigApplied config_applied = 12` to `Payload` oneof; add `ConfigApplied` message |
| `internal/cli/config.go` | New file: `config validate` and `config apply` Cobra subcommands |
| `internal/cli/root.go` | Register `config` command group |
| `cmd/gohomed/main.go` | Instantiate `config.Manager`; call `Apply` on startup; fatal on error |

---

## 5. Pkl modules

### 5.1 `gohome:base`

```pkl
module gohome.base

abstract class Secret {}
class EnvSecret extends Secret   { variable: String }    // env:FOO
class FileSecret extends Secret  { path: String }        // file:/run/secrets/foo
class KeyringSecret extends Secret { service: String; user: String }  // keyring:svc/user

class Metadata {
  name: String
  labels: Mapping<String, String> = new {}
}

class RetentionPolicy {
  maxAgeDays: Int?
  maxBytes: Int?
}
```

### 5.2 `gohome:carport`

```pkl
module gohome.carport

abstract class DriverInstance {
  driverName: String
  id: String
}
```

Drivers extend `DriverInstance` with their own typed fields. The `DriverManifest` Pkl class is reserved here for C5+ when `pkl_module` bytes are evaluated (DR-7).

### 5.3 `gohome:entities`

```pkl
module gohome.entities

abstract class Entity {
  id: String          // dotted-path: "light.living_room"
  friendlyName: String
  area: String?
}

class Light extends Entity {
  supportsBrightness: Boolean = false
  supportsColorTemp: Boolean  = false
}

class Thermostat extends Entity {
  minTemp: Float = 10.0
  maxTemp: Float = 35.0
}

class Switch extends Entity {}
class Sensor extends Entity { unit: String? }
class BinarySensor extends Entity {}
```

### 5.4 `gohome:automations`

```pkl
module gohome.automations
import "gohome:starlark" as starlark

class Trigger {
  kind: String           // "state_changed" | "time" | "event" | "webhook"
  condition: String?     // starlark.StarlarkCondition stub
}

class Action {
  kind: String           // "call_service" | "scene" | "script" | "starlark"
  body: String?          // starlark.StarlarkScript stub
}

class Automation {
  id: String
  trigger: Trigger
  actions: Listing<Action>
  enabled: Boolean = true
}
```

### 5.5 `gohome:dashboards`

```pkl
module gohome.dashboards

class WidgetInstance {
  widgetClass: String
  props: Mapping<String, Any>
  col: Int; row: Int; w: Int; h: Int
}

class Grid { widgets: Listing<WidgetInstance> }
class Page { title: String; grid: Grid }
class Dashboard { slug: String; pages: Listing<Page> }
```

### 5.6 `gohome:widgets`

```pkl
module gohome.widgets
// Standard widget class name constants referenced by WidgetInstance.widgetClass
const gauge:        String = "Gauge"
const lineChart:    String = "LineChart"
const entityToggle: String = "EntityToggle"
const markdown:     String = "Markdown"
const scriptButton: String = "ScriptButton"
```

### 5.7 `gohome:auth`

```pkl
module gohome.auth

class User {
  slug: String
  displayName: String
  roles: Listing<String>
  active: Boolean = true
}

class Role {
  slug: String
  permissions: Listing<String>
}

class Policy {
  role: String
  resource: String
  actions: Listing<String>
}
```

### 5.8 `gohome:starlark` (stub)

```pkl
module gohome.starlark

// Starlark snippet type aliases — plain String in C4.
// C5 activates isValidStarlark*() validator hooks via ExternalReader (see DR-6).
typealias StarlarkExpr      = String
typealias StarlarkScript    = String
typealias StarlarkCondition = String
```

---

## 6. Proto definitions

### 6.1 `proto/gohome/config/v1/snapshot.proto`

```protobuf
syntax = "proto3";
package gohome.config.v1;

message ConfigSnapshot {
  int64  evaluated_at_unix_ms = 1;
  string config_dir           = 2;

  repeated DriverInstanceConfig driver_instances = 10;
  repeated EntityConfig         entities         = 11;
  repeated AutomationConfig     automations      = 12;
  repeated DashboardConfig      dashboards       = 13;
  repeated UserConfig           users            = 14;
  repeated RoleConfig           roles            = 15;
  repeated PolicyConfig         policies         = 16;
}

message DriverInstanceConfig {
  string id          = 1;
  string driver_name = 2;
  bytes  config_hash = 3;   // sha256 of marshalled params; used for diff
  bytes  params      = 4;   // opaque marshalled config blob
}

message EntityConfig {
  string id            = 1;
  string friendly_name = 2;
  string entity_type   = 3;
  string area          = 4;
}

message AutomationConfig {
  string id      = 1;
  bytes  content = 2;   // serialised automation; opaque until C6
  bool   enabled = 3;
}

message DashboardConfig {
  string slug    = 1;
  bytes  content = 2;   // serialised dashboard; opaque until C10
}

message UserConfig   { string slug = 1; string display_name = 2; repeated string roles = 3; bool active = 4; }
message RoleConfig   { string slug = 1; repeated string permissions = 2; }
message PolicyConfig { string role = 1; string resource = 2; repeated string actions = 3; }
```

### 6.2 `proto/gohome/event/v1/event.proto` additions

Add to `Payload` oneof:
```protobuf
ConfigApplied config_applied = 12;
```

New message:
```protobuf
message ConfigApplied {
  int64 applied_at_unix_ms       = 1;
  int32 driver_instances_added   = 2;
  int32 driver_instances_removed = 3;
  int32 driver_instances_changed = 4;
  int32 automations_changed      = 5;
  bool  dry_run                  = 6;
}
```

---

## 7. `internal/config` package

### 7.1 `evaluator.go`

```go
package config

import (
    "context"
    "embed"

    "github.com/apple/pkl-go/pkl"
    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

//go:embed ../../pkl
var pklFS embed.FS

type evaluator struct {
    manager pkl.EvaluatorManager
}

func newEvaluator() (*evaluator, error) {
    mgr, err := pkl.NewEvaluatorManager(pkl.WithExternalReader(
        "gohome", gohomeReader{fs: pklFS},
    ))
    if err != nil {
        return nil, err
    }
    return &evaluator{manager: mgr}, nil
}

// Evaluate evaluates configDir/main.pkl and returns the ConfigSnapshot.
func (e *evaluator) Evaluate(ctx context.Context, configDir string) (*configpb.ConfigSnapshot, error)

// EvalError carries structured Pkl evaluation error information.
type EvalError struct {
    File    string
    Line    int
    Column  int
    Message string
}
```

`gohomeReader` implements `pkl.ExternalReader` by serving entries from `pklFS` under `pkl/gohome/`.

### 7.2 `compile.go`

```go
// Compile validates cross-references in a snapshot.
// querier is used to check driver names against the registry.
func Compile(snap *configpb.ConfigSnapshot, querier RegistryQuerier) []ValidationError

type ValidationError struct {
    Field   string
    Message string
}

type RegistryQuerier interface {
    DriverExists(name string) bool
}
```

Checks:
- Each `DriverInstanceConfig.driver_name` maps to a known driver in the registry
- Each `EntityConfig.area` (if set) is non-empty and well-formed (`"<type>.<name>"`)
- No duplicate `DriverInstanceConfig.id` or `EntityConfig.id` within the snapshot

### 7.3 `diff.go`

```go
type ConfigDiff struct {
    DriverInstancesAdded   []string   // driver instance IDs
    DriverInstancesRemoved []string
    DriverInstancesChanged []string
    AutomationsChanged     int
    DashboardsChanged      int
}

func Diff(old, next *configpb.ConfigSnapshot) *ConfigDiff
```

Driver instance diffing uses `config_hash` to detect changes without deserialising `params`.

### 7.4 `secrets.go`

```go
// ResolveSecrets resolves all Secret references in the snapshot in-place.
// Resolved values are never stored in the event log.
func ResolveSecrets(ctx context.Context, snap *configpb.ConfigSnapshot) error

// Keyring is the interface satisfied by go-keyring and by test doubles.
type Keyring interface {
    Get(service, user string) (string, error)
}
```

URI prefix dispatch: `env:` → `os.Getenv`, `file:` → `os.ReadFile`, `keyring:` → `Keyring.Get`.

### 7.5 `manager.go`

```go
type Manager struct {
    configDir   string
    ev          *evaluator
    store       eventstore.Store
    carportMgr  CarportManager
    mu          sync.RWMutex
    current     *configpb.ConfigSnapshot
}

type CarportManager interface {
    RegisterInstance(ctx context.Context, id, driverName string, params []byte) error
    UnregisterInstance(ctx context.Context, id string) error
}

func NewManager(configDir string, store eventstore.Store, carportMgr CarportManager) (*Manager, error)
func (m *Manager) Validate(ctx context.Context) (*configpb.ConfigSnapshot, *ConfigDiff, error)
func (m *Manager) Apply(ctx context.Context, dryRun bool) error
func (m *Manager) Current() *configpb.ConfigSnapshot
```

---

## 8. CLI

Both commands live in `internal/cli/config.go` and are registered on a `configCmd` parent command in `internal/cli/root.go`. Both accept `--config-dir` (default: `$GOHOME_CONFIG_DIR` → `$HOME/.config/gohome`).

### `gohome config validate`

Calls `Manager.Validate`. On success prints:

```
✓ Config valid
  Driver instances : 2
  Entities         : 14
  Automations      : 3
  Dashboards       : 1
```

On failure prints each `ValidationError` / `EvalError` (lipgloss red), one per line, with file path and line number where available. Exit 1.

### `gohome config apply [--dry-run]`

Calls `Manager.Apply(ctx, dryRun)`. Prints the diff table:

```
Driver instances   +2  -0  ~1
Automations        +0  -0  ~3
Dashboards         +1  -0  ~0
```

With `--dry-run`: prints diff, exits 0, no events appended, no carport calls made.

---

## 9. Secret sources

| URI prefix | Resolver | Example |
|---|---|---|
| `env:` | `os.Getenv(variable)` | `env:HUE_API_KEY` |
| `file:` | `os.ReadFile(path)` → trim whitespace | `file:/run/secrets/hue_key` |
| `keyring:` | `go-keyring` `Get(service, user)` | `keyring:gohome/hue_api_key` |

Pkl's `Secret` subclasses serialise to tagged strings in the evaluator output: `"__secret__:env:FOO"`, `"__secret__:file:/run/secrets/foo"`, `"__secret__:keyring:svc/user"`. `ResolveSecrets` walks the `ConfigSnapshot` proto, detects these tags, resolves them, and replaces the tagged string with the resolved value in-place. This keeps secret semantics entirely within Go — Pkl has no knowledge of resolution. Resolved values are never written to `ConfigApplied` or any event payload. If a secret cannot be resolved, `Apply` returns an error before any side-effects occur.

---

## 10. LSP support

`pkl/PklProject.pkl` at the repo root of the pkl directory declares the module root and evaluator settings:

```pkl
amends "pkl:Project"

evaluatorSettings {
  externalProperties {}
  modulePaths = List(".")
}
```

Users clone gohome and point their editor's `pkl.projectDir` setting at `pkl/`. `pkl-lsp` resolves `gohome:*` imports from the local `pkl/gohome/` directory, providing autocomplete and type errors for config files. No daemon involvement required.

---

## 11. Testing strategy

### Unit tests

- `compile_test.go` — table-driven; hand-crafted `ConfigSnapshot` values; assert correct `[]ValidationError`
- `diff_test.go` — pairs of snapshots; assert `ConfigDiff` fields
- `secrets_test.go` — table of URI strings; fake `os.Getenv` via env manipulation; stubbed `Keyring` interface; stubbed temp files
- `manager_test.go` — fake `evaluator`, fake `CarportManager`, fake `eventstore.Store`; assert `Apply` calls in correct order and appends correct event

### Integration tests (build tag `integration`)

Run via `task test:integration`. Require `pkl` binary on `PATH`.

- `evaluator_integration_test.go` — evaluates `testdata/valid/main.pkl`; asserts `ConfigSnapshot` fields
- `compile_integration_test.go` — evaluates `testdata/invalid-xref/main.pkl`; asserts the expected `ValidationError` is returned

### CLI tests

Use existing pattern: Cobra command invoked with a stubbed `*Manager` double. Assert stdout table and exit codes.

---

## 12. Daemon integration

In `cmd/gohomed/main.go`, after carport supervisor is ready:

```go
cfgMgr, err := config.NewManager(configDir, store, carportSupervisor)
if err != nil {
    slog.Error("config init failed", "err", err)
    os.Exit(1)
}
if err := cfgMgr.Apply(ctx, false); err != nil {
    slog.Error("config load failed", "err", err)
    os.Exit(1)
}
```

Config load failure is fatal — the daemon must not run with an invalid or unapplied config.

---

## 13. Decision record

| # | Decision | Rationale |
|---|---|---|
| DR-1 | pkl-go library (`github.com/apple/pkl-go`) over CLI subprocess | Clean Go API; evaluator process lifecycle managed automatically; intended Go integration point |
| DR-2 | LSP via `PklProject.pkl` only — no daemon proxy | Minimum useful thing for C4; daemon-proxied LSP belongs in C10 when Monaco editor is built |
| DR-3 | `gohome:*` ExternalReader over temp-dir extraction | No filesystem side-effects at startup; binary is self-contained |
| DR-4 | `ConfigApplied` carries diff metadata only, not full snapshot | Snapshot is large and in-memory; event log is for audit; `Manager.Current()` is the read path |
| DR-5 | `ResolveSecrets` mutates snapshot in-place before side-effects, never persisted | Secrets must not appear in the event store or diffs |
| DR-6 | **`gohome.starlark` typealiases are plain `String` in C4 — deferred to C5** | Keeps C4 free of `go.starlark.net` dependency. **C5 planner:** activate `isValidStarlark*()` validators in `starlark.pkl` and wire the parse-only ExternalReader hook in `evaluator.go` using `go.starlark.net/syntax`. |
| DR-7 | **Driver manifest Pkl evaluation deferred to post-C4** | `pkl_module` bytes stored in registry as-is (C2). Shape of instance config typing unclear until C6/C7. **Future planner:** add `EvaluateManifest(ctx, []byte) (*DriverManifestSchema, error)` to `evaluator.go`; the ExternalReader infrastructure is already in place. |

---

## 14. Task breakdown

1. **Proto additions** — add `ConfigApplied` to `event.proto`; create `proto/gohome/config/v1/snapshot.proto`; run `task proto`
2. **Pkl module scaffolding** — create `pkl/PklProject.pkl` and all eight `pkl/gohome/*.pkl` files
3. **`internal/config/evaluator.go`** — pkl-go wrapper + ExternalReader; unit test with fake FS
4. **`internal/config/compile.go`** — cross-reference validator; unit tests
5. **`internal/config/diff.go`** — diff engine; unit tests
6. **`internal/config/secrets.go`** — three secret resolvers; unit tests
7. **`internal/config/manager.go`** — `Manager` struct + `Validate`/`Apply`/`Current`; unit tests with all fakes
8. **Integration tests** — fixture config dirs; evaluator integration tests (build tag `integration`)
9. **CLI** — `gohome config validate` and `gohome config apply`; CLI tests
10. **Daemon wiring** — `cmd/gohomed/main.go` startup config load
11. **Definition of done** — `task build`, `task test`, `task test:race`, `task test:integration`, `task lint`, `go mod tidy`
