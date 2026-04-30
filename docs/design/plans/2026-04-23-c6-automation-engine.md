# C6 — Automation Engine Implementation Plan (Retroactive)

> **Note on history:** This plan was written **retroactively** after C6 had already been implemented straight from the design spec. C6 is the only milestone in the C1–C9 set whose paired plan was skipped during execution; this document exists so future contributors and audits have a task-by-task reference matching the actual implementation. Status markers (✅ DONE / ⚠️ PARTIAL / ❌ MISSING) reflect a static audit performed on 2026-04-29 against the `gohome` tree.

> **For agentic workers (forward-looking gap closure):** The remaining ⚠️ / ❌ items map 1:1 to entries in the C1–C8 cleanup task list; use `superpowers:executing-plans` to close them.

**Goal:** Add an automation engine and a script engine to `gohomed` that compile typed Pkl-defined automations and named scripts into runnable form, react to state-changes / events / time / webhooks, evaluate typed and Starlark conditions, dispatch HA-style sequence/parallel action blocks, and emit a complete `AutomationTriggered` / `AutomationFinished` / `ScriptInvoked` / `ScriptFinished` event chain under shared correlation IDs.

**Architecture:** Two peer packages — `internal/automation` (engine + run state) and `internal/script` (call-from-anywhere named scripts). Both compile from the C4 `ConfigSnapshot` and reload atomically on `ConfigApplied`. The engine subscribes to the eventstore, dispatches matchers via a per-kind registry (`trigger.Registry`, `trigger.TimeScheduler`), and runs each fire under a mode state machine (`single` / `parallel` / `restart` / `queued`). All persistence flows through `eventstore.Store.Append`; no new tables, no new migrations.

**Tech Stack:** `github.com/robfig/cron/v3` (cron parser used by `TimeScheduler`), `github.com/google/uuid` (correlation IDs), C5 `internal/starlark.Runtime` (typed conditions w/ Starlark fallback + `ScriptAction` body + `StarlarkAction`), C4 `internal/config` (Pkl evaluation + `ConfigApplied` event), C1 `internal/eventstore` (subscription + append), C1 `internal/state` (read-only state cache for matchers + conditions).

**Spec:** [`2026-04-23-c6-automation-engine-design.md`](../specs/2026-04-23-c6-automation-engine-design.md). Section references in this plan are to that document.

---

## Codebase orientation

Before starting any task, read:

| File | Why |
|---|---|
| `internal/eventstore/store.go`, `subscribe.go` | `Append`, `Subscribe`, `LatestPosition` — the I/O surface this milestone consumes |
| `internal/state/cache.go` | `state.Cache.Get(id)` — read-only by C6 |
| `internal/starlark/runtime.go` | C5 `Runtime.Execute` — the boundary for every Starlark call this milestone makes |
| `internal/config/manager.go` | Where `ConfigApplied` is emitted and the engines need to register reload hooks |
| `internal/config/pkl/gohome/automations.pkl` | Pre-C6 stub from C4; this milestone replaces it wholesale |
| `proto/gohome/event/v1/event.proto` | `Payload` oneof — fields 50–53 are reserved here |
| `proto/gohome/config/v1/snapshot.proto` | Where `AutomationConfig` / `ScriptConfig` and the typed Trigger/Condition/Action variants live |

---

## File map

### New files

