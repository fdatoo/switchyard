# In-browser Pkl + Starlark editors — design

**Status:** Draft
**Date:** 2026-05-12
**Branch:** `feat/new-ui`

## Problem

Switchyard's UI currently has empty `Settings → Pkl config` and
`Settings → Starlark` stubs, plus `Open Pkl config` CTAs in several
empty states. The daemon already exposes everything needed for a
full editor experience:

- `EditSessionService` — transactional single-file edits with
  `ListFiles`, `OpenForEdit` (returns raw text + AST), `CommitEdit`
  (hash-checked write), `SessionEvents` stream, `AbandonEdit`.
- `ConfigService` — `Validate`, `Apply`, `Reload`, `GetArtifact`,
  `RegenPreview` (AST → Pkl text).
- `ScriptService` — `List`, `Run`, `Eval`, `RunTests` (streaming).
- `StarlarkLSService` — Tokenize / Complete / Hover / LookupSymbol.

The gap is the UI surface, plus the daemon's regenerator currently
only supports `file_type = "automation" | "page"`. Scenes, areas,
and `entityAreas` overrides need their own renderers before
structured editing for those types is possible.

## Goal

Three iterations, each independently shippable:

**Iteration 1 — Regenerator coverage.** Extend `regen` with
`RenderScene`, `RenderArea`, `RenderEntityAreas`, and wire each
into `ConfigService.RegenPreview` (new `file_type` values). No UI
changes. Round-trip tested via Go unit tests.

**Iteration 2 — Text editor surface.** Monaco-based editors at
`/settings/pkl` and `/settings/starlark`. File tree + editor +
status bar. Save via `EditSessionService.CommitEdit` (raw text;
no AST round-trip in this iteration). Starlark editor gets a
test runner panel backed by `ScriptService.RunTests`. Replaces
the existing Pkl SettingsStub.

**Iteration 3 — Structured "+ New automation" form.** First
AST-driven flow. A `SyAutomationForm` modal accessible from
`/automations`' "+ New" button + the existing "Open Pkl config"
CTAs. Form builds an `AutomationConfig` proto in the client,
calls `RegenPreview({file_type: "automation"})` to get the Pkl
text, then writes the file via `EditSessionService.OpenForEdit` +
`CommitEdit`. The text editor still exists as the escape hatch.

Iteration 3 is one structured flow. Subsequent specs cover the
other types (Scene, Area, EntityAreas, Page) using the patterns
that 3 establishes.

In scope across all three:

- Regenerator coverage for Scene / Area / EntityAreas.
- `RegenPreview` dispatch for the new `file_type` values.
- Monaco editor wrapped as a focused `SyCodeEditor` component.
- Vite Monaco worker setup (`vite-plugin-monaco-editor`).
- Pkl Monarch grammar (~50 lines, hand-written).
- Starlark uses Monaco's built-in `python` highlighting.
- `EditSessionService`, `ConfigService`, `ScriptService` TS clients.
- File tree (`SyFileTree`), code-editor panel (`SyCodeEditorPanel`).
- Routes: `/settings/pkl` (replaces stub), `/settings/starlark` (new).
- Bottom panel: validation diagnostics (Pkl) / test runner (Starlark).
- `SyAutomationForm` modal with trigger / condition / action builders.
- Wire "+ New automation" CTA on `/automations` to the form.

Out of scope (each is its own follow-up):

- **StarlarkLSService integration.** Wiring `Tokenize` / `Complete`
  / `Hover` into Monaco's provider interfaces is a focused 1-2 day
  effort. The proto is wired, our text editor works without it.
- **Structured flows for non-automation types** (Scene, Area,
  EntityAreas, Page forms). Iteration 1 makes them possible at
  the daemon level; the UI side is its own spec — same pattern as
  iteration 3 but per-type form components and validation rules
  vary enough that each deserves its own design pass.
- **3-way merge UI** when `CommitEdit` reports `CommitConflict`.
  v1 surfaces the conflict and prompts to reload.
- **Validation while typing.** `ConfigService.Validate` takes a
  whole bundle; too expensive per keystroke. Validation runs on
  Save.
- **Diff view** between editor buffer and disk.
- **`AnalyzeRegenerability` surfacing.** Will matter once we have
  AST-aware edit flows for files containing format-only regions;
  not blocking for iteration 3 since fresh-form-emitted Pkl is
  always fully regenerable.
- **Search / find-in-files.**
- **Editor theming / font-size preferences.** Monaco defaults.
- **Multi-file unsaved-change navigation guards.** v1 uses native
  `confirm(...)` on route change with a dirty buffer.

## Architecture

