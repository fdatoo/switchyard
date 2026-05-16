# Plan 06 — Custom Pages + Three-Tier Widget Contract

> **Greenfield plan.** Replaces the C10 dashboard subsystem end-to-end. Zero deployed users means no migration path is needed. C10 code is deleted, not wrapped.

**Goal:** Ship Custom Pages — a stack-of-sections page model with a three-tier widget contract (Section, Tile, Cell). Deliver the full render path, edit-mode chrome with a 320px settings rail and live Pkl preview, a `PageService` replacing `DashboardService`, and nine built-in Section types, four Tile types, and two Cell types. Delete `react-grid-layout`, `web/src/dashboard/`, and `web/src/widgets/`.

**Spec refs:** §12 (Custom Pages, three-tier contract), §21 (greenfield — what we throw away from C10).

**Mockups:** `.superpowers/brainstorm/71337-1778492716/screenshots/08-custom-pages-01.png` (render mode), `08-custom-pages-02.png` (edit mode + settings rail), `08-custom-pages-03.png` (three-tier legend).

**Branch:** `feat/ui-v2-plan-06-custom-pages`
**Worktree:** `.claude/worktrees/plan-06-custom-pages`
**Depends on:** Plan 01 merged to main
**Linear parent:** TBD

---

## Decisions (locked — no ambiguity for the implementer)

1. **C10 dashboard subsystem is greenfield-replaced, not migrated.** `web/src/dashboard/render/Grid.tsx` is deleted. The `react-grid-layout` dependency (and its types) is removed from `web/package.json`. The C10 single-tier `WidgetInstance` shape (with `pos: Position`, `grid: Grid`, flat widget list) is gone; nothing carries it forward.

2. **`DashboardService` → `PageService`.** The new proto lives at `proto/switchyard/page/v1/page.proto`. Old path `proto/switchyard/v1alpha1/dashboard.proto` is deleted after `buf generate`. No backward-compat aliases: zero users, zero need.

3. **Three-tier widget contract (§12.2).** Pack manifests declare a `tiers` field: one or more of `section | tile | cell`. The tiers are mutually exclusive at render time:
   - **Section** — top-level, full content-width, owns its own internal layout.
   - **Tile** — lives inside tile-hosting Section types (`RoomGrid`, `StatGrid`); square-ish, action-oriented.
   - **Cell** — lives inside list-hosting Section types (`EntityList`, `ActivityFeed`); row-shaped, dense.

4. **Built-in Section types:** `Hero`, `Chart`, `EntityList`, `ActivityFeed`, `RoomGrid`, `Markdown`, `CameraGrid`, `StatGrid`, `WebhookButton`. Floorplan is deferred (§24).

5. **Built-in Tile types:** `RoomTile`, `StatTile`, `EntityToggle`, `SceneButton`.

6. **Built-in Cell types:** `EntityRow`, `EventRow`.

7. **Two-file Pkl split persists.** `pages/<slug>.pkl` is user-owned (title, metadata, references). `pages/<slug>.layout.pkl` is regenerator-owned (canonical section/tile/cell tree, written deterministically). The existing regen pipeline at `internal/dashboard/regen/` is renamed to `internal/page/regen/` — package name changes, logic adapts to the new model, tests are updated in place.

8. **Edit mode matches `08-custom-pages-02.png` exactly.** In edit mode the content area splits into `1fr 320px`. The left column shows the section stack with drag handles, gear, and `×` per section, plus `+ Add section` affordances between every section. The right rail (`.settings-rail`) shows the selected section's form fields and a live Pkl preview block at the bottom. Both surfaces ship in this plan.

9. **Save flow:** clicking "Save & exit" calls `PageService.SaveLayout(slug, sections[])`, which validates the section tree, runs the regen pipeline to produce deterministic Pkl, writes `pages/<slug>.layout.pkl`, and emits `ConfigApplied`. Conflict resolution is Plan 11's job.

10. **Widget pack distribution survives unchanged** (OCI + cosign + `/widgets/<pack>/<v>/<file>`). The manifest format gains a `tiers: string[]` field; existing packs without it are treated as `["section"]`.

