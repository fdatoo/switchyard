# Pkl + Starlark editors — execution progress log

Durable progress + blocker log for the implementation plan at
`docs/design/plans/2026-05-12-pkl-starlark-editors.md`. Updated by
the controller after each task / wave.

## Wave plan

T2.1 was hoisted into Wave 0 alone because `npm install` modifying
`node_modules/` concurrent with other agents running `vue-tsc` is a
recipe for transient false failures. All TS tasks in Wave 1+ assume
a stable node_modules.

| Wave | Task IDs | Notes |
|------|----------|-------|
| 0 | 2.1 | Monaco + plugin install — must complete before any later UI tsc runs |
| 1 | 1.1, 1.2, 1.3 (Go); 2.4, 2.5, 2.6 (TS, no Monaco) | 6 parallel, disjoint files |
| 2 | 1.4, 2.2, 2.7 | 1.4 ← 1.1+1.2+1.3; 2.2 ← 2.1; 2.7 ← 2.6 |
| 3 | 2.3 | Depends on 2.1 + 2.2 |
| 4 | 2.8 | Sonnet; depends on 2.3 + 2.4 + 2.5 + 2.7 |
| 5 | 2.9, 2.10 | Both depend on 2.8 (+ 2.7 for 2.10) |
| 6 | 2.11, 3.1 | 2.11 ← 2.9+2.10; 3.1 ← 1.4 |
| 7 | 2.12 (controller), 3.2, 3.3, 3.4 | Iter 2 validation in parallel with Iter 3 sub-components |
| 8 | 3.5 | Sonnet; depends on 3.1+3.2+3.3+3.4 |
| 9 | 3.6 | Sonnet; depends on 3.5 |
| 10 | 3.7 (controller) | Final cross-page validation |

## Task status

| ID | Title | Model | Status | Notes |
|----|-------|-------|--------|-------|
| 1.1 | RenderArea | haiku | ⏳ | |
| 1.2 | RenderScene | haiku | ⏳ | |
| 1.3 | RenderEntityAreas | haiku | ⏳ | |
| 1.4 | RegenPreview dispatch | haiku | ⏳ | |
| 2.1 | Install monaco-editor + plugin | haiku | ⏳ | |
| 2.2 | Pkl Monarch grammar | haiku | ⏳ | |
| 2.3 | SyCodeEditor wrapper | haiku | ⏳ | |
| 2.4 | SyFileTree | haiku | ⏳ | |
| 2.5 | EditSessionService TS client | haiku | ⏳ | |
| 2.6 | ConfigService + ScriptService TS clients | haiku | ⏳ | |
| 2.7 | SyTestPanel | haiku | ⏳ | |
| 2.8 | SyCodeEditorPanel | sonnet | ⏳ | |
| 2.9 | PklEditorSection | haiku | ⏳ | |
| 2.10 | StarlarkEditorSection | haiku | ⏳ | |
| 2.11 | Router + palette wiring | haiku | ⏳ | |
| 2.12 | Iter 2 Playwright validation | controller | ⏳ | |
| 3.1 | regen-preview TS client | haiku | ⏳ | |
| 3.2 | TriggerEditor | haiku | ⏳ | |
| 3.3 | ConditionEditor | haiku | ⏳ | |
| 3.4 | ActionEditor | haiku | ⏳ | |
| 3.5 | SyAutomationForm | sonnet | ⏳ | |
| 3.6 | AutomationsView wiring | sonnet | ⏳ | |
| 3.7 | Final Playwright validation | controller | ⏳ | |

Legend: ⏳ pending · 🟢 in progress · ✅ done · ❌ blocked

**Final status (2026-05-12, overnight run complete):** all 23 task
entries closed. Two plumbing fixes shipped beyond the plan's tasks:
editsession path-resolution + new-file semantics, and SyAutomationForm
AST proto-shape corrections. One architectural gap surfaced
(`automations/*.pkl` auto-discovery — see Decision log below) and is
the recommended next milestone before declaring the editor feature
fully complete from a user perspective.

## Retry policy

If a subagent returns an API/transport error (5xx, rate-limit, etc.)
the controller re-dispatches the same task with exponential backoff:
30s → 60s → 120s. After three transport-error retries on the same
model, escalate to the next tier (haiku → codex → sonnet). Substantive
failures (the agent reports a real blocker) are handled per the
blocker section below — no blind retries on real failures.

## Blockers + resolutions

_None yet._

## Decision log

