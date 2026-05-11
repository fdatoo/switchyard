# Plan 10 — Automation Editor

> **Editor plan.** Requires Plan 01 merged to main and Plan 11 (Pkl ↔ UI architecture) merged to main before this branch starts. Plan 11 provides `ConfigService.OpenForEdit`, `CommitEdit`, `AbandonEdit`.

**Goal:** A full automation editor with four typed section cards (Trigger, When, Do, OnFailure), a live 380px Pkl source pane with dirty/locked line tinting, mixed editability for Starlark fields, and a save flow wired to Plan 11's edit-session RPCs — replacing the Plan 01 placeholder routes at `/_authed/automations` and `/_authed/automations/$slug`.

**Spec refs:** §16 (Automation editor), §17 (Pkl ↔ UI architecture), §24 (multi-trigger deferral).

**Mockups:** `.superpowers/brainstorm/71337-1778492716/screenshots/12-automation-editor-01.png` (editor in edit state), `12-automation-editor-02.png` (conflict resolution).

**Branch:** `feat/ui-v2-plan-10-automation-editor`
**Worktree:** `.claude/worktrees/plan-10-automation-editor`
**Depends on:** Plan 01 merged to main + Plan 11 merged to main
**Linear parent:** TBD

---

## Decisions (locked — no ambiguity for the implementer)

1. **Routes:** `/_authed/automations` (list) and `/_authed/automations/$slug` (editor). The slug is the automation `id` from `AutomationConfig`, mapping 1:1 to the file `automations/<slug>.pkl` in the config dir.

2. **Four section cards in order: Trigger → When → Do → OnFailure.** Each card is collapsible but open by default. Collapsed headers show a one-line summary chip (e.g., "Sun event — sunset").

3. **Single trigger only in v2.** Per spec §24. The Trigger card shows one trigger at a time, type-selected via a `<select>` at the top. If the loaded automation has multiple triggers, the card shows a read-only banner "Multiple triggers — edit in Pkl editor →" and disables all form fields.

4. **Trigger types and their fields:**
   - `SunEvent` — event select (sunrise / sunset) + offset field (± minutes). Encoded as `EventTrigger { kind = "sun.sunrise" | "sun.sunset"; data["offset_ns"] }`.
   - `Time` — HH:MM `TimePicker`, with "Advanced: use cron" toggle switching to a raw cron string field.
   - `EntityStateChange` — `EntityPicker` (multi-select) + optional `from` / `to` / `forDur` fields.
   - `Webhook` — path text field + methods checkboxes (GET / POST / PUT).
   - `Manual` — read-only note: "Runs only when triggered manually or via Run now." Encoded as `EventTrigger { kind = "switchyard.manual" }`.

5. **Condition tree.** `WhenSection` renders a recursive `ConditionBuilder`. Leaf types: `EntityEq` (`StateCondition.equals`), `EntityNeq` (`StateCondition.not`), `EntityGreaterThan` (`NumericCondition op="gt"`), `TimeInRange` (`TimeCondition`). Group nodes: `All` / `Any` / `Not`. A `StarlarkCondition` leaf renders locked (expression preview truncated to 3 lines, "starlark" pill, "View in Pkl editor →").

6. **Action types in DoSection:**
   - `TurnOn` → `CallServiceAction { capability: "turn_on" }` — `EntityPicker`.
   - `SetBrightness` → `CallServiceAction { capability: "set_brightness", args.level }` — `EntityPicker` + `BrightnessSlider` (0–100%).
   - `RunScript` → `ScriptAction` — script-name select (from snapshot).
   - `Notify` → `CallServiceAction { capability: "notify", args.message }` — `EntityPicker` (filtered to `notify.*`) + message textarea.
   - `CallCapability` → `CallServiceAction` free-form — entity + capability text fields.
   - `SceneAction` → `ScenePicker`.
   - `WaitAction` → duration number + unit selector (s / min / h).
   - `StarlarkAction` → locked: body preview, "starlark" pill, "View in Pkl editor →", `LockedFieldBanner`.
   - `SequenceBlock` / `ParallelBlock` → nested action list with Add affordance; only one level deep in the form; deeper nesting shows "Open in Pkl editor →".