11. **`WidgetPackService` proto is untouched.** Only `dashboard.proto` is replaced; `widget_pack.proto` merely has its `dashboard.proto` import swapped to `page.proto`.

---

## File plan

### Proto

```
proto/switchyard/page/v1/
  page.proto          ← new; supersedes dashboard.proto
DELETE proto/switchyard/v1alpha1/dashboard.proto
```

`widget_pack.proto` import updated from `dashboard.proto` → `page/v1/page.proto`.

### Server (Go)

```
internal/page/
  service.go          ← PageService Connect handler (List / Get / Create / Delete / SaveLayout / GetWidgetCatalog)
  catalog.go          ← built-in tier registry + installed-pack lookup
  scaffold.go         ← creates pages/<slug>.pkl + pages/<slug>.layout.pkl stubs
  regen/
    regen.go          ← renamed + rewritten from internal/dashboard/regen/
    regen_test.go
    testdata/          ← updated golden fixtures
DELETE internal/dashboard/ (entire directory, after migration)
```

### Web

```
web/src/pages-system/
  model.ts                    ← Section / Tile / Cell TypeScript types; PageModel
  registry.ts                 ← registers all built-in tiers; resolveSection / resolveTile / resolveCell
  render/
    Page.tsx                  ← renders a PageModel as a stack of SectionFrame
    SectionFrame.tsx          ← surface wrapper + section heading bar
    TileHost.tsx              ← grid host for tile-bearing sections
    CellHost.tsx              ← list host for cell-bearing sections
  edit/
    EditChrome.tsx            ← top-bar "editing" state, Discard / Save & exit buttons
    SettingsRail.tsx          ← 320px right rail; form + live Pkl preview
    AddSectionAffordance.tsx  ← "+ Add section" slot between sections
    use-page-editor.ts        ← Zustand slice for edit state (selectedSectionId, dirty, sections[])
  widgets/
    sections/
      Hero.tsx
      Chart.tsx
      EntityList.tsx
      ActivityFeed.tsx
      RoomGrid.tsx
      Markdown.tsx
      CameraGrid.tsx
      StatGrid.tsx
      WebhookButton.tsx
    tiles/
      RoomTile.tsx
      StatTile.tsx
      EntityToggle.tsx
      SceneButton.tsx
    cells/
      EntityRow.tsx
      EventRow.tsx

web/src/routes/_authed/pages/$slug.tsx   ← replaces Plan 01 placeholder; wires PageService.Get
DELETE web/src/dashboard/               ← entire directory
DELETE web/src/widgets/                 ← entire directory (C10 single-tier widgets)
web/package.json                        ← remove react-grid-layout + @types/react-grid-layout
```

### Config / Pkl

```
internal/config/pkl/switchyard/
  pages.pkl           ← new module (replaces dashboards.pkl); Section / Tile / Cell / LayoutFile
DELETE internal/config/pkl/switchyard/dashboards.pkl
internal/dashboard/pklfs/testdata/ → internal/page/pklfs/testdata/  (moved with package rename)
```

### Sample data

```
examples/pages/energy-climate.pkl         ← user-owned; title + section metadata
examples/pages/energy-climate.layout.pkl  ← regenerator-owned; canonical section tree
```

---

## Task 6.1 — Define `page.proto`

**File:** `proto/switchyard/page/v1/page.proto`

Replace the C10 flat `WidgetInstance` model with the three-tier model. `PageService` RPCs: `List`, `Get`, `GetWidgetCatalog`, `Create`, `Delete`, `SaveLayout` (same surface as `DashboardService`, renamed). Key messages:

- `Page` — `slug`, `title`, `repeated Section sections`, `source_pkl`, `layout_pkl`, `writable`.
- `enum Tier` — `TIER_UNSPECIFIED=0`, `TIER_SECTION=1`, `TIER_TILE=2`, `TIER_CELL=3`.
- `Section` — `id`, `type` (e.g. `"Hero"`, `"@acme/MySection"`), `google.protobuf.Struct props`, `repeated Tile tiles`, `repeated Cell cells`.
- `Tile`, `Cell` — each has `id`, `type`, `props`.
- `WidgetClass` — `class_id`, `repeated Tier tiers`, `is_builtin`, `pack_name`, `pack_version`, `bundle_url`, `bundle_hash`, `SignatureStatus signature`.

