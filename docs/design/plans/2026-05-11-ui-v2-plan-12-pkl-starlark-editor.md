# Plan 12 — In-app Pkl + Starlark Editor (Monaco)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Monaco-backed in-app editor for Pkl config files and embedded Starlark expressions, including the `StarlarkLs` Connect-RPC language server, form-bound gutter markers, the AST breadcrumb, 3-way merge mode, and the full inspector panel.

**Architecture:** Monaco is loaded lazily behind a dynamic import so the base Switchyard bundle stays light. A new `StarlarkLs` Connect-RPC service runs alongside `switchyardd` and wraps the existing `internal/starlark` runtime to provide tokenisation, autocomplete, hover, and symbol lookup for `.star` files and embedded `starlark("…")` expressions in Pkl. The editor route lives at `/_authed/pkl-editor/*`, sharing the `AppRail` primitive already shipped by Plan 01. Form-bound markers bridge the gap between the source editor and Plan 11's `OpenForEdit` / `CommitEdit` lifecycle: the inspector shows which AST ranges are controlled by the WYSIWYG form editors, and the status-bar "Apply changes" button fires `CommitEdit`.

**Tech Stack:** Monaco Editor (via `monaco-editor` npm package, dynamic import), Go `go.starlark.net/starlark` (already present), Connect-RPC / protobuf (buf), React 18, TanStack Router, Vitest, Playwright.

**Spec refs:** §18 (full editor design), §17.3 (3-way merge / conflict resolution), §17.1 (OpenForEdit / CommitEdit lifecycle).

**Branch:** `feat/ui-v2-plan-12-pkl-starlark-editor`
**Worktree:** `.claude/worktrees/plan-12-pkl-starlark-editor`
**Depends on:** Plan 01 + Plan 11 merged to main
**Linear parent:** TBD

**Mockups:**
- `.superpowers/brainstorm/71337-1778492716/screenshots/13-pkl-starlark-editor-01.png` — full editor layout
- `.superpowers/brainstorm/71337-1778492716/screenshots/13-pkl-starlark-editor-02.png` — Starlark embedded language zoom

---

## Decisions (locked — no ambiguity for the implementer)

1. **Monaco is loaded lazily** via `import('monaco-editor')` inside a React `Suspense` boundary. The static bundle must not import `monaco-editor` at the top level. Verify with `task web:build` + `--bundle-analyzer` that the main chunk does not include it.
2. **Editor route:** `/_authed/pkl-editor/*` where `*` is the file path within `~/.switchyard/`. Example: `/_authed/pkl-editor/automations/sunset-lights.pkl`. The merge variant is `/_authed/pkl-editor/merge/*?session=<id>`.
3. **Layout (per §18.1):** `56px AppRail` (already from Plan 01) | `248px file tree` | `flex editor pane` | `320px inspector`. The AppRail is rendered only on the Pkl editor route (it is hidden in the default shell per Plan 01 decision 9).
4. **File tree** shows the Pkl config root directories (`automations`, `dashboards`, `displays`, `drivers`, `scripts`, `base`). Files display dirty (orange dot) and error (red badge) indicators. A search affordance at the top of the tree opens a `⌘P`-style fuzzy search (reuses Plan 05's `CommandPalette` overlay with a scoped file filter).
5. **Editor pane tabs:** each open file is a tab with a dirty-dot indicator. Closing a dirty tab prompts "Discard changes?". Tabs use the browser's `history.pushState` so the URL reflects the active file.
6. **AST breadcrumb:** computed server-side. `ConfigService.GetAstPath(file_path, line, col)` returns the structural path (e.g., `automations / sunset-lights.pkl › actions [2] . brightness`). Called debounced (150 ms) on every cursor move.
7. **Pkl language definition:** custom Monaco tokens definition in `web/src/pkl-editor/languages/pkl.ts`. Inherit token patterns from the open-source `vscode-pkl` extension (Apache-2.0; cite: `github.com/apple/pkl-vscode`). The implementer must check that file's `syntaxes/pkl.tmLanguage.json` and manually port the relevant token rules to the Monaco `IMonarchTokensProvider` format — do not load the TextMate grammar directly (Monaco's TextMate support requires a worker setup that complicates the lazy-load story).
8. **Embedded Starlark detection:** when the cursor enters a `starlark("…")` call, `embedded.ts` detects this by walking the Monaco model line-by-line and marking ranges via a simple regex (`/starlark\s*\(\s*r?"""/` or single-quoted variant). Inside those ranges, a secondary Monaco model (the "embedded model") is created for the Starlark content, and Monaco's `setTokensForLine` override is used to apply Starlark tokens. This is the simplest approach; a full injected language grammar is out of scope for this plan.
9. **`StarlarkLs` proto** lives at `proto/switchyard/starlarkls/v1/starlarkls.proto`. RPCs: `Tokenize(file_path, source)` → token spans; `Complete(file_path, source, line, col)` → completion items; `Hover(file_path, source, line, col)` → hover text; `LookupSymbol(name)` → definition location. All RPCs are unary (no streaming needed for the LS).
10. **Symbol extraction** (`internal/starlarkls/symbols.go`) reads `~/.switchyard/scripts/*.star` at server start and on file change, parses each file using `go.starlark.net/starlark` (`syntax.ParseFile`), and builds a flat `map[name]SymbolInfo`. `SymbolInfo` holds: `file`, `line`, `kind` (`function | global`), `doc string` (from the leading string literal in a `def` body). This map is injected into the `StarlarkLs` service.
11. **Form-bound markers:** the inspector shows which line ranges in the open file are "form-bound" (controlled by the Plan 11 WYSIWYG form editor). `ConfigService.GetFormBoundRegions(file_path)` (added in Plan 11) returns `[]FormBoundRegion{start_line, end_line, form_editor_id, label}`. Plan 12 renders a purple gutter decoration and a tinted background over those ranges. Clicking a gutter decoration navigates to the form editor via `router.navigate({ to: '/automations/$slug', params: { slug } })`.
12. **Status bar:** displays `Pkl <version> · <N> unsaved · <E> errors · <F> form-bound · Ln <l>, Col <c> · spaces:2 · UTF-8 · LF`. Three action buttons: **Format** (calls `ConfigService.FormatFile(file_path, content)` → returns formatted string; replaces model content); **Validate** (calls `ConfigService.ValidateFile(file_path, content)` → returns `[]Diagnostic`; decorates the model); **Apply changes** (⌘S — calls Plan 11's `CommitEdit`; triggers the conflict banner if `CONFLICT` is returned).
13. **3-way merge route** at `/_authed/pkl-editor/merge/*?session=<id>`: three Monaco instances side-by-side — left ("On disk now"), center ("Common ancestor"), right ("Your changes"). Gutter buttons let the user pick the left or right version hunk-by-hunk. "Save merged result" calls `CommitEdit` with `force=true` and the merged content.
14. **No Pkl language server** is wired in this plan. The Pkl LS integration is behind a `SWITCHYARD_PKL_LS_ENABLED=true` feature flag (reads from `window.__SY_FLAGS__`) and is a no-op unless set. The custom token definition (decision 7) is always active.

---

## File plan

### Created (Go — server)

```
proto/switchyard/starlarkls/v1/
  starlarkls.proto              ← StarlarkLsService: Tokenize, Complete, Hover, LookupSymbol
  starlarkls.pb.go              ← buf-generated (do not edit)
  starlarkls_grpc.pb.go         ← buf-generated

internal/starlarkls/
  service.go                    ← StarlarkLsService Connect-RPC implementation
  symbols.go                    ← file-watcher + syntax.ParseFile → SymbolInfo map
  service_test.go               ← unit tests: Complete, Hover, LookupSymbol
  symbols_test.go               ← SymbolInfo extraction from fixture .star files
```

### Created (web)

```
web/src/pkl-editor/
  route.tsx                     ← TanStack route: /_authed/pkl-editor/* (file picker → splits view)
  merge-route.tsx               ← TanStack route: /_authed/pkl-editor/merge/*?session=<id>
  Monaco.tsx                    ← lazy-loaded Monaco wrapper (Suspense boundary, dynamic import)
  languages/
    pkl.ts                      ← Monaco IMonarchTokensProvider for Pkl
    starlark.ts                 ← Monaco IMonarchTokensProvider for Starlark
  embedded.ts                   ← detects starlark("…") regions in a Monaco model; returns RangeMap
  FileTree.tsx                  ← 248px file tree; dirty/error badges; ⌘P affordance
  Inspector.tsx                 ← 320px inspector: type info, problems, form-bound regions, embedded info
  AstBreadcrumb.tsx             ← breadcrumb bar below tab strip
  StatusBar.tsx                 ← bottom bar with metrics + Format / Validate / Apply buttons
  form-bound-decorations.ts     ← registers purple gutter decorations + tinted background ranges
  merge-view.tsx                ← 3-pane layout + hunk pick buttons + save merged result
  index.ts                      ← barrel export

web/src/data/
  starlarkls-client.ts          ← typed Connect-RPC client for StarlarkLsService
```

### Modified

```
web/src/routes/_authed/            ← add pkl-editor.$.tsx + pkl-editor.merge.$.tsx as route files
web/src/shell/AppRail.tsx          ← render AppRail when pathname starts with /_authed/pkl-editor
web/src/data/client.ts             ← register starlarkls transport alongside existing clients
cmd/switchyardd/main.go            ← wire StarlarkLsService into the Connect-RPC mux
```

---

## Tasks

### Task 12.1 — `starlarkls.proto` + buf generate

**Files:**
- Create: `proto/switchyard/starlarkls/v1/starlarkls.proto`
- Run: `buf generate` in repo root

**Steps:**

- [ ] **Step 1: Author the proto**

```protobuf
// proto/switchyard/starlarkls/v1/starlarkls.proto
syntax = "proto3";
package switchyard.starlarkls.v1;
option go_package = "github.com/fdatoo/switchyard/proto/switchyard/starlarkls/v1;starlarkls";

service StarlarkLsService {
  rpc Tokenize(TokenizeRequest) returns (TokenizeResponse);
  rpc Complete(CompleteRequest)  returns (CompleteResponse);
  rpc Hover(HoverRequest)        returns (HoverResponse);
  rpc LookupSymbol(LookupSymbolRequest) returns (LookupSymbolResponse);
}

message TokenizeRequest {
  string file_path = 1;
  string source     = 2;
}
message TokenSpan {
  int32  start_line = 1;
  int32  start_col  = 2;
  int32  end_line   = 3;
  int32  end_col    = 4;
  string token_type = 5; // "keyword" | "identifier" | "string" | "number" | "comment" | "operator"
}
message TokenizeResponse {
  repeated TokenSpan spans = 1;
}

message CompleteRequest {
  string file_path = 1;
  string source     = 2;
  int32  line       = 3;
  int32  col        = 4;
}
message CompletionItem {
  string label        = 1;
  string kind         = 2; // "function" | "variable" | "keyword"
  string detail       = 3;
  string insert_text  = 4;
}
message CompleteResponse {
  repeated CompletionItem items = 1;
}

message HoverRequest {
  string file_path = 1;
  string source     = 2;
  int32  line       = 3;
  int32  col        = 4;
}
message HoverResponse {
  string markdown = 1; // empty = no hover
}

message LookupSymbolRequest {
  string name = 1;
}
message LookupSymbolResponse {
  string file_path = 1;
  int32  line      = 2;
  string kind      = 3; // "function" | "global"
  string doc       = 4;
}
```

- [ ] **Step 2: Run buf generate and verify**

```bash
buf generate
go build ./...
```

Expected: no errors; `proto/switchyard/starlarkls/v1/starlarkls.pb.go` and `starlarkls_grpc.pb.go` are created.

- [ ] **Step 3: Commit**

```bash
git add proto/switchyard/starlarkls/
git commit -m "feat(proto): StarlarkLsService (UI v2 plan 12)"
```

---

### Task 12.2 — Symbol extractor + tests

**Files:**
- Create: `internal/starlarkls/symbols.go`
- Create: `internal/starlarkls/symbols_test.go`

**Steps:**

- [ ] **Step 1: Write the failing tests**

```go
// internal/starlarkls/symbols_test.go
package starlarkls_test

import (
    "os"
    "path/filepath"
    "testing"
    "github.com/fdatoo/switchyard/internal/starlarkls"
)

func TestExtractSymbols(t *testing.T) {
    dir := t.TempDir()
    src := `
def compute_brightness(sun, now):
    """Returns brightness 0-100 based on sun altitude."""
    return int(sun.altitude * 100)

THRESHOLD = 50
`
    err := os.WriteFile(filepath.Join(dir, "util.star"), []byte(src), 0644)
    if err != nil { t.Fatal(err) }

    syms, err := starlarkls.ExtractSymbols(dir)
    if err != nil { t.Fatal(err) }

    fn, ok := syms["compute_brightness"]
    if !ok { t.Fatal("missing compute_brightness") }
    if fn.Kind != "function" { t.Errorf("got kind %q, want function", fn.Kind) }
    if fn.Doc == "" { t.Error("expected non-empty doc") }

    g, ok := syms["THRESHOLD"]
    if !ok { t.Fatal("missing THRESHOLD") }
    if g.Kind != "global" { t.Errorf("got kind %q, want global", g.Kind) }
}

func TestExtractSymbols_SyntaxError(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "bad.star"), []byte("def ("), 0644)
    // ExtractSymbols skips files with parse errors and logs them; it must not return an error itself.
    syms, err := starlarkls.ExtractSymbols(dir)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if len(syms) != 0 { t.Errorf("expected empty map, got %v", syms) }
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/starlarkls/... 2>&1 | head -5
```
Expected: `cannot find package` or `undefined: starlarkls.ExtractSymbols`.

- [ ] **Step 3: Implement `symbols.go`**

```go
// internal/starlarkls/symbols.go
package starlarkls

import (
    "log/slog"
    "path/filepath"
    "strings"

    "go.starlark.net/syntax"
)

// SymbolInfo describes a top-level symbol extracted from a .star file.
type SymbolInfo struct {
    File string
    Line int32
    Kind string // "function" | "global"
    Doc  string
}

// ExtractSymbols parses all *.star files in dir and returns a flat
// map[symbol_name]SymbolInfo. Files with parse errors are skipped with a log.
func ExtractSymbols(dir string) (map[string]SymbolInfo, error) {
    entries, err := filepath.Glob(filepath.Join(dir, "*.star"))
    if err != nil {
        return nil, err
    }
    out := make(map[string]SymbolInfo, len(entries)*8)
    for _, path := range entries {
        f, err := syntax.ParseFile(path, nil, syntax.RetainComments)
        if err != nil {
            slog.Warn("starlarkls: skipping file with parse error", "path", path, "err", err)
            continue
        }
        for _, stmt := range f.Stmts {
            switch s := stmt.(type) {
            case *syntax.DefStmt:
                doc := extractDoc(s)
                out[s.Name.Name] = SymbolInfo{
                    File: path,
                    Line: int32(s.Name.NamePos.Line),
                    Kind: "function",
                    Doc:  doc,
                }
            case *syntax.AssignStmt:
                if id, ok := s.LHS.(*syntax.Ident); ok && strings.ToUpper(id.Name) == id.Name {
                    out[id.Name] = SymbolInfo{
                        File: path,
                        Line: int32(id.NamePos.Line),
                        Kind: "global",
                    }
                }
            }
        }
    }
    return out, nil
}

func extractDoc(def *syntax.DefStmt) string {
    if len(def.Body) == 0 {
        return ""
    }
    expr, ok := def.Body[0].(*syntax.ExprStmt)
    if !ok {
        return ""
    }
    lit, ok := expr.X.(*syntax.Literal)
    if !ok || lit.Token != syntax.STRING {
        return ""
    }
    return strings.Trim(lit.Raw, `"'`)
}
```

- [ ] **Step 4: Run tests and verify they pass**

```bash
go test ./internal/starlarkls/... -v -run TestExtract
```
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/starlarkls/symbols.go internal/starlarkls/symbols_test.go
git commit -m "feat(server): starlarkls symbol extractor (UI v2 plan 12)"
```

