# Plan 05 — Command Palette

> **Depends on Plan 01.** Uses `LanguagePrimitives`, the `Surface` primitive, and the ⌘K affordance slot in `TopBar.tsx` that Plan 01 ships. Must not start until Plan 01 is merged to main.

**Goal:** A desktop command palette (⌘K) with a registered server-side verb catalog, client-side verb + argument parser, four default-state sections (recently used / suggested / jump-to / ask), configurable CLI preview, and a server-side `CommandCatalogService` that all in-process packages register their verbs into.

**Spec refs:** §11 (Command palette), §19.2 (not on mobile).

**Branch:** `feat/ui-v2-plan-05-command-palette`
**Worktree:** `.claude/worktrees/plan-05-command-palette`
**Depends on:** Plan 01 merged to main
**Linear parent:** TBD

---

## Decisions (locked — no ambiguity for the implementer)

1. **Verb catalog is server-side.** A new `CommandCatalogService` exposes one RPC: `List()`. It returns all registered verbs with name, description, argument schema (typed), required vs optional, CLI flag mapping, and a handler ref. The web palette fetches the catalog on mount (cached in TanStack Query with a 5-minute stale time).
2. **In-process registry.** Verbs are registered at startup by calling `registry.Register(verb)` from each domain package's `init()` or explicit `RegisterCommands()` function. No external plugin mechanism in v2.
3. **Desktop-only.** The ⌘K listener is registered only on breakpoints ≥ 1024px (`window.matchMedia("(min-width: 1024px)")`). Below that threshold, the listener is a no-op; mobile uses the search affordance from Plan 13.
4. **Default-state section order:** Recently used → Suggested → Jump to → Ask. Sections with no items are omitted from the rendered list entirely.
5. **Recently used:** last 7 days, stored in `localStorage` under key `sy.palette.recentlyUsed` as a JSON array of `{ verbName, args, ranAt }` records, capped at 50 entries. Deduplicated by exact verb+args match on write; most recent wins.
6. **Suggested:** computed on open from (a) the active TanStack Router route (route-to-suggestion map, hardcoded), and (b) any interestingness alerts surfaced in the most recent multiplexer snapshot. Suggestions are plain `SuggestedAction` objects `{ label, verb?, args?, href? }`.
7. **Jump to:** static list of named page routes with their keyboard shortcuts, generated from the route tree; identical to what the sidebar renders.
8. **Ask:** rendered only when `McpService.IsConfigured()` returns `true`. Opening it sends the current palette input text as the initial message to the agent chat flow (Plan 9 wires the chat panel; Plan 5 ships the gating hook and the "Ask" section row that, until Plan 9, navigates to a placeholder).
9. **Active state:** the input is tokenized into a verb prefix and space-separated `key:value` pairs. Tokenizer is purely client-side — no server round-trips during typing. Suggestions update synchronously from the cached catalog.
10. **Tab fills** the next missing required arg using the highlighted suggestion's first suggested value.
11. **CLI preview:** shown only when `localStorage.sy.palette.cliPreview === "on"`. Default is `"off"`. The Settings → Theme & language page (Plan 9) toggles it via `setPaletteCliPreview(value)` exported from `recently-used.ts`.
12. **Keyboard contract:** ⌘K opens; Esc closes; Enter runs and closes; ⇧↵ runs and stays open; ⌘' opens Ask (only if MCP configured — otherwise a no-op).
13. **Proto package path:** `proto/switchyard/commandcatalog/v1/catalog.proto`. ConnectRPC service name `switchyard.commandcatalog.v1.CommandCatalogService`. Go package `internal/commandcatalog`.
14. **Registration shims** for each domain that owns verbs live in the owning domain package (e.g., `internal/activity/register-commands.go`). The `switchyardd` binary calls all `RegisterCommands()` functions from `main.go` after the registry is initialized.
15. **`handler_ref`** in the proto is a string key (`"events.tail"`, `"entity.get"`, etc.) that the CLI and MCP server use to dispatch. The web palette never calls it directly; it constructs a ConnectRPC call to the appropriate service instead.

---

## Built-in verb catalog (exact arg schemas)

All argument types: `string`, `int`, `bool`, `duration` (e.g., `"1h"`, `"30m"`), `string_list`.