Delete `proto/switchyard/v1alpha1/dashboard.proto`. Update `widget_pack.proto` import.

**Acceptance:** `buf lint` passes. `buf generate` produces Go + Connect-ES stubs under `gen/`.

**Commit:** `feat(proto): page/v1 three-tier model, delete v1alpha1 dashboard proto`

---

## Task 6.2 — `buf generate`

Run `buf generate`. Verify generated files land in `gen/switchyard/page/v1/`. Delete stale `gen/switchyard/v1alpha1/dashboard*` outputs. Update every Go import that referenced the old dashboard proto package.

**Acceptance:** `go build ./...` green. No reference to `switchyard.v1alpha1.Dashboard` remains.

**Commit:** `chore(gen): regenerate protos after page/v1 rename`

---

## Task 6.3 — Rename `internal/dashboard/regen` → `internal/page/regen`

Move the regen package; update `package regen` and all Go imports. Adapt `regen.Render` to accept `*page.PageData` (with `[]SectionData` replacing `[]WidgetData`). Each `SectionData` carries `ID`, `Type`, `Props map[string]any`, `Tiles []TileData`, `Cells []CellData`.

Pkl output changes from a flat widget listing to a section listing (`sections: Listing<p.Section>`). Section Pkl blocks use typed class names: `new p.HeroSection { ... }`, `new p.ChartSection { ... }`, etc. Props remain sorted alphabetically; output stays deterministic.

Update `regen_test.go` golden fixtures. Delete `internal/dashboard/regen/`.

**Acceptance:** `go test ./internal/page/regen/...` green. `internal/dashboard/regen/` does not exist.

**Commit:** `refactor(server): rename dashboard/regen → page/regen, adapt to section model`

---

## Task 6.4 — Implement `PageService.Get` / `SaveLayout`

**Files:** `internal/page/service.go`, `internal/page/scaffold.go`

`Get` reads `pages/<slug>.pkl` (user-owned) + `pages/<slug>.layout.pkl` (regen-owned), evaluates both through the existing Pkl evaluator, and returns a `Page` proto.

`SaveLayout` receives the mutated `Page` proto from the browser, runs `regen.Render` on it to produce canonical Pkl, writes `pages/<slug>.layout.pkl` atomically (write-tmp → fsync → rename), then triggers `config.Manager.Reload` which emits `ConfigApplied`.

Wire `PageService` into the Connect listener in `internal/api/` (replacing the `DashboardService` mount). Delete `internal/dashboard/service.go` and `internal/dashboard/scaffold.go`.

**Acceptance:** `go test ./internal/page/...` green. Integration test: scaffold a page, `Get` it, mutate a section, `SaveLayout`, re-`Get` — section change is reflected.

**Commit:** `feat(server): PageService Get + SaveLayout`

---

## Task 6.5 — Catalog: built-in tier registry

**File:** `internal/page/catalog.go`

`GetWidgetCatalog` returns a `WidgetCatalog` listing all built-in Section, Tile, and Cell types (with `is_builtin = true`, `tiers` set appropriately) plus every installed pack's classes (tiers from manifest).

Built-in class IDs follow `@switchyard/builtin/<Type>` convention, e.g. `@switchyard/builtin/Hero`, `@switchyard/builtin/RoomTile`, `@switchyard/builtin/EntityRow`.

**Acceptance:** `go test ./internal/page/...` green; the returned catalog lists exactly the 15 built-in classes (9 sections + 4 tiles + 2 cells).

**Commit:** `feat(server): page widget catalog with built-in tier registry`

---

## Task 6.6 — `pages-system` model + registry + render skeleton

**Files:** `model.ts`, `registry.ts`, `render/{Page,SectionFrame,TileHost,CellHost}.tsx`

`model.ts` declares `SectionDef { id, type, props, tiles?, cells? }`, `TileDef`, `CellDef`, `PageModel`.

`registry.ts` exports `registerSection / registerTile / registerCell` and `resolveSection / resolveTile / resolveCell`, each falling back to an `Unknown*` stub that displays the type name.