| Path | Responsibility |
|---|---|
| `internal/automation/automation.go` | `Automation` struct (compiled shape: triggers, conditions, actions, mode, MaxQueued) |
| `internal/automation/engine.go` | `Engine` runtime: ctx lifecycle, trigger registration, `runLoop`, `Trigger`/`List`/`Get`/`SetEnabled`, mode-dispatched `fire` |
| `internal/automation/compile.go` | `CompileAutomations(snap, scriptEngine, runtime) → map[string]*Automation` |
| `internal/automation/mode.go` | `Mode` enum (`single`/`parallel`/`restart`/`queued`) + `ParseMode` |
| `internal/automation/reload.go` | Surgical reload diff: which IDs added / removed / changed, swap atomically without dropping in-flight |
| `internal/automation/run.go` | `executeRun` — emits `AutomationTriggered`, evaluates conditions, runs top-level block, classifies outcome, emits `AutomationFinished` |
| `internal/automation/errors.go` | `CompileError` aggregator + classification helpers |
| `internal/automation/deps.go` | `Deps` struct: `Store`, `State`, `Dispatcher`, `Scenes`, `Logger`, `Metrics` |
| `internal/automation/metric_executor.go` | Wraps leaf actions to record `gohome_automation_actions_total{automation_id, action_kind, result}` |
| `internal/automation/trigger/trigger.go` | `Match` envelope + per-kind `Registry` (`RegisterState`/`RegisterEvent`/`Unregister`/`Dispatch`) |
| `internal/automation/trigger/state.go` | `StateChangeMatcher` (from/to/forDur, hold cancellation) |
| `internal/automation/trigger/event.go` | `EventMatcher` + `WebhookMatcher` (data predicates, kind filter) |
| `internal/automation/trigger/time.go` | `TimeScheduler` — `at`, `cron`, `every` (built on robfig/cron/v3) |
| `internal/automation/condition/condition.go` | `Evaluator` interface + `Env` (state + runtime + event) |
| `internal/automation/condition/typed.go` | Typed variants: state, attribute, numeric, time-of-day, day-of-week |
| `internal/automation/condition/starlark.go` | Starlark expression condition (single line via C5 `KindTriggerCondition`) |
| `internal/automation/condition/compose.go` | `And`, `Or`, `Not` — short-circuit |
| `internal/automation/action/action.go` | `Executor` interface, `Run` context (correlation ID, snapshot, dispatcher, store, scenes, scripts, runtime, metrics) |
| `internal/automation/action/callservice.go` | `CallServiceAction` — `Dispatcher.Dispatch(entityID, capability, params)` |
| `internal/automation/action/scene.go` | `SceneAction` — emits `SceneApplied` via `Scenes` interface |
| `internal/automation/action/script.go` | `ScriptAction` — calls into `script.Engine.Call` under shared correlation ID |
| `internal/automation/action/starlark.go` | `StarlarkAction` — inline Starlark body via C5 runtime |
| `internal/automation/action/wait.go` | `WaitAction` — sleep with cancellation |
| `internal/automation/action/block.go` | `SequenceBlock`, `ParallelBlock`, `ChildCtrl` (continueOnError) |
| `internal/automation/testutil/synth.go` | `FakeState`, fake dispatcher, fake event-store helpers; reused by both unit and integration tests |
| `internal/automation/testutil/fake_scenes.go` | `FakeSceneApplier` — records applied scene IDs |
| `internal/automation/testutil/fixtures/` | Golden-file fixtures for compile error aggregation |
| `internal/automation/integration_test.go` | Spec §16.3 integration scenarios behind `//go:build integration` |
| `internal/automation/race_test.go` | Spec §16.4 race tests (`task test:race`) |
| `internal/automation/reload_test.go` | Reload diff coverage |
| `internal/automation/mode_test.go` | Mode state machine truth table |
| `internal/automation/compile_test.go` | Compile error path golden tests |
| `internal/automation/metrics_test.go` | Metric label assertions |
| `internal/automation/{action,condition,trigger}/*_test.go` | Per-leaf coverage |
| `internal/script/script.go` | `Script` struct (params, body) |
| `internal/script/engine.go` | `Engine` — `Call`, emit `ScriptInvoked` / `ScriptFinished`, list/get |
| `internal/script/compile.go` | `CompileScripts(snap) → map[string]*Script` |
| `internal/script/reload.go` | Atomic swap on `ConfigApplied`; in-flight calls run to completion |
| `internal/script/errors.go` | `CompileError` classification |
| `internal/script/testutil/synth.go` | Test helpers (param fixtures, fake event store) |
| `internal/cli/cmd_automation.go` | `gohome automation {list,get,enable,disable,trigger,trace}` |
| `internal/cli/cmd_script.go` | `gohome script {list,run}` |
| `internal/cli/styles_automation.go` | Lipgloss styles for automation/script CLI output |
| `internal/config/pkl/gohome/scripts.pkl` | New Pkl module — `Script` class with params + body |

### Modified files

| Path | Change |
|---|---|
| `internal/config/pkl/gohome/automations.pkl` | Wholesale rewrite — replaces C4 stub with typed Trigger/Condition/Action shape |
| `proto/gohome/event/v1/event.proto` | Reserve & assign 50–59 in `Payload.kind`; add `AutomationTriggered` (50), `AutomationFinished` (51), `ScriptInvoked` (52), `ScriptFinished` (53); add top-level `RunOutcome` enum (`OK`, `CONDITION_FAIL`, `ACTION_ERROR`, `LIMIT_EXCEEDED`, `CANCELLED`, `SKIPPED`) |
| `proto/gohome/config/v1/snapshot.proto` | Add `AutomationConfig`, `ScriptConfig`, typed `Trigger`/`Condition`/`Action` messages |
| `gen/` | Regenerated via `task proto`; staged + committed |
| `internal/config/evaluator.go` | Decode `AutomationConfig` and `ScriptConfig` into `ConfigSnapshot` |
| `internal/config/compile.go` | Cross-ref check: `ScriptAction.name` resolves against compiled scripts |
| `internal/config/manager.go` | On `ConfigApplied`: invoke `script.Reload` then `automation.Reload` (script-first so automations reference fresh script set) |
| `internal/daemon/daemon.go` | Construct `script.Engine` and `automation.Engine` in phase 5; wire into `ConfigApplied` reload chain; provide `SceneApplier` stub; engines started via `engine.Start(ctx)` |
| `cmd/gohomed/main.go` | Dependency injection for the runtime, eventstore subscriber, and scene stub |
| `go.mod`, `go.sum` | Add `github.com/robfig/cron/v3` |
| `internal/observability/metrics.go` | Register `gohome_automation_*` and `gohome_script_*` metrics (registered total, runs total by outcome, run duration histogram, in-flight gauge, conditions total, actions total, starlark steps histogram, triggers total) |

