# Starlark LSP wiring implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Light up the Starlark editor with completion, hover, go-to-definition, server-supplied tokens, and diagnostics by wiring the existing daemon `StarlarkLsService` to Monaco — plus extending the daemon with a new `Diagnose` RPC backed by an AST walker.

**Architecture:** Register a new Monaco `"starlark"` language with a Python-derived Monarch fallback grammar. Five providers (Completion, Hover, Definition, DocumentSemanticTokens, Diagnostics-via-markers) attach to that language and call the daemon's `StarlarkLsService` Connect RPCs. Daemon ships one new method (`Diagnose`) that parses the source, walks the AST collecting defined/loaded names, validates `load(...)` paths against the existing symbol index, and flags unresolved identifier references.

**Tech Stack:** Go (`go.starlark.net/syntax` already in go.mod), Connect, Vue 3 + TypeScript, Monaco Editor (`monaco-editor@^0.45.0` already in app/package.json).

**Spec:** `docs/design/specs/2026-05-12-starlark-lsp-design.md`

## Environment notes for the implementer

- This branch is `feat/starlark-lsp` (off main). Repo root: `/Users/fdatoo/Developer/Switchyard`.
- For `buf generate` to write `_grpc.pb.go` files cleanly, ensure `protoc-gen-go-grpc` is on PATH: `export PATH="$PATH:$(go env GOPATH)/bin"`. If it's missing, `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` once.
- The daemon's running dev binary may need to be rebuilt + restarted after daemon-side changes: `go build -o dist/switchyardd ./cmd/switchyardd && lsof -ti:8080 | xargs -I {} kill {}; rm -f /Users/fdatoo/.local/share/switchyard/switchyardd.lock; ./dist/switchyardd &`. The vite dev server picks up `.ts`/`.vue` changes via HMR.
- The Starlark editor route mounts `SyCodeEditorPanel` with `kind="starlark"`. Most likely accessible at `/settings/starlark` — confirm by grepping `app/src/router/`.

## File map

| File | Status | Responsibility |
|------|--------|----------------|
| `proto/switchyard/starlarkls/v1/starlarkls.proto` | MOD | Add `Diagnose` RPC + `DiagnoseRequest`/`Diagnostic`/`DiagnoseResponse` messages. |
| `gen/switchyard/starlarkls/v1/` | GEN | Regenerated proto bindings. |
| `internal/starlarkls/diagnose.go` | NEW | `Diagnose(filePath, source, symbols, predeclared) []Diagnostic` — parse + AST walker. |
| `internal/starlarkls/diagnose_test.go` | NEW | Table-driven unit tests. |
| `internal/starlarkls/predeclared.go` | NEW | Hand-curated set of Switchyard predeclared names + Starlark universe getter. |
| `internal/starlarkls/service.go` | MOD | Add `Diagnose` handler method delegating to `diagnose.go`. |
| `app/src/data/starlark-ls.ts` | NEW | TS client wrapping the 5 RPCs. |
| `app/src/lib/components/code-editor/starlark-grammar.ts` | NEW | Monarch grammar + language config (alongside existing `pkl-grammar.ts`). |
| `app/src/lib/components/code-editor/starlark-providers.ts` | NEW | The five Monaco providers + the diagnostics arming loop. |
| `app/src/lib/components/code-editor/SyCodeEditor.vue` | MOD | Accept `"starlark"` in the `language` prop union. Register Starlark language + providers (idempotent, mirror `ensurePklRegistered`). |
| `app/src/lib/components/code-editor-panel/SyCodeEditorPanel.vue` | MOD | Language computed returns `"starlark"` for kind `"starlark"`. Listen for `starlark-goto-definition` event and call `openFile()`. |

## Task 1 — Proto: Diagnose RPC

**Files:**
- Modify: `proto/switchyard/starlarkls/v1/starlarkls.proto`
- Generate: `gen/switchyard/starlarkls/v1/*.pb.go` + `*_grpc.pb.go` + `starlarklsv1connect/*.connect.go`

- [ ] **Step 1: Add RPC + messages to the proto**

Append to `proto/switchyard/starlarkls/v1/starlarkls.proto`:

```protobuf
service StarlarkLsService {
  rpc Tokenize(TokenizeRequest)         returns (TokenizeResponse);
  rpc Complete(CompleteRequest)         returns (CompleteResponse);
  rpc Hover(HoverRequest)               returns (HoverResponse);
  rpc LookupSymbol(LookupSymbolRequest) returns (LookupSymbolResponse);
  rpc Diagnose(DiagnoseRequest)         returns (DiagnoseResponse);
}

// Diagnose
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

Make sure the existing service block is the one containing all five RPCs after your edit — i.e. add `Diagnose` to the existing `service StarlarkLsService { ... }` rather than creating a duplicate.

- [ ] **Step 2: Regenerate**

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
buf generate
go build ./...
```

Expected: clean build. The new `Diagnose` method appears on `starlarklsv1connect.StarlarkLsServiceHandler` — the existing `*starlarkls.Service` no longer satisfies the interface, breaking the build at the interface assertion `var _ ... = (*Service)(nil)` in `service.go:13`. That's expected; Task 3 fixes it.

To unblock the build for the interim, add a tiny stub to `service.go` right above the `Tokenize` method:

```go
// Diagnose is implemented in Task 3; this stub exists temporarily so the
// service.go compiles after the proto regen.
func (s *Service) Diagnose(_ context.Context, _ *connect.Request[starlarkpb.DiagnoseRequest]) (*connect.Response[starlarkpb.DiagnoseResponse], error) {
    return connect.NewResponse(&starlarkpb.DiagnoseResponse{}), nil
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add proto/switchyard/starlarkls/v1/starlarkls.proto gen/switchyard/starlarkls/v1/ internal/starlarkls/service.go
git commit -m "feat(proto): StarlarkLsService.Diagnose RPC + stub"
```

## Task 2 — Predeclared name set

**Files:**
- Create: `internal/starlarkls/predeclared.go`

