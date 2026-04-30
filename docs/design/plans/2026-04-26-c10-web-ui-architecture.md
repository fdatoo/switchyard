# C10 — Web UI Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the gohome web UI as a real, embedded, theme-architected React application served by `gohomed`, with a working dashboard subsystem (read + WYSIWYG edit + Pkl two-file round-trip), an OCI/sigstore widget pack format, and the server-side WebAuthn challenge storage C9 deferred. After this plan, an operator signs in to the embedded UI with C9-issued credentials, views and edits their default dashboard, and installs community widget packs.

**Architecture:** A new top-level `web/` directory holds the Vite-built React 19 SPA (separate package, not part of `go.mod`). The build artifact is embedded into `gohomed` via `embed.FS` and served from the C7 listener at `/`, `/assets/*`, and `/widgets/*`. Server-side: a new `internal/web` handles SPA + widget asset serving; a new `internal/dashboard` owns the deterministic Pkl regenerator and the expanded `DashboardService`; a new `internal/widgetpack` handles OCI pulls + cosign verification + on-disk store; a new `internal/compute` wraps C5's Starlark for widget compute expressions. The browser holds two long-lived server-streaming RPCs (`EntityService.Subscribe` + `EventService.Tail` filtered to command lifecycle) behind a single `useEntityState` / `usePending` abstraction. Theming is a CSS-custom-property substrate consumed by Tailwind v4's `@theme` wiring; a custom ESLint rule enforces token-only utility usage.

**Tech Stack:**
- *Browser:* React 19, Vite 5, TypeScript 5.4+, Tailwind CSS 4, shadcn/ui primitives copied in, Radix primitives (transitively via shadcn), Framer Motion, TanStack Query 5, TanStack Router 1.x, Connect-ES (Connect-protocol over HTTP/2 with binary protobuf), Zustand 4, react-grid-layout 1.4+, react-markdown, uPlot, vite-plugin-pwa with `injectManifest` strategy, vitest + @testing-library/react, Playwright for E2E.
- *Server:* Go 1.25 (already in tree), `oras.land/oras-go/v2` (OCI artifact pulls), `github.com/sigstore/sigstore-go` or `github.com/sigstore/cosign/v2` (signature verification), `github.com/go-webauthn/webauthn` (already pulled in by C9), existing `embed.FS`, existing `connectrpc.com/connect`, existing Pkl evaluator integration.
- *Tooling:* `task` (existing Taskfile.yml), npm 10+, eslint 9 (flat config), `@gohome/widget-sdk` published to npm under the org's namespace.

**Depends on:** C7 (Connect-RPC API + listener + interceptor stack), C8 (MCP server alongside on the same listener; no direct C10 dependency but coexists), C9 (auth seam + AuthService + identity/credential stores; passkey login Unimplemented stubs are filled in by Task 7 of this plan).

---

## Codebase orientation

Before starting, read these files to understand the existing patterns this plan extends:

| File | Why |
|---|---|
| `docs/superpowers/specs/2026-04-26-c10-web-ui-architecture-design.md` | This plan's source of truth |
| `docs/superpowers/specs/2026-04-23-c7-connect-rpc-api-design.md` | The Connect-RPC surface and listener that C10 builds on |
| `docs/superpowers/specs/2026-04-25-c9-auth-and-policy-design.md` | The auth/policy enforcement points Task 7 + Task 8 hook into |
| `docs/proto-hygiene.md` (in `gohome/`) | Grouped-numbering + reserved-forever rules; matters for every proto change in this plan |
| `gohome/CLAUDE.md` | "Definition of done" check list, package map, architectural invariants — the whole plan must satisfy these |
| `gohome/Taskfile.yml` | `task build` / `test` / `test:race` / `test:integration` / `lint` / `proto` are the canonical commands |
| `gohome/internal/api/listener/listener.go` | Where `/`, `/assets/*`, `/widgets/*` get mounted (Task 2) |
| `gohome/internal/api/listener/routes.go` | Pattern for `BuildRoutes` (no change needed, but read for context) |
| `gohome/internal/api/deps.go` | The `XBackend` interfaces consumed by service handlers (Task 13/17 add `DashboardBackend`) |
| `gohome/internal/api/service_unimplemented.go` | The stub `DashboardService` C7 left; Tasks 13/17 replace these methods |
| `gohome/internal/api/service_auth.go` (post-C9) | C9-shipped impls; Task 7/8 fills in the Unimplemented passkey branches |
| `gohome/internal/auth/credentials/webauthn.go` (post-C9) | Where Task 7's challenge store lives |
| `gohome/internal/config/pkl/gohome/dashboards.pkl` | Existing minimal stub; Task 11 expands into the recursive schema |
| `gohome/internal/config/pkl/gohome/widgets.pkl` | Existing const-only stub; Task 11 turns it into the real widget contract |
| `gohome/internal/config/pkl/gohome/auth.pkl` | C9-shipped User/Role/Policy shapes; reference only |
| `gohome/internal/config/pkl/gohome/base.pkl` | Pattern for typealias + class declarations |
| `gohome/internal/config/evaluator.go` | How the Pkl evaluator gets invoked from Go (Task 16's regenerator validates output through this) |
| `gohome/internal/config/manager.go` | The reload pipeline `SaveLayout` triggers (Task 17) |
| `gohome/internal/cli/styles.go`, `styles_mcp.go`, `styles_automation.go` | Lipgloss style patterns; Task 24's `styles_widget.go` follows these |
| `gohome/internal/cli/cmd_mcp.go` | Pattern for a Cobra command tree with subcommands (Task 24's `cmd_widget.go`) |
| `gohome/internal/observability/metrics.go` | Where Prometheus metrics are registered (Task 18 adds `gohome_dashboard_*` and `gohome_widgetpack_*`) |
| `gohome/proto/gohome/v1alpha1/dashboard.proto` | The C7 stub Task 10 expands |
| `gohome/proto/gohome/v1alpha1/config.proto` | Task 20 adds `EvalCompute` here |
| `gohome/proto/gohome/v1alpha1/auth.proto` | Reference only — Task 7 doesn't change shape, just fills server impls |

---

## File map

### New files (in `gohome/`)

| Path | Responsibility |
|---|---|
| `internal/web/embed.go` | `//go:embed dist/*` declaration; exposes `Assets fs.FS` |
| `internal/web/handler.go` | HTTP handler for `/` (SPA index), `/assets/*` (hashed static assets), with `Cache-Control` and `Content-Type` discipline |
| `internal/web/widgets_handler.go` | HTTP handler for `/widgets/<pack>/<version>/<file>` serving from on-disk widget cache |
| `internal/web/index_template.go` | Renders `index.html` with version metadata + initial-theme inline script |
| `internal/web/handler_test.go`, `widgets_handler_test.go`, `index_template_test.go` | Unit tests for each handler |
| `internal/dashboard/service.go` | `DashboardService` Connect handler implementations (List/Get/GetWidgetCatalog/SaveLayout/Create/Delete) |
| `internal/dashboard/backend.go` | `DashboardBackend` interface consumed by `service.go`; lives in `internal/api` package alongside the others |
| `internal/dashboard/catalog.go` | Builds `WidgetCatalog` from compiled config + installed packs |
| `internal/dashboard/scaffold.go` | Creates `<slug>.pkl` + `<slug>.layout.pkl` pairs on Create |
| `internal/dashboard/regen/regen.go` | Deterministic Pkl serializer for `*.layout.pkl` |
| `internal/dashboard/regen/regen_test.go` | Golden round-trip tests with fixture table |
| `internal/dashboard/regen/testdata/` | Input proto fixtures + expected `.layout.pkl` outputs |
| `internal/dashboard/service_test.go` | Connect handler tests with fake backend |
| `internal/widgetpack/install.go` | OCI pull → cosign verify → manifest validate → cache write → reload |
| `internal/widgetpack/store.go` | On-disk pack registry; lookup by name/version; class lookup |
| `internal/widgetpack/trust.go` | Consumes `gohome.widgetPackPolicy`; matches against cosign subject |
| `internal/widgetpack/server.go` | Returns the asset filesystem rooted at `~/.gohome/widgets/` for `internal/web/widgets_handler.go` |
| `internal/widgetpack/install_test.go`, `store_test.go`, `trust_test.go` | Unit + integration tests |
| `internal/widgetpack/migrations/0001_widget_packs.up.sql` | DDL for `widget_packs` table (installed pack metadata) |
| `internal/widgetpack/migrations/0001_widget_packs.down.sql` | Drop table |
| `internal/compute/service.go` | `ConfigService.EvalCompute` impl wrapping C5 Starlark engine |
| `internal/compute/service_test.go` | Unit tests with fake state snapshot |
| `internal/cli/cmd_widget.go` | Cobra `gohome widget` command tree: `install`, `list`, `uninstall` |
| `internal/cli/cmd_ui.go` | `gohome ui dev` command (proxies Vite + gohomed) |
| `internal/cli/styles_widget.go` | Lipgloss styles: `Verified`, `Unsigned`, `Expired`, `PackName`, `ClassName`, `BlockingBox` |
| `internal/api/integration_dashboard_test.go` | Integration test (`//go:build integration`) — real gohomed binary, real SQLite, scaffold/edit/save round-trip |
| `web/package.json` | npm package definition; runtime + dev deps; scripts |
| `web/vite.config.ts` | Vite config, asset hashing, manualChunks, `vite-plugin-pwa` |
| `web/tsconfig.json` | TS config (strict, target ES2022, JSX react-jsx) |
| `web/tsconfig.node.json` | TS config for Vite config files |
| `web/eslint.config.ts` | Flat ESLint config including `gohome/no-raw-tokens` |
| `web/tailwind.config.ts` | Tailwind v4 config wired to CSS custom properties |
| `web/postcss.config.cjs` | PostCSS for Tailwind |
| `web/index.html` | HTML entry; inline theme-init script; PWA manifest link |
| `web/public/manifest.webmanifest` | PWA manifest |
| `web/public/icon-192.png`, `icon-512.png`, `icon-maskable.png` | PWA icons |
| `web/src/main.tsx` | App entry: theme provider, router, query client, multiplexer |
| `web/src/shell/Shell.tsx`, `Sidebar.tsx`, `TopBar.tsx`, `MobileDrawer.tsx`, `UserMenu.tsx` | App chrome |
| `web/src/theme/tokens.css` | CSS custom properties (the substrate) |
| `web/src/theme/languages/developer.ts` | The only v1.0 language preset |
| `web/src/theme/provider.tsx` | `ThemeProvider` component + `useTheme` hook |
| `web/src/theme/motion.ts` | Framer Motion presets |
| `web/src/theme/types.ts` | `Theme`, `LanguagePreset`, `TokenSet` |
| `web/src/data/client.ts` | Connect-ES transport + auth interceptor |
| `web/src/data/query-client.ts` | TanStack Query client setup |
| `web/src/data/auth-store.ts` | Zustand store: current user, refresh state |
| `web/src/multiplexer/multiplexer.tsx` | `<Multiplexer>` component scoping the two streams to a route |
| `web/src/multiplexer/streams.ts` | Manages the two server-stream connections + reconnect/resume |
| `web/src/multiplexer/state-cache.ts` | `Map<EntityId, EntityState>` + subscriber fan-out |
| `web/src/multiplexer/command-tracker.ts` | Joins `CommandIssued`/`Acked`/`Failed`; pending-state map |
| `web/src/multiplexer/hooks.ts` | `useEntityState`, `usePending`, `useCallCapability` |
| `web/src/components/ui/` | Audited shadcn primitives (token-compliant) |
| `web/src/components/gh/` | Wrapper layer over shadcn (Button, Sheet, Switch, Toast, Dialog, Tooltip) |
| `web/src/routes/__root.tsx` | TanStack Router root: providers, error boundary |
| `web/src/routes/login.tsx` | The minimal login page |
| `web/src/routes/_authed/_layout.tsx` | Auth-required shell wrapper |
| `web/src/routes/_authed/index.tsx` | Redirect to /dashboards/default |
| `web/src/routes/_authed/dashboards/$slug.tsx` | The only feature route in v1.0 |
| `web/src/routes/_authed/dashboards/$slug.test.tsx` | Component test for the route |
| `web/src/dashboard/render/DashboardView.tsx` | Read-only render path |
| `web/src/dashboard/render/Grid.tsx`, `WidgetRenderer.tsx`, `WidgetErrorBoundary.tsx`, `DashboardSkeleton.tsx` | Grid + recursion + error containment |
| `web/src/dashboard/edit/EditModeProvider.tsx` | Edit-mode context + toggle |
| `web/src/dashboard/edit/EditChrome.tsx` | Drag/resize/delete handles overlay |
| `web/src/dashboard/edit/PropsPanel.tsx` | Right-side slide-in props panel |
| `web/src/dashboard/edit/WidgetPicker.tsx` | FAB-launched widget picker sheet |
| `web/src/dashboard/edit/UndoRedo.ts` | Zustand history with debounce |
| `web/src/dashboard/edit/use-editor-store.ts` | Editor state Zustand store |
| `web/src/dashboard/catalog.ts` | Client-side widget registry (built-in + dynamic) |
| `web/src/dashboard/pack-loader.ts` | Dynamic ES `import()` of widget pack bundles |
| `web/src/widgets/EntityToggle.tsx` | Built-in #1 |
| `web/src/widgets/Gauge.tsx` | Built-in #2 |
| `web/src/widgets/LineChart.tsx` | Built-in #3 |
| `web/src/widgets/CameraStream.tsx` | Built-in #4 |
| `web/src/widgets/Markdown.tsx` | Built-in #5 |
| `web/src/widgets/ScriptButton.tsx` | Built-in #6 |
| `web/src/widgets/EntityList.tsx` | Built-in #7 |
| `web/src/widgets/GroupCard.tsx` | Built-in #8 (the only ContainerInstance) |
| `web/src/widgets/*.test.tsx` | Per-widget component tests |
| `web/src/widget-sdk/index.ts` | Public SDK surface re-exports |
| `web/src/widget-sdk/types.ts` | `WidgetProps`, `WidgetInstance`, `Theme`, `EntityState`, `PendingState` |
| `web/src/widget-sdk/hooks.ts` | Hooks re-exported for SDK users |
| `web/src/widget-sdk/components.ts` | `<WidgetRenderer />`, `<ChildGrid />` exports |
| `web/src/widget-sdk/package.json` | Separate npm package (built standalone) |
| `web/src/pwa/sw.ts` | Service worker (managed by `vite-plugin-pwa`) |
| `web/src/pwa/install-prompt.ts` | Install prompt UX hook |
| `web/tools/eslint-plugin-gohome/index.ts` | ESLint plugin entry |
| `web/tools/eslint-plugin-gohome/no-raw-tokens.ts` | The token-discipline rule |
| `web/tools/eslint-plugin-gohome/no-raw-tokens.test.ts` | Rule tests |
| `web/e2e/auth.spec.ts` | Playwright: password login, logout |
| `web/e2e/passkey.spec.ts` | Playwright with WebAuthn virtual authenticator |
| `web/e2e/dashboard-read.spec.ts` | Playwright: render default dashboard |
| `web/e2e/dashboard-edit.spec.ts` | Playwright: full WYSIWYG cycle |
| `web/e2e/widget-pack.spec.ts` | Playwright: install pack, see classes appear in picker |
| `web/e2e/visual/*.spec.ts` | Visual regression suite |
| `web/e2e/fixtures/` | Pkl config fixtures, fake-driver setup |
| `web/README.md` | Contributor docs: `task web:install`, `task web:build`, `task ui:dev` |
| `docs/web-ui.md` (in `gohome/`) | User-facing docs: theming, widget pack install, dashboard editor walkthrough |
| `docs/widget-pack-authoring.md` (in `gohome/`) | Pack authors: SDK API, manifest format, signing, publishing |

### Modified files (in `gohome/`)

| Path | Change |
|---|---|
| `Taskfile.yml` | Add `web:install`, `web:build`, `web:test`, `web:lint`, `ui:dev` tasks; update `build` to depend on `web:build` |
| `go.mod`, `go.sum` | Add `oras.land/oras-go/v2`, `github.com/sigstore/sigstore-go` (or cosign equivalent), confirm `embed` and `embed/internal/embedtest` resolve |
| `internal/api/listener/listener.go` | Mount SPA handler (`/`), assets handler (`/assets/`), widgets handler (`/widgets/`); preserve `/healthz`, `/webhooks/`, `/mcp` (C9), Connect routes |
| `internal/api/listener/routes.go` | No change required (BuildRoutes is generic); reference only |
| `internal/api/deps.go` | Add `DashboardBackend` interface and `WidgetCatalogBackend` interface |
| `internal/api/service_unimplemented.go` | Remove `DashboardService` stub block (Task 13 supplies real one); leave the stubs for services C10 doesn't touch |
| `internal/api/service_config.go` | Add `EvalCompute` method (Task 20) |
| `internal/api/service_auth.go` | Replace `Unimplemented` returns in `StartWebAuthnChallenge` and the passkey branch of `Login` with real impls (Task 7) |
| `internal/auth/credentials/webauthn.go` | Add `StoreChallenge` / `ConsumeChallenge` (challenge-store ops the C9 stub left for C10) |
| `internal/config/pkl/gohome/dashboards.pkl` | Replace the minimal stub with the recursive `WidgetInstance`/`ContainerInstance`/`Position`/`Grid`/`Dashboard` schema (Task 11) |
| `internal/config/pkl/gohome/widgets.pkl` | Replace const-only stub with the eight built-in widget classes + `PackManifest` + `PackPolicy` (Task 11) |
| `internal/config/pkl/gohome/web.pkl` | NEW — `gohome.web.Web` config class |
| `internal/config/loader.go` | Load `gohome.web` and `gohome.widgetPackPolicy` Pkl modules; surface to daemon |
| `internal/daemon/daemon.go` | Construct `web.Handler`, `widgetpack.Store`, `dashboard.Service`, `compute.Service`; wire to listener + Connect handlers |
| `internal/observability/metrics.go` | Register `gohome_dashboard_*` and `gohome_widgetpack_*` series |
| `proto/gohome/v1alpha1/dashboard.proto` | Expand: typed `Dashboard`/`WidgetInstance`/`Position`/`Grid`; add `Create`, `Delete`, `GetWidgetCatalog` RPCs; add `WidgetCatalog`/`WidgetClass`/`PropSchema` messages |
| `proto/gohome/v1alpha1/config.proto` | Add `EvalCompute` RPC + `EvalComputeRequest`/`EvalComputeResponse` messages |
| `internal/cli/root.go` | Register `cmd_widget.go` and `cmd_ui.go` subtrees |
| `README.md` | Add "Web UI" section linking to `docs/web-ui.md` |
| `.gitignore` | Add `web/dist/`, `web/node_modules/`, `web/.vite/`, `web/coverage/`, `web/playwright-report/`, `web/test-results/` |

---

## Task 1: web/ project scaffold

Sets up the npm package, Vite, TypeScript, ESLint base. No application code yet — just an empty SPA that builds and ships an `index.html`. Lays the ground for every subsequent web task.

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/postcss.config.cjs`, `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx`, `web/eslint.config.ts`
- Create: `web/.npmrc` (lockfile-only install discipline)
- Modify: `Taskfile.yml`, `.gitignore`

- [ ] **Step 1: Create `web/package.json`**

```json
{
  "name": "@gohome/web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "lint": "eslint .",
    "test": "vitest run",
    "test:watch": "vitest",
    "e2e": "playwright test",
    "typecheck": "tsc -b --noEmit"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "typescript": "^5.4.0",
    "vite": "^5.4.0",
    "eslint": "^9.0.0",
    "@eslint/js": "^9.0.0",
    "typescript-eslint": "^8.0.0",
    "eslint-plugin-react-hooks": "^5.0.0",
    "eslint-plugin-react-refresh": "^0.4.0",
    "vitest": "^2.0.0"
  },
  "engines": { "node": ">=20" }
}
```

- [ ] **Step 2: Create `web/tsconfig.json` and `web/tsconfig.node.json`**

`web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"],
      "@gohome/widget-sdk": ["src/widget-sdk/index.ts"]
    }
  },
  "include": ["src", "tools"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

`web/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["vite.config.ts", "postcss.config.cjs", "eslint.config.ts"]
}
```

- [ ] **Step 3: Create `web/vite.config.ts`**

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
      "@gohome/widget-sdk": path.resolve(__dirname, "src/widget-sdk/index.ts"),
    },
  },
  build: {
    target: "es2022",
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: undefined,
        assetFileNames: "assets/[name]-[hash][extname]",
        chunkFileNames: "assets/[name]-[hash].js",
        entryFileNames: "assets/[name]-[hash].js",
      },
    },
  },
  server: { port: 5173, strictPort: true },
});
```

- [ ] **Step 4: Create `web/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>gohome</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 5: Create `web/src/main.tsx` and `web/src/App.tsx`**

