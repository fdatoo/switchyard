# C6 — Automation & Script Engine Design

**Status:** Draft
**Date:** 2026-04-23
**Depends on:** C1 (Event Core), C4 (Pkl Config), C5 (Starlark Runtime)
**Unblocks:** C7 (Connect-RPC), C8 (MCP), C10 (Web UI)

---

## 1. Scope

The automation and script engine turns typed Pkl declarations into live handlers. It owns:

- Extending `gohome.automations` and introducing `gohome.scripts` Pkl modules.
- Compiling a `ConfigSnapshot` into an immutable runtime graph of automations and scripts.
- Matching incoming events against triggers (state changes, arbitrary events, time) and admitting runs under per-automation concurrency modes.
- Evaluating conditions and dispatching actions including nested sequence/parallel blocks.
- Running scripts as named callable Starlark functions invokable from automations, CLI, and (future) RPC/MCP/UI.
- Emitting `AutomationTriggered` / `AutomationFinished` / `ScriptInvoked` / `ScriptFinished` events for full history and tracing.
- CLI commands: `gohome automation {list,get,enable,disable,trigger,watch,trace}` and `gohome script {list,run}`.
- Surgical reload on `ConfigApplied` that preserves in-flight runs for unchanged automations.

**Explicit non-goals:**

- No Connect-RPC definitions (C7).
- No MCP tool wiring (C8).
- No web UI (C10).
- No webhook HTTP endpoint — `WebhookTrigger` is Pkl-validated but inert until C7.
- No sunrise/sunset trigger — emitted by a future `sun` driver as ordinary entity state changes.
- No scene resolution engine — C6 defines a `SceneApplier` dep interface; the concrete impl belongs to a later scene spec.
- No dedicated history projector — the event log is authoritative; a projector is a v1.x optimization if query latency ever hurts.
- No parallel blocks as a `mode` — covered by the `ParallelBlock` action.
- No persisted enable/disable override — in-memory only in v1; durable overrides are v1.x.

---

## 2. Background

Master design §6 calls for "trigger → conditions → actions" automations declared in Pkl, with Starlark used only where logic is required, and scripts as named callable Starlark functions. C4 shipped a thin `gohome.automations` stub (`Trigger{kind, entityId, condition}`, `Action{kind, body}`, `Automation{id, trigger, actions, enabled}`) that is too weak for v1. C5 is building the Starlark runtime that C6 sits on top of — `Runtime.Execute(ctx, kind, script, extraGlobals)` with `KindAutomation`, `KindTriggerCondition`, `KindScript` contexts, resource limits, and a `//`-resolver.

C6 replaces the C4 Pkl stub with a richly typed schema, adds `gohome.scripts`, and builds the runtime engine that compiles, matches, evaluates, and dispatches.

---

## 3. Architecture Overview

### 3.1 Package map

```
internal/automation/
├── engine.go                    # automation.Engine: Start/Stop/Reload lifecycle
├── automation.go                # runtime *Automation struct
├── compile.go                   # Compile(snapshot, scriptEngine, runtime, deps)
├── mode.go                      # runState + mode state machine
├── reload.go                    # surgical diff reload
├── run.go                       # Run struct + executeRun + outcome classification
├── errors.go                    # CompileError, DispatchError
├── trigger/
│   ├── trigger.go               # Matcher interface + Registry
│   ├── state.go                 # StateChangeMatcher (incl. forDur hold timers)
│   ├── event.go                 # EventMatcher
│   ├── time.go                  # TimeScheduler (robfig/cron/v3)
│   └── fake.go                  # test doubles
├── condition/
│   ├── condition.go             # Evaluator interface + Env
│   ├── typed.go                 # StateCondition, NumericCondition, TimeCondition
│   ├── starlark.go              # StarlarkCondition
│   └── compose.go               # And/Or/Not
├── action/
│   ├── action.go                # Executor interface
│   ├── callservice.go
│   ├── scene.go                 # + SceneApplier dep interface
│   ├── script.go
│   ├── starlark.go
│   ├── wait.go
│   └── block.go                 # SequenceBlock, ParallelBlock
└── testutil/
    ├── fake_scenes.go
    ├── synth.go
    └── fixtures/

internal/script/
├── engine.go                    # script.Engine: Call, List, Get, Reload, Stop
├── script.go                    # runtime *Script struct
├── compile.go
├── reload.go
├── errors.go
└── testutil/synth.go

internal/cli/
├── cmd_automation.go
├── cmd_script.go
└── styles_automation.go         # RunMarker, Correlation, Duration styles
```

`internal/automation` and `internal/script` are peer packages. The dependency flows one way: `internal/automation` imports `internal/script` (for `ScriptAction`). Both depend on `internal/starlark` (C5). This reflects the real runtime relationship — scripts are independently invokable, automations call into scripts.

### 3.2 Wiring at daemon startup

```
config.Manager ─┬─► ConfigSnapshot ─► script.Compile  ──► *script.Engine
                │                                              │
                └─► ConfigSnapshot ─► automation.Compile ──────┤
                                       (snapshot, engine)      │
                                       ─► *automation.Engine ◄─┘

daemon:
  1. constructs script.Engine, automation.Engine
  2. calls engine.Start(ctx) — subscribes to eventstore, starts time scheduler
  3. registers socket op handlers for automation_* and script_*
  4. subscribes config.Manager to call script.Reload then automation.Reload on ConfigApplied
```

### 3.3 Data flow of one automation run

```
event.StateChanged (from driver)
       │
       ▼
eventstore subscription
       │
       ▼
engine.onEvent  ──►  trigger.Registry.Dispatch(event)  ──► [Match₁, Match₂]
                                                              │
                                                              ▼
                                                    mode state machine
                                                              │ (admit / drop / queue / cancel-prior)
                                                              ▼
                                              run := newRun(auto, triggerEvent)
                                              emit AutomationTriggered{id, run.corr_id, position, trigger_kind}
                                                              │
                                                              ▼
                                              eval conditions (AND) ── fail ──► AutomationFinished{CONDITION_FAIL}
                                                              │ pass
                                                              ▼
                                              dispatch actions (top-level implicit SequenceBlock)
                                                 ├─ CallService → carport.Dispatch
                                                 ├─ Scene       → SceneApplier.Apply
                                                 ├─ Script      → script.Engine.Call (same corr_id)
                                                 ├─ Starlark    → runtime.Execute(KindAutomation, ...)
                                                 └─ Wait/Block  → scheduler / nested executors
                                                              │
                                                              ▼
                                              emit AutomationFinished{outcome, elapsed, steps, logs, error?}
```

Every run has a UUID correlation_id on its `AutomationTriggered` and `AutomationFinished` events. It also threads into `call_service` dispatch (via context, stamped onto `CommandIssued.source` as `"automation:<id>#<corr>"`) and into nested script calls (same corr_id, different event kinds), so one `gohome automation trace <corr_id>` shows the full causal chain.