The diagnose walker needs a set of names that count as "always resolved" (Starlark universe + Switchyard runtime globals). Rather than reaching into `internal/starlark/runtime.go`'s `buildStdlib` (which is context-kind dependent), we curate a superset here.

- [ ] **Step 1: Create the file**

`internal/starlarkls/predeclared.go`:

```go
package starlarkls

// predeclaredNames returns the union of Starlark universe builtins and
// Switchyard runtime globals. A reference to any of these names is
// considered resolved at the LSP layer.
//
// The Switchyard set is a hand-curated superset of buildStdlib's
// possible outputs across all ContextKinds; over-permissive is safer
// than under-permissive (avoids false-positive unresolved_name warnings
// for legitimate globals).
func predeclaredNames() map[string]bool {
	out := map[string]bool{}
	for _, n := range starlarkUniverse {
		out[n] = true
	}
	for _, n := range switchyardGlobals {
		out[n] = true
	}
	return out
}

// starlarkUniverse is the canonical Starlark universe — the names
// available without any imports in vanilla Starlark.
// See go.starlark.net/starlark.Universe (this is a hand-mirror to
// avoid pulling that package's heavy deps into this lightweight one).
var starlarkUniverse = []string{
	// Constants
	"True", "False", "None",
	// Types/constructors
	"bool", "bytes", "dict", "float", "int", "list", "set", "str", "tuple",
	// Functions
	"abs", "all", "any", "chr", "dir", "enumerate", "fail", "getattr", "hasattr",
	"hash", "len", "list", "max", "min", "ord", "print", "range", "repr",
	"reversed", "set", "sorted", "str", "tuple", "type", "zip",
}

// switchyardGlobals is the superset of names buildStdlib may expose.
// Keep in sync with internal/starlark/runtime.go:buildStdlib when
// adding new builtins.
var switchyardGlobals = []string{
	"state", "now", "time", "log",
	"call_service", "random",
	"sleep", "notify", "scene", "event",
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/starlarkls
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/starlarkls/predeclared.go
git commit -m "feat(starlarkls): curated predeclared-name set for diagnostics"
```

## Task 3 — Diagnose AST walker

**Files:**
- Create: `internal/starlarkls/diagnose.go`
- Create: `internal/starlarkls/diagnose_test.go`
- Modify: `internal/starlarkls/service.go` (replace the Task 1 stub)

### Step 1: Write the failing tests

Create `internal/starlarkls/diagnose_test.go`:

```go
package starlarkls

import (
	"strings"
	"testing"
)

// helper: run Diagnose and return diagnostics, panicking on any unexpected error.
func diag(t *testing.T, source string, symbols map[string]SymbolInfo) []Diagnostic {
	t.Helper()
	return Diagnose("test.star", source, symbols, predeclaredNames())
}

func TestDiagnose_HappyPath(t *testing.T) {
	src := `def foo():
    return 1

def bar():
    return foo()