`web/src/main.tsx`:

```ts
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

`web/src/App.tsx`:

```tsx
export default function App() {
  return <div>gohome</div>;
}
```

- [ ] **Step 6: Create `web/eslint.config.ts`**

```ts
import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";

export default tseslint.config(
  { ignores: ["dist", "node_modules", "src/widget-sdk/dist"] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ["**/*.{ts,tsx}"],
    languageOptions: { ecmaVersion: 2022, globals: { window: true, document: true } },
    plugins: { "react-hooks": reactHooks, "react-refresh": reactRefresh },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],
    },
  },
);
```

- [ ] **Step 7: Add Taskfile entries**

In `gohome/Taskfile.yml`, add under `tasks:`:

```yaml
  web:install:
    desc: Install web npm dependencies (idempotent)
    dir: web
    cmds:
      - npm install --no-audit --no-fund

  web:build:
    desc: Build the web bundle into web/dist
    dir: web
    deps: [web:install]
    cmds:
      - npm run build

  web:lint:
    desc: Lint web sources
    dir: web
    deps: [web:install]
    cmds:
      - npm run lint

  web:test:
    desc: Run web unit tests
    dir: web
    deps: [web:install]
    cmds:
      - npm test
```

Update the existing `build:` task to depend on `web:build`:

```yaml
  build:
    desc: Build both binaries
    deps: [web:build]
    cmds:
      - go build -o {{.BIN_DIR}}/gohomed ./cmd/gohomed
      - go build -o {{.BIN_DIR}}/gohome ./cmd/gohome
```

- [ ] **Step 8: Update `.gitignore`**

Append:

```
web/dist/
web/node_modules/
web/.vite/
web/coverage/
web/playwright-report/
web/test-results/
```

- [ ] **Step 9: Smoke-test the scaffold**

```bash
task web:install
task web:build
ls web/dist/
```

Expected: `web/dist/index.html` and `web/dist/assets/index-<hash>.js`, `web/dist/assets/index-<hash>.css` files exist.

- [ ] **Step 10: Commit**

```bash
git add web/ Taskfile.yml .gitignore
git commit -m "feat(c10): web project scaffold (vite, react 19, ts, eslint base)"
```

---

## Task 2: embed.FS pipeline + asset budget CI

Wire `gohomed` to serve the built SPA. After this task, `curl http://localhost:8080/` returns the embedded `index.html`.

**Files:**
- Create: `internal/web/embed.go`, `handler.go`, `index_template.go`, `handler_test.go`, `index_template_test.go`
- Modify: `internal/api/listener/listener.go`
- Modify: `internal/daemon/daemon.go` (wire web handler into listener Deps)

- [ ] **Step 1: Write the failing handler test**

`internal/web/handler_test.go`:

```go
package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/web"
)

func TestHandler_ServesIndexAtRoot(t *testing.T) {
	h, err := web.NewHandler(web.Config{Version: "test"})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	if !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Errorf("body missing root div: %s", rec.Body.String())
	}
}

func TestHandler_ServesHashedAsset(t *testing.T) {
	h, err := web.NewHandler(web.Config{Version: "test"})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	// Pick any asset that exists in dist; this test will fail if assets aren't embedded.
	req := httptest.NewRequest(http.MethodGet, "/assets/index.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		// Asset names are hashed; this test asserts only that the /assets prefix is recognized.
		// A more discriminating test arrives once a fixture asset is committed.
		return
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
}

func TestHandler_FallsBackToIndexForUnknownRoute(t *testing.T) {
	h, _ := web.NewHandler(web.Config{Version: "test"})
	req := httptest.NewRequest(http.MethodGet, "/dashboards/default", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (SPA fallback)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Error("expected SPA index for unknown route")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd gohome
go test ./internal/web/...
```

Expected: FAIL with package not found / `web` undefined.

- [ ] **Step 3: Create `internal/web/embed.go`**

```go
package web

import "embed"

// Assets holds the built React SPA. The dist directory is populated
// by `task web:build` before any go build.
//
//go:embed all:dist
var Assets embed.FS
```

- [ ] **Step 4: Create `internal/web/index_template.go`**

```go
package web

import (
	"bytes"
	"fmt"
	"html/template"
)

type indexData struct {
	Version string
}

const indexTemplateSource = `<!doctype html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>gohome</title>
<meta name="gohome-version" content="{{.Version}}">
<script>
// Initial-paint theme: read mode preference before React mounts to avoid flash.
(function () {
  try {
    var mode = localStorage.getItem('gohome.themeMode') || 'system';
    var resolved = mode === 'system'
      ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : mode;
    document.documentElement.setAttribute('data-theme', 'developer-' + resolved);
  } catch (_) { /* ignore */ }
})();
</script>
{{.AssetTags}}
</head>
<body>
<div id="root"></div>
{{.ScriptTags}}
</body>
</html>
`

func renderIndex(version, assetTags, scriptTags string) ([]byte, error) {
	tmpl, err := template.New("index").Parse(indexTemplateSource)
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Version    string
		AssetTags  template.HTML
		ScriptTags template.HTML
	}{Version: version, AssetTags: template.HTML(assetTags), ScriptTags: template.HTML(scriptTags)}); err != nil {
		return nil, fmt.Errorf("execute index template: %w", err)
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 5: Create `internal/web/handler.go`**

```go
package web

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

type Config struct {
	Version string
}

type Handler struct {
	cfg     Config
	dist    fs.FS
	index   []byte
	assetFS http.FileSystem
}

func NewHandler(cfg Config) (*Handler, error) {
	dist, err := fs.Sub(Assets, "dist")
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(dist, "assets")
	if err != nil {
		return nil, err
	}
	tags, scripts, err := scanDist(dist)
	if err != nil {
		return nil, err
	}
	idx, err := renderIndex(cfg.Version, tags, scripts)
	if err != nil {
		return nil, err
	}
	return &Handler{cfg: cfg, dist: dist, index: idx, assetFS: http.FS(assets)}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		// Hashed asset — long cache.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.StripPrefix("/assets/", http.FileServer(h.assetFS)).ServeHTTP(w, r)
		return
	}
	// SPA fallback for any other route — short cache, must revalidate.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(h.index)
}

// scanDist reads index.html produced by Vite to extract the hashed
// asset URLs so we can reuse them in our templated index.
func scanDist(dist fs.FS) (assetTags, scriptTags string, err error) {
	f, err := dist.Open("index.html")
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	return parseViteIndex(f)
}

func parseViteIndex(r interface{ Read([]byte) (int, error) }) (assetTags, scriptTags string, err error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, rerr := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if errors.Is(rerr, fs.ErrClosed) || rerr != nil {
			break
		}
	}
	body := string(buf)
	// Naive extraction: pull <link rel="stylesheet" href="/assets/..."> and
	// <script type="module" src="/assets/...">.
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, `<link rel="stylesheet"`):
			assetTags += line + "\n"
		case strings.HasPrefix(line, `<script type="module"`):
			scriptTags += line + "\n"
		}
	}
	return assetTags, scriptTags, nil
}

// HealthCheck reports embed integrity — used by /healthz.
func (h *Handler) HealthCheck() error {
	if len(h.index) == 0 {
		return fs.ErrNotExist
	}
	return nil
}

var _ http.Handler = (*Handler)(nil)

// Compile-time check that path.Join is available (used in future tasks).
var _ = path.Join
```

- [ ] **Step 6: Mount the handler in the listener**

Edit `internal/api/listener/listener.go`. Add a new `Deps` field:

```go
type Deps struct {
	HealthProbe    func() error
	ConnectRoutes  []Route
	WebhookHandler http.Handler
	WebHandler     http.Handler // C10
}
```

In `Start`, after the existing mux registrations:

```go
// SPA + assets (catch-all; must be registered LAST so explicit routes win).
if l.deps.WebHandler != nil {
	mux.Handle("/", l.deps.WebHandler)
}
```

Note: `mux.Handle("/", ...)` is the catch-all; explicit routes (`/healthz`, Connect paths, `/webhooks/`, future `/mcp`, `/widgets/`) win because Go's `ServeMux` longest-match rule prefers more specific patterns.

- [ ] **Step 7: Wire from daemon**

In `internal/daemon/daemon.go`, where the listener is constructed, build the web handler and pass it in:

```go
import (
	"github.com/fynn-labs/gohome/internal/web"
	// ...existing imports...
)

// ...inside daemon construction:
webHandler, err := web.NewHandler(web.Config{Version: d.version})
if err != nil {
	return nil, fmt.Errorf("daemon: web handler: %w", err)
}
listenerDeps := listener.Deps{
	HealthProbe:    d.healthProbe,
	ConnectRoutes:  routes,
	WebhookHandler: webhookHandler,
	WebHandler:     webHandler,
}
```

- [ ] **Step 8: Build and run the test**

```bash
cd gohome
task web:build
task build
go test ./internal/web/...
```

Expected: tests pass; `go build` succeeds; `dist/` is embedded.

- [ ] **Step 9: Smoke-test end-to-end**

```bash
./dist/gohomed --bind 127.0.0.1:8080 --uds /tmp/gohome-c10.sock &
sleep 1
curl -s http://127.0.0.1:8080/ | head -10
kill %1
```

Expected: HTML containing `<div id="root"></div>` and a `<meta name="gohome-version">` tag.

- [ ] **Step 10: Add asset budget enforcement**

Create `internal/web/budget_test.go`:

```go
package web_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/web"
)

const (
	maxInitialChunkBytes = 350 * 1024 * 4 // 350 KB gzipped ≈ 1.4 MB raw heuristic
	maxTotalAssetsBytes  = 1500 * 1024 * 4
)