---

## Tasks

Each task lists files, a short description, and acceptance criteria. The status marker on each task header reflects the 2026-04-29 audit against `gohome` `main`.

---

### Task 1 — Add `robfig/cron/v3` dependency  ✅ DONE

**Files:** `go.mod`, `go.sum`

- [x] **Step 1** — `go get github.com/robfig/cron/v3@latest`
- [x] **Step 2** — `go mod tidy`
- [x] **Step 3** — `task build`

**Acceptance:** `go.mod` declares `github.com/robfig/cron/v3`; both binaries still build.

**Status:** Present in `go.mod` (`v3.0.1`).

---

### Task 2 — Proto: event payload variants 50–53 + `RunOutcome`  ✅ DONE

**Files:** `proto/gohome/event/v1/event.proto`, `gen/`

- [x] **Step 1** — Reserve fields 50–59 inside `Payload.kind`.
- [x] **Step 2** — Add messages `AutomationTriggered` (50), `AutomationFinished` (51), `ScriptInvoked` (52), `ScriptFinished` (53).
- [x] **Step 3** — Add top-level enum `RunOutcome` with values `OUTCOME_UNSPECIFIED=0`, `OUTCOME_OK=1`, `OUTCOME_CONDITION_FAIL=2`, `OUTCOME_ACTION_ERROR=3`, `OUTCOME_LIMIT_EXCEEDED=4`, `OUTCOME_CANCELLED=5`, `OUTCOME_SKIPPED=6`.
- [x] **Step 4** — `task proto`; commit `gen/` updates.

**Acceptance:** `event.proto` references `AutomationTriggered`/`AutomationFinished`/`ScriptInvoked`/`ScriptFinished` at fields 50/51/52/53; `RunOutcome` is a top-level enum referenced by `AutomationFinished` and `ScriptFinished`; `gen/` reflects the changes.

**Status:** Confirmed at `proto/gohome/event/v1/event.proto:28-31` (oneof entries) and 93–158 (messages + enum).

---

### Task 3 — Proto: config snapshot additions  ✅ DONE

**Files:** `proto/gohome/config/v1/snapshot.proto`, `gen/`

- [x] **Step 1** — Add `AutomationConfig` and `ScriptConfig` messages; aggregate them on `ConfigSnapshot`.
- [x] **Step 2** — Add typed `Trigger` (oneof: state-change, event, time, webhook), `Condition` (oneof: typed variants + Starlark + And/Or/Not), `Action` (oneof: call_service, scene, script, starlark, wait, sequence_block, parallel_block).
- [x] **Step 3** — `task proto`.

**Acceptance:** `ConfigSnapshot` carries `repeated AutomationConfig` and `repeated ScriptConfig`; trigger/condition/action shapes match spec §4.

**Status:** Confirmed at `proto/gohome/config/v1/snapshot.proto:15` (`automations`), `:20` (`scripts`), and the typed-variant messages 41+.

---

### Task 4 — Pkl module: `gohome.scripts`  ✅ DONE

**Files:** `internal/config/pkl/gohome/scripts.pkl`

- [x] **Step 1** — Author `Script` class with `name`, `params: Map<String, ParamSpec>`, `body: String`.
- [x] **Step 2** — Wire `scripts: Listing<Script>` into the top-level `gohome.config` module.
- [x] **Step 3** — Add validators that reject empty body and disallow shadowing built-ins.

**Acceptance:** A minimal `pkl/main.pkl` declaring one script evaluates without error and shows up in `ConfigSnapshot.scripts` after `task test:integration`.

**Status:** File present; integrated via top-level config module.

---

### Task 5 — Pkl module: `gohome.automations` rewrite  ✅ DONE

**Files:** `internal/config/pkl/gohome/automations.pkl` (was a C4 stub)

- [x] **Step 1** — Define typed `Trigger`, `Condition`, `Action` classes mirroring spec §4.
- [x] **Step 2** — Define `Automation` class with `id`, `displayName`, `mode`, `maxQueued`, `triggers`, `conditions`, `actions`, `enabled`.
- [x] **Step 3** — Validators: at least one trigger; mode ∈ {`single`, `parallel`, `restart`, `queued`}; `maxQueued > 0` only when mode is `queued`.

**Acceptance:** Sample automations in `internal/config/pkl/testdata/` evaluate cleanly; invalid shapes are rejected with helpful path-anchored errors.

**Status:** File present, replaces the C4 stub. Integration tests under `internal/config/` exercise valid + invalid Pkl shapes.

---

### Task 6 — Compile error aggregator  ✅ DONE

**Files:** `internal/automation/errors.go`, `internal/automation/compile_test.go`

- [x] **Step 1** — Define `CompileError` with per-automation, per-trigger/condition/action path; aggregate into a single error.
- [x] **Step 2** — `compile_test.go`: golden fixtures for each invalid shape from §4 produce the expected error path.