`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_ParseError(t *testing.T) {
	src := `def foo(`
	got := diag(t, src, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 diagnostic, got %v", got)
	}
	if got[0].Code != "parse_error" || got[0].Severity != "error" {
		t.Errorf("want parse_error/error, got %+v", got[0])
	}
}

func TestDiagnose_LoadNotFound(t *testing.T) {
	src := `load("missing.star", "x")
print(x)
`
	symbols := map[string]SymbolInfo{
		// "x" intentionally absent; we want load failure to trigger
		// suppression so that print(x) doesn't ALSO complain.
	}
	got := diag(t, src, symbols)
	// Expect exactly one load_not_found, no unresolved_name (suppressed
	// because the load named "x", so x counts as loaded best-effort).
	var loadErrs, unresolved int
	for _, d := range got {
		switch d.Code {
		case "load_not_found":
			loadErrs++
		case "unresolved_name":
			unresolved++
		}
	}
	if loadErrs != 1 {
		t.Errorf("want 1 load_not_found, got %d (all: %+v)", loadErrs, got)
	}
	if unresolved != 0 {
		t.Errorf("want 0 unresolved_name (suppressed by best-effort load), got %d", unresolved)
	}
}

func TestDiagnose_LoadFound(t *testing.T) {
	src := `load("helpers.star", "fetch")
print(fetch())
`
	symbols := map[string]SymbolInfo{
		"fetch": {File: "/path/scripts/helpers.star", Kind: "function"},
	}
	got := diag(t, src, symbols)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics (load resolves + fetch is loaded), got %v", got)
	}
}

func TestDiagnose_UnresolvedName(t *testing.T) {
	src := `print(undefined_thing)`
	got := diag(t, src, nil)
	var found bool
	for _, d := range got {
		if d.Code == "unresolved_name" && d.Severity == "warning" && strings.Contains(d.Message, "undefined_thing") {
			found = true
		}
	}
	if !found {
		t.Errorf("want unresolved_name for undefined_thing, got %+v", got)
	}
}

func TestDiagnose_BuiltinResolves(t *testing.T) {
	src := `print(len([1,2,3]))`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics (print + len are builtins), got %v", got)
	}
}

func TestDiagnose_SwitchyardGlobalResolves(t *testing.T) {
	src := `state("light.x").value`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics (state is predeclared), got %v", got)
	}
}

func TestDiagnose_ParamShadowsGlobal(t *testing.T) {
	src := `x = 1
def foo(x):
    return x
`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics (param x shadows global x), got %v", got)
	}
}

func TestDiagnose_ForwardReference(t *testing.T) {
	src := `def a():
    return b()

def b():
    return 1
`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics (forward refs resolve at top level), got %v", got)
	}
}

func TestDiagnose_ComprehensionScope(t *testing.T) {
	src := `r = [y for y in range(10)]`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics (y bound by comprehension), got %v", got)
	}
}
```

### Step 2: Verify FAIL

```bash
go test ./internal/starlarkls -run TestDiagnose -v
```

Expected: build failure — `Diagnose`/`Diagnostic` undefined in package.

### Step 3: Create `internal/starlarkls/diagnose.go`

```go
package starlarkls

import (
	"fmt"
	"path/filepath"

	"go.starlark.net/syntax"
)

// Diagnostic is the daemon-internal representation; converted to the
// proto type in service.go.
type Diagnostic struct {
	StartLine int32
	StartCol  int32
	EndLine   int32
	EndCol    int32
	Severity  string // "error" | "warning"
	Message   string
	Code      string // "parse_error" | "load_not_found" | "unresolved_name"
}

// Diagnose parses source and walks the AST, returning a flat list of
// diagnostics. symbols maps name → SymbolInfo (used to validate load
// target paths via SymbolInfo.File). predeclared is the set of names
// considered always-resolved (Starlark universe + Switchyard globals).
func Diagnose(filePath, source string, symbols map[string]SymbolInfo, predeclared map[string]bool) []Diagnostic {
	opts := syntax.LegacyFileOptions()
	f, err := opts.Parse(filePath, []byte(source), 0)
	if err != nil {
		// Single parse-error diagnostic at the parser's position.
		if se, ok := err.(syntax.Error); ok {
			return []Diagnostic{{
				StartLine: int32(se.Pos.Line),
				StartCol:  int32(se.Pos.Col - 1),
				EndLine:   int32(se.Pos.Line),
				EndCol:    int32(se.Pos.Col),
				Severity:  "error",
				Message:   se.Msg,
				Code:      "parse_error",
			}}
		}
		return []Diagnostic{{
			StartLine: 1, StartCol: 0, EndLine: 1, EndCol: 1,
			Severity: "error", Message: err.Error(), Code: "parse_error",
		}}
	}

	// Build set of valid load-target paths from the symbol index.
	loadablePaths := map[string]bool{}
	for _, sym := range symbols {
		loadablePaths[filepath.Base(sym.File)] = true
	}

	// Pass 1: collect top-level names (functions + assignments + loads).
	topLevel := map[string]bool{}
	loadedNames := map[string]bool{}
	var loadStmts []*syntax.LoadStmt
	for _, stmt := range f.Stmts {
		switch s := stmt.(type) {
		case *syntax.DefStmt:
			topLevel[s.Name.Name] = true
		case *syntax.AssignStmt:
			collectAssignTargets(s.LHS, topLevel)
		case *syntax.LoadStmt:
			loadStmts = append(loadStmts, s)
			for i, name := range s.To {
				_ = i
				loadedNames[name.Name] = true
			}
		}
	}

	var diags []Diagnostic

	// Pass 2: validate load paths.
	for _, ls := range loadStmts {
		path := ls.Module.Value.(string)
		base := filepath.Base(path)
		if !loadablePaths[base] {
			diags = append(diags, Diagnostic{
				StartLine: int32(ls.Module.TokenPos.Line),
				StartCol:  int32(ls.Module.TokenPos.Col - 1),
				EndLine:   int32(ls.Module.TokenPos.Line),
				EndCol:    int32(ls.Module.TokenPos.Col - 1 + len(path) + 2), // +2 for quotes
				Severity:  "error",
				Message:   fmt.Sprintf("load: file %q not found in scripts directory", path),
				Code:      "load_not_found",
			})
			// Best-effort: load names still count as "imported" so we
			// don't cascade unresolved_name on every use.
		}
	}

	// Pass 3: walk references, flagging unresolved.
	walker := &refWalker{
		topLevel:    topLevel,
		loaded:      loadedNames,
		predeclared: predeclared,
		diags:       &diags,
	}
	for _, stmt := range f.Stmts {
		walker.walkStmt(stmt, nil)
	}

	return diags
}

func collectAssignTargets(expr syntax.Expr, out map[string]bool) {
	switch e := expr.(type) {
	case *syntax.Ident:
		out[e.Name] = true
	case *syntax.TupleExpr:
		for _, item := range e.List {
			collectAssignTargets(item, out)
		}
	case *syntax.ListExpr:
		for _, item := range e.List {
			collectAssignTargets(item, out)
		}
		// Index/attribute assignments (a[0] = .., a.b = ..) bind no new names.
	}
}

// refWalker walks an AST, tracking a stack of local-scope name sets.
type refWalker struct {
	topLevel    map[string]bool
	loaded      map[string]bool
	predeclared map[string]bool
	diags       *[]Diagnostic
}

func (w *refWalker) isResolved(name string, locals []map[string]bool) bool {
	for i := len(locals) - 1; i >= 0; i-- {
		if locals[i][name] {
			return true
		}
	}
	return w.topLevel[name] || w.loaded[name] || w.predeclared[name]
}

func (w *refWalker) flag(id *syntax.Ident) {
	*w.diags = append(*w.diags, Diagnostic{
		StartLine: int32(id.NamePos.Line),
		StartCol:  int32(id.NamePos.Col - 1),
		EndLine:   int32(id.NamePos.Line),
		EndCol:    int32(id.NamePos.Col - 1 + len(id.Name)),
		Severity:  "warning",
		Message:   fmt.Sprintf("unresolved name %q", id.Name),
		Code:      "unresolved_name",
	})
}

func (w *refWalker) walkStmt(stmt syntax.Stmt, locals []map[string]bool) {
	switch s := stmt.(type) {
	case *syntax.DefStmt:
		// Open a new function scope: params + body's defined names.
		scope := map[string]bool{}
		for _, p := range s.Params {
			if id, ok := p.(*syntax.Ident); ok {
				scope[id.Name] = true
			}
			// BinaryExpr handles default-value params; the LHS is the param name.
			if be, ok := p.(*syntax.BinaryExpr); ok {
				if id, ok := be.X.(*syntax.Ident); ok {
					scope[id.Name] = true
				}
			}
		}
		// Pre-collect body-level defines for this function scope.
		for _, bs := range s.Body {
			if as, ok := bs.(*syntax.AssignStmt); ok {
				collectAssignTargets(as.LHS, scope)
			}
			if ds, ok := bs.(*syntax.DefStmt); ok {
				scope[ds.Name.Name] = true
			}
		}
		nested := append(locals, scope)
		for _, bs := range s.Body {
			w.walkStmt(bs, nested)
		}
	case *syntax.ExprStmt:
		w.walkExpr(s.X, locals)
	case *syntax.AssignStmt:
		// LHS — bindings. RHS — references.
		w.walkExpr(s.RHS, locals)
	case *syntax.ReturnStmt:
		if s.Result != nil {
			w.walkExpr(s.Result, locals)
		}
	case *syntax.IfStmt:
		w.walkExpr(s.Cond, locals)
		for _, bs := range s.True {
			w.walkStmt(bs, locals)
		}
		for _, bs := range s.False {
			w.walkStmt(bs, locals)
		}
	case *syntax.ForStmt:
		// Bind the loop variable(s) into a new scope.
		scope := map[string]bool{}
		collectAssignTargets(s.Vars, scope)
		nested := append(locals, scope)
		w.walkExpr(s.X, locals) // iterable evaluated in outer scope
		for _, bs := range s.Body {
			w.walkStmt(bs, nested)
		}
	case *syntax.WhileStmt:
		w.walkExpr(s.Cond, locals)
		for _, bs := range s.Body {
			w.walkStmt(bs, locals)
		}
	case *syntax.BranchStmt, *syntax.LoadStmt:
		// nothing to do
	}
}

func (w *refWalker) walkExpr(expr syntax.Expr, locals []map[string]bool) {
	switch e := expr.(type) {
	case nil:
		return
	case *syntax.Ident:
		if !w.isResolved(e.Name, locals) {
			w.flag(e)
		}
	case *syntax.Literal:
		// nothing
	case *syntax.UnaryExpr:
		w.walkExpr(e.X, locals)
	case *syntax.BinaryExpr:
		w.walkExpr(e.X, locals)
		w.walkExpr(e.Y, locals)
	case *syntax.CallExpr:
		w.walkExpr(e.Fn, locals)
		for _, arg := range e.Args {
			w.walkExpr(arg, locals)
		}
	case *syntax.DotExpr:
		// Only the receiver is a name reference; the attr is a member name.
		w.walkExpr(e.X, locals)
	case *syntax.IndexExpr:
		w.walkExpr(e.X, locals)
		w.walkExpr(e.Y, locals)
	case *syntax.SliceExpr:
		w.walkExpr(e.X, locals)
		w.walkExpr(e.Lo, locals)
		w.walkExpr(e.Hi, locals)
		w.walkExpr(e.Step, locals)
	case *syntax.ListExpr:
		for _, item := range e.List {
			w.walkExpr(item, locals)
		}
	case *syntax.TupleExpr:
		for _, item := range e.List {
			w.walkExpr(item, locals)
		}
	case *syntax.DictExpr:
		for _, item := range e.List {
			if entry, ok := item.(*syntax.DictEntry); ok {
				w.walkExpr(entry.Key, locals)
				w.walkExpr(entry.Value, locals)
			}
		}
	case *syntax.Comprehension:
		// Comprehension introduces its own scope for the iteration vars.
		scope := map[string]bool{}
		for _, clause := range e.Clauses {
			if fc, ok := clause.(*syntax.ForClause); ok {
				collectAssignTargets(fc.Vars, scope)
			}
		}
		nested := append(locals, scope)
		for _, clause := range e.Clauses {
			switch c := clause.(type) {
			case *syntax.ForClause:
				w.walkExpr(c.X, locals) // iterable in outer scope
			case *syntax.IfClause:
				w.walkExpr(c.Cond, nested)
			}
		}
		w.walkExpr(e.Body, nested)
	case *syntax.LambdaExpr:
		scope := map[string]bool{}
		for _, p := range e.Params {
			if id, ok := p.(*syntax.Ident); ok {
				scope[id.Name] = true
			}
		}
		nested := append(locals, scope)
		w.walkExpr(e.Body, nested)
	case *syntax.CondExpr:
		w.walkExpr(e.Cond, locals)
		w.walkExpr(e.True, locals)
		w.walkExpr(e.False, locals)
	case *syntax.ParenExpr:
		w.walkExpr(e.X, locals)
	}
}
```

### Step 4: Replace the Task 1 stub in service.go

```go
// Diagnose reports parse errors + dangling load() paths + unresolved
// identifier references in the source.
func (s *Service) Diagnose(_ context.Context, req *connect.Request[starlarkpb.DiagnoseRequest]) (*connect.Response[starlarkpb.DiagnoseResponse], error) {
    diags := Diagnose(req.Msg.FilePath, req.Msg.Source, s.symbols, predeclaredNames())
    out := make([]*starlarkpb.Diagnostic, len(diags))
    for i, d := range diags {
        out[i] = &starlarkpb.Diagnostic{
            StartLine: d.StartLine,
            StartCol:  d.StartCol,
            EndLine:   d.EndLine,
            EndCol:    d.EndCol,
            Severity:  d.Severity,
            Message:   d.Message,
            Code:      d.Code,
        }
    }
    return connect.NewResponse(&starlarkpb.DiagnoseResponse{Diagnostics: out}), nil
}
```

### Step 5: Run tests to verify PASS

```bash
go test ./internal/starlarkls -count=1 -v
```

Expected: all tests pass (existing tokenize/complete/hover/lookup tests + the new diagnose tests).

If a specific case fails (e.g., comprehension scope), debug the AST walker — possible mismatches: `syntax.ForClause` field names, `Module` vs `ModulePath`, `Ident.NamePos` field. Adjust to match go.starlark.net's actual API by reading `go.starlark.net/syntax` types directly (`go doc go.starlark.net/syntax`).

### Step 6: Commit

```bash
git add internal/starlarkls/diagnose.go internal/starlarkls/diagnose_test.go internal/starlarkls/service.go
git commit -m "feat(starlarkls): Diagnose RPC + AST walker for parse/load/unresolved"
```

## Task 4 — TS client `app/src/data/starlark-ls.ts`

**Files:**
- Create: `app/src/data/starlark-ls.ts`

### Step 1: Write the client

Mirror existing clients (e.g. `app/src/data/scenes.ts`). All five RPCs:

```ts
/**
 * StarlarkLsService client. Wraps tokenize/complete/hover/lookupSymbol/diagnose
 * for the Monaco editor providers.
 */

import { rpcCall, type RpcOptions } from "./rpc";

const SVC = "switchyard.starlarkls.v1.StarlarkLsService";

// ---- shared types --------------------------------------------------

export interface TokenSpan {
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  tokenType: string;
}

export interface CompletionItem {
  label: string;
  kind: string;      // "function" | "variable" | "keyword"
  detail: string;
  insertText: string;
}

export interface Diagnostic {
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  severity: "error" | "warning";
  message: string;
  code: string;
}

// ---- decoders (snake/camel interop) --------------------------------

interface RawTokenSpan { start_line?: number; startLine?: number; start_col?: number; startCol?: number; end_line?: number; endLine?: number; end_col?: number; endCol?: number; token_type?: string; tokenType?: string }
interface RawCompletionItem { label?: string; kind?: string; detail?: string; insert_text?: string; insertText?: string }
interface RawDiagnostic { start_line?: number; startLine?: number; start_col?: number; startCol?: number; end_line?: number; endLine?: number; end_col?: number; endCol?: number; severity?: string; message?: string; code?: string }

function decodeTokenSpan(r: RawTokenSpan): TokenSpan {
  return {
    startLine: r.startLine ?? r.start_line ?? 0,
    startCol:  r.startCol  ?? r.start_col  ?? 0,
    endLine:   r.endLine   ?? r.end_line   ?? 0,
    endCol:    r.endCol    ?? r.end_col    ?? 0,
    tokenType: r.tokenType ?? r.token_type ?? "",
  };
}
function decodeCompletion(r: RawCompletionItem): CompletionItem {
  return {
    label:      r.label ?? "",
    kind:       r.kind  ?? "",
    detail:     r.detail ?? "",
    insertText: r.insertText ?? r.insert_text ?? r.label ?? "",
  };
}
function decodeDiagnostic(r: RawDiagnostic): Diagnostic {
  const severity = (r.severity ?? "warning") as "error" | "warning";
  return {
    startLine: r.startLine ?? r.start_line ?? 1,
    startCol:  r.startCol  ?? r.start_col  ?? 0,
    endLine:   r.endLine   ?? r.end_line   ?? 1,
    endCol:    r.endCol    ?? r.end_col    ?? 1,
    severity,
    message: r.message ?? "",
    code: r.code ?? "",
  };
}

// ---- RPC wrappers --------------------------------------------------

export async function tokenize(
  req: { filePath: string; source: string },
  opts: RpcOptions = {},
): Promise<{ spans: TokenSpan[] }> {
  const res = await rpcCall<unknown, { spans?: RawTokenSpan[] }>(
    `${SVC}/Tokenize`,
    { filePath: req.filePath, source: req.source },
    opts,
  );
  return { spans: (res.spans ?? []).map(decodeTokenSpan) };
}

export async function complete(
  req: { filePath: string; source: string; line: number; col: number },
  opts: RpcOptions = {},
): Promise<{ items: CompletionItem[] }> {
  const res = await rpcCall<unknown, { items?: RawCompletionItem[] }>(
    `${SVC}/Complete`,
    { filePath: req.filePath, source: req.source, line: req.line, col: req.col },
    opts,
  );
  return { items: (res.items ?? []).map(decodeCompletion) };
}

export async function hover(
  req: { filePath: string; source: string; line: number; col: number },
  opts: RpcOptions = {},
): Promise<{ markdown: string }> {
  const res = await rpcCall<unknown, { markdown?: string }>(
    `${SVC}/Hover`,
    { filePath: req.filePath, source: req.source, line: req.line, col: req.col },
    opts,
  );
  return { markdown: res.markdown ?? "" };
}

export async function lookupSymbol(
  req: { name: string },
  opts: RpcOptions = {},
): Promise<{ filePath: string; line: number; kind: string; doc: string }> {
  const res = await rpcCall<unknown, { file_path?: string; filePath?: string; line?: number; kind?: string; doc?: string }>(
    `${SVC}/LookupSymbol`,
    { name: req.name },
    opts,
  );
  return {
    filePath: res.filePath ?? res.file_path ?? "",
    line: res.line ?? 0,
    kind: res.kind ?? "",
    doc: res.doc ?? "",
  };
}

export async function diagnose(
  req: { filePath: string; source: string },
  opts: RpcOptions = {},
): Promise<{ diagnostics: Diagnostic[] }> {
  const res = await rpcCall<unknown, { diagnostics?: RawDiagnostic[] }>(
    `${SVC}/Diagnose`,
    { filePath: req.filePath, source: req.source },
    opts,
  );
  return { diagnostics: (res.diagnostics ?? []).map(decodeDiagnostic) };
}
```

### Step 2: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

Expected: PASS (modulo pre-existing TS6310).

### Step 3: Commit

```bash
cd /Users/fdatoo/Developer/Switchyard
git add app/src/data/starlark-ls.ts
git commit -m "feat(app): starlark-ls TS client (5 RPCs)"
```

## Task 5 — Monarch grammar + language config

**Files:**
- Create: `app/src/lib/components/code-editor/starlark-grammar.ts`

### Step 1: Write the grammar

Clone Python's structure (Monaco's built-in Python grammar is a useful reference but isn't directly importable — write a small custom Monarch tokenizer matching Starlark's syntax). Keep it small; deep coloring comes from the server via SemanticTokens.