7. **Locked fields.** Any proto variant the regenerator cannot round-trip renders as: grey monospace preview (3-line truncation), a `--sy-color-purple` "starlark" chip, a "View in Pkl editor →" link, and a `LockedFieldBanner` above the parent card reading "This [action/condition] uses a Starlark expression. Edit the expression in the Pkl editor; all other fields remain editable."

8. **Right rail — `PklSourcePane` at 380px.** Shows the regenerated Pkl source for `automations/<slug>.pkl` only. Line-by-line annotations: orange tint (`color-mix(in srgb, var(--sy-color-warn) 12%, transparent)`) for dirty lines. Grey tint (`color-mix(in srgb, var(--sy-color-fg-5) 8%, transparent)`) for locked regions (lines between `// locked-region:starlark` and `// end-locked-region` comments emitted by the regenerator). Tab bar: "Source" (default) and "Diff vs disk (N)" — the Diff tab shows a unified-diff view with `+` lines on green tint, `-` lines on red tint.

9. **Save flow using Plan 11 RPCs.**
   - On mount: call `ConfigService.OpenForEdit(filePath)` → receive `{ ast_json, lock_token, file_hash }`.
   - On every form change: update local AST; call `ConfigService.RegenPreview(fileType: "automation", ast_json)` → receive updated Pkl bytes; push to `PklSourcePane`.
   - "Save & exit": call `ConfigService.CommitEdit(filePath, lock_token, pkl_bytes, file_hash)`. If `committed=true` → navigate to `/_authed/automations`. If `conflict=true` → show `ConflictBanner`.
   - `ConflictBanner` offers three choices (per §17.3): (1) Discard mine, reload from file — call `AbandonEdit` then reload, (2) Overwrite with my changes — re-call `CommitEdit` with `file_hash` set to the on-disk hash from the conflict response, (3) Open 3-way merge — navigate to `/_authed/pkl-editor?file=<path>&merge=true` (Plan 12 owns this route).
   - "Discard": call `AbandonEdit`, navigate to list.
   - Unmount without explicit save: `useEffect` cleanup calls `AbandonEdit`.

10. **`useAutomationEditor` hook** encapsulates the full edit-session state machine: `{ editorState, pklSource, isDirty, conflict, lockToken, updateTrigger, updateConditions, updateActions, updateOnFailure, save, discard }`. All section components receive slice props + onChange callbacks; none talk to RPCs directly.

11. **`OnFailureSection` fields.** Strategy selector: "Do nothing" (default `IgnoreStrategy`), "Retry" (`RetryStrategy { maxAttempts, backoff }`), "Notify" (`NotifyStrategy { entity, message }`). Conditional fields render below the selector.