| Verb | Required args | Optional args | CLI form |
|------|--------------|--------------|----------|
| `events tail` | — | `source:string`, `kind:string`, `entity:string`, `since:duration` | `switchyard event tail` |
| `events query` | — | `kind:string`, `source:string`, `entity:string`, `issuedBy:string`, `since:duration`, `until:duration`, `limit:int` | `switchyard event query` |
| `entity get` | `id:string` | — | `switchyard entity get <id>` |
| `entity call-capability` | `id:string`, `capability:string` | `args:string` | `switchyard entity call <id> <capability>` |
| `automation run` | `id:string` | — | `switchyard automation run <id>` |
| `automation enable` | `id:string` | — | `switchyard automation enable <id>` |
| `automation disable` | `id:string` | — | `switchyard automation disable <id>` |
| `driver restart` | `name:string` | — | `switchyard driver restart <name>` |
| `driver logs` | `name:string` | `lines:int` | `switchyard driver logs <name>` |
| `driver list` | — | — | `switchyard driver list` |
| `config apply` | — | `path:string` | `switchyard config apply` |
| `config validate` | — | `path:string` | `switchyard config validate` |
| `pkl open` | `path:string` | — | `switchyard pkl open <path>` |
| `page open` | `slug:string` | — | `switchyard page open <slug>` |
| `page create` | `slug:string` | — | `switchyard page create <slug>` |
| `page export` | `slug:string` | — | `switchyard page export <slug>` |
| `widget install` | `oci_ref:string` | — | `switchyard widget install <oci_ref>` |
| `widget list` | — | — | `switchyard widget list` |
| `token issue` | `name:string`, `scopes:string_list` | — | `switchyard token issue <name>` |
| `passkey enroll` | — | — | `switchyard passkey enroll` |
| `display pair` | — | — | `switchyard display pair` |
| `display configure` | `id:string` | — | `switchyard display configure <id>` |

---

## File plan

### Created (server-side)

```
proto/switchyard/commandcatalog/v1/
  catalog.proto                         ← CommandCatalogService + Verb + ArgSchema messages

internal/commandcatalog/
  registry.go                           ← Verb type, Registry, global instance, service impl
  registry_test.go

internal/activity/
  register-commands.go                  ← registers: events tail, events query

internal/entity/
  register-commands.go                  ← registers: entity get, entity call-capability

internal/automation/
  register-commands.go                  ← registers: automation run/enable/disable

internal/driver/
  register-commands.go                  ← registers: driver restart, driver logs, driver list

internal/config/
  register-commands.go                  ← registers: config apply, config validate

internal/pkl/
  register-commands.go                  ← registers: pkl open

internal/dashboard/
  register-commands.go                  ← registers: page open, page create, page export

internal/widget/
  register-commands.go                  ← registers: widget install, widget list

internal/auth/
  register-commands.go                  ← registers: token issue, passkey enroll

internal/display/
  register-commands.go                  ← registers: display pair, display configure
```

### Created (web)

```
web/src/palette/
  palette.tsx                           ← modal shell (Radix Dialog-based)
  palette-state.ts                      ← parser, verb matching, suggestion builder
  palette-state.test.ts
  verb-catalog-client.ts                ← TanStack Query wrapper for CommandCatalogService.List
  recently-used.ts                      ← read/write localStorage, CLI preview pref export
  recently-used.test.ts
  keyboard.ts                           ← globalThis ⌘K listener, desktop breakpoint guard
  keyboard.test.ts
```

### Modified

```
web/src/shell/TopBar.tsx                ← wire ⌘K button to open the palette
web/src/main.tsx                        ← mount <PaletteProvider> around the router (provides open/close context)
internal/server/routes.go               ← register CommandCatalogService handler
cmd/switchyardd/main.go                 ← call all RegisterCommands() functions at startup
```

---

## Proto shape

```proto
syntax = "proto3";
package switchyard.commandcatalog.v1;

service CommandCatalogService {
  rpc List(ListRequest) returns (ListResponse) {}
}

message ListRequest {}

message ListResponse {
  repeated Verb verbs = 1;
}

message Verb {
  string name        = 1; // e.g. "events tail"
  string description = 2;
  repeated ArgSchema args = 3;
  string cli_form    = 4; // e.g. "switchyard event tail"
  string handler_ref = 5; // e.g. "events.tail"
}

message ArgSchema {
  string   name     = 1;
  ArgType  type     = 2;
  bool     required = 3;
  string   cli_flag = 4; // e.g. "--source"
  string   hint     = 5; // shown in the arg chip placeholder
}

enum ArgType {
  ARG_TYPE_UNSPECIFIED = 0;
  ARG_TYPE_STRING      = 1;
  ARG_TYPE_INT         = 2;
  ARG_TYPE_BOOL        = 3;
  ARG_TYPE_DURATION    = 4;
  ARG_TYPE_STRING_LIST = 5;
}
```