```ts
import * as monaco from "monaco-editor";

export const starlarkLanguageId = "starlark";

export const starlarkLanguageConfig: monaco.languages.LanguageConfiguration = {
  comments: { lineComment: "#" },
  brackets: [
    ["(", ")"],
    ["[", "]"],
    ["{", "}"],
  ],
  autoClosingPairs: [
    { open: "(", close: ")" },
    { open: "[", close: "]" },
    { open: "{", close: "}" },
    { open: '"', close: '"', notIn: ["string"] },
    { open: "'", close: "'", notIn: ["string"] },
  ],
  surroundingPairs: [
    { open: "(", close: ")" },
    { open: "[", close: "]" },
    { open: "{", close: "}" },
    { open: '"', close: '"' },
    { open: "'", close: "'" },
  ],
  wordPattern: /(-?\d*\.\d\w*)|([^\`~!@#%^&*()\-=+\[{\]}\\|;:'",.<>\/?\s]+)/,
};

const KEYWORDS = [
  "def", "if", "elif", "else", "for", "in", "while", "return",
  "and", "or", "not", "True", "False", "None",
  "load", "lambda", "pass", "break", "continue",
];

const BUILTINS = [
  "print", "len", "range", "list", "dict", "tuple", "set",
  "str", "int", "float", "bool", "type",
  "state", "now", "time", "log", "call_service", "random",
  "sleep", "notify", "scene", "event",
];

export const starlarkMonarchTokens: monaco.languages.IMonarchLanguage = {
  defaultToken: "",
  tokenPostfix: ".star",

  keywords: KEYWORDS,
  builtins: BUILTINS,

  brackets: [
    { open: "(", close: ")", token: "delimiter.parenthesis" },
    { open: "[", close: "]", token: "delimiter.square" },
    { open: "{", close: "}", token: "delimiter.curly" },
  ],

  tokenizer: {
    root: [
      // Comments
      [/#.*$/, "comment"],

      // Triple-quoted strings
      [/"""/, { token: "string.quote", bracket: "@open", next: "@tripleString" }],
      [/'''/, { token: "string.quote", bracket: "@open", next: "@tripleStringSingle" }],

      // Strings
      [/"([^"\\]|\\.)*$/, "string.invalid"],
      [/'([^'\\]|\\.)*$/, "string.invalid"],
      [/"/, { token: "string.quote", bracket: "@open", next: "@string" }],
      [/'/, { token: "string.quote", bracket: "@open", next: "@stringSingle" }],

      // Numbers
      [/0[xX][0-9a-fA-F]+/, "number.hex"],
      [/0[oO][0-7]+/, "number.octal"],
      [/0[bB][01]+/, "number.binary"],
      [/\d+(\.\d+)?([eE][+-]?\d+)?/, "number"],

      // Identifiers + keywords
      [/[A-Za-z_][A-Za-z0-9_]*/, {
        cases: {
          "@keywords": "keyword",
          "@builtins": "type.identifier",  // colored differently from regular ids
          "@default": "identifier",
        },
      }],

      // Operators
      [/[=+\-*/%<>!]+/, "operator"],
      [/[(){}\[\]]/, "@brackets"],
      [/[,;]/, "delimiter"],

      // Whitespace
      [/\s+/, "white"],
    ],

    string: [
      [/[^\\"]+/, "string"],
      [/\\./, "string.escape"],
      [/"/, { token: "string.quote", bracket: "@close", next: "@pop" }],
    ],
    stringSingle: [
      [/[^\\']+/, "string"],
      [/\\./, "string.escape"],
      [/'/, { token: "string.quote", bracket: "@close", next: "@pop" }],
    ],
    tripleString: [
      [/[^"]+/, "string"],
      [/"""/, { token: "string.quote", bracket: "@close", next: "@pop" }],
      [/"/, "string"],
    ],
    tripleStringSingle: [
      [/[^']+/, "string"],
      [/'''/, { token: "string.quote", bracket: "@close", next: "@pop" }],
      [/'/, "string"],
    ],
  },
};
```