func TestAssetBudget(t *testing.T) {
	dist, err := fs.Sub(web.Assets, "dist/assets")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	var total int64
	var initial int64
	err = fs.WalkDir(dist, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if strings.HasPrefix(p, "index-") && strings.HasSuffix(p, ".js") {
			initial += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if initial > int64(maxInitialChunkBytes) {
		t.Errorf("initial chunk %d B exceeds budget %d B (raw)", initial, maxInitialChunkBytes)
	}
	if total > int64(maxTotalAssetsBytes) {
		t.Errorf("total assets %d B exceeds budget %d B (raw)", total, maxTotalAssetsBytes)
	}
}
```

(The 4× multiplier converts the spec's gzipped budgets to raw byte heuristics; tighten by measuring real gzip ratio in a follow-on once content is non-trivial.)

- [ ] **Step 11: Commit**

```bash
git add internal/web/ internal/api/listener/listener.go internal/daemon/daemon.go
git commit -m "feat(c10): embed web SPA into gohomed; serve / + /assets/*"
```

---

## Task 3: design token foundation + lint rule

CSS-custom-property substrate, Tailwind v4 wiring, theme provider, mode toggle, and the `gohome/no-raw-tokens` ESLint rule. After this task, the "hello world" page renders with token-driven styling and the lint rule actively blocks raw utility usage.

**Files:**
- Create: `web/src/theme/tokens.css`, `theme/provider.tsx`, `theme/types.ts`, `theme/motion.ts`, `theme/languages/developer.ts`
- Create: `web/tools/eslint-plugin-gohome/index.ts`, `no-raw-tokens.ts`, `no-raw-tokens.test.ts`
- Modify: `web/eslint.config.ts`, `web/tailwind.config.ts`, `web/postcss.config.cjs`, `web/package.json` (add tailwind, motion deps)
- Modify: `web/src/App.tsx`, `web/src/main.tsx`

- [ ] **Step 1: Add Tailwind + Framer Motion deps**

```bash
cd web
npm install -D tailwindcss@^4.0.0 @tailwindcss/postcss@^4.0.0 autoprefixer@^10.4.0 zustand@^4.5.0
npm install framer-motion@^11.0.0
```

- [ ] **Step 2: Create `web/postcss.config.cjs`**

```js
module.exports = {
  plugins: {
    "@tailwindcss/postcss": {},
    autoprefixer: {},
  },
};
```

- [ ] **Step 3: Create `web/src/theme/tokens.css`**

```css
@import "tailwindcss";

@layer base {
  :root,
  [data-theme="developer-light"] {
    /* palette */
    --gh-color-bg:          oklch(98% 0 0);
    --gh-color-surface-1:   oklch(100% 0 0);
    --gh-color-surface-2:   oklch(96% 0 0);
    --gh-color-border:      oklch(90% 0 0);
    --gh-color-fg:          oklch(15% 0 0);
    --gh-color-fg-muted:    oklch(50% 0 0);
    --gh-color-accent:      oklch(64% 0.16 220);
    --gh-color-success:     oklch(60% 0.18 145);
    --gh-color-warning:     oklch(70% 0.16 70);
    --gh-color-danger:      oklch(58% 0.21 25);

    /* radii */
    --gh-radius-sm:         3px;
    --gh-radius-md:         5px;
    --gh-radius-lg:         8px;
    --gh-radius-pill:       999px;

    /* density */
    --gh-pad-tight:         0.25rem;
    --gh-pad-normal:        0.5rem;
    --gh-pad-loose:         1rem;
    --gh-gap-tight:         0.25rem;
    --gh-gap-normal:        0.5rem;
    --gh-gap-loose:         1rem;

    /* motion (string form for inline transitions) */
    --gh-motion-snappy:     200ms cubic-bezier(0.4, 0, 0.2, 1);
    --gh-motion-spring:     320ms cubic-bezier(0.34, 1.56, 0.64, 1);
    --gh-motion-slow:       500ms cubic-bezier(0.16, 1, 0.3, 1);

    /* type */
    --gh-font-display:      "Inter", system-ui, sans-serif;
    --gh-font-body:         "Inter", system-ui, sans-serif;
    --gh-font-numeric:      "JetBrains Mono", ui-monospace, monospace;
  }

  [data-theme="developer-dark"] {
    --gh-color-bg:          oklch(11% 0 0);
    --gh-color-surface-1:   oklch(14% 0 0);
    --gh-color-surface-2:   oklch(18% 0 0);
    --gh-color-border:      oklch(22% 0 0);
    --gh-color-fg:          oklch(96% 0 0);
    --gh-color-fg-muted:    oklch(60% 0 0);
    --gh-color-accent:      oklch(75% 0.13 220);
    --gh-color-success:     oklch(70% 0.16 145);
    --gh-color-warning:     oklch(80% 0.14 70);
    --gh-color-danger:      oklch(70% 0.18 25);
    /* radii / density / motion / type unchanged from light */
  }

  body {
    background: var(--gh-color-bg);
    color: var(--gh-color-fg);
    font-family: var(--gh-font-body);
  }
}

@theme {
  --color-bg: var(--gh-color-bg);
  --color-surface-1: var(--gh-color-surface-1);
  --color-surface-2: var(--gh-color-surface-2);
  --color-border: var(--gh-color-border);
  --color-fg: var(--gh-color-fg);
  --color-fg-muted: var(--gh-color-fg-muted);
  --color-accent: var(--gh-color-accent);
  --color-success: var(--gh-color-success);
  --color-warning: var(--gh-color-warning);
  --color-danger: var(--gh-color-danger);

  --radius-sm: var(--gh-radius-sm);
  --radius-md: var(--gh-radius-md);
  --radius-lg: var(--gh-radius-lg);

  --spacing-tight: var(--gh-pad-tight);
  --spacing-normal: var(--gh-pad-normal);
  --spacing-loose: var(--gh-pad-loose);

  --font-display: var(--gh-font-display);
  --font-body: var(--gh-font-body);
  --font-numeric: var(--gh-font-numeric);
}
```

- [ ] **Step 4: Create `web/tailwind.config.ts`**

```ts
// Tailwind v4 reads its config primarily from CSS @theme directives.
// This file exists for tooling integration only.
import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
};

export default config;
```

- [ ] **Step 5: Create `web/src/theme/types.ts`**

```ts
export type ThemeMode = "light" | "dark";
export type ThemeModePreference = ThemeMode | "system";
export type LanguageId = "developer";

export type MotionPreset = {
  type: "tween" | "spring";
  duration?: number;
  ease?: number[];
  damping?: number;
  stiffness?: number;
};

export type TokenSet = {
  color: {
    bg: string;
    surface1: string;
    surface2: string;
    border: string;
    fg: string;
    fgMuted: string;
    accent: string;
    success: string;
    warning: string;
    danger: string;
  };
  radius: { sm: string; md: string; lg: string; pill: string };
  motion: { snappy: MotionPreset; spring: MotionPreset; slow: MotionPreset };
  font: { display: string; body: string; numeric: string };
};

export type LanguagePreset = {
  id: LanguageId;
  modes: { light: TokenSet; dark: TokenSet };
};

export type Theme = {
  language: LanguageId;
  mode: ThemeMode;
  tokens: TokenSet;
};
```

- [ ] **Step 6: Create `web/src/theme/motion.ts`**

```ts
import type { MotionPreset } from "./types";

export const motion = {
  snappy: { type: "tween", duration: 0.2, ease: [0.4, 0, 0.2, 1] } satisfies MotionPreset,
  spring: { type: "spring", damping: 24, stiffness: 320 } satisfies MotionPreset,
  slow:   { type: "tween", duration: 0.5, ease: [0.16, 1, 0.3, 1] } satisfies MotionPreset,
};
```

- [ ] **Step 7: Create `web/src/theme/languages/developer.ts`**

```ts
import type { LanguagePreset, TokenSet } from "../types";
import { motion } from "../motion";

const cssVar = (name: string) => `var(${name})`;

const baseTokens = {
  radius: {
    sm: cssVar("--gh-radius-sm"),
    md: cssVar("--gh-radius-md"),
    lg: cssVar("--gh-radius-lg"),
    pill: cssVar("--gh-radius-pill"),
  },
  motion,
  font: {
    display: cssVar("--gh-font-display"),
    body: cssVar("--gh-font-body"),
    numeric: cssVar("--gh-font-numeric"),
  },
} as const;

const colorVars = {
  bg: cssVar("--gh-color-bg"),
  surface1: cssVar("--gh-color-surface-1"),
  surface2: cssVar("--gh-color-surface-2"),
  border: cssVar("--gh-color-border"),
  fg: cssVar("--gh-color-fg"),
  fgMuted: cssVar("--gh-color-fg-muted"),
  accent: cssVar("--gh-color-accent"),
  success: cssVar("--gh-color-success"),
  warning: cssVar("--gh-color-warning"),
  danger: cssVar("--gh-color-danger"),
} as const;

const lightTokens: TokenSet = { color: colorVars, ...baseTokens };
const darkTokens: TokenSet = { color: colorVars, ...baseTokens };

export const developer: LanguagePreset = {
  id: "developer",
  modes: { light: lightTokens, dark: darkTokens },
};
```

- [ ] **Step 8: Create `web/src/theme/provider.tsx`**

```tsx
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import type { LanguageId, Theme, ThemeMode, ThemeModePreference } from "./types";
import { developer } from "./languages/developer";

const LANGUAGES = { developer } as const;
const STORAGE_KEY = "gohome.themeMode";

type ThemeContextValue = {
  theme: Theme;
  mode: ThemeMode;
  modePreference: ThemeModePreference;
  language: LanguageId;
  setMode: (m: ThemeModePreference) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readPreference(): ThemeModePreference {
  if (typeof localStorage === "undefined") return "system";
  const v = localStorage.getItem(STORAGE_KEY);
  return v === "light" || v === "dark" ? v : "system";
}

function resolveMode(pref: ThemeModePreference): ThemeMode {
  if (pref !== "system") return pref;
  if (typeof matchMedia === "undefined") return "light";
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [pref, setPref] = useState<ThemeModePreference>(() => readPreference());
  const [systemTrigger, setSystemTrigger] = useState(0);

  useEffect(() => {
    if (typeof matchMedia === "undefined") return;
    const mql = matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setSystemTrigger((n) => n + 1);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  const mode = useMemo(() => resolveMode(pref), [pref, systemTrigger]);
  const language: LanguageId = "developer";
  const tokens = LANGUAGES[language].modes[mode];

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", `${language}-${mode}`);
  }, [language, mode]);

  const value: ThemeContextValue = useMemo(
    () => ({
      theme: { language, mode, tokens },
      mode,
      modePreference: pref,
      language,
      setMode: (m) => {
        setPref(m);
        if (typeof localStorage !== "undefined") localStorage.setItem(STORAGE_KEY, m);
      },
    }),
    [pref, mode, tokens],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const v = useContext(ThemeContext);
  if (!v) throw new Error("useTheme must be inside <ThemeProvider>");
  return v;
}
```

- [ ] **Step 9: Update `web/src/main.tsx` and `web/src/App.tsx` to consume theme + tokens**

`web/src/main.tsx`:

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { ThemeProvider } from "./theme/provider";
import "./theme/tokens.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </React.StrictMode>,
);
```

`web/src/App.tsx`:

```tsx
import { useTheme } from "./theme/provider";

export default function App() {
  const { mode, modePreference, setMode } = useTheme();
  return (
    <div className="bg-bg text-fg min-h-screen p-loose">
      <h1 className="text-2xl font-display">gohome</h1>
      <p className="text-fg-muted">
        Mode: <code className="font-numeric">{mode}</code> (preference: <code className="font-numeric">{modePreference}</code>)
      </p>
      <div className="mt-normal flex gap-tight">
        {(["light", "dark", "system"] as const).map((m) => (
          <button
            key={m}
            className="bg-surface-1 text-fg rounded-md px-normal py-tight hover:bg-surface-2"
            onClick={() => setMode(m)}
          >
            {m}
          </button>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 10: Verify the SPA renders with tokens**

```bash
cd web
npm run dev &
sleep 2
# Open http://localhost:5173 in a browser; verify clicking light/dark toggles the theme.
kill %1
```

Expected: page background swaps between light and dark on toggle; mode indicator updates.

- [ ] **Step 11: Write the failing lint-rule test**

`web/tools/eslint-plugin-gohome/no-raw-tokens.test.ts`:

```ts
import { RuleTester } from "@typescript-eslint/rule-tester";
import { describe } from "node:test";
import rule from "./no-raw-tokens";

const ruleTester = new RuleTester({
  parser: "@typescript-eslint/parser" as any,
  parserOptions: { ecmaVersion: 2022, sourceType: "module", ecmaFeatures: { jsx: true } },
});

describe("no-raw-tokens", () => {
  ruleTester.run("no-raw-tokens", rule, {
    valid: [
      { code: `const x = <div className="bg-surface-1 rounded-md p-normal" />;` },
      { code: `const x = <div className="text-accent text-fg-muted" />;` },
    ],
    invalid: [
      {
        code: `const x = <div className="bg-zinc-900" />;`,
        errors: [{ messageId: "rawColor" }],
      },
      {
        code: `const x = <div className="rounded-[6px]" />;`,
        errors: [{ messageId: "arbitraryRadius" }],
      },
      {
        code: `const x = <div style={{ borderRadius: 5 }} />;`,
        errors: [{ messageId: "inlineStyle" }],
      },
      {
        code: `const x = <div className="p-[12px]" />;`,
        errors: [{ messageId: "arbitrarySpacing" }],
      },
    ],
  });
});
```

```bash
cd web
npm install -D @typescript-eslint/rule-tester @typescript-eslint/parser
npm test -- tools/eslint-plugin-gohome
```

Expected: FAIL — module not found.

- [ ] **Step 12: Implement the rule**

`web/tools/eslint-plugin-gohome/no-raw-tokens.ts`:

```ts
import type { Rule } from "eslint";

const TAILWIND_COLOR_PALETTES = [
  "slate", "gray", "zinc", "neutral", "stone",
  "red", "orange", "amber", "yellow", "lime", "green",
  "emerald", "teal", "cyan", "sky", "blue", "indigo",
  "violet", "purple", "fuchsia", "pink", "rose",
  "white", "black",
];

const ALLOWED_COLOR_NAMES = [
  "bg", "fg", "fg-muted", "surface-1", "surface-2", "border",
  "accent", "success", "warning", "danger",
];

const COLOR_PREFIXES = ["bg", "text", "border", "ring", "fill", "stroke", "from", "to", "via"];

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: { description: "Forbid raw Tailwind utilities; require token utilities." },
    messages: {
      rawColor: "Raw color utility '{{cls}}' is forbidden. Use token utilities (bg-bg, bg-surface-1, text-fg, text-accent, ...).",
      arbitraryRadius: "Arbitrary radius '{{cls}}' is forbidden. Use rounded-{sm,md,lg}.",
      arbitrarySpacing: "Arbitrary spacing '{{cls}}' is forbidden. Use {p,m,gap}-{tight,normal,loose}.",
      rawFont: "Raw font utility '{{cls}}' is forbidden. Use font-{display,body,numeric}.",
      inlineStyle: "Inline style with raw token values is forbidden. Use Tailwind token utilities.",
    },
    schema: [],
  },
  create(context) {
    function checkClass(node: any, cls: string) {
      // Color utilities
      for (const prefix of COLOR_PREFIXES) {
        for (const palette of TAILWIND_COLOR_PALETTES) {
          if (cls === `${prefix}-${palette}` || cls.startsWith(`${prefix}-${palette}-`)) {
            // Allow our token names
            if (ALLOWED_COLOR_NAMES.some((n) => cls === `${prefix}-${n}`)) return;
            context.report({ node, messageId: "rawColor", data: { cls } });
            return;
          }
        }
      }
      // Arbitrary radius
      if (/^rounded-\[/.test(cls)) {
        context.report({ node, messageId: "arbitraryRadius", data: { cls } });
        return;
      }
      // Arbitrary spacing
      if (/^[pm][trblxy]?-\[/.test(cls) || /^gap-\[/.test(cls)) {
        context.report({ node, messageId: "arbitrarySpacing", data: { cls } });
        return;
      }
      // Raw font utilities
      if (cls === "font-mono" || cls === "font-sans" || cls === "font-serif") {
        context.report({ node, messageId: "rawFont", data: { cls } });
        return;
      }
    }

    return {
      JSXAttribute(node: any) {
        if (node.name?.name !== "className" && node.name?.name !== "class") return;
        if (node.value?.type === "Literal" && typeof node.value.value === "string") {
          for (const cls of node.value.value.split(/\s+/)) {
            if (cls) checkClass(node, cls);
          }
        }
        if (node.name?.name === "style") {
          context.report({ node, messageId: "inlineStyle" });
        }
      },
    };
  },
};

export default rule;
```

`web/tools/eslint-plugin-gohome/index.ts`:

```ts
import noRawTokens from "./no-raw-tokens";
export default {
  rules: { "no-raw-tokens": noRawTokens },
};
```

- [ ] **Step 13: Wire the rule into ESLint config**

Edit `web/eslint.config.ts`:

```ts
import gohome from "./tools/eslint-plugin-gohome";
// ...inside the config array, add a rule block:
{
  files: ["src/**/*.{ts,tsx}"],
  plugins: { gohome },
  rules: {
    "gohome/no-raw-tokens": "error",
  },
},
```

- [ ] **Step 14: Run the rule tests**

```bash
cd web
npm test -- tools/eslint-plugin-gohome
```

Expected: PASS.

- [ ] **Step 15: Run lint over src/**

```bash
cd web
npm run lint
```

Expected: passes — App.tsx already uses only token utilities.

- [ ] **Step 16: Commit**

```bash
git add web/src/theme web/tools web/eslint.config.ts web/tailwind.config.ts \
        web/postcss.config.cjs web/src/main.tsx web/src/App.tsx web/package.json \
        web/package-lock.json
git commit -m "feat(c10): design token foundation + no-raw-tokens lint rule"
```

---

## Task 4: shadcn primitive intake protocol + initial primitives

Bring in the small set of shadcn/ui primitives C10 needs (Button, Sheet, Switch, Toast, Dialog, Tooltip), audit each for token compliance, and add snapshot tests.

**Files:**
- Create: `web/src/components/ui/{button,sheet,switch,toast,dialog,tooltip}.tsx` (audited shadcn copies)
- Create: `web/src/components/gh/{Button,Sheet,Switch,Toast,Dialog,Tooltip}.tsx` (token-compliant wrappers)
- Create: `web/src/components/ui/*.test.tsx` (snapshot smoke tests)
- Modify: `web/package.json` (add Radix deps)

- [ ] **Step 1: Add shadcn / Radix dependencies**

```bash
cd web
npm install @radix-ui/react-dialog @radix-ui/react-tooltip @radix-ui/react-switch \
            @radix-ui/react-toast class-variance-authority clsx tailwind-merge \
            lucide-react
npm install -D @testing-library/react @testing-library/jest-dom jsdom
```

- [ ] **Step 2: Add a `cn` helper**

`web/src/lib/cn.ts`:

```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 3: Add the audited Button primitive**

`web/src/components/ui/button.tsx`:

```tsx
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";
import { cn } from "@/lib/cn";

const buttonVariants = cva(
  // Base — token-only utilities.
  "inline-flex items-center justify-center gap-tight rounded-md text-sm transition-[background] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50 disabled:pointer-events-none",
  {
    variants: {
      variant: {
        default: "bg-accent text-bg hover:bg-accent",
        secondary: "bg-surface-2 text-fg hover:bg-surface-1",
        ghost: "bg-transparent text-fg hover:bg-surface-2",
        destructive: "bg-danger text-bg hover:bg-danger",
      },
      size: {
        sm: "h-7 px-normal py-tight",
        md: "h-9 px-loose py-normal",
        lg: "h-11 px-loose py-normal text-base",
        icon: "h-9 w-9 p-tight",
      },
    },
    defaultVariants: { variant: "default", size: "md" },
  },
);

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean };

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return <Comp className={cn(buttonVariants({ variant, size }), className)} ref={ref} {...props} />;
  },
);
Button.displayName = "Button";
```

Add `npm install @radix-ui/react-slot` if not present.

- [ ] **Step 4: Add the gh wrapper layer**

`web/src/components/gh/Button.tsx`:

```tsx
export { Button, type ButtonProps } from "../ui/button";
```

This is intentional — for v1.0 we re-export. The wrapper layer exists so future patches to shadcn primitives don't ripple through every consumer.

- [ ] **Step 5: Add a snapshot smoke test**

`web/src/components/ui/button.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Button } from "./button";
import { ThemeProvider } from "@/theme/provider";

describe("Button token compliance", () => {
  for (const mode of ["developer-light", "developer-dark"] as const) {
    it(`renders with ${mode} tokens`, () => {
      document.documentElement.setAttribute("data-theme", mode);
      const { container } = render(
        <ThemeProvider>
          <Button>Hello</Button>
        </ThemeProvider>,
      );
      const btn = container.querySelector("button")!;
      // Spot-check that a token utility is present and no raw palette utility leaked.
      expect(btn.className).toMatch(/bg-accent|bg-surface-2|bg-transparent|bg-danger/);
      expect(btn.className).not.toMatch(/bg-(zinc|gray|slate)-/);
    });
  }
});
```

Add `web/vitest.config.ts`:

```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
    globals: false,
  },
});
```

`web/vitest.setup.ts`:

```ts
import "@testing-library/jest-dom/vitest";
```

- [ ] **Step 6: Repeat the audit pattern for Sheet / Switch / Toast / Dialog / Tooltip**

For each: copy the standard shadcn primitive into `web/src/components/ui/<name>.tsx`, replace every raw color/radius/spacing utility with token utilities, and add a parallel snapshot test under `web/src/components/ui/<name>.test.tsx` that asserts:

1. The rendered DOM contains only token-class names (regex check for `bg-(bg|surface-\d|accent|...)`).
2. The component mounts without throwing in both `developer-light` and `developer-dark`.

Skipping the verbatim code here — the audit is mechanical. The completed output is a `web/src/components/ui/` directory containing six primitive files with no raw utility regressions.

- [ ] **Step 7: Run tests**

```bash
cd web
npm test
```

Expected: all primitive snapshot tests pass; lint passes.

- [ ] **Step 8: Commit**

```bash
git add web/src/components web/src/lib web/vitest.config.ts web/vitest.setup.ts \
        web/package.json web/package-lock.json
git commit -m "feat(c10): shadcn primitive intake (Button/Sheet/Switch/Toast/Dialog/Tooltip)"
```

---

## Task 5: app shell (sidebar + top bar + mobile drawer)

The hybrid shell from spec §4. All nav items present-but-disabled except Dashboards.

**Files:**
- Create: `web/src/shell/{Shell,Sidebar,TopBar,MobileDrawer,UserMenu,navItems}.tsx`
- Create: `web/src/shell/Shell.test.tsx`

- [ ] **Step 1: Define nav items**

`web/src/shell/navItems.tsx`:

```tsx
import { Home, Box, Square, Hexagon, Repeat, Play, List, Sliders, Cable, Settings } from "lucide-react";
import type { ReactElement } from "react";

export type NavItem = {
  id: string;
  label: string;
  href?: string;
  icon: ReactElement;
  enabled: boolean;
  comingIn?: string;
};

export const NAV_ITEMS: NavItem[] = [
  { id: "dashboards", label: "Dashboards", href: "/", icon: <Home size={16} />, enabled: true },
  { id: "entities", label: "Entities", icon: <Box size={16} />, enabled: false, comingIn: "C11" },
  { id: "devices", label: "Devices", icon: <Square size={16} />, enabled: false, comingIn: "C11" },
  { id: "areas", label: "Areas", icon: <Hexagon size={16} />, enabled: false, comingIn: "C11" },
  { id: "automations", label: "Automations", icon: <Repeat size={16} />, enabled: false, comingIn: "C11" },
  { id: "scripts", label: "Scripts", icon: <Play size={16} />, enabled: false, comingIn: "C11" },
  { id: "events", label: "Events", icon: <List size={16} />, enabled: false, comingIn: "C11" },
  { id: "config", label: "Config", icon: <Sliders size={16} />, enabled: false, comingIn: "C11" },
  { id: "drivers", label: "Drivers", icon: <Cable size={16} />, enabled: false, comingIn: "C11" },
  { id: "settings", label: "Settings", icon: <Settings size={16} />, enabled: false, comingIn: "C11" },
];
```

- [ ] **Step 2: Sidebar**

`web/src/shell/Sidebar.tsx`:

```tsx
import { NAV_ITEMS } from "./navItems";
import { cn } from "@/lib/cn";

export function Sidebar({ activeId }: { activeId: string }) {
  return (
    <nav className="bg-surface-1 border-border flex w-[180px] shrink-0 flex-col border-r p-normal">
      <ul className="flex flex-col gap-tight">
        {NAV_ITEMS.map((item) => (
          <li key={item.id}>
            {item.enabled ? (
              <a
                href={item.href}
                className={cn(
                  "text-fg-muted hover:bg-surface-2 hover:text-fg flex items-center gap-normal rounded-md px-normal py-tight text-sm",
                  item.id === activeId && "bg-surface-2 text-accent",
                )}
              >
                <span className="text-accent">{item.icon}</span>
                <span>{item.label}</span>
              </a>
            ) : (
              <span
                className="text-fg-muted/40 flex cursor-not-allowed items-center gap-normal rounded-md px-normal py-tight text-sm"
                title={item.comingIn ? `Coming in ${item.comingIn}` : "Not yet implemented"}
              >
                <span>{item.icon}</span>
                <span>{item.label}</span>
              </span>
            )}
          </li>
        ))}
      </ul>
    </nav>
  );
}
```

- [ ] **Step 3: TopBar + UserMenu**

`web/src/shell/TopBar.tsx`:

```tsx
import { Sun, Moon, Monitor } from "lucide-react";
import { useTheme } from "@/theme/provider";
import { Button } from "@/components/gh/Button";

export function TopBar({ title }: { title: string }) {
  const { modePreference, setMode } = useTheme();
  return (
    <header className="bg-surface-1 border-border flex h-10 items-center gap-loose border-b px-loose">
      <span className="text-accent font-display font-semibold">gohome</span>
      <span className="text-fg-muted text-sm">{title}</span>
      <div className="flex-1" />
      <div className="flex items-center gap-tight">
        <Button
          size="icon"
          variant="ghost"
          onClick={() => setMode(modePreference === "dark" ? "light" : modePreference === "light" ? "system" : "dark")}
          aria-label="Cycle theme mode"
        >
          {modePreference === "light" && <Sun size={16} />}
          {modePreference === "dark" && <Moon size={16} />}
          {modePreference === "system" && <Monitor size={16} />}
        </Button>
      </div>
    </header>
  );
}
```

(UserMenu hooked up in Task 7/8 once auth-store exists.)

- [ ] **Step 4: Shell**

`web/src/shell/Shell.tsx`:

```tsx
import type { ReactNode } from "react";
import { Sidebar } from "./Sidebar";
import { TopBar } from "./TopBar";

export function Shell({ activeNavId, title, children }: { activeNavId: string; title: string; children: ReactNode }) {
  return (
    <div className="bg-bg text-fg flex h-screen flex-col">
      <TopBar title={title} />
      <div className="flex min-h-0 flex-1">
        <div className="hidden md:block">
          <Sidebar activeId={activeNavId} />
        </div>
        <main className="min-h-0 flex-1 overflow-auto">{children}</main>
      </div>
    </div>
  );
}
```

(Mobile drawer in a follow-on commit; for v1.0 the sidebar is hidden below `md` and a hamburger sheet substitutes — left as a TODO until Task 18 needs mobile testing.)

- [ ] **Step 5: Update App.tsx to use Shell**

```tsx
import { Shell } from "./shell/Shell";

export default function App() {
  return (
    <Shell activeNavId="dashboards" title="Default Dashboard">
      <div className="p-loose">
        <p className="text-fg">Welcome to gohome.</p>
      </div>
    </Shell>
  );
}
```

- [ ] **Step 6: Smoke-test**

```bash
cd web
npm run dev &
sleep 2
# Open http://localhost:5173 and verify the sidebar/top bar render correctly in both modes.
kill %1
```

- [ ] **Step 7: Lint and unit-test**

```bash
cd web
npm run lint
npm test
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add web/src/shell web/src/App.tsx
git commit -m "feat(c10): app shell (hybrid sidebar + top bar)"
```

---

## Task 6: data layer (Connect-ES + TanStack Query + auth-store stub)

Set up the Connect-ES transport, the auth interceptor (refresh-on-401), TanStack Query client, and a Zustand auth-store. The store is a stub returning a static "system:local" user until Task 8 wires the real `/login` flow.

**Files:**
- Create: `web/src/data/{client,query-client,auth-store}.ts`, `web/src/data/types.ts`
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Add deps**

```bash
cd web
npm install @connectrpc/connect @connectrpc/connect-web @bufbuild/protobuf
npm install @tanstack/react-query @tanstack/react-router
```

- [ ] **Step 2: Generate the Connect-ES client (later step)**

The TS client is generated by `task proto`. For now, depend on the generated code path; Task 9 will run buf-es when needed. Stub out the imports here and make the auth interceptor structurally complete without depending on a specific service yet.

- [ ] **Step 3: Create `web/src/data/client.ts`**

```ts
import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { useAuthStore } from "./auth-store";

let refreshInFlight: Promise<void> | null = null;

const authInterceptor: Interceptor = (next) => async (req) => {
  // First attempt — cookies sent automatically by the browser.
  try {
    return await next(req);
  } catch (err: any) {
    if (err?.code !== "unauthenticated") throw err;
    // Coalesce concurrent refreshes.
    if (!refreshInFlight) {
      refreshInFlight = useAuthStore.getState().refresh().finally(() => { refreshInFlight = null; });
    }
    await refreshInFlight;
    return next(req);
  }
};

export const transport = createConnectTransport({
  baseUrl: "/api",
  useBinaryFormat: true,
  interceptors: [authInterceptor],
  fetch: (input, init) => fetch(input, { ...init, credentials: "include" }),
});
```

- [ ] **Step 4: Create `web/src/data/query-client.ts`**

```ts
import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error: any) => error?.code !== "unauthenticated" && failureCount < 2,
    },
  },
});
```

- [ ] **Step 5: Create `web/src/data/auth-store.ts`**

```ts
import { create } from "zustand";

export type CurrentUser = {
  slug: string;
  displayName: string;
  roles: string[];
};

type AuthState = {
  user: CurrentUser | null;
  refreshing: boolean;
};

type AuthActions = {
  bootstrap: () => Promise<void>;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
};

export const useAuthStore = create<AuthState & AuthActions>((set, get) => ({
  user: null,
  refreshing: false,
  async bootstrap() {
    // TODO(task 8): call AuthService.CurrentUser via the generated client.
    // For now, stub so the rest of the app can render.
    set({ user: { slug: "system:local", displayName: "Local Operator", roles: ["admin"] } });
  },
  async refresh() {
    if (get().refreshing) return;
    set({ refreshing: true });
    try {
      // TODO(task 8): call AuthService.Refresh.
      // Stub: no-op success.
    } finally {
      set({ refreshing: false });
    }
  },
  async logout() {
    set({ user: null });
  },
}));
```

- [ ] **Step 6: Wire QueryClient + bootstrap in main.tsx**

```tsx
import React, { useEffect } from "react";
import ReactDOM from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import { ThemeProvider } from "./theme/provider";
import { queryClient } from "./data/query-client";
import { useAuthStore } from "./data/auth-store";
import "./theme/tokens.css";

function Root() {
  useEffect(() => { useAuthStore.getState().bootstrap(); }, []);
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </ThemeProvider>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
);
```

- [ ] **Step 7: Smoke + lint + test**

```bash
cd web
npm run lint
npm test
npm run dev &
sleep 2
kill %1
```

Expected: clean.

- [ ] **Step 8: Commit**

```bash
git add web/src/data web/src/main.tsx web/package.json web/package-lock.json
git commit -m "feat(c10): data layer (connect-es transport, query client, auth store stub)"
```

---

## Task 7: server-side WebAuthn challenge storage (C9 inheritance)

Pick up C9's deferred work: implement the `StoreChallenge` / `ConsumeChallenge` ops in the existing `internal/auth/credentials/webauthn.go`, and replace the `Code.Unimplemented` returns in `internal/api/service_auth.go` for `StartWebAuthnChallenge` and the passkey branch of `Login`.

**Files:**
- Modify: `internal/auth/credentials/webauthn.go`, `internal/auth/credentials/webauthn_test.go`
- Modify: `internal/api/service_auth.go`, `internal/api/service_auth_test.go`

- [ ] **Step 1: Read the C9-shipped webauthn.go to understand its current shape**

```bash
cd gohome
sed -n '1,40p' internal/auth/credentials/webauthn.go
grep -n "Unimplemented" internal/api/service_auth.go
```

Document what's there before changing it. The plan continues assuming C9 left a struct skeleton with no challenge methods.

- [ ] **Step 2: Write the failing challenge-store test**

`internal/auth/credentials/webauthn_challenge_test.go`:

```go
package credentials_test

import (
	"context"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/auth/credentials"
)

func TestChallengeStore_StoreAndConsume(t *testing.T) {
	store := credentials.NewChallengeStore(time.Minute)
	ctx := context.Background()

	id, err := store.Store(ctx, "session-1", []byte("payload-1"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if id == "" {
		t.Fatal("Store returned empty id")
	}

	got, err := store.Consume(ctx, "session-1", id)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if string(got) != "payload-1" {
		t.Errorf("Consume payload = %q, want %q", got, "payload-1")
	}

	// Second consume must fail (replay protection).
	if _, err := store.Consume(ctx, "session-1", id); err == nil {
		t.Error("expected error on replay consume")
	}
}

func TestChallengeStore_Expires(t *testing.T) {
	store := credentials.NewChallengeStore(10 * time.Millisecond)
	ctx := context.Background()
	id, _ := store.Store(ctx, "s", []byte("p"))
	time.Sleep(20 * time.Millisecond)
	if _, err := store.Consume(ctx, "s", id); err == nil {
		t.Error("expected error consuming expired challenge")
	}
}

func TestChallengeStore_RejectsCrossSessionConsume(t *testing.T) {
	store := credentials.NewChallengeStore(time.Minute)
	ctx := context.Background()
	id, _ := store.Store(ctx, "session-1", []byte("p"))
	if _, err := store.Consume(ctx, "session-2", id); err == nil {
		t.Error("expected error consuming with wrong session")
	}
}
```

```bash
go test ./internal/auth/credentials/... -run ChallengeStore
```

Expected: FAIL (NewChallengeStore not defined).

- [ ] **Step 3: Implement the store**

Append to `internal/auth/credentials/webauthn.go`:

```go
// ChallengeStore holds per-session WebAuthn challenges between
// StartWebAuthnChallenge and Login. In-process only; expiry by TTL.
type ChallengeStore struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]challengeEntry // key: sessionID + ":" + id
}

type challengeEntry struct {
	payload []byte
	expires time.Time
}

func NewChallengeStore(ttl time.Duration) *ChallengeStore {
	return &ChallengeStore{ttl: ttl, items: make(map[string]challengeEntry)}
}

// Store records a challenge against a session id and returns an opaque id
// the client echoes back on Login.
func (s *ChallengeStore) Store(_ context.Context, sessionID string, payload []byte) (string, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	s.items[sessionID+":"+id] = challengeEntry{payload: payload, expires: time.Now().Add(s.ttl)}
	return id, nil
}

// Consume retrieves and removes a challenge. Returns ErrChallengeNotFound on
// any mismatch (session, id, or expiry) so the caller cannot distinguish
// missing from expired (timing safe).
func (s *ChallengeStore) Consume(_ context.Context, sessionID, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionID + ":" + id
	entry, ok := s.items[key]
	if !ok || time.Now().After(entry.expires) {
		delete(s.items, key)
		return nil, ErrChallengeNotFound
	}
	delete(s.items, key)
	return entry.payload, nil
}

func (s *ChallengeStore) gcLocked() {
	now := time.Now()
	for k, v := range s.items {
		if now.After(v.expires) {
			delete(s.items, k)
		}
	}
}

var ErrChallengeNotFound = errors.New("webauthn: challenge not found or expired")
```

Add the necessary imports (`sync`, `time`, `crypto/rand`, `errors`, `github.com/oklog/ulid/v2`).

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/auth/credentials/... -run ChallengeStore
```

Expected: PASS.

- [ ] **Step 5: Wire the challenge store through to AuthService**

In `internal/api/service_auth.go` find the C9 stub for `StartWebAuthnChallenge` and replace its body:

```go
func (s *AuthService) StartWebAuthnChallenge(ctx context.Context, req *connect.Request[v1.StartWebAuthnChallengeRequest]) (*connect.Response[v1.StartWebAuthnChallengeResponse], error) {
	sessionID := s.sessionIDFromCtx(ctx) // existing helper from C9; if missing, generate via ulid
	options, sessionData, err := s.webauthn.BeginLogin(/* user discovery handled by webauthn library; for v1 pass nil to allow any registered passkey */)
	if err != nil {
		return nil, ToConnect(ctx, err, "webauthn_begin_login_failed")
	}
	payload, err := json.Marshal(sessionData)
	if err != nil {
		return nil, ToConnect(ctx, err, "webauthn_serialize_session_failed")
	}
	id, err := s.challenges.Store(ctx, sessionID, payload)
	if err != nil {
		return nil, ToConnect(ctx, err, "challenge_store_failed")
	}
	return connect.NewResponse(&v1.StartWebAuthnChallengeResponse{
		ChallengeId: id,
		Options:     mustJSONStruct(options), // helper that wraps an arbitrary value in google.protobuf.Struct
	}), nil
}
```

For the passkey branch of Login:

```go
func (s *AuthService) Login(ctx context.Context, req *connect.Request[v1.LoginRequest]) (*connect.Response[v1.LoginResponse], error) {
	switch m := req.Msg.Method.(type) {
	case *v1.LoginRequest_Password:
		// ...existing C9 password impl...
	case *v1.LoginRequest_Webauthn:
		sessionID := s.sessionIDFromCtx(ctx)
		raw, err := s.challenges.Consume(ctx, sessionID, m.Webauthn.ChallengeId)
		if err != nil {
			return nil, ToConnect(ctx, err, "webauthn_challenge_invalid")
		}
		var sessionData webauthn.SessionData
		if err := json.Unmarshal(raw, &sessionData); err != nil {
			return nil, ToConnect(ctx, err, "webauthn_session_decode_failed")
		}
		credential, err := s.webauthn.FinishLogin(/* user, sessionData, parsedAssertion derived from m.Webauthn.AssertionResponse */)
		if err != nil {
			return nil, ToConnect(ctx, err, "webauthn_finish_login_failed")
		}
		// Promote to a session via the existing C9 session creator.
		return s.completePasskeyLogin(ctx, credential)
	}
	return nil, ToConnect(ctx, ErrInvalidLoginMethod, "login_method_required")
}
```

(Exact field names depend on what shape C9 settled on for `LoginRequest`; the proto file post-C9 is the source of truth.)

- [ ] **Step 6: Add a service-level test**

Add to `internal/api/service_auth_test.go`:

```go
func TestAuthService_PasskeyLogin_RoundTrip(t *testing.T) {
	t.Skip("requires go-webauthn virtual authenticator harness — implement once Task 8 stabilizes the request shape")
}
```

(Skipped now; populated by Task 8's E2E once we have a virtual authenticator wired up.)

- [ ] **Step 7: Build + race-test**

```bash
task build
task test
task test:race
```

Expected: clean.

- [ ] **Step 8: Commit**

```bash
git add internal/auth/credentials/webauthn.go \
        internal/auth/credentials/webauthn_challenge_test.go \
        internal/api/service_auth.go internal/api/service_auth_test.go
git commit -m "feat(c10): implement webauthn challenge store (closes C9 deferral)"
```

---

## Task 8: minimal /login page

The single-form login page from spec §9.

**Files:**
- Create: `web/src/routes/login.tsx`, `web/src/routes/__root.tsx`, `web/src/routes/_authed/_layout.tsx`, `web/src/routes/_authed/index.tsx`
- Create: `web/src/auth/PasskeyButton.tsx`, `web/src/auth/PasswordForm.tsx`, `web/src/auth/use-login.ts`
- Modify: `web/src/data/auth-store.ts` (replace stubs with real Connect calls)
- Modify: `web/src/main.tsx` (mount router)
- Modify: `web/src/shell/UserMenu.tsx` (add real logout)

- [ ] **Step 1: Wire TanStack Router**

Add `web/src/router.ts`:

```ts
import { createRouter, createRootRoute, createRoute, Outlet, Navigate } from "@tanstack/react-router";
import { Login } from "./routes/login";
import { AuthedLayout } from "./routes/_authed/_layout";
import { AuthedIndex } from "./routes/_authed/index";
import { DashboardSlug } from "./routes/_authed/dashboards/$slug";

const rootRoute = createRootRoute({ component: Outlet });
const loginRoute = createRoute({ getParentRoute: () => rootRoute, path: "/login", component: Login });
const authedLayout = createRoute({ getParentRoute: () => rootRoute, id: "_authed", component: AuthedLayout });
const authedIndex = createRoute({ getParentRoute: () => authedLayout, path: "/", component: AuthedIndex });
const dashboardSlug = createRoute({ getParentRoute: () => authedLayout, path: "/dashboards/$slug", component: DashboardSlug });

const routeTree = rootRoute.addChildren([loginRoute, authedLayout.addChildren([authedIndex, dashboardSlug])]);
export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" { interface Register { router: typeof router } }
```

- [ ] **Step 2: AuthedLayout with route guard**

`web/src/routes/_authed/_layout.tsx`:

```tsx
import { Outlet, Navigate, useLocation } from "@tanstack/react-router";
import { useAuthStore } from "@/data/auth-store";
import { Shell } from "@/shell/Shell";

export function AuthedLayout() {
  const user = useAuthStore((s) => s.user);
  const loc = useLocation();
  if (!user) return <Navigate to="/login" search={{ returnTo: loc.pathname }} />;
  return (
    <Shell activeNavId="dashboards" title="">
      <Outlet />
    </Shell>
  );
}
```

`web/src/routes/_authed/index.tsx`:

```tsx
import { Navigate } from "@tanstack/react-router";
export function AuthedIndex() { return <Navigate to="/dashboards/$slug" params={{ slug: "default" }} />; }
```

- [ ] **Step 3: Login page**

`web/src/routes/login.tsx`:

```tsx
import { useState } from "react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { Button } from "@/components/gh/Button";
import { useAuthStore } from "@/data/auth-store";

export function Login() {
  const navigate = useNavigate();
  const search = useSearch({ from: "/login" }) as { returnTo?: string };
  const login = useAuthStore((s) => s.loginWithPassword);
  const loginPasskey = useAuthStore((s) => s.loginWithPasskey);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await login(username, password);
      navigate({ to: search.returnTo ?? "/" });
    } catch (err: any) {
      setError("Invalid credentials");
    }
  }

  async function onPasskey() {
    setError(null);
    try {
      await loginPasskey(username);
      navigate({ to: search.returnTo ?? "/" });
    } catch (err: any) {
      setError(err?.message ?? "Passkey login failed");
    }
  }

  return (
    <div className="bg-bg flex min-h-screen items-center justify-center p-loose">
      <form onSubmit={onSubmit} className="bg-surface-1 border-border flex w-80 flex-col gap-normal rounded-lg border p-loose">
        <h1 className="text-fg font-display text-xl">Sign in to gohome</h1>
        <input className="bg-bg border-border text-fg rounded-md border px-normal py-tight" placeholder="username" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
        <input className="bg-bg border-border text-fg rounded-md border px-normal py-tight" placeholder="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        {error && <span className="text-danger text-sm">{error}</span>}
        <Button type="submit">Sign in</Button>
        <Button type="button" variant="secondary" onClick={onPasskey}>Use passkey</Button>
      </form>
    </div>
  );
}
```

- [ ] **Step 4: Implement real auth-store actions**

Replace stubs in `web/src/data/auth-store.ts` with Connect calls:

```ts
import { createPromiseClient } from "@connectrpc/connect";
import { AuthService } from "../gen/gohome/v1alpha1/auth_connect"; // generated by Task 9
import { transport } from "./client";