---

## Tasks

### Task 5.1 — Define `catalog.proto` and run buf generate

**Files:** `proto/switchyard/commandcatalog/v1/catalog.proto`.

Author the proto schema exactly as the shape above. Run `buf generate` to produce the Go and TypeScript bindings. Verify the generated Go compiles (`go build ./...`).

**Acceptance:**
- `proto/switchyard/commandcatalog/v1/catalog.proto` exists with `CommandCatalogService`, `Verb`, `ArgSchema`, and `ArgType`.
- `buf generate` exits 0 and produces generated files in the expected output paths.
- `go build ./...` is green.

**Commit:** `feat(proto): CommandCatalogService for verb catalog (UI v2 plan 05)`

---

### Task 5.2 — Server-side registry + CommandCatalogService impl + test

**File:** `internal/commandcatalog/registry.go`

Implement:

```go
type ArgType int
const (
  ArgTypeString ArgType = iota + 1
  ArgTypeInt
  ArgTypeBool
  ArgTypeDuration
  ArgTypeStringList
)

type ArgSchema struct {
  Name     string
  Type     ArgType
  Required bool
  CLIFlag  string
  Hint     string
}

type Verb struct {
  Name        string
  Description string
  Args        []ArgSchema
  CLIForm     string
  HandlerRef  string
}

type Registry struct { … }
func NewRegistry() *Registry
func (r *Registry) Register(v Verb)
func (r *Registry) All() []Verb

// CommandCatalogService implements the generated ConnectRPC interface.
type CommandCatalogService struct { registry *Registry }
func (s *CommandCatalogService) List(ctx, req) (resp, error)
```

Register one verb in the same file (`events tail`) so the test can verify the service without depending on the activity package.

**TDD:**
1. Write `registry_test.go`. Assertions:
   - Registering a verb and calling `All()` returns it.
   - Registering a duplicate name (same `Name`) panics (fail fast; registrations happen at startup).
   - `List` RPC returns the registered verb with all fields populated.
2. Implement to make the tests pass.

**Acceptance:** `go test ./internal/commandcatalog/...` green.

**Commit:** `feat(go): CommandCatalog registry + service (UI v2 plan 05)`

---

### Task 5.3 — Per-domain registration shims

**Files:** one `register-commands.go` per domain package listed in the file plan.

Each file exposes a `RegisterCommands(r *commandcatalog.Registry)` function. The function calls `r.Register(...)` for each verb the domain owns (see the built-in verb catalog table). Use the exact arg schemas from the table.