### Step 2: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

### Step 3: Commit

```bash
git add app/src/lib/components/code-editor/starlark-grammar.ts
git commit -m "feat(app): Starlark Monarch grammar + language config"
```

## Task 6 — Monaco providers + diagnostics

**Files:**
- Create: `app/src/lib/components/code-editor/starlark-providers.ts`

### Step 1: Write the providers module

```ts
import * as monaco from "monaco-editor";
import {
  tokenize, complete, hover, lookupSymbol, diagnose,
  type CompletionItem as DaemonCompletion,
  type Diagnostic as DaemonDiagnostic,
} from "@/data/starlark-ls";

let registered = false;

/** Register all five Monaco providers + diagnostics arming.
 *  Idempotent — safe to call multiple times. */
export function setupStarlarkProviders(): void {
  if (registered) return;
  registered = true;

  monaco.languages.registerCompletionItemProvider("starlark", {
    triggerCharacters: ["."],
    provideCompletionItems: async (model, position) => {
      try {
        const r = await complete({
          filePath: model.uri.path,
          source: model.getValue(),
          line: position.lineNumber,
          col: position.column - 1, // daemon 0-based col
        });
        const word = model.getWordUntilPosition(position);
        const range: monaco.IRange = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };
        return {
          suggestions: r.items.map((it: DaemonCompletion) => ({
            label: it.label,
            kind: completionKindOf(it.kind),
            detail: it.detail,
            insertText: it.insertText || it.label,
            range,
          })),
        };
      } catch {
        return { suggestions: [] };
      }
    },
  });

  monaco.languages.registerHoverProvider("starlark", {
    provideHover: async (model, position) => {
      try {
        const r = await hover({
          filePath: model.uri.path,
          source: model.getValue(),
          line: position.lineNumber,
          col: position.column - 1,
        });
        if (!r.markdown) return null;
        const word = model.getWordAtPosition(position);
        const range: monaco.IRange | undefined = word ? {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        } : undefined;
        return { contents: [{ value: r.markdown }], range };
      } catch {
        return null;
      }
    },
  });

  monaco.languages.registerDefinitionProvider("starlark", {
    provideDefinition: async (model, position) => {
      const word = model.getWordAtPosition(position);
      if (!word) return null;
      try {
        const r = await lookupSymbol({ name: word.word });
        if (!r.filePath) return null;
        window.dispatchEvent(new CustomEvent("starlark-goto-definition", {
          detail: { filePath: r.filePath, line: r.line },
        }));
      } catch { /* silently ignore */ }
      return null; // we handle the navigation ourselves via the custom event
    },
  });

  monaco.languages.registerDocumentSemanticTokensProvider("starlark", {
    getLegend: () => ({
      tokenTypes: ["keyword", "identifier", "string", "number", "comment", "operator"],
      tokenModifiers: [],
    }),
    provideDocumentSemanticTokens: async (model) => {
      try {
        const r = await tokenize({
          filePath: model.uri.path,
          source: model.getValue(),
        });
        return {
          data: encodeSemanticTokens(r.spans),
          resultId: undefined,
        };
      } catch {
        return { data: new Uint32Array(0), resultId: undefined };
      }
    },
    releaseDocumentSemanticTokens: () => {},
  });

  // Diagnostics: arm any current or future "starlark" models.
  for (const m of monaco.editor.getModels()) {
    if (m.getLanguageId() === "starlark") armDiagnostics(m);
  }
  monaco.editor.onDidCreateModel((m) => {
    if (m.getLanguageId() === "starlark") armDiagnostics(m);
  });
}

function completionKindOf(kind: string): monaco.languages.CompletionItemKind {
  switch (kind) {
    case "function": return monaco.languages.CompletionItemKind.Function;
    case "variable": return monaco.languages.CompletionItemKind.Variable;
    case "keyword":  return monaco.languages.CompletionItemKind.Keyword;
    default:         return monaco.languages.CompletionItemKind.Text;
  }
}

const TOKEN_TYPE_INDEX: Record<string, number> = {
  keyword: 0, identifier: 1, string: 2, number: 3, comment: 4, operator: 5,
};

/** Encode TokenSpans into the LSP semantic-tokens uint32 wire format:
 *  [deltaLine, deltaCol, length, tokenTypeIdx, tokenModifiersBitmask] per token. */
function encodeSemanticTokens(spans: { startLine: number; startCol: number; endLine: number; endCol: number; tokenType: string }[]): Uint32Array {
  // Sort by position (server is expected to but be defensive).
  const sorted = [...spans].sort((a, b) => a.startLine - b.startLine || a.startCol - b.startCol);
  const out: number[] = [];
  let prevLine = 0;
  let prevCol = 0;
  for (const s of sorted) {
    const line = s.startLine - 1;   // Monaco semantic tokens use 0-based lines
    const col = s.startCol;          // already 0-based from daemon
    const length = s.endLine === s.startLine ? Math.max(1, s.endCol - s.startCol) : 1;
    const typeIdx = TOKEN_TYPE_INDEX[s.tokenType] ?? 1;
    const dLine = line - prevLine;
    const dCol = dLine === 0 ? col - prevCol : col;
    out.push(dLine, dCol, length, typeIdx, 0);
    prevLine = line;
    prevCol = col;
  }
  return new Uint32Array(out);
}

function armDiagnostics(model: monaco.editor.ITextModel): void {
  let timer: number | null = null;
  const fire = (): void => {
    if (timer !== null) window.clearTimeout(timer);
    timer = window.setTimeout(async () => {
      try {
        const r = await diagnose({ filePath: model.uri.path, source: model.getValue() });
        monaco.editor.setModelMarkers(model, "starlark", r.diagnostics.map(diagToMonaco));
      } catch {
        monaco.editor.setModelMarkers(model, "starlark", []);
      }
    }, 300);
  };
  model.onDidChangeContent(fire);
  fire();
}

function diagToMonaco(d: DaemonDiagnostic): monaco.editor.IMarkerData {
  return {
    severity: d.severity === "error"
      ? monaco.MarkerSeverity.Error
      : monaco.MarkerSeverity.Warning,
    message: d.message,
    code: d.code,
    startLineNumber: d.startLine,
    startColumn: d.startCol + 1, // Monaco columns are 1-based
    endLineNumber: d.endLine,
    endColumn: d.endCol + 1,
  };
}
```

