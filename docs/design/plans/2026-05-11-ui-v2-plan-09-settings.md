# Plan 09 — Settings Sub-shell

> **Depends on Plan 01 merged to main.** Replaces the `/_authed/settings` and `/_authed/settings/$section` placeholders (created by Plan 01) with a fully functional settings sub-shell.

**Goal:** Ship a 220 px nav-rail + content-pane Settings sub-shell at `/_authed/settings/$section`, covering eight sections (Account, Drivers, Pkl config, Widget packs, Displays, Theme & language, Diagnostics, About), a new `DriverManagementService` proto + Go service, and per-section alert badges fed by the interestingness pipeline.

**Spec refs:** §15 (Settings Architecture), §5 (IA), §6 (app shell), §13 (permission scopes).

**Branch:** `feat/ui-v2-plan-09-settings`
**Worktree:** `.claude/worktrees/plan-09-settings`
**Depends on:** Plan 01 (`feat/ui-v2-plan-01-tokens-shell-ia`) merged to main
**Linear parent:** TBD (filled in after issue creation)

---

## Decisions (locked — no ambiguity for the implementer)

1. **Route shape:** `/_authed/settings/index.tsx` redirects to `account`. `/_authed/settings/$section.tsx` accepts `account | drivers | pkl-config | widget-packs | displays | theme-language | diagnostics | about`; any other value shows a "not found" fallback within the sub-shell.

2. **Sub-shell layout:** `220px` `SettingsNav` rail + `flex: 1` content pane. Layout replaces both Plan 01 placeholders.

3. **SettingsNav badge counts:** pulled from `ActivityService.Stories({ filter: { interesting_only: true } })` (Plan 03). Grouped by category → section: `failure | performance | anomaly` → `drivers`; `security | configuration` → `account`; `novelty` → `widget-packs`. Format: `"Drivers · 1 alert"` / `"Drivers · 3 alerts"`. Badge omitted at zero. Hook stubbed until Plan 03 merges.

4. **`DriverManagementService`** is a new proto at `proto/switchyard/driver/v1/management.proto` (package `switchyard.driver.v1`). Does not touch the existing `v1alpha1.DriverService`. Methods: `List`, `Get`, `Restart`, `Stop`, `Logs(last_n)`. Generated code lands in `gen/switchyard/driver/v1/`.

5. **Drivers section layout:** `Running` group (status `healthy | reconnecting | degraded`) and `Available` group (registry drivers not installed). Click a row to expand its inline detail card; only one card open at a time.

6. **Expanded driver card contents** (match mockup `11-settings-and-developer-01.png`): identity table (`pack`, `version`, `pid`, `socket`, `config`, `otel span`); metrics row (`entities`, `events/day`, `last cmd ack`, `reconnects today`); recent logs — last 8 lines — monospace dark terminal block; actions: `Open in Time-machine` → `/activity?driver=$id`, `Inspect entities` → `/devices?driver=$id`, `Stop driver`, `Restart`.

7. **Permission gating:** `Restart` and `Stop` are always rendered. Without scope `settings.drivers.write` (from auth store) they are `disabled` with `title="Requires the settings.drivers.write scope"`. No RPC is sent when disabled.

8. **PklConfig section** is a launcher-only card in Plan 09. A disabled `Open Pkl editor` button has `title="Coming in Plan 12 — Pkl / Starlark Editor"` and a footnote. No editor functionality.

9. **WidgetPacks section:** installed packs list (OCI ref, version, signature status chip: `verified` / `unverified` / `pending`) + `+ Install` button → inline dialog with OCI ref `<input>`. Install calls existing `widgetpack.InstallWidgetPack` RPC.

10. **Displays section** embeds `<PlaceholderPage title="Display list" plan="Plan 07" />`. Plan 07 owns the real list component; Plan 09 provides the shell entry point only.

