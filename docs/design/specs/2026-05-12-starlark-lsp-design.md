# Starlark LSP wiring — design spec

**Date:** 2026-05-12
**Status:** approved, ready for plan
**Closes:** the gap where `internal/starlarkls` ships a real Connect service (`Tokenize`/`Complete`/`Hover`/`LookupSymbol`) the front-end editor never calls. Today `SyCodeEditorPanel.vue:42` opens `.star` files in Monaco with `language: "python"` and zero LSP integration.

## Goal

Make the Starlark editor feel like a real coding surface: server-supplied colors, completion, hover docs, go-to-definition, and live diagnostics (parse errors, dangling `load()` paths, unresolved names).

Concretely:

1. Register a new Monaco `"starlark"` language with a Monarch fallback grammar (cloned from Python with `load(…)` adjustments). Map `.star` files to it.
2. Wire five providers attached to the `"starlark"` language: Completion, Hover, Definition, Document Semantic Tokens, Diagnostics.
3. Extend the daemon's `StarlarkLsService` with a `Diagnose` RPC that parses + analyzes a source string and returns Diagnostics (parse errors, `load_not_found`, `unresolved_name`).

## Non-goals

- LSP semantic-tokens delta encoding. v1 always returns full token sets.
- Wiring LSP into Pkl-form Starlark fields (`StarlarkAction`/`StarlarkCondition` inputs that today render as `<textarea>` inside `SyAutomationForm`). Separate scope.
- Type checking, dead-code analysis, or formatting.
- Multi-file refactoring (rename across files).
- Workspace-wide diagnostics that aggregate across files. v1 diagnostics is per-file on open + edit.

## Architecture

```
                   ┌──────────────────────────────────────┐
   Monaco edits ──►│ SyCodeEditorPanel (language=starlark)│
                   └────────────────┬─────────────────────┘
                                    │ provideX hooks fire
                                    ▼
                   ┌──────────────────────────────────────┐
                   │ starlark-providers.ts                │
                   │   • CompletionItemProvider           │
                   │   • HoverProvider                    │
                   │   • DefinitionProvider               │
                   │   • DocumentSemanticTokensProvider   │
                   │   • setModelMarkers (diagnostics)    │
                   └────────────────┬─────────────────────┘
                                    │ Connect RPCs
                                    ▼
                   ┌──────────────────────────────────────┐
                   │ StarlarkLsService (daemon)           │
                   │   Tokenize / Complete / Hover /      │
                   │   LookupSymbol / Diagnose (new)      │
                   └──────────────────────────────────────┘
```

## File map

| File | Status | Responsibility |
|------|--------|----------------|
| `proto/switchyard/starlarkls/v1/starlarkls.proto` | MOD | Add `Diagnose` RPC + `DiagnoseRequest`/`Diagnostic`/`DiagnoseResponse` messages. |
| `gen/.../starlarkls/v1/` | GEN | Regenerated proto bindings. |
| `internal/starlarkls/diagnose.go` | NEW | AST walker that produces diagnostics. |
| `internal/starlarkls/diagnose_test.go` | NEW | Unit tests. |
| `internal/starlarkls/service.go` | MOD | Add `Diagnose` method to the existing `Service`. |
| `app/src/data/starlark-ls.ts` | NEW | TS client wrapping the five RPCs. |
| `app/src/lib/monaco/starlark.ts` | NEW | `setupStarlark(monaco)`: language registration + Monarch fallback grammar + extension mapping. |
| `app/src/lib/monaco/starlark-providers.ts` | NEW | The five Monaco providers wired to the RPC client. Also semantic-tokens encoder and the diagnostics debounce loop. |
| `app/src/lib/monaco/index.ts` | MOD (or NEW) | Single entry point that wires Pkl + Starlark setups; called once at Monaco init. |
| `app/src/lib/components/code-editor-panel/SyCodeEditorPanel.vue` | MOD | One-line: language computed returns `"starlark"` not `"python"` for `kind === "starlark"`. Also: listen for the `starlark-goto-definition` window event and call `openFile()`. |

## Daemon-side: Diagnose

### Proto extension

