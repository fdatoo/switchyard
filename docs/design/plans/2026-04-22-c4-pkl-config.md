# C4 — Pkl Schema & Config Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Pkl-based configuration pipeline to gohome: `gohome.*` Pkl schema modules embedded in the binary, an `internal/config` package that evaluates/validates/diffs/applies configs, new protos for `ConfigSnapshot` and `ConfigApplied`, and `gohome config validate/apply` CLI commands.

**Architecture:** The user writes a `main.pkl` config file that amends the `gohome:config` root module. `config.Manager` evaluates it via pkl-go (outputting JSON), validates cross-references, computes a diff against the current snapshot, resolves secrets, applies carport side-effects, and appends a `ConfigApplied` event. `Manager` is a standalone struct that other packages observe via the event store — consistent with how `registry` and `carport` work.

**Tech Stack:** Go 1.25, `github.com/apple/pkl-go` (pkl evaluator client), `github.com/zalando/go-keyring` (system keyring), `google.golang.org/protobuf`, `github.com/spf13/cobra`, `github.com/charmbracelet/lipgloss`, existing `internal/eventstore`, `internal/carport`.

---

## Codebase orientation

Before starting, read these files to understand existing patterns:

| File | Why |
|---|---|
| `internal/daemon/daemon.go` | Where config.Manager will be wired (after carport, before Phase 5) |
| `internal/carport/config.go` | `carport.Instance` shape — C4 will read driver instances from Pkl instead of TOML |
| `internal/cli/driver.go` | CLI command pattern to follow for `config validate/apply` |
| `internal/cli/root.go` | Where to register `newConfigCmd` |
| `internal/cli/styles.go` | `Success`, `Error`, `Dim` styles for output |
| `proto/gohome/event/v1/event.proto` | Current `Payload` oneof — `ConfigApplied` goes at field 12 |
| `buf.gen.yaml` | Proto generation config — no changes needed, new file is auto-discovered |

---

## File map

### New files

| Path | Responsibility |
|---|---|
| `proto/gohome/config/v1/snapshot.proto` | `ConfigSnapshot` and sub-messages |
| `internal/config/pkl/PklProject.pkl` | LSP project descriptor |
| `internal/config/pkl/gohome/config.pkl` | Root module; `main.pkl` amends this; outputs JSON |
| `internal/config/pkl/gohome/base.pkl` | `Secret` typealias + tagged-string sub-aliases; `Metadata`; `RetentionPolicy` |
| `internal/config/pkl/gohome/carport.pkl` | `DriverInstance` base class |
| `internal/config/pkl/gohome/entities.pkl` | `Entity`, `Light`, `Thermostat`, `Switch`, `Sensor`, `BinarySensor` |
| `internal/config/pkl/gohome/automations.pkl` | `Automation`, `Trigger`, `Action` (Starlark fields are plain `String` stubs) |
| `internal/config/pkl/gohome/dashboards.pkl` | `Dashboard`, `Page`, `Grid`, `WidgetInstance` |
| `internal/config/pkl/gohome/widgets.pkl` | Widget class name constants |
| `internal/config/pkl/gohome/auth.pkl` | `User`, `Role`, `Policy` |
| `internal/config/pkl/gohome/starlark.pkl` | `StarlarkExpr/Script/Condition` as plain `String` stubs |
| `internal/config/errors.go` | `EvalError`, `ValidationError` types |
| `internal/config/evaluator.go` | pkl-go wrapper + `gohome:` ModuleReader; `Evaluate(ctx, dir)` |
| `internal/config/compile.go` | `Compile(snap, querier)` cross-reference validation |
| `internal/config/diff.go` | `Diff(old, new)` → `*ConfigDiff` |
| `internal/config/secrets.go` | `ResolveSecrets(ctx, snap, kr)` — env/file/keyring resolvers |
| `internal/config/manager.go` | `Manager` struct + `Validate`/`Apply`/`Current` |
| `internal/config/evaluator_test.go` | Unit test with fake `configEvaluator` (interface) |
| `internal/config/compile_test.go` | Table-driven cross-ref validation tests |
| `internal/config/diff_test.go` | Diff engine tests |
| `internal/config/secrets_test.go` | Secret resolver tests |
| `internal/config/manager_test.go` | Manager unit tests with all fakes |
| `internal/config/testdata/valid/main.pkl` | Fixture: 1 driver instance, 1 entity |
| `internal/config/testdata/invalid-xref/main.pkl` | Fixture: broken entity cross-reference |
| `internal/config/evaluator_integration_test.go` | Integration tests (build tag `integration`) |
| `internal/cli/config.go` | `gohome config validate` + `gohome config apply` commands |

### Modified files

| Path | Change |
|---|---|
| `proto/gohome/event/v1/event.proto` | Add `ConfigApplied config_applied = 12`; add `ConfigApplied` message |
| `internal/cli/root.go` | Register `newConfigCmd` |
| `internal/daemon/daemon.go` | Add `config *config.Manager` field; wire after carport (Phase 4.6) |

---

## Task 1: Proto additions and code generation

**Files:**
- Create: `proto/gohome/config/v1/snapshot.proto`
- Modify: `proto/gohome/event/v1/event.proto`

- [ ] **Step 1: Create the snapshot proto**

```
mkdir -p /path/to/gohome/proto/gohome/config/v1
```

Create `proto/gohome/config/v1/snapshot.proto`:

```protobuf
// See docs/proto-hygiene.md for grouping conventions.

syntax = "proto3";

package gohome.config.v1;

message ConfigSnapshot {
  // 1-9: meta
  int64  evaluated_at_unix_ms = 1;
  string config_dir           = 2;

  // 10-19: contents
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
  bytes  config_hash = 3;   // sha256 of params; used for diff without deserialising
  bytes  params      = 4;   // full JSON blob of the Pkl DriverInstance object
}

message EntityConfig {
  string id            = 1;
  string friendly_name = 2;
  string entity_type   = 3;
  string area          = 4;
}

message AutomationConfig {
  string id      = 1;
  bytes  content = 2;   // opaque JSON until C6
  bool   enabled = 3;
}

message DashboardConfig {
  string slug    = 1;
  bytes  content = 2;   // opaque JSON until C10
}

message UserConfig {
  string          slug         = 1;
  string          display_name = 2;
  repeated string roles        = 3;
  bool            active       = 4;
}

message RoleConfig {
  string          slug        = 1;
  repeated string permissions = 2;
}

message PolicyConfig {
  string          role     = 1;
  string          resource = 2;
  repeated string actions  = 3;
}
```

- [ ] **Step 2: Extend event.proto with ConfigApplied**

In `proto/gohome/event/v1/event.proto`, add to the `Payload` oneof (between `command_ack = 12` — wait, check existing numbering first):

Read the current `event.proto`. The current `Payload` oneof has:
- `system = 1`
- `state_changed = 10`
- `command_issued = 11`
- `command_ack = 12`
- `entity_registered = 20`
- `entity_unregistered = 21`
- `driver_event = 30`

Add `config_applied = 40` (use 40 to keep config-plane events in the 40-49 range, not 12 which conflicts with `command_ack`):

```protobuf
// Add inside the Payload oneof, after the driver-typed passthrough group:
    // 40-49: config plane
    ConfigApplied config_applied = 40;
```

And add the `ConfigApplied` message after the existing messages:

```protobuf
message ConfigApplied {
  // 1-9: meta / payload
  int64 applied_at_unix_ms       = 1;
  int32 driver_instances_added   = 2;
  int32 driver_instances_removed = 3;
  int32 driver_instances_changed = 4;
  int32 automations_changed      = 5;
  bool  dry_run                  = 6;
}
```

> **Note:** The master design shows field 12 for `config_applied` in the Payload oneof, but field 12 is already taken by `command_ack` in the current implementation. Use field 40 instead. This is intentional.