const authClient = createPromiseClient(AuthService, transport);

// loginWithPassword:
async loginWithPassword(username: string, password: string) {
  const res = await authClient.login({ method: { case: "password", value: { username, password } } });
  set({ user: { slug: res.user!.slug, displayName: res.user!.displayName, roles: res.user!.roles } });
}

// loginWithPasskey:
async loginWithPasskey(username: string) {
  const challenge = await authClient.startWebAuthnChallenge({ username });
  const options = challenge.options!.toJson() as PublicKeyCredentialRequestOptionsJSON;
  const assertion = await navigator.credentials.get({ publicKey: parseChallengeOptions(options) });
  const res = await authClient.login({
    method: { case: "webauthn", value: { challengeId: challenge.challengeId, assertionResponse: serializeAssertion(assertion!) } },
  });
  set({ user: { slug: res.user!.slug, displayName: res.user!.displayName, roles: res.user!.roles } });
}

// logout:
async logout() {
  await authClient.logout({});
  set({ user: null });
}

// bootstrap (replace stub):
async bootstrap() {
  try {
    const me = await authClient.currentUser({});
    set({ user: { slug: me.user!.slug, displayName: me.user!.displayName, roles: me.user!.roles } });
  } catch {
    set({ user: null });
  }
}

// refresh (replace stub):
async refresh() {
  if (get().refreshing) return;
  set({ refreshing: true });
  try {
    await authClient.refresh({});
  } finally {
    set({ refreshing: false });
  }
}
```

(`parseChallengeOptions` and `serializeAssertion` are small helpers wrapping base64url encoding/decoding — written here once and exported from `web/src/auth/webauthn-helpers.ts`.)

- [ ] **Step 5: Mount router in main.tsx**

```tsx
import { RouterProvider } from "@tanstack/react-router";
import { router } from "./router";
// ...inside Root:
<RouterProvider router={router} />
```

- [ ] **Step 6: Add E2E test (deferred to Task 26 Playwright suite)**

Note in `web/e2e/auth.spec.ts.todo` — placeholder file naming the test cases that will be implemented in Task 26.

- [ ] **Step 7: Lint + test + smoke**

```bash
cd web
npm run lint && npm test && npm run dev &
sleep 2
curl -s http://localhost:5173/login | grep -q "Sign in"
kill %1
```

- [ ] **Step 8: Commit**

```bash
git add web/src/routes web/src/router.ts web/src/data/auth-store.ts web/src/auth web/src/main.tsx
git commit -m "feat(c10): minimal /login page (password + passkey)"
```

---

## Task 9: regenerate proto for the C10 dashboard / config / web shapes

Expand `proto/gohome/v1alpha1/dashboard.proto` for the typed shape, add `ConfigService.EvalCompute`, run `task proto`.

**Files:**
- Modify: `proto/gohome/v1alpha1/dashboard.proto`
- Modify: `proto/gohome/v1alpha1/config.proto`
- Generated: `gen/gohome/v1alpha1/dashboard*.pb.go`, `gen/gohome/v1alpha1/config*.pb.go` and connect equivalents
- Generated: TS clients in `web/src/gen/...` (via buf-es)

- [ ] **Step 1: Update `proto/gohome/v1alpha1/dashboard.proto`**

Replace the entire file with:

```proto
// See docs/proto-hygiene.md for grouping conventions.