### Step 2: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

### Step 3: Commit

```bash
git add app/src/lib/components/code-editor/starlark-providers.ts
git commit -m "feat(app): Monaco providers (5) + diagnostics for Starlark"
```

## Task 7 — SyCodeEditor registers Starlark

**Files:**
- Modify: `app/src/lib/components/code-editor/SyCodeEditor.vue`

### Step 1: Accept "starlark" in the language union + register

Edit `app/src/lib/components/code-editor/SyCodeEditor.vue`:

1. Import the new modules at the top of `<script setup>`:

```ts
import { starlarkLanguageId, starlarkLanguageConfig, starlarkMonarchTokens } from "./starlark-grammar";
import { setupStarlarkProviders } from "./starlark-providers";
```

2. Update the `language` prop type:

```ts
const props = defineProps<{
  modelValue: string;
  language: "pkl" | "python" | "starlark";
  readonly?: boolean;
  filename?: string;
}>();
```

3. Add an idempotent registration sibling to `ensurePklRegistered`:

```ts
let starlarkRegistered = false;
function ensureStarlarkRegistered(): void {
  if (starlarkRegistered) return;
  starlarkRegistered = true;
  monaco.languages.register({ id: starlarkLanguageId, extensions: [".star"], aliases: ["Starlark", "starlark"] });
  monaco.languages.setLanguageConfiguration(starlarkLanguageId, starlarkLanguageConfig);
  monaco.languages.setMonarchTokensProvider(starlarkLanguageId, starlarkMonarchTokens);
  setupStarlarkProviders();
}
```