### Page layout (iterations 2 and 3)

```
SettingsLayout
   └─ <route>  /settings/pkl  or  /settings/starlark
      └─ <SyCodeEditorPanel kind="pkl" | "starlark">
            ┌─────────────────────────────────────────────────────┐
            │ Status bar                                          │
            │   <filename>     [● dirty]   [Save]  [Discard]      │
            ├──────────────┬──────────────────────────────────────┤
            │ File tree    │ Monaco editor                        │
            │   • main.pkl │   (language: pkl / python)           │
            │   • secrets… │                                      │
            │   • handlers/│                                      │
            │     • ride.s │                                      │
            ├──────────────┴──────────────────────────────────────┤
            │ Bottom panel (optional)                             │
            │   Pkl:      validation diagnostics                  │
            │   Starlark: test runner (RunTests stream)           │
            └─────────────────────────────────────────────────────┘
```

Iteration 3 adds a "+ New" button to the existing
`/automations` page that opens `SyAutomationForm` as a modal
(over the page; not a separate route).

### Iteration 1 — Regenerator coverage

The existing `internal/automation/regen/regen.go` exports
`Render(*AutomationConfig) ([]byte, error)` and the pattern is
straightforward — a `pklWriter` collects lines, each top-level
proto field gets a `renderX` helper. Adding three renderers
mirrors this:

```go
func RenderScene(s *configpb.SceneConfig) ([]byte, error)
func RenderArea(a *configpb.AreaConfig) ([]byte, error)
func RenderEntityAreas(m map[string]string) ([]byte, error)
```

`RegenPreview` (in `internal/api/service_config.go`) currently
dispatches on `file_type = "automation" | "page"`. Three new
branches:

```go
case "scene":        return regen.RenderScene(parseScene(req.AstJson))
case "area":         return regen.RenderArea(parseArea(req.AstJson))
case "entity_areas": return regen.RenderEntityAreas(parseEntityAreas(req.AstJson))
```

`parseScene` / `parseArea` / `parseEntityAreas` unmarshal the
JSON the client sent. We keep the request `ast_json` field — same
contract as the existing flow.

Tests in `regen_test.go` round-trip each renderer (Render → re-parse
via Pkl evaluator → assert structural equality with the input).

This iteration has zero UI changes; it lands as standalone Go work.

### Iteration 2 — Text editor surface

#### Data flow

```
SyCodeEditorPanel mount
  ├─ EditSessionService.ListFiles({kind: "pkl" | "starlark"})
  │     ↓
  │   file tree renders                        (left rail)
  │
  └─ user selects a file
        ↓
       EditSessionService.OpenForEdit({file_path})
        ↓
       editor loads ancestor_pkl into Monaco; store session_id +
       lock_token + file_hash
        ↓
       user types — Monaco dirties — status bar shows ●
        ↓
       Save:
         EditSessionService.CommitEdit({
           file_path, lock_token,
           regenerated_pkl: <editor value>,
           expected_file_hash: <stored hash>,
         })
        ↓
       success → reset dirty, update stored hash, Activity surfaces
                 config.applied event from the daemon's file
                 watcher (already wired).
       conflict → banner "this file changed on disk; reload to
                  reconcile", Reload button re-opens the file.

  parallel: EditSessionService.SessionEvents stream
        ↓
       ExternalEditDetected → same banner.
```

Starlark editor's bottom panel:

```
user clicks "Run tests":
  ScriptService.RunTests({script_id}) [server-stream]
        ↓
  each RunTestsResponse renders as one row
  on TestsDone the runner returns to idle
```

#### Files (iteration 2)

| Path | Action | Responsibility |
|---|---|---|
| `app/package.json` | modify | Add `monaco-editor` + `vite-plugin-monaco-editor` deps |
| `app/vite.config.ts` | modify | Wire the Monaco plugin so workers chunk separately |
| `app/src/lib/components/code-editor/SyCodeEditor.vue` | new | Thin Monaco wrapper |
| `app/src/lib/components/code-editor/pkl-grammar.ts` | new | Monarch grammar for Pkl |
| `app/src/lib/components/file-tree/SyFileTree.vue` | new | Two-level file tree |
| `app/src/lib/components/code-editor-panel/SyCodeEditorPanel.vue` | new | Tree + editor + status bar composition |
| `app/src/lib/components/code-editor-panel/SyTestPanel.vue` | new | Streaming test runner (Starlark only) |
| `app/src/data/edit-session.ts` | new | EditSessionService TS client |
| `app/src/data/config-service.ts` | new | ConfigService TS client (Reload) |
| `app/src/data/script-service.ts` | new | ScriptService TS client (RunTests streaming) |
| `app/src/views/settings/sections/PklEditorSection.vue` | new | `/settings/pkl` route |
| `app/src/views/settings/sections/StarlarkEditorSection.vue` | new | `/settings/starlark` route |
| `app/src/lib/index.ts` | modify | Export new components |
| `app/src/router/index.ts` | modify | Replace `SettingsStub` for /settings/pkl; add /settings/starlark |
| `app/src/views/AppLayout.vue` | modify | Add /settings/starlark to the palette catalog |

