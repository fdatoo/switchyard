# C5 — Starlark Runtime Design

**Milestone:** C5  
**Status:** Approved  
**Date:** 2026-04-22  
**Author:** Fynn Datoo

---

## 1. Scope

C5 adds the Starlark execution substrate to gohome. It lives entirely in the `github.com/fynn-labs/gohome` repo.

**In scope:**
- `internal/starlark` package: `Runtime` struct, six execution contexts with scoped stdlibs, resource limits
- `scene` and `event` injected struct globals with method attributes
- `load("//...")` resolver for user-defined Starlark modules
- Module cache on `Runtime`, invalidated on `ConfigApplied`
- Pkl ↔ Starlark bridging: activate `isValidStarlark*()` validators in `starlark.pkl` and `internal/config/evaluator.go`
- `gohome eval <file.star>` — daemon-connected CLI command
- `gohome test <file_test.star>` — user Starlark test runner CLI command
- `internal/starlark/testutil` — Go test harness package
- All CLI output using lipgloss styles from `internal/cli/styles.go`

**Out of scope (explicitly deferred):**
- Automation trigger/condition evaluation loop (C6)
- Automation/script compilation from Pkl (C6)
- MCP `eval_starlark` tool wiring (C8)
- Widget compute invocation from the web UI (C10)
- `vault:`, `1password:` secret backends (post-C4)

---

## 2. Background

C4 built the Pkl config pipeline and declared Starlark snippets as plain `String` typealiases — stubs waiting for C5. C6 (Automation & Script Engine) will compile Pkl automation declarations into runtime objects and fire them; it depends on C5 for the execution substrate. C5 delivers that substrate: a sandboxed, resource-limited Starlark runner with scoped stdlibs, a `load()` resolver, and tooling for users and developers to test scripts.

**Engine:** `go.starlark.net` — Google-maintained canonical Go implementation of Starlark.

---

## 3. Architecture

### 3.1 `Runtime`

`Runtime` is constructed once by the daemon after C4's config is loaded and lives for the daemon's lifetime.

```go
type Runtime struct {
    state      StateReader
    dispatcher CommandDispatcher
    store      EventAppender
    logger     *slog.Logger
    configDir  string   // for load("//...") resolution
    metrics    *observability.Metrics

    mu          sync.RWMutex
    moduleCache map[string]starlark.StringDict  // abs path → exported symbols
}

func NewRuntime(
    state      StateReader,
    dispatcher CommandDispatcher,
    store      EventAppender,
    logger     *slog.Logger,
    configDir  string,
    metrics    *observability.Metrics,
) *Runtime
```

### 3.2 Dependency interfaces

```go
// StateReader is satisfied by internal/state.Cache.
type StateReader interface {
    Get(entityID string) (*EntityState, bool)
}

// CommandDispatcher is satisfied by internal/carport.Host.
type CommandDispatcher interface {
    Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*DispatchResult, error)
}

// DispatchResult carries the driver's response to a command.
type DispatchResult struct {
    Ok    bool
    Error string
}

// EventAppender is satisfied by internal/eventstore.Store.
type EventAppender interface {
    Append(ctx context.Context, e eventstore.Event) (uint64, error)
}
```

### 3.3 `Execute`

```go
type ContextKind int

const (
    KindAutomation ContextKind = iota
    KindComputedEntity
    KindTriggerCondition
    KindScript
    KindWidgetCompute
    KindMCPEval
)

type Result struct {
    Value   starlark.Value
    Logs    []string
    Elapsed time.Duration
    Steps   uint64
}

func (r *Runtime) Execute(
    ctx          context.Context,
    kind         ContextKind,
    script       string,
    extraGlobals starlark.StringDict,
) (*Result, error)
```

`Execute` is the single entry point for all Starlark execution. Internally it:

1. Builds the stdlib `starlark.StringDict` for `kind` via a dispatch table.
2. Merges `extraGlobals` (caller-supplied bindings, e.g. trigger payload).
3. Creates a `starlark.Thread` with the `//`-resolver wired as `thread.Load`.
4. Enforces resource limits: `thread.SetMaxSteps(n)` for step count; a watchdog goroutine calling `thread.Cancel("timeout")` after the wall-clock deadline.
5. Runs `starlark.ExecFile` (for scripts) or `starlark.Eval` (for expressions).
6. Returns `*Result` or a typed error.