4. In `onMounted`, register Starlark whenever the prop requests it:

```ts
onMounted(() => {
  if (!hostEl.value) return;
  if (props.language === "pkl") ensurePklRegistered();
  if (props.language === "starlark") ensureStarlarkRegistered();
  editor = monaco.editor.create(hostEl.value, {
    value: props.modelValue,
    language: props.language,
    readOnly: props.readonly ?? false,
    automaticLayout: true,
    minimap: { enabled: false },
    fontSize: 13,
    scrollBeyondLastLine: false,
    tabSize: 2,
    "semanticHighlighting.enabled": true, // enables our DocumentSemanticTokensProvider
  } as monaco.editor.IStandaloneEditorConstructionOptions);
  editor.onDidChangeModelContent(() => {
    const v = editor?.getValue() ?? "";
    if (v !== props.modelValue) emit("update:modelValue", v);
  });
});
```

5. Update the language-change watcher so a swap from `pkl` → `starlark` registers Starlark on the fly:

```ts
watch(() => props.language, (lang) => {
  if (!editor) return;
  if (lang === "starlark") ensureStarlarkRegistered();
  const model = editor.getModel();
  if (model) monaco.editor.setModelLanguage(model, lang);
});
```

### Step 2: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

Expected: clean.

### Step 3: Commit