- [ ] **Step 3: Run proto generation and verify**

```bash
cd gohome && task proto
```

Expected: no errors. New files `gen/gohome/config/v1/snapshot.pb.go` and updated `gen/gohome/event/v1/event.pb.go` are created/updated.

```bash
task build
```

Expected: compiles cleanly.

- [ ] **Step 4: Commit**

```bash
git add proto/gohome/config/v1/snapshot.proto proto/gohome/event/v1/event.proto gen/
git commit -m "feat(c4): add ConfigSnapshot and ConfigApplied protos"
```

---

## Task 2: Pkl modules

**Files:**
- Create: `internal/config/pkl/PklProject.pkl`
- Create: `internal/config/pkl/gohome/config.pkl`
- Create: `internal/config/pkl/gohome/base.pkl`
- Create: `internal/config/pkl/gohome/carport.pkl`
- Create: `internal/config/pkl/gohome/entities.pkl`
- Create: `internal/config/pkl/gohome/automations.pkl`
- Create: `internal/config/pkl/gohome/dashboards.pkl`
- Create: `internal/config/pkl/gohome/widgets.pkl`
- Create: `internal/config/pkl/gohome/auth.pkl`
- Create: `internal/config/pkl/gohome/starlark.pkl`

- [ ] **Step 1: Create `PklProject.pkl`**

`internal/config/pkl/PklProject.pkl`:

```pkl
amends "pkl:Project"

evaluatorSettings {
  modulePaths = List(".")
}
```

> For LSP support: users point their editor's `pkl.projectDir` setting at `<repo>/internal/config/pkl/`. `pkl-lsp` then resolves `gohome:*` imports from the local `gohome/` directory.

- [ ] **Step 2: Create `gohome:config` — the root module**

`internal/config/pkl/gohome/config.pkl`:

```pkl
module gohome.config

import "gohome:entities" as entities
import "gohome:carport" as carport
import "gohome:automations" as automations
import "gohome:dashboards" as dashboards
import "gohome:auth" as auth

driverInstances: Listing<carport.DriverInstance> = new {}
entities: Listing<entities.Entity> = new {}
automations: Listing<automations.Automation> = new {}
dashboards: Listing<dashboards.Dashboard> = new {}
users: Listing<auth.User> = new {}
roles: Listing<auth.Role> = new {}
policies: Listing<auth.Policy> = new {}

output {
  renderer = new JsonRenderer {}
}
```

This is the module `main.pkl` amends. The `output { renderer = new JsonRenderer {} }` means evaluating `main.pkl` produces JSON — exactly what Go's `json.Unmarshal` expects.

- [ ] **Step 3: Create `gohome:base`**

`internal/config/pkl/gohome/base.pkl`:

```pkl
module gohome.base

// Secrets are tagged strings. Go's ResolveSecrets walks the evaluated JSON
// and replaces these with resolved values before applying side-effects.
// Secrets are NEVER written to the event log.
typealias EnvSecret     = String(matches(Regex("env:[A-Z_][A-Z0-9_]*")))
typealias FileSecret    = String(matches(Regex("file:/.+")))
typealias KeyringSecret = String(matches(Regex("keyring:[^/]+/.+")))
typealias Secret        = String(matches(Regex("(env:[A-Z_]|file:/|keyring:).+")))

class Metadata {
  name: String
  labels: Mapping<String, String> = new {}
}

class RetentionPolicy {
  maxAgeDays: Int?
  maxBytes: Int?
}
```

- [ ] **Step 4: Create `gohome:carport`**

`internal/config/pkl/gohome/carport.pkl`:

```pkl
module gohome.carport

// DriverInstance is the base class for all driver instance configs.
// Driver authors extend this with their own typed fields.
// The pkl_module field in DriverManifest is reserved for C5+ (DR-7).
abstract class DriverInstance {
  id: String(!isEmpty)
  driverName: String(!isEmpty)
}
```

- [ ] **Step 5: Create `gohome:entities`**

`internal/config/pkl/gohome/entities.pkl`:

```pkl
module gohome.entities

abstract class Entity {
  id: String(matches(Regex("[a-z_]+\\.[a-z_]+")))   // dotted-path: "light.living_room"
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

class Sensor extends Entity {
  unit: String?
}

class BinarySensor extends Entity {}
```

- [ ] **Step 6: Create `gohome:starlark` (stub)**

`internal/config/pkl/gohome/starlark.pkl`:

```pkl
module gohome.starlark

// Starlark snippet type aliases — plain String in C4.
// C5 activates isValidStarlark*() validators via ModuleReader hook (DR-6).
// C5 planner: activate validators using go.starlark.net/syntax and wire
// a Pkl external function or custom module reader in evaluator.go.
typealias StarlarkExpr      = String
typealias StarlarkScript    = String
typealias StarlarkCondition = String
```

- [ ] **Step 7: Create `gohome:automations`**

`internal/config/pkl/gohome/automations.pkl`:

```pkl
module gohome.automations

import "gohome:starlark" as starlark

class Trigger {
  kind: String(oneOf("state_changed", "time", "event", "webhook"))
  entityId: String?       // for state_changed triggers
  condition: starlark.StarlarkCondition?
}

class Action {
  kind: String(oneOf("call_service", "scene", "script", "starlark"))
  body: starlark.StarlarkScript?
}

class Automation {
  id: String(!isEmpty)
  trigger: Trigger
  actions: Listing<Action>
  enabled: Boolean = true
}
```

- [ ] **Step 8: Create `gohome:dashboards`, `gohome:widgets`, `gohome:auth`**

`internal/config/pkl/gohome/dashboards.pkl`:

```pkl
module gohome.dashboards

class WidgetInstance {
  widgetClass: String
  props: Mapping<String, Any> = new {}
  col: Int; row: Int; w: Int; h: Int
}

class Grid {
  widgets: Listing<WidgetInstance> = new {}
}

class Page {
  title: String
  grid: Grid
}

class Dashboard {
  slug: String(!isEmpty)
  pages: Listing<Page>
}
```

`internal/config/pkl/gohome/widgets.pkl`:

```pkl
module gohome.widgets

const gauge:        String = "Gauge"
const lineChart:    String = "LineChart"
const entityToggle: String = "EntityToggle"
const markdown:     String = "Markdown"
const scriptButton: String = "ScriptButton"
```

`internal/config/pkl/gohome/auth.pkl`:

```pkl
module gohome.auth

class User {
  slug: String(!isEmpty)
  displayName: String
  roles: Listing<String>
  active: Boolean = true
}

class Role {
  slug: String(!isEmpty)
  permissions: Listing<String>
}

class Policy {
  role: String(!isEmpty)
  resource: String(!isEmpty)
  actions: Listing<String>
}
```

- [ ] **Step 9: Verify all modules parse**

If `pkl` is installed locally:

```bash
cd internal/config/pkl
pkl eval gohome/config.pkl
```

Expected: outputs `{}` (empty JSON object — all lists default to empty).

- [ ] **Step 10: Commit**

```bash
git add internal/config/pkl/
git commit -m "feat(c4): add gohome.* Pkl schema modules"
```

---

## Task 3: `errors.go` and `evaluator.go`

**Files:**
- Create: `internal/config/errors.go`
- Create: `internal/config/evaluator.go`

- [ ] **Step 1: Add pkl-go and go-keyring dependencies**

```bash
cd gohome
go get github.com/apple/pkl-go/pkl@latest
go get github.com/zalando/go-keyring@latest
go mod tidy
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 2: Create `errors.go`**

`internal/config/errors.go`:

```go
package config

import "fmt"

// EvalError is a structured error from the Pkl evaluator with location info.
type EvalError struct {
    File    string
    Line    int
    Column  int
    Message string
}

func (e *EvalError) Error() string {
    if e.File != "" {
        return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
    }
    return e.Message
}