**Acceptance:** Every documented invalid Pkl shape produces a `CompileError` whose path string matches the spec table; `task test ./internal/automation/` exercises all golden fixtures.

**Status:** `errors.go` + `compile_test.go` present with golden-style fixtures inline.

---

### Task 7 — Trigger registry + types  ✅ DONE

**Files:** `internal/automation/trigger/trigger.go`, `state.go`, `event.go`, `time.go`, plus `_test.go` peers

- [x] **Step 1** — `Match` envelope: automation ID, trigger kind, optional `*eventstore.Event`.
- [x] **Step 2** — `Registry`: `RegisterState`, `RegisterEvent`, `Unregister`, `Dispatch(event) []Match`.
- [x] **Step 3** — `StateChangeMatcher` with `from` / `to` / `forDur` + hold-cancel semantics; `SetDeliverHold` callback used by the engine.
- [x] **Step 4** — `EventMatcher` and `WebhookMatcher` with kind + data predicates.
- [x] **Step 5** — `TimeScheduler` via robfig/cron/v3; `AddAt`, `AddCron`, `AddEvery`; `Run(ctx)` + `Ready()` channel.

**Acceptance:** Unit tests cover combinations (`from`+`to`, `forDur`, hold cancel by intervening change), event predicate matching, time-scheduler ordering and reschedule on reload.

**Status:** Files + tests present. `WebhookMatcher` lives in `event.go`. Webhook is wired into the trigger registry but Pkl-side validation is what the audit flagged as "no targeted compile test" (see Task 27).

---

### Task 8 — Conditions  ✅ DONE

**Files:** `internal/automation/condition/{condition,typed,starlark,compose}.go` + `_test.go` peers

- [x] **Step 1** — `Evaluator` interface + `Env{State, Runtime, Event, Now, Loc, Logger}`.
- [x] **Step 2** — Typed variants with cheap evaluation (state equality, attribute compare, numeric range, time-of-day window, day-of-week set).
- [x] **Step 3** — Starlark condition via C5 runtime `KindTriggerCondition` (read-only event globals; no `call_service`/`sleep`).
- [x] **Step 4** — `And` / `Or` / `Not` — short-circuit; pass through env unchanged.

**Acceptance:** Composition identity tests (`And(true, x) == x`, `Or(false, x) == x`, etc.); short-circuit verified by counting Starlark calls; failure / error / pass paths each emit the right metric label.

**Status:** All four files plus tests; condition coverage measured at 69.2% (below 75% gate).

---

### Task 9 — Actions  ✅ DONE

**Files:** `internal/automation/action/{action,callservice,scene,script,starlark,wait,block}.go` + `_test.go` peers

- [x] **Step 1** — `Executor` interface + `Run` struct (correlation ID, dispatcher, store, scenes, scripts, runtime, logger, metrics).
- [x] **Step 2** — `CallServiceAction` — calls `Dispatcher.Dispatch`; emits no extra event (the dispatch path already emits `CommandIssued`/`CommandAck` via carport).
- [x] **Step 3** — `SceneAction` — calls `Scenes.Apply`; appends `SceneApplied` event.
- [x] **Step 4** — `ScriptAction` — calls `script.Engine.Call` under shared correlation ID.
- [x] **Step 5** — `StarlarkAction` — inline body via C5 `Runtime.Execute` (`KindAutomation`).
- [x] **Step 6** — `WaitAction` — `time.Sleep` honoring `ctx.Done()`.
- [x] **Step 7** — `SequenceBlock` (abort on first error unless `continueOnError`) and `ParallelBlock` (cancel siblings on first error).

**Acceptance:** Each leaf has fake-based unit tests asserting recorded calls. Block tests cover abort-vs-continue and cross-sibling cancellation propagation within ≤50 ms.

**Status:** Every file present with tests; `metric_executor.go` wraps leaves so `gohome_automation_actions_total{automation_id, action_kind, result}` is recorded uniformly.

---

### Task 10 — Mode state machine  ✅ DONE

**Files:** `internal/automation/mode.go`, `mode_test.go`

- [x] **Step 1** — `Mode` enum (`single`/`parallel`/`restart`/`queued`) + `ParseMode`.
- [x] **Step 2** — `runState` per automation: `running atomic.Bool`, queue (chan), `restartGen int64`, `activeCancel context.CancelFunc`.
- [x] **Step 3** — Unit table tests: synthetic fires per mode → asserted emission order + outcomes.

**Acceptance:** `TestMode_Single`, `TestMode_Parallel`, `TestMode_QueuedOverflow` — all covered.

**Status:** All three present at `mode_test.go:64,82,97`.

---

### Task 11 — Engine core (`automation.Engine`)  ✅ DONE

**Files:** `internal/automation/engine.go`, `automation.go`, `deps.go`