Reload is driven by `ConfigApplied`: `engine.Reload(newSnapshot)` diffs old vs new, unregisters removed, registers added, re-compiles changed, leaves unchanged runs alone.

---

## 4. Pkl Schema

### 4.1 `gohome.automations` (replaces C4 stub)

```pkl
module gohome.automations

import "gohome:starlark" as starlark

// ── Triggers ────────────────────────────────────────────────
abstract class Trigger

class StateChangeTrigger extends Trigger {
  entities: Listing<String(!isEmpty)>
  from:     String?
  to:       String?
  forDur:   Duration?
}

class EventTrigger extends Trigger {
  kind: String(!isEmpty)                 // e.g. "driver_event.hue.button_press"
  data: Mapping<String, String>?         // equality filters on event data
}

class TimeTrigger extends Trigger {
  at:    String?                          // "HH:MM" local time, daily
  cron:  String?                          // full cron expression
  every: Duration?
}                                         // exactly one of at/cron/every

class WebhookTrigger extends Trigger {
  path:    String(matches(Regex(#"^/[a-zA-Z0-9/_-]+$"#)))
  methods: Listing<String> = new { "POST" }
}                                         // validated in C6, wired by C7

// ── Conditions ──────────────────────────────────────────────
abstract class Condition

class StateCondition extends Condition {
  entity: String(!isEmpty)
  equals: String?
  oneOf:  Listing<String>?
  not:    String?
}                                         // exactly one operator

class NumericCondition extends Condition {
  entity:    String(!isEmpty)
  attribute: String = "value"
  op:        String(oneOf("lt","lte","eq","gte","gt"))
  value:     Number
}

class TimeCondition extends Condition {
  after:    String?                       // "HH:MM" local
  before:   String?
  weekdays: Listing<String>?              // ["mon","tue",...]
}

class StarlarkCondition extends Condition {
  expr: starlark.StarlarkCondition        // single-expression, runs in KindTriggerCondition
}

class AndCondition extends Condition { all: Listing<Condition> }
class OrCondition  extends Condition { any: Listing<Condition> }
class NotCondition extends Condition { not: Condition }

// ── Actions ─────────────────────────────────────────────────
abstract class Action {
  continueOnError: Boolean = false
}

class CallServiceAction extends Action {
  entity:     String(!isEmpty)
  capability: String(!isEmpty)
  args:       Mapping<String, String>?
}

class SceneAction   extends Action { slug: String(!isEmpty) }
class ScriptAction  extends Action { name: String(!isEmpty); args: Mapping<String, String>? }
class StarlarkAction extends Action { body: starlark.StarlarkScript }
class WaitAction    extends Action { duration: Duration }

class SequenceBlock extends Action { actions: Listing<Action> }
class ParallelBlock extends Action { actions: Listing<Action> }

// ── Automation ──────────────────────────────────────────────
class Automation {
  id:         String(!isEmpty)
  triggers:   Listing<Trigger>                                  // ANY-match semantics
  conditions: Listing<Condition> = new {}                       // ALL-pass (AND)
  actions:    Listing<Action>                                   // implicit sequence
  mode:       String(oneOf("single","queued","restart","parallel")) = "single"
  maxQueued:  Int = 10                                          // only used when mode="queued"
  enabled:    Boolean = true
}
```

### 4.2 `gohome.scripts` (new)

```pkl
module gohome.scripts

import "gohome:starlark" as starlark

class ScriptParam {
  name:     String(!isEmpty)
  type:     String(oneOf("string","int","float","bool","entity_id"))
  required: Boolean = true
  default:  String?                       // stringified; parsed against type at compile
}

class Script {
  name:    String(!isEmpty)
  params:  Listing<ScriptParam> = new {}
  handler: starlark.StarlarkScript        // runs in KindScript
}
```

### 4.3 Authoring conventions

- One trigger or multiple (ANY matches).
- Short Starlark stays inline via triple-quoted Pkl strings. Anything longer moves to a `.star` file and is either `load()`ed inside the body or registered as a `Script` and invoked via `ScriptAction`.
- `WaitAction` is scheduler-tracked — it does not pin a Starlark thread for the duration.
- Place cheap typed conditions before Starlark ones inside `AndCondition` to short-circuit evaluation.

---

## 5. Proto Additions

### 5.1 `proto/gohome/event/v1/event.proto`

New payload variants (reserving 50-59 for the automation plane):

```proto
message Payload {
  oneof kind {
    // ...1-49 existing...
    // 50-59: automation/script plane
    AutomationTriggered automation_triggered = 50;
    AutomationFinished  automation_finished  = 51;
    ScriptInvoked       script_invoked       = 52;
    ScriptFinished      script_finished      = 53;
  }
}

message AutomationTriggered {
  string automation_id           = 1;
  string correlation_id          = 2;   // UUID per run
  uint64 trigger_event_position  = 3;   // 0 for time / manual triggers
  string trigger_kind            = 4;   // "state_changed" | "event" | "time" | "manual"
  string invoked_by              = 5;   // "cli:<user>" / "api:<token>" / "" for subscribed
}

// Top-level enum so both AutomationFinished and ScriptFinished can reference it.
enum RunOutcome {
  OUTCOME_UNSPECIFIED    = 0;
  OUTCOME_OK             = 1;
  OUTCOME_CONDITION_FAIL = 2;   // automation-only (scripts have no conditions)
  OUTCOME_ACTION_ERROR   = 3;
  OUTCOME_LIMIT_EXCEEDED = 4;
  OUTCOME_CANCELLED      = 5;
  OUTCOME_SKIPPED        = 6;   // automation-only (scripts have no admission gate)
}

message AutomationFinished {
  string   automation_id  = 1;
  string   correlation_id = 2;
  RunOutcome outcome      = 3;
  string   error          = 4;
  int64    elapsed_ms     = 5;
  uint64   starlark_steps = 6;
  repeated string log_lines = 7;
}

message ScriptInvoked {
  string              script_name    = 1;
  string              correlation_id = 2;
  string              invoked_by     = 3;  // "cli:<user>" | "automation:<id>" | "api:<token>"
  map<string, string> args           = 4;
}

message ScriptFinished {
  string   script_name    = 1;
  string   correlation_id = 2;
  RunOutcome outcome      = 3;
  string   error          = 4;
  int64    elapsed_ms     = 5;
  uint64   starlark_steps = 6;
  repeated string log_lines = 7;
  string   return_value   = 8;                   // Starlark repr() of return
}
```

### 5.2 `proto/gohome/config/v1/snapshot.proto`

