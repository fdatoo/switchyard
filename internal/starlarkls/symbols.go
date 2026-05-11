// Package starlarkls provides the StarlarkLsService Connect-RPC implementation
// and supporting utilities for Starlark language intelligence features.
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
	opts := syntax.LegacyFileOptions()
	for _, path := range entries {
		f, err := opts.Parse(path, nil, syntax.RetainComments)
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
				if id, ok := s.LHS.(*syntax.Ident); ok && strings.ToUpper(id.Name) == id.Name && id.Name != "_" {
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
