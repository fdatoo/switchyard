# Codex handoff prompt — Starlark LSP wiring

Paste this whole document into Codex as the first message. It is self-contained: everything you need to pick up where the previous agent left off.

---

## You are picking up an in-progress feature

A previous agent (Claude) brainstormed, spec'd, and planned a Starlark LSP wiring feature for the Switchyard repo. The brainstorming + spec + plan are already written and committed. **Your job is to execute the plan.** Do not re-design.

- **Repo:** `/Users/fdatoo/Developer/Switchyard`
- **Current branch:** `feat/starlark-lsp` (off main; main already contains the prior agent's Track A + Track B work which this builds on)
- **Spec (read this first):** `docs/design/specs/2026-05-12-starlark-lsp-design.md`
- **Plan (the work):** `docs/design/plans/2026-05-12-starlark-lsp.md`
- **Approach:** TDD per task. The plan provides full code for each step. Use subagent-driven execution if you have it, otherwise execute the tasks in order yourself. 9 tasks total; an in-plan "Wave plan" section identifies what can run in parallel.

## Environment quirks the plan doesn't fully cover

The previous agent ran into these on prior tracks. Save yourself the debugging time.

### 1. `protoc-gen-go-grpc` must be on PATH

`buf generate` needs `protoc-gen-go-grpc` to write the `_grpc.pb.go` files. If it's missing, you'll see a regeneration that silently disables grpc binding updates — Track A had this exact bug.

Before running `buf generate`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
which protoc-gen-go-grpc || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

The plan's Task 1 mentions this; do it once at session start.

### 2. The daemon's already running and has a lockfile

The previous agent left a `dist/switchyardd` running in the background to validate Track A + B. It serves on:

- TCP `127.0.0.1:8080`
- Admin port `9190`
- Unix socket `~/.local/share/switchyard/switchyardd.sock`
- Lockfile `~/.local/share/switchyard/switchyardd.lock`

After daemon-side changes (anything in `internal/` or `cmd/switchyardd/`), rebuild + restart:

```bash
go build -o dist/switchyardd ./cmd/switchyardd
lsof -ti:8080 2>/dev/null | xargs -I {} kill {}
sleep 1
rm -f ~/.local/share/switchyard/switchyardd.lock
./dist/switchyardd &
sleep 4
test -e ~/.local/share/switchyard/switchyardd.sock && echo "daemon up" || echo "daemon FAILED"
```

The lockfile-clear step is essential — if you skip it, the new daemon will refuse to start with `lockfile: switchyardd already running (pid …)` even after you've killed the old one.

If the prior daemon was a foreground `go run`, it may have left children. `lsof -ti:8080` is the canonical "what's holding the port" probe.

### 3. The vite dev server is running on :5174

The previous agent started `npm run dev` (vite) in a background shell. If it's still alive, your TypeScript/Vue changes hot-reload automatically. To check:

```bash
lsof -ti:5174 2>/dev/null
```

If nothing's there, start it:

```bash
cd app && npm run dev &
```

The vite proxy default-routes daemon RPCs to the Unix socket above. No extra config.

### 4. `vite-plugin-monaco-editor` has a Node-API bug

The plugin calls `fs.rmdirSync(path, { recursive: true })` which Node 16+ rejects. A `patch-package` patch lives at `app/patches/vite-plugin-monaco-editor+1.1.0.patch` and is applied via `postinstall`. **Do not run `npm install` without keeping that file**, and do not delete `app/patches/` — vite will crash on startup without it.

If you ever see `TypeError [ERR_INVALID_ARG_VALUE]: The property 'options.recursive' is no longer supported` from vite-plugin-monaco-editor, the fix is `cd app && npx patch-package`.

### 5. There is also a `tsconfig.node.json` typecheck warning

`vue-tsc -b --noEmit` reports `TS6310: Referenced project 'tsconfig.node.json' may not disable emit`. **This is pre-existing and unrelated to anything in this plan.** Ignore it. The real signal is whether any OTHER errors appear.

## Validation: use Playwright

The plan's Task 9 calls for a manual or Playwright pass. If you have access to Playwright (via MCP, your own Playwright binding, or a saved-state browser session), drive the validation that way rather than asking the user to manually click around. The previous agent used MCP's `mcp__playwright__browser_*` tools; if you have the same, the flow is:

```js
// 1. Navigate to the starlark editor
await page.goto("http://localhost:5174/settings/starlark");

// 2. Wait for the file tree to load
await page.waitForSelector("[role='treeitem']");

// 3. Click an existing .star file
await page.locator("[role='treeitem']").first().click();

// 4. The Monaco editor mounts. Type a typo and wait for diagnostics:
await page.locator(".monaco-editor textarea").type("prnt(1)\n");
await page.waitForTimeout(500); // diagnose debounce is 300ms
// Then assert a marker appears for "prnt" — Monaco renders marker as
// `.squiggly-warning` decoration; check via DOM presence or by reading
// model markers via `await page.evaluate(() => monaco.editor.getModelMarkers({}))`.
```

If you don't have Playwright, ask the user to validate manually with the checklist from the plan's Task 9 step 3, or have them open the browser and tell you what they see.

## Branch ergonomics

This is the user's normal workflow:

- Land work on a feature branch (`feat/starlark-lsp` in this case).
- When complete + validated, fast-forward `main` to the feature branch and push:

```bash
git checkout main
git merge --ff-only feat/starlark-lsp
git push origin main
```

There's no PR-based workflow; the solo developer reviews their own work via the diff. Don't open PRs on GitHub; just push.

**Do confirm with the user before pushing to main** — that's a shared branch even if there's only one developer.

## What's on `main` (context for the plan's references)

The plan references things shipped on earlier tracks that you should know exist:

- **`internal/automation/scene.Applier`** — runs scene actions in parallel, used by `SceneService.Apply` (Track B). Not used by the LSP work, but exists nearby.
- **`internal/config.Reloader` + `ConfigPubsub`** — debounced config reloader + pubsub for `ConfigChanged` events (Track A).
- **`app/src/stores/config-store.ts`** — front-end singleton subscribed to `ConfigService.Subscribe` (Track A). The Starlark editor doesn't need to integrate with it, but if you find yourself wondering "how does a list refresh after a save?", that's the answer.
- **`internal/starlarkls`** package — the LSP backend lives here, four RPCs already shipped, you're adding Diagnose.
- **`app/src/lib/components/code-editor/`** — `SyCodeEditor.vue` (Monaco wrapper, currently Pkl-aware), `pkl-grammar.ts` (existing Monarch grammar). Mirror the pkl pattern for Starlark.
- **`app/src/lib/components/code-editor-panel/SyCodeEditorPanel.vue`** — the outer shell with file tree + editor. Sets `language` on the inner SyCodeEditor based on `kind` prop.

## Memory & preferences the user has expressed

- **No emojis** in code/commits/UI unless explicitly requested.
- **Conventional-commit-ish messages** (`feat(scope): …`, `fix(scope): …`, `docs: …`, `test(scope): …`). Trailer: `Co-Authored-By: <model name> <noreply@anthropic.com>` if the model produced the commit. The previous agent used `Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>`; substitute your own model identity.
- **TDD discipline**: failing test → impl → passing test → commit. The plan structures every task this way.
- **Hardware-testing rule (memory):** real Hue/sensor hardware is OK for E2E, but **never** the Bedroom lamp. Validation should avoid touching `light.bedroom_*` entities. The Office and Living-room entities are safe.

## Recovery if you get stuck

1. **Build broken after a regen?** Did `protoc-gen-go-grpc` run? Did the interface assertion break? The plan's Task 1 Step 2 explicitly handles this.
2. **Daemon won't start?** Lockfile + port: see section above.
3. **Vite shows an import error?** `app/patches/` — see section above.
4. **Test references a field that doesn't exist?** `go doc` the type. The previous agent learned several go.starlark.net field names by reading the package docs directly. Same here.
5. **Truly blocked?** Stop, write the blocker to `docs/design/plans/2026-05-12-starlark-lsp-progress.md` (create it; mirror the format of `docs/design/plans/2026-05-12-reactive-config-subscription-progress.md`), and tell the user.

## How to start

1. Read the spec: `docs/design/specs/2026-05-12-starlark-lsp-design.md`.
2. Read the plan: `docs/design/plans/2026-05-12-starlark-lsp.md`.
3. Run `git log --oneline -5` to confirm you're on `feat/starlark-lsp` at the plan-commit head.
4. Ensure environment quirks above are addressed.
5. Begin with Task 1.

Good luck.
