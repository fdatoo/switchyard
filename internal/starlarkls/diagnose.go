package starlarkls

import (
	"fmt"
	"path/filepath"

	starlarkpb "github.com/fdatoo/switchyard/gen/switchyard/starlarkls/v1"
	"go.starlark.net/syntax"
)

// Diagnostic is the daemon-internal representation of a Starlark editor
// diagnostic.
type Diagnostic struct {
	StartLine int32
	StartCol  int32
	EndLine   int32
	EndCol    int32
	Severity  string
	Message   string
	Code      string
}

func (d Diagnostic) toProto() *starlarkpb.Diagnostic {
	return &starlarkpb.Diagnostic{
		StartLine: d.StartLine,
		StartCol:  d.StartCol,
		EndLine:   d.EndLine,
		EndCol:    d.EndCol,
		Severity:  d.Severity,
		Message:   d.Message,
		Code:      d.Code,
	}
}

// Diagnose parses source and reports parse errors, dangling load() targets,
// and unresolved identifier references.
func Diagnose(filePath, source string, symbols map[string]SymbolInfo, predeclared map[string]bool) []Diagnostic {
	opts := syntax.LegacyFileOptions()
	f, err := opts.Parse(filePath, []byte(source), 0)
	if err != nil {
		return []Diagnostic{parseDiagnostic(err)}
	}

	topLevel := map[string]bool{}
	loadedNames := map[string]bool{}
	var loads []*syntax.LoadStmt

	for _, stmt := range f.Stmts {
		switch s := stmt.(type) {
		case *syntax.DefStmt:
			topLevel[s.Name.Name] = true
		case *syntax.AssignStmt:
			collectAssignTargets(s.LHS, topLevel)
		case *syntax.LoadStmt:
			loads = append(loads, s)
			for i, name := range s.From {
				binding := name
				if i < len(s.To) {
					binding = s.To[i]
				}
				loadedNames[binding.Name] = true
			}
		}
	}

	var diags []Diagnostic
	loadablePaths := loadableSymbolPaths(symbols)
	for _, load := range loads {
		path := load.ModuleName()
		if !loadablePaths[path] && !loadablePaths[filepath.Base(path)] {
			diags = append(diags, Diagnostic{
				StartLine: int32(load.Module.TokenPos.Line),
				StartCol:  int32(load.Module.TokenPos.Col - 1),
				EndLine:   int32(load.Module.TokenPos.Line),
				EndCol:    int32(load.Module.TokenPos.Col-1) + int32(len(load.Module.Raw)),
				Severity:  "error",
				Message:   fmt.Sprintf("load: file %q not found in scripts directory", path),
				Code:      "load_not_found",
			})
		}
	}

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

func parseDiagnostic(err error) Diagnostic {
	if se, ok := err.(syntax.Error); ok {
		return Diagnostic{
			StartLine: int32(se.Pos.Line),
			StartCol:  int32(max(se.Pos.Col-1, 0)),
			EndLine:   int32(se.Pos.Line),
			EndCol:    int32(max(se.Pos.Col, 1)),
			Severity:  "error",
			Message:   se.Msg,
			Code:      "parse_error",
		}
	}
	return Diagnostic{
		StartLine: 1,
		StartCol:  0,
		EndLine:   1,
		EndCol:    1,
		Severity:  "error",
		Message:   err.Error(),
		Code:      "parse_error",
	}
}

func loadableSymbolPaths(symbols map[string]SymbolInfo) map[string]bool {
	out := map[string]bool{}
	for _, sym := range symbols {
		if sym.File == "" {
			continue
		}
		out[sym.File] = true
		out[filepath.Base(sym.File)] = true
	}
	return out
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
	}
}

func collectParam(expr syntax.Expr, out map[string]bool) {
	switch e := expr.(type) {
	case *syntax.Ident:
		out[e.Name] = true
	case *syntax.BinaryExpr:
		collectParam(e.X, out)
	case *syntax.UnaryExpr:
		if e.X != nil {
			collectParam(e.X, out)
		}
	}
}

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
		EndCol:    int32(id.NamePos.Col-1) + int32(len(id.Name)),
		Severity:  "warning",
		Message:   fmt.Sprintf("unresolved name %q", id.Name),
		Code:      "unresolved_name",
	})
}