- [x] **Step 1** — `Engine` constructor wiring `Deps`, `runStates`, `triggers`, `scheduler`, `runtime`, `scriptEngine`, `scriptCaller`.
- [x] **Step 2** — `Start(ctx)`: subscribe from `LatestPosition`, spawn `runLoop` and `scheduler.Run`.
- [x] **Step 3** — `runLoop`: dispatch matches from event subscription + scheduler; admit through `fire(ctx, m, invokedBy)`.
- [x] **Step 4** — Public surface: `Trigger(ctx, id, invokedBy)`, `List`, `Get`, `SetEnabled` (in-memory only per D13).
- [x] **Step 5** — `Stop(ctx)`: cancel engine ctx, wait `inFlight`, bounded by 30 s.

**Acceptance:** Engine starts, fires synthetic matches, finishes runs, stops cleanly within bounded time. Race detector clean (`task test:race`).

**Status:** Confirmed at `engine.go:38-192`. Race tests at `race_test.go:80,133,180`.

---

### Task 12 — `executeRun` + outcome classifier  ✅ DONE

**Files:** `internal/automation/run.go`

- [x] **Step 1** — Generate UUID, emit `AutomationTriggered` (with `TriggerEventPosition` if any).
- [x] **Step 2** — Evaluate conditions (short-circuit in compose); on failure, emit `AutomationFinished{CONDITION_FAIL}`.
- [x] **Step 3** — Run top-level block (implicit sequence); classify error via `classify(err, ctx)` into `OK` / `LIMIT_EXCEEDED` / `CANCELLED` / `ACTION_ERROR`.
- [x] **Step 4** — Emit `AutomationFinished` carrying outcome, error string, elapsed ms, Starlark steps, captured logs.
- [x] **Step 5** — Skipped fires (`mode=single` re-entry, `mode=queued` overflow) emit `AutomationTriggered` + `AutomationFinished{SKIPPED}` for self-consistency (D12).

**Acceptance:** Integration test `TestIntegration_GoldenPath` observes `AutomationTriggered` → action events → `AutomationFinished{OK}` under one correlation ID.

**Status:** Confirmed in `run.go`; skip emission lives in `engine.go:emitSkipped`.

---

### Task 13 — Surgical reload  ✅ DONE

**Files:** `internal/automation/reload.go`, `reload_test.go`

- [x] **Step 1** — Diff old vs. new automations: `added`, `removed`, `changed` (deep compare beyond ID).
- [x] **Step 2** — For `added`: register triggers; create `runState`.
- [x] **Step 3** — For `removed`: unregister, cancel any active run (`activeCancel`), close queues.
- [x] **Step 4** — For `changed`: stop matchers atomically, replace, register fresh; in-flight runs are bound to the **old** automation (snapshot capture in `runState`) so they finish.
- [x] **Step 5** — Tests: `TestReload_NoChange_KeepsMatchers`, `TestReload_AddsNew`, `TestReload_RemovesOld`, `TestReload_NoChange_MatcherPreservesLastState`.

**Acceptance:** Reload during an active fire never produces a half-compiled engine; `task test:race` passes.

**Status:** `reload.go` + 4 tests; race coverage adds `TestRace_ReloadDuringActiveFire`.

---

### Task 14 — Script engine (`internal/script`)  ✅ DONE

**Files:** `internal/script/{engine,script,compile,reload,errors}.go` + tests

- [x] **Step 1** — `Script` struct with name, params (typed), body string, compiled module.
- [x] **Step 2** — `Engine.Call(ctx, name, args, invokedBy, sharedCorrID) (*CallResult, error)` — coerces args to typed params, executes in C5 `KindScript`, captures logs + steps.
- [x] **Step 3** — Emit `ScriptInvoked` on entry and `ScriptFinished` on exit; both share the inbound correlation ID (or new UUID if standalone).
- [x] **Step 4** — `Reload` performs atomic swap of compiled-script map; in-flight calls run to completion.
- [x] **Step 5** — Tests: param coercion table, event emission, atomic-swap-during-call.

**Acceptance:** `TestIntegration_ScriptInvocationCorrelation` threads a single correlation ID through automation → script → finished events.

**Status:** All files present; `engine.go:136`/`:187` show the `ScriptInvoked`/`ScriptFinished` emissions. Coverage at 74.2% — just below the 75% gate.

---

### Task 15 — Config-snapshot decode + cross-refs  ✅ DONE

**Files:** `internal/config/evaluator.go`, `internal/config/compile.go`

- [x] **Step 1** — Populate `ConfigSnapshot.Automations` and `.Scripts` from the Pkl-evaluated tree.
- [x] **Step 2** — Cross-ref check in compile.go: every `ScriptAction.name` resolves to a defined script.
- [x] **Step 3** — Emit `ConfigApplied` only after both compilations succeed.

**Acceptance:** Validation rejects an automation referencing an undefined script with a path-anchored error; valid configs flow through cleanly.

**Status:** Both files updated; tested under `internal/config/compile_test.go`.

---

### Task 16 — Reload chain on `ConfigApplied`  ✅ DONE

**Files:** `internal/config/manager.go`, `internal/daemon/daemon.go`

- [x] **Step 1** — In `manager.go`, after each successful apply, invoke `script.Reload` then `automation.Reload` (script-first so automations see fresh script set).
- [x] **Step 2** — In `daemon.go`, register reload hooks during phase 5 construction.
- [x] **Step 3** — Failure mode: log and keep prior compiled set (no half-applied state); never crash the daemon.