### Iteration 3 — `SyAutomationForm`

A modal that builds an `AutomationConfig` proto in the client
through a series of typed inputs:

```
SyAutomationForm
   ├─ Header
   │   id          (text input)
   │   displayName (text input)
   │   areas       (tag picker — one entry per existing area)
   │
   ├─ Triggers
   │   [ + Add trigger ]
   │   per-trigger row:
   │     kind: dropdown (state_changed | time | event | webhook)
   │     trigger-specific fields appear conditionally
   │
   ├─ Conditions (optional)
   │   [ + Add condition ]
   │   per-row: state | numeric | time | starlark | and | or | not
   │
   ├─ Actions
   │   [ + Add action ]
   │   per-action row:
   │     kind: dropdown (call_service | call_script | etc.)
   │     action-specific fields
   │
   ├─ Footer
   │   [ Cancel ]      [ Save ]
   │
```

On Save:

1. Build `AutomationConfig` proto from the form state.
2. `RegenPreview({file_type: "automation", ast_json: <serialised>})`
   → daemon returns canonical Pkl bytes.
3. Decide target file. v1 always writes to `automations/<id>.pkl`
   (one automation per file, created if absent). This is the
   simplest unambiguous strategy and matches the existing
   `examples/automations/*.pkl` convention.
4. `EditSessionService.OpenForEdit({file_path})` — empty file
   when new; returns ancestor + lock.
5. `EditSessionService.CommitEdit({regenerated_pkl: <regen output>, ...})`.
6. Close modal; the global `/automations` list refreshes via its
   own poll/store.

Editing an existing automation reuses the same form, pre-populated
by reading the automation's `AutomationConfig` from the snapshot
(via the existing `ConfigService.GetArtifact`).

The form lives in `app/src/views/automations/SyAutomationForm.vue`
+ a few sub-components for trigger / condition / action editors
(each is a small `<component>` selected by kind).

#### Files (iteration 3)

| Path | Action | Responsibility |
|---|---|---|
| `app/src/views/automations/SyAutomationForm.vue` | new | Modal hosting the form; emits save/cancel |
| `app/src/views/automations/TriggerEditor.vue` | new | Per-trigger row; dynamic sub-form by kind |
| `app/src/views/automations/ConditionEditor.vue` | new | Per-condition row, recursive for and/or/not |
| `app/src/views/automations/ActionEditor.vue` | new | Per-action row |
| `app/src/data/regen-preview.ts` | new | `regenPreview({fileType, ast})` TS client |
| `app/src/views/AutomationsView.vue` | modify | "+ New" button opens `SyAutomationForm`; edit affordance on each row |

#### Per-input shape

Trigger types and fields are defined by the `TriggerConfig` proto
(`StateChangeTrigger`, `EventTrigger`, `TimeTrigger`,
`WebhookTrigger`). The form's dropdown switches between sub-forms
matching each. Same pattern for conditions and actions.

Trigger sub-forms:

- **State changed:** entity (picker over `entityStore`), `from`
  (optional), `to` (optional), `hold` (duration, optional).
- **Time:** cron string (text input) + a "validate" affordance
  via `EvalCompute` later — for v1, accept as-is.
- **Event:** kind (text input), filter (key/value rows).
- **Webhook:** path (text input).

Action sub-forms:

- **Call service:** entity (picker), capability (text input —
  could be a per-entity dropdown but capability discovery is a
  separate problem), args (key/value rows).
- Other action types deferred to follow-up specs — `call_service`
  is the dominant case.

Condition sub-forms (parity with action coverage in iteration 3):

- **State:** entity, equals / not, value.
- **Numeric:** entity, operator (<, ≤, =, ≥, >), value.
- **Time:** between (cron-like), or "weekday" / "weekend".
- **Starlark:** opens a small inline Monaco for an expression.
  Borrows `SyCodeEditor`.
- **and / or / not:** nested condition lists.

### Daemon changes summary

- **Iteration 1:** `internal/automation/regen/regen.go` gains
  three new renderers; `internal/api/service_config.go`
  `RegenPreview` handler gains three new dispatch cases. Tests in
  `regen_test.go` round-trip each.