Remove the seed verb from `registry.go` (it was only there for Task 5.2's test isolation).

Wire all `RegisterCommands()` calls in `cmd/switchyardd/main.go` (after `commandcatalog.NewRegistry()` and before the HTTP server starts).

**Acceptance:**
- `go build ./...` green.
- A table-driven test in `internal/commandcatalog/registry_test.go` that constructs a registry, calls all `RegisterCommands` functions, and asserts that all 22 verbs from the catalog table are present and have the correct required/optional arg shapes.

**Commit:** `feat(go): per-domain CommandCatalog registration shims (UI v2 plan 05)`

---

### Task 5.4 — CommandCatalogService mounted + integration test

**Files:** `internal/server/routes.go`, new `internal/commandcatalog/service_test.go`.

Mount the service in the ConnectRPC router alongside the existing services.

Write an integration test that spins up an in-process test server, calls `CommandCatalogService.List`, and asserts:
- At least 22 verbs are returned.
- `events tail` has `source`, `kind`, `entity`, `since` as optional args.
- `entity get` has `id` as a required arg.
- `driver logs` has `name` required and `lines` optional.

**Acceptance:** `go test ./internal/commandcatalog/...` and `go test ./internal/server/...` both green.

**Commit:** `feat(go): mount CommandCatalogService + integration test (UI v2 plan 05)`

---

### Task 5.5 — Client-side palette state machine (parser) + tests

**Files:** `web/src/palette/palette-state.ts`, `web/src/palette/palette-state.test.ts`.

The state machine is a pure function: `parsePaletteInput(input: string, catalog: Verb[]): ParsedPaletteState`.

```ts
type ParsedPaletteState =
  | { kind: "empty" }
  | { kind: "partial"; verbCandidates: Verb[]; rawInput: string }
  | { kind: "resolved"; verb: Verb; filledArgs: Record<string, string>; missingRequired: ArgSchema[]; missingOptional: ArgSchema[]; cliPreview: string };
```

Tokenization rules:
- Input is split on whitespace.
- The first one or two tokens are matched against verb names (try two-word match first, then one-word).
- Remaining tokens of the form `key:value` fill the matching arg by name.
- Remaining tokens that are not `key:value` are checked against arg names in schema order; the first unfilled positional arg gets the value.
- `cliPreview` is assembled from `verb.cliForm` + filled args rendered as `--flag=value` in schema order.

**TDD** (write tests before implementation):

| Input | Expected kind | Key assertions |
|-------|--------------|----------------|
| `""` | `empty` | — |
| `"tai"` | `partial` | candidates include `events tail` |
| `"tail"` | `partial` | candidates include `events tail` |
| `"events tail"` | `resolved` | verb.name = `events tail`; no filledArgs; missingRequired empty; missingOptional has `source`, `kind`, `entity`, `since` |
| `"events tail z2m"` | `resolved` | filledArgs = `{ source: "z2m" }` (first positional fills first arg) |
| `"tail source:z2m kind:err"` | `resolved` | verb matches `events tail`; filledArgs = `{ source: "z2m", kind: "err" }` |
| `"entity get abc-123"` | `resolved` | filledArgs = `{ id: "abc-123" }`; missingRequired empty |
| `"entity get"` | `resolved` | missingRequired = `[{ name: "id" }]` |

**Acceptance:** `task web:test` green; all 8 table cases pass.

**Commit:** `feat(web): palette state machine (parser + suggestions) (UI v2 plan 05)`

---

### Task 5.6 — Palette modal UI — default + active states

**File:** `web/src/palette/palette.tsx`

Built on Radix `Dialog` with a custom `DialogContent` override (no backdrop close on click outside — Esc only). Uses the `Surface` primitive from Plan 01's `LanguagePrimitives`.

**Default state** (when `parsePaletteState.kind === "empty"`):
- Render four sections in order, omitting any with zero items:
  - **Recently used** — read from `useRecentlyUsed()` (last 7 days, up to 5 shown, "More →" if >5). Each row shows verb name + arg summary chips + relative time.
  - **Suggested** — from `useSuggested(currentRoute)`. Each row shows label + optional context annotation.
  - **Jump to** — static rows: Activity (⌘3), Rooms › [active room if any], Settings › Drivers, Settings › Account · passkeys.
  - **Ask** — single row "Ask the Switchyard agent…" shown only when `useMcpConfigured()` returns true.
- Section headers are small-caps `--sy-color-fg-4` labels with a secondary annotation (e.g., "LAST 7 DAYS" next to "RECENTLY USED").
- Keyboard hint bar at the bottom: `↵ open · ↑↓ navigate · ⇥ fill arg · ⇧↵ run + stay · esc close`.

**Active state** (when `parsePaletteState.kind === "resolved" | "partial"`):
- Below the input, render:
  - Resolved-as row: verb chip (filled, accent-colored) + arg chips (filled = solid, missing required = dashed accent, missing optional = dashed muted). When `cliPreview` pref is on, show the CLI string right-aligned in `--sy-font-numeric --sy-color-fg-4`.
  - Result list: top match highlighted; variants with different arg combinations shown below it.
  - Related actions section (if verb has a matching entity/driver in the current context).
  - Help row: verb name + "verb · N args · accepts …" + docs link.
- When `parsePaletteState.kind === "partial"`: show matching verb candidates as the result list without the arg chips row.

**Visual spec:** match mockups `07-command-palette-01.png` (default) and `07-command-palette-02.png` (active). Palette is 580px wide max, centered, 56px from top. Items are 48px tall. Accent color for verb chips: `--sy-color-accent`. Jump-to section icon background: `--sy-color-purple` at 20% opacity.

**TDD:**
- Render palette in default state → assert "RECENTLY USED" and "JUMP TO" sections visible.
- Render with MCP not configured → assert "Ask" section absent.
- Render with MCP configured → assert "Ask" section present.
- Render with input `"tail z2m"` → assert verb chip `events tail` visible + arg chip `source :z2m` visible.
- Render with `cliPreview = "on"` → assert CLI string `switchyard event tail --source=z2m` visible.
- Render with `cliPreview = "off"` → assert CLI string absent.

**Acceptance:** `task web:test` green; visual output matches mockups on manual review.

**Commit:** `feat(web): palette modal UI (default + active states) (UI v2 plan 05)`

---

### Task 5.7 — Wire ⌘K keyboard listener + TopBar button

**Files:** `web/src/palette/keyboard.ts`, `web/src/shell/TopBar.tsx`, `web/src/main.tsx`.

`keyboard.ts` exports `useGlobalPaletteShortcut(open: () => void)`. It calls `window.addEventListener("keydown", ...)` inside a `useEffect`. The listener fires only when `window.matchMedia("(min-width: 1024px)").matches` is true at the time of the keydown event. Handles both ⌘K (macOS) and Ctrl+K (Windows/Linux). Cleans up on unmount.

A separate `useMcpAskShortcut(openAsk: () => void)` handles ⌘' / Ctrl+' and is a no-op when MCP is not configured.

`TopBar.tsx`: replace the placeholder `<button>` from Plan 01 (which did nothing) with `<button onClick={openPalette}>`. The button text is `⌘K` on macOS, `Ctrl+K` on other platforms (detected via `navigator.platform`).

`main.tsx`: wrap the router in `<PaletteProvider>` which holds the open/closed state and exposes `openPalette` via context. `PaletteProvider` renders `<Palette />` at the root level so it appears above all page content.

**TDD:**
- `keyboard.test.ts`: fire a synthetic `keydown` event with `{key: "k", metaKey: true}` at 1280px → assert `open` was called.
- Fire the same event at 800px (mock `matchMedia` to return false) → assert `open` was NOT called.
- Fire with `{key: "'", metaKey: true}` when MCP configured → assert `openAsk` called.
- Fire with `{key: "'", metaKey: true}` when MCP not configured → assert `openAsk` not called.

**Acceptance:** `task web:test` green; ⌘K opens the palette in the running app; TopBar button also opens it.

**Commit:** `feat(web): ⌘K keyboard listener + TopBar wiring (UI v2 plan 05)`

---

### Task 5.8 — CLI preview rendering + toggle wiring

**Files:** `web/src/palette/recently-used.ts` (extend), `web/src/palette/palette.tsx` (extend).

`recently-used.ts` already holds `localStorage` helpers; extend it:

```ts
export function getPaletteCliPreview(): boolean
export function setPaletteCliPreview(on: boolean): void
export function usePaletteCliPreview(): [boolean, (on: boolean) => void]
```

`usePaletteCliPreview` is a React hook backed by a `useSyncExternalStore` subscription to the storage key `sy.palette.cliPreview`. Default is `false` (off).

In `palette.tsx`, consume `usePaletteCliPreview()`. When `on`, render the CLI preview string: right-aligned in the arg-chips row, monospace (`--sy-font-numeric`), `--sy-color-fg-4`, with a small copy-to-clipboard button that appears on hover.

**TDD:**
- `recently-used.test.ts`: `getPaletteCliPreview()` returns false when key is absent; returns true when key is `"on"`; `setPaletteCliPreview(true)` writes `"on"`.
- `palette.tsx` test: render with `sy.palette.cliPreview = "on"` and resolved `events tail source=z2m` → assert `switchyard event tail --source=z2m` is in the document.
- Same with `cliPreview = "off"` → assert it is absent.

**Acceptance:** `task web:test` green; toggling the preference in the dev console immediately shows/hides the CLI preview in the palette (the `useSyncExternalStore` subscription drives it without a page reload).

**Commit:** `feat(web): CLI preview rendering + localStorage toggle (UI v2 plan 05)`

---

### Task 5.9 — Ask-an-agent affordance gated by `McpService.IsConfigured()`

**Files:** `web/src/palette/palette.tsx` (extend), new `web/src/palette/use-mcp-configured.ts`.

`use-mcp-configured.ts` exports `useMcpConfigured(): boolean`. It calls `McpService.IsConfigured()` via TanStack Query, cached for the session (no background refetch). The hook returns `false` while loading.

The `McpService.IsConfigured()` RPC is defined in the existing C8 proto (`proto/switchyard/mcp/v1/mcp.proto`). If `IsConfigured` does not yet exist on the service, add it (a boolean-return unary RPC that reads the server's MCP config presence). Server implementation returns `true` when the MCP server config section exists in the loaded Pkl config.

In `palette.tsx`:
- The "Ask" section row is rendered only when `useMcpConfigured() === true`.
- Clicking "Ask" (or pressing ⌘') dispatches a `navigate` to `/ask?q=<current-input>` — a placeholder route that Plan 9 (Settings + agent chat panel) will claim. In Plan 5, that route renders a `<PlaceholderPage title="Ask" plan="Plan 09" />`.
- Add `_authed/ask.tsx` placeholder route to the route tree (if Plan 01 didn't include it).

**TDD:**
- Mock `McpService.IsConfigured` returning false → render palette → assert "Ask" section absent.
- Mock returning true → assert "Ask" section present with text "Ask the Switchyard agent…".
- Clicking "Ask" row → assert navigation to `/ask`.
- ⌘' with MCP true → assert navigation to `/ask?q=<input>`.

**Acceptance:** `task web:test` green; end-to-end: with no MCP config, the Ask section is invisible.

**Commit:** `feat(web): Ask affordance gated by McpService.IsConfigured (UI v2 plan 05)`

---

### Task 5.10 — Playwright snapshot test

**File:** `web/e2e/palette-snapshot.spec.ts`

Two scenarios:

1. **Default state** — open the palette via `.click('[data-testid="topbar-palette-btn"]')`, assert no input, take a full-component screenshot of the palette element (`data-testid="palette-modal"`). Commit reference image under `web/e2e/__screenshots__/palette/`.

2. **Active state** — type `"tail z2m"` into the palette input, wait for the resolved-as row to appear (`data-testid="palette-resolved-verb"`), take screenshot. Commit reference image.

Both snapshots run in `friendly-dark` (set `document.documentElement.dataset.theme = "friendly-dark"` via `page.evaluate`). Two language modes is enough; the palette's tokens are the same across languages.

The test mocks the `CommandCatalogService.List` response (MSW handler registered in `web/e2e/fixtures/msw-handlers.ts`) so the test is hermetic.

**Acceptance:** `task web:e2e` green; reference images committed; the test is added to the CI matrix's `web-e2e` job.

**Commit:** `test(web): Playwright snapshot of command palette (UI v2 plan 05)`

---

## Test plan

- `go test ./internal/commandcatalog/...` — registry, service impl, all 22 verbs registered with correct schemas.
- `go test ./internal/server/...` — CommandCatalogService mounted and reachable.
- `go build ./...` — clean.
- `task web:test` — palette-state parser (8 table cases), recently-used localStorage helpers, CLI preview toggle, keyboard shortcut routing, palette modal section visibility gating, Ask gating.
- `task web:e2e` — Playwright snapshots of default and active states in friendly-dark.
- Manual smoke: `task ui:dev`, open palette with ⌘K, type `"tail z2m"`, verify verb chip + arg chip + CLI preview (after enabling it via dev console).

---

## Acceptance criteria for merging

- All Go tests and `go vet ./...` green locally and in CI.
- `task web:lint`, `task web:test`, `task web:build` all green.
- `task web:e2e` snapshot test passes; reference images committed.
- All 22 built-in verbs present in the catalog with correct required/optional arg schemas (verified by the integration test in Task 5.4).
- ⌘K opens the palette on desktop breakpoints; does not fire on mobile breakpoints.
- Parser resolves `"tail z2m"` to `events tail` with `source=z2m` and correct missing-arg chips.
- CLI preview visible when pref is on; absent when off.
- Ask section absent when MCP not configured; present when configured.
- ⇧↵ runs the command and keeps the palette open; Esc closes; Tab fills the next missing arg.
- TopBar ⌘K button opens the palette.
- Plan 01's `LanguagePrimitives` `Surface` primitive used for the palette backdrop; no hardcoded colors.
- Linear parent issue + sub-tasks transition to `Done`.
- Branch merged via `git merge --no-ff` into main.