---

### Task 12.3 — `StarlarkLsService` implementation + tests

**Files:**
- Create: `internal/starlarkls/service.go`
- Create: `internal/starlarkls/service_test.go`

**Steps:**

- [ ] **Step 1: Write the failing tests**

```go
// internal/starlarkls/service_test.go
package starlarkls_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    starlarkpb "github.com/fdatoo/switchyard/proto/switchyard/starlarkls/v1"
    "github.com/fdatoo/switchyard/internal/starlarkls"
)

func setupService(t *testing.T) (starlarkpb.StarlarkLsServiceHandler, string) {
    t.Helper()
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "util.star"), []byte(`
def compute_brightness(sun, now):
    """Returns brightness 0-100."""
    return int(sun.altitude * 100)
`), 0644)
    syms, err := starlarkls.ExtractSymbols(dir)
    if err != nil { t.Fatal(err) }
    return starlarkls.NewService(syms, dir), dir
}

func TestComplete_GlobalSymbol(t *testing.T) {
    svc, dir := setupService(t)
    resp, err := svc.Complete(context.Background(), connect.NewRequest(&starlarkpb.CompleteRequest{
        FilePath: filepath.Join(dir, "util.star"),
        Source:   "compute_b",
        Line:     1,
        Col:      9,
    }))
    if err != nil { t.Fatal(err) }
    found := false
    for _, item := range resp.Msg.Items {
        if item.Label == "compute_brightness" { found = true }
    }
    if !found { t.Error("expected compute_brightness in completion items") }
}

func TestHover_KnownSymbol(t *testing.T) {
    svc, dir := setupService(t)
    resp, err := svc.Hover(context.Background(), connect.NewRequest(&starlarkpb.HoverRequest{
        FilePath: filepath.Join(dir, "util.star"),
        Source:   "compute_brightness(sun, now)",
        Line:     1,
        Col:      5,
    }))
    if err != nil { t.Fatal(err) }
    if resp.Msg.Markdown == "" { t.Error("expected non-empty hover markdown") }
}

func TestLookupSymbol_Found(t *testing.T) {
    svc, _ := setupService(t)
    resp, err := svc.LookupSymbol(context.Background(), connect.NewRequest(&starlarkpb.LookupSymbolRequest{
        Name: "compute_brightness",
    }))
    if err != nil { t.Fatal(err) }
    if resp.Msg.Kind != "function" { t.Errorf("want function, got %q", resp.Msg.Kind) }
}

func TestLookupSymbol_NotFound(t *testing.T) {
    svc, _ := setupService(t)
    _, err := svc.LookupSymbol(context.Background(), connect.NewRequest(&starlarkpb.LookupSymbolRequest{
        Name: "nonexistent",
    }))
    if err == nil { t.Error("expected connect NOT_FOUND error") }
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/starlarkls/... 2>&1 | head -5
```
Expected: compile errors (service not implemented yet).

- [ ] **Step 3: Implement `service.go`**