11. **ThemeLanguage section** persists via `localStorage` key `sy.theme.v2` — same key `LanguageProvider` uses. Section calls `setLanguage` / `setMode` from `useLanguage()`; changes are live. Four controls: mode segmented control (`Light / Dark / System`), language segmented control (`Friendly / Ambient / Developer`), CLI-preview toggle (stored as `sy.theme.v2.cliPreview: boolean`), motion-reduction toggle (stored as `sy.theme.v2.motionReduction: "on" | "off" | "system"`; when `"on"`, `data-reduce-motion="true"` is set on `documentElement` and all `--sy-motion-*` tokens are overridden to `0ms`).

12. **Diagnostics section** calls `SystemService.Diagnostics` RPC (existing in `v1alpha1/system.proto`). Renders: health summary card, event-store stats table (size, age, snapshot count), `Export support bundle` button. If `SystemService.ExportSupportBundle` doesn't exist, add it to the proto before buf-generating.

13. **About section:** all values injected via Vite env vars: `VITE_SY_VERSION`, `VITE_SY_BUILD_SHA` (first 8 chars), `VITE_SY_BUILD_DATE`, `VITE_SY_LICENSE`, `VITE_SY_BINARY_FINGERPRINT`. Renders a `<code>` block for the fingerprint (shows `"not available"` when empty). Links to docs and issue tracker.

14. **Token discipline:** every style value uses `--sy-*` tokens. `switchyard/no-raw-tokens` must remain green.

---

## File plan

### Created — Go

```
proto/switchyard/driver/v1/
  management.proto                  ← DriverManagementService + messages

internal/driver/management/
  service.go                        ← DriverManagementService implementation
  service_test.go                   ← unit tests with a fake Registry interface
```

### Created — Web

```
web/src/routes/_authed/settings/index.tsx          ← redirect → /settings/account
web/src/routes/_authed/settings/$section.tsx       ← sub-shell layout (lazy-loads sections)

web/src/pages/settings/
  SettingsNav.tsx                                  ← 220px nav rail; badge slots
  SettingsNav.test.tsx
  useSettingsBadges.ts                             ← badge hook (stub → Plan 03 stream)

  sections/
    Account.tsx                                    ← passkeys, sessions, issued tokens
    Account.test.tsx
    Drivers/
      index.tsx                                    ← Running / Available split + expand state
      DriverCard.tsx                               ← collapsed row + expanded inline card
      DriverCard.test.tsx
      ExpandedDetail.tsx                           ← identity table, metrics, logs, actions
      ExpandedDetail.test.tsx
    PklConfig.tsx                                  ← launcher card only
    WidgetPacks.tsx                                ← installed list + install dialog
    WidgetPacks.test.tsx
    Displays.tsx                                   ← PlaceholderPage wrapper for Plan 07
    ThemeLanguage.tsx                              ← four controls wired to useLanguage()
    ThemeLanguage.test.tsx
    Diagnostics.tsx                                ← health + event-store + export button
    About.tsx                                      ← static build-time info

web/src/data/driver-management-client.ts           ← typed ConnectRPC client wrapper

web/e2e/settings-snapshot.spec.ts                  ← Playwright snapshot (Drivers + Z2M expanded)
```

### Modified

```
web/src/routes/_authed/settings/index.tsx     ← replace PlaceholderPage with redirect
web/src/routes/_authed/settings/$section.tsx  ← replace PlaceholderPage with sub-shell
```

---

## Tasks

### Task 9.1 — Define `DriverManagementService` proto + buf generate

**Files:** Create `proto/switchyard/driver/v1/management.proto`.

Define `DriverManagementService` with five RPCs: `List` → `ListDriversResponse{ repeated DriverSummary running; repeated RegistryDriver available; }`; `Get(id)`; `Restart(id, reason)`; `Stop(id, reason)`; `Logs(id, last_n)` → `{ repeated string lines; }`. `DriverSummary` carries: `id`, `pack`, `version`, `status`, `uptime_seconds`, `pid`, `socket`, `config_file`, `otel_span`, `entity_count`, `events_per_day`, `last_cmd_ack_ms`, `reconnects_today`, `reconnecting_since`. `RegistryDriver` carries: `id`, `pack`, `version`, `status` (`"available" | "update_available"`).

Run `buf generate`. Verify `gen/switchyard/driver/v1/` is created without errors.