`Page.tsx` renders `sections.map(s => <SectionFrame key={s.id} def={s} />)`. `SectionFrame.tsx` resolves from the registry, wraps in a `--sy-color-surface-1` card (`--sy-radius-lg` + `--sy-shadow`). `TileHost.tsx` — CSS grid `auto-fill minmax(160px, 1fr)`. `CellHost.tsx` — vertical list with `border-bottom: 1px solid var(--sy-color-line-soft)`.

**TDD:** `Page.test.tsx` — one Hero section, assert title visible. `SectionFrame.test.tsx` — unknown type renders stub with type name.

**Commit:** `feat(web): pages-system model, registry, render skeleton`

---

## Task 6.7 — Sections: `Hero`, `Chart`, `EntityList`

**Files:** `web/src/pages-system/widgets/sections/{Hero,Chart,EntityList}.tsx`

**`Hero`** — title, subtitle, 4-column stat grid (label / value / unit / delta). Delta colour: positive energy/CO₂ deltas → `--sy-color-bad`; negative → `--sy-color-good` (rising CO₂ is bad per mockup).

**`Chart`** — title, subtitle, window-range chips (1h / 6h / 24h / 7d), placeholder `<svg>` chart area at 180px height (uPlot wired in Plan 08), legend row. Props: `title`, `subtitle`, `series: {source, label, overlay?}[]`, `window`, `liveTail`, `showLegend`, `fillArea`.

**`EntityList`** — header with tag-filter chip + "Configure filter" ghost button; rows via `<CellHost>` with `EntityRow` cells. Props: `title`, `filter: {tag?, entityIds?}`, `cells`.

All three call `registerSection` as a module side-effect.

**TDD:** snapshot test each section with fixture props; assert `--sy-*` tokens only (no raw colour literals).

**Commit:** `feat(web): sections Hero, Chart, EntityList`

---

## Task 6.8 — Sections: `ActivityFeed`, `RoomGrid`, `Markdown`

**Files:** `web/src/pages-system/widgets/sections/{ActivityFeed,RoomGrid,Markdown}.tsx`

**`ActivityFeed`** renders a header with a time-window chip and an "Open in Activity" ghost button. Rows delegate to `<CellHost>` using `EventRow` cells. Props: `title`, `filter`, `window`, `maxRows`.

**`RoomGrid`** renders a header and delegates to `<TileHost>` with `RoomTile` children. Props: `title`, `roomSlugs: string[]`, `tiles: TileDef[]`.

**`Markdown`** renders sanitised markdown via `react-markdown` with `--sy-font-body` typography. Props: `content`.

**TDD:** `Markdown.test.tsx` — render with `content="# Hello"` → assert an `<h1>` is present.

**Commit:** `feat(web): sections ActivityFeed, RoomGrid, Markdown`

---

## Task 6.9 — Sections: `CameraGrid`, `StatGrid`, `WebhookButton`

**Files:** `web/src/pages-system/widgets/sections/{CameraGrid,StatGrid,WebhookButton}.tsx`

**`CameraGrid`** renders a grid of `<img>` placeholders (stream integration is Plan 08). Props: `title`, `cameras: {entityId, label}[]`, `columns`.

**`StatGrid`** renders a header and delegates to `<TileHost>` with `StatTile` children. Props: `title`, `tiles: TileDef[]`.

**`WebhookButton`** renders a single prominent button. On click it calls the `ScriptService.RunWebhook` RPC (or shows a confirmation modal if `confirm: true`). Props: `label`, `webhookId`, `confirm`, `confirmText`.

**TDD:** `WebhookButton.test.tsx` — click the button → assert `RunWebhook` was called with the configured webhook ID.

**Commit:** `feat(web): sections CameraGrid, StatGrid, WebhookButton`

---

## Task 6.10 — Tiles: `RoomTile`, `StatTile`, `EntityToggle`, `SceneButton`

**Files:** `web/src/pages-system/widgets/tiles/{RoomTile,StatTile,EntityToggle,SceneButton}.tsx`

Square-ish cards rendered inside `TileHost`; `--sy-*` tokens only; each calls `registerTile`.

