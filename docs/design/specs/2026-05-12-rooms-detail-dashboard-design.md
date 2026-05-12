# Room detail dashboard — design

**Status:** Draft
**Date:** 2026-05-12
**Branch:** `feat/new-ui`

## Problem

The Vue UI's `/rooms` page lists areas as a grid of `SyRoomTile` cards but
the tiles are non-interactive and the page provides no per-room view. Users
can see that a room exists but can't drill into "what's in here", "what's
going on here", "what would I want to control / scene-trigger here". The
entityStore foundation we just shipped already exposes per-area views;
nothing consumes them.

## Goal

A `/rooms/:id` route that renders a room's full state at a glance:
identity, current entity state with controls, declared scenes, recent
activity scoped to the room's entities, and the automations declared to
operate on the room.

In scope:

- New `/rooms/:id` route (`RoomDetailView`).
- Header (room name + breadcrumb), summary band (entity count, type
  breakdown, on/off summary), scenes row, entities section, activity
  section, automations section.
- Three small backend pieces that unblock the activity + automations
  sections:
  1. `StoriesFilter.entity_ids` (proto + daemon filter logic) so the
     client can scope stories to "events touching one of these
     entities", which the UI translates as "events in this room" by
     resolving area → entities client-side.
  2. Pkl `Automation.areas: Listing<String>`, surfaced through the engine
     and registered as `Automation.area_ids` on the proto, with
     `ListAutomationsRequest.area_id` for filtering.
  3. UI clients for `SceneService.List` / `Apply` (no proto change —
     scenes are already global; the UI just renders them all per room
     as a v1 simplification).
- Make `/rooms` tiles clickable, navigating to `/rooms/:id`.

Out of scope (deferred follow-ups):

- Scene → area scoping. Scenes today are global; per-room curation is a
  separate Pkl design.
- Inferred automation→area linkage via Starlark AST analysis. We use
  declarative Pkl tagging — explicit, reliable, no analyzer.
- Streaming activity (`Tail` instead of one-shot `Stories`). The Activity
  page itself doesn't stream; bringing streaming to a per-room dashboard
  is its own design.
- Per-area metrics dashboards (energy use, etc.) — none of this data
  exists yet.

## Architecture

### Page layout

```
RoomDetailView (/rooms/:id)
   ┌──────────────────────────────────────────┐
   │ Header                                   │
   │   ‹ Rooms / Bedroom                      │
   │   Bedroom                                │
   │   3 lights · 1 sensor · 2 on / 2 off     │
   ├──────────────────────────────────────────┤
   │ Scenes                                   │
   │   [Wind down] [Reading] [All off]        │
   ├──────────────────────────────────────────┤
   │ Entities                                 │
   │   Lights                                 │
   │     SyEntityRow ×N                       │
   │   Sensors                                │
   │     SyEntityRow ×N                       │
   ├──────────────────────────────────────────┤
   │ Recent activity              View all → │
   │   SyEventRow ×5 (scoped to area)         │
   ├──────────────────────────────────────────┤
   │ Automations             Open editor → │
   │   SyAutomationCard ×N (scoped to area)   │
   └──────────────────────────────────────────┘
```

### Data flow

```
RoomDetailView
   ├─ entityStore.byArea(id)              ← already exists
   ├─ listScenes()                         ← new (data/scenes.ts)
   ├─ applyScene(id)                       ← new
   ├─ listStories({ entityIds })           ← extended (data/activity.ts)
   └─ listAutomations({ areaId })          ← extended (data/automations.ts)
```

`entityStore.byArea` is already populated for every area; navigating to
`/rooms/:id` is instantaneous after the store hydrates.

### Backend changes

**B1. `StoriesFilter.entity_ids`**

`proto/switchyard/activity/v1/activity.proto`:
```diff
 message StoriesFilter {
   string   kind        = 1;
   string   source      = 2;
   string   entity_id   = 3;
+  repeated string entity_ids = 4;
   ...
 }
```

The existing `entity_id` (singular) stays for back-compat. When
`entity_ids` is non-empty, the daemon's stories query matches a story if
any of its inner events' `entity` field is in the set. If both fields are
set, treat as union.

Daemon implementation lives in `internal/activity` — the coalescer
already filters internally; extending the in-memory filter to a set is a
small change. Tests cover singleton ↔ set parity and the union semantic.

**B2. Automation `areas`**

`internal/config/pkl/switchyard/automations.pkl` — add to the
`Automation` class:
```pkl
areas: Listing<String> = new {}
```

The Pkl→engine bridge already serialises Automation to a runtime struct;
extend that struct with `Areas []string` and surface it on the proto:

`proto/switchyard/v1alpha1/automation.proto`:
```diff
 message Automation {
   string id           = 1;
   string display_name = 2;
   string mode         = 3;
   bool   enabled      = 4;
   uint32 in_flight    = 5;
+  repeated string area_ids = 6;
 }

 message ListAutomationsRequest {
   PageRequest page = 1;
+  string area_id = 2;
 }
```

Daemon: `AutomationService.List` filters by `area_id` when set —
`automation.areas` contains it.

This is **declarative** linkage: the user types the area names into the
Pkl config. We pick declarative over inferred (AST analysis) because
inferred is brittle for dynamically-built entity ids and adds a Starlark
analyzer dependency we don't otherwise need.

### Frontend additions

- **New file** `app/src/views/RoomDetailView.vue` — the route component.
  Loads the area name from `listAreas` (or computes from `route.params.id`
  + the existing area cache), reads `entityStore.byArea(id)`, fetches
  scenes + stories + automations on mount.