```protobuf
service StarlarkLsService {
  rpc Tokenize(TokenizeRequest)         returns (TokenizeResponse);
  rpc Complete(CompleteRequest)         returns (CompleteResponse);
  rpc Hover(HoverRequest)               returns (HoverResponse);
  rpc LookupSymbol(LookupSymbolRequest) returns (LookupSymbolResponse);
  rpc Diagnose(DiagnoseRequest)         returns (DiagnoseResponse);
}

message DiagnoseRequest {
  string file_path = 1;
  string source    = 2;
}

message Diagnostic {
  int32  start_line = 1;
  int32  start_col  = 2;
  int32  end_line   = 3;
  int32  end_col    = 4;
  string severity   = 5;  // "error" | "warning"
  string message    = 6;
  string code       = 7;  // "parse_error" | "load_not_found" | "unresolved_name"
}

message DiagnoseResponse {
  repeated Diagnostic diagnostics = 1;
}
```

### Analysis pipeline

`internal/starlarkls/diagnose.go` exports `Diagnose(filePath, source string) []Diagnostic`. The Service handler delegates to it.

1. **Parse:** `syntax.Parse(filePath, source, 0)`. On error → return one `Diagnostic{code: "parse_error", severity: "error"}` with the parser's position. Stop — can't analyze a file that doesn't parse.
2. **Collect defined names** by walking the AST:
   - Top-level `def` statements → function name in module scope.
   - Top-level assignments (`x = …`, `x, y = …`) → names in module scope.
   - Function bodies: parameters (positional, keyword, vararg, kwarg) → function-scope names.
   - `for x in …:` / `for x, y in …:` → loop-scope names.
   - List/dict/set comprehension iteration vars → comprehension-scope names.
3. **Collect loaded names** by walking `load("path", name, alias="orig")`:
   - Track each load's target `path` string for the next step.
   - Track each imported name (or its alias) as module-scope.