12. **"Run now"** calls `AutomationService.Trigger(id)` → `{ run_id }`, then navigates to `/_authed/time-machine/<run_id>` (Plan 04's route). No other wiring in Plan 10.

13. **`OnFailureConfig` proto** is new in this plan. Added to `switchyard/config/v1/snapshot.proto` as `on_failure = 13` in `AutomationConfig`. Three strategy messages: `RetryStrategy`, `NotifyStrategy`, `IgnoreStrategy`. Pkl schema gains matching `OnFailureStrategy` classes in `internal/config/pkl/switchyard/automations.pkl`.

14. **`RegenPreview` RPC** is new in this plan. Added to `ConfigService` in `proto/switchyard/v1alpha1/config.proto`. Accepts `{ file_type: "automation" | "page", ast_json }`, returns `{ pkl_bytes, error }`. Backed by `internal/automation/regen/` (new package, mirrors `internal/dashboard/regen/` in structure).

15. **`GetAutomationDetail` RPC** is new in this plan. Added to `AutomationService`. Returns `{ automation, ast_json, file_path }`. `ast_json` is the JSON-encoded `AutomationConfig` for the editor to hydrate its local state.

16. **Mobile.** On viewports < 760 px, `PklSourcePane` is hidden; a "View Pkl source" button in the top bar opens it as a bottom sheet. Section cards stack full-width. Full mobile polish is Plan 13.

---

## File plan

### Proto

```
proto/switchyard/config/v1/snapshot.proto     ← add OnFailureConfig + on_failure field to AutomationConfig
proto/switchyard/v1alpha1/automation.proto    ← add GetAutomationDetail RPC
proto/switchyard/v1alpha1/config.proto        ← add RegenPreview RPC to ConfigService
```

### Go

```
internal/automation/regen/regen.go            ← deterministic Pkl regenerator for AutomationConfig
internal/automation/regen/regen_test.go       ← golden tests (one per trigger/condition/action variant)
internal/automation/regen/testdata/           ← golden *.pkl files
internal/config/pkl/switchyard/automations.pkl ← add OnFailureStrategy classes
internal/api/automation_handler.go            ← add GetDetail handler
internal/api/config_edit_handler.go           ← add RegenPreview handler (thin; real lock logic in Plan 11)
```

### Web

```
web/src/routes/_authed/automations/index.tsx  ← replaces Plan 01 placeholder
web/src/routes/_authed/automations/$slug.tsx  ← replaces Plan 01 placeholder
web/src/pages/automations/AutomationList.tsx
web/src/pages/automations/AutomationEditor.tsx
web/src/pages/automations/useAutomationEditor.ts
web/src/pages/automations/sections/TriggerSection.tsx
web/src/pages/automations/sections/WhenSection.tsx
web/src/pages/automations/sections/DoSection.tsx
web/src/pages/automations/sections/OnFailureSection.tsx
web/src/pages/automations/actions/TurnOnAction.tsx
web/src/pages/automations/actions/SetBrightnessAction.tsx
web/src/pages/automations/actions/RunScriptAction.tsx
web/src/pages/automations/actions/NotifyAction.tsx
web/src/pages/automations/actions/CallCapabilityAction.tsx
web/src/pages/automations/actions/SceneAction.tsx
web/src/pages/automations/actions/WaitAction.tsx
web/src/pages/automations/actions/StarlarkActionLocked.tsx
web/src/pages/automations/editors/EntityPicker.tsx
web/src/pages/automations/editors/ScenePicker.tsx
web/src/pages/automations/editors/BrightnessSlider.tsx
web/src/pages/automations/editors/TimePicker.tsx
web/src/pages/automations/editors/ConditionBuilder.tsx
web/src/pages/automations/PklSourcePane.tsx
web/src/pages/automations/LockedFieldBanner.tsx
web/src/pages/automations/ConflictBanner.tsx
web/e2e/automation-editor-snapshot.spec.ts
```

### Sample

```
examples/automations/sunset-lights.pkl        ← matches the mockup
```

---

## Tasks

### Task 10.1 — Proto: `OnFailureConfig` + `GetAutomationDetail` + `RegenPreview`

**Files:** `proto/switchyard/config/v1/snapshot.proto`, `proto/switchyard/v1alpha1/automation.proto`, `proto/switchyard/v1alpha1/config.proto`

Add `OnFailureConfig` message (with `RetryStrategy`, `NotifyStrategy`, `IgnoreStrategy` variant messages) to `snapshot.proto`. Add field `on_failure = 13` to `AutomationConfig`. Add `GetAutomationDetail` RPC + messages to `AutomationService`. Add `RegenPreview` RPC + messages to `ConfigService`. Run `buf generate`. Run `go build ./...` — zero errors.

Follow `dev/proto-hygiene.md` conventions: field 13 reserved after field 12, package comment, no `_UNSPECIFIED` sentinel on the strategy oneof (presence encodes the choice). `GetAutomationDetailResponse` carries `ast_json string` (JSON of `AutomationConfig`), `file_path string`.

**Acceptance:** `buf lint` clean; generated files in `gen/` land correctly; `go build ./...` green.

**Commit:** `feat(proto): OnFailureConfig + GetAutomationDetail + RegenPreview (plan 10)`

---

### Task 10.2 — Pkl schema: `OnFailureStrategy` classes

**File:** `internal/config/pkl/switchyard/automations.pkl`

Add abstract `OnFailureStrategy` and three concrete classes: `RetryStrategy { maxAttempts: Int(isPositive); backoff: Duration }`, `NotifyStrategy { entity: String(!isEmpty); message: String(!isEmpty) }`, `IgnoreStrategy {}`. Add field `onFailure: OnFailureStrategy = new IgnoreStrategy {}` to `class Automation`. Run `go build ./...`.

**Acceptance:** `go build ./...` green; existing `go test ./internal/automation/...` still passes.

**Commit:** `feat(pkl): OnFailureStrategy classes (plan 10)`

---

### Task 10.3 — Go: automation regenerator (`internal/automation/regen/`)

**Files:** `internal/automation/regen/regen.go`, `internal/automation/regen/regen_test.go`, `internal/automation/regen/testdata/*.pkl`

Model the package on `internal/dashboard/regen/`. Export a single `Render(ac *configpb.AutomationConfig) ([]byte, error)` function. Output is deterministic: the header is a fixed `import "switchyard:automations" as auto` + generated comment; triggers in proto list order; locked Starlark blocks annotated with `// locked-region:starlark` / `// end-locked-region` comments (the `PklSourcePane` uses these to tint lines grey). `OnFailureConfig` serialises to the correct Pkl class; omitted when nil (default `IgnoreStrategy`).

**TDD first** — one golden test per variant before implementing `Render`:
- `TestRender_TimeTrigger` — `TimeTrigger.at = "21:30"`.
- `TestRender_StateChangeTrigger` — `StateChangeTrigger { entities, to }` + `CallServiceAction`.
- `TestRender_StarlarkCondition_LockedAnnotation` — `StarlarkCondition` emits the `// locked-region` marker pair.
- `TestRender_OnFailure_Retry` — `RetryStrategy { maxAttempts=3, backoff=5.s }`.
- `TestRender_Deterministic` — same `AutomationConfig` with two actions in proto order produces byte-identical output across 20 calls.

Seed golden files from actual `Render` output once the implementation makes the tests pass. Commit the goldens.

**Acceptance:** `go test ./internal/automation/regen/... -v` — all 5 tests pass.

**Commit:** `feat(regen): automation Pkl regenerator + golden tests (plan 10)`

---

### Task 10.4 — Go: server handlers for `GetDetail` and `RegenPreview`

**Files:** `internal/api/automation_handler.go` (modify), `internal/api/config_edit_handler.go` (create)

`GetDetail` handler: look up the `AutomationConfig` by `id` from the live snapshot, JSON-encode it via `protojson.Marshal`, resolve `file_path` as `<config_dir>/automations/<id>.pkl`.

`RegenPreview` handler: for `file_type = "automation"`, unmarshal `ast_json` into `*configpb.AutomationConfig` via `protojson.Unmarshal`, call `regen.Render`, return bytes. Return a gRPC `InvalidArgument` error (with the parse/render error text) if `ast_json` is malformed; do not crash. `OpenForEdit`, `CommitEdit`, `AbandonEdit` are Plan 11's responsibility — stub them here with `codes.Unimplemented` so the plan compiles independently.

**TDD:** table-driven tests covering valid `ast_json`, malformed JSON, and unknown `file_type`.

**Acceptance:** `go test ./internal/api/...` green; `go build ./...` green.

**Commit:** `feat(api): GetDetail + RegenPreview handlers (plan 10)`

---

### Task 10.5 — Web: `AutomationList` page + list route

**Files:** `web/src/pages/automations/AutomationList.tsx`, `web/src/routes/_authed/automations/index.tsx`

Replace the Plan 01 placeholder. `AutomationList` queries `AutomationService.List` via `useQuery`. Renders a `<ul>` of automation rows: each row is a `<Link to="/_authed/automations/$slug">`, shows `displayName || id`, and shows a "disabled" chip when `enabled = false`. Empty state: "No automations yet. Create one in `~/.switchyard/automations/`."

**TDD:** mock `useQuery` returning two automations (one disabled); assert both names render; assert the disabled chip appears for the disabled one; assert each link's `href`.

**Acceptance:** `npm run test -- AutomationList --run` green; visiting the route renders without crash.

**Commit:** `feat(web): AutomationList page + list route (plan 10)`

---

### Task 10.6 — Web: `useAutomationEditor` hook

**File:** `web/src/pages/automations/useAutomationEditor.ts`

State machine hook. On mount: call `OpenForEdit(filePath)`, store `lockToken` + `fileHash` in refs, parse `ast_json` into `editorState`. On `editorState` change: call `RegenPreview` and store the result in `pklSource`. Exports: `editorState`, `pklSource`, `isDirty`, `conflict`, `lockToken`, `updateTrigger`, `updateConditions`, `updateActions`, `updateOnFailure`, `save` (calls `CommitEdit`; sets `conflict` on `CONFLICT` response), `discard` (calls `AbandonEdit`). Cleanup on unmount: `AbandonEdit` if `lockToken` is set.

**TDD (mock `useMutation`):**
- Mount: asserts `OpenForEdit` called with `{ filePath: "automations/sunset-lights.pkl" }`; `isDirty` starts `false`.
- `updateTrigger(...)`: asserts `isDirty` becomes `true`.
- `save()` on clean commit: asserts `CommitEdit` called; `isDirty` resets to `false`.
- `save()` on conflict response: asserts `conflict` is set (non-null).
- `discard()`: asserts `AbandonEdit` called.

**Acceptance:** `npm run test -- useAutomationEditor --run` green.

**Commit:** `feat(web): useAutomationEditor hook (plan 10)`

---

### Task 10.7 — Web: `AutomationEditor` shell + route wiring

**Files:** `web/src/pages/automations/AutomationEditor.tsx`, `web/src/routes/_authed/automations/$slug.tsx`

Replace the Plan 01 placeholder. `AutomationEditor` receives `slug` as a prop (passed by the route). Renders: header with `<h1>`, Discard button, "Run now" button, "Save & exit" button (disabled when `!isDirty`). Below header: `ConflictBanner` (conditional on `conflict`). Then the four section cards. Right of the section stack: `PklSourcePane`. Layout: `display: grid; grid-template-columns: 1fr 380px` — collapses to single column below 760 px (source pane hidden, "View Pkl source" button appears in header).

The "Run now" button calls `AutomationService.Trigger(slug)` → navigates to `/_authed/time-machine/<run_id>`.

**TDD:** render with mocked `useAutomationEditor` returning `isDirty = false`; assert "Save & exit" is disabled. Render with `isDirty = true`; assert enabled. Render with non-null `conflict`; assert `ConflictBanner` is present.

**Acceptance:** `npm run test -- AutomationEditor --run` green; route renders without crash.

**Commit:** `feat(web): AutomationEditor shell + route wiring (plan 10)`

---

### Task 10.8 — Web: `TriggerSection`

**File:** `web/src/pages/automations/sections/TriggerSection.tsx`

Props: `{ trigger: TriggerDraft | null, onChange }`. Type-select `<select>` at the top with five options. Below the select, the appropriate form renders for the selected type (decision 4). Use `TimePicker`, `EntityPicker` from `editors/`. The "Multiple triggers" read-only banner shows when `trigger` is null and the loaded AST had `triggers.length > 1` (pass `multipleTriggersDetected?: boolean` prop).

**TDD:** render Time trigger → asserts time input present; render Manual trigger → asserts read-only note; changing type-select to "Webhook" fires `onChange` with `{ type: "Webhook" }`; `multipleTriggersDetected` banner renders when true.

**Commit:** `feat(web): TriggerSection with all five trigger types (plan 10)`

---

### Task 10.9 — Web: `WhenSection` + `ConditionBuilder`

**Files:** `web/src/pages/automations/editors/ConditionBuilder.tsx`, `web/src/pages/automations/sections/WhenSection.tsx`

`ConditionBuilder` is recursive. It handles leaf nodes (`StateCondition`, `NumericCondition`, `TimeCondition`) with appropriate inline editors, group nodes (`AndCondition`, `OrCondition`, `NotCondition`) by rendering a labelled group box with recursive children, and `StarlarkCondition` by rendering the locked preview. Each non-Starlark leaf and group has an "× Remove" button. `WhenSection` renders an "empty state" when `conditions` is empty, maps over the list calling `ConditionBuilder`, and has a "+ Add condition" button that appends a default `StateCondition`.

**TDD:** empty state text; `StateCondition` leaf renders entity name; Starlark leaf renders "View in Pkl editor →" link; clicking "× Remove" on first of two conditions fires `onChange` with one-element array.

**Commit:** `feat(web): WhenSection + ConditionBuilder (plan 10)`

---

### Task 10.10 — Web: `DoSection` + action components

**Files:** `web/src/pages/automations/sections/DoSection.tsx`, `web/src/pages/automations/actions/*.tsx`, `web/src/pages/automations/LockedFieldBanner.tsx`

`DoSection` maps the `actions` array to `ActionCard` components. Each `ActionCard` dispatches to the right action component by `_type` + `capability` combination (decision 6). Above the list, a "+ Add action" button appends a default `CallServiceAction`. Each non-locked card has a remove button.

`LockedFieldBanner` renders a one-line note with `background: var(--sy-color-surface-2); border-left: 3px solid var(--sy-color-warn); padding: var(--sy-space-2) var(--sy-space-3)`.

Each action component uses only `--sy-*` tokens — `switchyard/no-raw-tokens` must pass.

**TDD:** `TurnOnAction` renders entity picker; `SetBrightnessAction` renders slider at correct value; `StarlarkActionLocked` renders "starlark" chip and "View in Pkl editor →"; `DoSection` with empty actions shows "No actions defined yet."

**Commit:** `feat(web): DoSection + all action types + LockedFieldBanner (plan 10)`

---

### Task 10.11 — Web: `OnFailureSection`

**File:** `web/src/pages/automations/sections/OnFailureSection.tsx`

Props: `{ onFailure: OnFailureDraft, onChange }`. Strategy selector: three options. Conditional fields for Retry (maxAttempts, backoff seconds) and Notify (entity text, message text). Default `ignore` renders no extra fields. All values flow up via `onChange`.

**TDD:** default "Do nothing" strategy — no extra fields rendered; selecting "Retry" reveals maxAttempts and backoff inputs; changing maxAttempts fires `onChange` with updated `maxAttempts`.

**Commit:** `feat(web): OnFailureSection (plan 10)`

---

### Task 10.12 — Web: inline editors

**Files:** `web/src/pages/automations/editors/{EntityPicker,ScenePicker,BrightnessSlider,TimePicker}.tsx`

`EntityPicker` queries `ConfigService.GetArtifact` for the entity list; renders a `<select multiple>` (when `multi=true`) or single `<select>`. `ScenePicker` filters the entity list to `id.startsWith("scene.")`. `BrightnessSlider` is a controlled `<input type="range" min=0 max=100>` with a `{value}%` label. `TimePicker` is a labelled `<input type="time">`. All four use only `--sy-*` tokens.

No custom TDD required beyond what section tests exercise; each editor is simple enough to be covered by its parent component's test.

**Commit:** `feat(web): inline editors — EntityPicker, ScenePicker, BrightnessSlider, TimePicker (plan 10)`

---

### Task 10.13 — Web: `PklSourcePane`

**Files:** `web/src/pages/automations/PklSourcePane.tsx` + test

Props: `{ source: string, isDirty: boolean }`. Parse `source` by splitting on `\n`. Track a `inLocked` flag: set on lines containing `// locked-region`, cleared after `// end-locked-region`. Lines inside the locked region get the grey tint class; dirty (non-locked) lines get the orange tint class when `isDirty` is true. Line numbers in a fixed-width left column. Two tabs in a `role="tablist"`: "Source" (default) and "Diff vs disk (N)". The Diff tab renders only the dirty lines with `+` prefix on green-tinted rows.

**TDD:** all source lines render; line with `StarlarkCondition` inside locked markers gets `pkl-source-pane__line--locked` class; when `isDirty=true` a non-locked line gets `pkl-source-pane__line--dirty`; Diff tab button label shows count.

**Commit:** `feat(web): PklSourcePane with dirty/locked tinting + diff tab (plan 10)`

---

### Task 10.14 — Web: `ConflictBanner`

**File:** `web/src/pages/automations/ConflictBanner.tsx`

Props: `{ conflict: { conflictAt: string, newFileHash: string }, filePath: string, lockToken: string, pklBytes: Uint8Array, onReload: () => void }`. Three affordances per §17.3: (1) "Discard mine, reload from file" button — calls `AbandonEdit`, then `onReload()`; (2) "Overwrite file with my changes" button — re-calls `CommitEdit` with `fileHash: conflict.newFileHash` (the on-disk hash); (3) "Open 3-way merge" link — navigates to `/_authed/pkl-editor?file=<filePath>&merge=true`. The banner message: "External edit detected. `<filePath>` was modified at `<conflictAt>`. Choose how to reconcile."

**TDD:** all three affordances render; "Discard mine" click fires `AbandonEdit`; "Open 3-way merge" link has the correct `href`.

**Commit:** `feat(web): ConflictBanner with three reconciliation options (plan 10)`

---

### Task 10.15 — Sample automation + Playwright snapshot

**Files:** `examples/automations/sunset-lights.pkl`, `web/e2e/automation-editor-snapshot.spec.ts`

`sunset-lights.pkl` reproduces the mockup: `EventTrigger { kind = "sun.sunset" }`, `StateCondition { entity = "light.living_room_ceiling"; not = "on" }`, two actions (`CallServiceAction set_brightness level=40`, `CallServiceAction capability=notify`), `onFailure = new IgnoreStrategy {}`.

Playwright test: stub `AutomationService.GetDetail` and `ConfigService.OpenForEdit` / `RegenPreview` with `page.route`. Navigate to `/_authed/automations/sunset-lights`. Assert: four section card headings visible; `PklSourcePane` visible. Simulate a dirty edit (change trigger type to "Time"). Assert orange tint class is present on a dirty line. Take screenshot in `friendly-light` and `friendly-dark` themes. Commit reference images under `web/e2e/__screenshots__/automation-editor-snapshot/`.

**Acceptance:** `task web:e2e` passes; screenshots stable across re-runs.

**Commit:** `test(web): automation editor Playwright snapshot + sunset-lights sample (plan 10)`

---

## Test plan

- `go test ./internal/automation/regen/... -v` — five golden tests pass.
- `go test ./internal/api/...` — `GetDetail` + `RegenPreview` handler tests pass.
- `go build ./...` — zero errors.
- `npm run test --run` (from `web/`) — all new component, hook, and editor tests pass; `switchyard/no-raw-tokens` lint is green across all new files.
- `task web:build` — strict TypeScript compiles; no `PlaceholderPage` remaining at either automation route.
- `task web:e2e` — Playwright snapshots match reference images.
- Manual smoke: `task ui:dev` → navigate to `/automations` → click "sunset-lights" → verify four section cards, source pane updates on a field edit, orange dirty tint appears.

## Acceptance criteria for merging

- Plan 11 merged first; `OpenForEdit` / `CommitEdit` / `AbandonEdit` RPCs are live (not stubs).
- All tests + typecheck + lint green locally and in CI.
- Both automation routes replace their Plan 01 placeholders and compile under the strict TS config.
- `PklSourcePane` shows orange dirty lines and grey locked-region lines in the correct places.
- `ConflictBanner` appears when `CommitEdit` returns `conflict = true`; all three reconciliation paths are reachable.
- "Run now" navigates to `/_authed/time-machine/<run_id>` (Plan 04's route).
- `examples/automations/sunset-lights.pkl` is valid Pkl (`pkl eval examples/automations/sunset-lights.pkl` exits 0).
- Regen golden tests and Playwright snapshots are committed.
- No `--sy-*` token violations; no raw hex or spacing values in any new file.
- Linear parent issue + sub-tasks moved to `Done`; branch merged via `git merge --no-ff` into main.