```go
// internal/starlarkls/service.go
package starlarkls

import (
    "context"
    "strings"

    "connectrpc.com/connect"
    starlarkpb "github.com/fdatoo/switchyard/proto/switchyard/starlarkls/v1"
    starlarkpbconnect "github.com/fdatoo/switchyard/proto/switchyard/starlarkls/v1/starlarkls/v1connect"
)

var _ starlarkpbconnect.StarlarkLsServiceHandler = (*Service)(nil)

type Service struct {
    symbols map[string]SymbolInfo
    dir     string
}

func NewService(symbols map[string]SymbolInfo, scriptsDir string) *Service {
    return &Service{symbols: symbols, dir: scriptsDir}
}

// Tokenize returns token spans by doing a lightweight scan of the source.
// For this plan, we return keyword tokens only; a full tokeniser is a
// future enhancement.
func (s *Service) Tokenize(_ context.Context, req *connect.Request[starlarkpb.TokenizeRequest]) (*connect.Response[starlarkpb.TokenizeResponse], error) {
    keywords := []string{"def", "if", "else", "elif", "for", "in", "return", "and", "or", "not", "True", "False", "None", "load", "lambda", "pass", "break", "continue"}
    var spans []*starlarkpb.TokenSpan
    for lineIdx, line := range strings.Split(req.Msg.Source, "\n") {
        for _, kw := range keywords {
            col := strings.Index(line, kw)
            if col < 0 { continue }
            spans = append(spans, &starlarkpb.TokenSpan{
                StartLine: int32(lineIdx + 1),
                StartCol:  int32(col),
                EndLine:   int32(lineIdx + 1),
                EndCol:    int32(col + len(kw)),
                TokenType: "keyword",
            })
        }
    }
    return connect.NewResponse(&starlarkpb.TokenizeResponse{Spans: spans}), nil
}

func (s *Service) Complete(_ context.Context, req *connect.Request[starlarkpb.CompleteRequest]) (*connect.Response[starlarkpb.CompleteResponse], error) {
    // Extract the partial token before the cursor.
    lines := strings.Split(req.Msg.Source, "\n")
    col := int(req.Msg.Col)
    if int(req.Msg.Line)-1 >= len(lines) {
        return connect.NewResponse(&starlarkpb.CompleteResponse{}), nil
    }
    line := lines[req.Msg.Line-1]
    if col > len(line) { col = len(line) }
    prefix := wordBefore(line[:col])

    var items []*starlarkpb.CompletionItem
    for name, sym := range s.symbols {
        if strings.HasPrefix(name, prefix) {
            items = append(items, &starlarkpb.CompletionItem{
                Label:       name,
                Kind:        sym.Kind,
                Detail:      sym.Doc,
                InsertText:  name,
            })
        }
    }
    return connect.NewResponse(&starlarkpb.CompleteResponse{Items: items}), nil
}

func (s *Service) Hover(_ context.Context, req *connect.Request[starlarkpb.HoverRequest]) (*connect.Response[starlarkpb.HoverResponse], error) {
    lines := strings.Split(req.Msg.Source, "\n")
    if int(req.Msg.Line)-1 >= len(lines) {
        return connect.NewResponse(&starlarkpb.HoverResponse{}), nil
    }
    word := wordAt(lines[req.Msg.Line-1], int(req.Msg.Col))
    sym, ok := s.symbols[word]
    if !ok {
        return connect.NewResponse(&starlarkpb.HoverResponse{}), nil
    }
    md := "**" + sym.Kind + "** `" + word + "`"
    if sym.Doc != "" { md += "\n\n" + sym.Doc }
    return connect.NewResponse(&starlarkpb.HoverResponse{Markdown: md}), nil
}

func (s *Service) LookupSymbol(_ context.Context, req *connect.Request[starlarkpb.LookupSymbolRequest]) (*connect.Response[starlarkpb.LookupSymbolResponse], error) {
    sym, ok := s.symbols[req.Msg.Name]
    if !ok {
        return nil, connect.NewError(connect.CodeNotFound, nil)
    }
    return connect.NewResponse(&starlarkpb.LookupSymbolResponse{
        FilePath: sym.File,
        Line:     sym.Line,
        Kind:     sym.Kind,
        Doc:      sym.Doc,
    }), nil
}

// wordBefore returns the identifier-like token ending at s.
func wordBefore(s string) string {
    i := len(s)
    for i > 0 && isIdent(s[i-1]) { i-- }
    return s[i:]
}

// wordAt returns the identifier-like token around column col (0-based).
func wordAt(line string, col int) string {
    if col > len(line) { col = len(line) }
    start := col
    for start > 0 && isIdent(line[start-1]) { start-- }
    end := col
    for end < len(line) && isIdent(line[end]) { end++ }
    return line[start:end]
}

func isIdent(b byte) bool {
    return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/starlarkls/... -v
```
Expected: all tests pass.

- [ ] **Step 5: Wire service into the daemon**

In `cmd/switchyardd/main.go`, add:
```go
// After existing service wires:
starSyms, _ := starlarkls.ExtractSymbols(filepath.Join(cfg.ConfigDir, "scripts"))
starLsSvc := starlarkls.NewService(starSyms, filepath.Join(cfg.ConfigDir, "scripts"))
mux.Handle(starlarkpbconnect.NewStarlarkLsServiceHandler(starLsSvc))
```

Run `go build ./cmd/switchyardd` to verify it compiles.

- [ ] **Step 6: Commit**

```bash
git add internal/starlarkls/ cmd/switchyardd/main.go proto/switchyard/starlarkls/
git commit -m "feat(server): StarlarkLsService + wire into daemon (UI v2 plan 12)"
```

---

### Task 12.4 — Lazy Monaco wrapper

**Files:**
- Create: `web/src/pkl-editor/Monaco.tsx`

**Steps:**

- [ ] **Step 1: Install monaco-editor**

```bash
cd web && npm install monaco-editor
```

Verify `monaco-editor` appears in `web/package.json` dependencies (not devDependencies).

- [ ] **Step 2: Write the failing test**

```ts
// web/src/pkl-editor/Monaco.test.tsx
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import Monaco from "./Monaco";

test("renders loading fallback before Monaco resolves", () => {
  render(
    <Suspense fallback={<div data-testid="loading">Loading editor…</div>}>
      <Monaco language="pkl" value="" onChange={() => {}} />
    </Suspense>
  );
  expect(screen.getByTestId("loading")).toBeInTheDocument();
});
```

- [ ] **Step 3: Run to verify failure**

```bash
cd web && npx vitest run src/pkl-editor/Monaco.test.tsx 2>&1 | tail -5
```
Expected: `Cannot find module './Monaco'`.

- [ ] **Step 4: Implement `Monaco.tsx`**

```tsx
// web/src/pkl-editor/Monaco.tsx
import { lazy, Suspense, useEffect, useRef } from "react";

// Dynamic import — Monaco must never appear in the base chunk.
const MonacoEditor = lazy(() =>
  import("monaco-editor").then((m) => ({
    default: function MonacoEditorInner({
      language,
      value,
      onChange,
      options,
      onEditorMount,
    }: MonacoProps) {
      const containerRef = useRef<HTMLDivElement>(null);
      const editorRef = useRef<m.editor.IStandaloneCodeEditor | null>(null);
      useEffect(() => {
        if (!containerRef.current) return;
        const editor = m.editor.create(containerRef.current, {
          value,
          language,
          theme: "vs-dark",
          automaticLayout: true,
          fontSize: 13,
          lineNumbers: "on",
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          ...options,
        });
        editorRef.current = editor;
        onEditorMount?.(editor, m);
        const sub = editor.onDidChangeModelContent(() => {
          onChange?.(editor.getValue());
        });
        return () => { sub.dispose(); editor.dispose(); };
      // eslint-disable-next-line react-hooks/exhaustive-deps
      }, []);
      return <div ref={containerRef} style={{ width: "100%", height: "100%" }} />;
    },
  }))
);

export interface MonacoProps {
  language: "pkl" | "starlark" | "plaintext";
  value: string;
  onChange?: (value: string) => void;
  options?: Record<string, unknown>;
  onEditorMount?: (editor: unknown, monaco: unknown) => void;
}

export default function Monaco(props: MonacoProps) {
  return (
    <Suspense fallback={<div className="editor-loading" data-testid="editor-loading">Loading editor…</div>}>
      <MonacoEditor {...props} />
    </Suspense>
  );
}
```

- [ ] **Step 5: Verify the base bundle does not contain Monaco**

```bash
cd web && npm run build -- --mode production
# Check that the main chunk does not contain "monaco-editor"
grep -r "monaco-editor" dist/assets/index-*.js | wc -l
```
Expected: `0` (Monaco lives in a separate async chunk).

- [ ] **Step 6: Commit**

```bash
git add web/src/pkl-editor/Monaco.tsx web/src/pkl-editor/Monaco.test.tsx web/package.json web/package-lock.json
git commit -m "feat(web): lazy Monaco wrapper (UI v2 plan 12)"
```

---

### Task 12.5 — Pkl language definition

**Files:**
- Create: `web/src/pkl-editor/languages/pkl.ts`

**Steps:**

- [ ] **Step 1: Write the test**

```ts
// web/src/pkl-editor/languages/pkl.test.ts
import { pklLanguageDefinition } from "./pkl";

test("pklLanguageDefinition has required fields", () => {
  expect(pklLanguageDefinition.tokenizer).toBeDefined();
  expect(pklLanguageDefinition.keywords).toContain("amends");
  expect(pklLanguageDefinition.keywords).toContain("module");
  expect(pklLanguageDefinition.keywords).toContain("class");
  expect(pklLanguageDefinition.keywords).toContain("extends");
  expect(pklLanguageDefinition.keywords).toContain("import");
  expect(pklLanguageDefinition.keywords).toContain("function");
  expect(pklLanguageDefinition.keywords).toContain("let");
  expect(pklLanguageDefinition.keywords).toContain("when");
  expect(pklLanguageDefinition.keywords).toContain("is");
  expect(pklLanguageDefinition.keywords).toContain("as");
  expect(pklLanguageDefinition.keywords).toContain("new");
  expect(pklLanguageDefinition.keywords).toContain("this");
  expect(pklLanguageDefinition.keywords).toContain("outer");
  expect(pklLanguageDefinition.keywords).toContain("super");
  expect(pklLanguageDefinition.keywords).toContain("null");
  expect(pklLanguageDefinition.keywords).toContain("true");
  expect(pklLanguageDefinition.keywords).toContain("false");
});
```

- [ ] **Step 2: Run to verify failure**

```bash
cd web && npx vitest run src/pkl-editor/languages/pkl.test.ts 2>&1 | tail -5
```

- [ ] **Step 3: Implement `pkl.ts`**

Reference: `github.com/apple/pkl-vscode` `syntaxes/pkl.tmLanguage.json` (Apache-2.0). Port token rules to Monaco `IMonarchTokensProvider` format.