4. **Validate load targets:** for each `load(path, …)`, check that `path` resolves to an existing `.star` file in the symbol index (which the Service already has — passed in via `Service.symbols` map's file paths). Failure → `Diagnostic{code: "load_not_found", severity: "error"}` at the load statement's path-literal position.
5. **Walk identifier references** (`syntax.Ident` nodes in load-positions-of-use, not in defining positions): for each reference, check whether the name is in the active scope chain ∪ loaded names ∪ Starlark universe (`True/False/None/len/print/dict/list/tuple/range/...`) ∪ Switchyard predeclared (e.g., `state`, `event`, `entity` — the runtime's `Predeclared()` set if exposed, else a hand-curated fallback list).
   - Unresolved → `Diagnostic{code: "unresolved_name", severity: "warning"}` at the identifier's position.

The walk is one O(N) pass over the AST. For typical files (<200 lines) this is sub-millisecond.

### Service wiring

`internal/starlarkls/service.go` gets a new method:

```go
func (s *Service) Diagnose(_ context.Context, req *connect.Request[starlarkpb.DiagnoseRequest]) (*connect.Response[starlarkpb.DiagnoseResponse], error) {
    diags := diagnose(req.Msg.FilePath, req.Msg.Source, s.symbols, predeclaredNames())
    out := make([]*starlarkpb.Diagnostic, len(diags))
    for i, d := range diags { out[i] = d.ToProto() }
    return connect.NewResponse(&starlarkpb.DiagnoseResponse{Diagnostics: out}), nil
}
```

Where `diagnose(...)` is the function in `diagnose.go` and `predeclaredNames()` returns a `map[string]bool` of allowed unresolved names. If the runtime's `Predeclared()` is reachable from this package, use that; otherwise hand-curated with a TODO to consolidate.

## Front-end: providers + diagnostics

### `app/src/data/starlark-ls.ts`

Five RPC wrappers mirroring `app/src/data/scenes.ts` etc:

```ts
const SVC = "switchyard.starlarkls.v1.StarlarkLsService";

export interface TokenSpan { startLine: number; startCol: number; endLine: number; endCol: number; tokenType: string; }
export interface CompletionItem { label: string; kind: string; detail: string; insertText: string; }
export interface Diagnostic { startLine: number; startCol: number; endLine: number; endCol: number; severity: "error" | "warning"; message: string; code: string; }

export async function tokenize(req: { filePath: string; source: string }): Promise<{ spans: TokenSpan[] }> { /* rpcCall */ }
export async function complete(req: { filePath: string; source: string; line: number; col: number }): Promise<{ items: CompletionItem[] }> { /* rpcCall */ }
export async function hover(req: { filePath: string; source: string; line: number; col: number }): Promise<{ markdown: string }> { /* rpcCall */ }
export async function lookupSymbol(req: { name: string }): Promise<{ filePath: string; line: number; kind: string; doc: string }> { /* rpcCall */ }
export async function diagnose(req: { filePath: string; source: string }): Promise<{ diagnostics: Diagnostic[] }> { /* rpcCall */ }
```

Snake/camel field-name interop follows the existing pattern from `data/entities.ts`.

### `app/src/lib/monaco/starlark.ts`

Single function `setupStarlark(monaco)`. Idempotent — registering twice is a no-op.

```ts
export function setupStarlark(monaco: typeof import("monaco-editor")): void {
  if (monaco.languages.getLanguages().some((l) => l.id === "starlark")) return;
  monaco.languages.register({
    id: "starlark",
    extensions: [".star"],
    aliases: ["Starlark", "starlark"],
  });
  monaco.languages.setMonarchTokensProvider("starlark", monarchGrammar);
  monaco.languages.setLanguageConfiguration("starlark", languageConfig);
}
```

`monarchGrammar` clones Python's grammar (Monaco ships it) and adds rules for:
- `load(…)` recognized as a special statement (`load` colored as keyword).
- Triple-quoted strings (already Python-shared).
- Hex/binary/octal numeric literals (already Python-shared).

`languageConfig` provides brackets, comment tokens (`#`), word pattern, auto-closing pairs (`()`/`[]`/`{}`/`""`/`''`).

### `app/src/lib/monaco/starlark-providers.ts`

`setupStarlarkProviders(monaco)` registers the four providers and arms diagnostics on every model with language `"starlark"`. Called once globally, alongside `setupStarlark`.

Each provider follows the shapes in section 2 (CompletionItemProvider, HoverProvider, DefinitionProvider, DocumentSemanticTokensProvider) — full code in the plan.

**Diagnostics wiring:**

```ts
function armDiagnostics(monaco, model) {
  const fire = debounce(async () => {
    try {
      const r = await diagnose({ filePath: model.uri.path, source: model.getValue() });
      monaco.editor.setModelMarkers(model, "starlark", r.diagnostics.map(diagToMonaco));
    } catch {
      monaco.editor.setModelMarkers(model, "starlark", []);
    }
  }, 300);
  model.onDidChangeContent(fire);
  fire();
}

// Listen for new models becoming starlark, arm them.
monaco.editor.onDidCreateModel((model) => {
  if (model.getLanguageId() === "starlark") armDiagnostics(monaco, model);
});
// Also handle models that already exist when setup runs.
for (const model of monaco.editor.getModels()) {
  if (model.getLanguageId() === "starlark") armDiagnostics(monaco, model);
}
```

### Go-to-definition cross-file affordance

The DefinitionProvider doesn't return a Monaco `Location` (which would require pre-registering target file models). Instead it dispatches a custom `window` event:

```ts
window.dispatchEvent(new CustomEvent("starlark-goto-definition", {
  detail: { filePath, line },
}));
```

`SyCodeEditorPanel.vue` listens for this event (registered in `onMounted`, removed in `onBeforeUnmount`) and calls its own `openFile(filePath)`, then positions the cursor at `line` via `editor.revealLineInCenter(line)` + `editor.setPosition({ lineNumber: line, column: 1 })`.

### `SyCodeEditorPanel.vue` changes

Two edits:

1. Line 42 `language` computed: `props.kind === "starlark" ? "starlark" : "pkl"`.
2. Add the event listener in `onMounted`/`onBeforeUnmount`.

## Error handling

| Failure | Behavior |
|---------|----------|
| Daemon down or returning error | All providers return empty; diagnostics clear their markers; editor stays functional with Monarch-only colors. No toast, no console spam. |
| File not in symbol index (new unsaved file) | Complete returns whatever the daemon found by parsing the current source; cross-file completions absent. Hover/LookupSymbol return empty for cross-file symbols. |
| Parse error in current file | Diagnostics returns the parse error; deeper analysis (unresolved names, load checks) is skipped for that frame. User fixes the parse error; subsequent frame catches the rest. |
| `load()` target file not found | One `load_not_found` error on the path-literal. Names imported by that load count as "loaded" anyway (best-effort) to suppress cascading `unresolved_name` warnings — otherwise a single typo'd load creates noise across every use site. |
| Symbol-extraction failure at daemon boot | Existing behavior: warn-log, empty symbol map. All RPCs return empty results. Editor still works. |
| User opens file before daemon boot completes | SyCodeEditorPanel already gates on `listFiles()`; if the file tree loads, the LSP service is up. |
| Two `.star` editors open simultaneously | Future tabs — each `model.uri.path` is distinct; providers correctly identify the requesting file. |
| Forward references | Name collection is two-pass (collect all top-level definitions first, then walk references); forward refs within a file resolve correctly. |
| Variable shadowing | Walker tracks active scopes; deepest wins. Standard lexical scope. |

## Testing

### Unit (Go) — `internal/starlarkls/diagnose_test.go`

Table-driven against literal source snippets:

- `def foo(): pass` → no diagnostics.
- `def foo(:` → one `parse_error` diagnostic.
- `load("missing.star", "x")\nprint(x)` → one `load_not_found` on the load; no `unresolved_name` on the `x` reference (best-effort suppression).
- `print(undefined_thing)` → one `unresolved_name` warning at the identifier.
- Variable shadowing case: `x = 1; def foo(x): print(x)` → no diagnostics (the parameter shadows the global).
- Forward reference: `def a(): b()\ndef b(): pass` → no diagnostics.
- Comprehension scope: `[y for y in range(10)]` → no diagnostics on `y`.
- Builtin reference: `print(len([1,2,3]))` → no diagnostics (universe).
- Switchyard predeclared: `state("light.x").value` → no diagnostics (predeclared).

### Unit (TS) — `app/src/lib/monaco/starlark-providers.test.ts`

Mock the five RPCs. For each provider:

- CompletionItemProvider returns `{ suggestions: [...] }` with kinds mapped correctly.
- HoverProvider returns `null` when daemon returns empty markdown; returns `{ contents, range }` when present.
- DefinitionProvider fires the `starlark-goto-definition` event with the right detail.
- SemanticTokensProvider's encoder converts `[]TokenSpan` to the LSP delta-uint32 wire format correctly.
- Diagnostics: `diagToMonaco` maps severity strings to `MarkerSeverity` enum values; line/col indices shift by +1 (Monaco is 1-based).

### Playwright (live)

After all wiring is in place, navigate to whatever route hosts the Starlark editor (likely `/settings/starlark`). Open an existing `.star` file from the file tree. Assert:

1. Within ~500ms the editor shows server-supplied semantic tokens (test by inspecting model decorations or by visual diff if needed).
2. Type a name with a typo (`prnt`) → after debounce, a red squiggly appears under it with the `unresolved_name` message.
3. Hover over a known function → tooltip with markdown.
4. Place cursor at a known function name + Ctrl+Space → completion dropdown lists items.
5. ⌘-click on a function call → editor switches to the function's defining file, cursor near the target line.

## Migration

None. Adding a new Monaco language + providers is purely additive. Existing `.star` files keep working; they just gain LSP features. Old behavior (Python-tokenized .star files) is replaced cleanly via the file-extension mapping on the new language.

## Open questions deferred to v2

- Workspace-wide diagnostics (a panel listing all current `.star` warnings/errors across all files).
- Delta-encoded semantic tokens (Monaco's `previousResultId` parameter would let the daemon return deltas; v1 returns full tokens every time).
- Type checking. go.starlark.net's AST has no type info; would need a separate type system.
- Lint-on-save in addition to lint-on-edit (the editor already does this implicitly via the model's change event firing on file load).
- Refactoring: rename across files, extract function, etc. The daemon would need broader semantic understanding.