func (w *refWalker) walkStmt(stmt syntax.Stmt, locals []map[string]bool) {
	switch s := stmt.(type) {
	case *syntax.DefStmt:
		scope := map[string]bool{}
		for _, p := range s.Params {
			collectParam(p, scope)
			w.walkParamDefault(p, locals)
		}
		collectBodyBindings(s.Body, scope)
		nested := append(locals, scope)
		for _, bs := range s.Body {
			w.walkStmt(bs, nested)
		}
	case *syntax.ExprStmt:
		w.walkExpr(s.X, locals)
	case *syntax.AssignStmt:
		w.walkExpr(s.RHS, locals)
	case *syntax.ReturnStmt:
		w.walkExpr(s.Result, locals)
	case *syntax.IfStmt:
		w.walkExpr(s.Cond, locals)
		for _, bs := range s.True {
			w.walkStmt(bs, locals)
		}
		for _, bs := range s.False {
			w.walkStmt(bs, locals)
		}
	case *syntax.ForStmt:
		scope := map[string]bool{}
		collectAssignTargets(s.Vars, scope)
		nested := append(locals, scope)
		w.walkExpr(s.X, locals)
		for _, bs := range s.Body {
			w.walkStmt(bs, nested)
		}
	case *syntax.WhileStmt:
		w.walkExpr(s.Cond, locals)
		for _, bs := range s.Body {
			w.walkStmt(bs, locals)
		}
	case *syntax.BranchStmt, *syntax.LoadStmt:
	}
}

func collectBodyBindings(stmts []syntax.Stmt, out map[string]bool) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *syntax.AssignStmt:
			collectAssignTargets(s.LHS, out)
		case *syntax.DefStmt:
			out[s.Name.Name] = true
		}
	}
}

func (w *refWalker) walkParamDefault(expr syntax.Expr, locals []map[string]bool) {
	if be, ok := expr.(*syntax.BinaryExpr); ok {
		w.walkExpr(be.Y, locals)
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
	case *syntax.UnaryExpr:
		w.walkExpr(e.X, locals)
	case *syntax.BinaryExpr:
		w.walkExpr(e.X, locals)
		w.walkExpr(e.Y, locals)
	case *syntax.CallExpr:
		w.walkExpr(e.Fn, locals)
		for _, arg := range e.Args {
			w.walkCallArg(arg, locals)
		}
	case *syntax.DotExpr:
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
		w.walkComprehension(e, locals)
	case *syntax.LambdaExpr:
		scope := map[string]bool{}
		for _, p := range e.Params {
			collectParam(p, scope)
			w.walkParamDefault(p, locals)
		}
		w.walkExpr(e.Body, append(locals, scope))
	case *syntax.CondExpr:
		w.walkExpr(e.Cond, locals)
		w.walkExpr(e.True, locals)
		w.walkExpr(e.False, locals)
	case *syntax.ParenExpr:
		w.walkExpr(e.X, locals)
	}
}

func (w *refWalker) walkCallArg(expr syntax.Expr, locals []map[string]bool) {
	switch e := expr.(type) {
	case *syntax.BinaryExpr:
		w.walkExpr(e.Y, locals)
	case *syntax.UnaryExpr:
		w.walkExpr(e.X, locals)
	default:
		w.walkExpr(expr, locals)
	}
}

func (w *refWalker) walkComprehension(expr *syntax.Comprehension, locals []map[string]bool) {
	scope := map[string]bool{}
	nested := append(locals, scope)
	for _, clause := range expr.Clauses {
		switch c := clause.(type) {
		case *syntax.ForClause:
			w.walkExpr(c.X, nested[:len(nested)-1])
			collectAssignTargets(c.Vars, scope)
		case *syntax.IfClause:
			w.walkExpr(c.Cond, nested)
		}
	}
	w.walkExpr(expr.Body, nested)
}
