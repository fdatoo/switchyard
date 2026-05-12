package starlarkls

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	starlarkpb "github.com/fdatoo/switchyard/gen/switchyard/starlarkls/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/starlarkls/v1/starlarklsv1connect"
)

var _ starlarklsv1connect.StarlarkLsServiceHandler = (*Service)(nil)

// Service implements the StarlarkLsService Connect-RPC handler.
type Service struct {
	symbols map[string]SymbolInfo
	dir     string
}

// NewService creates a new StarlarkLsService with the given pre-extracted symbol map.
func NewService(symbols map[string]SymbolInfo, scriptsDir string) *Service {
	return &Service{symbols: symbols, dir: scriptsDir}
}

// Diagnose reports parse errors, dangling load() paths, and unresolved
// identifier references in the source.
func (s *Service) Diagnose(_ context.Context, req *connect.Request[starlarkpb.DiagnoseRequest]) (*connect.Response[starlarkpb.DiagnoseResponse], error) {
	diags := Diagnose(req.Msg.FilePath, req.Msg.Source, s.symbols, predeclaredNames())
	out := make([]*starlarkpb.Diagnostic, len(diags))
	for i, diag := range diags {
		out[i] = diag.toProto()
	}
	return connect.NewResponse(&starlarkpb.DiagnoseResponse{Diagnostics: out}), nil
}

// Tokenize returns token spans by scanning for known Starlark keywords.
func (s *Service) Tokenize(_ context.Context, req *connect.Request[starlarkpb.TokenizeRequest]) (*connect.Response[starlarkpb.TokenizeResponse], error) {
	keywords := []string{
		"def", "if", "else", "elif", "for", "in", "return", "and", "or",
		"not", "True", "False", "None", "load", "lambda", "pass", "break", "continue",
	}
	var spans []*starlarkpb.TokenSpan
	for lineIdx, line := range strings.Split(req.Msg.Source, "\n") {
		for _, kw := range keywords {
			col := strings.Index(line, kw)
			if col < 0 {
				continue
			}
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

// Complete returns completion items whose names match the prefix at the cursor.
func (s *Service) Complete(_ context.Context, req *connect.Request[starlarkpb.CompleteRequest]) (*connect.Response[starlarkpb.CompleteResponse], error) {
	lines := strings.Split(req.Msg.Source, "\n")
	col := int(req.Msg.Col)
	lineIdx := int(req.Msg.Line) - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		return connect.NewResponse(&starlarkpb.CompleteResponse{}), nil
	}
	line := lines[lineIdx]
	if col > len(line) {
		col = len(line)
	}
	prefix := wordBefore(line[:col])

	var items []*starlarkpb.CompletionItem
	for name, sym := range s.symbols {
		if strings.HasPrefix(name, prefix) {
			items = append(items, &starlarkpb.CompletionItem{
				Label:      name,
				Kind:       sym.Kind,
				Detail:     sym.Doc,
				InsertText: name,
			})
		}
	}
	return connect.NewResponse(&starlarkpb.CompleteResponse{Items: items}), nil
}

// Hover returns markdown documentation for the symbol under the cursor.
func (s *Service) Hover(_ context.Context, req *connect.Request[starlarkpb.HoverRequest]) (*connect.Response[starlarkpb.HoverResponse], error) {
	lines := strings.Split(req.Msg.Source, "\n")
	lineIdx := int(req.Msg.Line) - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		return connect.NewResponse(&starlarkpb.HoverResponse{}), nil
	}
	word := wordAt(lines[lineIdx], int(req.Msg.Col))
	sym, ok := s.symbols[word]
	if !ok {
		return connect.NewResponse(&starlarkpb.HoverResponse{}), nil
	}
	md := fmt.Sprintf("**%s** `%s`", sym.Kind, word)
	if sym.Doc != "" {
		md += "\n\n" + sym.Doc
	}
	return connect.NewResponse(&starlarkpb.HoverResponse{Markdown: md}), nil
}

// LookupSymbol returns the definition location for the named symbol.
func (s *Service) LookupSymbol(_ context.Context, req *connect.Request[starlarkpb.LookupSymbolRequest]) (*connect.Response[starlarkpb.LookupSymbolResponse], error) {
	sym, ok := s.symbols[req.Msg.Name]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("symbol %q not found", req.Msg.Name))
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
	for i > 0 && isIdent(s[i-1]) {
		i--
	}
	return s[i:]
}

// wordAt returns the identifier-like token around column col (0-based).
func wordAt(line string, col int) string {
	if col > len(line) {
		col = len(line)
	}
	start := col
	for start > 0 && isIdent(line[start-1]) {
		start--
	}
	end := col
	for end < len(line) && isIdent(line[end]) {
		end++
	}
	return line[start:end]
}

func isIdent(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