### 3.4 `InvalidateModuleCache`

Called by the daemon when a `ConfigApplied` event is received, ensuring edited `.star` files take effect on the next execution without a restart.

```go
func (r *Runtime) InvalidateModuleCache()
```

---

## 4. Execution contexts and stdlibs

| Kind | Wall-clock | Max steps | Stdlib |
|---|---|---|---|
| `KindAutomation` | 30s | 10 000 000 | `state`, `call_service`, `sleep`, `now`, `log`, `notify`, `scene`, `event`, `random`, `time` |
| `KindComputedEntity` | 100ms | 500 000 | `state` (read-only), `now` |
| `KindTriggerCondition` | 50ms | 100 000 | `state`, `event` (read-only), `now` |
| `KindScript` | 30s | 10 000 000 | Same as `KindAutomation` + `params` from `extraGlobals` |
| `KindWidgetCompute` | 50ms | 100 000 | `state` (read-only, cached snapshot) |
| `KindMCPEval` | 30s | 10 000 000 | Configurable; read-only by default |

### 4.1 Stdlib definitions

**`state(entity_id) → EntityState`**  
Reads from `StateReader`. Returns a Starlark struct with `.state` (string) and `.attributes` (dict). In read-only contexts (`KindComputedEntity`, `KindTriggerCondition`, `KindWidgetCompute`) the same function is injected — the read-only constraint is enforced structurally (no write functions are present).

**`call_service(entity_id, capability, **kwargs) → result`**  
Dispatches via `CommandDispatcher`. Raises a Starlark error on failure. Only in `KindAutomation`, `KindScript`, `KindMCPEval` (when write-capable).

**`sleep(seconds)`**  
Implemented as `time.Sleep` with `ctx` cancellation check. Only in `KindAutomation` and `KindScript`.

**`now() → time.Time`**  
Returns current UTC time via `go.starlark.net/lib/time`.

**`log(msg, level="info")`**  
Writes via `slog.Logger` tagged with context kind and entity ID. Appended to `Result.Logs` for CLI streaming.

**`notify(target, message)`**  
Routes through `EventAppender`. Only in `KindAutomation` and `KindScript`.

**`random() → float`**  
`math/rand` seeded per execution. In `testutil`, seed is injected for determinism.

**`time`**  
The `go.starlark.net/lib/time` module, injected as a global.

**`scene` (struct global)**  
Injected in `KindAutomation` and `KindScript`. Has one method:
- `scene.apply(slug)` — dispatches a scene application via `EventAppender`.

**`event` (struct global, context-dependent)**  
- In `KindAutomation` and `KindScript`: has `.fire(kind, data)` method plus read fields from `extraGlobals` trigger payload (`.kind`, `.entity_id`, `.data`).
- In `KindTriggerCondition`: read-only struct with `.kind`, `.entity_id`, `.data` from the triggering event. No `.fire()` method.

---

## 5. Resource limits

Wall-clock and step limits per context (§4). Enforcement:

- **Step counter**: `thread.SetMaxSteps(n)` — `go.starlark.net` checks between steps and cancels with `ErrSteps` on breach.
- **Wall-clock**: a watchdog goroutine calls `thread.Cancel("timeout")` after the context deadline. The watchdog goroutine is always started and cancelled via a `defer` when `Execute` returns — no leak.
- **Memory**: Go GC is the effective cap. No explicit Starlark memory accounting in C5; a hard per-execution memory limit is a post-C5 enhancement if needed.

On any breach, `Execute` returns a `*LimitError` containing the limit kind and context. C6 uses this to emit an `AutomationFinished` event with `success=false` and the error detail.

```go
type LimitError struct {
    Kind    LimitKind   // LimitSteps | LimitWallClock
    Context ContextKind
    Detail  string
}
```

---

## 6. `load("//...")` resolver

`thread.Load` is set to a resolver that handles `//`-prefixed paths only.

```
load("//lib/helpers.star", "compute_temp")
```

