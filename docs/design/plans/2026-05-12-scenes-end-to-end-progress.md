# Scenes end-to-end — execution progress log

Durable log for `docs/design/plans/2026-05-12-scenes-end-to-end.md`.
Branch: `feat/scenes-end-to-end` (off main).

## Task status

| ID | Title | Model | Status | Notes |
|----|-------|-------|--------|-------|
| 1 | Proto changes (SceneConfig + Scene + SceneApplied) | haiku | ✅ | |
| 2 | Pkl schema + sceneJSON | haiku | ✅ | |
| 3 | Compile-time dangling area check | haiku | ✅ | |
| 4 | RenderScene emits area_id | haiku | ✅ | |
| 5 | scene.Applier package | sonnet | ✅ | |
| 6 | RealSceneService | sonnet | ✅ | |
| 7 | Wire daemon (replace stub + service) | sonnet | ✅ | |
| 8 | Daemon E2E for Apply | sonnet | ✅ | |
| 9 | TS Scene.areaId + SySceneForm | haiku | ✅ | |
| 10 | SyAreaForm | haiku | ✅ | |
| 11 | RoomsView (global scenes section + form triggers) | sonnet | ✅ | |
| 12 | RoomDetailView (scoped filter + form trigger) | sonnet | ✅ | |
| 13 | RegenPreview area+scene verification | haiku | ✅ | |
| 14 | Loop-closure validation | controller | ✅ | |

✅ pending · 🟢 in progress · ✅ done · ❌ blocked

## Decision log

- **SE-1:** Used tag 54 for `SceneApplied` in the Payload oneof (next free).
  `RunOutcome` enum was already present in event.proto.
- **SE-5:** Real enum values are `RunOutcome_OUTCOME_OK` /
  `RunOutcome_OUTCOME_ACTION_ERROR` (not the `_RUN_OUTCOME_SUCCESS/FAILURE`
  the plan guessed). `ParallelBlock` field name is `ChildCtrl` not `Ctrls`.
  Applier code adjusted.
- **SE-7:** `script.Engine` doesn't directly satisfy `action.ScriptCaller`
  (return-type mismatch on `CallResult`). Added `scriptCallerAdapter` +
  `scriptCallResultAdapter` in daemon.go to bridge.
- **SE-8:** `daemon.Config` has no `TCPPort` field — TCP bind is
  Pkl-config-driven. Test sets `listener { tcp { bind = "127.0.0.1:<free>" } }`
  in main.pkl directly. SocketPath field is for the CLI mutative-ops
  socket, separate from the API listener UDS at `<dataDir>/switchyardd.sock`.
- **SE-14 (mid-validation fix):** `RoomDetailView` initially hid the
  entire Scenes section (including the "+ New scene" button) when no
  scenes were scoped to the room — users couldn't create the first
  scoped scene from the UI. Fixed in commit `531defe`: section header
  + button always render; inner list shows an empty-state caption
  ("No scenes yet") when empty.

## Blockers + resolutions

- **Stale daemon at validation time:** prior `dist/switchyardd` process
  (PID 97772) was holding port 8080 + a stale lockfile. Killed + cleared
  lockfile + rebuilt + restarted. No actual blocker.

## Loop-closure validation (SE-14)

Driven via Playwright against the live dev daemon + vite:

1. ✅ `/rooms` → "+ New scene" → fill id/displayName → Save →
   "Playwright validation" appears in GLOBAL SCENES section within 2s
   (no page reload).
2. ✅ `/rooms/kitchen` → Scenes header + "+ New scene" visible →
   form shows "Scoped to room: kitchen" caption → Save → "Kitchen PW Test"
   appears in Kitchen's Scenes section.
3. ✅ Returned to `/rooms` → GLOBAL SCENES shows ONLY the global scene
   (room-scoped scenes correctly filtered out).
4. ✅ Track A's reactive `configStore.onChanged` pipeline refreshes
   both views automatically after form save.
5. ✅ Daemon E2E test (`TestScene_ApplyAndNotFound`) proves Apply
   returns correlation_id for valid scene + NotFound for unknown.