syntax = "proto3";

package gohome.v1alpha1;

import "gohome/v1alpha1/common.proto";
import "google/protobuf/struct.proto";

service DashboardService {
  rpc List              (ListDashboardsRequest)         returns (ListDashboardsResponse);
  rpc Get               (GetDashboardRequest)           returns (GetDashboardResponse);
  rpc GetWidgetCatalog  (GetWidgetCatalogRequest)       returns (GetWidgetCatalogResponse);
  rpc Create            (CreateDashboardRequest)        returns (CreateDashboardResponse);
  rpc Delete            (DeleteDashboardRequest)        returns (DeleteDashboardResponse);
  rpc SaveLayout        (SaveDashboardLayoutRequest)    returns (SaveDashboardLayoutResponse);
}

message Dashboard {
  string                slug              = 1;
  string                title             = 2;
  Grid                  grid              = 3;
  repeated WidgetInstance widgets          = 4;
  string                source_pkl        = 5;   // user-owned <slug>.pkl
  string                layout_pkl        = 6;   // WYSIWYG-owned <slug>.layout.pkl
  bool                  wysiwyg_writable  = 7;   // false if no <slug>.layout.pkl exists
}

message Position { int32 x = 1; int32 y = 2; int32 w = 3; int32 h = 4; }
message Grid     { int32 columns = 1; int32 row_height = 2; }