Resolution:
1. Strip `//` prefix; resolve relative to `configDir`.
2. Reject any path that escapes `configDir` (contains `..` or resolves outside).
3. Check in-progress set for cycles; return error on cycle.
4. Check module cache (`Runtime.moduleCache`) — return cached `StringDict` on hit.
5. On miss: read file, execute in a fresh thread with `KindScript` limits, cache the exported symbols, return them.

Any URI scheme other than `//` (e.g. bare relative paths, `http:`, `file:` outside configDir) is rejected with a clear error.

The module cache is a `map[string]starlark.StringDict` keyed by absolute path, guarded by `sync.RWMutex`. `InvalidateModuleCache` clears it entirely on `ConfigApplied`.

---

## 7. Pkl ↔ Starlark bridging

C4 left `internal/config/pkl/gohome/starlark.pkl` with plain `String` typealiases and a comment pointing here. C5 activates them.

### 7.1 `ParseOnly` in `internal/starlark`

```go
// ParseOnly parses src as a Starlark expression (expr=true) or script (expr=false).
// Returns a syntax error if parsing fails; nil on success. Does not execute.
func ParseOnly(src string, expr bool) error
```

Uses `go.starlark.net/syntax.ParseExpr` / `syntax.Parse`. No `Runtime` required.

### 7.2 Validator ModuleReader in `internal/config/evaluator.go`

A second `ModuleReader` is registered for the `gohome-validator:` scheme. When Pkl calls `isValidStarlarkExpr(src)`, the reader invokes `starlark.ParseOnly(src, true)` and returns `"true"` or `"false"` to Pkl. Same for `isValidStarlarkScript` and `isValidStarlarkCondition`.

### 7.3 `starlark.pkl` update

Replace C4 stubs:

```pkl
module gohome.starlark

typealias StarlarkExpr      = String(isValidStarlarkExpr(this))
typealias StarlarkScript    = String(isValidStarlarkScript(this))
typealias StarlarkCondition = String(isValidStarlarkCondition(this))

@External
external function isValidStarlarkExpr(src: String): Boolean

@External
external function isValidStarlarkScript(src: String): Boolean

@External
external function isValidStarlarkCondition(src: String): Boolean
```

After C5, `gohome config validate` returns a structured error with file and line for any Starlark syntax mistake in a `.pkl` config file.

---

## 8. `gohome eval` CLI command

Dials the running daemon's Unix socket. Protocol reuses the existing socket JSON handler:

**Request:**
```json
{ "op": "starlark_eval", "script": "...", "context": "automation" }
```

**Streaming response** (one JSON line per `log()` call):
```json
{ "type": "log", "level": "info", "msg": "lights are off" }
```

**Final response:**
```json
{ "type": "result", "ok": true, "value": "None", "elapsed_ms": 42, "steps": 1234 }
{ "type": "result", "ok": false, "error": "line 3: undefined: state", "elapsed_ms": 5 }
```

**CLI flags:**
- `--context automation|computed|condition|script|mcp` (default `automation`)
- `--data-dir` inherited from global flags for socket path

**Output styling (lipgloss):**
- `log()` lines: `Dim` style prefix `[log]`, message unstyled
- Final value (non-None): `EntityID` style
- Elapsed + steps: `Dim`
- Errors: `Error` style, to stderr
- Success indicator: `Success` style

Exit 0 on success, 1 on error.

---

## 9. `gohome test` CLI command

Runs `.star` files where functions named `test_*` are test cases. Each function receives no arguments and may call `assert(cond, msg)`. Execution uses the live daemon (same as `gohome eval`) so tests run against real state.

**Invocation:**
```bash
gohome test automations/test_lighting.star
gohome test automations/   # all *_test.star files in directory
```

**Output styling (lipgloss):**
```
--- PASS: test_lights_turn_on  (12ms / 1 234 steps)
--- FAIL: test_scene_evening   (8ms / 890 steps)
    line 4: assertion failed: expected scene to apply
FAIL
```

- `PASS`: `Success` style
- `FAIL` labels and summary: `Error` style
- Elapsed / steps: `Dim`
- File/line in failure messages: `EntityID` style

Exit 0 if all tests pass, 1 if any fail.

---

## 10. `internal/starlark/testutil` package