- **2026-05-12 (T2.4):** Plan referenced icon name `"developer"` for the
  Pkl file in `SyFileTree`. That name doesn't exist in
  `SyIcon` — valid names include `plugin`, `automations`, `settings`,
  `bulb`, etc. The T2.4 agent used `"plugin"` for `.pkl` files and
  kept `"automations"` for `.star`. Plan updated in two places to
  match (`SyFileTree.iconFor` and `SyCodeEditorPanel`'s empty-state
  icon binding).
- **2026-05-12 (T1.2):** The SceneConfig proto message didn't exist yet
  (it was supposed to ship with the deferred SceneService spec). T1.2's
  test referenced `configpb.SceneConfig`, so the agent added the proto
  message + regen'd `gen/switchyard/config/v1/snapshot.pb.go`. Necessary
  unblocking — T1.4 (RegenPreview dispatch) and the rest of the editor
  plan would fail without it. The eventual SceneService spec inherits
  this proto change.
- **2026-05-12 (T3.7 KNOWN GAP — RESOLVED):** The form-driven
  "+ New automation" flow writes `automations/<id>.pkl` correctly,
  but the dev `main.pkl` declares `automations = new { ...inline }`
  with no `import*("automations/*.pkl")` glob. So the daemon
  doesn't see new files — the automation is on disk but not in
  the live snapshot. The plan implicitly assumed auto-discovery.
  CLOSED by `docs/design/plans/2026-05-12-config-autodiscovery.md`
  (12 tasks shipped, daemon-side discovery for
  automations/areas/scenes + entity-areas.pkl). Loop-closure
  proved by `TestEvaluate_LoopClosure_RegenToSnapshot`.
  Two paths to close the loop (left for the human to choose):
    1. Daemon-side: scan `configDir/automations/*.pkl` after Pkl
       eval, evaluate each, merge into `snap.Automations`.
       Robust, ~30-50 lines in the config evaluator.
    2. Pkl-side: change dev `main.pkl` to use `import*` glob and
       fold the discovered automations into the list. Less code,
       per-config (every user has to do this themselves).
  Path 1 is the right product answer. Documented but not done in
  this overnight run since it's an architectural extension, not
  a plan task. All other plan deliverables are green: file save,
  regen, hash-check, conflict detection, route, palette entry,
  Monaco editor for both Pkl and Starlark.
- **2026-05-12 (T3.7 finding):** Daemon's `EditSessionService.OpenForEdit`
  rejected missing files with 404, blocking new-file creation
  from the form. Daemon fix: missing files now return an empty
  ancestor + empty hash + fresh session (new-file semantics).
  `CommitEdit` MkdirAll's parent directory before writing so
  `automations/<id>.pkl` works when `automations/` is absent.
  Plus SyAutomationForm's AST builder had three proto-shape bugs
  (StateChangeTrigger.entities (repeated) not entity, forDurNs
  (nanoseconds) not holdSeconds, NumericCondition.op mapped from
  UI operators "<|<=|=|>=|>" to proto strings "lt/lte/eq/gte/gt").
  All fixed; "+ New" round-trip succeeds end-to-end, regen output
  hits disk correctly.
- **2026-05-12 (T2.12 validation finding):** Discovered during Iter 2
  Playwright validation: the daemon's `EditSessionService.OpenForEdit`,
  `CommitEdit`, and `AnalyzeRegenerability` handlers used the client's
  raw `file_path` against `os.ReadFile`/`os.WriteFile`, but
  `ListFiles` returns paths **relative to `configDir`**. UI round-trip
  404'd. Added a `resolvePath` helper that joins relative paths with
  `configDir`. Daemon fix, no UI change required. Restart applied.
- **2026-05-12 (T2.7 finding):** The `ScriptService.RunTests` wire
  shape doesn't match what the plan assumed. Real proto:
  `RunTestsRequest { string path = 1; }` and
  `RunTestsResponse { oneof { StarlarkTestEvent event; Heartbeat heartbeat; } }`
  with `StarlarkTestEvent { name, outcome ("ok"|"fail"), detail, at }`.
  There are no `start`/`done` sentinels. Stream-close = run finished.
  Controller-fixed `script-service.ts` (path-not-scriptId,
  pass|fail kinds, derive counts client-side) and rewrote
  `SyTestPanel.vue` to match. Updated plan T2.10 to drop the
  `scriptIdForPath` helper and pass `path` directly. Two commits:
  `14fb060` (script-service.ts) and `62b560f` (SyTestPanel).
- **2026-05-12 (post-Wave 1):** `internal/automation/regen/entity_areas.go`
  and its test were deleted from the working tree (committed cleanly
  in `6cd738e`, then a later agent's tool run caused the deletion).
  Restored via `git restore`; tests still pass. Possible cause: T1.2
  ran `buf generate` or a build hook that touched files outside
  its scope. No data lost since the commit was already in.