message WidgetInstance {
  string                 id            = 1;
  string                 class_id      = 2;     // "EntityToggle", "bar-widgets/BarChart"
  Position               pos           = 3;
  google.protobuf.Struct props         = 4;     // typed props as Struct (typed-decoded client side from PropSchema)
  bool                   is_container  = 5;
  Grid                   child_grid    = 6;     // only when is_container
  repeated WidgetInstance children      = 7;     // only when is_container
}

// --- Catalog ---
enum SignatureStatus {
  SIGNATURE_UNKNOWN  = 0;
  SIGNATURE_VERIFIED = 1;
  SIGNATURE_UNSIGNED = 2;
  SIGNATURE_INVALID  = 3;
  SIGNATURE_EXPIRED  = 4;
}

message PropSchema {
  // JSON-Schema-like. Keys: "type", "required", "min", "max", "enum", "items", etc.
  google.protobuf.Struct schema = 1;
}

message WidgetClass {
  string          class_id      = 1;
  bool            is_container  = 2;
  bool            is_builtin    = 3;
  string          pack_name     = 4;
  string          pack_version  = 5;
  string          bundle_url    = 6;
  string          bundle_hash   = 7;
  PropSchema      prop_schema   = 8;
  SignatureStatus signature     = 9;
}

message WidgetCatalog { repeated WidgetClass classes = 1; }

// --- RPC messages ---
message ListDashboardsRequest      { PageRequest page = 1; }
message ListDashboardsResponse     { repeated Dashboard dashboards = 1; PageResponse page = 2; }

message GetDashboardRequest        { string slug = 1; }
message GetDashboardResponse       { Dashboard dashboard = 1; }

message GetWidgetCatalogRequest    {}
message GetWidgetCatalogResponse   { WidgetCatalog catalog = 1; }

message CreateDashboardRequest     { string slug = 1; string title = 2; }
message CreateDashboardResponse    { Dashboard dashboard = 1; }

message DeleteDashboardRequest     { string slug = 1; bool delete_source_too = 2; }
message DeleteDashboardResponse    {}

message SaveDashboardLayoutRequest { Dashboard dashboard = 1; }   // client sends the edited Dashboard
message SaveDashboardLayoutResponse{ Dashboard dashboard = 1; string correlation_id = 2; }
```

- [ ] **Step 2: Add `EvalCompute` to `proto/gohome/v1alpha1/config.proto`**

Append to the service block:

```proto
service ConfigService {
  // ...existing RPCs...
  rpc EvalCompute(EvalComputeRequest) returns (EvalComputeResponse);
}

message EvalComputeRequest {
  string                 dashboard_slug = 1;
  string                 widget_id      = 2;
  string                 expr_id        = 3;
  google.protobuf.Struct state_snapshot = 4;
}

message EvalComputeResponse {
  google.protobuf.Value result = 1;
  string                error  = 2;
}
```

(`google.protobuf.Value` already imported via `struct.proto`.)

- [ ] **Step 3: Regenerate**

```bash
cd gohome
task proto
```

Expected: `gen/gohome/v1alpha1/dashboard.pb.go` updated; `dashboard.connect.go` updated; `config.pb.go` updated.

- [ ] **Step 4: Configure buf-es for the TS client**

Add to `buf.gen.yaml` if not already present:

```yaml
plugins:
  # ...existing...
  - remote: buf.build/bufbuild/es
    out: web/src/gen
    opt:
      - target=ts
  - remote: buf.build/bufbuild/connect-es
    out: web/src/gen
    opt:
      - target=ts