**Commit:** `feat(proto): DriverManagementService for settings sub-shell (UI v2 plan 09)`

---

### Task 9.2 — Server-side `DriverManagementService` + tests

**Files:** Create `internal/driver/management/service.go`, `service_test.go`.

Define a `Registry` interface: `ListRunning`, `ListAvailable`, `Get`, `Restart`, `Stop`, `Logs`. `Service` wraps a `Registry` and satisfies the generated connect-go handler interface. All five methods delegate to the registry and wrap errors in `connect.CodeInternal`.

Write `service_test.go` with a `fakeRegistry` struct (in-memory slice). Test: `List` returns correct running + available split; `Get` returns the right driver or `CodeNotFound`; `Logs` returns the expected lines; `Restart` and `Stop` call through without error.

Run `go test ./internal/driver/management/... -v`. All tests must pass.

**Commit:** `feat(driver): DriverManagementService implementation + tests (UI v2 plan 09)`

---

### Task 9.3 — `SettingsNav` rail with badge slots

**Files:** Create `web/src/pages/settings/SettingsNav.tsx`, `SettingsNav.test.tsx`.

`SettingsNav` accepts `badges: { account?: number; drivers?: number; "widget-packs"?: number }`. Renders eight `<Link>` entries in order. Active link (matched via `useParams().section`) gets `aria-current="page"` and accent styling. Badge renders as `"1 alert"` (singular) or `"N alerts"` (plural) in `--sy-color-warn` when count > 0; omitted otherwise.

Tests (vitest + @testing-library/react): all eight labels present; active section gets `aria-current="page"`; badge renders when count > 0; badge absent when count is 0.

**Commit:** `feat(web): SettingsNav rail with badge slots (UI v2 plan 09)`

---

### Task 9.4 — Wire `$section` route to sub-shell layout

**Files:** Modify `web/src/routes/_authed/settings/index.tsx` and `$section.tsx`.

`index.tsx`: `<Navigate to="/settings/account" replace />`.

`$section.tsx`: flex container — `<SettingsNav badges={useSettingsBadges()} />` + `<main>`. `SECTION_MAP` is a record of `React.lazy()` imports for the eight section components. Unknown `section` param renders a `"Section not found"` paragraph. Wrap the lazy component in `<Suspense fallback={null}>`.

Create `web/src/pages/settings/useSettingsBadges.ts` as a stub returning `{}` until Plan 03 merges (include commented-out `ActivityService.Stories` stream wiring with category→section mapping documented).

Verify with `npx tsc --noEmit`. No errors.

**Commit:** `feat(web): settings sub-shell layout + section routing (UI v2 plan 09)`

---

### Task 9.5 — Account section

**Files:** Create `web/src/pages/settings/sections/Account.tsx`, `Account.test.tsx`.

Three sub-cards (each a bordered surface): **Passkeys** — calls `AuthService.ListPasskeys` (C9) via a `usePasskeys()` hook in `web/src/data/auth-client.ts`; each row shows name + creation date + `Revoke` button. **Active sessions** — `useSessions()`, each row shows user-agent + last-seen + Revoke. **Issued tokens** — `useTokens()`, each row shows label + expiry + Revoke. Revoke buttons call `revokePasskey(id)` / `revokeSession(id)` / `revokeToken(id)` from the same client file.

Tests (vitest, mock `../../../data/auth-client`): headings "Passkeys", "Active sessions", "Issued tokens" present; passkey row data renders.

**Commit:** `feat(web): Settings Account section (UI v2 plan 09)`

---

### Task 9.6 — Drivers section: list + expanded card + actions

**Files:** Create `web/src/pages/settings/sections/Drivers/index.tsx`, `DriverCard.tsx`, `DriverCard.test.tsx`, `ExpandedDetail.tsx`, `ExpandedDetail.test.tsx`, `web/src/data/driver-management-client.ts`.

`driver-management-client.ts`: `createClient(DriverManagementService, createConnectTransport({ baseUrl: "/api" }))` — export as `driverClient`.