// ValidationError is a cross-reference or semantic error found during Compile.
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    if e.Field != "" {
        return fmt.Sprintf("%s: %s", e.Field, e.Message)
    }
    return e.Message
}
```

- [ ] **Step 3: Write the failing test for evaluator**

The `configEvaluator` interface must be in production code (not a test file) since `manager.go` will depend on it. Add it to the bottom of `evaluator.go` as you write it in Step 5, then write the test:

`internal/config/evaluator_test.go`:

```go
package config

import (
    "context"
    "testing"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

// fakeEval is a test double for configEvaluator.
type fakeEval struct {
    snap *configpb.ConfigSnapshot
    err  error
}

func (f *fakeEval) Evaluate(_ context.Context, _ string) (*configpb.ConfigSnapshot, error) {
    return f.snap, f.err
}

func TestFakeEvalSatisfiesInterface(t *testing.T) {
    var _ configEvaluator = &fakeEval{}
}
```

- [ ] **Step 4: Run test to verify it fails (it won't — it's a compile-time check)**

```bash
go test ./internal/config/... -run TestFakeEvalSatisfiesInterface -v
```

Expected: PASS immediately (compile-time assertion).

- [ ] **Step 5: Create `evaluator.go`**

`internal/config/evaluator.go`:

```go
package config

import (
    "context"
    "crypto/sha256"
    "embed"
    "encoding/json"
    "fmt"
    "net/url"
    "strings"
    "time"

    "github.com/apple/pkl-go/pkl"
    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

//go:embed pkl
var pklFS embed.FS

// pklEvaluator wraps pkl-go and implements configEvaluator.
type pklEvaluator struct {
    ev pkl.Evaluator
}

func newPklEvaluator(ctx context.Context) (*pklEvaluator, error) {
    ev, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions,
        pkl.WithModuleReaders(&gohomeModuleReader{}),
    )
    if err != nil {
        return nil, fmt.Errorf("pkl evaluator: %w", err)
    }
    return &pklEvaluator{ev: ev}, nil
}

// configEvaluator is the interface Manager depends on — defined here so it is
// available to both manager.go and test doubles in evaluator_test.go.
type configEvaluator interface {
    Evaluate(ctx context.Context, configDir string) (*configpb.ConfigSnapshot, error)
}

// gohomeModuleReader serves gohome:* modules from the embedded FS.
// It implements pkl.ModuleReader.
// Scheme: "gohome". URI form: "gohome:base", "gohome:entities", etc.
type gohomeModuleReader struct{}

func (r *gohomeModuleReader) Scheme() string               { return "gohome" }
func (r *gohomeModuleReader) IsGlobbable() bool            { return false }
func (r *gohomeModuleReader) HasHierarchicalUris() bool    { return false }
func (r *gohomeModuleReader) ListElements(_ url.URL) ([]pkl.PathElement, error) {
    return nil, nil
}

func (r *gohomeModuleReader) Read(u url.URL) (string, error) {
    // u.Opaque is "base", "entities", "config", etc.
    name := u.Opaque
    path := "pkl/gohome/" + name + ".pkl"
    data, err := pklFS.ReadFile(path)
    if err != nil {
        return "", fmt.Errorf("gohome module %q not found", name)
    }
    return string(data), nil
}

// Evaluate evaluates configDir/main.pkl and returns a ConfigSnapshot.
func (e *pklEvaluator) Evaluate(ctx context.Context, configDir string) (*configpb.ConfigSnapshot, error) {
    mainPath := configDir + "/main.pkl"
    text, err := e.ev.EvaluateOutputText(ctx, pkl.FileSource(mainPath))
    if err != nil {
        return nil, &EvalError{Message: err.Error()}
    }
    return parseConfigJSON(text, configDir)
}

// configJSON mirrors the fields of gohome:config for JSON unmarshalling.
type configJSON struct {
    DriverInstances []json.RawMessage `json:"driverInstances"`
    Entities        []entityJSON      `json:"entities"`
    Automations     []automationJSON  `json:"automations"`
    Dashboards      []dashboardJSON   `json:"dashboards"`
    Users           []userJSON        `json:"users"`
    Roles           []roleJSON        `json:"roles"`
    Policies        []policyJSON      `json:"policies"`
}

type entityJSON struct {
    ID           string `json:"id"`
    FriendlyName string `json:"friendlyName"`
    EntityType   string `json:"type"`     // Pkl class name, e.g. "Light"
    Area         string `json:"area"`
}

type automationJSON struct {
    ID      string          `json:"id"`
    Enabled bool            `json:"enabled"`
    Raw     json.RawMessage `json:"-"`
}

type dashboardJSON struct {
    Slug string          `json:"slug"`
    Raw  json.RawMessage `json:"-"`
}

type userJSON struct {
    Slug        string   `json:"slug"`
    DisplayName string   `json:"displayName"`
    Roles       []string `json:"roles"`
    Active      bool     `json:"active"`
}

type roleJSON struct {
    Slug        string   `json:"slug"`
    Permissions []string `json:"permissions"`
}

type policyJSON struct {
    Role     string   `json:"role"`
    Resource string   `json:"resource"`
    Actions  []string `json:"actions"`
}

func parseConfigJSON(text, configDir string) (*configpb.ConfigSnapshot, error) {
    var raw configJSON
    if err := json.Unmarshal([]byte(text), &raw); err != nil {
        return nil, fmt.Errorf("parse config output: %w", err)
    }

    snap := &configpb.ConfigSnapshot{
        EvaluatedAtUnixMs: time.Now().UnixMilli(),
        ConfigDir:         configDir,
    }

    // Driver instances: each element is raw JSON; hash it for diffing.
    for _, rawInst := range raw.DriverInstances {
        var base struct {
            ID         string `json:"id"`
            DriverName string `json:"driverName"`
        }
        if err := json.Unmarshal(rawInst, &base); err != nil {
            return nil, fmt.Errorf("parse driver instance: %w", err)
        }
        h := sha256.Sum256(rawInst)
        snap.DriverInstances = append(snap.DriverInstances, &configpb.DriverInstanceConfig{
            Id:         base.ID,
            DriverName: base.DriverName,
            ConfigHash: h[:],
            Params:     rawInst,
        })
    }

    // Entities
    for _, e := range raw.Entities {
        snap.Entities = append(snap.Entities, &configpb.EntityConfig{
            Id:           e.ID,
            FriendlyName: e.FriendlyName,
            EntityType:   e.EntityType,
            Area:         e.Area,
        })
    }

    // Automations (opaque until C6)
    // Pkl outputs all fields including defaults, so a.Enabled is always accurate.
    for _, a := range raw.Automations {
        b, _ := json.Marshal(a)
        snap.Automations = append(snap.Automations, &configpb.AutomationConfig{
            Id:      strings.TrimSpace(a.ID),
            Content: b,
            Enabled: a.Enabled,
        })
    }

    // Dashboards (opaque until C10)
    for _, d := range raw.Dashboards {
        b, _ := json.Marshal(d)
        snap.Dashboards = append(snap.Dashboards, &configpb.DashboardConfig{
            Slug:    d.Slug,
            Content: b,
        })
    }

    // Auth
    for _, u := range raw.Users {
        snap.Users = append(snap.Users, &configpb.UserConfig{
            Slug:        u.Slug,
            DisplayName: u.DisplayName,
            Roles:       u.Roles,
            Active:      u.Active,
        })
    }
    for _, r := range raw.Roles {
        snap.Roles = append(snap.Roles, &configpb.RoleConfig{
            Slug:        r.Slug,
            Permissions: r.Permissions,
        })
    }
    for _, p := range raw.Policies {
        snap.Policies = append(snap.Policies, &configpb.PolicyConfig{
            Role:     p.Role,
            Resource: p.Resource,
            Actions:  p.Actions,
        })
    }

    return snap, nil
}
```

> **pkl-go API note:** `pkl.WithModuleReaders` takes `...pkl.ModuleReader`. The `pkl.ModuleReader` interface is defined in `github.com/apple/pkl-go/pkl` — check pkg.go.dev for the exact method set if the compiler complains about missing methods (common ones: `Scheme`, `IsGlobbable`, `HasHierarchicalUris`, `ListElements`, `Read`). The `gohomeModuleReader` above implements the full interface.

- [ ] **Step 6: Run build to verify compilation**

```bash
task build
```

Expected: compiles. If pkl-go has a slightly different `ModuleReader` interface, fix the missing methods by checking the pkg.go.dev docs for `github.com/apple/pkl-go/pkl`.

- [ ] **Step 7: Commit**

```bash
git add internal/config/errors.go internal/config/evaluator.go internal/config/evaluator_test.go go.mod go.sum
git commit -m "feat(c4): add config evaluator with gohome:* module reader"
```

---

## Task 4: `compile.go` — cross-reference validation

**Files:**
- Create: `internal/config/compile.go`
- Test: `internal/config/compile_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/compile_test.go`:

```go
package config

import (
    "testing"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

type fakeQuerier struct {
    knownDrivers map[string]bool
}

func (f *fakeQuerier) DriverExists(name string) bool {
    return f.knownDrivers[name]
}

func TestCompile_DuplicateDriverInstanceID(t *testing.T) {
    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue-main", DriverName: "hue"},
            {Id: "hue-main", DriverName: "hue"},
        },
    }
    errs := Compile(snap, &fakeQuerier{knownDrivers: map[string]bool{"hue": true}})
    if len(errs) == 0 {
        t.Fatal("expected duplicate ID error, got none")
    }
}