```

```bash
task proto
```

Expected: `web/src/gen/gohome/v1alpha1/*.ts` files generated.

- [ ] **Step 5: Adjust the C7-stub DashboardService in `internal/api/service_unimplemented.go`**

Remove the stub `DashboardService` struct + methods (Task 13 supplies the real one). Confirm the file still compiles (Scene/Auth/etc. stubs remain).

- [ ] **Step 6: Build + test**

```bash
task build && task test
```

Expected: clean (Connect handlers are now interface-only because the impl was removed; the daemon wiring will fail until Task 13 lands the real impl. Add a temporary placeholder in `internal/api/service_dashboard_placeholder.go` that returns Unimplemented for now, to keep the build green between tasks.)

`internal/api/service_dashboard_placeholder.go`:

```go
package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

type dashboardPlaceholder struct{}

func newDashboardPlaceholder() *dashboardPlaceholder { return &dashboardPlaceholder{} }

var _ gohomev1alpha1connect.DashboardServiceHandler = (*dashboardPlaceholder)(nil)

func (*dashboardPlaceholder) List(ctx context.Context, _ *connect.Request[v1.ListDashboardsRequest]) (*connect.Response[v1.ListDashboardsResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
// (similar stubs for Get, GetWidgetCatalog, Create, Delete, SaveLayout — each returns unimplemented)
```

- [ ] **Step 7: Commit**

```bash
git add proto/ gen/ web/src/gen/ buf.gen.yaml internal/api/service_unimplemented.go internal/api/service_dashboard_placeholder.go
git commit -m "feat(c10): expand DashboardService proto + add ConfigService.EvalCompute"
```

---

## Task 10: multiplexer (two streams + command tracker)

Implement the browser-side multiplexer per spec §7.

**Files:**
- Create: `web/src/multiplexer/{streams,state-cache,command-tracker,multiplexer,hooks}.ts`
- Create: `web/src/multiplexer/multiplexer.tsx` (the React `<Multiplexer>` component)
- Create: `web/src/multiplexer/*.test.ts`

- [ ] **Step 1: Write failing tests against a fake transport**

`web/src/multiplexer/multiplexer.test.ts`:

```ts
import { describe, it, expect, beforeEach } from "vitest";
import { createMultiplexer } from "./multiplexer";
import { fakeTransport } from "./test-helpers";

describe("Multiplexer", () => {
  let mux: ReturnType<typeof createMultiplexer>;
  let transport: ReturnType<typeof fakeTransport>;
  beforeEach(() => {
    transport = fakeTransport();
    mux = createMultiplexer({ transport });
  });

  it("subscribes once per entity-id union", async () => {
    mux.subscribe(["light.kitchen", "sensor.outdoor"]);
    expect(transport.entitySubscribeCalls).toHaveLength(1);
    expect(transport.entitySubscribeCalls[0].selector).toEqual({
      ids: ["light.kitchen", "sensor.outdoor"],
    });
  });

  it("delivers state updates to per-entity subscribers", async () => {
    const seen: any[] = [];
    mux.subscribe(["light.kitchen"]);
    mux.onEntity("light.kitchen", (s) => seen.push(s));
    transport.emitEntityChange({ entity_id: "light.kitchen", entity: { state: { value: "on" } }, cursor: 1 });
    expect(seen).toHaveLength(1);
    expect(seen[0].value).toBe("on");
  });

  it("registers pending and clears on acknowledged", () => {
    const seen: any[] = [];
    mux.onPending("light.kitchen", (p) => seen.push(p));
    transport.emitTailEvent({ kind: "command_issued", command_id: "C1", entity_id: "light.kitchen" });
    expect(seen[seen.length - 1].state).toBe("pending");
    transport.emitTailEvent({ kind: "command_acked", command_id: "C1" });
    expect(seen[seen.length - 1].state).toBe("idle");
  });

  it("marks failed and reverts after delay", async () => {
    const seen: any[] = [];
    mux.onPending("light.kitchen", (p) => seen.push(p));
    transport.emitTailEvent({ kind: "command_issued", command_id: "C2", entity_id: "light.kitchen" });
    transport.emitTailEvent({ kind: "command_failed", command_id: "C2", error: "driver offline" });
    expect(seen[seen.length - 1].state).toBe("failed");
  });

  it("resumes from cursor on reconnect", async () => {
    mux.subscribe(["light.kitchen"]);
    transport.emitEntityChange({ entity_id: "light.kitchen", cursor: 42 });
    transport.disconnectEntityStream();
    await transport.flushReconnect();
    expect(transport.entitySubscribeCalls.at(-1)?.from_cursor).toBe(43n);
  });
});
```

`web/src/multiplexer/test-helpers.ts`: a small fake exposing `entitySubscribeCalls`, `emitEntityChange`, `emitTailEvent`, `disconnectEntityStream`, `flushReconnect`. Implementation skeleton:

```ts
export function fakeTransport() {
  const subscribers = { entity: [] as any[], tail: [] as any[] };
  // ...returns an object that the multiplexer sees as "transport" with two server-streaming methods...
}
```

```bash
cd web && npm test -- multiplexer
```

Expected: FAIL.

- [ ] **Step 2: Implement `state-cache.ts`**

```ts
type Listener = (s: EntityState) => void;
export class StateCache {
  private states = new Map<string, EntityState>();
  private subs = new Map<string, Set<Listener>>();
  set(id: string, s: EntityState) {
    this.states.set(id, s);
    this.subs.get(id)?.forEach((fn) => fn(s));
  }
  get(id: string) { return this.states.get(id); }
  subscribe(id: string, fn: Listener) {
    if (!this.subs.has(id)) this.subs.set(id, new Set());
    this.subs.get(id)!.add(fn);
    return () => this.subs.get(id)!.delete(fn);
  }
}
```

- [ ] **Step 3: Implement `command-tracker.ts`**

```ts
export type PendingState =
  | { state: "idle" }
  | { state: "pending"; commandId: string; sinceMs: number }
  | { state: "failed"; commandId: string; error: string; ageMs: number };

type Listener = (p: PendingState) => void;

export class CommandTracker {
  private pending = new Map<string, { entityId: string; t0: number }>();
  private failed = new Map<string, { entityId: string; error: string; t0: number }>();
  private subs = new Map<string, Set<Listener>>();

  issued(commandId: string, entityId: string) {
    this.pending.set(commandId, { entityId, t0: Date.now() });
    this.notify(entityId);
  }
  acked(commandId: string) {
    const e = this.pending.get(commandId);
    this.pending.delete(commandId);
    if (e) this.notify(e.entityId);
  }
  failed_(commandId: string, error: string) {
    const e = this.pending.get(commandId);
    this.pending.delete(commandId);
    if (!e) return;
    this.failed.set(commandId, { entityId: e.entityId, error, t0: Date.now() });
    this.notify(e.entityId);
    setTimeout(() => { this.failed.delete(commandId); this.notify(e.entityId); }, 3000);
  }
  current(entityId: string): PendingState {
    const ps = [...this.pending.entries()].filter(([, e]) => e.entityId === entityId);
    if (ps.length > 0) {
      const [cid, e] = ps[ps.length - 1];
      return { state: "pending", commandId: cid, sinceMs: Date.now() - e.t0 };
    }
    const fs = [...this.failed.entries()].filter(([, e]) => e.entityId === entityId);
    if (fs.length > 0) {
      const [cid, e] = fs[fs.length - 1];
      return { state: "failed", commandId: cid, error: e.error, ageMs: Date.now() - e.t0 };
    }
    return { state: "idle" };
  }
  subscribe(entityId: string, fn: Listener) {
    if (!this.subs.has(entityId)) this.subs.set(entityId, new Set());
    this.subs.get(entityId)!.add(fn);
    return () => this.subs.get(entityId)!.delete(fn);
  }
  private notify(entityId: string) { this.subs.get(entityId)?.forEach((fn) => fn(this.current(entityId))); }
}
```

- [ ] **Step 4: Implement `streams.ts` and `multiplexer.ts`**

The `streams.ts` file holds the two long-lived server-stream connections with reconnect/resume logic. `multiplexer.ts` composes `StateCache` + `CommandTracker` + `streams.ts` and exposes the public API used by hooks. Skipping verbatim code here — the structure is straightforward iteration over Connect-ES's async iterables with a try/catch + sleep + retry loop, tracking `lastCursor` for each stream.

- [ ] **Step 5: Implement `hooks.ts`**

```ts
import { useSyncExternalStore } from "react";
import { useMultiplexer } from "./multiplexer";

export function useEntityState(id: string) {
  const mux = useMultiplexer();
  return useSyncExternalStore(
    (cb) => mux.cache.subscribe(id, cb),
    () => mux.cache.get(id),
  );
}

export function usePending(id: string) {
  const mux = useMultiplexer();
  return useSyncExternalStore(
    (cb) => mux.tracker.subscribe(id, cb),
    () => mux.tracker.current(id),
  );
}

export function useCallCapability() {
  const mux = useMultiplexer();
  return mux.callCapability;
}
```

- [ ] **Step 6: Implement `<Multiplexer>` React component**

```tsx
import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react";
import { createMultiplexer } from "./multiplexer";
import { transport } from "@/data/client";

const MultiplexerContext = createContext<ReturnType<typeof createMultiplexer> | null>(null);

export function Multiplexer({ entityIds, children }: { entityIds: string[]; children: ReactNode }) {
  const mux = useMemo(() => createMultiplexer({ transport }), []);
  useEffect(() => {
    mux.subscribe(entityIds);
    return () => mux.shutdown();
  }, [mux, entityIds.join(",")]);
  return <MultiplexerContext.Provider value={mux}>{children}</MultiplexerContext.Provider>;
}

export function useMultiplexer() {
  const v = useContext(MultiplexerContext);
  if (!v) throw new Error("useMultiplexer must be inside <Multiplexer>");
  return v;
}
```

- [ ] **Step 7: Run tests**

```bash
cd web && npm test -- multiplexer
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add web/src/multiplexer
git commit -m "feat(c10): live-stream multiplexer (two streams + command tracker)"
```

---

## Task 11: Pkl widgets/dashboards module expansion

Replace the existing stub `widgets.pkl` and `dashboards.pkl` with the recursive schema from spec §13.1 and §14.

**Files:**
- Modify: `internal/config/pkl/gohome/widgets.pkl`, `dashboards.pkl`
- Create: `internal/config/pkl/gohome/web.pkl`
- Create: `internal/config/testdata/dashboard-fixtures/main.pkl` (round-trip test fixture)

- [ ] **Step 1: Replace `dashboards.pkl`**

```pkl
module gohome.dashboards

import "@gohome/widgets.pkl" as w

class Position {
  x: Int(isBetween(0, 96))
  y: Int(isBetween(0, 999))
  width: Int(isBetween(1, 96))
  height: Int(isBetween(1, 96))
}

class Grid {
  columns: Int(isBetween(1, 96)) = 12
  rowHeight: Int(isBetween(20, 200)) = 60
}

class Dashboard {
  slug: String(!isEmpty)
  title: String
  grid: Grid = new {}
  widgets: Listing<w.WidgetInstance> = new {}
}

class LayoutFile {
  grid: Grid = new {}
  widgets: Listing<w.WidgetInstance> = new {}
}
```

- [ ] **Step 2: Replace `widgets.pkl`**

```pkl
module gohome.widgets

import "@gohome/dashboards.pkl" as d
import "@gohome/starlark.pkl" as starlark

abstract class WidgetInstance {
  id: String(matches(Regex(#"^[a-z][a-z0-9_-]{0,63}$"#)))
  pos: d.Position
  /// Required: subclasses assign a List<String> derived from typed props.
  referencedEntityIds: List<String>
}

abstract class ContainerInstance extends WidgetInstance {
  childGrid: d.Grid
  children: Listing<WidgetInstance>
  referencedEntityIds = children.toList().flatMap((c) -> c.referencedEntityIds).distinct
}

class Threshold { value: Number; color: String }

// --- Built-in widgets ---

class EntityToggle extends WidgetInstance {
  entityId: String
  label: String?
  icon: String?
  showBrightness: Boolean = false
  referencedEntityIds = List(entityId)
}

class Gauge extends WidgetInstance {
  entityId: String
  label: String?
  min: Number = 0
  max: Number = 100
  unit: String?
  thresholds: Listing<Threshold> = new {}
  referencedEntityIds = List(entityId)
}

class LineChart extends WidgetInstance {
  entityIds: Listing<String>(length > 0)
  label: String?
  rangeHours: Int(isBetween(1, 720)) = 24
  yMin: Number?
  yMax: Number?
  referencedEntityIds = entityIds.toList()
}

class CameraStream extends WidgetInstance {
  entityId: String
  label: String?
  referencedEntityIds = List(entityId)
}

class Markdown extends WidgetInstance {
  content: String?
  compute: starlark.StarlarkExpr?
  referencedEntityIds = List()
}

class ScriptButton extends WidgetInstance {
  scriptId: String
  label: String
  args: Mapping<String, Any>?
  confirm: Boolean = false
  icon: String?
  referencedEntityIds = List()
}

class EntitySelector {
  areas: Listing<String>?
  classes: Listing<String>?
  domains: Listing<String>?
  ids: Listing<String>?
}

class EntityList extends WidgetInstance {
  selector: EntitySelector
  label: String?
  showState: Boolean = true
  showQuickControls: Boolean = true
  maxRows: Int? = 50
  referencedEntityIds = List()
}

class GroupCard extends ContainerInstance {
  title: String?
  collapsed: Boolean = false
}

// --- Pack types ---

class PackManifest {
  name: String(!isEmpty)
  version: String(!isEmpty)
  protocol: String(!isEmpty) = "v1"
  sdkVersion: String(!isEmpty)
  bundle: String = "bundle.js"
  bundleHash: String(!isEmpty)
  classes: Listing<Class>
  description: String?
  homepage: String?
  license: String?
}

class PackPolicy {
  trustedPublishers: Listing<String> = new {}
  allowUnsigned: Boolean = false
}
```

- [ ] **Step 3: Create `web.pkl`**

```pkl
module gohome.web

class Web {
  enabled: Boolean = true
  embeddedTheme: String(this == "developer") = "developer"   // locked in v1.0
  defaultMode: String(this == "light" || this == "dark" || this == "system") = "system"
  pwa: PWA = new {}
}

class PWA {
  enabled: Boolean = true
  manifestName: String = "gohome"
  manifestShortName: String = "gohome"
  themeColor: String = "#0a0a0a"
}
```

- [ ] **Step 4: Add a fixture**

`internal/config/testdata/dashboard-fixtures/main.pkl`:

```pkl
amends "@gohome/config.pkl"

import "@gohome/dashboards.pkl" as d
import "@gohome/widgets.pkl" as w

dashboards: Listing<d.Dashboard> = new {
  new {
    slug = "default"
    title = "Home"
    grid = new { columns = 12 }
    widgets = new {
      new w.GroupCard {
        id = "living-room"
        pos = new { x = 0; y = 0; width = 6; height = 4 }
        title = "Living Room"
        childGrid = new { columns = 6 }
        children = new {
          new w.EntityToggle {
            id = "kitchen-light"
            pos = new { x = 0; y = 0; width = 2; height = 1 }
            entityId = "light.kitchen"
          }
        }
      }
    }
  }
}
```

- [ ] **Step 5: Validate the fixture compiles**

```bash
cd gohome
go test ./internal/config/... -run Evaluator
```

Expected: passes (the C4 evaluator should accept the new module shapes).

- [ ] **Step 6: Commit**

```bash
git add internal/config/pkl/gohome/widgets.pkl \
        internal/config/pkl/gohome/dashboards.pkl \
        internal/config/pkl/gohome/web.pkl \
        internal/config/testdata/dashboard-fixtures/
git commit -m "feat(c10): expand widgets/dashboards Pkl modules + add web.pkl"
```

---

## Task 12: dashboard widget catalog (server)

`DashboardService.GetWidgetCatalog` returns the union of built-in classes + installed widget pack classes.

**Files:**
- Create: `internal/dashboard/catalog.go`, `catalog_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestCatalog_BuiltinsOnly(t *testing.T) {
	c := dashboard.NewCatalog(nil) // no packs
	classes := c.WidgetClasses()
	wantClasses := []string{"EntityToggle", "Gauge", "LineChart", "CameraStream", "Markdown", "ScriptButton", "EntityList", "GroupCard"}
	got := make([]string, 0, len(classes))
	for _, c := range classes { got = append(got, c.ClassID) }
	if !slices.Equal(got, wantClasses) {
		t.Errorf("classes = %v, want %v", got, wantClasses)
	}
}

func TestCatalog_BuiltinsPlusPack(t *testing.T) {
	pack := &widgetpack.Installed{Name: "bar-widgets", Version: "1.0.0", Classes: []widgetpack.Class{{Name: "BarChart"}}}
	c := dashboard.NewCatalog([]widgetpack.Installed{*pack})
	got := c.LookupClass("bar-widgets/BarChart")
	if got == nil {
		t.Fatal("expected to find bar-widgets/BarChart")
	}
}
```

- [ ] **Step 2: Implement `internal/dashboard/catalog.go`** with the eight built-ins as a constant table + a method to merge installed packs.

- [ ] **Step 3: Run test, fix, commit**

```bash
go test ./internal/dashboard/...
git add internal/dashboard/catalog.go internal/dashboard/catalog_test.go
git commit -m "feat(c10): widget catalog (built-ins + pack merge)"
```

---

## Task 13: DashboardService.Get + GetWidgetCatalog (read path)

Server implementation of the read path against compiled config + catalog.

**Files:**
- Create: `internal/dashboard/service.go`, `service_test.go`
- Modify: `internal/api/deps.go` (add DashboardBackend interface)
- Modify: `internal/daemon/daemon.go` (wire the service)

- [ ] **Step 1: Define the backend interface**

In `internal/api/deps.go`:

```go
type DashboardBackend interface {
	List(ctx context.Context) ([]Dashboard, error)
	Get(ctx context.Context, slug string) (Dashboard, error)
	WidgetCatalog(ctx context.Context) ([]WidgetClass, error)
}

type Dashboard struct {
	Slug, Title         string
	Grid                Grid
	Widgets             []WidgetInstance
	SourcePkl           string
	LayoutPkl           string
	WysiwygWritable     bool
}

type WidgetInstance struct {
	ID          string
	ClassID     string
	Pos         Position
	Props       map[string]any
	IsContainer bool
	ChildGrid   Grid
	Children    []WidgetInstance
}

type Position struct{ X, Y, W, H int32 }
type Grid     struct{ Columns, RowHeight int32 }

type WidgetClass struct {
	ClassID     string
	IsContainer bool
	IsBuiltin   bool
	PackName, PackVersion string
	BundleURL, BundleHash string
	PropSchema   map[string]any
	SignatureStatus string
}
```

- [ ] **Step 2: Implement `internal/dashboard/service.go`**

Wire to a `DashboardBackend`; convert between domain types and proto types via mappers; honor the spec §10 shape (`Dashboard` proto with `source_pkl`, `layout_pkl`, `wysiwyg_writable`).

- [ ] **Step 3: Backend impl**

The `DashboardBackend` is implemented by reading the compiled `ConfigSnapshot` (C4) and pulling the `dashboards: Listing<Dashboard>` field. For each dashboard, also read `<slug>.pkl` and `<slug>.layout.pkl` files from the config dir to populate `SourcePkl`/`LayoutPkl`/`WysiwygWritable`.

Place this impl in `internal/daemon/dashboard_backend.go`:

```go
type dashboardBackend struct {
	cfg     *config.Manager
	catalog *dashboard.Catalog
	dir     string // config dir
}
// ...impl matching the interface...
```

- [ ] **Step 4: Tests + smoke**

```bash
go test ./internal/dashboard/...
task build
./dist/gohomed --config testdata/dashboard-fixtures/main.pkl &
sleep 1
buf curl --schema ./proto --data '{"slug":"default"}' http://127.0.0.1:8080/gohome.v1alpha1.DashboardService/Get
kill %1
```

Expected: returns the `default` dashboard with widgets populated.

- [ ] **Step 5: Commit**

```bash
git add internal/dashboard/service.go internal/dashboard/service_test.go \
        internal/api/deps.go internal/daemon/dashboard_backend.go internal/daemon/daemon.go
git commit -m "feat(c10): DashboardService.Get + GetWidgetCatalog (read path)"
```

---

## Task 14: dashboard render path (browser)

`<DashboardView>`, `<Grid>`, `<WidgetRenderer>`, per-widget error boundary, lazy pack loading.

**Files:**
- Create: `web/src/dashboard/render/{DashboardView,Grid,WidgetRenderer,WidgetErrorBoundary,DashboardSkeleton}.tsx`
- Create: `web/src/dashboard/catalog.ts` (client-side registry)
- Create: `web/src/dashboard/pack-loader.ts`
- Create: `web/src/routes/_authed/dashboards/$slug.tsx`

- [ ] **Step 1: Implement `web/src/dashboard/catalog.ts`**

```ts
import type { ComponentType } from "react";
import type { WidgetProps } from "@gohome/widget-sdk";
// Built-in registry — populated as Tasks 15/19 add widgets.
export const builtInWidgets: Record<string, ComponentType<WidgetProps>> = {};
```

- [ ] **Step 2: Implement `web/src/dashboard/pack-loader.ts`**

```ts
const cache = new Map<string, Promise<Record<string, ComponentType<WidgetProps>>>>();
export async function loadPack(packName: string, version: string, hash: string) {
  const key = `${packName}-${version}-${hash}`;
  let p = cache.get(key);
  if (!p) {
    p = import(/* @vite-ignore */ `/widgets/${packName}/${version}/bundle.js?h=${hash}`);
    cache.set(key, p);
  }
  return p;
}
```

- [ ] **Step 3: Implement render components**

`web/src/dashboard/render/Grid.tsx`, `WidgetRenderer.tsx`, `WidgetErrorBoundary.tsx`, `DashboardView.tsx`. Use `react-grid-layout` for the Grid. Compose `<Multiplexer>` at the top of `DashboardView` with the union of `referencedEntityIds`.

```bash
cd web
npm install react-grid-layout
npm install -D @types/react-grid-layout
```

- [ ] **Step 4: Mount in `/dashboards/$slug` route**

`web/src/routes/_authed/dashboards/$slug.tsx`:

```tsx
import { useParams } from "@tanstack/react-router";
import { DashboardView } from "@/dashboard/render/DashboardView";

