# Plan 07 — Displays + Ambient Language

> **Depends on:** Plan 01 (token system + shell + IA) and Plan 06 (Custom Pages + three-tier widgets) merged to main. Plan 07 extends Plan 01's `LanguagePrimitives` provider, Plan 06's `RoomTile`, and the `PageService` proto that Plan 06 already ships.

**Goal:** Wire the `ambient` language end-to-end. A Display is a Custom Page bound to a target device, rendered in `ambient` mode with no shell chrome, per-tile fidelity slots, a time-of-day gradient, and an alert pill fed by the interestingness pipeline. Includes the full pairing flow (operator-side UI + device-side redemption page), a new `DisplayService` proto + server implementation, and sample displays.

**Spec refs:** §13 (Ambient + Displays), §4.2 (ambient language definition), §9 (interestingness alerts on displays).

**Branch:** `feat/ui-v2-plan-07-displays-ambient`
**Worktree:** `.claude/worktrees/plan-07-displays-ambient`
**Depends on:** Plan 01 + Plan 06 merged to main
**Linear parent:** TBD

---

## Decisions (locked — no ambiguity for the implementer)

1. **A Display is a Custom Page bound to a target device**, rendered through the ambient language. `Display.page_slug` references an existing Custom Page slug (from Plan 06's `PageService`). If the page is deleted, the display shows a built-in fallback ambient template (room grid + scene strip).
2. **`DisplayService` proto** lives at `proto/switchyard/display/v1/display.proto`. RPCs: `Pair()` → returns a 6-digit code with 5-minute TTL; `RedeemPairCode(code, device_name)` → claims the code and stores a per-display token; `List()`, `Get(id)`, `Update(id, config)`, `Unpair(id)`.
3. **Pairing flow:** operator clicks "Pair new display" → modal with 6-digit code + countdown. Target device opens `https://<host>/pair` (new public route, no auth required) → enters the code → backend redeems it and stores a per-display token bound to the named device. Per-display token is not a user session; it authenticates only `DisplayService` and `PageService.Render` calls.
4. **Per-display config shape:**
   - `page_slug` string — which Custom Page to render (empty = built-in ambient template).
   - `tile_overrides` map\<tile_id, FidelityOverride\> — per-tile fidelity (see decision 5).
   - `idle_behavior` struct: `wake_on_motion bool`, `dim_after_minutes int32`, `off_after_minutes int32`.
   - `allowed_interactions` repeated string — entity glob list restricting what this display may control.
   - `alert_threshold` enum `NONE | LOW | MEDIUM | HIGH` — minimum interestingness severity to surface the ambient alert pill.
5. **Per-tile fidelity slots:** three independent axes:
   - `width`: `standard` (default) | `wide` (double column).
   - `scenes`: `0` | `2` | `4` inline scene chips.
   - `metric`: `none` | `sensor` | `presence` | `now_playing` | `next_automation` | `last_activity`.
   Defaults are computed server-side by `FidelityRecommender`: rooms with high entity count or frequent interaction history score higher and receive `scenes:4 metric:sensor`. Rooms with few entities score minimal (`scenes:0 metric:none`).
6. **Ambient language primitives** (`Button`, `Surface`, `Chip`, `Pill`) are glassmorphic capsule variants: `backdrop-filter: blur(20px)`, `border-radius: var(--sy-radius-xl)` (22–28px), `background: color-mix(in srgb, var(--sy-color-surface-1) 55%, transparent)`. Registered via Plan 01's `LanguagePrimitives` provider, filling the `null` slots that Plan 01 left for the `ambient` language.
7. **Time-of-day gradient:** a `useTodGradient()` hook computes the `--sy-gradient-tod` CSS gradient string client-side from the local solar table. Use `github.com/sj14/astral` (already present in the automation engine) server-side for sunrise/sunset times; the web client receives them via a lightweight `SolarService.GetTable()` RPC (returns today's solar events as timestamps). The hook maps local time to five named phases (pre-sunrise, sunrise, midday, sunset, night) and linearly interpolates gradient stops between them. Returns a CSS `radial-gradient` string. `AmbientRoot` binds it to `--sy-gradient-tod` as an inline style on the root element. Updated every 5 minutes via `setInterval`; CSS `transition: --sy-gradient-tod 90s linear` handles the smooth drift.
8. **Alert state:** when the interestingness pipeline emits a `failure` or `causation` event at severity ≥ the display's `alert_threshold`, the ambient renderer surfaces: (a) a centered `AlertPill` at top showing the alert summary; (b) affected `RoomTile`s dim to `opacity: 0.55` with a "stale" badge; (c) the scene strip trims to safe scenes (excludes scenes whose entity-set overlaps the stale entity set). Alert clears automatically when the pipeline emits a matching `resolved` tag.
9. **The renderer at `/display/:id`** is a public route outside `_authed`. The `Shell` does NOT wrap this route. Auth is via the per-display token stored in `localStorage` under key `sy.display.<id>.token`; the Connect transport reads it from there for `DisplayService` and `PageService` calls. No cookie session is involved.
10. **`/pair` public route** — rendered outside `_authed`, no Shell. Accepts a 6-digit code via a large-digit input, calls `DisplayService.RedeemPairCode()`, stores the returned per-display token in `localStorage`, then navigates to `/display/:id`.

---

## File plan

### Created

```
proto/switchyard/display/v1/
  display.proto               ← DisplayService + Display message + FidelityOverride
  display.pb.go               ← buf-generated (do not edit)
  display_grpc.pb.go          ← buf-generated

internal/display/
  service.go                  ← DisplayService gRPC implementation
  pairing.go                  ← pair-code store (in-memory TTL map, backed by auth service)
  fidelity_recommender.go     ← FidelityRecommender + tests

web/src/routes/
  display.$id.tsx             ← PUBLIC renderer route (outside _authed)
  pair.tsx                    ← PUBLIC pair-code redemption page

web/src/routes/_authed/displays/
  index.tsx                   ← Display list + "Pair new display" affordance
  $slug.tsx                   ← Per-Display config editor (replaces Plan 01 placeholder)

web/src/ambient/
  AmbientRoot.tsx             ← Root wrapper: no shell, applies data-language="ambient", binds --sy-gradient-tod
  useTodGradient.ts           ← Gradient hook (solar table RPC → phase map → CSS string)
  AlertPill.tsx               ← Alert pill component listening to interestingness events

web/src/theme/primitives/ambient/
  button.tsx                  ← Glassmorphic capsule button
  chip.tsx                    ← Glassmorphic chip
  pill.tsx                    ← Glassmorphic status pill
  surface.tsx                 ← Glassmorphic surface card

displays/
  kitchen-wall.pkl            ← Sample display (wide RoomGrid + scene strip)
  bedroom-nightstand.pkl      ← Sample display (minimal fidelity, 2 rooms)
```

### Modified

```
proto/switchyard/solar/v1/solar.proto     ← add SolarService.GetTable() RPC if not present
internal/solar/service.go                 ← wire GetTable() using astral (already used in automations)
web/src/theme/languages/ambient.css       ← expand: glassmorphic token values, --sy-gradient-tod phases
web/src/theme/primitives-provider.tsx     ← register ambient primitive set (fills the null slots from Plan 01)
web/src/pages-system/widgets/tiles/RoomTile.tsx  ← add fidelity slot props (width / scenes / metric)
web/src/shell/Sidebar.tsx                 ← populate "Displays" section from DisplayService.List()
web/src/routes/_authed/_layout.tsx        ← exclude /display/:id + /pair from Shell wrapping (already handled by route structure)
web/go.mod                                ← add astral if not already present
```

---

## Tasks

### Task 7.1 — Define `display.proto` + `buf generate`

**Files:** `proto/switchyard/display/v1/display.proto`. If `SolarService.GetTable()` does not exist in `proto/switchyard/solar/v1/solar.proto`, add it (returns sunrise, sunset, solar_noon for today + tomorrow).

**How:** author protos; run `buf generate`; commit generated files; wire both services into the Connect-RPC mux in `cmd/switchyardd/main.go`.

**Acceptance:** `buf lint` clean; `go build ./...` passes; both services appear in the service registry.

**Commit:** `feat(proto): DisplayService + SolarService.GetTable (UI v2 plan 07)`

### Task 7.2 — Server-side `DisplayService` + pairing flow + tests

**Files:** `internal/display/service.go`, `internal/display/pairing.go`.

`pairing.go` owns an in-memory `PairCodeStore` (`sync.Map` of `code → {device_name, expires_at}`). Codes are 6-digit zero-padded random strings. Entries expire lazily on read + a background sweep goroutine every minute. The store is injected into `service.go`.

`service.go` implements `DisplayService`. `RedeemPairCode()` validates + removes the code, creates a `Display` record, generates a per-display JWT (audience `display:<id>`), persists via the Pkl filesystem backend (Plan 06 pattern), and returns the display ID + token. `List/Get/Update/Unpair` operate on the Pkl-backed store.

**TDD:**
- `pairing_test.go`: code expiry, double-redeem rejected, expired code rejected, 6-digit format.
- `service_test.go`: `Pair()` returns non-empty code; `RedeemPairCode()` returns token; second redeem returns `NOT_FOUND`; `Update()` persists overrides and reads back.

**Acceptance:** `go test ./internal/display/...` green.

**Commit:** `feat(server): DisplayService + pairing flow (UI v2 plan 07)`

### Task 7.3 — `FidelityRecommender` + tests

**File:** `internal/display/fidelity_recommender.go`.

The recommender accepts a slice of `Room` (entity count, sensor count, interaction count in the last 30 days) and returns a `map[room_id]FidelityOverride`. Scoring:

- `entity_count ≥ 5 AND interaction_count_30d ≥ 10` → `scenes:4, metric:sensor`
- `entity_count ≥ 3 OR interaction_count_30d ≥ 5` → `scenes:2, metric:presence`
- Otherwise → `scenes:0, metric:none`
- A room with `sensor_count ≥ 1` always gets `metric:sensor` regardless of entity/interaction score.
- `width` is always `standard` by default; only user overrides can promote to `wide`.

**TDD:** table-driven tests covering the boundary conditions above plus the sensor-count override.

**Acceptance:** `go test ./internal/display/...` green; recommender is called by `DisplayService.Pair()` to populate initial `tile_overrides` (user can override later).

**Commit:** `feat(server): FidelityRecommender (UI v2 plan 07)`

### Task 7.4 — Register ambient language primitives

**Files:** `web/src/theme/primitives/ambient/{button,chip,pill,surface}.tsx`, `web/src/theme/primitives-provider.tsx`, `web/src/theme/languages/ambient.css`.

Each ambient primitive is a glassmorphic capsule variant:
- `backdrop-filter: blur(20px) saturate(1.4)`
- `background: color-mix(in srgb, var(--sy-color-surface-1) 55%, transparent)`
- `border: 1px solid color-mix(in srgb, var(--sy-color-line) 30%, transparent)`
- `border-radius: var(--sy-radius-xl)` for Surface and Button; `var(--sy-radius-pill)` for Chip and Pill.

Expand `ambient.css` to add the glassmorphic overrides and the five gradient-phase token sets:
```css
:root[data-language="ambient"] {
  --sy-tod-pre-sunrise: radial-gradient(ellipse at 60% 40%, #0d1b2a 0%, #0a0e1a 100%);
  --sy-tod-sunrise:     radial-gradient(ellipse at 60% 40%, #7c3a2a 0%, #2c1a3a 100%);
  --sy-tod-midday:      radial-gradient(ellipse at 60% 40%, #1a2a4a 0%, #2a1a3a 100%);
  --sy-tod-sunset:      radial-gradient(ellipse at 60% 40%, #c25a2a 0%, #3a1a4a 100%);
  --sy-tod-night:       radial-gradient(ellipse at 60% 40%, #0f0a1a 0%, #0a0a14 100%);
}
```

Register all four ambient components in `primitives-provider.tsx` under `language: "ambient"`.

**TDD:**
- Render `<Surface>` under `LanguagePrimitives` with `language="ambient"` — assert it renders with `data-primitive="ambient-surface"`.
- Assert `usePrimitive("Button")` with `language="ambient"` returns the ambient button, not the friendly one.

**Commit:** `feat(web): ambient language primitives (UI v2 plan 07)`

### Task 7.5 — `useTodGradient` hook + unit test

**File:** `web/src/ambient/useTodGradient.ts`

On mount, calls `SolarService.GetTable()` for sunrise/sunset/solar-noon timestamps. Maps `Date.now()` to one of five phases (pre-sunrise / sunrise / midday / sunset / night) with linear interpolation in ±90-minute transition windows. Returns a CSS gradient string. Refreshes every 5 minutes via `setInterval`; cleans up on unmount. Falls back to `--sy-tod-night` on RPC failure.

**Unit test** (`useTodGradient.test.ts`): mock `SolarService`; advance fake clocks to three phases; assert gradient string matches; assert interval cleanup.

**Acceptance:** `task web:test` green; hook is exported from `web/src/ambient/index.ts`.

**Commit:** `feat(web): useTodGradient hook (UI v2 plan 07)`

### Task 7.6 — `AmbientRoot` component

**File:** `web/src/ambient/AmbientRoot.tsx`

Sets `data-language="ambient"` on its own root `<div>` (not `documentElement`, so operator tabs keep their language). Calls `useTodGradient()` and binds the gradient to `--sy-gradient-tod` as an inline style. Wraps children in `<LanguagePrimitives language="ambient">`. Base style: `min-height: 100dvh; background: var(--sy-gradient-tod); transition: background 90s linear`. No Shell rendered.

**TDD:** assert `data-language="ambient"` is on the root div (not `documentElement`); assert a child `<Surface>` resolves to the ambient primitive.

**Commit:** `feat(web): AmbientRoot component (UI v2 plan 07)`

### Task 7.7 — `/display/:id` public route + per-display token auth

**File:** `web/src/routes/display.$id.tsx`

The route:
1. Reads `localStorage.getItem('sy.display.<id>.token')`. If absent, redirects to `/pair?hint=<id>`.
2. Constructs a Connect transport with the per-display token in the `Authorization: Bearer` header (a second transport instance, separate from the user session transport).
3. Calls `DisplayService.Get(id)` to fetch display config; calls `PageService.GetPage(page_slug)` to fetch the Custom Page definition.
4. Renders `<AmbientRoot>` with the assembled page. Uses `<AlertPill>` if alert state is active.
5. Subscribes to `EventService.Tail()` (filtered to interestingness tags) to keep the alert state live.

The route is registered in `web/src/routes/__root.tsx` outside the `_authed` tree so the Shell layout does not wrap it.

**Acceptance:** navigating to `/display/unknown-id` redirects to `/pair`; navigating with a valid token renders the ambient page with `data-language="ambient"` at the root.

**Commit:** `feat(web): /display/:id public route (UI v2 plan 07)`

### Task 7.8 — `/pair` public route + redemption flow

**File:** `web/src/routes/pair.tsx`

The pair page:
- No Shell, no nav.
- Shows the Switchyard wordmark at top.
- A 6-digit code input (six `<input type="text" maxLength={1}>` cells with auto-focus advance), or a single `<input type="text" inputMode="numeric" maxLength={6}>` with large-digit styling — choose the single-input approach for simplicity.
- On submit, calls `DisplayService.RedeemPairCode(code, device_name)` where `device_name` defaults to `navigator.userAgent` trimmed to 40 chars (user can edit it inline before submitting).
- On success: stores `sy.display.<id>.token` in `localStorage` and navigates to `/display/<id>`.
- On error: shows "Code not found or expired. Ask the operator for a new code." with a retry affordance.

**TDD:**
- Mock `DisplayService.RedeemPairCode`; submit a valid code; assert navigation to `/display/<id>` and that `localStorage` contains the token.
- Submit an invalid code; assert error message is shown.

**Commit:** `feat(web): /pair public route + pair-code redemption (UI v2 plan 07)`

### Task 7.9 — Operator Display editor UI (list + per-display config)

**Files:** `web/src/routes/_authed/displays/index.tsx`, `web/src/routes/_authed/displays/$slug.tsx`.

**List page (`index.tsx`):**
- Fetches `DisplayService.List()` and renders a table: device name, assigned page, last seen, alert threshold, actions (Configure, Unpair).
- "Pair new display" button opens a modal: calls `DisplayService.Pair()` → shows the 6-digit code with a 5-minute countdown ring; closes after successful redemption or expiry.
- Populates the "Displays" section of the Sidebar (replaces "No displays yet." from Plan 01).

**Config page (`$slug.tsx` — replaces Plan 01 placeholder):**
- Loads the display via `DisplayService.Get(id)`.
- Form sections: (1) Assigned Page (page picker dropdown populated from `PageService.List()`); (2) Fidelity Overrides (a room-by-room table with width/scenes/metric dropdowns and a "Reset to recommended" per-row action); (3) Idle Behavior (three numeric/toggle inputs); (4) Allowed Interactions (entity glob multiselect); (5) Alert Threshold (radio group: None / Low / Medium / High). Save calls `DisplayService.Update()`.
- "Preview in new tab" button opens `/display/<id>` in a new tab.
- "Unpair" button (destructive, requires confirmation) calls `DisplayService.Unpair()`.

**TDD:**
- Render the list with one mocked display; assert the device name and page slug are shown; assert "Pair new display" button is present.
- Render the config page; change the alert threshold to High; click Save; assert `DisplayService.Update` was called with `alert_threshold: ALERT_HIGH`.

**Commit:** `feat(web): Display list + per-display config editor (UI v2 plan 07)`

### Task 7.10 — `RoomTile` fidelity slot support

**File:** `web/src/pages-system/widgets/tiles/RoomTile.tsx` (added by Plan 06).

Add props:
```ts
interface RoomTileFidelity {
  width?: "standard" | "wide";
  scenes?: 0 | 2 | 4;
  metric?: "none" | "sensor" | "presence" | "now_playing" | "next_automation" | "last_activity";
}
```

- `width: "wide"` sets `grid-column: span 2` (the ambient grid is a CSS grid with `auto-fill minmax(320px, 1fr)`).
- `scenes` controls how many scene chips render in the inline strip (0 = no strip; 2 = first two scenes; 4 = first four).
- `metric` renders a metric row at the bottom of the tile: sensor value, presence indicator, now-playing track, next scheduled automation time, or last entity change timestamp. `"none"` renders nothing.

Defaults: `width:"standard"`, `scenes:2`, `metric:"sensor"` (these are the defaults shown in the mockup's "balanced" fidelity slot).

**TDD:** snapshot tests for all three fidelity presets shown in `10-ambient-v2-02.png` (minimal / balanced / rich).

**Commit:** `feat(web): RoomTile fidelity slot props (UI v2 plan 07)`

### Task 7.11 — `AlertPill` + interestingness subscription

**File:** `web/src/ambient/AlertPill.tsx`

The pill:
- Listens to the `EventService.Tail()` stream filtered to `interesting_because` tags `failure` and `causation`.
- Maintains local state: `{ active: boolean; message: string; affectedEntityIds: string[] }`.
- When an event with matching severity arrives, sets `active: true` and surfaces a pill at `position: fixed; top: var(--sy-space-3); left: 50%; transform: translateX(-50%)` with the Ambient `Pill` primitive (glassmorphic).
- Exposes `affectedEntityIds` via a React context so `RoomTile`s can dim themselves.

The `RoomTile` reads from this context: if any of its entity IDs appear in `affectedEntityIds`, it renders at `opacity: 0.55`, shows a "stale" badge below the room name, and clips scene chips to exclude unsafe scenes (those whose entity-set intersects `affectedEntityIds`).

The `AlertPill` uses the display's `alert_threshold` (passed as a prop from the ambient route) to gate whether to activate.

**TDD:**
- Mock the EventService stream; push a `failure` event with severity `MEDIUM`; render `<AlertPill alertThreshold="LOW">` — assert the pill becomes visible with the right text.
- Push a `failure` event with severity `LOW`; render `<AlertPill alertThreshold="HIGH">` — assert pill does NOT appear.
- Assert that a room tile whose entity ID is in `affectedEntityIds` renders at reduced opacity.

**Commit:** `feat(web): AlertPill + affected-tile dimming (UI v2 plan 07)`

### Task 7.12 — Wire alert threshold per display

**Files:** `web/src/routes/display.$id.tsx`, `web/src/ambient/AmbientRoot.tsx`.

Thread `display.alert_threshold` (already fetched in Task 7.7) down to `<AlertPill alertThreshold={...}>`. Smoke-test: change threshold in the operator editor, reload the display route, confirm the gate changes.

**Acceptance:** `task web:test` green.

**Commit:** `feat(web): wire alert threshold to ambient renderer (UI v2 plan 07)`

### Task 7.13 — Sample displays (Pkl)

**Files:** `displays/kitchen-wall.pkl`, `displays/bedroom-nightstand.pkl`.

`kitchen-wall.pkl`:
```pkl
amends "package://pkg.switchyard.local/display@1/display.pkl"
device_name = "Kitchen Wall"
page_slug = "home"
tile_overrides {
  ["kitchen"] { width = "wide"; scenes = 4; metric = "sensor" }
  ["living"]  { width = "standard"; scenes = 2; metric = "now_playing" }
}
idle_behavior { dim_after_minutes = 5; off_after_minutes = 30 }
alert_threshold = "medium"
```

`bedroom-nightstand.pkl`:
```pkl
amends "package://pkg.switchyard.local/display@1/display.pkl"
device_name = "Bedroom Nightstand"
page_slug = "home"
tile_overrides {
  ["bedroom"]  { width = "wide"; scenes = 2; metric = "none" }
  ["hallway"]  { width = "standard"; scenes = 0; metric = "last_activity" }
}
idle_behavior { wake_on_motion = true; dim_after_minutes = 2; off_after_minutes = 10 }
alert_threshold = "high"
```

**Acceptance:** `pkl eval displays/kitchen-wall.pkl` and `pkl eval displays/bedroom-nightstand.pkl` succeed (no validation errors).

**Commit:** `feat(config): sample display configs (UI v2 plan 07)`

### Task 7.14 — Playwright snapshot tests

**File:** `web/e2e/ambient-snapshot.spec.ts`

Three scenarios:
1. **Midday:** mock `SolarService.GetTable()` so `Date.now()` falls in the midday phase. Render `/display/test-id` (balanced fidelity, no alert). Assert no AlertPill; take full-page screenshot.
2. **Sunset:** mock solar table to 45 minutes before sunset. Assert gradient variable contains sunset-phase stops; take full-page screenshot.
3. **Pair page:** snapshot both success and error states.

**Acceptance:** `task web:e2e` green; reference images in `web/e2e/__screenshots__/ambient-snapshot/`.

**Commit:** `test(web): Playwright snapshots for ambient display (UI v2 plan 07)`

---

## Test plan

- `go test ./internal/display/...` — pairing, code expiry, fidelity recommender scoring, service CRUD.
- `task web:test` — `useTodGradient` hook (clock-mocked phases), `AlertPill` threshold gating, `RoomTile` fidelity snapshot tests, `AmbientRoot` data-language assertion, `/pair` redemption flow.
- `task web:build` — bundle compiles with new public routes outside `_authed`.
- `task web:e2e` — ambient snapshot at midday + sunset; `/pair` page states.
- Manual smoke: `task ui:dev` → pair a display → navigate to `/display/<id>` → confirm no Shell chrome, ambient gradient background, time-appropriate gradient, room tiles with correct fidelity. Simulate an alert by mocking an interestingness event → confirm AlertPill appears and tiles dim.

## Acceptance criteria for merging

- All tests + typecheck + lint green locally and in CI.
- `/display/:id` renders with `data-language="ambient"`, no Shell chrome, and the full ambient token surface applied.
- `/pair` successfully redeems a code and navigates to `/display/:id`.
- The time-of-day gradient transitions smoothly (confirmed by running for at least two 5-minute refresh cycles in the smoke test).
- `RoomTile` fidelity props are respected: minimal / balanced / rich render distinctly (confirmed against `10-ambient-v2-02.png`).
- Alert pill appears only at or above the configured threshold; affected tiles dim to 0.55 opacity.
- Both sample Pkl display configs parse cleanly.
- Sidebar "Displays" section populates from `DisplayService.List()`.
- Linear parent issue + sub-tasks transition all the way to `Done`.
- Branch is merged via `git merge --no-ff` into main.