- **New file** `app/src/data/scenes.ts` — `listScenes()` + `applyScene(id)`.
- **New file** `app/src/lib/components/scene/SyScene.vue` — chip variant
  for scene activation. Rounded pill with name; busy state during apply.
- **Extended** `app/src/data/activity.ts` — `listStories` accepts
  `entityIds: string[]`.
- **Extended** `app/src/data/automations.ts` — `listAutomations` accepts
  `areaId: string`.
- **Updated** `app/src/router/index.ts` — adds `/rooms/:id` mapping to
  `RoomDetailView`.
- **Updated** `app/src/views/RoomsView.vue` — `SyRoomTile` becomes
  clickable (`href` or `to` prop), navigates to `/rooms/:id`.
- **Updated** `app/src/router/crumbs.ts` — adds `/rooms/:id` →
  ["Rooms", "<area name>"] breadcrumb derivation.

### Component breakdown

- **Header section** uses `SyText` (display + caption) and `SyBadge` chips
  for the type breakdown. No new component.
- **Scenes row** uses the new `SyScene` chip in a horizontal flex
  container. Empty state hidden when there are no scenes (the row just
  isn't rendered).
- **Entities section** groups via plain template logic; each group is a
  `SyText` label + a stack of `SyEntityRow`. Empty per-type groups are
  hidden. Whole-section empty state shows "No entities in this room".
- **Activity section** uses `SyEventRow` (existing) + `SyEmptyState`.
  "View all →" links to plain `/activity` (no pre-filter). Pre-populating
  `entity_ids` requires extending Activity's URL-state schema; deferred
  to a separate iteration. Loading and error states match every other
  page.
- **Automations section** reuses `SyAutomationCard` (existing). Toggle
  + run actions reuse `enableAutomation` / `disableAutomation` /
  `triggerAutomation`. Empty state: "No automations declared for this
  room. Add one in the Pkl config."

### State / lifecycle

`RoomDetailView` is a vanilla composition-API component:
- Resolves `areaId` from `route.params.id`.
- Calls `listAreas()` on mount (cheap — RoomsView does the same) to
  resolve the area's display name. If the id isn't in the result, the
  "unknown room" empty state below kicks in.
- Reads entities reactively from `entityStore.byArea(areaId).value`.
- Fires `loadScenes`, `loadStories({ entityIds: entitiesInArea })`, and
  `loadAutomations({ areaId })` on mount; cleans up via `AbortController`.
- Refresh button on the header re-runs all three (entities are already
  live via the store).

### Iterations

This spec is one feature shipped in three iterations to keep each
diff reviewable and each step independently usable:

1. **Iteration 1 — UI scaffold + entities + scenes.** Route, view,
   clickable tiles, scenes client + chip, entities section. No proto
   changes, no backend changes. Activity + Automations sections render
   with "Coming soon" empty states.
2. **Iteration 2 — Stories area scoping.** `StoriesFilter.entity_ids`
   proto + daemon filter + client wrapper. Activity section becomes
   real on the room detail page.
3. **Iteration 3 — Automations area linkage.** Pkl `areas` field
   (requires Pkl-binding regen — `make gen` or equivalent) + engine
   surface + proto + `ListAutomationsRequest.area_id` filter + client
   wrapper. Automations section becomes real. Testing this end-to-end
   requires at least one automation declared in the dev Pkl config —
   the fixture currently has zero, so iteration 3 includes adding a
   sample automation tagged with an area.

Each iteration ends with green tests + Playwright validation against
the running daemon.

## Errors and edge cases

- **Unknown `:id`** in the route: render a `SyEmptyState` "This room
  doesn't exist" with a "Back to Rooms" button. Don't 404 the SPA route —
  the user might have followed a stale link.
- **Empty area** (zero entities, zero scenes, zero stories, zero
  automations): show the header + a single big empty state instead of
  five empty sections. Saves screen real estate.
- **`entityStore` not yet hydrated** when route loads: bind sections to
  `hydrated && byArea(...).length === 0` so the "no entities" message
  doesn't flash before the store loads.
- **Scene `Apply` failure**: chip surfaces inline error pill, same
  pattern as `SyEntityRow`.

## Testing

- **Lab specimens** for `SyScene` (deferred, follow-up — pattern with
  the rest of the lib).
- **Manual end-to-end via Playwright**:
  1. Open `/rooms`, click any tile → lands on `/rooms/:id` with the
     correct room name and entity list.
  2. Toggle a light from the room detail → bulb flips → state line in
     the row updates from the stream.
  3. Tap a scene → busy state, then apply succeeds; verify via daemon
     log that `SceneService.Apply` was called with the right id.
  4. (Iter 2+) Verify only events touching this room's entities appear
     in the Activity section.
  5. (Iter 3+) After adding a sample automation tagged
     `areas = ["bedroom"]` to the dev Pkl config, verify it appears
     when viewing `/rooms/bedroom` and not in other rooms.
- **Go tests** for backend changes:
  - `internal/activity` — TestStories_EntityIDsFilter (singleton ↔ set
    parity + union semantics).
  - `internal/automation` (and engine) — TestAutomation_AreasSurfaced
    (Pkl → runtime → proto round-trip).
  - `internal/api` — TestAutomationService_ListByArea (filter logic).
- **No new TypeScript test infra** — typecheck + Playwright remains the
  verification strategy for `app/`.