**Acceptance:** Editing a config and re-applying refreshes both engines without dropping in-flight runs.

**Status:** Confirmed in daemon construction — script + automation engines are constructed and chained on reload.

---

### Task 17 — Daemon wiring  ✅ DONE

**Files:** `internal/daemon/daemon.go`, `cmd/gohomed/main.go`

- [x] **Step 1** — Phase 5: construct C5 `Runtime` (already done by C5), then `script.NewEngine(scriptMap, runtime, deps)`, then `automation.NewEngine(autos, scriptEngine, runtime, deps)`.
- [x] **Step 2** — Provide a `SceneApplier` stub (D14): records scene IDs, emits `SceneApplied`. Concrete scene engine deferred to its own spec.
- [x] **Step 3** — Start engines on enter-readiness; stop on shutdown.

**Acceptance:** `gohomed` starts cleanly with empty config and with a fully-populated config.

**Status:** Daemon construction + start/stop confirmed; SceneApplier stub in daemon adapters.

---

### Task 18 — Observability: metrics + logs  ✅ DONE

**Files:** `internal/observability/metrics.go`, every emit-site

- [x] **Step 1** — Register `gohome_automation_registered`, `gohome_automation_inflight{automation_id}`, `gohome_automation_runs_total{automation_id, outcome}`, `gohome_automation_run_duration_seconds{automation_id}`, `gohome_automation_conditions_total{automation_id, result}`, `gohome_automation_actions_total{automation_id, action_kind, result}`, `gohome_automation_starlark_steps{automation_id}`, `gohome_automation_triggers_total{automation_id, trigger_kind}`.
- [x] **Step 2** — Mirror script metrics: `gohome_script_runs_total{name, outcome}`, `gohome_script_duration_seconds{name}`, etc.
- [x] **Step 3** — Slog at info on automation/script start + finish (with correlation ID) and warn on skipped/cancelled.

**Acceptance:** `metrics_test.go` asserts label sets match the spec table exactly.

**Status:** `metrics_test.go` present in both packages.

---

### Task 19 — `gohome automation` CLI  ✅ DONE

**Files:** `internal/cli/cmd_automation.go`, `internal/cli/styles_automation.go`

- [x] **Step 1** — Subcommands: `list`, `get <id>`, `enable <id>`, `disable <id>`, `trigger <id>`, `trace <corr-or-id>`.
- [x] **Step 2** — Lipgloss styles per spec §14.3: separators, IDs (entity-id style), outcomes (success/error/dim).
- [x] **Step 3** — Outputs respect `--format=human|json`.

**Acceptance:** `gohome automation list` against a running daemon produces a styled table; exit codes are non-zero only on failure.

**Status:** Files present; commands implemented over Connect-RPC `AutomationService` (see Known Deviations §1).

---

### Task 20 — `gohome script` CLI  ✅ DONE

**Files:** `internal/cli/cmd_script.go`

- [x] **Step 1** — Subcommands: `list`, `run <name> [--arg key=value]…`.
- [x] **Step 2** — Stream `stdout`/`stderr` from the script call.

**Acceptance:** `gohome script run hello --arg name=world` prints the script's stdout and exits zero on success.

**Status:** Implemented over Connect-RPC `ScriptService`.

---

### Task 21 — Integration: golden-path automation  ✅ DONE

**Files:** `internal/automation/integration_test.go` (`TestIntegration_GoldenPath`)