func TestCompile_UnknownDriver(t *testing.T) {
    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue-main", DriverName: "nonexistent"},
        },
    }
    errs := Compile(snap, &fakeQuerier{knownDrivers: map[string]bool{}})
    if len(errs) == 0 {
        t.Fatal("expected unknown driver error, got none")
    }
}

func TestCompile_InvalidEntityID(t *testing.T) {
    snap := &configpb.ConfigSnapshot{
        Entities: []*configpb.EntityConfig{
            {Id: "invalid_no_dot", FriendlyName: "Bad"},
        },
    }
    errs := Compile(snap, &fakeQuerier{})
    if len(errs) == 0 {
        t.Fatal("expected entity ID error, got none")
    }
}

func TestCompile_Valid(t *testing.T) {
    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue-main", DriverName: "hue"},
        },
        Entities: []*configpb.EntityConfig{
            {Id: "light.living_room", FriendlyName: "Living Room"},
        },
    }
    errs := Compile(snap, &fakeQuerier{knownDrivers: map[string]bool{"hue": true}})
    if len(errs) != 0 {
        t.Fatalf("expected no errors, got: %v", errs)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestCompile -v
```

Expected: FAIL — `Compile` undefined.

- [ ] **Step 3: Implement `compile.go`**

`internal/config/compile.go`:

```go
package config

import (
    "fmt"
    "strings"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

// RegistryQuerier checks whether a driver name is known to the registry.
type RegistryQuerier interface {
    DriverExists(name string) bool
}

// Compile validates cross-references in a snapshot.
// Returns all errors found — does not stop at the first.
func Compile(snap *configpb.ConfigSnapshot, querier RegistryQuerier) []ValidationError {
    var errs []ValidationError

    // Check for duplicate driver instance IDs and unknown driver names.
    seenInstances := map[string]bool{}
    for _, di := range snap.GetDriverInstances() {
        if seenInstances[di.GetId()] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("driverInstances[%s]", di.GetId()),
                Message: "duplicate driver instance id",
            })
        }
        seenInstances[di.GetId()] = true

        if querier != nil && di.GetDriverName() != "" && !querier.DriverExists(di.GetDriverName()) {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("driverInstances[%s].driverName", di.GetId()),
                Message: fmt.Sprintf("unknown driver %q — is the driver binary registered?", di.GetDriverName()),
            })
        }
    }

    // Check for duplicate entity IDs and validate dotted-path format.
    seenEntities := map[string]bool{}
    for _, e := range snap.GetEntities() {
        if seenEntities[e.GetId()] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("entities[%s]", e.GetId()),
                Message: "duplicate entity id",
            })
        }
        seenEntities[e.GetId()] = true

        if !isValidEntityID(e.GetId()) {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("entities[%s].id", e.GetId()),
                Message: `entity id must be "<type>.<name>" e.g. "light.living_room"`,
            })
        }
    }

    // Check for duplicate automation IDs.
    seenAutomations := map[string]bool{}
    for _, a := range snap.GetAutomations() {
        if seenAutomations[a.GetId()] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("automations[%s]", a.GetId()),
                Message: "duplicate automation id",
            })
        }
        seenAutomations[a.GetId()] = true
    }

    return errs
}

// isValidEntityID returns true if id matches "<type>.<name>" with no extra dots.
func isValidEntityID(id string) bool {
    parts := strings.SplitN(id, ".", 2)
    return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -run TestCompile -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/compile.go internal/config/compile_test.go
git commit -m "feat(c4): add cross-reference compiler"
```

---

## Task 5: `diff.go` — snapshot diff engine

**Files:**
- Create: `internal/config/diff.go`
- Test: `internal/config/diff_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/diff_test.go`:

```go
package config

import (
    "bytes"
    "crypto/sha256"
    "testing"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

func makeInst(id, driverName string, params []byte) *configpb.DriverInstanceConfig {
    h := sha256.Sum256(params)
    return &configpb.DriverInstanceConfig{Id: id, DriverName: driverName, ConfigHash: h[:], Params: params}
}

func TestDiff_AddedInstance(t *testing.T) {
    old := &configpb.ConfigSnapshot{}
    next := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main"}`))},
    }
    d := Diff(old, next)
    if len(d.DriverInstancesAdded) != 1 || d.DriverInstancesAdded[0] != "hue-main" {
        t.Fatalf("expected 1 added, got %+v", d)
    }
}

func TestDiff_RemovedInstance(t *testing.T) {
    old := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main"}`))},
    }
    next := &configpb.ConfigSnapshot{}
    d := Diff(old, next)
    if len(d.DriverInstancesRemoved) != 1 {
        t.Fatalf("expected 1 removed, got %+v", d)
    }
}

func TestDiff_ChangedInstance(t *testing.T) {
    old := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main","bridgeIP":"1.2.3.4"}`))},
    }
    next := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main","bridgeIP":"5.6.7.8"}`))},
    }
    d := Diff(old, next)
    if len(d.DriverInstancesChanged) != 1 {
        t.Fatalf("expected 1 changed, got %+v", d)
    }
}