```proto
message ConfigSnapshot {
  // ...existing...
  repeated AutomationConfig automations = 12;
  repeated ScriptConfig     scripts     = 13;
}

message AutomationConfig {
  string                   id         = 1;
  repeated TriggerConfig   triggers   = 2;
  repeated ConditionConfig conditions = 3;
  repeated ActionConfig    actions    = 4;
  Mode                     mode       = 5;
  int32                    max_queued = 6;
  bool                     enabled    = 7;

  enum Mode {
    MODE_SINGLE   = 0;
    MODE_QUEUED   = 1;
    MODE_RESTART  = 2;
    MODE_PARALLEL = 3;
  }
}

message TriggerConfig {
  oneof kind {
    StateChangeTrigger state_change = 1;
    EventTrigger       event        = 2;
    TimeTrigger        time         = 3;
    WebhookTrigger     webhook      = 4;
  }
}

message ConditionConfig {
  oneof kind {
    StateCondition    state    = 1;
    NumericCondition  numeric  = 2;
    TimeCondition     time     = 3;
    StarlarkCondition starlark = 4;
    AndCondition      and      = 5;
    OrCondition       or       = 6;
    NotCondition      not      = 7;
  }
}

message ActionConfig {
  bool continue_on_error = 1;
  oneof kind {
    CallServiceAction call_service = 2;
    SceneAction       scene        = 3;
    ScriptAction      script       = 4;
    StarlarkAction    starlark     = 5;
    WaitAction        wait         = 6;
    SequenceBlock     sequence     = 7;
    ParallelBlock     parallel     = 8;
  }
}

// Individual trigger/condition/action messages mirror their Pkl classes field-for-field.
// (omitted here; spelled out in the proto file)

message ScriptConfig {
  string               name    = 1;
  repeated ScriptParam params  = 2;
  string               handler = 3;
}

message ScriptParam {
  string name     = 1;
  Type   type     = 2;
  bool   required = 3;
  string default  = 4;

  enum Type {
    TYPE_UNSPECIFIED = 0;
    TYPE_STRING      = 1;
    TYPE_INT         = 2;
    TYPE_FLOAT       = 3;
    TYPE_BOOL        = 4;
    TYPE_ENTITY_ID   = 5;
  }
}
```

Field groupings follow `docs/proto-hygiene.md`. `task proto` regenerates `gen/`; staged and committed alongside the implementation.

---

## 6. Compilation

Two entry points, both pure (no I/O):

```go
// internal/script/compile.go
func Compile(
    snapshot *configpb.ConfigSnapshot,
    runtime  *starlark.Runtime,
    deps     Deps,
) (*Engine, error)

// internal/automation/compile.go
func Compile(
    snapshot     *configpb.ConfigSnapshot,
    scriptEngine *script.Engine,
    runtime      *starlark.Runtime,
    deps         Deps,
) (*Engine, error)
```

`Deps` is a small struct each package defines for injection — the dependencies the engine needs at runtime but doesn't construct itself:

```go
// internal/automation/deps.go  (and analogous in internal/script)
type Deps struct {
    State      state.Reader             // internal/state.Cache
    Dispatcher carport.Dispatcher       // internal/carport.Host
    Store      eventstore.Appender      // internal/eventstore.Store
    Scenes     action.SceneApplier      // stub in v1; see §10.1
    Logger     *slog.Logger             // internal/observability
    Metrics    *observability.Metrics
    Tracer     tracing.Tracer
    Clock      func() time.Time          // injectable for tests; defaults to time.Now
}
```

Script engine `Deps` is a subset (no `Dispatcher`, no `Scenes`).

### 6.1 Script compilation

For each `ScriptConfig`:
1. `starlark.ParseOnly(handler, false)` — belt-and-braces guard; runtime never panics on bad input.
2. Validate param types; coerce and cache default values per declared `Type`.
3. Build a `*Script`; index by name; duplicate names are an aggregated error.

### 6.2 Automation compilation

The runtime `Automation` is immutable:

```go
type Automation struct {
    ID         string
    Triggers   []trigger.Matcher     // ANY-match
    Conditions []condition.Evaluator // AND
    Actions    []action.Executor     // implicit top-level sequence
    Mode       Mode
    MaxQueued  int
    Enabled    bool
}
```

The compiler walks each `AutomationConfig` and:

1. **Validates structural invariants** Pkl can't express. `TimeTrigger` must have exactly one of `at`/`cron`/`every`. `StateCondition` must have exactly one operator. Empty `AndCondition.all` / `OrCondition.any` are errors.
2. **Builds trigger matchers.** Parses cron strings via `github.com/robfig/cron/v3`. Validates entity IDs are well-formed (`<type>.<name>`). Unknown entity IDs in triggers are a compile **warning** (drivers may register entities post-apply), not an error.
3. **Builds condition evaluators.** Typed variants compile to native Go evaluators. `StarlarkCondition` compiles to a closure that calls `runtime.Execute(KindTriggerCondition, expr, ...)`. Every Starlark expression is `ParseOnly`-guarded.
4. **Builds action executors recursively.** Leaf actions compile to their executors; `SequenceBlock`/`ParallelBlock` recurse and wrap child executors. Finite AST — no cycles to detect.
5. **Resolves script references.** `ScriptAction{name}` must resolve against the compiled script registry.

### 6.3 Error aggregation

The compiler accumulates all errors and returns a `CompileError` carrying each `ItemError`:

```go
type ItemError struct {
    ID      string    // automation or script ID
    Path    string    // "automations[evening_lights].actions[2].sequence.actions[0].call_service.entity"
    Reason  string    // human-readable
}
```

Authors get every problem at once from `gohome config validate`, not one at a time.

### 6.4 Trigger dedup

Two automations may share identical state-change triggers. The compiler produces separate `Matcher` instances; the registry indexes by `(kind, entity)` so a single inbound event fans out cheaply.

### 6.5 Output

`Compile` returns `*Engine` populated but not started. `Engine.Start(ctx)` subscribes to the event store and starts the scheduler — keeping `Compile` pure makes it trivially testable.

```go
type Engine struct {
    automations map[string]*Automation
    triggers    *trigger.Registry
    scheduler   *trigger.TimeScheduler
    scripts     *script.Engine        // borrowed, not owned
    runtime     *starlark.Runtime     // borrowed
    deps        Deps

    mu       sync.Mutex
    runStates map[string]*runState    // per-automation mode state
    inFlight sync.WaitGroup
}
```

---

## 7. Trigger Matching

### 7.1 Subscription