```ts
// web/src/pkl-editor/languages/pkl.ts
// Ported from apple/pkl-vscode (Apache-2.0): https://github.com/apple/pkl-vscode/blob/main/syntaxes/pkl.tmLanguage.json
import type { languages } from "monaco-editor";

export const PKL_LANGUAGE_ID = "pkl";

export const pklLanguageDefinition: languages.IMonarchLanguage = {
  defaultToken: "",
  tokenPostfix: ".pkl",
  keywords: [
    "amends", "module", "class", "typealias", "extends", "import", "import*",
    "function", "local", "hidden", "fixed", "const", "abstract", "open",
    "external", "let", "when", "is", "as", "new", "this", "outer", "super",
    "null", "true", "false", "nothing", "unknown", "if", "else", "for", "in",
    "read", "read?", "read*", "throw", "trace", "import@",
  ],
  builtins: ["String", "Int", "Float", "Boolean", "Duration", "DataSize",
    "Pair", "List", "Set", "Map", "Listing", "Mapping", "Dynamic", "Any",
    "Number", "Regex", "Resource"],
  operators: ["=", "!=", "==", "<", ">", "<=", ">=", "&&", "||", "!", "?",
    "??", "|>", "->", "?.", "...", "@"],
  tokenizer: {
    root: [
      [/\/\/\/.*$/, "comment.doc"],
      [/\/\/.*$/, "comment.line"],
      [/\/\*/, "comment.block", "@comment"],
      [/"/, "string", "@string_double"],
      [/#"/, "string", "@string_multiline"],
      [/\b(true|false|null|nothing)\b/, "constant.language"],
      [/\b\d+(\.\d+)?\b/, "number"],
      [/\b[A-Z][a-zA-Z0-9_]*\b/, { cases: { "@builtins": "type.builtin", "@default": "type" } }],
      [/\b[a-z_$][a-zA-Z0-9_$]*\b/, { cases: { "@keywords": "keyword", "@default": "identifier" } }],
      [/[{}[\]()]/, "@brackets"],
      [/[=!<>|&?]+/, "operator"],
    ],
    comment: [
      [/[^/*]+/, "comment.block"],
      [/\*\//, "comment.block", "@pop"],
      [/[/*]/, "comment.block"],
    ],
    string_double: [
      [/[^"\\]+/, "string"],
      [/\\\\./, "string.escape"],
      [/"/, "string", "@pop"],
    ],
    string_multiline: [
      [/[^"#]+/, "string"],
      [/"(?!##)/, "string"],
      [/#"/, "string", "@pop"],
    ],
  },
};

export function registerPklLanguage(monaco: typeof import("monaco-editor")) {
  if (monaco.languages.getLanguages().some((l) => l.id === PKL_LANGUAGE_ID)) return;
  monaco.languages.register({ id: PKL_LANGUAGE_ID, extensions: [".pkl"] });
  monaco.languages.setMonarchTokensProvider(PKL_LANGUAGE_ID, pklLanguageDefinition);
  monaco.languages.setLanguageConfiguration(PKL_LANGUAGE_ID, {
    comments: { lineComment: "//", blockComment: ["/*", "*/"] },
    brackets: [["{", "}"], ["[", "]"], ["(", ")"]],
    autoClosingPairs: [
      { open: "{", close: "}" }, { open: "[", close: "]" },
      { open: "(", close: ")" }, { open: '"', close: '"' },
    ],
    indentationRules: { increaseIndentPattern: /^.*\{[^}]*$/, decreaseIndentPattern: /^[^{]*\}/ },
  });
}
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/languages/pkl.test.ts
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/pkl-editor/languages/pkl.ts web/src/pkl-editor/languages/pkl.test.ts
git commit -m "feat(web): Pkl Monaco language definition (UI v2 plan 12)"
```

---

### Task 12.6 — Starlark language definition

**Files:**
- Create: `web/src/pkl-editor/languages/starlark.ts`

**Steps:**

- [ ] **Step 1: Write the test**

```ts
// web/src/pkl-editor/languages/starlark.test.ts
import { starlarkLanguageDefinition } from "./starlark";

test("starlark tokenizer has keywords and builtins", () => {
  expect(starlarkLanguageDefinition.keywords).toContain("def");
  expect(starlarkLanguageDefinition.keywords).toContain("return");
  expect(starlarkLanguageDefinition.keywords).toContain("load");
  expect(starlarkLanguageDefinition.builtins).toContain("len");
  expect(starlarkLanguageDefinition.builtins).toContain("range");
  expect(starlarkLanguageDefinition.builtins).toContain("print");
  expect(starlarkLanguageDefinition.builtins).toContain("int");
  expect(starlarkLanguageDefinition.builtins).toContain("str");
});
```

- [ ] **Step 2: Implement `starlark.ts`**

```ts
// web/src/pkl-editor/languages/starlark.ts
import type { languages } from "monaco-editor";

export const STARLARK_LANGUAGE_ID = "starlark";

export const starlarkLanguageDefinition: languages.IMonarchLanguage & {
  keywords: string[]; builtins: string[];
} = {
  defaultToken: "",
  tokenPostfix: ".star",
  keywords: ["and", "break", "continue", "def", "elif", "else", "for", "if",
    "in", "lambda", "load", "not", "or", "pass", "return", "True", "False", "None"],
  builtins: ["abs", "all", "any", "bool", "dict", "dir", "enumerate", "fail",
    "float", "getattr", "hasattr", "hash", "int", "len", "list", "max", "min",
    "print", "range", "repr", "reversed", "set", "sorted", "str", "tuple",
    "type", "zip"],
  tokenizer: {
    root: [
      [/#.*$/, "comment.line"],
      [/"(?:[^"\\]|\\.)*"/, "string"],
      [/'(?:[^'\\]|\\.)*'/, "string"],
      [/"""[\s\S]*?"""/, "string.multiline"],
      [/'''[\s\S]*?'''/, "string.multiline"],
      [/\b\d+(\.\d+)?\b/, "number"],
      [/\b[A-Z_][A-Z0-9_]*\b/, "variable.constant"],
      [/\b[a-z_][a-zA-Z0-9_]*\b/, { cases: {
        "@keywords": "keyword",
        "@builtins": "support.function",
        "@default": "identifier",
      }}],
      [/[{}[\]()]/, "@brackets"],
      [/[=!<>+\-*/%&|^~]+/, "operator"],
    ],
  },
};

export function registerStarlarkLanguage(monaco: typeof import("monaco-editor")) {
  if (monaco.languages.getLanguages().some((l) => l.id === STARLARK_LANGUAGE_ID)) return;
  monaco.languages.register({ id: STARLARK_LANGUAGE_ID, extensions: [".star"] });
  monaco.languages.setMonarchTokensProvider(STARLARK_LANGUAGE_ID, starlarkLanguageDefinition);
  monaco.languages.setLanguageConfiguration(STARLARK_LANGUAGE_ID, {
    comments: { lineComment: "#" },
    brackets: [["{", "}"], ["[", "]"], ["(", ")"]],
    autoClosingPairs: [
      { open: "{", close: "}" }, { open: "[", close: "]" },
      { open: "(", close: ")" }, { open: '"', close: '"' }, { open: "'", close: "'" },
    ],
  });
}
```

- [ ] **Step 3: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/languages/starlark.test.ts
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/pkl-editor/languages/starlark.ts web/src/pkl-editor/languages/starlark.test.ts
git commit -m "feat(web): Starlark Monaco language definition (UI v2 plan 12)"
```

---

### Task 12.7 — Embedded Starlark detection

**Files:**
- Create: `web/src/pkl-editor/embedded.ts`

**Steps:**

- [ ] **Step 1: Write the failing test**

```ts
// web/src/pkl-editor/embedded.test.ts
import { findStarlarkRegions } from "./embedded";

const PKL_SOURCE = `
module foo

brightness = starlark("""
def compute(sun, now):
    return int(sun.altitude * 100)
compute(sun, now)
""")

name = "hello"
`.trim();

test("detects starlark region line range", () => {
  const regions = findStarlarkRegions(PKL_SOURCE);
  expect(regions).toHaveLength(1);
  // The region starts after the opening triple-quote and ends before the closing one.
  expect(regions[0].startLine).toBeGreaterThan(0);
  expect(regions[0].endLine).toBeGreaterThan(regions[0].startLine);
});

test("returns empty for source with no starlark call", () => {
  expect(findStarlarkRegions("name = 42")).toHaveLength(0);
});
```

- [ ] **Step 2: Implement `embedded.ts`**

```ts
// web/src/pkl-editor/embedded.ts
// Detects starlark("…") and starlark("""…""") regions within a Pkl source string.
// Returns an array of line ranges (1-based) for the Starlark content inside the call.

export interface StarlarkRegion {
  /** 1-based line of the first Starlark source line (after the opening quote) */
  startLine: number;
  /** 1-based line of the last Starlark source line (before the closing quote) */
  endLine: number;
  /** Character offset of the Starlark content start within the full source */
  startOffset: number;
  /** Character offset of the Starlark content end within the full source */
  endOffset: number;
}