**Acceptance (spec §16.3 #1):** Motion sensor `off→on` fires light turn-on; the full event chain (`AutomationTriggered` → `CommandIssued` → `CommandAck` → `AutomationFinished{OK}`) is observed under one correlation ID.

**Status:** Present at `integration_test.go:25`.

---

### Task 22 — Integration: hold-duration trigger  ✅ DONE

**Files:** `integration_test.go` (`TestIntegration_HoldDurationFires`, `TestIntegration_HoldDurationCancels`)

**Acceptance (spec §16.3 #2):** `forDur: 5s` fires only on sustained state; spurious intermediate changes suppress.

**Status:** Two tests at `integration_test.go:146,188`.

---

### Task 23 — Integration: mode=restart cancellation  ✅ DONE

**Files:** `integration_test.go` (`TestIntegration_ModeRestartCancelsPrior`)

**Acceptance (spec §16.3 #3):** Rapid re-fire cancels the prior run (emits `CANCELLED`); the new run completes `OK`.

**Status:** Present at `integration_test.go:223`.

---

### Task 24 — Integration: condition short-circuit  ✅ DONE

**Files:** `integration_test.go` (`TestIntegration_ConditionShortCircuit`)

**Acceptance (spec §16.3 #5):** Cheap typed condition fails → Starlark runtime not invoked (verified via call counter).

**Status:** Present at `integration_test.go:277`.

---

### Task 25 — Integration: scene stub  ✅ DONE

**Files:** `integration_test.go` (`TestIntegration_SceneStub`)

**Acceptance (spec §16.3 #6):** `SceneAction` appends `SceneApplied`; no commands dispatched.

**Status:** Present at `integration_test.go:329`.

---

### Task 26 — Integration: script-from-automation correlation  ✅ DONE

**Files:** `integration_test.go` (`TestIntegration_ScriptInvocationCorrelation`)

**Acceptance (spec §16.3 #7):** Same correlation ID threads from `AutomationTriggered` through `ScriptInvoked` and `ScriptFinished`.

**Status:** Present at `integration_test.go:400`.

---

### Task 27 — Integration: parallel-block cancellation  ✅ DONE

**Files:** `integration_test.go` (`TestIntegration_ParallelBlockCancellation`)

**Acceptance (spec §16.3 #8):** One failing action cancels siblings within 50 ms; outcome is `OUTCOME_ACTION_ERROR`.

**Status:** Present at `integration_test.go:511`.

---

### Task 28 — Integration: mode=queued overflow  ⚠️ PARTIAL

**Files (planned):** `integration_test.go` `TestIntegration_ModeQueuedOverflow`

**Acceptance (spec §16.3 #4):** Five rapid fires with `maxQueued: 2` → 2 complete `OK`, 3 emit `SKIPPED` with reason `queue full`. The integration version must use a real eventstore so emission ordering is observable.

**Status:** Only `TestMode_QueuedOverflow` (unit, in `mode_test.go`) exists; no integration counterpart. Captured in the C1–C8 cleanup list as "P1 #7 — author 4 missing C6 integration tests".

---

### Task 29 — Integration: reload preserves in-flight  ❌ MISSING

**Files (planned):** `integration_test.go` `TestIntegration_ReloadPreservesInFlight`

**Acceptance (spec §16.3 #9):** While automation A runs, add automation B; A is uninterrupted, B becomes registered; both complete cleanly.

**Status:** Not implemented. `TestReload_AddsNew` is unit-only and does not run a real fire concurrently.

---

### Task 30 — Integration: reload cancels removed  ❌ MISSING

**Files (planned):** `integration_test.go` `TestIntegration_ReloadCancelsRemoved`

**Acceptance (spec §16.3 #10):** Remove automation A while it is running; A emits `CANCELLED` and the engine drops its matchers.

**Status:** `TestReload_RemovesOld` is unit-only — does not exercise an active in-flight run, so the `CANCELLED` emission is not asserted end-to-end.

---

### Task 31 — Integration: CLI `automation trigger`  ❌ MISSING

**Files (planned):** `integration_test.go` `TestIntegration_CLITrigger`

**Acceptance (spec §16.3 #11):** Spawn `gohomed`, dial via Connect-RPC, run `gohome automation trigger <id>`, assert exit code zero and styled output.

**Status:** Not implemented.

---

### Task 32 — Integration: CLI `automation trace`  ❌ MISSING

**Files (planned):** `integration_test.go` `TestIntegration_CLITrace`

**Acceptance (spec §16.3 #12):** Capture a correlation ID from `automation trigger`, then `gohome automation trace <corr>` renders all expected nodes (Triggered → action events → Finished) with correct style classes.

**Status:** Blocked at audit time by the `Trace` adapter being a TODO stub (`internal/daemon/api_adapters.go:Trace`). Adapter has since been wired (cleanup task #3) but the integration test itself is still missing.

---

### Task 33 — Race tests  ✅ DONE

**Files:** `internal/automation/race_test.go`

- [x] `TestRace_ModeSingle100Fires` — 100 concurrent fires under `mode=single`: exactly one admitted, rest `SKIPPED`, no deadlocks (spec §16.4 #1).
- [x] `TestRace_ModeParallel100Fires` — 100 concurrent fires under `mode=parallel`: all complete, no shared-state corruption (spec §16.4 #2).
- [x] `TestRace_ReloadDuringActiveFire` — reload racing an active fire: pre- or post-swap, never half-compiled (spec §16.4 #3).

**Acceptance:** `task test:race` clean across all three.

**Status:** All present in `race_test.go`.

---

### Task 34 — Webhook trigger compile test  ❌ MISSING

**Files (planned):** `internal/automation/compile_test.go` — `TestCompile_WebhookTriggerValidates`

**Acceptance (spec §16.5):** Webhook matcher is inert in v1; a single compile test confirms it validates against the Pkl shape and round-trips through the snapshot proto. The C7 design later activates it; this single test exists only to ensure the v1 bake-in does not silently drop the variant.

**Status:** Not present. Webhook is wired into the trigger registry but no targeted compile-test exists.

---

### Task 35 — Trigger fakes location  ⚠️ PARTIAL

**Files (planned):** `internal/automation/trigger/fake.go`

**Acceptance (spec §15.1 file map):** Trigger fakes live in `trigger/fake.go` so unit tests in `trigger/` can use them without import cycles back through `testutil`.

**Status:** No `trigger/fake.go`; equivalent helpers live in `internal/automation/testutil/synth.go`. Functionally adequate but off-spec layout — captured as cleanup task #11.

---

### Task 36 — Test fixtures directory  ❌ MISSING

**Files (planned):** `internal/automation/testutil/fixtures/`

**Acceptance (spec §15.1):** Golden-file fixtures for compile-error aggregation live in a dedicated `fixtures/` directory under `testutil`.

**Status:** Directory absent. Compile-error tests inline their fixtures in `compile_test.go`. Captured as cleanup task #12.

---

### Task 37 — Script testutil  ❌ MISSING

**Files (planned):** `internal/script/testutil/synth.go`

**Acceptance (spec §15.1):** Per-package test helpers (param fixtures, event-store fakes specific to script).

**Status:** Directory absent. Script tests reuse `internal/starlark/testutil` and `internal/automation/testutil` directly. Captured as cleanup task #12.

---

### Task 38 — `Trace` daemon-adapter wiring  ⚠️ PARTIAL

**Files:** `internal/daemon/api_adapters.go`, `internal/automation/integration_test.go`

**Acceptance (spec §14.2 / D11):** `gohome automation trace <corr-or-id>` streams `AutomationTriggered` + `AutomationFinished` (plus future per-step events) filtered by automation_id and/or correlation_id.

**Status:** At audit time (2026-04-29) the adapter was a TODO stub. Cleanup task #3 wired it to subscribe to the eventstore filtered by kinds `automation_triggered`/`automation_finished` and (optionally) by `Source`/`CorrelationIDs`. Marked PARTIAL because the integration test (Task 32) is still missing.

---

### Task 39 — Coverage gate (D16: 75%)  ⚠️ PARTIAL

**Files:** `.github/workflows/*.yml`

**Acceptance (D16):** `task test:race` produces ≥ 75% line coverage across `internal/automation/...` and `internal/script/...` and CI fails if below.

**Status:** Coverage at audit time:

| Package | Coverage |
|---|---|
| `internal/automation/action` | 76.9% ✅ |
| `internal/automation` (top-level) | 56.2% ❌ |
| `internal/automation/condition` | 69.2% ❌ |
| `internal/automation/trigger` | 61.3% ❌ |
| `internal/script` | 74.2% ❌ (just below) |

CI does not enforce the gate today. Captured as cleanup task #6.

---

## Known deviations from the spec

1. **Daemon socket ops superseded by Connect-RPC.** Spec §14.1 named a new `internal/daemon/socket.go` adding `automation_*` and `script_*` op-codes to a Unix-socket JSON protocol. Implementation took the C7 Connect-RPC path instead — `AutomationService` and `ScriptService` cover the same surface (List/Get/Enable/Disable/Trigger/Trace; List/Run). Functionally equivalent but the spec-named file does not exist.
2. **`internal/automation/trigger/fake.go` missing.** Trigger fakes live in `internal/automation/testutil/synth.go`. Off-spec layout, no functional impact.
3. **`internal/automation/testutil/fixtures/` missing.** Compile-error fixtures inlined in `compile_test.go` rather than living in their own directory.
4. **`internal/script/testutil/synth.go` missing.** Script tests reuse C5 starlark testutil + automation testutil.
5. **`Trace` daemon adapter was a TODO stub at first audit.** Wired by cleanup task #3 after the audit. CLI integration tests for `automation trace` (and `automation trigger`) still TBD.
6. **Webhook compile test missing.** Webhook matcher is wired and Pkl-validated; no targeted compile test confirms it round-trips through `task proto`.
7. **Coverage gate not enforced.** D16 calls for ≥ 75% across `automation/` + `script/`; current coverage misses in 4 of 6 sub-packages and CI does not gate on it.
8. **4 of 12 integration scenarios missing** (Tasks 28, 29, 30, 31, 32 above) — one PARTIAL (queued overflow has unit-only) plus four outright missing.

---

## Definition of Done (mirrors spec §16)

- [x] `task build` (both binaries)
- [x] `task test` (unit, race-detector clean)
- [x] `task test:race` (race tests in `race_test.go`)
- [x] `task test:integration` — 8 of 12 §16.3 scenarios green
- [ ] `task test:integration` — **all 12** §16.3 scenarios green ❌ (Tasks 28–32 outstanding)
- [x] `task lint`
- [x] `go mod tidy` clean
- [x] `task proto` — `gen/` reflects all C6 additions
- [ ] Coverage ≥ 75% across `internal/automation/...` + `internal/script/...` ❌
- [ ] CI fails on coverage regression ❌
- [x] Webhook matcher Pkl-validates and is wired into the trigger registry
- [ ] Webhook compile test asserts round-trip ❌
- [x] `gohome automation {list,get,enable,disable,trigger}` work end-to-end
- [ ] `gohome automation trace <corr>` works end-to-end ⚠️ (adapter wired post-audit; integration test pending)
- [x] `gohome script {list,run}` work end-to-end
- [x] Daemon `ConfigApplied` triggers `script.Reload` then `automation.Reload`; in-flight runs uninterrupted

---

*End of C6 implementation plan.*