Provides a pre-wired `Runtime` with injectable fakes for use in Go tests (primarily C6).

```go
// FakeState maps entity ID to state for test injection.
type FakeState map[string]*EntityState

// FakeDispatcher records all Dispatch calls for assertion.
type FakeDispatcher struct {
    mu    sync.Mutex
    Calls []DispatchCall
}

type DispatchCall struct {
    EntityID   string
    Capability string
    Args       map[string]string
}

// NewTestRuntime returns a Runtime with fake dependencies.
// seed controls random() output for determinism (0 = random seed).
func NewTestRuntime(state FakeState, dispatcher *FakeDispatcher, seed int64) *Runtime

// RunScript executes script in the given context kind and returns the result.
// Calls t.Fatal on execution error unless the test expects one.
func RunScript(t testing.TB, rt *Runtime, kind ContextKind, script string) *ScriptResult

type ScriptResult struct {
    Value   starlark.Value
    Logs    []string
    Steps   uint64
    Elapsed time.Duration
}

// Assertion helpers
func AssertValue(t testing.TB, result *ScriptResult, expected starlark.Value)
func AssertLog(t testing.TB, result *ScriptResult, contains string)
func AssertCallService(t testing.TB, d *FakeDispatcher, entityID, capability string)
func AssertNoCallService(t testing.TB, d *FakeDispatcher)
func AssertError(t testing.TB, err error, contains string)
```

Example usage (C6 test):

```go
d := &testutil.FakeDispatcher{}
rt := testutil.NewTestRuntime(
    testutil.FakeState{"light.kitchen": {State: "off"}},
    d,
    0,
)
res := testutil.RunScript(t, rt, starlark.KindAutomation,
    `if state("light.kitchen").state == "off":
         call_service("light.kitchen", "turn_on", brightness=80)`)
testutil.AssertCallService(t, d, "light.kitchen", "turn_on")
```

---

## 11. File map

### New files

| Path | Responsibility |
|---|---|
| `internal/starlark/runtime.go` | `Runtime` struct, `NewRuntime`, `Execute`, `InvalidateModuleCache` |
| `internal/starlark/context.go` | `ContextKind` enum, per-context stdlib builders, resource limit dispatch table |
| `internal/starlark/stdlib.go` | Stdlib builtin implementations: `state`, `call_service`, `sleep`, `now`, `log`, `notify`, `scene`, `event`, `random` |
| `internal/starlark/loader.go` | `load("//...")` resolver, module cache |
| `internal/starlark/limits.go` | `LimitError`, watchdog goroutine, `SetMaxSteps` wiring |
| `internal/starlark/parse.go` | `ParseOnly(src string, expr bool) error` |
| `internal/starlark/testutil/testutil.go` | `NewTestRuntime`, `RunScript`, `FakeState`, `FakeDispatcher`, assertion helpers |
| `internal/starlark/runtime_test.go` | Unit tests for `Execute`, resource limits, stdlib scoping |
| `internal/starlark/loader_test.go` | Unit tests for `load()` resolver, cache invalidation, cycle detection |
| `internal/starlark/parse_test.go` | Unit tests for `ParseOnly` |
| `internal/cli/eval.go` | `gohome eval` Cobra command |
| `internal/cli/test.go` | `gohome test` Cobra command |

### Modified files

| Path | Change |
|---|---|
| `internal/config/pkl/gohome/starlark.pkl` | Activate `isValidStarlark*()` validators (replace C4 stubs) |
| `internal/config/evaluator.go` | Register `gohome-validator:` ModuleReader; call `starlark.ParseOnly` |
| `internal/cli/root.go` | Register `newEvalCmd` and `newTestCmd` |
| `internal/daemon/daemon.go` | Construct `starlark.Runtime` after config load; register `starlark_eval` socket handler; call `runtime.InvalidateModuleCache` on `ConfigApplied` |

---

## 12. Testing strategy