// Matches starlark(""" or starlark(" or starlark(r""" variants.
const OPEN_TRIPLE = /starlark\s*\(\s*(?:r?""")/g;
const CLOSE_TRIPLE = /"""\s*\)/g;
const OPEN_SINGLE = /starlark\s*\(\s*(?:r?")/g;
const CLOSE_SINGLE = /(?<!\\)"\s*\)/g;

export function findStarlarkRegions(source: string): StarlarkRegion[] {
  const regions: StarlarkRegion[] = [];
  const lines = source.split("\n");

  function offsetToLine(offset: number): number {
    let remaining = offset;
    for (let i = 0; i < lines.length; i++) {
      if (remaining <= lines[i].length) return i + 1;
      remaining -= lines[i].length + 1;
    }
    return lines.length;
  }

  // Try triple-quoted first (most common in Pkl Starlark usage).
  OPEN_TRIPLE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = OPEN_TRIPLE.exec(source)) !== null) {
    const contentStart = m.index + m[0].length;
    CLOSE_TRIPLE.lastIndex = contentStart;
    const close = CLOSE_TRIPLE.exec(source);
    if (!close) continue;
    const contentEnd = close.index;
    regions.push({
      startLine: offsetToLine(contentStart),
      endLine: offsetToLine(contentEnd),
      startOffset: contentStart,
      endOffset: contentEnd,
    });
    OPEN_TRIPLE.lastIndex = close.index + close[0].length;
  }

  return regions;
}

/** Returns true if the given 1-based line falls inside any Starlark region. */
export function lineInStarlarkRegion(regions: StarlarkRegion[], line: number): boolean {
  return regions.some((r) => line >= r.startLine && line <= r.endLine);
}
```

- [ ] **Step 3: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/embedded.test.ts
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/pkl-editor/embedded.ts web/src/pkl-editor/embedded.test.ts
git commit -m "feat(web): embedded Starlark region detector (UI v2 plan 12)"
```

---

### Task 12.8 — `FileTree` component

**Files:**
- Create: `web/src/pkl-editor/FileTree.tsx`

**Steps:**

- [ ] **Step 1: Write the test**

```tsx
// web/src/pkl-editor/FileTree.test.tsx
import { render, screen, fireEvent } from "@testing-library/react";
import FileTree from "./FileTree";

const FILES = [
  { path: "automations/sunset-lights.pkl", dirty: true, hasError: false },
  { path: "automations/morning.pkl", dirty: false, hasError: true },
  { path: "base/main.pkl", dirty: false, hasError: false },
];

test("renders dirty dot for dirty files", () => {
  render(<FileTree files={FILES} activePath="" onSelect={() => {}} onSearch={() => {}} />);
  const dots = screen.getAllByRole("status");
  expect(dots.some((d) => d.getAttribute("aria-label") === "unsaved changes")).toBe(true);
});

test("renders error badge for files with errors", () => {
  render(<FileTree files={FILES} activePath="" onSelect={() => {}} onSearch={() => {}} />);
  expect(screen.getByLabelText("has errors")).toBeInTheDocument();
});

test("calls onSelect with full path on file click", () => {
  const onSelect = vi.fn();
  render(<FileTree files={FILES} activePath="" onSelect={onSelect} onSearch={() => {}} />);
  fireEvent.click(screen.getByText("sunset-lights.pkl"));
  expect(onSelect).toHaveBeenCalledWith("automations/sunset-lights.pkl");
});

test("calls onSearch when search input is activated", () => {
  const onSearch = vi.fn();
  render(<FileTree files={FILES} activePath="" onSelect={() => {}} onSearch={onSearch} />);
  fireEvent.click(screen.getByPlaceholderText(/find file/i));
  expect(onSearch).toHaveBeenCalled();
});
```

- [ ] **Step 2: Implement `FileTree.tsx`**

```tsx
// web/src/pkl-editor/FileTree.tsx
import { useMemo } from "react";

export interface FileEntry {
  path: string; // relative to ~/.switchyard/
  dirty: boolean;
  hasError: boolean;
}

interface FileTreeProps {
  files: FileEntry[];
  activePath: string;
  onSelect: (path: string) => void;
  onSearch: () => void;
}

function groupByDir(files: FileEntry[]): Map<string, FileEntry[]> {
  const map = new Map<string, FileEntry[]>();
  for (const f of files) {
    const slash = f.path.indexOf("/");
    const dir = slash >= 0 ? f.path.slice(0, slash) : "";
    const group = map.get(dir) ?? [];
    group.push(f);
    map.set(dir, group);
  }
  return map;
}

export default function FileTree({ files, activePath, onSelect, onSearch }: FileTreeProps) {
  const groups = useMemo(() => groupByDir(files), [files]);

  return (
    <aside
      style={{ width: 248, flexShrink: 0, overflow: "auto", borderRight: "1px solid var(--sy-color-line)" }}
    >
      {/* Search affordance — opens ⌘P palette scoped to files */}
      <button
        onClick={onSearch}
        style={{ display: "block", width: "100%", textAlign: "left", padding: "var(--sy-space-2) var(--sy-space-3)",
          background: "var(--sy-color-surface-1)", border: "none", cursor: "pointer", color: "var(--sy-color-fg-3)" }}
      >
        <input
          readOnly
          placeholder="Find file (⌘P)…"
          onClick={onSearch}
          style={{ pointerEvents: "none", background: "transparent", border: "none", width: "100%", color: "inherit" }}
        />
      </button>
      {Array.from(groups.entries()).map(([dir, entries]) => (
        <div key={dir}>
          {dir && (
            <div style={{ padding: "var(--sy-space-1) var(--sy-space-3)", fontSize: 11,
              color: "var(--sy-color-fg-4)", textTransform: "uppercase", letterSpacing: "0.06em" }}>
              {dir}
            </div>
          )}
          {entries.map((f) => {
            const name = f.path.includes("/") ? f.path.split("/").pop()! : f.path;
            return (
              <button
                key={f.path}
                onClick={() => onSelect(f.path)}
                aria-current={f.path === activePath ? "page" : undefined}
                style={{
                  display: "flex", alignItems: "center", gap: "var(--sy-space-1)",
                  width: "100%", padding: "var(--sy-space-1) var(--sy-space-3)", textAlign: "left",
                  background: f.path === activePath ? "var(--sy-color-accent-soft)" : "transparent",
                  border: "none", cursor: "pointer", color: "var(--sy-color-fg)",
                }}
              >
                <span style={{ flex: 1, fontSize: 13 }}>{name}</span>
                {f.dirty && (
                  <span role="status" aria-label="unsaved changes"
                    style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--sy-color-warn)" }} />
                )}
                {f.hasError && (
                  <span aria-label="has errors"
                    style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--sy-color-bad)" }} />
                )}
              </button>
            );
          })}
        </div>
      ))}
    </aside>
  );
}
```

- [ ] **Step 3: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/FileTree.test.tsx
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/pkl-editor/FileTree.tsx web/src/pkl-editor/FileTree.test.tsx
git commit -m "feat(web): FileTree component with dirty/error badges (UI v2 plan 12)"
```

---

### Task 12.9 — Form-bound gutter decorations

**Files:**
- Create: `web/src/pkl-editor/form-bound-decorations.ts`

**Steps:**

- [ ] **Step 1: Write the test**

```ts
// web/src/pkl-editor/form-bound-decorations.test.ts
import { buildDecorations } from "./form-bound-decorations";

const REGIONS = [
  { startLine: 5, endLine: 8, formEditorId: "automations/sunset-lights.pkl", label: "actions[0]" },
];

test("returns one decoration per form-bound region", () => {
  const decs = buildDecorations(REGIONS);
  expect(decs).toHaveLength(1);
  expect(decs[0].range.startLineNumber).toBe(5);
  expect(decs[0].range.endLineNumber).toBe(8);
  expect(decs[0].options.className).toContain("form-bound-region");
  expect(decs[0].options.glyphMarginClassName).toContain("form-bound-glyph");
});
```

- [ ] **Step 2: Implement `form-bound-decorations.ts`**

```ts
// web/src/pkl-editor/form-bound-decorations.ts
// Builds Monaco editor decorations for form-bound regions returned by
// ConfigService.GetFormBoundRegions (added in Plan 11).

export interface FormBoundRegion {
  startLine: number;
  endLine: number;
  formEditorId: string; // e.g. "automations/sunset-lights.pkl"
  label: string;        // e.g. "actions[0]"
}

export interface MonacoDecoration {
  range: { startLineNumber: number; startColumn: number; endLineNumber: number; endColumn: number };
  options: {
    className?: string;
    glyphMarginClassName?: string;
    glyphMarginHoverMessage?: { value: string };
    isWholeLine?: boolean;
    overviewRuler?: { color: string; position: number };
  };
}

export function buildDecorations(regions: FormBoundRegion[]): MonacoDecoration[] {
  return regions.map((r) => ({
    range: {
      startLineNumber: r.startLine,
      startColumn: 1,
      endLineNumber: r.endLine,
      endColumn: Number.MAX_SAFE_INTEGER,
    },
    options: {
      className: "form-bound-region",       // purple tinted background; defined in pkl-editor.css
      glyphMarginClassName: "form-bound-glyph",  // purple bar in the gutter
      glyphMarginHoverMessage: {
        value: `**Form-bound region** — _${r.label}_\n\n[Reveal in form editor →](action:revealFormEditor?${r.formEditorId})`,
      },
      isWholeLine: true,
      overviewRuler: { color: "var(--sy-color-purple)", position: 4 /* OverviewRulerLane.Right */ },
    },
  }));
}
```

Add a `web/src/pkl-editor/pkl-editor.css` with:
```css
.form-bound-region {
  background: color-mix(in srgb, var(--sy-color-purple) 10%, transparent);
}
.form-bound-glyph {
  width: 3px !important;
  background: var(--sy-color-purple);
  left: 8px;
}
```

- [ ] **Step 3: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/form-bound-decorations.test.ts
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/pkl-editor/form-bound-decorations.ts web/src/pkl-editor/form-bound-decorations.test.ts web/src/pkl-editor/pkl-editor.css
git commit -m "feat(web): form-bound gutter decorations (UI v2 plan 12)"
```

---

### Task 12.10 — `AstBreadcrumb` + `Inspector` + `StatusBar`

**Files:**
- Create: `web/src/pkl-editor/AstBreadcrumb.tsx`
- Create: `web/src/pkl-editor/Inspector.tsx`
- Create: `web/src/pkl-editor/StatusBar.tsx`

**Steps:**

- [ ] **Step 1: Write the tests**

```tsx
// web/src/pkl-editor/AstBreadcrumb.test.tsx
import { render, screen } from "@testing-library/react";
import AstBreadcrumb from "./AstBreadcrumb";

test("renders path segments as breadcrumb items", () => {
  render(<AstBreadcrumb path={["automations", "sunset-lights.pkl", "actions [2]", "brightness"]} />);
  expect(screen.getByText("automations")).toBeInTheDocument();
  expect(screen.getByText("brightness")).toBeInTheDocument();
});

test("renders empty state when path is empty", () => {
  const { container } = render(<AstBreadcrumb path={[]} />);
  expect(container.querySelector("nav")?.children.length).toBe(0);
});
```

```tsx
// web/src/pkl-editor/StatusBar.test.tsx
import { render, screen, fireEvent } from "@testing-library/react";
import StatusBar from "./StatusBar";

test("shows unsaved count and error count", () => {
  render(
    <StatusBar
      pklVersion="0.27"
      unsavedCount={3}
      errorCount={1}
      formBoundCount={1}
      line={18}
      col={60}
      onFormat={vi.fn()}
      onValidate={vi.fn()}
      onApply={vi.fn()}
    />
  );
  expect(screen.getByText(/3 unsaved/)).toBeInTheDocument();
  expect(screen.getByText(/1 error/)).toBeInTheDocument();
});

test("calls onApply when Apply changes is clicked", () => {
  const onApply = vi.fn();
  render(
    <StatusBar pklVersion="0.27" unsavedCount={1} errorCount={0} formBoundCount={0}
      line={1} col={1} onFormat={vi.fn()} onValidate={vi.fn()} onApply={onApply} />
  );
  fireEvent.click(screen.getByRole("button", { name: /apply changes/i }));
  expect(onApply).toHaveBeenCalledTimes(1);
});
```

- [ ] **Step 2: Implement `AstBreadcrumb.tsx`**

```tsx
// web/src/pkl-editor/AstBreadcrumb.tsx
interface AstBreadcrumbProps { path: string[] }

export default function AstBreadcrumb({ path }: AstBreadcrumbProps) {
  if (path.length === 0) return <nav aria-label="AST path" />;
  return (
    <nav aria-label="AST path" style={{ display: "flex", alignItems: "center", gap: 4,
      padding: "2px var(--sy-space-3)", fontSize: 12, color: "var(--sy-color-fg-3)",
      borderBottom: "1px solid var(--sy-color-line)", overflow: "hidden", whiteSpace: "nowrap" }}>
      {path.map((seg, i) => (
        <span key={i} style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <span>{seg}</span>
          {i < path.length - 1 && <span style={{ color: "var(--sy-color-fg-4)" }}>›</span>}
        </span>
      ))}
    </nav>
  );
}
```

- [ ] **Step 3: Implement `StatusBar.tsx`**

```tsx
// web/src/pkl-editor/StatusBar.tsx
interface StatusBarProps {
  pklVersion: string;
  unsavedCount: number;
  errorCount: number;
  formBoundCount: number;
  line: number;
  col: number;
  onFormat: () => void;
  onValidate: () => void;
  onApply: () => void;
}

export default function StatusBar({ pklVersion, unsavedCount, errorCount, formBoundCount,
  line, col, onFormat, onValidate, onApply }: StatusBarProps) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: "var(--sy-space-3)",
      padding: "0 var(--sy-space-3)", height: 28, fontSize: 12,
      borderTop: "1px solid var(--sy-color-line)", background: "var(--sy-color-surface-1)",
      color: "var(--sy-color-fg-3)", flexShrink: 0 }}>
      <span>Pkl {pklVersion}</span>
      <span>·</span>
      <span>{unsavedCount} unsaved</span>
      <span>·</span>
      <span style={{ color: errorCount > 0 ? "var(--sy-color-bad)" : undefined }}>
        {errorCount} error{errorCount !== 1 ? "s" : ""}
      </span>
      <span>·</span>
      <span>{formBoundCount} form-bound region{formBoundCount !== 1 ? "s" : ""}</span>
      <span>·</span>
      <span>Ln {line}, Col {col}</span>
      <span>·</span>
      <span>spaces:2 · UTF-8 · LF</span>
      <div style={{ flex: 1 }} />
      <button onClick={onFormat} style={{ fontSize: 12, padding: "2px 8px", cursor: "pointer" }}>Format</button>
      <button onClick={onValidate} style={{ fontSize: 12, padding: "2px 8px", cursor: "pointer" }}>Validate</button>
      <button onClick={onApply} style={{ fontSize: 12, padding: "2px 8px", cursor: "pointer",
        background: "var(--sy-color-accent)", color: "#fff", border: "none", borderRadius: "var(--sy-radius-sm)" }}>
        Apply changes
      </button>
    </div>
  );
}
```

- [ ] **Step 4: Implement `Inspector.tsx` (skeleton — wired in Task 12.11)**

```tsx
// web/src/pkl-editor/Inspector.tsx
import type { FormBoundRegion } from "./form-bound-decorations";
import type { StarlarkRegion } from "./embedded";

interface InspectorProps {
  filePath: string;
  cursorLine: number;
  cursorCol: number;
  formBoundRegions: FormBoundRegion[];
  starlarkRegions: StarlarkRegion[];
  problems: Array<{ line: number; message: string; severity: "error" | "warning" }>;
  onRevealFormEditor: (editorId: string) => void;
}

export default function Inspector({
  formBoundRegions, starlarkRegions, cursorLine, problems, onRevealFormEditor,
}: InspectorProps) {
  const activeFBR = formBoundRegions.find((r) => cursorLine >= r.startLine && cursorLine <= r.endLine);
  const inStarlark = starlarkRegions.some((r) => cursorLine >= r.startLine && cursorLine <= r.endLine);

  return (
    <aside style={{ width: 320, flexShrink: 0, overflow: "auto", borderLeft: "1px solid var(--sy-color-line)",
      padding: "var(--sy-space-3)", fontSize: 12, display: "flex", flexDirection: "column", gap: "var(--sy-space-3)" }}>

      {activeFBR && (
        <section>
          <h4 style={{ margin: 0, color: "var(--sy-color-purple)" }}>Form-bound region</h4>
          <p style={{ margin: "4px 0", color: "var(--sy-color-fg-2)" }}>{activeFBR.label}</p>
          <button onClick={() => onRevealFormEditor(activeFBR.formEditorId)} style={{ fontSize: 12 }}>
            Reveal in form editor →
          </button>
        </section>
      )}

      {inStarlark && (
        <section>
          <h4 style={{ margin: 0, color: "var(--sy-color-fg-2)" }}>Embedded Starlark</h4>
          <p style={{ margin: "4px 0", color: "var(--sy-color-fg-3)" }}>
            Starlark expression. Autocomplete and hover from StarlarkLs.
          </p>
        </section>
      )}

      {problems.length > 0 && (
        <section>
          <h4 style={{ margin: 0 }}>Problems — this file</h4>
          <ul style={{ margin: "4px 0", padding: 0, listStyle: "none" }}>
            {problems.map((p, i) => (
              <li key={i} style={{ color: p.severity === "error" ? "var(--sy-color-bad)" : "var(--sy-color-warn)",
                padding: "2px 0" }}>
                Ln {p.line}: {p.message}
              </li>
            ))}
          </ul>
        </section>
      )}

      {!activeFBR && !inStarlark && problems.length === 0 && (
        <p style={{ color: "var(--sy-color-fg-4)" }}>No information at cursor.</p>
      )}
    </aside>
  );
}
```

- [ ] **Step 5: Run all tests**

```bash
cd web && npx vitest run src/pkl-editor/AstBreadcrumb.test.tsx src/pkl-editor/StatusBar.test.tsx
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/pkl-editor/AstBreadcrumb.tsx web/src/pkl-editor/AstBreadcrumb.test.tsx \
  web/src/pkl-editor/StatusBar.tsx web/src/pkl-editor/StatusBar.test.tsx \
  web/src/pkl-editor/Inspector.tsx
git commit -m "feat(web): AstBreadcrumb, Inspector, StatusBar (UI v2 plan 12)"
```

---

### Task 12.11 — Main editor route (`route.tsx`)

**Files:**
- Create: `web/src/pkl-editor/route.tsx`
- Modify: `web/src/routes/_authed/` (add `pkl-editor.$.tsx` file that re-exports the route component)

**Steps:**

- [ ] **Step 1: Write the test**

```tsx
// web/src/pkl-editor/route.test.tsx
import { render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import PklEditorRoute from "./route";

// Mock Monaco so we don't load the real editor in tests
vi.mock("./Monaco", () => ({
  default: ({ value }: { value: string }) => <textarea data-testid="editor" defaultValue={value} />,
}));

vi.mock("../data/starlarkls-client", () => ({
  starlarkLsClient: { complete: vi.fn().mockResolvedValue({ items: [] }) },
}));

test("renders FileTree and editor area", async () => {
  const router = createMemoryRouter(
    [{ path: "/pkl-editor/*", element: <PklEditorRoute /> }],
    { initialEntries: ["/pkl-editor/automations/sunset-lights.pkl"] }
  );
  render(<RouterProvider router={router} />);
  await waitFor(() => {
    expect(screen.getByTestId("editor")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Implement `route.tsx`**

```tsx
// web/src/pkl-editor/route.tsx
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import Monaco from "./Monaco";
import FileTree, { type FileEntry } from "./FileTree";
import Inspector from "./Inspector";
import AstBreadcrumb from "./AstBreadcrumb";
import StatusBar from "./StatusBar";
import { buildDecorations, type FormBoundRegion } from "./form-bound-decorations";
import { findStarlarkRegions } from "./embedded";
import { registerPklLanguage } from "./languages/pkl";
import { registerStarlarkLanguage, STARLARK_LANGUAGE_ID } from "./languages/starlark";

// Placeholder — Plan 11 provides the real client; import it here once available.
// import { configClient } from "../data/client";

export default function PklEditorRoute() {
  const { "*": filePath = "" } = useParams();
  const navigate = useNavigate();

  const [files, setFiles] = useState<FileEntry[]>([]);
  const [content, setContent] = useState("");
  const [astPath, setAstPath] = useState<string[]>([]);
  const [formBoundRegions] = useState<FormBoundRegion[]>([]);
  const [problems, setProblems] = useState<Array<{ line: number; message: string; severity: "error" | "warning" }>>([]);
  const [cursorLine, setCursorLine] = useState(1);
  const [cursorCol, setCursorCol] = useState(1);
  const editorRef = useRef<unknown>(null);

  const starlarkRegions = useMemo(() => findStarlarkRegions(content), [content]);

  // Register languages once Monaco loads.
  const handleEditorMount = useCallback((editor: unknown, monaco: unknown) => {
    const m = monaco as typeof import("monaco-editor");
    registerPklLanguage(m);
    registerStarlarkLanguage(m);
    editorRef.current = editor;

    const ed = editor as import("monaco-editor").editor.IStandaloneCodeEditor;
    ed.onDidChangeCursorPosition((e) => {
      setCursorLine(e.position.lineNumber);
      setCursorCol(e.position.column);
    });
  }, []);

  // In a real implementation this would call ConfigService.OpenForEdit(filePath)
  // and ConfigService.GetFormBoundRegions(filePath). Placeholder for now.
  useEffect(() => {
    if (!filePath) return;
    // Placeholder: real content comes from ConfigService.OpenForEdit
    setContent(`// ${filePath}\n`);
    setFiles([{ path: filePath, dirty: false, hasError: false }]);
  }, [filePath]);

  const handleFormat = () => { /* ConfigService.FormatFile */ };
  const handleValidate = () => { /* ConfigService.ValidateFile → setProblems */ };
  const handleApply = () => { /* Plan 11 CommitEdit */ };

  const handleRevealFormEditor = (editorId: string) => {
    // Navigate to the form editor. Path format depends on the form-editor's route.
    navigate(`/${editorId.replace(/\.pkl$/, "").replace(/\//g, "/")}`);
  };

  const unsavedCount = files.filter((f) => f.dirty).length;

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden" }}>
      {/* AppRail (56px) is rendered by the shell layout when on this route */}
      <FileTree
        files={files}
        activePath={filePath}
        onSelect={(p) => navigate(`/_authed/pkl-editor/${p}`)}
        onSearch={() => { /* open ⌘P palette scoped to files */ }}
      />
      <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <AstBreadcrumb path={astPath} />
        <div style={{ flex: 1, overflow: "hidden" }}>
          <Monaco
            language={filePath.endsWith(".star") ? "starlark" : "pkl"}
            value={content}
            onChange={setContent}
            onEditorMount={handleEditorMount}
          />
        </div>
        <StatusBar
          pklVersion="0.27"
          unsavedCount={unsavedCount}
          errorCount={problems.filter((p) => p.severity === "error").length}
          formBoundCount={formBoundRegions.length}
          line={cursorLine}
          col={cursorCol}
          onFormat={handleFormat}
          onValidate={handleValidate}
          onApply={handleApply}
        />
      </div>
      <Inspector
        filePath={filePath}
        cursorLine={cursorLine}
        cursorCol={cursorCol}
        formBoundRegions={formBoundRegions}
        starlarkRegions={starlarkRegions}
        problems={problems}
        onRevealFormEditor={handleRevealFormEditor}
      />
    </div>
  );
}
```

- [ ] **Step 3: Register the route in the TanStack router**

In `web/src/routes/_authed/pkl-editor.$.tsx` (create this file):
```tsx
import { createFileRoute } from "@tanstack/react-router";
import PklEditorRoute from "../../pkl-editor/route";
export const Route = createFileRoute("/_authed/pkl-editor/$")({ component: PklEditorRoute });
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/route.test.tsx
```
Expected: PASS (Monaco is mocked; the route renders without loading the real editor).

- [ ] **Step 5: Commit**

```bash
git add web/src/pkl-editor/route.tsx web/src/pkl-editor/route.test.tsx \
  web/src/routes/_authed/pkl-editor.$.tsx