`Drivers/index.tsx`: calls `driverClient.list({})` in `useEffect`; splits result into `running` + `available`; tracks `expanded: string | null` (only one card open). Renders `Running` heading + `DriverCard` list, then `Available` heading + install-row list.

`DriverCard.tsx`: collapsed row shows pack name, version, uptime, status chip (color-coded via `--sy-color-good/warn/bad/fg-4`). Click calls `driverClient.logs({ id, lastN: 8 })`, stores lines, toggles `expanded` prop. When expanded renders `<ExpandedDetail>`.

`ExpandedDetail.tsx`: identity `<table>`, metrics row (four stat tiles), `<pre>` log block (`#0d1117` background, `--sy-font-numeric`), four action buttons. `Stop driver` and `Restart` are `disabled` + have `title={SCOPE_TOOLTIP}` when `hasWriteScope` is false. Scope is read from auth store in `DriverCard` and passed as prop.

`DriverCard.test.tsx`: pack + version render; status chip shows; `onToggle` called on click; `Stop driver` / `Restart` are `disabled` and carry the scope tooltip when `hasWriteScope={false}`; buttons are enabled when `hasWriteScope={true}`.

`ExpandedDetail.test.tsx`: identity rows render; log lines appear; action links point to correct hrefs.

**Commit:** `feat(web): Settings Drivers section with expanded card + scope gating (UI v2 plan 09)`

---

### Task 9.7 — PklConfig, WidgetPacks, Displays sections

**Files:** Create `PklConfig.tsx`, `WidgetPacks.tsx`, `WidgetPacks.test.tsx`, `Displays.tsx`.

**PklConfig:** Single card with description and a `disabled` `Open Pkl editor` button. `title` attribute: `"Coming in Plan 12 — Pkl / Starlark Editor"`. Footnote below the button with same copy.

**WidgetPacks:** Calls `useInstalledPacks()` from `web/src/data/widget-pack-client.ts` (create stub if absent; calls `widgetpack.ListWidgetPacks` RPC). Each pack row shows OCI ref (monospace), version, signature status chip (`verified` → `--sy-color-good`, `unverified` → `--sy-color-warn`, `pending` → `--sy-color-fg-4`). `+ Install` button opens an inline `role="dialog"` with an OCI ref `<input>` (placeholder `ghcr.io/owner/pack:version`) and an `Install` submit button calling `installPack(ociRef)` from the client. Tests: pack ref renders; `verified` chip present; dialog opens on button click; input present in dialog.

**Displays:** `<PlaceholderPage title="Display list" plan="Plan 07" />` wrapped in a `<div>` with an `<h1>Displays</h1>` header.

**Commit:** `feat(web): Settings PklConfig, WidgetPacks, Displays sections (UI v2 plan 09)`

---

### Task 9.8 — ThemeLanguage section

**Files:** Create `ThemeLanguage.tsx`, `ThemeLanguage.test.tsx`.

Two `SegmentedControl` components: mode (`Light / Dark / System`) calls `setMode(key)`; language (`Friendly / Ambient / Developer`) calls `setLanguage(key)`. Active option gets `aria-pressed="true"` and accent background. Two `<Toggle>` checkboxes: CLI preview (`sy.theme.v2.cliPreview`) and reduce motion (`sy.theme.v2.motionReduction`); when reduce-motion is `"on"` the component sets `document.documentElement.dataset.reduceMotion = "true"` and injects a `<style>` tag overriding all `--sy-motion-*` vars to `0ms`.

Tests (mock `useLanguage`): mode buttons render; clicking `Light` calls `setMode("light")`; language buttons render; clicking `Developer` calls `setLanguage("developer")`.

**Commit:** `feat(web): Settings ThemeLanguage section (UI v2 plan 09)`

---

### Task 9.9 — Diagnostics + About sections

**Files:** Create `Diagnostics.tsx`, `About.tsx`.

**Diagnostics:** Calls `SystemService.Diagnostics` via `web/src/data/system-client.ts` (create stub if absent). Renders health summary card, event-store stats table (size, age, snapshot count), `Export support bundle` button that calls `SystemService.ExportSupportBundle` and triggers a browser `<a download>` blob URL. If `ExportSupportBundle` RPC does not exist in `v1alpha1/system.proto`, add it (returns a byte stream or a download URL) and run `buf generate`.