- `RoomTile` — room name, icon placeholder, entity-count badge. Props: `roomSlug`, `label`, `entityCount`.
- `StatTile` — large value + unit + label. Props: `entityId`, `label`, `unit`, `precision`.
- `EntityToggle` — entity name + toggle switch; calls `EntityService.SetState` on change. Props: `entityId`, `label`.
- `SceneButton` — accent-filled button; calls `SceneService.Activate`. Props: `sceneId`, `label`.

**TDD:** `EntityToggle.test.tsx` — toggle → assert `SetState` mock called with correct entity ID.

**Commit:** `feat(web): tiles RoomTile, StatTile, EntityToggle, SceneButton`

---

## Task 6.11 — Cells: `EntityRow`, `EventRow`

**Files:** `web/src/pages-system/widgets/cells/{EntityRow,EventRow}.tsx`

Row-shaped, rendered inside `CellHost`; each calls `registerCell`.

- `EntityRow` — icon gradient, entity name, entity ID (monospace), current value + unit, sparkline placeholder. Props: `entityId`, `label`, `unit`.
- `EventRow` — severity dot (good/warn), who, what, relative timestamp. Props: `entityId`, `summary`, `severity`, `timestamp`.

**TDD:** `EntityRow.test.tsx` — fixture props → entity name and ID text present.

**Commit:** `feat(web): cells EntityRow, EventRow`

---

## Task 6.12 — Edit-mode chrome: handles, AddSection affordance, settings rail

**Files:** `web/src/pages-system/edit/{EditChrome,SettingsRail,AddSectionAffordance}.tsx`, `web/src/pages-system/edit/use-page-editor.ts`

`use-page-editor.ts` is a Zustand slice: `sections[]`, `selectedSectionId`, `dirty`, plus `selectSection`, `moveSection`, `deleteSection`, `addSection(after, type)`, `updateSectionProps`.

`EditChrome.tsx` wraps the page content area. When active, the layout switches to `grid-template-columns: 1fr 320px`; top bar gains an accent "· editing" label plus Discard and "Save & exit" buttons.

`SectionFrame.tsx` gains an edit-mode overlay: drag handle (⠿), gear (⚙), and delete (×) in the top-right corner, visible on hover or when selected. Selected state: `border: 2px solid var(--sy-color-accent)` + `box-shadow: 0 0 0 5px var(--sy-color-accent-soft)` + floating pill label (e.g. "CHART · SELECTED") above the top-left corner.

`AddSectionAffordance.tsx` renders a faint `+ Add section` pill between every section pair. On click it opens a section-type picker modal listing all registered Section types.

`SettingsRail.tsx` renders the selected section's hand-authored form fields (built-in sections) plus the `PklPreview` pane at the bottom (Task 6.13).

**TDD:** `use-page-editor.test.ts` — move / delete / add produce correct state. `EditChrome.test.tsx` — edit mode active + section selected → drag handle and rail are in the DOM.

**Commit:** `feat(web): edit-mode chrome — handles, AddSection, SettingsRail`

---

## Task 6.13 — Live Pkl preview pane

**File:** `web/src/pages-system/edit/SettingsRail.tsx` (extending Task 6.12)

The `PklPreview` sub-component inside `SettingsRail` calls the server-side `PageService.PreviewSectionPkl(sectionDef)` RPC — or, as a lighter alternative, runs the client-side mini-serialiser (a trimmed version of the regen logic ported to TypeScript) to produce the Pkl snippet inline without a round-trip.

**Decision (locked):** ship the **client-side TypeScript serialiser** in Plan 06. It renders the section's Pkl block using the same deterministic rules as `internal/page/regen`: props alphabetical, string literals quoted, boolean values unquoted. The output is shown in a monospace code block styled as `background: #1c1c24; color: #d4d2cb` (matching the mockup) with simple keyword/string highlighting.

**Acceptance:** edit the "Window" chip in the Chart section settings form — the Pkl preview updates without a network round-trip, changing `window = "24h"` to the newly selected value.

**Commit:** `feat(web): live Pkl preview in settings rail (client-side serialiser)`

---

## Task 6.14 — Sample page: `energy-climate`

**Files:** `examples/pages/energy-climate.pkl` (user-owned), `examples/pages/energy-climate.layout.pkl` (regen-owned)