git commit -m "feat(web): pkl-editor main route (UI v2 plan 12)"
```

---

### Task 12.12 — `starlarkls-client.ts` + StarlarkLs autocomplete provider

**Files:**
- Create: `web/src/data/starlarkls-client.ts`
- Modify: `web/src/pkl-editor/route.tsx` (wire the completion provider into Monaco)

**Steps:**

- [ ] **Step 1: Write the test**

```ts
// web/src/data/starlarkls-client.test.ts
import { buildStarlarkLsClient } from "./starlarkls-client";

test("buildStarlarkLsClient returns object with complete, hover, lookupSymbol", () => {
  const c = buildStarlarkLsClient("http://localhost:8080");
  expect(typeof c.complete).toBe("function");
  expect(typeof c.hover).toBe("function");
  expect(typeof c.lookupSymbol).toBe("function");
});
```

- [ ] **Step 2: Implement `starlarkls-client.ts`**

```ts
// web/src/data/starlarkls-client.ts
// Typed wrapper around StarlarkLsService Connect-RPC calls.
// Uses the same Connect transport as the rest of the web app.

import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient } from "@connectrpc/connect";
import { StarlarkLsService } from "../../proto/switchyard/starlarkls/v1/starlarkls_connect";

export function buildStarlarkLsClient(baseUrl: string) {
  const transport = createConnectTransport({ baseUrl });
  const rpc = createClient(StarlarkLsService, transport);
  return {
    complete: (filePath: string, source: string, line: number, col: number) =>
      rpc.complete({ filePath, source, line, col }),
    hover: (filePath: string, source: string, line: number, col: number) =>
      rpc.hover({ filePath, source, line, col }),
    lookupSymbol: (name: string) =>
      rpc.lookupSymbol({ name }),
  };
}