**About:** All values from `import.meta.env.VITE_SY_*` constants. Rows: version, build SHA (8 chars, mono), build date, license. Fingerprint in a `<code>` block (shows `"not available"` when empty). Two external links: docs + issue tracker.

Verify `npx tsc --noEmit`. No errors.

**Commit:** `feat(web): Settings Diagnostics + About sections (UI v2 plan 09)`

---

### Task 9.10 — Wire badge counts from interestingness

**Files:** Update `web/src/pages/settings/useSettingsBadges.ts`.

When Plan 03 is on the branch, replace the stub with the real stream: `activityClient.stories({ filter: { interestingOnly: true } })`. Iterate the async stream and accumulate counts: `failure | performance | anomaly` → `drivers`; `security | configuration` → `account`; `novelty` → `widget-packs`. Call `setBadges(counts)` after stream closes. Guard: if Plan 03 client is not yet available, keep the stub (empty badges) and leave the stream code in a comment.

**Commit:** `feat(web): useSettingsBadges wired to interestingness pipeline (UI v2 plan 09)`

---

### Task 9.11 — Lint + typecheck sweep

Run in `web/`:

```bash
npx tsc --noEmit
npx eslint src/pages/settings/ src/routes/_authed/settings/ src/data/driver-management-client.ts
```

Fix any `switchyard/no-raw-tokens` violations (replace raw colors with `--sy-*` tokens). Fix any TS errors.

**Commit:** `chore(web): fix lint + typecheck for settings sub-shell (UI v2 plan 09)`

---

### Task 9.12 — Playwright snapshot: Drivers section, Z2M expanded

**Files:** Create `web/e2e/settings-snapshot.spec.ts`.

`page.route` mocks for `DriverManagementService/List` (return three drivers: Hue Bridge healthy, Zigbee2MQTT reconnecting with full identity/metrics, ESPHome healthy; plus one available: HomeKit Bridge) and `DriverManagementService/Logs` (return eight realistic Z2M log lines matching the mockup). Navigate to `/settings/drivers`, wait for `@switchyard/z2m` to appear, click its row button to expand, wait for log lines, take a `toHaveScreenshot` snapshot named `settings-drivers-z2m-expanded-friendly-light.png`.

Run `npm run test:e2e -- --update-snapshots` to generate reference images. Re-run without the flag to confirm pass. Check in the screenshot.

**Commit:** `test(web): Playwright snapshot of settings Drivers section Z2M expanded (UI v2 plan 09)`

---

## Test plan

- `go test ./internal/driver/management/... -v` — all service tests pass.
- `npx vitest run src/pages/settings/` — SettingsNav (4), Account (4), DriverCard (6), WidgetPacks (3), ThemeLanguage (4).
- `npx tsc --noEmit` — zero errors.
- `npx eslint src/pages/settings/` — zero violations.
- `npm run test:e2e -- e2e/settings-snapshot.spec.ts` — snapshot matches reference.
- Manual smoke: `task ui:dev` → `/settings/drivers` → click Z2M row → expanded card shows identity table, metrics, log terminal, action buttons. Without `settings.drivers.write` scope in auth store, Stop/Restart are disabled with tooltip.

## Acceptance criteria for merging

- All tests + typecheck + lint green locally and in CI.
- Eight sections reachable; active section highlighted in the nav rail.
- Drivers section: Running / Available split; one card expandable at a time; Stop/Restart scope-gated.
- ThemeLanguage changes take effect live via `useLanguage()`.
- PklConfig shows disabled launcher with Plan 12 footnote.
- Displays renders Plan 07 placeholder.
- No raw colors or `--gh-*` tokens in any new component.
- `DriverManagementService` proto generated; Go service compiles and tests pass.
- Playwright snapshot checked in and passing in CI.
- Linear parent issue + sub-tasks transition to `Done`.
- Branch merged via `git merge --no-ff` into main.