`energy-climate.pkl` imports `energy-climate.layout.pkl` and declares `page = new p.Page { slug = "energy-climate"; title = "Energy & Climate"; sections = layout.sections }`.

`energy-climate.layout.pkl` is hand-authored to match the regenerator's deterministic output for the four mockup sections: `HeroSection` (four stats: Power, Energy today, Indoor temp, Office CO₂), `ChartSection` (Power draw, 24h window), `EntityListSection` (tag `climate`, four `EntityRow` cells), `ActivityFeedSection` (climate sensors, 30-minute window).

**Acceptance:** `PageService.Get("energy-climate")` returns a `Page` proto with four sections.

**Commit:** `feat(config): sample page energy-climate.pkl`

---

## Task 6.15 — Delete C10 dashboard subsystem

In a single commit: delete `web/src/dashboard/`, `web/src/widgets/`, `internal/dashboard/`, `internal/config/pkl/switchyard/dashboards.pkl`; remove `react-grid-layout` and `@types/react-grid-layout` from `web/package.json`; run `npm install`. Update all broken imports. Replace the Plan 01 placeholder at `web/src/routes/_authed/pages/$slug.tsx` with the real implementation from Tasks 6.6 + 6.12.

**Acceptance:** `task web:build` and `go build ./...` green. `rg "react-grid-layout" web/` and `rg "internal/dashboard" .` both return nothing.

**Commit:** `chore: delete C10 dashboard subsystem (react-grid-layout, web/src/dashboard, web/src/widgets, internal/dashboard)`

---

## Task 6.16 — Playwright snapshot: render + edit mode

**File:** `web/e2e/custom-pages.spec.ts`

Two scenarios against the `energy-climate` sample page:

1. **Render mode** — navigate to `/pages/energy-climate`; assert page title and four section headings are visible; no edit chrome present; screenshot → `web/e2e/__screenshots__/custom-pages/render.png`.

2. **Edit mode** — click "Edit page"; assert "· editing" indicator, "Save & exit" / "Discard" buttons; click Chart section → accent border and "CHART · SELECTED" pill visible; settings rail heading reads "Chart section"; Pkl preview block present; screenshot → `web/e2e/__screenshots__/custom-pages/edit.png`.

**Acceptance:** `task web:e2e` green in CI.

**Commit:** `test(web): Playwright snapshots for custom pages render + edit mode`

---

## Token discipline

All new React components must pass `switchyard/no-raw-tokens`. The Pkl preview pane is the only intentional exception — it renders code in a fixed dark theme regardless of the active language (`#1c1c24` background, `#d4d2cb` text). Annotate those two lines with `// eslint-disable-line switchyard/no-raw-tokens` and a brief explanation.

---

## Test plan

- `go test ./internal/page/...` — PageService, regen golden tests, catalog all green.
- `go build ./...` — no broken imports.
- `task web:test` — all new component unit tests + `use-page-editor` state tests pass.
- `task web:lint` — `switchyard/no-raw-tokens` accepts all new files (except annotated Pkl preview lines).
- `task web:build` — bundle succeeds; `react-grid-layout` is absent from the bundle.
- `task web:e2e` — Playwright render + edit snapshots match.
- Manual smoke: `task ui:dev` → navigate to `/pages/energy-climate` → render mode looks like `08-custom-pages-01.png`; click "Edit page" → edit mode with rail looks like `08-custom-pages-02.png`.

## Acceptance criteria for merging

- All tests + typecheck + lint green locally and in CI.
- `proto/switchyard/v1alpha1/dashboard.proto` does not exist.
- `web/src/dashboard/` does not exist.
- `web/src/widgets/` does not exist.
- `react-grid-layout` does not appear in `web/package.json` or the built bundle.
- `internal/dashboard/` does not exist.
- `PageService` (not `DashboardService`) is the registered Connect handler.
- The `energy-climate` sample page renders all four sections in both render and edit mode.
- The settings rail live Pkl preview updates synchronously when a section prop changes.
- Widget pack tiers field is present in `WidgetClass` proto; existing packs without it default to `["section"]`.
- Linear parent issue + sub-tasks transition all the way to `Done`.
- Branch is merged via `git merge --no-ff` into main.