func TestDiff_NoChange(t *testing.T) {
    params := []byte(`{"id":"hue-main"}`)
    old := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", params)},
    }
    next := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", params)},
    }
    d := Diff(old, next)
    if len(d.DriverInstancesAdded)+len(d.DriverInstancesRemoved)+len(d.DriverInstancesChanged) != 0 {
        t.Fatalf("expected no diff, got %+v", d)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestDiff -v
```

Expected: FAIL — `Diff` undefined.

- [ ] **Step 3: Implement `diff.go`**

`internal/config/diff.go`:

```go
package config

import (
    "bytes"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

// ConfigDiff describes what changed between two snapshots.
type ConfigDiff struct {
    DriverInstancesAdded   []string // driver instance IDs
    DriverInstancesRemoved []string
    DriverInstancesChanged []string
    AutomationsChanged     int
    DashboardsChanged      int
}

// Diff computes the minimal changeset between old and next snapshots.
// Nil old is treated as empty (first-ever apply).
func Diff(old, next *configpb.ConfigSnapshot) *ConfigDiff {
    d := &ConfigDiff{}

    // Build maps from the old snapshot.
    oldInsts := map[string][]byte{} // id → config_hash
    if old != nil {
        for _, di := range old.GetDriverInstances() {
            oldInsts[di.GetId()] = di.GetConfigHash()
        }
    }

    // Compare driver instances.
    nextIDs := map[string]bool{}
    for _, di := range next.GetDriverInstances() {
        nextIDs[di.GetId()] = true
        oldHash, existed := oldInsts[di.GetId()]
        if !existed {
            d.DriverInstancesAdded = append(d.DriverInstancesAdded, di.GetId())
        } else if !bytes.Equal(oldHash, di.GetConfigHash()) {
            d.DriverInstancesChanged = append(d.DriverInstancesChanged, di.GetId())
        }
    }
    for id := range oldInsts {
        if !nextIDs[id] {
            d.DriverInstancesRemoved = append(d.DriverInstancesRemoved, id)
        }
    }

    // Count automation changes (opaque until C6 — count mismatches suffice).
    oldAutoCount := 0
    if old != nil {
        oldAutoCount = len(old.GetAutomations())
    }
    if len(next.GetAutomations()) != oldAutoCount {
        d.AutomationsChanged = len(next.GetAutomations()) - oldAutoCount
    }

    // Count dashboard changes.
    oldDashCount := 0
    if old != nil {
        oldDashCount = len(old.GetDashboards())
    }
    if len(next.GetDashboards()) != oldDashCount {
        d.DashboardsChanged = len(next.GetDashboards()) - oldDashCount
    }

    return d
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -run TestDiff -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/diff.go internal/config/diff_test.go
git commit -m "feat(c4): add snapshot diff engine"
```

---

## Task 6: `secrets.go` — secret resolvers

**Files:**
- Create: `internal/config/secrets.go`
- Test: `internal/config/secrets_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/secrets_test.go`:

```go
package config

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

type fakeKeyring struct {
    data map[string]string
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
    v, ok := f.data[service+"/"+user]
    if !ok {
        return "", fmt.Errorf("not found")
    }
    return v, nil
}

func TestResolveSecrets_Env(t *testing.T) {
    t.Setenv("TEST_API_KEY", "secret-value")
    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue-main", Params: []byte(`{"id":"hue-main","apiKey":"env:TEST_API_KEY"}`)},
        },
    }
    if err := ResolveSecrets(context.Background(), snap, nil); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !bytes.Contains(snap.DriverInstances[0].Params, []byte("secret-value")) {
        t.Fatalf("secret not resolved: %s", snap.DriverInstances[0].Params)
    }
}

func TestResolveSecrets_File(t *testing.T) {
    dir := t.TempDir()
    secretFile := filepath.Join(dir, "api_key")
    _ = os.WriteFile(secretFile, []byte("  file-secret-value\n"), 0o600)

    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "test", Params: []byte(`{"id":"test","token":"file:` + secretFile + `"}`)},
        },
    }
    if err := ResolveSecrets(context.Background(), snap, nil); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !bytes.Contains(snap.DriverInstances[0].Params, []byte("file-secret-value")) {
        t.Fatalf("file secret not resolved: %s", snap.DriverInstances[0].Params)
    }
}

