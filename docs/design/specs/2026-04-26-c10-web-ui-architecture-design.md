# C10 — Web UI Architecture Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Date:** 2026-04-26
**Status:** Draft
**Depends on:** C1 (Event Core), C4 (Pkl Config), C5 (Starlark Runtime), C7 (Connect-RPC API), C9 (Auth & Policy — in flight)
**Closes:** the master design's §7.3 web-UI architecture roadmap (foundation + dashboard subsystem). Feature pages (entities, automations, events explorer, config editor, drivers, rich auth UI) are explicit follow-on milestones, not C10.

---

## Table of Contents

1. [Scope](#1-scope)
2. [Background](#2-background)
3. [Architecture Overview](#3-architecture-overview)
4. [App Shell & Routing](#4-app-shell--routing)
5. [Design Tokens & Theming Architecture](#5-design-tokens--theming-architecture)
6. [Data Layer](#6-data-layer)
7. [Multiplexer](#7-multiplexer)
8. [Pending-State UX Pattern](#8-pending-state-ux-pattern)
9. [Login Flow](#9-login-flow)
10. [Dashboard Render Path](#10-dashboard-render-path)
11. [Dashboard Edit Mode](#11-dashboard-edit-mode)
12. [Pkl Round-Trip](#12-pkl-round-trip)
13. [Widget Contract](#13-widget-contract)
14. [v1 Built-In Widgets](#14-v1-built-in-widgets)
15. [Widget Pack Format](#15-widget-pack-format)
16. [PWA & Theming Defaults](#16-pwa--theming-defaults)
17. [Build & Embed Pipeline](#17-build--embed-pipeline)
18. [Testing Strategy](#18-testing-strategy)
19. [CLI Surface](#19-cli-surface)
20. [Configuration](#20-configuration)
21. [Implementation Order](#21-implementation-order)
22. [Decision Record](#22-decision-record)
23. [Explicit Deferrals](#23-explicit-deferrals)

---

## 1. Scope

C10 establishes the gohome web UI as a real, embedded, theme-architected React application served by `gohomed`, with a working dashboard subsystem (read + WYSIWYG edit + Pkl round-trip) and a community-extensible widget pack format. After C10, an operator can sign in to the embedded UI with credentials issued by C9, view their default dashboard, drag widgets around to edit it, save those edits back to Pkl source, and install community widget packs from OCI registries.

C10 is deliberately *not* a "ship the whole web app" milestone. It builds the foundation that every future page (entities, automations, events explorer, config editor, drivers UI, rich auth UI, settings) will layer on top of.

### 1.1 In scope

- **Embedded React 19 SPA** built with Vite, served by `gohomed` from `embed.FS` as hash-suffixed static assets, with a Vite dev-mode workflow for contributors.
- **App shell & routing** with TanStack Router: hybrid sidebar + thin top bar layout, mobile drawer reflow, theme toggle, current-route awareness.
- **Design token architecture** — CSS-custom-property substrate, Tailwind v4 `@theme` wiring, motion presets, language registry. Ships with a single language preset (`developer`); `friendly` and `ambient` are first-class extension targets the architecture must accommodate without component rewrites.
- **Tailwind v4 + shadcn/ui with enforced token discipline.** Custom ESLint rule bans raw color/radius/spacing utilities; copied-in shadcn primitives audited on intake.
- **Data layer** — Connect-ES generated client, TanStack Query for server-state, Zustand for UI-state, HttpOnly cookie session with Connect interceptor that refreshes on 401 via `AuthService.Refresh`, redirect to `/login` on refresh failure.
- **Live-stream multiplexer** holding two long-lived server streams: `EntityService.Subscribe` (typed state changes, server-policy-filtered) and `EventService.Tail` filtered to the command-lifecycle kinds (`CommandIssued`/`CommandAcknowledged`/`CommandFailed`). Cursor-resumable on reconnect. Single per-component subscription abstraction over both.
- **Pending-state UX pattern** — every command-issuing widget renders three visual states (idle / pending / settled) via a client-side command tracker keyed by `command_id` that joins the two streams.
- **Minimal `/login` page** — single form against C9's `AuthService.Login` (password) and `AuthService.StartWebAuthnChallenge` (passkey for already-registered credentials). No enrollment UI, no token UI, no user management.
- **Dashboard subsystem read path** — `DashboardService.Get(slug)` returns a typed `Dashboard` proto (compiled `WidgetInstance` list + grid spec + raw Pkl source for downstream "view source" use); `DashboardService.GetWidgetCatalog()` returns known widget classes (built-in + installed pack) with their prop schemas, JS bundle URLs/hashes, and signature verification status; renderer uses `react-grid-layout` and recursive component mounting through containers.
- **Dashboard subsystem edit path** — inline edit chrome (drag handle, resize handle, delete affordance, selection ring), right-side slide-in property panel, floating "+" widget picker; drag/drop into and out of containers; per-edit-session undo/redo; Discard / Save controls in the top bar.
- **Pkl round-trip via two-file split** — every WYSIWYG-managed dashboard is `dashboards/<slug>.pkl` (user-owned, hand-editable) plus `dashboards/<slug>.layout.pkl` (WYSIWYG-owned, regenerated canonically by a deterministic Pkl serializer on save). New dashboards scaffolded by the server. Dashboards without a `<slug>.layout.pkl` are detected and marked read-only-via-editor.
- **Widget contract** — Pkl `WidgetInstance` / `ContainerInstance` abstract classes, recursive children for containers (shipped now to avoid future schema migration), React component contract receiving a narrow `WidgetProps` SDK (state, pending, callCapability, runScript, evalCompute, instance, theme).
- **Eight built-in widgets** — `EntityToggle`, `Gauge`, `LineChart`, `CameraStream` (MJPEG), `Markdown`, `ScriptButton`, `EntityList`, `GroupCard` (the only container in v1.0).
- **Widget pack format** — OCI artifact (`ghcr.io/foo/bar-widgets:1.0.0`) containing `manifest.pkl`, `bundle.js`, optional README; sigstore (cosign) signing mandatory by default; install via `gohome widget install <oci-ref>`; runtime loading via dynamic ES `import()` from gohomed-served URL with `?h=<sha256>` cache key; lazy per-dashboard.
- **PWA shell-only** — service worker caches the app shell so the page loads on flaky LAN; manifest makes it installable; reconnecting banner when gohomed unreachable.
- **Theming defaults** — light/dark/system mode toggle persisted in `localStorage`; language locked to `developer` in v1.0 (switcher hidden until a second preset ships).
- **Compute-Starlark execution** — server-side via `ConfigService.EvalCompute(dashboard_id, widget_id, expr_id, state_snapshot)` RPC; v1.0 is naive recompute on each state-change frame the widget cares about; debouncing/batching deferred.
- **CLI surface** — `gohome widget {install,list,uninstall}`, `gohome ui dev`.
- **Configuration** — `gohome.web` and `gohome.widgetPackPolicy` Pkl modules.
- **Tests** — Vitest + Testing Library for components and hooks, Playwright E2E against a real gohomed binary, deterministic Pkl round-trip tests on the server, ESLint test for the token-discipline rule.

### 1.2 Out of scope

See §23 for the exhaustive deferral list with rationale. In short: every feature page beyond `/login` and `/dashboards/:slug`; rich auth management UI; OIDC login flow; offline state caching; push notifications; container widgets beyond `GroupCard`; widgets beyond the eight; widget pack version pinning; iframe sandboxing of pack JS; WebRTC for cameras; client-side Starlark; user-facing language switcher.

### 1.3 Inherited from the master design

- React 19 + Vite + TypeScript + Tailwind + shadcn/ui + Radix + Framer Motion + TanStack Query + TanStack Router + Connect-ES.
- Single binary, single port, embedded assets via `embed.FS`.
- Connect-RPC for all server communication; MCP runs alongside but is for agents, not the browser.
- Native mobile apps deferred indefinitely (PWA only).

### 1.4 Inherited from C9

C9 deferred to C10's web flows: server-side WebAuthn challenge storage and the passkey assertion verification that makes `AuthService.StartWebAuthnChallenge` and the passkey branch of `AuthService.Login` return real responses (both ship in C9 as `Code.Unimplemented` stubs). C10 implements this in `internal/auth/credentials/webauthn.go` and wires it through the existing `AuthService` impls. See §9.6.

---

## 2. Background

The master design's §7.3 names the web UI's full route inventory (`/dashboards`, `/entities`, `/automations`, `/events`, `/config`, `/drivers`, `/auth`, `/settings`, etc.) but the §10 child-doc roadmap allocates exactly one C-number for it: "C10 — Web UI Architecture: routing, data layer, live-stream multiplexer, dashboard editor, widget registry, widget pack format, theming, PWA." Treating that literally — and given C8/C9 each shipped specs of ~800–1,300 lines for narrower scopes — C10 must be the *foundation + dashboard subsystem*; feature pages are deliberate follow-on work that the foundation makes cheap.

Three commitments from the master design shape this whole spec:

1. **Event sourcing as the spine.** The browser does not write state directly. It calls `EntityService.CallCapability`, the server appends `CommandIssued`, the driver acts and emits `StateChanged`, the multiplexer delivers it back to the browser. The widget contract — particularly the pending-state UX (§8) — exists to make this loop honest in the UI rather than papering over it with optimistic local mutations.
2. **Pkl as the source of truth for dashboard layout.** A dashboard is not a database row; it is a Pkl module compiled to a typed proto. WYSIWYG editing is a *Pkl source mutation*, not a state update. The two-file split (§12) is the architecture that lets a deterministic regenerator coexist with hand-written Pkl.
3. **AI agents are first-class API consumers.** The browser is one client of the Connect-RPC + MCP surface; an MCP-driven agent is another. The UI must not invent endpoints that aren't also reachable from MCP. Anything the WYSIWYG editor does — including dashboard creation, widget insertion, and `SaveLayout` — must go through `DashboardService` RPCs that an agent could equally call.

C7 shipped the Connect-RPC surface that backs every fetch and stream the UI makes. C9 (in flight, depended on but not blocking design) is shipping the auth and policy enforcement that the browser's session cookie and per-RPC permissions ride on. C5 shipped the Starlark runtime that compute-Starlark in widget props will execute against. C4 shipped the Pkl evaluator and reload pipeline that the dashboard save path emits `ConfigApplied` events into.

---

## 3. Architecture Overview

### 3.1 Process and component map

```
                                                ┌────────────────────────────────────────┐
   Browser ──TLS──► gohomed :8080               │             gohomed                    │
                       ├── /healthz             │                                        │
                       ├── /webhooks/{slug}     │  ┌──────────────────────────────────┐  │
                       ├── /mcp                 │  │ HTTP mux (C7 listener)           │  │
                       ├── /assets/*            │  │  /                  ─► embed.FS  │  │
                       ├── /widgets/*           │  │  /assets/*          ─► embed.FS  │  │
                       └── /api/* (Connect-RPC) │  │  /widgets/<p>/<v>/  ─► widget    │  │
                                                │  │                       cache disk │  │
                                                │  │  /api/*             ─► Connect   │  │
                                                │  └────────┬─────────────────────────┘  │
                                                │           │                            │
                                                │  ┌────────▼─────────────────────────┐  │
                                                │  │ interceptor stack (C7+C9)        │  │
                                                │  │   authenticate, authorize        │  │
                                                │  └────────┬─────────────────────────┘  │
                                                │           │                            │
                                                │  ┌────────▼──────┐  ┌──────────────┐  │
                                                │  │ Connect svc   │  │ Dashboard    │  │
                                                │  │ implementations│  │ Pkl mutator │  │
                                                │  └────────┬──────┘  └──────┬───────┘  │
                                                │           │                │          │
                                                │  ┌────────▼────────────────▼───────┐  │
                                                │  │ eventstore + state + registry   │  │
                                                │  │  (C1) + config (C4) + automation│  │
                                                │  │  (C6) + auth (C9)               │  │
                                                │  └─────────────────────────────────┘  │
                                                └────────────────────────────────────────┘
```

The browser sees four route classes on `gohomed`'s HTTP listener:

- **`/`, `/assets/*`** — embedded React SPA. All non-`/api`, non-`/widgets`, non-`/healthz`, non-`/mcp`, non-`/webhooks` routes return `index.html` (TanStack Router does the rest client-side).
- **`/widgets/<pack>/<version>/<file>`** — installed widget pack assets, served from the on-disk widget cache (`~/.gohome/widgets/<pack>/<version>/`).
- **`/api/*`** — Connect-RPC. The browser uses Connect-protocol over HTTP/2 with binary protobuf (Connect-ES default for typed clients).
- **`/healthz`, `/webhooks/<slug>`, `/mcp`** — orthogonal, owned by other modules.

### 3.2 Internal browser-side modules

```
src/
├── main.tsx                  # entry: theme provider, router, query client, multiplexer
├── shell/                    # app chrome (sidebar, top bar, theme toggle, mobile drawer)
├── theme/                    # token system, language registry, motion presets
│   ├── tokens.css            # CSS custom properties (the substrate)
│   ├── languages/
│   │   └── developer.ts      # the only v1.0 language preset
│   └── provider.tsx          # ThemeProvider context
├── data/                     # data layer
│   ├── client.ts             # Connect-ES transport with auth interceptor
│   ├── query-client.ts       # TanStack Query setup
│   └── auth-store.ts         # Zustand: current user, session refresh state
├── multiplexer/              # the two-stream subscription manager
│   ├── multiplexer.ts        # holds Subscribe + filtered Tail; exposes useEntityState, usePending
│   └── command-tracker.ts    # joins CommandIssued/Acked/Failed to local pending state
├── routes/                   # TanStack Router file-based routes
│   ├── _authed/              # routes requiring authentication
│   │   └── dashboards/$slug.tsx
│   └── login.tsx             # the only public route
├── dashboard/                # rendering + edit
│   ├── render/               # read-only render path (grid + recursive widgets)
│   ├── edit/                 # WYSIWYG: handles, props panel, picker, undo/redo
│   └── catalog.ts            # widget registry (built-in + dynamic-imported packs)
├── widgets/                  # the eight built-ins
│   ├── EntityToggle.tsx
│   ├── Gauge.tsx
│   ├── LineChart.tsx
│   ├── CameraStream.tsx
│   ├── Markdown.tsx
│   ├── ScriptButton.tsx
│   ├── EntityList.tsx
│   └── GroupCard.tsx         # the only container in v1.0
├── widget-sdk/               # the public surface widget packs depend on
│   ├── index.ts              # re-exports types & hooks for pack authors
│   └── package.json          # published as @gohome/widget-sdk
└── pwa/                      # service worker, manifest helpers
```

The `widget-sdk` is the one part of this code published as a separate npm package — it's the contract third-party widget packs build against (with React + this SDK as externals).

### 3.3 Internal server-side modules added by C10

```
internal/
├── web/                      # serves embedded SPA + widget cache
│   ├── assets.go             # embed.FS, index.html template, asset hashing
│   ├── widgets_handler.go    # /widgets/<pack>/<v>/<file> handler
│   └── handler.go            # the /, /assets routing
├── dashboard/                # Pkl mutator + DashboardService impl
│   ├── service.go            # Get, GetWidgetCatalog, SaveLayout, Create, Delete
│   ├── catalog.go            # builds WidgetCatalog from compiled config + installed packs
│   ├── regen/                # deterministic Pkl serializer for *.layout.pkl
│   │   ├── regen.go
│   │   └── regen_test.go     # round-trip golden tests
│   └── scaffold.go           # creates new <slug>.pkl + <slug>.layout.pkl pairs
├── widgetpack/               # install, verify, list, serve
│   ├── install.go            # OCI pull + cosign verify + manifest validate + cache write
│   ├── store.go              # on-disk pack registry, version management
│   ├── catalog.go            # exposes installed packs to dashboard.catalog
│   └── trust.go              # consumes gohome.widgetPackPolicy
└── compute/                  # per-widget Starlark eval (C5 wrapped for dashboard use)
    └── service.go            # ConfigService.EvalCompute implementation
```

### 3.4 Public contracts introduced by C10

1. **`WidgetInstance` / `ContainerInstance` Pkl classes** — the schema users reference in dashboard Pkl. Recursive: container's `children` is `Listing<WidgetInstance>`. Versioned under `gohome.widgets` (semver).
2. **`@gohome/widget-sdk` npm package** — the React types and hooks third-party widget packs build against. Versioned (semver); a pack declares its required SDK version in `manifest.pkl`.
3. **Widget pack manifest format** — `gohome.widgets.PackManifest` Pkl class. Versioned; `protocol = "v1"` is the C10 baseline.
4. **`*.layout.pkl` regeneration shape** — the canonical Pkl shape the regenerator emits. Documented and golden-tested; users who hand-edit `*.layout.pkl` can rely on this shape being stable across gohomed versions within a major.

### 3.5 Data flow: a tap on a light toggle

End-to-end, illustrating how every architectural commitment ties together:

1. User taps `EntityToggle` for `light.kitchen`.
2. Component calls `props.callCapability("light.kitchen", "turn_on", {})`.
3. SDK invokes `EntityService.CallCapability` via Connect-ES; server appends `CommandIssued{command_id=C123, entity=light.kitchen}` to the event log.
4. The multiplexer's `EventService.Tail` (filtered to command lifecycle) delivers `CommandIssued` back to the browser. Command tracker registers `C123 → pending`.
5. The toggle component's `usePending("light.kitchen")` hook re-renders in pending state (subtle pulse, optimistic ON visual).
6. Hue driver (in this example) receives the command on its Carport stream, talks to the bridge, gets confirmation, emits `StateChanged{entity=light.kitchen, value=on, brightness=255}` and `CommandAcknowledged{command_id=C123}`.
7. `EntityService.Subscribe` delivers `StateChanged`; multiplexer updates entity state cache; toggle's `useEntityState("light.kitchen")` re-renders solid ON.
8. `EventService.Tail` (command lifecycle) delivers `CommandAcknowledged`; command tracker removes `C123`; pending visual fades out.
9. If a `CommandFailed` had arrived instead, the tracker would have surfaced an error toast and reverted the optimistic pending visual.

The server is authoritative throughout. The browser never asserts state it hasn't seen confirmed by the event log.

---

## 4. App Shell & Routing

### 4.1 Layout

**Hybrid sidebar + thin top bar**, rendered by `src/shell/Shell.tsx`:

```
┌──────────────────────────────────────────────────────────────┐
│ gohome    Default Dashboard                       ◑ ◯ ⚙      │  ← top bar (40 px)
├──────────┬───────────────────────────────────────────────────┤
│ ⌂ Dashboards│                                                │
│ ◇ Entities │            <Outlet />                           │
│ ▢ Devices  │            (route content)                      │
│ ⬡ Areas    │                                                 │
│ ↻ Automations│                                               │
│ ▷ Scripts  │                                                 │
│ ⊟ Events   │                                                 │
│ ⌥ Config   │                                                 │
│ ⚐ Drivers  │                                                 │
│            │                                                 │
│ v0.1.0     │                                                 │
└──────────┴───────────────────────────────────────────────────┘
```

In v1.0 the only nav items wired to working routes are **Dashboards** and the user menu; the rest are present-but-disabled placeholders (greyed, cursor:not-allowed, tooltip "Coming in C11"). This is deliberate: a working stub now establishes the nav structure that future milestones populate, and prevents the awkward "where do I add this nav item?" bikeshed every feature-page milestone would otherwise re-litigate.

### 4.2 Reflow

- **≥1024 px**: sidebar full-width (~180 px) with icon + label.
- **768–1023 px**: sidebar collapsed to icon-rail (~48 px); labels in tooltip on hover.
- **<768 px**: sidebar hidden; hamburger button in top bar opens drawer (Radix `Sheet`).

Reflow is governed by Tailwind breakpoints reading the `--gh-breakpoint-*` token set. Switching to a more compact density preset later (a future `friendly-compact` language) only changes the breakpoint values, not the markup.

### 4.3 Routing

TanStack Router file-based routing under `src/routes/`:

```
routes/
├── __root.tsx               # ThemeProvider, QueryClient, Multiplexer scope
├── login.tsx                # public; redirects to ?returnTo on success
└── _authed/                 # all routes under this require an auth session
    ├── _layout.tsx          # the Shell with sidebar + top bar
    ├── index.tsx            # redirects to /dashboards/default
    └── dashboards/
        └── $slug.tsx        # the only "feature" route in v1.0
```

`_authed/_layout.tsx` includes a router-level `beforeLoad` guard: if the auth-store's current-user is null, redirect to `/login?returnTo=<current-path>`. The `/login` page on success calls `auth.refresh()` and navigates to `returnTo` (defaulting to `/`).

All feature pages added in follow-on milestones land under `_authed/`, automatically picking up the shell + auth guard.

### 4.4 What's *not* in C10

- No global search palette (deferred — it requires every feature service to register search providers).
- No notifications panel (depends on push notifications, deferred to v1.x).
- No "what's new" / changelog UI.
- No in-app help (links to docs from a "?" button in the top bar are fine; in-app help system is later).

---

## 5. Design Tokens & Theming Architecture

This is the most architecturally consequential section in C10. The decision to ship one design language now (`developer`) while holding open `friendly` and `ambient` as future-additive presets means every visual primitive must read tokens, never hardcode.

### 5.1 Token taxonomy

Tokens are declared as CSS custom properties on `:root` (and overridden on `[data-theme]` selectors). Categories:

| Category | Sample tokens | Notes |
|---|---|---|
| **Palette** | `--gh-color-bg`, `--gh-color-surface-1`, `--gh-color-surface-2`, `--gh-color-border`, `--gh-color-fg`, `--gh-color-fg-muted`, `--gh-color-accent`, `--gh-color-success`, `--gh-color-warning`, `--gh-color-danger` | Semantic, not color-named. `--gh-color-accent` becomes purple in `friendly`, cyan in `developer`. |
| **Radii** | `--gh-radius-sm`, `--gh-radius-md`, `--gh-radius-lg`, `--gh-radius-pill` | `developer`: 3/5/8/999 px. `friendly`: 8/12/16/999 px. `ambient`: 12/16/24/999 px. |
| **Density / spacing** | `--gh-pad-tight`, `--gh-pad-normal`, `--gh-pad-loose`, `--gh-gap-tight`, `--gh-gap-normal`, `--gh-gap-loose` | Component padding/gap reference these. |
| **Surface treatment** | `--gh-surface-flat`, `--gh-surface-elev-1`, `--gh-surface-elev-2`, `--gh-surface-blur` | Compose `background`, `border`, `box-shadow`, `backdrop-filter`. `developer` uses flat + hairline border; `friendly` uses subtle shadow; `ambient` uses backdrop-blur. |
| **Motion** | `--gh-motion-snappy`, `--gh-motion-spring`, `--gh-motion-slow` | Stored as `cubic-bezier()` strings + duration. Framer Motion presets read these. |
| **Type** | `--gh-font-display`, `--gh-font-body`, `--gh-font-numeric`, `--gh-text-xs`, `--gh-text-sm`, `--gh-text-md`, `--gh-text-lg`, `--gh-text-xl`, `--gh-weight-normal`, `--gh-weight-medium`, `--gh-weight-bold` | `developer` uses Inter for display, JetBrains Mono for numerics; `friendly` uses Geist; `ambient` uses ultralight Inter. |
| **Breakpoints** | `--gh-bp-sm`, `--gh-bp-md`, `--gh-bp-lg`, `--gh-bp-xl` | Read by Tailwind via the `screens` plugin. |

The token *names* are the contract. Every value above can change between language presets and across mode (light/dark) without breaking any component.

### 5.2 CSS substrate and Tailwind wiring

`src/theme/tokens.css` declares the developer light + dark token sets:

```css
:root, [data-theme="developer-light"] {
  --gh-color-bg: oklch(98% 0 0);
  --gh-color-surface-1: oklch(100% 0 0);
  --gh-color-accent: oklch(64% 0.16 220);
  --gh-radius-md: 5px;
  --gh-pad-normal: 0.5rem;
  --gh-motion-snappy: 200ms cubic-bezier(0.4, 0, 0.2, 1);
  --gh-font-numeric: "JetBrains Mono", ui-monospace, monospace;
  /* ... */
}

[data-theme="developer-dark"] {
  --gh-color-bg: oklch(11% 0 0);
  --gh-color-surface-1: oklch(14% 0 0);
  --gh-color-accent: oklch(75% 0.13 220);
  /* radii, density, motion, type unchanged from light */
}
```

`tailwind.config.ts` reads the token names via `theme.extend`:

```ts
export default {
  theme: {
    extend: {
      colors: {
        bg: 'var(--gh-color-bg)',
        'surface-1': 'var(--gh-color-surface-1)',
        accent: 'var(--gh-color-accent)',
        // ...
      },
      borderRadius: {
        sm: 'var(--gh-radius-sm)',
        md: 'var(--gh-radius-md)',
        lg: 'var(--gh-radius-lg)',
      },
      spacing: {
        tight: 'var(--gh-pad-tight)',
        normal: 'var(--gh-pad-normal)',
        loose: 'var(--gh-pad-loose)',
      },
    },
  },
}
```

Components write `bg-surface-1 rounded-md p-normal` — never `bg-zinc-900 rounded-[5px] p-2`.

### 5.3 Token discipline (the lint rule)

A custom ESLint rule, `gohome/no-raw-tokens`, flags class names that bypass the token system:

- Banned color utilities: any from Tailwind's default palette (`bg-zinc-*`, `text-cyan-*`, etc.) — only the token-mapped utilities (`bg-bg`, `bg-surface-1`, `text-fg`, `text-accent`, etc.) pass.
- Banned radius utilities: `rounded-[Npx]`, `rounded-none` (use `rounded-sm` mapping to `0` if needed via tokens).
- Banned spacing arbitrary values: `p-[Npx]`, `gap-[Nrem]`, etc.
- Banned style-prop usage: `style={{ borderRadius: 5 }}` triggers the rule.

The rule lives in `tools/eslint-plugin-gohome/`. CI runs lint with `--max-warnings 0`. Allowed escape hatch: a per-line `// eslint-disable-next-line gohome/no-raw-tokens — reason: <text>` comment with a mandatory reason string. A separate audit task (in §21) enumerates the legitimate exceptions and keeps them under 10 occurrences.

### 5.4 Language registry

`src/theme/languages/` holds one file per language preset. Each exports:

```ts
export type LanguagePreset = {
  id: string;                 // "developer"
  modes: {
    light: TokenSet;
    dark: TokenSet;
  };
  motion: {
    snappy: MotionPreset;
    spring: MotionPreset;
    slow: MotionPreset;
  };
  fonts: { display: string; body: string; numeric: string };
};
```

The `ThemeProvider` reads `(language, mode)` from the auth-store + `localStorage` and writes the corresponding `data-theme` attribute on `<html>`. CSS handles the rest via the variable cascade.

`developer` is the only language registered in v1.0. Adding `friendly` later is a single new file in this directory; no component touches needed. Adding `ambient` is the same — its `surface` tokens use `backdrop-filter: blur()`, but every component already composes surfaces via the token, so they pick it up automatically.

### 5.5 Motion presets

Framer Motion configurations as named exports in `src/theme/motion.ts`, reading from CSS vars at runtime:

```ts
export const motion = {
  snappy: { type: "tween", duration: 0.2, ease: [0.4, 0, 0.2, 1] },
  spring: { type: "spring", damping: 24, stiffness: 320 },
  slow:   { type: "tween", duration: 0.5, ease: [0.16, 1, 0.3, 1] },
};
```

In v1.0 these are static (no var-derivation) since `developer` only needs `snappy`. When `friendly` arrives, motion presets become language-derived (the file becomes `language.motion`). Components reference `motion.snappy`, never raw `transition` configs.

### 5.6 shadcn/ui integration

shadcn primitives are *copied into* the project, not imported as a library. C10's intake protocol for any shadcn primitive:

1. `npx shadcn-ui@latest add <component>` lays the file into `src/components/ui/`.
2. **Token audit pass** — find every raw color, radius, spacing utility and replace with token utility. Re-run the lint rule until clean.
3. Add a snapshot test at `src/components/ui/<component>.test.tsx` that renders the component in `developer-light` and `developer-dark` and asserts the rendered class names contain `bg-surface-*` patterns (smoke test for token compliance).

A small wrapper layer at `src/components/gh/` sits above shadcn primitives and is the only public import surface for the rest of the app. This insulates against future shadcn API shifts and provides a stable place to layer in motion presets.

### 5.7 Widget SDK theme exposure

`WidgetProps.theme` exposes the active token set as a typed object:

```ts
export type Theme = {
  language: "developer";   // future: | "friendly" | "ambient"
  mode: "light" | "dark";
  tokens: {
    color: { bg: string; surface1: string; surface2: string; accent: string; /* ... */ };
    radius: { sm: string; md: string; lg: string };
    motion: { snappy: MotionPreset; spring: MotionPreset; slow: MotionPreset };
  };
};
```

Tokens are passed as the resolved CSS variable references (`var(--gh-color-accent)`), not literal values. Widget pack code can use them as either:

- Tailwind utility composition via the same `bg-surface-1` etc. (works because gohomed serves the bundle into the same DOM where the global stylesheet is loaded).
- Inline style (`style={{ background: theme.tokens.color.surface1 }}`) for cases where dynamic computation is needed.

The SDK does *not* expose raw color values, prevents widgets from defining their own palette, and disallows widgets from setting global CSS. Containment is by convention; v1.x iframe sandboxing would enforce it.

### 5.8 Disqualifying patterns

Patterns explicitly banned in any code under `src/` (enforced by lint where mechanizable, code-review otherwise):

- Hardcoded color literals (`#fff`, `oklch(...)`, named colors) outside `src/theme/`.
- Raw Tailwind utilities from the default palette outside `src/theme/`.
- Inline `style={{ borderRadius: 5 }}`, `style={{ padding: '0.5rem' }}` etc.
- `transition-all`, `transition-colors` (use motion presets).
- `font-mono`, `font-sans` Tailwind utilities (use `font-numeric`, `font-body` token-mapped utilities).
- Per-component palette logic (`isDark ? 'bg-zinc-900' : 'bg-white'`) — let the token cascade do this.

---

## 6. Data Layer

### 6.1 Connect-ES transport

A single `Transport` instance, created in `src/data/client.ts`:

```ts
export const transport = createConnectTransport({
  baseUrl: "/api",
  useBinaryFormat: true,        // protobuf binary, not JSON
  interceptors: [authInterceptor],
});
```

Service-specific clients (`createPromiseClient(EntityService, transport)`, etc.) are constructed lazily where needed and memoized.

### 6.2 Auth interceptor

`src/data/client.ts` defines `authInterceptor`:

- Every outgoing request rides on the HttpOnly session cookie set by `AuthService.Login` (browser handles this automatically; the interceptor sets `credentials: "include"` on the underlying `fetch`).
- On a `Code.Unauthenticated` response, the interceptor:
  1. Acquires a per-process refresh lock (so concurrent in-flight requests don't all trigger `Refresh`).
  2. Calls `AuthService.Refresh()` once.
  3. On success, retries the original request once.
  4. On failure (refresh rejected, no refresh cookie, network error after retry), clears the auth-store's current user and triggers a router navigation to `/login?returnTo=<current path>`.

The retry is single-shot. If the retried request still fails with `Unauthenticated`, the user is redirected to login regardless. This avoids retry storms.

### 6.3 TanStack Query

Standard config:

```ts
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,                    // server data is generally fresh
      refetchOnWindowFocus: false,          // multiplexer keeps state live
      retry: (failureCount, error) =>
        !isUnauthenticated(error) && failureCount < 2,
    },
  },
});
```

Query key conventions: `["dashboard", slug]`, `["dashboard-catalog"]`, `["entities", filter]`, `["current-user"]`. Mutations invalidate matching keys on success.

A `ConfigApplied` event from the multiplexer triggers `queryClient.invalidateQueries({ queryKey: ["dashboard"] })` and `["dashboard-catalog"]` so any other open browser tab refetches when config changes.

### 6.4 Auth store (Zustand)

`src/data/auth-store.ts`:

```ts
type AuthState = {
  user: CurrentUser | null;
  refreshing: boolean;
  refreshLock: Promise<void> | null;
};
type AuthActions = {
  bootstrap(): Promise<void>;          // called on app start, hits CurrentUser
  refresh(): Promise<void>;            // serialized via refreshLock
  logout(): Promise<void>;
};
```

The auth store is the single source of truth for "is the user logged in." Router guards and the auth interceptor both consult it.

### 6.5 Error handling

Three error tiers:

- **Network / transport errors**: surface as toasts; queries retry up to twice; mutations don't retry.
- **`Code.Unauthenticated`**: handled by the interceptor (see §6.2).
- **`Code.PermissionDenied`**: surfaced as an inline message in the affected component ("This action isn't allowed by your current policy"). Calls `AuthService.ExplainAuthorization` for richer detail when available (C9 RPC).
- **`Code.FailedPrecondition` and validation errors** (e.g., `SaveLayout` failing because the resulting Pkl doesn't validate): surfaced inline in the editor with the server's error message.

A top-level `ErrorBoundary` catches any uncaught render errors and shows a "Something went wrong — reload" screen with the error message + a copy-to-clipboard button. Per-widget error boundaries (§13) prevent one bad widget from killing the dashboard.

---

## 7. Multiplexer

### 7.1 Why two streams

Per Q4 of the brainstorm, the multiplexer holds two long-lived server streams rather than the single raw `EventService.Tail` mentioned in master design §7.3:

- **`EntityService.Subscribe`** delivers `StateChanged | EntityRegistered | EntityUnregistered` typed and server-policy-filtered. This is the primary data feed for widgets.
- **`EventService.Tail` filtered to `kind IN ("command_issued", "command_acked", "command_failed")`** delivers command lifecycle events for the pending-state UX (§8). The filter is server-side so we don't ship every event to every browser.

Both are server-streaming Connect-RPCs over HTTP/2. The browser sees a single `useEntityState(entityId)` / `usePending(entityId)` hook abstraction; the two-stream plumbing is internal.

### 7.2 Subscription scope

The selector for `EntityService.Subscribe` is the **union of `referencedEntityIds` across all visible widgets in the current route**, computed each time the dashboard mounts and each time the visible widget set changes. The Subscribe RPC supports `from_cursor=0` to get current state on initial load (per C7 §4.5), so the first frame after mount includes the initial state for every referenced entity.

When the user navigates between dashboards, the multiplexer recomputes the union and issues an updated Subscribe RPC (server-side: cancel old subscription, start new). The brief (sub-100ms) gap is acceptable — entity state is local-cached during the swap.

### 7.3 Cursor and reconnect

Each stream maintains a `lastCursor` (monotonic event position). On disconnect:

1. Exponential backoff: 100ms, 200ms, 400ms, ..., capped at 10s.
2. On retry, resume with `from_cursor=lastCursor + 1`.
3. The reconnecting banner appears after the first failed retry (~100ms delay so transient blips don't flash).
4. Successful reconnect emits an event the auth-store consumes to drop the banner.
5. If a reconnect attempt returns `Code.Unauthenticated`, the standard auth-interceptor flow takes over (refresh, then retry, then redirect to login).

### 7.4 Fan-out

Internally the multiplexer maintains:

- `Map<EntityId, EntityState>` — current state cache, fed by Subscribe.
- `Map<CommandId, PendingCommand>` — pending command tracker, fed by Tail-filtered.
- `Set<Subscriber>` per entity / per command — components subscribe via React hooks; on update, only affected subscribers re-render.

Subscribers are weak: when a component unmounts its hook teardown removes the subscription. No reference counting; entries in the state cache persist (they're cheap and the next mount benefits from cache hit).

### 7.5 Backpressure

In practice not a concern — the home automation event rate is well below browser-render rates even with hundreds of entities. v1.0 makes no attempt at coalescing or batching. If we ever measure thrash from extreme rates, a coalescing layer (one render per animation frame max per entity) is a localized addition.

### 7.6 Cross-tab sync

If a user opens two tabs, each runs its own multiplexer with its own pair of streams. Both see the same events and stay in sync naturally. No `BroadcastChannel` plumbing needed for v1.0. (`ConfigApplied` invalidating dashboard queries — §6.3 — is what makes a layout edit in tab A appear in tab B.)

---

## 8. Pending-State UX Pattern

Per Q3, the chosen reconciliation model is **pending-state hybrid**: command-issuing widgets render three visual states (idle / pending / settled) driven by the multiplexer's command tracker.

### 8.1 The contract

Every command-issuing widget calls `props.callCapability(entityId, capability, args)`. The SDK returns `{ command_id }` immediately (the server's response to `EntityService.CallCapability` is the `CommandIssued` event metadata, not the command's outcome).

The widget then reads `props.pending[entityId]` (or `props.pending[entityId][capability]` for finer-grained widgets) which the multiplexer populates from the command tracker. Three states, exposed as a discriminated union:

```ts
type PendingState =
  | { state: "idle" }
  | { state: "pending"; commandId: string; sinceMs: number }
  | { state: "failed"; commandId: string; error: string; ageMs: number };
```

Widgets render:
- **`idle`** — normal state from `props.state[entityId]`.
- **`pending`** — same state but with a pending visual treatment (subtle pulse via Framer Motion, slightly muted color); show optimistic target value if known. Auto-clears when the matching `StateChanged` arrives or the command tracker times out (default: 5s).
- **`failed`** — last state with a danger badge; auto-clears after 3s. A toast surfaces the error message.

### 8.2 Command tracker semantics

`src/multiplexer/command-tracker.ts` rules:

- Register: `EventService.Tail` delivers `CommandIssued{command_id, entity_id, capability, args}` → tracker creates `pending` entry.
- Settle: `CommandAcknowledged{command_id}` → tracker removes entry (state convergence is signalled by the `StateChanged` from the Subscribe stream).
- Fail: `CommandFailed{command_id, error}` → tracker marks `failed` for 3s, then removes.
- Timeout: pending entries older than 5s with no Acked/Failed → tracker marks `failed{error: "Driver did not acknowledge command"}`. This is rare (driver crashed mid-command) but the UI should not spin forever.

### 8.3 Multiple concurrent pendings on one entity

If a user double-taps a toggle, two `CommandIssued` events register; the tracker holds both. The visual stays "pending" until both settle. This is intentional — the user shouldn't see the visual flip mid-double-tap.

If a user issues `turn_on` and then immediately `set_brightness` on the same entity, both pending; the tracker keys by `command_id`, not by entity. The widget can choose to render the union ("pending: 2 commands in flight") or just the most recent.

### 8.4 Why not optimistic

Optimistic local mutation would feel snappier but conflicts with the "event log is the truth" architectural commitment. A user toggling a light that fails (driver disconnected, bridge offline) would see the UI lie before reverting — eroding trust. Pending-state honestly says "I'm trying" while still feeling responsive.

---

## 9. Login Flow

### 9.1 Page

`src/routes/login.tsx`. A single form with:

- Username field.
- Password field.
- "Use passkey instead" button.
- Submit button.
- Error region for failed attempts.

No "create account," no "forgot password," no "register passkey," no "OIDC sign in." Those land in the future rich auth UI milestone.

### 9.2 Password flow

1. Form submit → `AuthService.Login({ username, password })`.
2. On success, server sets HttpOnly session cookie + refresh cookie; response includes `{ user }`.
3. `auth-store.user = user`; navigate to `returnTo` query param (default `/`).
4. On `Code.Unauthenticated` or `Code.InvalidArgument`, show a generic "Invalid credentials" error (don't reveal whether username exists).
5. On `Code.PermissionDenied` (e.g., user disabled), show "Your account is disabled. Contact an administrator."

### 9.3 Passkey flow

1. "Use passkey" button → `AuthService.StartWebAuthnChallenge()` returns a WebAuthn challenge.
2. Browser calls `navigator.credentials.get({ publicKey: challenge })`.
3. On success, response asserted via `AuthService.Login({ webauthnAssertion })`.
4. Same cookie/store/navigation flow as password.
5. Errors (no registered credentials for this device, user cancelled, browser doesn't support WebAuthn) surface inline.

### 9.4 Logout

A "Sign out" item in the user menu (top bar) calls `AuthService.Logout()`, clears the auth-store, navigates to `/login`. The server invalidates the session cookie.

### 9.5 What this isn't

This is enough flow for a household-declared user with credentials already provisioned (via `gohome auth bootstrap`, password hash in their Pkl, or pre-registered passkey via CLI) to sign into the browser. Onboarding flows — first-run wizard, passkey enrollment ceremony, OIDC SSO — are not C10.

### 9.6 Inherited from C9 — server-side passkey machinery

C9 shipped its `AuthService.StartWebAuthnChallenge` and the passkey branch of `AuthService.Login` returning `Code.Unimplemented`, deferring "server-side challenge storage" to C10's web flows. C10 picks this up: an in-process challenge store keyed by session id, time-bounded (5 min default), cleared on consume. The store is internal to `internal/auth/credentials/webauthn.go` (a package C9 already created); C10 fills in the `StoreChallenge` / `ConsumeChallenge` operations and wires them through the `AuthService` impls so the passkey path actually returns the WebAuthn challenge and verifies the assertion. The browser side described in §9.3 above depends on this inherited work landing in the same milestone.

---

## 10. Dashboard Render Path

### 10.1 The shape returned by `DashboardService.Get(slug)`

```proto
message Dashboard {
  string slug = 1;
  string title = 2;
  Grid grid = 3;
  repeated WidgetInstance widgets = 4;     // recursive — see §13
  string source_pkl = 5;                   // the user-owned <slug>.pkl content
  string layout_pkl = 6;                   // the WYSIWYG-owned <slug>.layout.pkl content
  bool wysiwyg_writable = 7;               // false if no <slug>.layout.pkl exists
}
```

`source_pkl` and `layout_pkl` ride along on the read response so the future "view source" toggle and the edit path don't need additional RPCs. Both fields are small (typically <10 KB).

`wysiwyg_writable=false` triggers a UI banner: "This dashboard's layout is hand-written. Editing here is disabled. Edit `dashboards/<slug>.pkl` directly."

### 10.2 The shape returned by `DashboardService.GetWidgetCatalog()`

```proto
message WidgetCatalog {
  repeated WidgetClass classes = 1;
}
message WidgetClass {
  string class_id = 1;             // "EntityToggle", "bar-widgets/BarChart"
  bool is_container = 2;
  bool is_builtin = 3;
  string pack_name = 4;            // empty for built-ins
  string pack_version = 5;
  string bundle_url = 6;           // empty for built-ins
  string bundle_hash = 7;          // sha256
  PropSchema prop_schema = 8;      // JSON-Schema-like description for the editor
  SignatureStatus signature = 9;   // VERIFIED, UNSIGNED, INVALID, EXPIRED
}
```

The catalog is cacheable — it changes only on `WidgetPackInstalled` / `WidgetPackUninstalled` events. The browser caches via TanStack Query; the multiplexer invalidates the cache key when those events arrive.

### 10.3 Render flow

`src/dashboard/render/DashboardView.tsx`:

```tsx
function DashboardView({ slug }: { slug: string }) {
  const { data: dashboard } = useQuery({ queryKey: ["dashboard", slug], ... });
  const { data: catalog }   = useQuery({ queryKey: ["dashboard-catalog"], ... });

  // Eager-import packs referenced by visible widgets
  const usedPacks = useMemo(() => collectUsedPacks(dashboard, catalog), [dashboard, catalog]);
  const packsReady = useDynamicImports(usedPacks);                 // suspends until loaded

  if (!packsReady) return <DashboardSkeleton />;

  return (
    <Multiplexer entityIds={collectReferencedEntities(dashboard.widgets)}>
      <Grid spec={dashboard.grid}>
        {dashboard.widgets.map(w => <WidgetRenderer key={w.id} instance={w} />)}
      </Grid>
    </Multiplexer>
  );
}
```

`WidgetRenderer` recursively dispatches: container instances render their child grid + child widgets; leaf instances render their component from the registry, mounted with `WidgetProps`.

### 10.4 Initial state

The `Multiplexer` component opens (or replaces) the `EntityService.Subscribe` selector to the union of entity IDs. With `from_cursor=0`, the first frame includes current state for every entity — so widgets render immediately with real state, not a loading spinner.

### 10.5 Per-widget error boundary

Each rendered widget is wrapped in a per-widget `ErrorBoundary` (`src/dashboard/render/WidgetErrorBoundary.tsx`):

```tsx
<WidgetErrorBoundary widgetId={instance.id} widgetClass={instance.classId}>
  <WidgetRenderer instance={instance} />
</WidgetErrorBoundary>
```

A crashing widget renders an in-grid error tile with the widget id, class, error message, and a "Retry" button. The dashboard stays usable. Errors are reported to a per-session log a future `gohome diag` collector can pick up.

---

## 11. Dashboard Edit Mode

Per Q6: **inline chrome + slide-in props panel + FAB widget picker**.

### 11.1 Entering / leaving edit mode

A "Edit" button in the top bar (visible only on dashboard routes, only for users with the relevant policy) toggles edit mode. Entering:

1. Replace the top bar's right-side controls with `Discard` + `Save` buttons and an "Edit mode" badge.
2. Clone the current `Dashboard` proto into editor state (`useDashboardEditor` Zustand store).
3. Add `editing` class on the grid root → CSS reveals drag handles, resize handles, delete affordances on every tile.
4. FAB ("+") appears bottom-right; opens the widget picker (Radix `Sheet`).

Leaving (Discard, Save, route navigation):

- **Discard** with unsaved changes → confirm dialog. On confirm, drop editor state, exit edit mode.
- **Save** → see §12.
- **Route navigation away** with unsaved changes → confirm dialog ("Discard your unsaved layout changes?").
- **Network loss during edit** → reconnecting banner shows; editor state preserved locally; user can keep editing offline; Save retries when reconnected.

### 11.2 Drag / drop / resize

Powered by `react-grid-layout` with `isDraggable={true}`, `isResizable={true}`, `compactType={null}` (no auto-pack — users want positional fidelity). Each `react-grid-layout` event (drag stop, resize stop) updates the editor state's widget `pos`. Container widgets (§13) host their own nested `react-grid-layout` instance with the same config.

Drag-into-container detection: when a drag's drop coordinates fall within a container's bounds, the dropped widget is moved from the parent's `widgets` to the container's `children` (and its `pos` reinterpreted in container coords). Drag-out is the inverse.

### 11.3 Widget picker (FAB)

The FAB opens a `Sheet` showing widget classes from `WidgetCatalog`, grouped by:

- Built-ins (the eight)
- Installed packs (one section per pack)

Each entry shows an icon, class name, and short description. Click → widget appears at the next available grid slot in the current container (or page-level grid if no container is selected). Edit selection then auto-jumps to the new widget so the user can immediately tune props.

### 11.4 Selection and props panel

Clicking a tile in edit mode selects it (yellow ring per the mockup). The right-side `Sheet` slides in (closeable, but auto-opens on first selection per session) showing the widget's properties.

Property fields are generated from the widget class's `PropSchema`:

| `PropSchema.type` | Field type |
|---|---|
| `string` (free) | `<input type="text">` |
| `string` (enum) | `<select>` |
| `string` (entity-ref) | Entity picker (autocomplete over `EntityService.List`) |
| `number` | `<input type="number">` with min/max if specified |
| `boolean` | `<Switch>` |
| `starlark-expr` | `<textarea>` with monaco-lite syntax highlighting (Monaco lazy-imported) |
| `list<string>` | Tag-style multi-input |
| `list<entity-ref>` | Multi-entity picker |

Container widgets show "Children: N" and a button to "Edit children" which closes the props panel and returns focus to the canvas.

### 11.5 Undo / redo

Editor state is a Zustand store with built-in history (push on every mutation, max 50 entries, debounced 200ms for drag operations to avoid history flooding). Cmd/Ctrl-Z and Cmd/Ctrl-Shift-Z bound globally while in edit mode. A "history dot" indicator in the top bar shows pending undoable changes.

### 11.6 Save flow

Save click → `DashboardService.SaveLayout({ slug, dashboard })` (the editor's current dashboard proto). Server: §12. On success: editor state cleared, exit edit mode, dashboard query invalidated → re-renders from new server state. On failure: error inline in the top bar (e.g., "Pkl validation failed: <error>"); editor state preserved so user can fix.

### 11.7 New dashboard creation

A "New dashboard" button under the dashboards nav item opens a modal asking for slug + title. On confirm: `DashboardService.Create({ slug, title })` → server scaffolds `<slug>.pkl` + `<slug>.layout.pkl` (see §12.5) → response with the new Dashboard proto → router navigates to `/dashboards/<slug>` in edit mode.

### 11.8 Delete

Right-click on a dashboard in the sidebar → context menu with "Delete." Confirm dialog. `DashboardService.Delete({ slug })` deletes both the user-owned `.pkl` and the WYSIWYG-owned `.layout.pkl`. Hand-only dashboards (no `.layout.pkl`) can also be deleted but with an extra confirmation step ("This dashboard's layout is hand-written. Delete the source file too?").

---

## 12. Pkl Round-Trip

The architectural commitment from Q7: **two-file split**.

### 12.1 File layout

For every WYSIWYG-managed dashboard:

```
gohome/dashboards/
├── default.pkl          # USER-OWNED — hand-editable, never written by server
└── default.layout.pkl   # WYSIWYG-OWNED — regenerated canonically on every Save
```

### 12.2 Canonical user-owned `<slug>.pkl` (scaffold output)

```pkl
@ModuleInfo { minPklVersion = "0.27.0" }
module gohome.user.dashboards.default

import "@gohome/dashboards.pkl" as d
import "default.layout.pkl" as layout

dashboard: d.Dashboard = new {
  slug    = "default"
  title   = "Home"
  grid    = layout.grid
  widgets = layout.widgets
}
```

This file is created once by the scaffold and never touched again by the server. The user is free to add imports, helpers, conditional widget construction, etc., as long as `dashboard: d.Dashboard` resolves. (If they want WYSIWYG to keep working, they should keep the `import "default.layout.pkl" as layout` line and the `widgets = layout.widgets` reference so the editor's writes flow through. Renaming the import alias is fine; the scaffold detection only needs the file to exist.)

### 12.3 Canonical WYSIWYG-owned `<slug>.layout.pkl` (regenerator output)

```pkl
@ModuleInfo { minPklVersion = "0.27.0" }
module gohome.layouts.default

import "@gohome/dashboards.pkl" as d
import "@gohome/widgets.pkl" as w

grid: d.Grid = new {
  columns   = 12
  rowHeight = 60
}

widgets: Listing<d.WidgetInstance> = new {
  new w.GroupCard {
    id = "living-room"
    pos = new { x = 0; y = 0; w = 6; h = 4 }
    title = "Living Room"
    childGrid = new { columns = 6; rowHeight = 60 }
    children = new {
      new w.EntityToggle {
        id = "kitchen-light"
        pos = new { x = 0; y = 0; w = 2; h = 1 }
        entityId = "light.kitchen"
      }
      new w.Gauge {
        id = "outdoor-temp"
        pos = new { x = 2; y = 0; w = 2; h = 1 }
        entityId = "sensor.outdoor"
        unit = "°F"
      }
    }
  }
  new w.LineChart {
    id = "power-24h"
    pos = new { x = 6; y = 0; w = 6; h = 2 }
    entityIds = new { "sensor.power" }
    rangeHours = 24
  }
}
```

### 12.4 Regenerator rules

`internal/dashboard/regen/regen.go` produces `*.layout.pkl` from a typed `Dashboard` proto. Rules:

- **Deterministic.** Same input proto → byte-identical output. (Property order: `id`, `pos`, then class-specific props alphabetical. Widget order: as-given by the proto, which is the user's drag order.)
- **No comments, no blank lines within widget definitions.** The file is machine-owned; humans who read it see a uniform shape.
- **One blank line between top-level widgets** (visual breathing room for git diffs).
- **2-space indentation.** Pkl is whitespace-insensitive but the canonical shape is fixed.
- **String literals always double-quoted.** No identifier shorthand even where Pkl allows it.
- **Numbers preserved as-given.** Integers stay integers; floats keep their precision.
- **Starlark snippets pass through as raw strings** (`compute = "avg(state('sensor.x'))"`). The regenerator does no Starlark parsing.

Output goes through Pkl evaluator validation before write — if the regenerated file fails to parse, the save is rejected and the on-disk file is untouched.

### 12.5 Scaffold

`DashboardService.Create({ slug, title })`:

1. Reject if `<slug>.pkl` or `<slug>.layout.pkl` already exists.
2. Write canonical `<slug>.pkl` per §12.2 (with the requested title interpolated).
3. Write canonical `<slug>.layout.pkl` per §12.3 with `widgets = new {}` and a default 12-col grid.
4. Run `gohome config validate` (programmatic, in-process). If invalid, roll back and return the error.
5. Emit `ConfigApplied` event.
6. Return the Dashboard proto.

### 12.6 Save

`DashboardService.SaveLayout({ slug, dashboard })`:

1. Confirm `<slug>.layout.pkl` exists (else return `wysiwyg_not_writable`).
2. Confirm the `dashboard.slug` matches `slug` (defense against client bugs).
3. Run regenerator over `dashboard.widgets` + `dashboard.grid` → new `<slug>.layout.pkl` content.
4. Validate the new content via the Pkl evaluator (loads the user-owned `<slug>.pkl` referencing the new layout). If invalid, return `Code.FailedPrecondition` with the evaluator error.
5. Atomic file write: write to `<slug>.layout.pkl.tmp` then rename to `<slug>.layout.pkl`.
6. Re-trigger config compile (the existing C4 `gohome config apply` programmatic path).
7. Emit `ConfigApplied` event with the dashboard slug as a metadata field.
8. Return the new Dashboard proto.

### 12.7 Round-trip golden tests

`internal/dashboard/regen/regen_test.go` table-driven golden tests:

- A set of input `Dashboard` proto fixtures.
- For each, the expected `<slug>.layout.pkl` text byte-for-byte.
- A round-trip assertion: regenerate → re-parse via Pkl evaluator → assert the parsed `Dashboard` matches the input proto.

Tests cover: empty layout, single widget, nested containers, deeply nested (3 levels), Starlark snippets in props, special characters in string props, very long entity ID lists.

### 12.8 Detecting hand-only dashboards

Server scans the dashboards config for any `Dashboard` whose `slug` matches a `<slug>.pkl` file but has no corresponding `<slug>.layout.pkl`. Those are marked `wysiwyg_writable=false` in `Get` responses; the editor disables Save for them.

### 12.9 What happens if a user hand-edits `<slug>.layout.pkl`

The next WYSIWYG save will overwrite it. This is the contract — `<slug>.layout.pkl` is server-owned. A future `gohome config diff` (deferred) could warn about pending changes to layout files before save. For v1.0, the file's preamble comment makes this clear:

```pkl
@ModuleInfo { minPklVersion = "0.27.0" }
/// Generated by gohome WYSIWYG editor. Do not hand-edit — changes will be
/// overwritten on the next dashboard save. Edit dashboards/<slug>.pkl instead.
module gohome.layouts.default
```

The doc-comment is preserved across regenerations (it's part of the canonical template, emitted unconditionally).

---

## 13. Widget Contract

### 13.1 Pkl side

The `gohome.widgets` Pkl module declares the abstract base classes and the eight v1 built-ins.

```pkl
module gohome.widgets

import "@gohome/dashboards.pkl" as d

abstract class WidgetInstance {
  id: String(matches(Regex(#"^[a-z][a-z0-9_-]{0,63}$"#)))
  pos: d.Position
  /// Entity IDs this widget reads. Multiplexer subscription is the union over visible widgets.
  /// Required: every concrete subclass must assign a value (uses `List<String>` value semantics
  /// so the default `flatMap`+`distinct` composition in `ContainerInstance` works cleanly).
  referencedEntityIds: List<String>
}

abstract class ContainerInstance extends WidgetInstance {
  childGrid: d.Grid
  children: Listing<WidgetInstance>
  /// Default: union of children's references. Subclasses may add their own selectors.
  referencedEntityIds = children.toList().flatMap((c) -> c.referencedEntityIds).distinct
}

class Position {
  x: Int(isBetween(0, 96))
  y: Int(isBetween(0, 999))
  w: Int(isBetween(1, 96))
  h: Int(isBetween(1, 96))
}

class Grid {
  columns: Int(isBetween(1, 96)) = 12
  rowHeight: Int(isBetween(20, 200)) = 60
}
```

The eight built-ins are `class EntityToggle extends WidgetInstance`, `class Gauge extends WidgetInstance`, etc., declared in the same module with their typed props. (Full props in §14.)

The schema is **versioned semver**. v1 is the C10 baseline. Adding properties is minor; adding new widget classes is minor; renaming or removing properties is major.

### 13.2 React side — `WidgetProps`

The `@gohome/widget-sdk` package exports:

```ts
export type WidgetProps<T extends WidgetInstance = WidgetInstance> = {
  /** The Pkl-validated instance, typed via codegen. */
  instance: T;
  /** Live state for this widget's referenced entities. */
  state: Readonly<Record<string, EntityState>>;
  /** Pending-state map for command lifecycle on this widget's entities. */
  pending: Readonly<Record<string, PendingState>>;
  /** Issue a typed capability call. Resolves with command_id immediately. */
  callCapability: (entityId: string, capability: string, args: Record<string, unknown>) => Promise<{ commandId: string }>;
  /** Run a named script. Returns when the run completes. */
  runScript: (scriptId: string, args?: Record<string, unknown>) => Promise<RunResult>;
  /** Evaluate a server-side Starlark compute expression for this widget. */
  evalCompute: (exprId: string, contextOverride?: Record<string, unknown>) => Promise<unknown>;
  /** Active theme tokens (read-only). */
  theme: Theme;
};
```

The SDK is intentionally narrow. Widgets may not:
- Issue arbitrary Connect-RPCs.
- Read cookies, localStorage, or sessionStorage.
- Open WebSockets or `EventSource` connections.
- Set global CSS or `document.title`.
- Modify the parent DOM.

Containment is by convention in v1.0 (we cannot mechanically enforce these in a non-sandboxed context). The signature trust policy (§15) is the actual security boundary.

### 13.3 Container widgets

A `ContainerInstance` widget receives `instance.children: WidgetInstance[]` (Pkl `Listing<WidgetInstance>` decoded to a TS array) and is expected to render them recursively via the SDK's `<WidgetRenderer />` component:

```tsx
import { WidgetRenderer, type WidgetProps } from "@gohome/widget-sdk";

export default function GroupCard({ instance, theme }: WidgetProps<GroupCardInstance>) {
  return (
    <div className="bg-surface-1 rounded-md p-normal" style={{ background: theme.tokens.color.surface1 }}>
      {instance.title && <h3>{instance.title}</h3>}
      <ChildGrid spec={instance.childGrid}>
        {instance.children.map(c => <WidgetRenderer key={c.id} instance={c} />)}
      </ChildGrid>
    </div>
  );
}
```

`<WidgetRenderer />` and `<ChildGrid />` are SDK exports; they handle the recursion + per-widget error boundary + state subscription transparently.

### 13.4 Compute Starlark integration

A widget instance may declare a property typed `gohome.starlark.StarlarkExpr` (e.g., `Markdown.compute`). At config-load time, those expressions are registered with `internal/compute/service.go` and assigned a stable `expr_id`. The widget's React component calls `props.evalCompute("compute")` to fetch the current value; the SDK handles the RPC + caching the result keyed by `(widget_id, expr_id, state-snapshot-hash)`.

**v1.0 is naive.** The cache key includes the current state hash for the widget's referenced entities; whenever the multiplexer updates any of those, the cache invalidates and `evalCompute` re-fetches. For dashboards with a few compute widgets and modest state churn this is fine. Heavy use will thrash; debouncing/batching is a v1.x perf concern.

### 13.5 Lifecycle

A widget component mounts when its tile enters the visible viewport (no virtualization in v1.0 — all visible-route widgets mount eagerly). It unmounts when the route changes or the tile is removed. The SDK manages multiplexer subscriptions across mount/unmount; widget code does not need to subscribe explicitly.

### 13.6 Codegen for typed instances

`@gohome/widget-sdk` ships generated TypeScript types for the v1 built-ins. Pack authors generate types for their own widget classes via a CLI tool:

```bash
npx @gohome/widget-sdk-cli generate ./manifest.pkl > ./src/types.ts
```

This reads the Pkl manifest and produces TypeScript interfaces for each widget class. Packs without typed generation can use `WidgetProps<WidgetInstance>` (untyped instance access) but lose autocomplete.

---

## 14. v1 Built-In Widgets

All eight live in `src/widgets/` and are bundled into the embedded SPA. Their Pkl classes are part of `gohome.widgets`.

### 14.1 EntityToggle

```pkl
class EntityToggle extends WidgetInstance {
  entityId: String                                        // e.g., "light.kitchen"
  label: String?                                          // overrides entity friendly name
  icon: String?                                           // Lucide icon name
  showBrightness: Boolean = false                         // light entities only
  referencedEntityIds = List(entityId)
}
```

Renders the entity's name + icon + a toggle (Radix `Switch`). Tapping calls `callCapability(entityId, "turn_on" | "turn_off")` and uses pending-state UX (§8). For light entities with `showBrightness=true`, a slider appears below; dragging issues `set_brightness` commands debounced 200ms.

### 14.2 Gauge

```pkl
class Gauge extends WidgetInstance {
  entityId: String
  label: String?
  min: Number = 0
  max: Number = 100
  unit: String?                                           // "°F", "kWh", "%"
  thresholds: Listing<Threshold> = new {}                 // optional color bands
  referencedEntityIds = List(entityId)
}
class Threshold { value: Number; color: String }          // color = "success" | "warning" | "danger"
```

Renders a circular or linear gauge (auto-picked by aspect ratio). Numeric value uses `theme.tokens.font.numeric`. Out-of-range values clamp visually but display the real number.

### 14.3 LineChart

```pkl
class LineChart extends WidgetInstance {
  entityIds: Listing<String>(length > 0)
  label: String?
  rangeHours: Int(isBetween(1, 720)) = 24                 // up to 30 days
  yMin: Number?                                           // auto if unset
  yMax: Number?
  referencedEntityIds = entityIds.toList()
}
```

Fetches via `EventService.Query({ kinds: ["state_changed"], entities: entityIds, since: now-rangeHours })` on mount and on every multiplexer state-change for any tracked entity (debounced 1s). Renders with a lightweight charting library (uPlot — small, fast, theme-friendly via CSS).

### 14.4 CameraStream

```pkl
class CameraStream extends WidgetInstance {
  entityId: String                                        // a camera-class entity
  label: String?
  referencedEntityIds = List(entityId)
}
```

v1.0 is **MJPEG only**. The widget reads the entity's `stream_url` capability via state and renders an `<img>` with that URL (browsers handle MJPEG natively). WebRTC, HLS, and snapshot-fallback are deferred to v1.x.

### 14.5 Markdown

```pkl
class Markdown extends WidgetInstance {
  content: String?                                        // static
  compute: gohome.starlark.StarlarkExpr?                  // dynamic (runs server-side)
  referencedEntityIds = List()                            // determined by compute analyzer, not statically
}
```

Either `content` or `compute` (not both). Static content rendered via `react-markdown` with safe defaults (no raw HTML, no scripts). Compute results are stringified and rendered the same way.

When `compute` is set, the widget invokes `evalCompute("compute")` whenever its referenced state changes. The compute expression's referenced entities are extracted at config-load time by the Starlark analyzer (C5) and added to the multiplexer subscription union — even though `referencedEntityIds` is empty (the static Pkl-declared list), the dashboard's subscription union includes compute-referenced entities via a separate server-side `referencedComputeEntityIds` collection performed during catalog construction.

### 14.6 ScriptButton

```pkl
class ScriptButton extends WidgetInstance {
  scriptId: String
  label: String
  args: Mapping<String, Any>?
  confirm: Boolean = false                                // show a confirmation dialog
  icon: String?
  referencedEntityIds = List()
}
```

Renders as a button. Click → optional confirm dialog → `runScript(scriptId, args)`. Pending visual while the run is in flight; success/failure toast on completion.

### 14.7 EntityList

```pkl
class EntityList extends WidgetInstance {
  selector: EntitySelector                                // { areas?, classes?, domains?, ids? }
  label: String?
  showState: Boolean = true
  showQuickControls: Boolean = true                       // toggle/slider per row
  maxRows: Int? = 50
  referencedEntityIds = List()                            // dynamic; server resolves selector per-render
}
```

The selector is resolved against the registry; matching entities are subscribed via a per-widget addendum to the multiplexer union. Renders as a scrollable list with one row per entity. Quick controls per row: for boolean entities a toggle; for dimmable lights a brightness slider; for sensors no control. Clicking the row navigates to the (future) entity detail page; in C10 this is a no-op with a tooltip ("Coming in C11").

The dynamic entity set is the one place `referencedEntityIds` is statically empty — the server's catalog construction supplements with the resolved selector. New entities matching the selector appear in the list without dashboard reload (the multiplexer's `EntityRegistered` event triggers a re-resolution).

### 14.8 GroupCard

```pkl
class GroupCard extends ContainerInstance {
  title: String?
  collapsed: Boolean = false
  // inherits children, childGrid, referencedEntityIds from ContainerInstance
}
```

The only container in v1.0. Renders a `bg-surface-1 rounded-md` panel with optional title bar (collapsible if `title` is set). Children render in a nested `react-grid-layout` instance with `childGrid` as its spec. Containers participate in drag/drop (the parent's grid drags the container as a unit; the container's nested grid drags its children).

The Pkl `ContainerInstance` is the v1 schema's contract that future container widgets (`Tabs`, `Accordion`, etc.) extend without schema migration.

---

## 15. Widget Pack Format

### 15.1 Distribution: OCI artifact

A pack is an OCI artifact at `ghcr.io/<org>/<pack>:<version>` (or any OCI registry). Contents:

```
manifest.pkl       # gohome.widgets.PackManifest
bundle.js          # built ES module
README.md          # optional, surfaced in install CLI
LICENSE            # optional, surfaced in install CLI
```

Layout media type: `application/vnd.gohome.widgetpack.v1+tar+gzip`. The artifact is a single gzipped tarball of those files.

### 15.2 Manifest

```pkl
@ModuleInfo { minPklVersion = "0.27.0" }
module gohome.widgets.bar
import "@gohome/widgets.pkl" as w

manifest: w.PackManifest = new {
  name        = "bar-widgets"
  version     = "1.0.0"
  protocol    = "v1"
  sdkVersion  = "1.0.0"                 // version of @gohome/widget-sdk this pack builds against
  bundle      = "bundle.js"
  bundleHash  = "sha256:..."
  classes     = new { BarChart; PieChart }
  description = "Bar and pie chart widgets for gohome dashboards."
  homepage    = "https://github.com/foo/bar-widgets"
  license     = "MIT"
}

class BarChart extends w.WidgetInstance {
  entityIds: Listing<String>(length > 0)
  label: String?
  referencedEntityIds = entityIds.toList()
}

class PieChart extends w.WidgetInstance { /* ... */ }
```

The Pkl class names (`BarChart`, `PieChart`) match the JS bundle's named exports — single source of truth.

### 15.3 Bundle

A built ES module:

```js
import { createWidget } from "@gohome/widget-sdk";

export const BarChart = createWidget(/* ... */);
export const PieChart = createWidget(/* ... */);
```

`react`, `react-dom`, and `@gohome/widget-sdk` are externals (provided by the host app). Build target: `es2022` (matches the host SPA). Recommended bundler: Vite library mode or esbuild.

### 15.4 Install: `gohome widget install <oci-ref>`

Server-side flow (`internal/widgetpack/install.go`):

1. Pull artifact from `<oci-ref>` (using `oras-go` library).
2. Cosign verify signature against `gohome.widgetPackPolicy.trustedPublishers`. If `allowUnsigned=false` (default) and verification fails → reject.
3. Extract tarball into `~/.gohome/widgets/<pack>/<version>/`.
4. Validate `manifest.pkl` against the `gohome.widgets.PackManifest` schema (loads via the Pkl evaluator).
5. Verify `bundle.js` SHA256 matches `manifest.bundleHash`.
6. Verify `manifest.sdkVersion` is compatible with the host's shipped SDK version (semver-compatible major).
7. Verify no class-name collision with already-installed packs (or built-ins). Conflict → reject with the conflicting names listed.
8. Update the in-memory pack registry.
9. Trigger config reload so user dashboards referencing the new classes resolve.
10. Emit `WidgetPackInstalled{name, version, classes}` event.
11. Return success metadata (classes added, file sizes, signature status).

Failures clean up the partial install (delete the cache directory).

### 15.5 List, uninstall, multi-version

`gohome widget list`: lists installed packs from the registry. Output styled via lipgloss (see §19).

`gohome widget uninstall <pack> [--version <v>]`: removes the cache directory; if a user dashboard still references a class from the pack, the uninstall is rejected unless `--force` is passed (which marks affected dashboards as broken until the user reinstalls or removes the references).

Multiple versions of a pack may coexist on disk. Resolution (which version a dashboard's `import "@widgets/<pack>"` resolves to): **latest installed wins**. Pinning is deferred (`widgets-lock.pkl`, v1.x).

### 15.6 Trust policy

```pkl
import "@gohome/widgets.pkl" as widgets

widgetPackPolicy: widgets.PackPolicy = new {
  trustedPublishers = new {
    "https://github.com/gohome/widgets-*"      // first-party packs
    "https://github.com/myhandle/*"            // your own publishing identity
  }
  allowUnsigned = false                         // dev-only escape hatch
}
```

`trustedPublishers` patterns match cosign's "subject identity" claim. Sigstore's keyless signing (Fulcio-issued ephemeral certs bound to OIDC identities) is the default trust mechanism — the same approach Kubernetes, Cosign, and OCI ecosystem tools use.

`allowUnsigned=true` is intended only for local pack development. The CLI emits a banner warning whenever an unsigned pack is installed under this mode.

### 15.7 Runtime loading

The browser loads pack bundles via dynamic ES `import()`:

```ts
async function loadPack(packName: string, version: string, hash: string) {
  const url = `/widgets/${packName}/${version}/bundle.js?h=${hash}`;
  const module = await import(/* @vite-ignore */ url);
  return module;  // { BarChart, PieChart, ... }
}
```

The hash query param ensures aggressive HTTP caching (the URL changes on every version bump). gohomed's `/widgets/...` handler serves the cached file with `Cache-Control: public, max-age=31536000, immutable`.

Loading is **lazy per dashboard**: when a dashboard mounts, `collectUsedPacks()` finds which packs supply its classes, and the dashboard render suspends on `Promise.all([...packs].map(loadPack))`. Already-loaded packs from prior dashboard visits are cached in `Map<packName-version-hash, Module>` for the session.

### 15.8 Browser-side security model

Pack code runs in the **main app context** — no iframe sandbox. The narrow `WidgetProps` SDK is the convention; signature verification is the actual security boundary (a malicious pack from an untrusted publisher cannot install). v1.x may add iframe sandboxing for users wanting defense-in-depth (cost: theme tokens don't transfer across iframe boundaries; significant cosmetic compromise).

CSP headers for `gohomed`:

```
Content-Security-Policy: default-src 'self';
  script-src 'self' 'wasm-unsafe-eval';
  style-src 'self' 'unsafe-inline';
  img-src 'self' data: blob:;
  media-src 'self' blob:;
  connect-src 'self';
  frame-ancestors 'none';
```

Pack bundles served from `/widgets/...` count as `'self'` so dynamic-import works without CSP exceptions.

---

## 16. PWA & Theming Defaults

### 16.1 PWA scope

**Shell-only**:

- Service worker (`src/pwa/sw.ts`) caches the app shell (HTML, JS, CSS, fonts, icons) on first load. Strategy: **stale-while-revalidate** for assets, **network-first with cache fallback** for the entry HTML.
- Manifest (`public/manifest.webmanifest`) declares icons (192/512/maskable), name, theme color, display mode = `standalone`.
- No IndexedDB state cache — when gohomed is unreachable, dashboards render the last-seen state visibly greyed out with a "Reconnecting…" banner. Multiplexer reconnect handles the recovery.
- No background sync, no push notifications — both deferred to v1.x.

The service worker is bundled via `vite-plugin-pwa` with `injectManifest` strategy (we own the worker code).

### 16.2 Theme defaults

- **Mode toggle.** Light / Dark / System. Persisted in `localStorage` under `gohome.themeMode`. Default: `system`. Toggle in the top bar's user menu; visual transitions use `motion.snappy`.
- **Language.** Locked to `developer` in v1.0. The token system supports `friendly` and `ambient` per §5; no UI to switch — the future picker lights up once a second preset ships.
- **System mode detection.** `prefers-color-scheme` media query observed via `matchMedia`; updates the active mode reactively.
- **Initial paint.** A small inline `<script>` in `index.html` reads `localStorage.gohome.themeMode` (or `system` fallback) and writes the `data-theme` attribute *before* the React app mounts — prevents the white-flash-then-dark-mode flicker on first paint.

### 16.3 Palette (developer language)

Light + dark variants. Authoritative values in `src/theme/tokens.css`. Summary:

| Token | developer-light | developer-dark |
|---|---|---|
| `--gh-color-bg` | oklch(98% 0 0) | oklch(11% 0 0) |
| `--gh-color-surface-1` | oklch(100% 0 0) | oklch(14% 0 0) |
| `--gh-color-surface-2` | oklch(96% 0 0) | oklch(18% 0 0) |
| `--gh-color-border` | oklch(90% 0 0) | oklch(22% 0 0) |
| `--gh-color-fg` | oklch(15% 0 0) | oklch(96% 0 0) |
| `--gh-color-fg-muted` | oklch(50% 0 0) | oklch(60% 0 0) |
| `--gh-color-accent` | oklch(64% 0.16 220) | oklch(75% 0.13 220) |
| `--gh-color-success` | oklch(60% 0.18 145) | oklch(70% 0.16 145) |
| `--gh-color-warning` | oklch(70% 0.16 70) | oklch(80% 0.14 70) |
| `--gh-color-danger` | oklch(58% 0.21 25) | oklch(70% 0.18 25) |

(`oklch` chosen over `hsl` for perceptually uniform interpolation. Browsers supporting OKLCH = Chrome 111+, Safari 16.4+, Firefox 113+ — well within the v1.0 evergreen support floor.)

### 16.4 Browser support floor

- Chrome 113+, Firefox 113+, Safari 16.4+, Edge 113+.
- ES2022 syntax target; no polyfills.
- Service worker required (excludes legacy IE/old Safari).

Documented in the README; gohomed serves a static fallback HTML to user agents below the floor explaining the requirement.

---

## 17. Build & Embed Pipeline

### 17.1 Project layout

```
gohome/
└── web/                          # the React project (separate package, not part of go.mod)
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    ├── eslint.config.ts          # includes gohome/no-raw-tokens
    ├── tailwind.config.ts
    ├── public/
    │   └── manifest.webmanifest
    ├── src/                      # see §3.2
    └── tools/
        └── eslint-plugin-gohome/
```

### 17.2 Build

`vite build` outputs to `web/dist/`:

```
dist/
├── index.html                                          # entry, hash-injected
├── assets/
│   ├── index-[hash].js
│   ├── index-[hash].css
│   ├── chunk-[hash].js                                 # code-split chunks
│   ├── inter-var-[hash].woff2
│   └── jetbrains-mono-var-[hash].woff2
└── manifest.webmanifest
```

All filenames are content-hashed; `Cache-Control: immutable` works for everything except `index.html` (revalidated on every load).

### 17.3 Embedding

`internal/web/assets.go` uses `//go:embed dist/*` to pull `web/dist/` into the binary. A small Go template substitutes runtime values into `index.html` (e.g., the gohomed version for the `<meta>` tag, the active build commit). All `/assets/*` requests serve directly from the `embed.FS`.

The Go side never touches `web/`'s source; the build pipeline is:

```
$ task web:install      # npm install
$ task web:build        # vite build → web/dist/
$ task build            # go build, embed.FS pulls web/dist/
```

CI runs all three in sequence. The binary is what gets shipped.

### 17.4 Dev mode

For contributor convenience:

```
$ gohome ui dev
```

This starts gohomed *and* a Vite dev server on a sibling port, with gohomed proxying `/assets/*` and `/` to Vite. Hot module reload works, source maps work, and Connect-RPC + multiplexer hit the real gohomed. The dev mode is a one-process orchestration — if either side dies, the other follows.

If `gohomed` is run in production mode (`gohome ui dev` not used), the embedded build is served. Detection: presence of the `dist/` directory at runtime overrides the embedded build (useful for debugging an installed binary against a freshly-built UI).

### 17.5 Asset budgets

CI fails the build if `dist/assets/index-*.js` exceeds:

- 350 KB gzipped (initial chunk, blocks first paint)
- 1.5 MB gzipped (total of all eager chunks)

Lazy-loaded chunks (Monaco for the future config editor, code-split routes) are not budgeted. Built-in widgets are part of the initial bundle.

### 17.6 No SSR, no static export

The SPA is a SPA. `gohomed` serves `index.html` for all unmatched routes; TanStack Router renders client-side. SSR is a non-goal — this is a private-LAN admin UI, not a public web app.

---

## 18. Testing Strategy

### 18.1 Layers

| Layer | Tool | Location | What it covers |
|---|---|---|---|
| Unit (browser) | Vitest | `web/src/**/*.test.ts` | Pure functions, hooks (with `@testing-library/react-hooks`), reducers, the command tracker, the multiplexer subscription manager (with mocked Connect transport). |
| Component | Vitest + Testing Library | `web/src/**/*.test.tsx` | Each built-in widget mounted with mocked `WidgetProps`; light/dark snapshot of every shadcn primitive (token-compliance smoke test). |
| Integration (server) | Go tests | `internal/dashboard/**/*_test.go`, `internal/widgetpack/**/*_test.go` | Pkl regenerator round-trip golden tests; pack install end-to-end against a local OCI registry. |
| E2E | Playwright | `web/e2e/**/*.spec.ts` | Login flow (password + passkey via virtual authenticator); dashboard read; full edit cycle (drag a widget, change a prop, save, reload, verify); widget pack install (against a fixture registry); pending-state UX (mocked driver). |
| Visual regression | Playwright + screenshot diff | `web/e2e/visual/**/*.spec.ts` | Per-route, per-mode (light/dark) screenshot comparison; threshold 0.1%. |
| Lint | ESLint with `gohome/no-raw-tokens` | All source | Token discipline; CI fails on any warnings. |

### 18.2 E2E harness

The Playwright suite spins up a real `gohomed` binary against a fresh ephemeral SQLite DB and a Pkl config fixture. Browser interactions are real (chromium headless); the multiplexer connects to the real listener; entity events are produced by a fake driver registered in the fixture config. The suite is the real proof that the data flow described in §3.5 actually works.

### 18.3 Pkl round-trip golden tests

`internal/dashboard/regen/regen_test.go` is the single most important test in C10. Failure modes the table explicitly covers:

- Empty layout → empty `widgets = new {}`.
- Single leaf widget.
- Two siblings.
- One container with two children.
- Nested containers (3 levels deep).
- Starlark expressions in props (verbatim pass-through).
- Special characters in string props (escaping).
- Unicode in string props.
- Very long entity ID lists (50 entries).
- Malformed input proto (validation rejects before write).

Each test asserts:
1. Regenerated output byte-matches expected.
2. Re-evaluated Pkl matches the input proto (round-trip identity).
3. Regenerator is deterministic across 10 invocations on the same input.

### 18.4 What's not tested

- Cross-browser parity beyond Chrome (manual smoke pass on Firefox and Safari at release; not in CI).
- Performance (no perf benchmarks in v1.0; we measure when we have a complaint).
- Accessibility (lint catches missing alt text and aria roles; full screen-reader pass deferred to a dedicated a11y milestone).

---

## 19. CLI Surface

All CLI output uses lipgloss styles consistent with the existing `gohome` CLI.

### 19.1 `gohome widget install <oci-ref>`

```
Installing bar-widgets v1.0.0 from ghcr.io/foo/bar-widgets:1.0.0
  ✓ pulled artifact (signed by https://github.com/foo)
  ✓ verified cosign signature
  ✓ validated manifest (2 classes: BarChart, PieChart)
  ✓ verified bundle hash
  ✓ written to ~/.gohome/widgets/bar-widgets/1.0.0
  ✓ reloaded config
Pack installed. 2 widget classes available: BarChart, PieChart.
```

Lipgloss style mapping:

| Element | Style |
|---|---|
| Top-line bold heading (`Installing ...`) | `lipgloss.Bold().Foreground(accent)` |
| Step lines (`✓ ...`) | `lipgloss.Foreground(success)` for ✓; `lipgloss.Foreground(danger)` for ✗ on failure |
| Final summary | `lipgloss.Bold()` |
| Class names in summary | `lipgloss.Foreground(accent)` |
| Warning banner (unsigned install) | `lipgloss.Background(warning).Foreground(bg).Padding(0, 1)` |

Errors render as a red-bordered box with the failure step, the underlying cause, and a "What to do" hint.

### 19.2 `gohome widget list`

Renders a table:

```
NAME            VERSION   CLASSES                  PUBLISHER                         STATUS
bar-widgets     1.0.0     BarChart, PieChart       github.com/foo                    verified
old-widgets     0.3.2     LegacyChart              github.com/abandoned              ✗ signature expired
local-dev       0.0.1     TestWidget               (local file)                      unsigned
```

Lipgloss table via `lipgloss.NewStyle().Border(lipgloss.NormalBorder())` per row; status column uses `success`/`warning`/`danger` palette per state. Compact table fits 80-col terminals; `--wide` switches to extended columns (install date, bundle size, SDK version).

### 19.3 `gohome widget uninstall <pack> [--version <v>] [--force]`

```
Uninstalling bar-widgets v1.0.0
  ✓ no dashboards reference this pack
  ✓ removed ~/.gohome/widgets/bar-widgets/1.0.0
  ✓ reloaded config
Pack uninstalled.
```

If dashboards do reference it:

```
Cannot uninstall bar-widgets v1.0.0 — these dashboards reference it:
  - dashboards/default.pkl       (uses BarChart)
  - dashboards/kitchen.pkl       (uses BarChart, PieChart)

Re-run with --force to uninstall anyway (these dashboards will fail to load).
```

The blocking message uses a `lipgloss.Foreground(warning)` heading, listed dashboards in `lipgloss.Bold()`.

### 19.4 `gohome ui dev`

```
gohome UI dev mode

  ▸ Vite dev server:  http://localhost:5173    (HMR enabled)
  ▸ gohomed:          http://localhost:8080

  Open http://localhost:5173 in your browser.

  Press Ctrl-C to stop both processes.
```

Streaming output from both processes is interleaved with prefixed labels (`[vite]`, `[gohomed]`) styled in different muted colors.

### 19.5 What's not added

- No `gohome dashboard {create,delete,list}` CLI in C10 — these go through the UI or `gohome config apply` against hand-edited Pkl. (Adding CLI dashboard management in a future milestone is straightforward; not needed to validate C10's contract.)

---

## 20. Configuration

Two new Pkl modules.

### 20.1 `gohome.web` (the embedded UI configuration)

```pkl
import "@gohome/web.pkl" as web

gohome.web: web.Web = new {
  enabled = true                    // default: true. Set to false to disable the UI on this gohomed instance.
  embeddedTheme = "developer"       // locked to "developer" in v1.0
  defaultMode = "system"            // "light" | "dark" | "system"
  pwa = new {
    enabled = true
    manifestName = "gohome"         // override for branded deployments
    manifestShortName = "gohome"
    themeColor = "#0a0a0a"          // browser chrome color for installed PWA
  }
}
```

If `gohome.web.enabled = false`, gohomed serves only `/api`, `/healthz`, `/mcp`, `/webhooks/*` — `index.html` and `/assets/*` return 404. Useful for headless edge deployments.

### 20.2 `gohome.widgetPackPolicy`

```pkl
import "@gohome/widgets.pkl" as widgets

gohome.widgetPackPolicy: widgets.PackPolicy = new {
  trustedPublishers = List(
    "https://github.com/gohome/widgets-*",
  )
  allowUnsigned = false             // default false. Dev-only escape.
}
```

Uninstall and install operations both consult this policy. Changing the policy and reloading config does *not* uninstall already-installed packs that no longer satisfy it; instead, `gohome widget list` shows them with status `✗ no longer trusted`, and dashboards using them keep working until manually uninstalled.

---

## 21. Implementation Order

Engineered to maximize "thing the user can see and react to" per increment. Each numbered item is intended as one PR or one tight commit series.

1. **Web project scaffold** — `web/` directory, `package.json`, `vite.config.ts`, `tsconfig.json`, basic `index.html`. CI runs `vite build` and uploads the artifact.
2. **embed.FS pipeline** — `internal/web/assets.go`, gohomed serves `index.html` + `/assets/*` from embed. Asset budgets in CI.
3. **Token foundation** — `src/theme/tokens.css`, `tailwind.config.ts` wiring, `eslint-plugin-gohome/no-raw-tokens` rule, light/dark mode toggle. Ship a "hello world" page proving tokens flow correctly across modes. **No application logic yet.**
4. **shadcn primitive intake protocol** — bring in the few primitives (`Button`, `Sheet`, `Switch`, `Toast`, `Dialog`) C10 needs, audit each for token compliance, snapshot tests.
5. **App shell** — sidebar + top bar, mobile drawer reflow, route placeholder. All nav items present-but-disabled except Dashboards. Static; no data layer yet.
6. **Data layer** — Connect-ES transport, query client, auth-store stub (single static "system:local" user as placeholder), error toast infrastructure.
7. **Login page** — `/login` route, password flow against `AuthService.Login`, auth interceptor with refresh-on-401, route guard. (Passkey flow added after C9 ships its WebAuthn endpoints.)
8. **Multiplexer** — both server streams, command tracker, `useEntityState` / `usePending` hooks. Test with mocked Connect transport.
9. **`DashboardService` server side** — `Get`, `GetWidgetCatalog` returning the eight built-in classes. No editing yet.
10. **Dashboard render path** — `/dashboards/$slug` route, grid + recursive `WidgetRenderer`, per-widget error boundary.
11. **First three built-in widgets** — `EntityToggle`, `Markdown`, `Gauge`. Implements the pending-state UX; verifies the SDK works end-to-end. Single-tile dashboards renderable via hand-edited Pkl.
12. **Pkl regenerator + round-trip tests** — `internal/dashboard/regen/`, full golden test table. **No editor UI yet.** This is implementation-first to lock down the contract.
13. **`DashboardService.SaveLayout` + `Create` + `Delete`** — uses the regenerator. Tests via Go integration; no UI yet.
14. **Edit mode UI** — handles, selection, props panel, FAB picker, undo/redo, save/discard. Uses the existing SaveLayout RPC.
15. **Remaining built-in widgets** — `LineChart`, `CameraStream`, `ScriptButton`, `EntityList`, `GroupCard` (the container). E2E tests for each.
16. **Compute Starlark** — `ConfigService.EvalCompute` server side; `evalCompute` SDK hook; wire up `Markdown.compute`.
17. **Widget pack format — server side** — `internal/widgetpack/` with install, list, uninstall; OCI pull; cosign verify; manifest validate; on-disk store; trust policy.
18. **Widget pack — runtime loading** — `/widgets/<pack>/<v>/<file>` handler; client-side dynamic import; `WidgetCatalog` sourcing from installed packs.
19. **`@gohome/widget-sdk` npm package** — extract types and hooks; publish to npm (or initially to a private registry). Documentation. Test harness for pack authors.
20. **CLI surface** — `gohome widget install/list/uninstall`, `gohome ui dev`. Lipgloss styling per §19.
21. **PWA service worker** — `vite-plugin-pwa` config, manifest, install prompt UX in the user menu.
22. **E2E suite expansion** — full Playwright coverage of login, dashboard read/edit/create/delete, pack install/uninstall, pending-state UX with fake driver.
23. **Visual regression baseline** — capture screenshots of every route in light + dark; CI threshold check.
24. **Documentation pass** — README in `web/`, contributor guide for design tokens, widget pack authoring guide, configuration reference for `gohome.web` and `gohome.widgetPackPolicy`.

Each item is independently reviewable. Critically, items 1–9 ship a working "you can sign in and see *something*" app before any dashboard editing exists — feedback comes early and often.

---

## 22. Decision Record

| # | Decision | Alternatives considered | Reason |
|---|---|---|---|
| 1 | C10 = foundation + dashboard subsystem only | Foundation only / whole web UI | Matches the master design's literal §10 entry; makes feature pages cheap follow-ons rather than an unbounded mega-spec. |
| 2 | Minimal `/login` form (B1) for auth UI | No browser auth UI / full auth UI | Without a login page nothing in C10 is reachable; rich auth UI is its own milestone because of WebAuthn enrollment complexity. |
| 3 | Hybrid sidebar + thin top bar shell | Top-only / icon-rail-only | Sidebar grows gracefully as feature pages add nav items; top bar holds page title + theme toggle. Icon-rail rejected on discoverability grounds. |
| 4 | Pending-state hybrid for command UX | Pure event-driven / optimistic mutation | Honest about the event-sourced architecture: "I'm trying" is the truth. Optimistic would lie when commands fail; pure event-driven feels sluggish. |
| 5 | Two streams in the multiplexer | Single raw `EventService.Tail` | Uses C7's typed `EntityService.Subscribe` (server-policy-filtered, lower bandwidth) as primary; `EventService.Tail` filtered to command lifecycle for pending reconciliation. |
| 6 | `Dashboard` proto carries source + layout Pkl strings | Fetch source separately | Cheap to include; "view source" toggle and editor save flow both need it. |
| 7 | Widget catalog as separate RPC | Inline in every Get response | Catalog changes only on pack install/uninstall; cacheable independently. |
| 8 | Inline edit chrome + slide-in props panel + FAB picker | Nav-becomes-palette / bottom drawer | App nav stays put — escape from edit mode by clicking another route. Familiar pattern (Figma, Notion). |
| 9 | Two-file split for Pkl round-trip | Always-regenerate / AST mutator | Unambiguous ownership boundary. Regenerator is dumb and deterministic. Hand-written Pkl is never touched. AST mutator was a research project we don't need. |
| 10 | Recursive `ContainerInstance` shipped in v1.0 | Add containers in a later milestone | Schema migration cost (every dashboard, every Pkl round-trip, every renderer assumption) is way higher than the up-front recursive structure. |
| 11 | Eight built-in widgets | Six (master design) / fewer / many more | EntityList's value justifies a leaf-widget addition; GroupCard ships now to lock the recursive schema. Other tempting widgets (Climate, MediaPlayer, Lock, Map) are real follow-on widget-pack territory. |
| 12 | Server-side Starlark for compute widgets | Client-side runtime / no compute | All logic flows through gohomed (sandboxed, policy-checked, audited). Client bundle stays small. v1.0 naive recompute; debouncing is v1.x. |
| 13 | Widget packs as OCI artifacts with cosign | Tarball-over-HTTP / npm registry | Matches drivers' distribution; one signing/registry/verification stack. |
| 14 | Dynamic ES import for widget loading | Static script tags / iframe sandbox | Lazy per-dashboard; HTTP-cached by hash; matches modern bundler patterns. iframe deferred to v1.x because of the styling/theme cost. |
| 15 | Signature is the security boundary | iframe sandbox / both | Trusted-publisher gate is enforceable; runtime sandbox costs theme transferability. Defer iframe to v1.x with the perf/UX work. |
| 16 | PWA shell-only | Read-only offline / offline-first | Read-only offline value is thin given gohomed runs on the LAN you're disconnected from. Offline-first command queuing is a different product. |
| 17 | Ship `developer` design language; architect for `friendly` and `ambient` | Hard-code one / ship multiple | Token architecture lands now (otherwise `friendly`/`ambient` are full rewrites later); only one preset shipped to constrain v1.0 scope. |
| 18 | Tailwind v4 + shadcn with enforced token discipline (lint rule) | Panda CSS / vanilla-extract / CSS Modules | Ecosystem inertia + shadcn breadth; Panda was the architecturally honest swap but doesn't pay back its ecosystem cost for v1.0. Discipline is mechanizable via lint. |
| 19 | OKLCH color tokens | HSL / sRGB hex | Perceptually uniform interpolation; modern browser support is universal at our floor. |
| 20 | Per-widget error boundary | Single dashboard-level boundary | One bad widget shouldn't kill the dashboard; community packs make this non-negotiable. |
| 21 | Undo/redo in edit mode (Zustand history) | Save-only / no undo | Drag operations lose precision otherwise; users will accidentally drag and want one-keystroke recovery. |
| 22 | No version pinning for widget packs in v1.0 | Per-dashboard pin / `widgets-lock.pkl` | Single installed version is the common case; pinning matters once dozens of packs are in play. |
| 23 | Cosign keyless signing default | GPG / sigstore-only / no signing | Aligns with broader OCI ecosystem; no per-publisher PKI to manage. |
| 24 | No SSR | Static export / SSR | Private-LAN admin UI; SSR adds complexity for no user-visible benefit. |

---

## 23. Explicit Deferrals

Named here so the spec acknowledges them without blocking.

### 23.1 Deferred to follow-on milestones (post-C10)

- **Feature pages**: `/entities`, `/devices`, `/areas`, `/zones`, `/automations`, `/scripts`, `/events` explorer, `/config` Monaco/LSP editor, `/drivers`, `/settings`. The C10 sidebar reserves their slots as disabled placeholders.
- **Rich auth UI**: passkey enrollment management (registration ceremony in browser), API token list/create/revoke, user management, OIDC login flow.
- **`gohome dashboard` CLI** (create/delete/list dashboards from CLI). Dashboard lifecycle in v1.0 is via the WYSIWYG editor or hand-edited Pkl.
- **Global search palette** — needs every feature service to register search providers; lands once those services exist.
- **Notifications panel** — depends on push notifications.

### 23.2 Deferred to v1.x

- **Read-only offline mode** (IndexedDB last-state cache during gohomed outages).
- **Push notifications** and notification permission requesting.
- **User-facing language switcher** — locked to `developer` in v1.0; the picker lights up when a second preset (`friendly` or `ambient`) ships. Architecture is in place.
- **Additional language presets**: `friendly` (Apple Home / consumer aesthetic) and `ambient` (frosted glass kiosk aesthetic). Architecturally accommodated; not bundled.
- **Pkl-configurable per-dashboard theme overrides** (`gohome.theme = …` block). Architecture is in place.
- **Container widgets beyond `GroupCard`**: `Tabs`, `Accordion`, `SplitView`, etc. The recursive schema accommodates them additively.
- **Additional built-in widgets**: `AreaCard`, `Climate`, `MediaPlayer`, `Lock`, `Map`. Strong candidates for community widget packs in the meantime.
- **Widget pack version pinning** (`widgets-lock.pkl`, per-dashboard pinning).
- **iframe sandboxing** of community widget pack JS. v1.0 trust boundary is sigstore signature; v1.x adds optional iframe defense-in-depth (with the styling/theme cost).
- **WebRTC for `CameraStream`** — v1.0 is MJPEG only.
- **Compute-Starlark caching/debouncing/batching** — v1.0 is naive recompute on every state-change frame for affected widgets.
- **In-app help system** beyond a "?" link to docs.
- **Background sync** for any deferred-write operations (dashboard saves, etc.). v1.0 retries on reconnect.

### 23.3 Out of scope indefinitely

- **Offline-first command queuing** with conflict resolution. Different product.
- **Client-side Starlark runtime.** All Starlark always runs server-side.
- **AST-preserving Pkl source mutator.** Two-file split is the architecture.
- **Pkl source comment/formatting preservation in WYSIWYG-owned `.layout.pkl`.** Regenerator is canonical; the doc-comment preamble is preserved unconditionally.
- **Custom community widget palettes** (overriding the core token palette). Token system intentionally doesn't expose this.
- **SSR / static export** of the SPA.
- **Cross-browser parity testing in CI** beyond Chrome (manual smoke at release).
- **Native mobile apps** (inherited from master design — PWA only).

---

*End of C10 design document.*