- **Iterations 2 + 3:** zero daemon changes — every needed RPC
  exists and is implemented.

## Bundle / build

Monaco is ~2MB minified. `vite-plugin-monaco-editor` chunks it
into separate bundles loaded only when the editor route is
visited. Initial `/`, `/rooms`, etc. bundle stays unaffected.

The Pkl Monarch grammar is hand-written and small (one .ts file,
~50 lines). It covers keywords, primitives, strings, comments,
brackets. Starlark uses Monaco's built-in `python`.

## Errors and edge cases

- **CommitEdit conflict**: banner + Reload action. User's editor
  buffer is preserved until they explicitly Reload (which
  re-opens with the new on-disk content).
- **SessionEvents disconnect**: silent reconnect with exponential
  backoff (mirrors the entity store pattern we built earlier).
- **OpenForEdit NotFound**: refresh the file tree.
- **RegenPreview returns an error**: surface as inline form
  validation error; don't close the modal.
- **Form-emitted file overwrites an unrelated existing file**:
  guarded by the `file_path` choice (`automations/<id>.pkl` —
  same `id` writes the same file deterministically; user is
  warned in the form that saving with an existing id overwrites
  that automation).
- **Unsaved changes on route change**: native `confirm("Discard
  unsaved changes?")` for v1.
- **Empty file tree**: render a `SyEmptyState` "No Pkl/Starlark
  files yet" — no Create affordance in v1 (a real install has
  at least `main.pkl`).

## Testing

### Go

- **Iteration 1:** new tests in
  `internal/automation/regen/regen_test.go`:
  - `TestRenderScene_Roundtrip` — build a SceneConfig, Render,
    re-parse via the evaluator, assert structural equality.
  - `TestRenderArea_Roundtrip` — same shape for area.
  - `TestRenderEntityAreas_Roundtrip` — map round-trip.
  - `TestRegenPreview_DispatchesByFileType` — service-level test
    covering each new `file_type` branch.
- Iterations 2 + 3: no Go changes, so no new Go tests; standing
  `go test ./...` still green.

### TS

`vue-tsc -b --noEmit` clean across all new files. No vitest infra
in `app/`; verification is typecheck + manual + Playwright.

### Manual / Playwright

**Iteration 2:**
1. `/settings/pkl` — tree shows current files; clicking
   `main.pkl` loads it into Monaco; edit a comment and Save
   succeeds; broken edit (mismatched braces) gets rejected by
   the daemon and surfaces as a toast.
2. External edit: `echo >> ~/.local/share/switchyard/config/main.pkl`
   from a separate shell triggers the ExternalEditDetected banner.
3. `/settings/starlark` — tree shows .star files; "Run tests" on
   a handler streams results into the bottom panel.

**Iteration 3:**
1. `/automations` "+ New". Form opens.
2. Fill: id=test-iter3, displayName=Test, add one state_changed
   trigger on a real entity, one call_service action turning that
   entity on.
3. Save. The form closes; the new automation appears in the list;
   `/automations/<id>.pkl` exists with the generated Pkl;
   AutomationService re-evaluates and includes it.
4. Click Edit on it. Form re-opens pre-populated.
5. Modify, Save. Verify the file updates.

Take screenshots: `pkl-editor-loaded.png`, `pkl-editor-dirty.png`,
`starlark-editor-tests.png`, `automation-form-new.png`,
`automation-form-edit.png`.

## Open questions to acknowledge

- **Per-entity capability discovery for the action sub-form.**
  Today, capability is a free-text field. Driver capabilities
  live on `entity.capabilities` — the Hue driver, for instance,
  populates `Light.brightness/colorTemp/colorRgb` only when those
  exist. The form could provide a dropdown derived from that.
  Deferred until "capabilities populated server-side for hue" lands
  (an out-of-scope item from earlier polish list).
- **Cron validation in the time trigger.** Free-text v1; integrate
  `EvalCompute` or a cron parser in a follow-up.
- **What about migrating existing automations declared inline in
  `main.pkl`?** Iteration 3's flow always writes to
  `automations/<id>.pkl`. If the user has automations declared
  inline in `main.pkl` from past edits, the form-based edit flow
  would extract them into separate files. The migration helper
  (or the choice to leave them alone) is out of scope for v1 —
  the form opens any automation by id, regardless of which file
  it lives in, by reading from `ConfigService.GetArtifact`. Save
  writes to `automations/<id>.pkl`, leaving any inline declaration
  behind. The user resolves the duplicate manually. Awkward but
  honest; cleaner extraction is a follow-up.