export type StarlarkLsClient = ReturnType<typeof buildStarlarkLsClient>;

// Singleton used by the editor route.
export const starlarkLsClient = buildStarlarkLsClient(window.location.origin);
```

- [ ] **Step 3: Register Monaco completion + hover providers in `route.tsx`**

Inside `handleEditorMount` in `route.tsx`, add after language registration:
```ts
// Register Starlark autocomplete provider
m.languages.registerCompletionItemProvider(STARLARK_LANGUAGE_ID, {
  triggerCharacters: [".", "_"],
  provideCompletionItems: async (model, position) => {
    const source = model.getValue();
    const resp = await starlarkLsClient.complete(filePath, source, position.lineNumber, position.column);
    return {
      suggestions: resp.items.map((item) => ({
        label: item.label,
        kind: item.kind === "function" ? m.languages.CompletionItemKind.Function : m.languages.CompletionItemKind.Variable,
        detail: item.detail,
        insertText: item.insertText,
        range: {
          startLineNumber: position.lineNumber, startColumn: position.column,
          endLineNumber: position.lineNumber, endColumn: position.column,
        },
      })),
    };
  },
});

// Register Starlark hover provider
m.languages.registerHoverProvider(STARLARK_LANGUAGE_ID, {
  provideHover: async (model, position) => {
    const source = model.getValue();
    const resp = await starlarkLsClient.hover(filePath, source, position.lineNumber, position.column);
    if (!resp.markdown) return null;
    return { contents: [{ value: resp.markdown }] };
  },
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/data/starlarkls-client.test.ts
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/data/starlarkls-client.ts web/src/data/starlarkls-client.test.ts \
  web/src/pkl-editor/route.tsx
git commit -m "feat(web): StarlarkLs client + Monaco completion/hover providers (UI v2 plan 12)"
```

---

### Task 12.13 — 3-way merge route

**Files:**
- Create: `web/src/pkl-editor/merge-view.tsx`
- Create: `web/src/pkl-editor/merge-route.tsx`
- Create: `web/src/routes/_authed/pkl-editor.merge.$.tsx`

**Steps:**

- [ ] **Step 1: Write the test**

```tsx
// web/src/pkl-editor/merge-route.test.tsx
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import MergeRoute from "./merge-route";

vi.mock("./Monaco", () => ({
  default: ({ value, "data-testid": testId }: { value: string; "data-testid"?: string }) =>
    <textarea data-testid={testId ?? "editor"} defaultValue={value} />,
}));

test("renders three editor panes with correct labels", async () => {
  const router = createMemoryRouter(
    [{ path: "/pkl-editor/merge/*", element: <MergeRoute /> }],
    { initialEntries: ["/pkl-editor/merge/automations/sunset-lights.pkl?session=abc"] }
  );
  render(<RouterProvider router={router} />);
  expect(screen.getByText(/on disk now/i)).toBeInTheDocument();
  expect(screen.getByText(/common ancestor/i)).toBeInTheDocument();
  expect(screen.getByText(/your changes/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: Implement `merge-view.tsx`**

```tsx
// web/src/pkl-editor/merge-view.tsx
import Monaco from "./Monaco";

interface MergeViewProps {
  diskContent: string;
  ancestorContent: string;
  yourContent: string;
  onPickLeft: () => void;
  onPickRight: () => void;
  onSave: (mergedContent: string) => void;
}

export default function MergeView({
  diskContent, ancestorContent, yourContent, onSave,
}: MergeViewProps) {
  return (
    <div style={{ display: "flex", flex: 1, overflow: "hidden", gap: 1,
      background: "var(--sy-color-line)" }}>
      {([
        { label: "On disk now", content: diskContent, readOnly: true },
        { label: "Common ancestor — when you opened it", content: ancestorContent, readOnly: true },
        { label: "Your changes", content: yourContent, readOnly: false },
      ] as const).map(({ label, content, readOnly }) => (
        <div key={label} style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
          <div style={{ padding: "4px var(--sy-space-2)", fontSize: 11,
            background: "var(--sy-color-surface-1)", color: "var(--sy-color-fg-3)" }}>
            {label}
          </div>
          <div style={{ flex: 1, overflow: "hidden" }}>
            <Monaco
              language="pkl"
              value={content}
              onChange={readOnly ? undefined : () => {}}
              options={{ readOnly }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Implement `merge-route.tsx`**

```tsx
// web/src/pkl-editor/merge-route.tsx
import { useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import MergeView from "./merge-view";

export default function MergeRoute() {
  const { "*": filePath = "" } = useParams();
  const [searchParams] = useSearchParams();
  const sessionId = searchParams.get("session") ?? "";

  // Placeholder content — real implementation fetches from ConfigService using sessionId.
  const [diskContent] = useState(`// On-disk version of ${filePath}`);
  const [ancestorContent] = useState(`// Common ancestor of ${filePath}`);
  const [yourContent] = useState(`// Your in-memory changes to ${filePath}`);

  const handleSave = (_merged: string) => {
    // Call ConfigService.CommitEdit(filePath, lockToken, merged, expectedHash, force=true)
    console.info("save merged result for session", sessionId);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh" }}>
      <div style={{ padding: "var(--sy-space-2) var(--sy-space-3)", fontSize: 12,
        background: "var(--sy-color-warn)", color: "#000" }}>
        Merge conflict in <strong>{filePath}</strong> — pick changes then "Save merged result".
      </div>
      <MergeView
        diskContent={diskContent}
        ancestorContent={ancestorContent}
        yourContent={yourContent}
        onPickLeft={() => {}}
        onPickRight={() => {}}
        onSave={handleSave}
      />
      <div style={{ display: "flex", justifyContent: "flex-end", padding: "var(--sy-space-2) var(--sy-space-3)",
        borderTop: "1px solid var(--sy-color-line)", gap: "var(--sy-space-2)" }}>
        <button onClick={() => history.back()}>Cancel</button>
        <button onClick={() => handleSave(yourContent)}
          style={{ background: "var(--sy-color-accent)", color: "#fff", border: "none",
            padding: "4px 12px", borderRadius: "var(--sy-radius-sm)", cursor: "pointer" }}>
          Save merged result
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Register route**

```tsx
// web/src/routes/_authed/pkl-editor.merge.$.tsx
import { createFileRoute } from "@tanstack/react-router";
import MergeRoute from "../../pkl-editor/merge-route";
export const Route = createFileRoute("/_authed/pkl-editor/merge/$")({ component: MergeRoute });
```

- [ ] **Step 5: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/merge-route.test.tsx
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/pkl-editor/merge-view.tsx web/src/pkl-editor/merge-route.tsx \
  web/src/pkl-editor/merge-route.test.tsx web/src/routes/_authed/pkl-editor.merge.$.tsx
git commit -m "feat(web): 3-way merge route (UI v2 plan 12)"
```

---

### Task 12.14 — Integration test: edit → validate → apply

**Files:**
- Create: `web/src/pkl-editor/integration.test.tsx`

**Steps:**

- [ ] **Step 1: Write the test**

```tsx
// web/src/pkl-editor/integration.test.tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import PklEditorRoute from "./route";

vi.mock("./Monaco", () => ({
  default: ({ value, onChange }: { value: string; onChange?: (v: string) => void }) => (
    <textarea
      data-testid="editor"
      defaultValue={value}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}));

// Mock ConfigService CommitEdit: returns success on first call.
const mockCommitEdit = vi.fn().mockResolvedValue({ status: "ok" });
vi.mock("../data/client", () => ({
  configClient: { commitEdit: mockCommitEdit, openForEdit: vi.fn().mockResolvedValue({ content: "name = 1\n" }) },
}));

test("editing content and clicking Apply calls CommitEdit", async () => {
  const router = createMemoryRouter(
    [{ path: "/pkl-editor/*", element: <PklEditorRoute /> }],
    { initialEntries: ["/pkl-editor/automations/sunset-lights.pkl"] }
  );
  render(<RouterProvider router={router} />);

  const editor = await screen.findByTestId("editor");
  fireEvent.change(editor, { target: { value: "name = 2\n" } });

  fireEvent.click(screen.getByRole("button", { name: /apply changes/i }));

  await waitFor(() => {
    expect(mockCommitEdit).toHaveBeenCalledWith(
      expect.objectContaining({ content: "name = 2\n" })
    );
  });
});

test("validation errors appear in status bar", async () => {
  const mockValidate = vi.fn().mockResolvedValue({
    problems: [{ line: 1, message: "Expected Int, got String", severity: "error" }],
  });
  vi.mock("../data/client", () => ({
    configClient: { validateFile: mockValidate, openForEdit: vi.fn().mockResolvedValue({ content: "" }) },
  }));

  const router = createMemoryRouter(
    [{ path: "/pkl-editor/*", element: <PklEditorRoute /> }],
    { initialEntries: ["/pkl-editor/automations/sunset-lights.pkl"] }
  );
  render(<RouterProvider router={router} />);

  fireEvent.click(await screen.findByRole("button", { name: /validate/i }));

  await waitFor(() => {
    expect(screen.getByText(/1 error/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests**

```bash
cd web && npx vitest run src/pkl-editor/integration.test.tsx
```
Expected: PASS (both tests pass with mocked services).

- [ ] **Step 3: Commit**

```bash
git add web/src/pkl-editor/integration.test.tsx
git commit -m "test(web): pkl-editor integration test (edit + validate + apply) (UI v2 plan 12)"
```

---

### Task 12.15 — Playwright end-to-end test

**Files:**
- Create: `web/e2e/pkl-editor.spec.ts`

**Steps:**

- [ ] **Step 1: Write the Playwright spec**

```ts
// web/e2e/pkl-editor.spec.ts
import { test, expect } from "@playwright/test";

test.describe("Pkl editor", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the editor route with a test fixture file.
    // The dev server must be running: task ui:dev
    await page.goto("http://localhost:5173/_authed/pkl-editor/automations/sunset-lights.pkl");
    // Wait for Monaco to lazy-load (look for the editor container).
    await page.waitForSelector(".monaco-editor", { timeout: 15_000 });
  });

  test("editor loads and shows the file tree", async ({ page }) => {
    await expect(page.locator("[aria-label='unsaved changes'], [aria-label='has errors']").first()).toBeHidden();
    // File tree shows automations directory
    await expect(page.getByText("automations")).toBeVisible();
  });

  test("embedded Starlark line has different glyph color", async ({ page }) => {
    // Monaco renders token colors as inline spans — look for a token that would be Starlark-highlighted
    // by checking that a line containing starlark("") has a different data-token attribute.
    // This is a visual smoke-test; exact token assertion depends on Monaco internals.
    const editor = page.locator(".monaco-editor .view-lines");
    await expect(editor).toBeVisible();
  });

  test("snapshot: editor in developer theme", async ({ page }) => {
    await page.evaluate(() => {
      document.documentElement.dataset.theme = "developer";
    });
    await expect(page.locator(".monaco-editor")).toBeVisible();
    await page.screenshot({ path: "web/e2e/__screenshots__/pkl-editor/developer-theme.png", fullPage: true });
  });
});
```

- [ ] **Step 2: Run the e2e test against the dev server**

```bash
cd web && npx playwright test e2e/pkl-editor.spec.ts
```
Expected: all tests pass; screenshot committed.

- [ ] **Step 3: Commit**

```bash
git add web/e2e/pkl-editor.spec.ts web/e2e/__screenshots__/pkl-editor/
git commit -m "test(web): Playwright e2e for pkl-editor (UI v2 plan 12)"
```

---

### Task 12.16 — Barrel + final wiring checks

**Files:**
- Create: `web/src/pkl-editor/index.ts`

**Steps:**

- [ ] **Step 1: Write the barrel**

```ts
// web/src/pkl-editor/index.ts
export { default as Monaco } from "./Monaco";
export { default as FileTree } from "./FileTree";
export { default as Inspector } from "./Inspector";
export { default as AstBreadcrumb } from "./AstBreadcrumb";
export { default as StatusBar } from "./StatusBar";
export { default as MergeView } from "./merge-view";
export { findStarlarkRegions, lineInStarlarkRegion } from "./embedded";
export { buildDecorations } from "./form-bound-decorations";
export { registerPklLanguage, PKL_LANGUAGE_ID } from "./languages/pkl";
export { registerStarlarkLanguage, STARLARK_LANGUAGE_ID } from "./languages/starlark";
```

- [ ] **Step 2: Full test suite + build**

```bash
cd web && npx vitest run && npm run build
```
Expected: zero test failures; build succeeds; `dist/` does not contain `monaco-editor` in the main chunk (verify with `ls -lh dist/assets/` — the large Monaco chunk appears as a separate file).

```bash
go test ./internal/starlarkls/... && go build ./...
```
Expected: zero failures.

- [ ] **Step 3: Verify AppRail appears on the editor route**

The `AppRail` component (Plan 01) must be visible when `window.location.pathname.startsWith('/_authed/pkl-editor')`. Check `web/src/shell/AppRail.tsx` and update if needed.

- [ ] **Step 4: Commit**

```bash
git add web/src/pkl-editor/index.ts web/src/shell/AppRail.tsx
git commit -m "feat(web): pkl-editor barrel + AppRail wiring (UI v2 plan 12)"
```

---

## Test plan

- `go test ./internal/starlarkls/...` — symbol extractor, service Complete/Hover/LookupSymbol, NOT_FOUND for unknown symbols.
- `cd web && npx vitest run` — all 14 component/unit tests pass: Monaco lazy load, Pkl tokens, Starlark tokens, embedded region detection, FileTree badges, form-bound decorations, AstBreadcrumb, StatusBar, Inspector, route render, StarlarkLs client, merge route, integration edit→apply.
- `npm run build` — base bundle contains no Monaco code; separate async chunk confirmed.
- `npx playwright test e2e/pkl-editor.spec.ts` — editor loads, file tree visible, developer-theme screenshot committed.
- Manual smoke: `task ui:dev` → navigate to `/_authed/pkl-editor/automations/sunset-lights.pkl` → confirm Monaco loads lazily, Pkl tokens coloured, file tree shows dirty/error state, form-bound regions purple, Starlark embedded region detected, status bar shows correct counts, "Apply changes" → network call visible in devtools.

## Acceptance criteria for merging

- All tests + typecheck + lint green locally and in CI.
- Monaco is not in the base bundle (`dist/assets/index-*.js` has zero `monaco-editor` matches).
- The Pkl language definition highlights keywords, strings, types, and comments correctly (compare against `13-pkl-starlark-editor-01.png`).
- Embedded Starlark inside `starlark("…")` is detected and shown with Starlark token colours (compare against `13-pkl-starlark-editor-02.png`).
- The file tree shows dirty (orange) and error (red) badges.
- The inspector shows form-bound regions in purple with a "Reveal in form editor →" link.
- The status bar displays `Pkl 0.27 · N unsaved · E errors · F form-bound · Ln l, Col c · spaces:2 · UTF-8 · LF`.
- The 3-way merge route renders three editor panes with the correct header labels.
- `StarlarkLsService` is reachable at `/_connect/switchyard.starlarkls.v1.StarlarkLsService/Complete`.
- `go test ./...` and `task web:test` both green in CI.
- Linear parent issue + sub-tasks transition to `Done`.
- Branch merged via `git merge --no-ff` into main.