**Unit tests (`internal/starlark/`):**
- Each stdlib builtin tested in isolation via `testutil.NewTestRuntime`
- Context scoping: assert `call_service` raises in `KindComputedEntity`; `sleep` raises in `KindTriggerCondition`
- Resource limits: step counter exceeded → `LimitError{Kind: LimitSteps}`; short wall-clock timeout with a `sleep` call → `LimitError{Kind: LimitWallClock}`
- `load()`: path traversal rejection; circular load detection; cache hit vs miss; cache invalidation clears prior entries
- `ParseOnly`: valid expression, valid script, syntax errors with line/column

**Unit tests (`internal/config/`):**
- Pkl validator activation: fixture `.pkl` with broken `StarlarkExpr` value asserts `EvalError` from `gohome config validate`

**Integration tests (build tag `integration`):**
- Full `Execute` path with real `go.starlark.net` execution and `FakeState`/`FakeDispatcher`
- Wall-clock limit with real `time.Sleep` in the script

**CLI tests:**
- `gohome eval`: stubbed daemon socket double returns canned results; assert lipgloss-formatted stdout and exit codes
- `gohome test`: stubbed socket double returns pass/fail results; assert coloured output and exit codes

---

## 13. Decision record

| # | Decision | Rationale |
|---|---|---|
| DR-1 | Single `Execute(ctx, kind, script, extraGlobals)` entry point | Minimal public API; context dispatch is internal; unified `Result` simplifies test harness |
| DR-2 | `Runtime` struct owns all daemon dependencies | One construction site; per-context stdlib scoping via dispatch table, not varying deps |
| DR-3 | `scene` and `event` are injected struct globals with method attributes | Matches master design dotted form; reads naturally in Starlark |
| DR-4 | `event` in `KindTriggerCondition` is read-only (`.kind`, `.entity_id`, `.data`) | Trigger conditions are pure — no side effects by design |
| DR-5 | Wall-clock via watchdog goroutine calling `thread.Cancel()` | `go.starlark.net` has no built-in wall-clock; watchdog is the canonical pattern |
| DR-6 | `load()` accepts `//`-prefixed paths only, resolved from `configDir` | No network or arbitrary filesystem access |
| DR-7 | Module cache on `Runtime`, invalidated on `ConfigApplied` | Edited `.star` files take effect on next config apply without daemon restart |
| DR-8 | `gohome test` runs against live daemon, not offline | Tests against real state are more valuable; offline syntax checking is already `gohome config validate` |
| DR-9 | Pkl validator activation wired via `gohome-validator:` ModuleReader in `evaluator.go` | C4 already has the ExternalReader infrastructure; no new plumbing needed |
| DR-10 | All C5 CLI output uses lipgloss styles from `internal/cli/styles.go` | Consistency with rest of CLI; no new style definitions |
| DR-11 | No explicit Starlark memory accounting in C5 | Go GC is the effective cap; a hard per-execution limit is a post-C5 enhancement if needed |

---

## 14. Task breakdown

1. **`go.starlark.net` dependency** — `go get go.starlark.net@latest`, `go mod tidy`
2. **`internal/starlark/parse.go`** — `ParseOnly`; unit tests
3. **Pkl validator activation** — update `starlark.pkl`; register `gohome-validator:` ModuleReader in `evaluator.go`; integration test
4. **`internal/starlark/limits.go`** — `LimitError`, watchdog goroutine; unit tests
5. **`internal/starlark/stdlib.go`** — all builtin implementations; unit tests per builtin
6. **`internal/starlark/context.go`** — `ContextKind` enum, per-context stdlib builders, resource dispatch table
7. **`internal/starlark/loader.go`** — `load("//...")` resolver, module cache, `InvalidateModuleCache`; unit tests
8. **`internal/starlark/runtime.go`** — `Runtime`, `NewRuntime`, `Execute`; unit tests
9. **`internal/starlark/testutil/testutil.go`** — `NewTestRuntime`, `RunScript`, assertion helpers
10. **`gohome eval` CLI** — `internal/cli/eval.go`; daemon socket handler; lipgloss output; CLI tests
11. **`gohome test` CLI** — `internal/cli/test.go`; lipgloss output; CLI tests
12. **Daemon wiring** — construct `Runtime` after config load; register socket handler; wire `InvalidateModuleCache` on `ConfigApplied`
13. **Definition of done** — `task build`, `task test`, `task test:race`, `task test:integration`, `task lint`, `go mod tidy`