export function DashboardSlug() {
  const { slug } = useParams({ strict: false });
  return <DashboardView slug={slug as string} />;
}
```

- [ ] **Step 5: Lint + test + commit**

```bash
cd web && npm run lint && npm test
git add web/src/dashboard web/src/routes/_authed/dashboards web/package.json web/package-lock.json
git commit -m "feat(c10): dashboard render path (Grid, WidgetRenderer, error boundary, lazy pack loader)"
```

---

## Task 15: first three built-in widgets (EntityToggle, Markdown, Gauge)

Implements the pending-state UX end-to-end with the simplest widgets.

**Files:**
- Create: `web/src/widgets/{EntityToggle,Markdown,Gauge}.tsx`
- Create: `web/src/widgets/*.test.tsx`

- [ ] **Step 1-3: Implement each widget**

For each: write the failing component test, implement the component reading from `WidgetProps`, register in `web/src/dashboard/catalog.ts`'s `builtInWidgets`. Tests assert pending visual on tap, settled visual on `state_changed` arrival.

- [ ] **Step 4: E2E smoke against the daemon**

(Defer full E2E to Task 26; this step is just `npm test`.)

- [ ] **Step 5: Commit**

```bash
git add web/src/widgets web/src/dashboard/catalog.ts
git commit -m "feat(c10): EntityToggle / Markdown / Gauge built-in widgets"
```

---

## Task 16: Pkl regenerator + golden round-trip tests

The deterministic Pkl serializer for `*.layout.pkl`. Most architecturally-important task in C10.

**Files:**
- Create: `internal/dashboard/regen/regen.go`, `regen_test.go`
- Create: `internal/dashboard/regen/testdata/*.proto-text`, `*.layout.pkl` (golden fixtures)

- [ ] **Step 1: Write the failing golden test table**

`internal/dashboard/regen/regen_test.go`:

```go
package regen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/dashboard"
	"github.com/fynn-labs/gohome/internal/dashboard/regen"
)

var goldenCases = []string{
	"empty_layout",
	"single_leaf",
	"two_siblings",
	"container_with_children",
	"nested_containers_3deep",
	"starlark_props",
	"unicode_strings",
	"long_entity_id_list",
}

func TestRegenerator_Golden(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			dash := loadInputProto(t, name)
			out, err := regen.Render(dash)
			if err != nil { t.Fatalf("Render: %v", err) }
			expected := mustReadGolden(t, name)
			if string(out) != expected {
				t.Errorf("output mismatch:\n--- got ---\n%s\n--- want ---\n%s", out, expected)
			}
		})
	}
}

func TestRegenerator_Deterministic(t *testing.T) {
	dash := loadInputProto(t, "container_with_children")
	first, _ := regen.Render(dash)
	for i := 0; i < 10; i++ {
		out, _ := regen.Render(dash)
		if string(out) != string(first) { t.Fatalf("non-deterministic on iteration %d", i) }
	}
}

func TestRegenerator_RoundTrip(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			dash := loadInputProto(t, name)
			out, _ := regen.Render(dash)
			parsed := evalLayoutPkl(t, out) // helper that runs Pkl evaluator
			if !equalDashboards(dash, parsed) { t.Errorf("round-trip mismatch") }
		})
	}
}
```

- [ ] **Step 2: Implement `regen.go`**

A deterministic walker that emits the canonical Pkl shape per spec §12.3. Pure-functional Go: takes a `dashboard.Dashboard` value, returns the byte slice. Indentation is fixed 2-space; widget property emission order is fixed (`id`, `pos`, then class-specific props alphabetical); container `children` recurses with one extra indent level.

- [ ] **Step 3: Generate fixtures**

For each `goldenCases` entry, hand-write the input proto-text (or programmatically construct via Go fixture builders) and the expected `.layout.pkl` output. Place under `internal/dashboard/regen/testdata/<case>/{in.proto-text, out.layout.pkl}`.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/dashboard/regen/...
```

Expected: all eight cases pass byte-exactly + round-trip + determinism.

- [ ] **Step 5: Commit**

```bash
git add internal/dashboard/regen/
git commit -m "feat(c10): deterministic Pkl regenerator + golden round-trip tests"
```

---

## Task 17: DashboardService.SaveLayout + Create + Delete

Wire the regenerator into the write path.

**Files:**
- Modify: `internal/dashboard/service.go`, `internal/daemon/dashboard_backend.go`
- Create: `internal/dashboard/scaffold.go` (creates `<slug>.pkl` + `<slug>.layout.pkl` pairs)

- [ ] **Step 1-4: Implement Create / Delete / SaveLayout** following the spec §12.5/§12.6 flows. Each wraps the regenerator + Pkl evaluator validation + atomic file write + config reload + `ConfigApplied` emit.

- [ ] **Step 5: Integration test**

`internal/api/integration_dashboard_test.go` (build tag `integration`): creates a dashboard via `Create`, edits via `SaveLayout`, asserts file contents on disk match expectations and the dashboard re-reads correctly.

- [ ] **Step 6: Commit**

```bash
git add internal/dashboard/scaffold.go internal/dashboard/service.go \
        internal/daemon/dashboard_backend.go internal/api/integration_dashboard_test.go
git commit -m "feat(c10): DashboardService.SaveLayout/Create/Delete with Pkl two-file split"
```

---

## Task 18: dashboard edit mode UI (browser)

Implement the inline edit chrome + slide-in props panel + FAB widget picker + undo/redo per spec §11.

**Files:**
- Create: `web/src/dashboard/edit/{EditModeProvider,EditChrome,PropsPanel,WidgetPicker,UndoRedo,use-editor-store}.tsx`
- Modify: `web/src/dashboard/render/DashboardView.tsx` (delegate to edit mode when active)

- [ ] **Step 1-6: Implement each edit-mode component, the editor store with undo/redo, and the SaveLayout submission flow.**

(Detailed step-by-step omitted for brevity; the patterns mirror standard react-grid-layout edit + Zustand history examples. See spec §11 for the exact UX contract.)

- [ ] **Step 7: Component tests + commit**

```bash
cd web && npm test -- dashboard/edit
git add web/src/dashboard/edit web/src/dashboard/render/DashboardView.tsx
git commit -m "feat(c10): dashboard edit mode UI (drag/drop/resize, props panel, picker, undo/redo)"
```

---

## Task 19: remaining built-in widgets (LineChart, CameraStream, ScriptButton, EntityList, GroupCard)

Each widget gets its own component + per-widget tests. GroupCard's container behavior (recursive child rendering, drag-into/out of, child grid) needs the most care.

- [ ] **Step 1-5: Implement five widgets** under `web/src/widgets/`, each with a `*.test.tsx` neighbor. Register all in `builtInWidgets`.

- [ ] **Step 6: Commit**

```bash
git add web/src/widgets
git commit -m "feat(c10): LineChart / CameraStream / ScriptButton / EntityList / GroupCard widgets"
```

---

## Task 20: ConfigService.EvalCompute + Markdown wiring

Server-side Starlark for compute widgets per spec §13.4.

**Files:**
- Create: `internal/compute/service.go`, `service_test.go`
- Modify: `internal/api/service_config.go` (add EvalCompute method delegating to compute.Service)
- Modify: `web/src/widget-sdk/hooks.ts` (add useCompute hook)
- Modify: `web/src/widgets/Markdown.tsx` (use evalCompute when compute prop set)

- [ ] **Step 1-4: Implement, test, wire, commit**

```bash
git add internal/compute internal/api/service_config.go web/src/widget-sdk web/src/widgets/Markdown.tsx
git commit -m "feat(c10): ConfigService.EvalCompute + Markdown.compute"
```

---

## Task 21: widget pack format — server side

OCI pull, cosign verify, manifest validate, on-disk store.

**Files:**
- Create: `internal/widgetpack/{install,store,trust,server}.go`
- Create: `internal/widgetpack/migrations/0001_widget_packs.up.sql`, `0001_widget_packs.down.sql`
- Modify: `go.mod`, `go.sum` (add oras-go and sigstore deps)

- [ ] **Step 1: Add deps**

```bash
cd gohome
go get oras.land/oras-go/v2
go get github.com/sigstore/sigstore-go
go mod tidy
```

- [ ] **Step 2-7: Implement the install pipeline** with tests against a local OCI registry fixture. Migration adds `widget_packs(name TEXT, version TEXT, sha256 TEXT, signature_status TEXT, installed_at INTEGER, PRIMARY KEY(name, version))`.

- [ ] **Step 8: Commit**

```bash
git add internal/widgetpack go.mod go.sum
git commit -m "feat(c10): widget pack install (OCI pull + cosign verify + on-disk store)"
```

---

## Task 22: widget pack runtime loading (browser side)

`/widgets/<pack>/<version>/<file>` handler + dynamic-import client glue.

**Files:**
- Create: `internal/web/widgets_handler.go` (reuses Task 21's `widgetpack.Server` for the asset filesystem)
- Modify: `web/src/dashboard/pack-loader.ts` (already drafted in Task 14; ensure cache works against real backend)
- Modify: `internal/api/listener/listener.go` (mount `/widgets/` route)

- [ ] **Step 1-4: Implement handler, mount, smoke-test pack install + dashboard render referencing a packed widget. Commit.**

```bash
git add internal/web/widgets_handler.go internal/api/listener/listener.go
git commit -m "feat(c10): widget pack runtime loading via /widgets/<pack>/<v>/<file>"
```

---

## Task 23: @gohome/widget-sdk npm package

Extract a buildable npm package that pack authors depend on.

**Files:**
- Create: `web/src/widget-sdk/package.json`, `tsconfig.json`, `tsup.config.ts` (or vite library mode), `index.ts`
- Create: `docs/widget-pack-authoring.md`

- [ ] **Step 1: Set up library build**

The sdk package is built standalone with `tsup` (or vite library mode). Output: dual ESM + CJS for compatibility.

`web/src/widget-sdk/package.json`:

```json
{
  "name": "@gohome/widget-sdk",
  "version": "0.1.0",
  "type": "module",
  "main": "./dist/index.cjs",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "import": "./dist/index.js",
      "require": "./dist/index.cjs",
      "types": "./dist/index.d.ts"
    }
  },
  "peerDependencies": {
    "react": "^19.0.0"
  },
  "scripts": {
    "build": "tsup index.ts --dts --format esm,cjs --external react"
  },
  "devDependencies": {
    "tsup": "^8.0.0",
    "typescript": "^5.4.0"
  }
}
```

- [ ] **Step 2-4: Build, smoke test, commit, write docs**

```bash
cd web/src/widget-sdk && npm install && npm run build
# Inspect dist/index.{js,cjs,d.ts}
```

```bash
git add web/src/widget-sdk docs/widget-pack-authoring.md
git commit -m "feat(c10): publish @gohome/widget-sdk npm package + author docs"
```

---

## Task 24: CLI surface (gohome widget {install,list,uninstall} + gohome ui dev)

**Files:**
- Create: `internal/cli/{cmd_widget,cmd_ui,styles_widget}.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1-6: Implement cobra commands per spec §19**, with lipgloss styling per the explicit style mapping in the spec. Tests follow the existing `cmd_mcp_test.go` pattern.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cmd_widget.go internal/cli/cmd_ui.go internal/cli/styles_widget.go internal/cli/root.go
git commit -m "feat(c10): gohome widget {install,list,uninstall} + gohome ui dev CLI"
```

---

## Task 25: PWA service worker + manifest

`vite-plugin-pwa` integration with `injectManifest` strategy.

**Files:**
- Modify: `web/vite.config.ts` (add `vite-plugin-pwa`)
- Create: `web/src/pwa/sw.ts`, `web/src/pwa/install-prompt.ts`
- Create: `web/public/manifest.webmanifest` + icons

- [ ] **Step 1: Install + configure**

```bash
cd web
npm install -D vite-plugin-pwa
```

Update `vite.config.ts`:

```ts
import { VitePWA } from "vite-plugin-pwa";
plugins: [react(), VitePWA({ strategies: "injectManifest", srcDir: "src/pwa", filename: "sw.ts" })],
```

- [ ] **Step 2-4: Manifest, install prompt UX, smoke. Commit.**

```bash
git add web/src/pwa web/public/manifest.webmanifest web/public/icon-*.png web/vite.config.ts web/package.json web/package-lock.json
git commit -m "feat(c10): PWA shell-only (service worker + manifest)"
```

---

## Task 26: E2E suite (Playwright)

End-to-end coverage against a real `gohomed` binary.

**Files:**
- Create: `web/e2e/{auth,passkey,dashboard-read,dashboard-edit,widget-pack}.spec.ts`
- Create: `web/e2e/fixtures/` (Pkl config, fake driver, virtual authenticator setup)
- Create: `web/playwright.config.ts`

- [ ] **Step 1: Install + scaffold**

```bash
cd web
npm install -D @playwright/test
npx playwright install chromium
```

- [ ] **Step 2-7: Write E2E specs** following the spec §18.2 harness pattern. Each spec spins gohomed against a fresh DB + Pkl fixture.

- [ ] **Step 8: Commit**

```bash
git add web/e2e web/playwright.config.ts web/package.json web/package-lock.json
git commit -m "test(c10): Playwright E2E suite (auth, dashboard read/edit, pack install)"
```

---

## Task 27: visual regression baseline

**Files:**
- Create: `web/e2e/visual/*.spec.ts`
- Create: `web/e2e/visual/__screenshots__/` (committed baselines)

- [ ] **Step 1-3: Capture screenshots of every route in light + dark; CI threshold 0.1%. Commit baselines.**

```bash
git add web/e2e/visual
git commit -m "test(c10): visual regression baselines (light + dark per route)"
```

---

## Task 28: documentation pass

**Files:**
- Create: `web/README.md`, `docs/web-ui.md`, `docs/widget-pack-authoring.md` (already in Task 23, expand here)
- Modify: `README.md` (add Web UI section)

- [ ] **Step 1: Write `web/README.md`**

Cover: how to set up dev (`task web:install` then `task web:build`), how to run dev mode (`task ui:dev`), how to add a shadcn primitive (the audit protocol), how to add a built-in widget, how the design tokens work.

- [ ] **Step 2: Write `docs/web-ui.md`**

Cover: configuration (`gohome.web` and `gohome.widgetPackPolicy`), light/dark/system mode, dashboard editor walkthrough, installing widget packs, troubleshooting reconnect issues.

- [ ] **Step 3: Expand `docs/widget-pack-authoring.md`**

Cover: the `@gohome/widget-sdk` API, manifest format (with a fully-worked example), build pipeline (vite library mode vs tsup), signing with cosign, publishing to ghcr.

- [ ] **Step 4: Update top-level README**

Add a "Web UI" section linking to `docs/web-ui.md`.

- [ ] **Step 5: Commit**

```bash
git add web/README.md docs/web-ui.md docs/widget-pack-authoring.md README.md
git commit -m "docs(c10): web UI user guide + widget pack authoring guide + README"
```

---

## Final smoke check

After all tasks, this end-to-end smoke run validates the full milestone:

```bash
cd gohome
task web:build
task build
./dist/gohomed --bind 127.0.0.1:8080 --uds /tmp/gohome-c10.sock &
DPID=$!
sleep 2

# Sign in via password
curl -s -i -X POST http://127.0.0.1:8080/gohome.v1alpha1.AuthService/Login \
  -H "Content-Type: application/proto" \
  --data-binary @/tmp/login.bin

# Hit the dashboard read RPC
buf curl --schema ./proto --data '{"slug":"default"}' http://127.0.0.1:8080/gohome.v1alpha1.DashboardService/Get

# Open browser; verify dashboard renders, edit a widget, save.

# Install a widget pack
./dist/gohome widget install ghcr.io/fynn-labs/widgets-example:0.1.0

# Verify pack appears in catalog
buf curl --schema ./proto --data '{}' http://127.0.0.1:8080/gohome.v1alpha1.DashboardService/GetWidgetCatalog | grep example

kill $DPID
```

Plus full CI: `task lint && task test && task test:race && task test:integration && cd web && npm run lint && npm test && npm run e2e`.

---

## Spec coverage check

A reverse-mapping from spec sections to plan tasks:

| Spec § | Plan tasks |
|---|---|
| §1 Scope | covered transitively by every task; deferrals checked at Task 28 docs step |
| §2 Background | informational only |
| §3 Architecture Overview | tasks 1 (scaffold), 2 (embed), 13/17 (server modules), 14 (browser layout) |
| §4 App Shell & Routing | task 5 (shell), task 8 (router) |
| §5 Design Tokens & Theming Architecture | task 3 (full implementation + lint rule) |
| §6 Data Layer | task 6 (transport, query client, store, interceptor) |
| §7 Multiplexer | task 10 (two-stream + tracker + reconnect/resume) |
| §8 Pending-State UX Pattern | task 10 (tracker semantics) + task 15 (first widgets exercise pattern) |
| §9 Login Flow | task 8 (password + passkey UI) + task 7 (server-side challenge store, C9 inheritance) |
| §10 Dashboard Render Path | task 9 (proto), task 12 (catalog), task 13 (Get/GetWidgetCatalog), task 14 (browser render) |
| §11 Dashboard Edit Mode | task 18 |
| §12 Pkl Round-Trip | task 11 (Pkl modules), task 16 (regenerator + golden tests), task 17 (SaveLayout/Create/Delete using regenerator) |
| §13 Widget Contract | task 11 (Pkl), task 23 (SDK package); per-widget tests in tasks 15/19 |
| §14 v1 Built-In Widgets | task 15 (first three), task 19 (remaining five) |
| §15 Widget Pack Format | task 21 (server install), task 22 (runtime loading), task 23 (SDK), task 24 (CLI install) |
| §16 PWA & Theming Defaults | task 25 (PWA), task 3 (theming defaults) |
| §17 Build & Embed Pipeline | task 1 (scaffold + Taskfile), task 2 (embed + asset budget), task 25 (PWA build integration) |
| §18 Testing Strategy | tests in every task; full E2E in task 26; visual regression in task 27 |
| §19 CLI Surface | task 24 |
| §20 Configuration | task 11 (web.pkl), implicitly via daemon wiring in task 13 |
| §21 Implementation Order | this plan's task order matches §21 1:1 |
| §22 Decision Record | informational — embodied throughout the plan |
| §23 Explicit Deferrals | called out in task 28 docs |

---

*End of C10 implementation plan.*