func TestResolveSecrets_Keyring(t *testing.T) {
    kr := &fakeKeyring{data: map[string]string{"gohome/hue_key": "kr-secret"}}
    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue", Params: []byte(`{"id":"hue","apiKey":"keyring:gohome/hue_key"}`)},
        },
    }
    if err := ResolveSecrets(context.Background(), snap, kr); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !bytes.Contains(snap.DriverInstances[0].Params, []byte("kr-secret")) {
        t.Fatalf("keyring secret not resolved: %s", snap.DriverInstances[0].Params)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestResolveSecrets -v
```

Expected: FAIL — `ResolveSecrets` undefined.

- [ ] **Step 3: Implement `secrets.go`**

`internal/config/secrets.go`:

```go
package config

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"

    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

// Keyring is satisfied by go-keyring and by test doubles.
type Keyring interface {
    Get(service, user string) (string, error)
}

// ResolveSecrets resolves tagged secret strings in driver instance params in-place.
// Secrets are NEVER written to the event log — call this after Compile, before Apply side-effects.
// If kr is nil, keyring: secrets return an error.
func ResolveSecrets(ctx context.Context, snap *configpb.ConfigSnapshot, kr Keyring) error {
    for _, di := range snap.GetDriverInstances() {
        resolved, err := resolveJSONSecrets(di.GetParams(), kr)
        if err != nil {
            return fmt.Errorf("driver instance %q: %w", di.GetId(), err)
        }
        di.Params = resolved
    }
    return nil
}

// resolveJSONSecrets walks a JSON blob and replaces secret-tagged strings.
func resolveJSONSecrets(data []byte, kr Keyring) ([]byte, error) {
    if len(data) == 0 {
        return data, nil
    }
    var obj map[string]interface{}
    if err := json.Unmarshal(data, &obj); err != nil {
        // Not a JSON object — return as-is (e.g. primitive or array).
        return data, nil
    }
    changed := false
    if err := walkMap(obj, kr, &changed); err != nil {
        return nil, err
    }
    if !changed {
        return data, nil
    }
    return json.Marshal(obj)
}

func walkMap(m map[string]interface{}, kr Keyring, changed *bool) error {
    for k, v := range m {
        switch val := v.(type) {
        case string:
            resolved, err := resolveSecret(val, kr)
            if err != nil {
                return fmt.Errorf("field %q: %w", k, err)
            }
            if resolved != val {
                m[k] = resolved
                *changed = true
            }
        case map[string]interface{}:
            if err := walkMap(val, kr, changed); err != nil {
                return err
            }
        }
    }
    return nil
}

// resolveSecret resolves one tagged string. Returns s unchanged if not a secret tag.
func resolveSecret(s string, kr Keyring) (string, error) {
    switch {
    case strings.HasPrefix(s, "env:"):
        varName := s[4:]
        val := os.Getenv(varName)
        if val == "" {
            return "", fmt.Errorf("env var %q is not set", varName)
        }
        return val, nil

    case strings.HasPrefix(s, "file:"):
        path := s[5:]
        data, err := os.ReadFile(path)
        if err != nil {
            return "", fmt.Errorf("read secret file %q: %w", path, err)
        }
        return strings.TrimSpace(string(data)), nil

    case strings.HasPrefix(s, "keyring:"):
        if kr == nil {
            return "", fmt.Errorf("keyring not available (secret: %q)", s)
        }
        // Format: keyring:service/user
        rest := s[8:]
        idx := strings.LastIndex(rest, "/")
        if idx < 0 {
            return "", fmt.Errorf("invalid keyring secret %q: want keyring:service/user", s)
        }
        service, user := rest[:idx], rest[idx+1:]
        return kr.Get(service, user)

    default:
        return s, nil
    }
}
```

- [ ] **Step 4: Add missing `bytes` import to test file**

The test uses `bytes.Contains` — add `"bytes"` and `"fmt"` to the test imports.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/config/... -run TestResolveSecrets -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/secrets.go internal/config/secrets_test.go
git commit -m "feat(c4): add secret resolvers (env/file/keyring)"
```

---

## Task 7: `manager.go` — the main orchestrator

**Files:**
- Create: `internal/config/manager.go`
- Test: `internal/config/manager_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/manager_test.go`:

```go
package config

import (
    "context"
    "testing"
    "time"

    eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
    "github.com/fynn-labs/gohome/internal/eventstore"
)

type fakeCarport struct {
    registered   []string
    unregistered []string
}

func (f *fakeCarport) RegisterInstance(_ context.Context, id, _ string, _ []byte) error {
    f.registered = append(f.registered, id)
    return nil
}

func (f *fakeCarport) UnregisterInstance(_ context.Context, id string) error {
    f.unregistered = append(f.unregistered, id)
    return nil
}

type fakeStore struct {
    appended []eventstore.Event
}

func (f *fakeStore) Append(_ context.Context, e eventstore.Event) (uint64, error) {
    f.appended = append(f.appended, e)
    return uint64(len(f.appended)), nil
}

func TestManager_Apply_CallsCarportAndAppends(t *testing.T) {
    snap := &configpb.ConfigSnapshot{
        EvaluatedAtUnixMs: time.Now().UnixMilli(),
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue-main", DriverName: "hue", Params: []byte(`{"id":"hue-main"}`)},
        },
    }

    fakeEv := &fakeEval{snap: snap}
    fc := &fakeCarport{}
    fs := &fakeStore{}

    mgr := &Manager{
        configDir:  "/fake",
        ev:         fakeEv,
        store:      fs,
        carportMgr: fc,
    }

    if err := mgr.Apply(context.Background(), false); err != nil {
        t.Fatalf("Apply: %v", err)
    }

    if len(fc.registered) != 1 || fc.registered[0] != "hue-main" {
        t.Errorf("expected hue-main registered, got %v", fc.registered)
    }
    if len(fs.appended) != 1 {
        t.Fatalf("expected 1 event appended, got %d", len(fs.appended))
    }
    ev := fs.appended[0]
    applied := ev.Payload.GetConfigApplied()
    if applied == nil {
        t.Fatal("expected ConfigApplied payload")
    }
    if applied.DriverInstancesAdded != 1 {
        t.Errorf("expected 1 driver added, got %d", applied.DriverInstancesAdded)
    }
}

func TestManager_Apply_DryRun_NoSideEffects(t *testing.T) {
    snap := &configpb.ConfigSnapshot{
        DriverInstances: []*configpb.DriverInstanceConfig{
            {Id: "hue-main", DriverName: "hue", Params: []byte(`{}`)},
        },
    }
    fc := &fakeCarport{}
    fs := &fakeStore{}
    mgr := &Manager{configDir: "/fake", ev: &fakeEval{snap: snap}, store: fs, carportMgr: fc}

    if err := mgr.Apply(context.Background(), true); err != nil {
        t.Fatalf("dry-run Apply: %v", err)
    }
    if len(fc.registered) != 0 {
        t.Errorf("dry-run should not register instances")
    }
    if len(fs.appended) != 0 {
        t.Errorf("dry-run should not append events")
    }
}

func TestManager_Current_NilBeforeApply(t *testing.T) {
    mgr := &Manager{}
    if mgr.Current() != nil {
        t.Error("Current() should be nil before Apply")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestManager -v
```

Expected: FAIL — `Manager` undefined.

- [ ] **Step 3: Implement `manager.go`**

`internal/config/manager.go`:

```go
package config

import (
    "context"
    "fmt"
    "sync"
    "time"

    eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
    configpb "github.com/fynn-labs/gohome/gen/gohome/config/v1"
    "github.com/fynn-labs/gohome/internal/eventstore"
)

// CarportManager is the subset of carport.Host that config.Manager needs.
// For C4, the daemon passes a no-op implementation; dynamic carport management
// will be wired when carport.Host gains RegisterInstance/UnregisterInstance methods.
type CarportManager interface {
    RegisterInstance(ctx context.Context, id, driverName string, params []byte) error
    UnregisterInstance(ctx context.Context, id string) error
}

// eventStore is the subset of eventstore.Store that Manager needs.
type eventStore interface {
    Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// Manager is the main entry point for config evaluation, validation, and application.
// It is safe for concurrent use.
type Manager struct {
    configDir  string
    ev         configEvaluator
    store      eventStore
    carportMgr CarportManager
    keyring    Keyring

    mu      sync.RWMutex
    current *configpb.ConfigSnapshot
}

// NewManager creates a Manager that evaluates config at configDir/main.pkl.
func NewManager(ctx context.Context, configDir string, store eventStore, carportMgr CarportManager) (*Manager, error) {
    ev, err := newPklEvaluator(ctx)
    if err != nil {
        return nil, fmt.Errorf("init pkl evaluator: %w", err)
    }
    return &Manager{
        configDir:  configDir,
        ev:         ev,
        store:      store,
        carportMgr: carportMgr,
    }, nil
}

// Current returns the most-recently-applied ConfigSnapshot. Nil before first Apply.
func (m *Manager) Current() *configpb.ConfigSnapshot {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.current
}

// Validate evaluates and cross-ref-validates config. Returns snapshot + diff with no side-effects.
func (m *Manager) Validate(ctx context.Context) (*configpb.ConfigSnapshot, *ConfigDiff, error) {
    snap, err := m.ev.Evaluate(ctx, m.configDir)
    if err != nil {
        return nil, nil, err
    }
    // For C4, RegistryQuerier is nil — driver existence checks are skipped.
    // Wire a real querier when registry.Registry is accessible from config.Manager.
    if errs := Compile(snap, nil); len(errs) != 0 {
        return nil, nil, &compileErrors{errs: errs}
    }
    m.mu.RLock()
    diff := Diff(m.current, snap)
    m.mu.RUnlock()
    return snap, diff, nil
}

// Apply runs Validate, resolves secrets, applies carport side-effects, and appends ConfigApplied.
// If dryRun is true, stops after diff — no secrets resolved, no events appended.
func (m *Manager) Apply(ctx context.Context, dryRun bool) error {
    snap, diff, err := m.Validate(ctx)
    if err != nil {
        return err
    }
    if dryRun {
        return nil
    }

    // Resolve secrets in-place before any side-effects.
    if err := ResolveSecrets(ctx, snap, m.keyring); err != nil {
        return fmt.Errorf("resolve secrets: %w", err)
    }

    // Apply carport side-effects.
    for _, id := range diff.DriverInstancesRemoved {
        if err := m.carportMgr.UnregisterInstance(ctx, id); err != nil {
            return fmt.Errorf("unregister %q: %w", id, err)
        }
    }
    for _, id := range diff.DriverInstancesAdded {
        di := findInstance(snap, id)
        if err := m.carportMgr.RegisterInstance(ctx, di.GetId(), di.GetDriverName(), di.GetParams()); err != nil {
            return fmt.Errorf("register %q: %w", id, err)
        }
    }
    for _, id := range diff.DriverInstancesChanged {
        di := findInstance(snap, id)
        if err := m.carportMgr.UnregisterInstance(ctx, id); err != nil {
            return fmt.Errorf("unregister changed %q: %w", id, err)
        }
        if err := m.carportMgr.RegisterInstance(ctx, di.GetId(), di.GetDriverName(), di.GetParams()); err != nil {
            return fmt.Errorf("re-register changed %q: %w", id, err)
        }
    }

    // Commit.
    m.mu.Lock()
    m.current = snap
    m.mu.Unlock()

    _, err = m.store.Append(ctx, eventstore.Event{
        Kind:      "config",
        Source:    "config.Manager",
        Timestamp: time.Now(),
        Payload: &eventv1.Payload{Kind: &eventv1.Payload_ConfigApplied{
            ConfigApplied: &eventv1.ConfigApplied{
                AppliedAtUnixMs:       snap.GetEvaluatedAtUnixMs(),
                DriverInstancesAdded:  int32(len(diff.DriverInstancesAdded)),
                DriverInstancesRemoved: int32(len(diff.DriverInstancesRemoved)),
                DriverInstancesChanged: int32(len(diff.DriverInstancesChanged)),
                AutomationsChanged:    int32(diff.AutomationsChanged),
            },
        }},
    })
    return err
}

func findInstance(snap *configpb.ConfigSnapshot, id string) *configpb.DriverInstanceConfig {
    for _, di := range snap.GetDriverInstances() {
        if di.GetId() == id {
            return di
        }
    }
    return nil
}

// compileErrors wraps multiple ValidationErrors for clean error rendering.
type compileErrors struct {
    errs []ValidationError
}

func (e *compileErrors) Error() string {
    if len(e.errs) == 1 {
        return e.errs[0].Error()
    }
    return fmt.Sprintf("%d validation errors (first: %s)", len(e.errs), e.errs[0].Error())
}

// Errors returns the individual ValidationErrors for CLI rendering.
func (e *compileErrors) Errors() []ValidationError { return e.errs }
```

> **Note on `RegistryQuerier`:** In C4, `Compile` is called with `nil` querier so driver-existence checks are skipped. When `registry.Registry` is wired into `Manager` (post-C4), pass `reg` as the querier and the checks will activate automatically.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -run TestManager -v
```

Expected: all PASS.

- [ ] **Step 5: Run full unit test suite**

```bash
task test
```

Expected: PASS with no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "feat(c4): add config.Manager with validate/apply/current"
```

---

## Task 8: Integration tests and testdata fixtures

**Files:**
- Create: `internal/config/testdata/valid/main.pkl`
- Create: `internal/config/testdata/invalid-xref/main.pkl`
- Create: `internal/config/evaluator_integration_test.go`

- [ ] **Step 1: Create valid fixture**

`internal/config/testdata/valid/main.pkl`:

```pkl
amends "gohome:config"

import "gohome:entities" as entities
import "gohome:carport" as carport

driverInstances {
  new carport.DriverInstance {
    id = "fake-main"
    driverName = "fake"
  }
}

entities {
  new entities.Light {
    id = "light.living_room"
    friendlyName = "Living Room"
    supportsBrightness = true
  }
}
```

- [ ] **Step 2: Create invalid-xref fixture**

`internal/config/testdata/invalid-xref/main.pkl`:

```pkl
amends "gohome:config"

import "gohome:entities" as entities

entities {
  new entities.Light {
    id = "invalid_no_dot"     // entity ID has no dot — should fail Compile
    friendlyName = "Bad Entity"
  }
}
```

- [ ] **Step 3: Write the integration test**

`internal/config/evaluator_integration_test.go`:

```go
//go:build integration

package config

import (
    "context"
    "path/filepath"
    "runtime"
    "testing"
)

func testdataDir(t *testing.T, name string) string {
    t.Helper()
    _, file, _, _ := runtime.Caller(0)
    return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestEvaluator_ValidConfig(t *testing.T) {
    ctx := context.Background()
    ev, err := newPklEvaluator(ctx)
    if err != nil {
        t.Fatalf("newPklEvaluator: %v", err)
    }

    snap, err := ev.Evaluate(ctx, testdataDir(t, "valid"))
    if err != nil {
        t.Fatalf("Evaluate: %v", err)
    }
    if len(snap.GetDriverInstances()) != 1 {
        t.Errorf("expected 1 driver instance, got %d", len(snap.GetDriverInstances()))
    }
    if snap.GetDriverInstances()[0].GetId() != "fake-main" {
        t.Errorf("unexpected id: %s", snap.GetDriverInstances()[0].GetId())
    }
    if len(snap.GetEntities()) != 1 {
        t.Errorf("expected 1 entity, got %d", len(snap.GetEntities()))
    }
    if snap.GetEntities()[0].GetId() != "light.living_room" {
        t.Errorf("unexpected entity id: %s", snap.GetEntities()[0].GetId())
    }
}

func TestEvaluator_InvalidXref(t *testing.T) {
    ctx := context.Background()
    ev, err := newPklEvaluator(ctx)
    if err != nil {
        t.Fatalf("newPklEvaluator: %v", err)
    }

    snap, err := ev.Evaluate(ctx, testdataDir(t, "invalid-xref"))
    if err != nil {
        t.Fatalf("Evaluate: %v", err)
    }
    errs := Compile(snap, nil)
    if len(errs) == 0 {
        t.Fatal("expected validation errors for invalid-xref fixture")
    }
    found := false
    for _, e := range errs {
        if e.Field != "" {
            found = true
        }
    }
    if !found {
        t.Errorf("expected a ValidationError with a non-empty Field, got: %v", errs)
    }
}
```

- [ ] **Step 4: Run integration tests (requires `pkl` binary on PATH)**

```bash
task test:integration 2>&1 | grep -E "config|PASS|FAIL|ok"
```

Expected: integration tests in `internal/config/` pass.

If `pkl` is not installed: `brew install pkl` or download from https://pkl-lang.org/main/current/pkl-cli/index.html#installation.

- [ ] **Step 5: Commit**

```bash
git add internal/config/testdata/ internal/config/evaluator_integration_test.go
git commit -m "feat(c4): add config evaluator integration tests and fixtures"
```

---

## Task 9: CLI — `gohome config validate` and `gohome config apply`

**Files:**
- Create: `internal/cli/config.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Create `internal/cli/config.go`**

```go
package cli

import (
    "context"
    "errors"
    "fmt"
    "os"

    lgtable "github.com/charmbracelet/lipgloss/table"
    "github.com/charmbracelet/lipgloss"
    "github.com/spf13/cobra"

    "github.com/fynn-labs/gohome/internal/config"
    "github.com/fynn-labs/gohome/internal/eventstore"
    "github.com/fynn-labs/gohome/internal/storage"
)

func newConfigCmd(gf *globalFlags) *cobra.Command {
    var configDir string

    c := &cobra.Command{
        Use:   "config",
        Short: "Validate and apply Pkl configuration",
    }
    c.PersistentFlags().StringVar(&configDir, "config-dir", defaultConfigDir(), "config directory containing main.pkl")
    c.AddCommand(newConfigValidateCmd(gf, &configDir))
    c.AddCommand(newConfigApplyCmd(gf, &configDir))
    return c
}

func newConfigValidateCmd(gf *globalFlags, configDir *string) *cobra.Command {
    return &cobra.Command{
        Use:   "validate",
        Short: "Evaluate and validate main.pkl without applying",
        Run: func(cmd *cobra.Command, _ []string) {
            ctx := cmd.Context()
            mgr := buildManager(ctx, gf, *configDir)
            snap, diff, err := mgr.Validate(ctx)
            if err != nil {
                printConfigError(err)
                os.Exit(1)
            }
            _ = diff
            fmt.Println(Success.Render("✓ Config valid"))
            t := lgtable.New().StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
            t.Row("Driver instances", fmt.Sprintf("%d", len(snap.GetDriverInstances())))
            t.Row("Entities", fmt.Sprintf("%d", len(snap.GetEntities())))
            t.Row("Automations", fmt.Sprintf("%d", len(snap.GetAutomations())))
            t.Row("Dashboards", fmt.Sprintf("%d", len(snap.GetDashboards())))
            fmt.Println(t)
        },
    }
}

func newConfigApplyCmd(gf *globalFlags, configDir *string) *cobra.Command {
    var dryRun bool
    cmd := &cobra.Command{
        Use:   "apply",
        Short: "Evaluate, validate, and apply main.pkl",
        Run: func(cmd *cobra.Command, _ []string) {
            ctx := cmd.Context()
            mgr := buildManager(ctx, gf, *configDir)

            // Get current snapshot for diff display.
            _, diff, err := mgr.Validate(ctx)
            if err != nil {
                printConfigError(err)
                os.Exit(1)
            }

            if err := mgr.Apply(ctx, dryRun); err != nil {
                printConfigError(err)
                os.Exit(1)
            }

            label := ""
            if dryRun {
                label = Dim.Render(" (dry-run)")
            }
            fmt.Printf("Config applied%s\n", label)
            t := lgtable.New().
                Headers("Resource", "Added", "Removed", "Changed").
                StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
            t.Row("Driver instances",
                fmt.Sprintf("+%d", len(diff.DriverInstancesAdded)),
                fmt.Sprintf("-%d", len(diff.DriverInstancesRemoved)),
                fmt.Sprintf("~%d", len(diff.DriverInstancesChanged)),
            )
            t.Row("Automations", "+0", "-0", fmt.Sprintf("~%d", diff.AutomationsChanged))
            t.Row("Dashboards", "+0", "-0", fmt.Sprintf("~%d", diff.DashboardsChanged))
            fmt.Println(t)
        },
    }
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print diff without applying")
    return cmd
}

func buildManager(ctx context.Context, gf *globalFlags, configDir string) *config.Manager {
    db, err := storage.Open(ctx, storage.Config{Path: expandHome(gf.DataDir) + "/gohome.db"})
    dieOnError(err)
    store, err := eventstore.Open(ctx, eventstore.Config{}, db, nullLogger(), nullMetrics())
    dieOnError(err)
    mgr, err := config.NewManager(ctx, configDir, store, &nopCarportManager{})
    dieOnError(err)
    return mgr
}

// nopCarportManager satisfies config.CarportManager for the CLI context where
// we don't have access to a running carport supervisor.
type nopCarportManager struct{}

func (n *nopCarportManager) RegisterInstance(_ context.Context, _, _ string, _ []byte) error {
    return nil
}
func (n *nopCarportManager) UnregisterInstance(_ context.Context, _ string) error {
    return nil
}

func printConfigError(err error) {
    var ce interface{ Errors() []config.ValidationError }
    if errors.As(err, &ce) {
        for _, ve := range ce.Errors() {
            fmt.Fprintln(os.Stderr, Error.Render("error:")+fmt.Sprintf(" %s: %s", ve.Field, ve.Message))
        }
        return
    }
    var ee *config.EvalError
    if errors.As(err, &ee) {
        fmt.Fprintln(os.Stderr, Error.Render("eval error:")+fmt.Sprintf(" %s", ee.Error()))
        return
    }
    fmt.Fprintln(os.Stderr, Error.Render("error:")+fmt.Sprintf(" %s", err.Error()))
}

func defaultConfigDir() string {
    if v := os.Getenv("GOHOME_CONFIG_DIR"); v != "" {
        return v
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return ".config/gohome"
    }
    return home + "/.config/gohome"
}
```

- [ ] **Step 2: Register in `root.go`**

In `internal/cli/root.go`, add to `NewRoot()` after the existing `root.AddCommand` calls:

```go
root.AddCommand(newConfigCmd(gf))
```

- [ ] **Step 3: Build to verify compilation**

```bash
task build
```

Expected: both binaries compile cleanly. Run `./dist/gohome config --help` to verify the command appears.

- [ ] **Step 4: Manual smoke test**

Create a minimal test config:

```bash
mkdir -p /tmp/gohome-test-config
cat > /tmp/gohome-test-config/main.pkl << 'EOF'
amends "gohome:config"
EOF
./dist/gohome config validate --config-dir /tmp/gohome-test-config
```

Expected output:
```
✓ Config valid
  Driver instances  0
  Entities          0
  Automations       0
  Dashboards        0
```

- [ ] **Step 5: Commit**

```bash
git add internal/cli/config.go internal/cli/root.go
git commit -m "feat(c4): add gohome config validate/apply CLI commands"
```

---

## Task 10: Daemon wiring

**Files:**
- Modify: `internal/daemon/daemon.go`

- [ ] **Step 1: Add config.Manager to the Daemon struct**

In `internal/daemon/daemon.go`, add the import and field:

```go
import (
    // ... existing imports ...
    "github.com/fynn-labs/gohome/internal/config"
)
```

In the `Daemon` struct, add:

```go
type Daemon struct {
    // ... existing fields ...
    configMgr *config.Manager
}
```

Also add `configDir string` to `Config`:

```go
type Config struct {
    // ... existing fields ...
    ConfigDir string  // path to gohome config directory containing main.pkl
}
```

And in `Config.WithDefaults()` (or wherever defaults are set):

```go
if cfg.ConfigDir == "" {
    cfg.ConfigDir = filepath.Join(expandHome("~"), ".config", "gohome")
}
```

- [ ] **Step 2: Wire config.Manager in `Run` after carport (Phase 4.6)**

In `daemon.Run`, after the existing Phase 4.5 carport block and before Phase 5, add:

```go
// Phase 4.6: config — evaluate and apply Pkl config
d.phase.Store(46) // interim phase; not surfaced externally
cfgMgr, err := config.NewManager(ctx, d.cfg.ConfigDir, d.store, &nopCarportManager{})
if err != nil {
    return fmt.Errorf("config manager: %w", err)
}
d.configMgr = cfgMgr
if err := d.configMgr.Apply(ctx, false); err != nil {
    d.logger.Error("initial config load failed", "err", err)
    return fmt.Errorf("config load: %w", err)
}
d.logger.Info("config applied", "config_dir", d.cfg.ConfigDir)
```

Add `nopCarportManager` in the daemon package (same as CLI — satisfies `config.CarportManager` until `carport.Host` gains those methods):

```go
type nopCarportManager struct{}

func (n *nopCarportManager) RegisterInstance(_ context.Context, _, _ string, _ []byte) error {
    return nil
}
func (n *nopCarportManager) UnregisterInstance(_ context.Context, _ string) error {
    return nil
}
```

Also add `--config-dir` flag in `cmd/gohomed/main.go`:

```go
configDir = flag.String("config-dir", "", "config directory with main.pkl (default ~/.config/gohome)")
```

And pass it in `daemon.Config`:

```go
cfg := daemon.Config{
    // ... existing fields ...
    ConfigDir: *configDir,
}
```

- [ ] **Step 3: Build and run unit tests**

```bash
task build && task test
```

Expected: both pass cleanly.

- [ ] **Step 4: Commit**

```bash
git add internal/daemon/daemon.go cmd/gohomed/main.go
git commit -m "feat(c4): wire config.Manager into daemon startup"
```

---

## Task 11: Definition of done

- [ ] **Step 1: Full build**

```bash
task build
```

Expected: no errors.

- [ ] **Step 2: Unit tests**

```bash
task test
```

Expected: PASS.

- [ ] **Step 3: Race detector**

```bash
task test:race
```

Expected: PASS — no data races.

- [ ] **Step 4: Integration tests**

```bash
task test:integration
```

Expected: PASS — requires `pkl` on PATH.

- [ ] **Step 5: Lint**

```bash
task lint
```

Expected: no issues. Fix any golangci-lint complaints before proceeding.

- [ ] **Step 6: Tidy**

```bash
go mod tidy
```

Expected: no changes to `go.mod` or `go.sum`. If there are changes, stage and commit them.

- [ ] **Step 7: Verify proto is current**

No `.proto` files changed after Task 1 — no need to re-run `task proto`. Confirm:

```bash
git diff gen/
```

Expected: no uncommitted changes in `gen/`.

- [ ] **Step 8: Final commit if needed**

If there are any stray uncommitted changes (e.g. `go.mod` after tidy):

```bash
git add go.mod go.sum
git commit -m "chore: go mod tidy after c4 dependencies"
```

---

## Self-review checklist (for the implementer)

After all tasks pass:

- [ ] `gohome config validate --config-dir <path>` prints a summary table for a valid config
- [ ] `gohome config validate --config-dir <path>` exits 1 and prints structured errors for an invalid config
- [ ] `gohome config apply --dry-run --config-dir <path>` prints the diff table without touching carport or the event store
- [ ] `gohome config apply --config-dir <path>` appends a `ConfigApplied` event (verify with `gohome events list`)
- [ ] `gohomed --config-dir <path>` starts cleanly when `main.pkl` is valid
- [ ] `gohomed --config-dir <path>` exits non-zero with a clear error when `main.pkl` is invalid
- [ ] Secrets: `env:VARNAME` in driver instance params is resolved before carport registration
- [ ] `pkl-lsp` resolves `gohome:*` imports when `pkl.projectDir` is set to `internal/config/pkl/`