`Engine.Start(ctx)` calls `eventstore.Store.Subscribe(ctx, fromPosition, filter)` with a filter selecting `StateChanged`, `DriverEvent`, `SystemEvent`, and custom events fired via `event.fire()`. `fromPosition` is the current tail — we never replay history into triggers (no re-firing of last week's door-opens).

### 7.2 Registry

```go
type Registry struct {
    mu            sync.RWMutex
    stateByEntity map[string][]*StateChangeMatcher
    eventByKind   map[string][]*EventMatcher
}

func (r *Registry) Dispatch(e eventstore.Event) []Match
```

Index lookups are O(1). A `Match` carries the matched automation ID, the trigger that fired, and the originating event.

### 7.3 `StateChangeMatcher`

```go
type StateChangeMatcher struct {
    automationID string
    entities     map[string]struct{}
    from, to     string            // "" = any
    forDur       time.Duration     // 0 = no hold

    mu     sync.Mutex
    timers map[string]*time.Timer  // per-entity
}
```

Per inbound `StateChanged`:
1. Entity in `entities`? miss → return.
2. `from` set and prior state mismatches → cancel any hold timer; return.
3. `to` set and new state mismatches → cancel any hold timer; return.
4. `forDur == 0` → emit match immediately.
5. Otherwise `time.AfterFunc(forDur, emitMatch)`; cancel if an intervening state change breaks the match.

Per-entity timers; on engine shutdown all timers are drained.

### 7.4 `EventMatcher`

```go
type EventMatcher struct {
    automationID string
    kind         string            // exact match
    data         map[string]string // ALL entries must equal
}
```

Wildcard/prefix matches are v1.x. Authors needing patterns today write a `StarlarkCondition` evaluating `event.kind.startswith("driver_event.hue.")`.

### 7.5 `TimeScheduler`

```go
type TimeScheduler struct {
    loc     *time.Location
    mu      sync.Mutex
    entries []scheduledEntry    // min-heap
    wakeCh  chan struct{}       // break sleep on registration change
    ready   chan Match
}
```

`at`, `cron`, and `every` normalize to `cronv3.Schedule`:

- `at: "07:30"` → `Parse("30 7 * * *")`
- `cron` → `Parse` directly
- `every: 15m` → a small `everySchedule` implementing the interface (`Next(last)` = `last.Add(interval)`)

Loop sleeps until earliest `nextFire`, delivers a `Match`, recomputes, resifts. Reload uses `wakeCh` to interrupt. Local system TZ from daemon env at startup; per-trigger TZ override is v1.x.

### 7.6 Engine inner loop

```go
for {
    select {
    case ev := <-eventSub.Out():
        for _, m := range e.triggers.Dispatch(ev) {
            go e.fire(ctx, m)      // admission via mode state machine
        }
    case m := <-e.scheduler.Ready():
        go e.fire(ctx, m)
    case <-ctx.Done():
        e.scheduler.Stop()
        e.inFlight.Wait()
        return
    }
}
```

---

## 8. Condition Evaluation

Evaluated after trigger match, before any action. Any `false` or error → `AutomationFinished{OUTCOME_CONDITION_FAIL}`; no actions run.

```go
type Evaluator interface {
    Evaluate(ctx context.Context, env Env) (bool, error)
}

type Env struct {
    State StateReader
    Event *eventstore.Event   // nil for time/manual triggers
    Now   time.Time
    Loc   *time.Location
}
```

**Typed evaluators:**

- `StateCondition` — reads state; missing entity → `false` + warn log.
- `NumericCondition` — reads attribute, parses as `float64`; parse failure → `false` + log.
- `TimeCondition` — `env.Now` in `env.Loc` against `after`/`before` (overnight-aware) and `weekdays`.

**Starlark:**

- `StarlarkCondition` — `runtime.Execute(KindTriggerCondition, expr, {event, now})`; `state` in stdlib per C5. Result's Starlark truthiness decides. `LimitError`, parse errors, runtime errors all treated as `false` + warn log (no run abort) — matches HA mental model where condition errors silently skip.

**Composition:**

- `AndCondition.all` — short-circuits on first `false`.
- `OrCondition.any` — short-circuits on first `true`.
- `NotCondition.not` — inverts result; preserves (does not invert) errors.

**Ordering.** Declaration order from Pkl; not auto-reordered. Authors gate expensive Starlark conditions behind cheap typed ones.

---

## 9. Observability

Follows `internal/observability` conventions strictly — no new logger, no parallel Prometheus registry, no alternative tracer.

### 9.1 Logging (slog)

Standard attribute set on every C6 log line:

```go
slog.String("component", "automation")  // or "script"
slog.String("automation_id", id)         // or "script_name"
slog.String("correlation_id", runID)
slog.String("phase", "trigger|condition|action|run")
```

Levels:

- **DEBUG** — trigger matches, condition outcomes, action start/finish.
- **INFO** — mode-state-machine decisions (drop/cancel/queue).
- **WARN** — condition errors, `continueOnError` action failures, recoverable limit breaches.
- **ERROR** — compile errors at reload, unhandled action failures that abort a run.

User `log()` calls inside Starlark route through the same logger and are additionally captured into `AutomationFinished.log_lines`.

### 9.2 Metrics

Registered via `observability.Metrics`. Namespaces: `gohome_automation_*`, `gohome_script_*`.

```
# Counters
gohome_automation_triggers_total{automation_id, trigger_kind}
gohome_automation_runs_total{automation_id, outcome}
gohome_automation_conditions_total{automation_id, result}
gohome_automation_actions_total{automation_id, action_kind, result}
gohome_automation_reload_failures_total

gohome_script_invocations_total{script_name, outcome, invoked_by_kind}

# Histograms
gohome_automation_run_duration_seconds{automation_id}
gohome_automation_starlark_steps{automation_id}
gohome_script_duration_seconds{script_name}

# Gauges
gohome_automation_inflight{automation_id}
gohome_automation_registered
gohome_script_registered
```

Cardinality: `automation_id` and `script_name` are config-bounded (~O(100)). `invoked_by_kind` is the prefix before `:` (`cli`, `automation`, `api`, `mcp`) — bounded. No `entity_id` or `correlation_id` labels.

### 9.3 Tracing (OTel stubs)

Uses `internal/observability/tracing.StartSpan`. Span hierarchy:

```
automation.run          attrs: automation.id, correlation_id, trigger.kind
 ├─ automation.conditions       attrs: condition.count
 │   └─ automation.condition    attrs: condition.kind, condition.index, condition.result
 ├─ automation.actions
 │   ├─ automation.action       attrs: action.kind, action.index, action.result
 │   │   └─ starlark.execute    (inherited from C5)
 │   └─ ...
 └─ (outcome set on run span before End)

script.call             attrs: script.name, correlation_id, invoked_by
 └─ starlark.execute
```

`CallServiceAction` stamps correlation_id into `CommandIssued.source` as `"automation:<id>#<corr>"`, so downstream (C3) spans carry the lineage.

Default Grafana dashboards are C13 scope; C6 documents the metric names so dashboards have stable targets.

---

## 10. Action Dispatch

```go
type Executor interface {
    Execute(ctx context.Context, run *Run) error
}

type Run struct {
    CorrelationID string
    AutomationID  string                 // empty for Script runs
    State         StateReader
    Dispatcher    CommandDispatcher
    Store         EventAppender
    Scenes        SceneApplier
    Scripts       *script.Engine
    Runtime       *starlark.Runtime
    Logger        *slog.Logger
    Tracer        tracing.Tracer
    TriggerEvent  *eventstore.Event      // nil for time/manual triggers

    mu    sync.Mutex
    Steps uint64
    Logs  []string
}
```

`Run.Steps` and `Run.Logs` are guarded by `mu` — parallel blocks mutate them concurrently.

### 10.1 Leaf executors

- **`CallServiceAction`** — `Dispatcher.Dispatch(ctxWithCorrID, entity, capability, args)`. Correlation propagated via ctx value; stamped onto `CommandIssued.source`. `!result.Ok` returns `DispatchError{entity, capability, result.Error}`.
- **`SceneAction`** — `Scenes.Apply(ctx, slug, corrID)`. `SceneApplier` is a dep interface; v1 daemon wires a stub that emits `SceneApplied` and warn-logs "scene engine not yet implemented". A real scene-engine replaces the stub in a later spec without changing C6.
- **`ScriptAction`** — `Scripts.Call(ctx, name, args, invokedBy, sharedCorrID)`. `sharedCorrID = run.CorrelationID` so the script runs under the automation's corr_id. `invokedBy = "automation:<id>"`. Arg validation (required params, type coercion) lives in `Scripts.Call`.
- **`StarlarkAction`** — `Runtime.Execute(ctx, KindAutomation, body, extra)` where
  ```go
  extra := starlark.StringDict{
      "event":          starlarkEventValue(run.TriggerEvent),
      "correlation_id": starlark.String(run.CorrelationID),
  }
  ```
  On return: accumulate `result.Steps` and `result.Logs` under `run.mu`. `LimitError` surfaces as an action error.
- **`WaitAction`** — `select { case <-time.After(d): case <-ctx.Done(): return ctx.Err() }`. No Starlark thread pinned; `inflight` gauge still counts the automation.

### 10.2 Block executors

- **`SequenceBlock`** — iterate children in order:
  - `err == nil` → continue.
  - `err != nil && child.ContinueOnError` → WARN log, increment `gohome_automation_actions_total{result="skipped_continue"}`, continue.
  - `err != nil && !child.ContinueOnError` → return `err`.

  `Automation.actions` at the top level is an implicit `SequenceBlock`.

- **`ParallelBlock`** — spawn each child via `errgroup.WithContext(ctx)`. First hard-error child (continueOnError=false) cancels the group's context; sibling waits/dispatches/Starlark all receive `ctx.Err()` and unwind. Soft-error children (continueOnError=true) log WARN and do not cancel. `errgroup.Wait()` returns the first hard error.

  Concurrent children may target the same entity; the engine does not serialize commands — driver/scene-engine concern.

### 10.3 Run flow

```go
func (e *Engine) executeRun(ctx context.Context, auto *Automation, m trigger.Match) Outcome {
    run := newRun(auto, m, e.deps, uuid.NewString())
    spanCtx, span := e.tracer.Start(ctx, "automation.run",
        "automation.id", auto.ID, "correlation_id", run.CorrelationID,
        "trigger.kind", m.TriggerKind)
    defer span.End()

    e.store.Append(ctx, eventForTriggered(auto, run, m))

    if !evalConditions(spanCtx, auto.Conditions, run) {
        e.store.Append(ctx, eventForFinished(run, OutcomeConditionFail, nil))
        return OutcomeConditionFail
    }

    top := &SequenceBlock{Actions: auto.Actions}
    err := top.Execute(spanCtx, run)

    outcome := classify(err, ctx)
    e.store.Append(ctx, eventForFinished(run, outcome, err))
    return outcome
}
```

Classification for `AutomationFinished.outcome`:

| Error source | Outcome |
|---|---|
| `*starlark.LimitError` | `OUTCOME_LIMIT_EXCEEDED` |
| `ctx.Err() == context.Canceled` | `OUTCOME_CANCELLED` |
| other non-nil | `OUTCOME_ACTION_ERROR` |
| nil | `OUTCOME_OK` |

---

## 11. Script Engine

Mirrors the automation executor, driven by name-based invocation.

```go
type Engine struct {
    mu      sync.RWMutex
    scripts map[string]*Script
    runtime *starlark.Runtime
    deps    Deps
}

type Script struct {
    Name    string
    Params  []Param
    Handler string
}

type Param struct {
    Name     string
    Type     ParamType   // STRING | INT | FLOAT | BOOL | ENTITY_ID
    Required bool
    Default  any         // pre-coerced from ScriptParam.default at compile
}

func (e *Engine) Call(
    ctx          context.Context,
    name         string,
    args         map[string]string,
    invokedBy    string,
    sharedCorrID string,
) (*CallResult, error)
```

### 11.1 `Call` flow

1. Read `scripts[name]` under `RLock`, capture `*Script` locally. Unknown name → `ErrScriptNotFound`.
2. `correlationID = sharedCorrID` or `uuid.NewString()`.
3. Validate args: required present, coerce by declared type, empty → default, unknown keys rejected. Errors → `ErrScriptArgs`.
4. Build `extra`:
   ```go
   extra := starlark.StringDict{
       "params":         asStarlarkDict(coercedArgs),
       "correlation_id": starlark.String(correlationID),
   }
   ```
5. Emit `ScriptInvoked{name, correlation_id, invoked_by, args}`.
6. Tracing span `script.call`.
7. `runtime.Execute(spanCtx, KindScript, script.Handler, extra)`.
8. Emit `ScriptFinished{name, correlation_id, outcome, error?, elapsed_ms, steps, logs, return_value}`.
9. Metrics: `gohome_script_invocations_total`, `gohome_script_duration_seconds`.
10. Return `CallResult`.

### 11.2 Concurrency

Scripts have no mode gate — always parallel-per-call. Authors needing serialization rely on driver idempotency or use entities as locks. Adding a `mode` to scripts is v1.x.

### 11.3 Public surface

```go
func Compile(snapshot *configpb.ConfigSnapshot, runtime *starlark.Runtime, deps Deps) (*Engine, error)
func (e *Engine) Call(ctx, name, args, invokedBy, sharedCorrID) (*CallResult, error)
func (e *Engine) Reload(snapshot *configpb.ConfigSnapshot) error
func (e *Engine) List() []ScriptSummary
func (e *Engine) Get(name string) (*ScriptSummary, error)
func (e *Engine) Stop(ctx context.Context) error       // waits for in-flight calls to drain
```

---

## 12. Concurrency: The Mode State Machine

Per-automation admission. State lives in `runState` keyed by `automation_id`.

```go
type runState struct {
    auto *Automation

    // mode=single
    running atomic.Bool

    // mode=restart
    restartMu    sync.Mutex
    activeCancel context.CancelFunc

    // mode=queued
    queueOnce sync.Once
    queue     chan pending    // buffered, size auto.MaxQueued
}

type pending struct {
    triggerEvent *eventstore.Event
    triggerKind  string
    enqueuedAt   time.Time
}
```

### 12.1 Admission (`engine.fire`)

```go
func (e *Engine) fire(ctx context.Context, m trigger.Match) {
    rs := e.runStateFor(m.AutomationID)
    auto := rs.auto
    if !auto.Enabled { return }

    switch auto.Mode {
    case ModeSingle:
        if !rs.running.CompareAndSwap(false, true) {
            e.emitSkipped(auto, m, "already running")
            return
        }
        defer rs.running.Store(false)
        e.inFlight.Add(1); defer e.inFlight.Done()
        e.executeRun(ctx, auto, m)

    case ModeRestart:
        rs.restartMu.Lock()
        if rs.activeCancel != nil { rs.activeCancel() }
        subCtx, cancel := context.WithCancel(ctx)
        rs.activeCancel = cancel
        rs.restartMu.Unlock()

        e.inFlight.Add(1)
        go func() {
            defer e.inFlight.Done()
            e.executeRun(subCtx, auto, m)
            rs.restartMu.Lock()
            if rs.activeCancel == cancel { rs.activeCancel = nil }
            rs.restartMu.Unlock()
        }()

    case ModeQueued:
        rs.queueOnce.Do(func() {
            rs.queue = make(chan pending, auto.MaxQueued)
            e.inFlight.Add(1)
            go e.queuePump(ctx, rs)
        })
        select {
        case rs.queue <- pending{m.Event, m.TriggerKind, time.Now()}:
        default:
            e.emitSkipped(auto, m, "queue full (max=%d)", auto.MaxQueued)
        }

    case ModeParallel:
        e.inFlight.Add(1)
        go func() {
            defer e.inFlight.Done()
            e.executeRun(ctx, auto, m)
        }()
    }
}
```

### 12.2 Queue pump

One goroutine per queued automation, lazily started:

```go
func (e *Engine) queuePump(ctx context.Context, rs *runState) {
    defer e.inFlight.Done()
    for {
        select {
        case pend := <-rs.queue:
            e.executeRun(ctx, rs.auto, pend.toMatch())
        case <-ctx.Done():
            for {
                select {
                case pend := <-rs.queue:
                    e.emitSkipped(rs.auto, pend.toMatch(), "shutdown")
                default:
                    return
                }
            }
        }
    }
}
```

### 12.3 Skipped/cancelled fires are recorded

`emitSkipped` writes both `AutomationTriggered` and `AutomationFinished{OUTCOME_SKIPPED}` with a shared correlation_id. This keeps the event log self-consistent and makes "why didn't my automation run?" answerable via `gohome automation trace`. Retention policy (C1 §5.7) trims old events at the same rate as everything else; high-noise environments use `mode: queued` to serialize instead of spamming.

### 12.4 Cancellation

For `mode=restart`, `ctx` cancellation propagates: Starlark cancels via `thread.Cancel` (C5 watchdog, co-operative at step boundary), `CallServiceAction` sees cancelled ctx, `WaitAction` wakes immediately. The cancelled run classifies to `OUTCOME_CANCELLED`.

### 12.5 Shutdown

`Engine.Stop(ctx)` cancels the engine context and waits `inFlight.Wait()` with a grace (default 30s = longest `KindAutomation` wall-clock). Runs still in-flight after grace are hard-aborted. Queue pumps drain queues, emitting SKIPPED for undelivered fires.

### 12.6 Mode change via reload

Reload replaces `rs.auto` under `mu` while preserving in-flight runs under their old mode. New fires use the new mode; old runs finish naturally.

---

## 13. Reload on `ConfigApplied`

The daemon subscribes to `ConfigApplied` and calls `script.Engine.Reload(newSnapshot)` then `automation.Engine.Reload(newSnapshot)` — scripts first so automations never reference stale script definitions during the swap window.

### 13.1 Script reload (simple atomic swap)

```go
func (e *script.Engine) Reload(snapshot *configpb.ConfigSnapshot) error {
    newScripts, err := compileScripts(snapshot, e.runtime, e.deps)
    if err != nil { return err }
    e.mu.Lock()
    e.scripts = newScripts
    e.mu.Unlock()
    return nil
}
```

In-flight calls captured their `*Script` under RLock at entry; they continue against the old definition. New calls use the new map.

### 13.2 Automation reload (surgical)

```go
func (e *Engine) Reload(snapshot *configpb.ConfigSnapshot) error {
    newAutos, err := compile(snapshot, e.scripts, e.runtime, e.deps)
    if err != nil {
        e.metrics.Reload().Fail()
        return err
    }

    e.mu.Lock()
    defer e.mu.Unlock()

    for id, oldA := range e.automations {
        newA, ok := newAutos[id]
        switch {
        case !ok:
            e.unregisterTriggers(oldA)
            e.cancelInFlight(id, "automation removed")
            delete(e.runStates, id)
        case !structurallyEqual(oldA, newA):
            e.unregisterTriggers(oldA)
            e.registerTriggers(newA)
            e.runStates[id].auto = newA
            if oldA.Mode != newA.Mode {
                e.resetModeState(e.runStates[id])
            }
        }
    }
    for id, newA := range newAutos {
        if _, ok := e.automations[id]; !ok {
            e.runStates[id] = &runState{auto: newA}
            e.registerTriggers(newA)
        }
    }

    e.automations = newAutos
    return nil
}
```

`structurallyEqual` compares proto representations field-by-field. Cheap; avoids needless re-registration.

### 13.3 TimeScheduler reload

```go
func (s *TimeScheduler) Reset(entries []scheduledEntry) {
    s.mu.Lock()
    s.entries = entries
    heap.Init(&s.entries)
    s.mu.Unlock()
    s.wakeCh <- struct{}{}
}
```

### 13.4 Failure mode

Compile error → reload rejected atomically; the live engine keeps the previous snapshot, logs WARN, increments `gohome_automation_reload_failures_total`. The `gohome config apply` command (C4) already surfaces the compile errors to the user.

### 13.5 Shutdown order

`automation.Engine.Stop(ctx)` then `script.Engine.Stop(ctx)`. Order matters because in-flight automations may still be holding `ScriptAction` calls; the script engine must outlive the automation engine during drain.

---

## 14. CLI and History Surface

All commands dial the daemon's Unix socket (JSON-per-line protocol, reused from C5 §8). No direct DB access.

### 14.1 New socket operations

| op | description |
|---|---|
| `automation_list`, `automation_get`, `automation_enable`, `automation_disable`, `automation_trigger` | direct engine calls |
| `automation_watch` | streaming subscription on `AutomationTriggered` / `AutomationFinished` |
| `automation_trace` | event-log scan, returns assembled timeline |
| `script_list`, `script_run` | direct engine calls |

No RPC in C6 — these stay on the Unix socket handler that already serves `starlark_eval`. C7 replaces this with typed Connect-RPC.

### 14.2 Commands

**`gohome automation list`** — tabular summary. `LAST RUN` resolved by scanning the event log backward for the most recent `AutomationFinished{automation_id=<id>}`.

**`gohome automation get <id>`** — full detail: trigger/condition/action tree, runtime state (enabled, in-flight count), last 10 runs.

**`gohome automation enable|disable <id>`** — runtime toggle. **In-memory only in v1**; reverts on daemon restart. Help text states this explicitly. Durable overrides via replayed `SystemEvent` are v1.x. Authors wanting durable disable edit `enabled: false` in Pkl.

**`gohome automation trigger <id>`** — manually fires an automation. Engine synthesizes a `Match` with `trigger_kind: "manual"`, no underlying event, `invoked_by: "cli:<user>"`. Conditions see `event = None`.

**`gohome automation watch [--id <id>] [--scripts]`** — streams automation (and optionally script) lifecycle events in real time, reusing existing subscription plumbing.

**`gohome automation trace <correlation-id>`** — reconstructs one run's timeline by querying the event log for all events with the matching correlation_id, including indirect references (`CommandIssued.source` contains the corr_id). Displays as a timestamped tree: Triggered → Conditions → Actions (with nested CommandIssued/CommandAck, ScriptInvoked/ScriptFinished) → Finished.

**`gohome script list`** — registered scripts with param signatures.

**`gohome script run <name> [--arg key=value ...]`** — streams `log()` lines live, prints return value and elapsed on exit. Exit 0 on `OUTCOME_OK`.

### 14.3 Styling (lipgloss)

Reuses existing styles from `internal/cli` (`EntityID`, `Dim`, `Error`, `Success`, `Warning`, `Header`, `Accent`). Three new named styles added under the same package:

| Style | Purpose | Rendering |
|---|---|---|
| `RunMarker` | lifecycle glyphs (▶ ✓ ✗ ⟲ ⊘) | color per outcome: Success green for ✓, Error red for ✗, Warning yellow for ⟲/⊘, Accent blue for ▶ |
| `Correlation` | `corr=a3f2…` | `Dim` + `Italic`; truncated to first 4 hex chars |
| `Duration` | `12ms`, `340ms` | `Dim` when <100ms, `Warning` 100ms–1s, `Error` ≥1s — visual latency budget |

Per-command spec:

- **`list`** — `Header` column headers; `EntityID` for ID; plain MODE; `Success` green `"yes"` / `Dim` `"no"` for ENABLED; `IN-FLIGHT > 0` in `Warning`; LAST RUN timestamp `Dim` + `RunMarker` glyph + `Duration`-styled elapsed. Empty: `Dim` "no automations registered".
- **`get`** — `Header` section titles; kinds in `Accent`; entity refs in `EntityID`; Starlark bodies in bordered box with `Dim`; runs table as in `list`.
- **`watch`** — `Dim` timestamp, `RunMarker` glyph, `EntityID` automation id, `Dim` parenthesized context, `Duration` on finish line, right-padded `Correlation`. Error lines expand indented `Error` text.
- **`trace`** — tree via `lipgloss.Tree` (or equivalent box-drawing); node labels `EntityID`; phase labels `Accent`; outcomes via `RunMarker`; command events as `Dim` sub-branches; failure branches expand with `Error` context; summary bar `Success` or `Error` background.
- **`trigger`** — immediate echo `"▶ triggered <id> (manual) corr=<short>"` in `Accent`; inline tail of resulting run until `AutomationFinished`. Exit code matches outcome.
- **`enable|disable`** — one-line confirmation. `Success` for enable, `Warning` for disable; `Dim` note "(in-memory; reverts on daemon restart — edit Pkl for durable change)".
- **`script list`** — `Header` row; `EntityID` script name; params as `name:type` pairs, `Accent` name, `Dim` type; required without brackets, optional `[bracketed]`, defaults `=<value>` in `Dim`.
- **`script run`** — identical streaming format to C5's `gohome eval`: `[log]` lines in `Dim`, final value in `EntityID`, elapsed + steps in `Dim`, errors in `Error` to stderr.

All color styles use `lipgloss.AdaptiveColor`; `NO_COLOR` honored via `lipgloss.SetDefaultRenderer` (already wired in `internal/cli`).

---

## 15. File Map

### 15.1 New files

```
internal/automation/
├── engine.go, automation.go, compile.go, mode.go, reload.go, run.go, errors.go
├── trigger/{trigger,state,event,time,fake}.go
├── condition/{condition,typed,starlark,compose}.go
├── action/{action,callservice,scene,script,starlark,wait,block}.go
└── testutil/{fake_scenes,synth}.go + fixtures/

internal/script/
├── engine.go, script.go, compile.go, reload.go, errors.go
└── testutil/synth.go

internal/cli/
├── cmd_automation.go, cmd_script.go
└── styles_automation.go

internal/config/pkl/gohome/
└── scripts.pkl
```

### 15.2 Modified files

```
internal/config/pkl/gohome/automations.pkl   # full rewrite (replaces C4 stub)
proto/gohome/event/v1/event.proto            # +4 payload variants, reserve 50-59
proto/gohome/config/v1/snapshot.proto        # +AutomationConfig, ScriptConfig, typed Trigger/Condition/Action
gen/                                         # regenerated via task proto; staged + committed
internal/config/evaluator.go                 # populate AutomationConfig + ScriptConfig in ConfigSnapshot
internal/config/compile.go                   # cross-ref check: ScriptAction.name resolves
internal/config/manager.go                   # on ConfigApplied: script.Reload then automation.Reload
internal/daemon/daemon.go                    # construct engines, wire subscription, register socket ops
internal/daemon/socket.go                    # add automation_* and script_* ops
cmd/gohomed/main.go                          # dep injection (SceneApplier stub, runtime, eventstore subscriber)
go.mod, go.sum                               # +robfig/cron/v3, +golang.org/x/sync/errgroup (if absent), +google/uuid (likely present)
```

### 15.3 Unchanged

`internal/eventstore`, `internal/state`, `internal/registry`, `internal/storage`, `internal/observability` — reused as-is. No new tables, no new migrations.

### 15.4 Size estimate

~4500 lines new Go across `internal/automation` + `internal/script` + CLI; ~400 lines Pkl + proto. Tests roughly double the Go footprint.

---

## 16. Testing Strategy

Convention: unit under plain `go test`, integration under the `integration` build tag (`task test:integration`), all with `-race` in CI. Coverage gate **75%** across `internal/automation` + `internal/script`.

### 16.1 Unit tests

| Package | Coverage focus | Shape |
|---|---|---|
| `automation/trigger` | `StateChangeMatcher` × {from,to,forDur} combinations (incl. hold-cancel); `EventMatcher` data predicates; `TimeScheduler` sort/reset | Table `(matcher, inbound[], want_matches_at[])` |
| `automation/condition` | Every typed variant × pass/fail/error; composition identities; Starlark short-circuit verified via call counter | Table + property-style |
| `automation/action` | Each leaf with fakes; `SequenceBlock` abort-vs-continue; `ParallelBlock` cancellation propagation | Fake-based, assert recorded calls |
| `automation/mode` | Sequence harness: synthetic fires per mode → assert order of emissions + outcomes; queued overflow | Table `(mode, fires[], want_events[])` |
| `automation` top | `Compile` error aggregation: every invalid Pkl shape from §4 produces expected `CompileError` path; reload diff classifier | Golden-file fixtures |
| `script` | Param coercion table; `Call` event emissions; reload atomic swap does not break in-flight | Table + small race |

### 16.2 Fakes

`internal/automation/testutil`:

```go
type FakeDispatcher struct { Calls []DispatchCall; Err error }
type FakeSceneApplier struct { Applied []string; Err error }
type FakeState map[string]*starlark.EntityState    // reuses C5 EntityState
type FakeStore struct { Appended []eventstore.Event }
```

`internal/starlark/testutil` (from C5) supplies a pre-wired `*starlark.Runtime` — reused directly for any test involving Starlark.

### 16.3 Integration tests (`//go:build integration`)

Real `eventstore` (in-memory SQLite) + real Pkl evaluator + real C5 runtime.

1. **Golden-path automation.** Motion `off→on` fires light turn-on; assert full event chain (`AutomationTriggered` → `CommandIssued` → `CommandAck` → `AutomationFinished{OK}`) under one correlation_id.
2. **Hold-duration trigger.** `forDur: 5s` fires only on sustained state; spurious intervening changes suppress.
3. **Mode=restart.** Rapid re-fire cancels prior run (emits CANCELLED), new run completes OK.
4. **Mode=queued overflow.** 5 rapid fires with `maxQueued: 2`; 2 complete OK, 3 emit SKIPPED.
5. **Condition short-circuit.** Cheap state condition fails → Starlark runtime not called (verified via counter).
6. **Scene stub.** `SceneAction` → `SceneApplied` appended; no commands dispatched.
7. **Script invocation from automation.** Same correlation_id threads through to `ScriptInvoked`/`ScriptFinished`.
8. **Parallel block cancellation.** One failing action cancels siblings within 50ms; finishes `OUTCOME_ACTION_ERROR`.
9. **Reload preserves in-flight.** Add B while A runs; A uninterrupted, B registered.
10. **Reload cancels removed.** Remove A while running; A emits CANCELLED.
11. **CLI `gohome automation trigger`.** Dial socket, trigger, assert output and exit code.
12. **CLI `gohome automation trace`.** Capture corr, assert tree renders all expected nodes.

### 16.4 Race tests (`task test:race`)

- `mode=single` + 100 concurrent fires: one admitted, no corruption, no deadlocks.
- `mode=parallel` + 100 concurrent fires: all complete, no shared-state races.
- Reload racing active fire: either pre- or post-swap, never a half-compiled engine.

### 16.5 Out of scope

- C7 (RPC), C8 (MCP), C10 (UI) integration — tested when those specs land.
- C5 Starlark internals — covered in C5.
- C4 Pkl evaluator — covered in C4.
- Webhook matcher — inert in v1; single compile test confirms it validates but produces no live matcher.

---

## 17. Decision Record

| # | Decision | Rationale |
|---|---|---|
| D1 | Richly structured Pkl schema (typed triggers, conditions, actions). | Static diff + UI rendering without running Starlark; validation surfaces at `config validate`; matches master design "declared in Pkl by shape". |
| D2 | `mode=single` default with per-automation override. | Loud, visible failure mode for re-entrancy; matches HA mental model; per-automation run-state is needed for UI anyway. |
| D3 | Scripts live in C6 alongside automations. | 90% shared infrastructure; splitting doubles compiler/registry cost later. |
| D4 | Triggers v1: `state_changed`, `event`, `time`. Webhook Pkl-validated but inert (C7). Sun → future driver. | Webhook needs HTTP surface owned by C7; sun-as-driver keeps trigger taxonomy small and reuses state-change machinery. |
| D5 | First-class `Condition` with typed variants + And/Or/Not. | UI condition chips; typed conditions short-circuit cheap before Starlark; HA-migrator expectations. |
| D6 | HA-style nestable `SequenceBlock` / `ParallelBlock` actions. | Full expressiveness for sequences and parallel fan-out; top-level is implicit sequence. |
| D7 | Abort-on-error default with per-action `continueOnError`. | Imperative mental model; best-effort flows get explicit opt-in. |
| D8 | Events-only history (`AutomationTriggered`/`AutomationFinished`, `ScriptInvoked`/`ScriptFinished`) + `watch` via existing subscription plumbing. | Event log authoritative and position-indexed; v1-scale backward scans are cheap. Dedicated projector is v1.x if latency hurts. |
| D9 | Peer packages `internal/automation` + `internal/script`. | Scripts independently callable (CLI, future RPC/MCP/UI); dependency flows automation → script; matches `internal/carport` / `internal/state` / `internal/registry` convention. |
| D10 | Top-level runtime named `automation.Engine`. | Parallel to `script.Engine`; no grep-collision with `starlark.Runtime`. |
| D11 | Correlation ID shared from automation into nested script calls. | `gohome automation trace <corr>` shows full causal chain under one ID. |
| D12 | Skipped/cancelled fires still emit `AutomationTriggered`+`AutomationFinished`. | Self-consistent event log; answerable "why didn't my automation run?". |
| D13 | Enable/disable via CLI is in-memory only in v1. | Keeps Pkl authoritative; avoids persisted-override machinery this spec doesn't need. Durable override is v1.x. |
| D14 | `SceneApplier` is a dep interface; concrete scene engine deferred. | Scene resolution belongs to its own spec; C6 must not block on it. |
| D15 | Per-trigger timezone override deferred to v1.x. | System-local TZ covers single-home use cases; one more matrix dimension in scheduler tests. |
| D16 | 75% coverage gate. | Logic-dense, testable; step up from C4's 70% justified by misrouting-bug risk. |

---

*End of C6 design document.*