```bash
git add app/src/lib/components/code-editor/SyCodeEditor.vue
git commit -m "feat(app): SyCodeEditor registers Starlark language + providers"
```

## Task 8 — SyCodeEditorPanel uses "starlark" + handles goto-def event

**Files:**
- Modify: `app/src/lib/components/code-editor-panel/SyCodeEditorPanel.vue`

### Step 1: Language computed

Find line 42 (the `language` computed). Change:

```ts
const language = computed<"pkl" | "starlark">(() => props.kind === "pkl" ? "pkl" : "starlark");
```

(Was `"python"`; now `"starlark"`.)

### Step 2: Listen for goto-definition events

Add a handler that calls `openFile()` + positions the cursor:

In the existing `<script setup>` near the bottom, after `onMounted` exists (or add one):

```ts
import { onMounted, onBeforeUnmount } from "vue"; // ensure these are imported

// at top-level script-setup:
let editorInstance: { revealLineInCenter?: (line: number) => void; setPosition?: (p: { lineNumber: number; column: number }) => void } | null = null;

function onGotoDefinition(e: Event): void {
  const detail = (e as CustomEvent).detail as { filePath: string; line: number };
  if (!detail) return;
  void openFile(detail.filePath).then(() => {
    // openFile() loads buffer; SyCodeEditor's reactive setValue runs.
    // We then have to reach into Monaco to position the cursor. The
    // editor instance isn't directly exposed by SyCodeEditor's current
    // API; the simplest workaround is to set `pendingCursorLine` state
    // that the editor sees on next render, or grow SyCodeEditor with a
    // `setPosition(line)` imperative API exposed via defineExpose.
    //
    // For v1, just store the target line and let the user scroll. A
    // follow-up task can plumb the imperative API.
    pendingCursorLine.value = detail.line;
  });
}

const pendingCursorLine = ref<number | null>(null);

onMounted(() => {
  window.addEventListener("starlark-goto-definition", onGotoDefinition as EventListener);
});

onBeforeUnmount(() => {
  window.removeEventListener("starlark-goto-definition", onGotoDefinition as EventListener);
});
```

The cursor-positioning is intentionally minimal in v1. To go further, `SyCodeEditor.vue` would need to `defineExpose({ setPosition })` and `SyCodeEditorPanel` would call it after `openFile` settles. That's a refinement; v1 just opens the file.

### Step 3: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

### Step 4: Commit

```bash
git add app/src/lib/components/code-editor-panel/SyCodeEditorPanel.vue
git commit -m "feat(app): SyCodeEditorPanel uses starlark language + listens for goto-def"
```

## Task 9 — Validation pass

Rebuild + restart the daemon, then drive the editor end-to-end.

### Step 1: Rebuild + restart daemon

```bash
go build -o dist/switchyardd ./cmd/switchyardd
lsof -ti:8080 2>/dev/null | xargs -I {} kill {}
rm -f /Users/fdatoo/.local/share/switchyard/switchyardd.lock
./dist/switchyardd &
sleep 4
test -e /Users/fdatoo/.local/share/switchyard/switchyardd.sock && echo "daemon up"
```

### Step 2: Run the full test suite

```bash
go test ./... -count=1
cd app && npm run typecheck
```

Expected: all green (modulo pre-existing TS6310).

### Step 3: Manual / Playwright drive

Find the route that hosts `SyCodeEditorPanel kind="starlark"`. Likely `/settings/starlark` — check `app/src/router/`.

Open the page. Open an existing `.star` file from the file tree (e.g. one from `<configDir>/scripts/`). Verify, in order:

1. **Tokens load:** the editor briefly shows Monarch-fallback colors, then within ~300ms the server's semantic tokens land and (if they differ from the local grammar) update the coloring.
2. **Diagnostics:** type a typo identifier like `prnt(1)`. After 300ms a yellow warning underline appears with `unresolved name "prnt"`. Fix it to `print(1)` — warning clears.
3. **Hover:** mouse over a function defined elsewhere (or a builtin like `print`). Tooltip appears with markdown.
4. **Completion:** type a prefix that matches a known symbol. Press Ctrl+Space. Dropdown lists symbols.
5. **Go-to-definition:** ⌘-click on a function call whose definition lives in another `.star` file. The editor switches to that file. (Cursor positioning is a v2 follow-up per Task 8 step 2.)
6. **Bad load:** edit the file to add `load("nonexistent.star", "x")`. After debounce, a red error underline appears on the path string with `load: file "nonexistent.star" not found in scripts directory`.

If any step fails, debug per `superpowers:systematic-debugging`. Don't lower acceptance bars.

### Step 4: Commit a validation note

If validation passes, optionally append to the progress log (if you maintain one). No code commit needed unless something broke and needed fixing.

## Wave plan for subagent-driven execution

| Wave | Tasks | Notes |
|------|-------|-------|
| 0 | 1 | Proto + regen — gates everything daemon-side |
| 1 | 2 | Predeclared name list (file-disjoint from rest) |
| 2 | 3 | Diagnose AST walker — depends on 1 + 2 |
| 3 | 4 | TS client — depends only on proto (Wave 0) |
| 4 | 5 | Monarch grammar — disjoint from 4; can run in parallel |
| 5 | 6 | Monaco providers — depends on 4 + 5 |
| 6 | 7 | SyCodeEditor registration — depends on 5 + 6 |
| 7 | 8 | SyCodeEditorPanel — depends on 7 |
| 8 | 9 | Validation |

Total: 9 tasks, with Waves 3-4 parallelizable.
