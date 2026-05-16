package pkllsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"connectrpc.com/connect"

	pkllsppb "github.com/fdatoo/switchyard/gen/switchyard/pkllsp/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/pkllsp/v1/pkllspv1connect"
)

var _ pkllspv1connect.PklLsServiceHandler = (*Service)(nil)

type Config struct {
	BinaryPath             string
	ConfigDir              string
	SwitchyardNamespaceDir string
	Logger                 *slog.Logger
}

type Service struct {
	cfg Config

	mu     sync.Mutex
	client *client
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	c := s.client
	s.client = nil
	s.mu.Unlock()
	if c == nil {
		return nil
	}
	return c.close(ctx)
}

func (s *Service) Complete(ctx context.Context, req *connect.Request[pkllsppb.CompleteRequest]) (*connect.Response[pkllsppb.CompleteResponse], error) {
	c, err := s.ensureClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	uri, _, err := c.syncDocument(req.Msg.FilePath, req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	raw, err := c.request(ctx, "textDocument/completion", map[string]any{
		"textDocument": map[string]string{"uri": uri},
		"position":     lspPosition{Line: oneToZero(req.Msg.Line), Character: int32ToUint32(req.Msg.Col)},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	items := decodeCompletionItems(raw)
	out := make([]*pkllsppb.CompletionItem, 0, len(items))
	for _, item := range items {
		out = append(out, &pkllsppb.CompletionItem{
			Label:      item.Label,
			Kind:       completionKind(item.Kind),
			Detail:     item.Detail,
			InsertText: firstNonEmpty(item.InsertText, item.Label),
		})
	}
	return connect.NewResponse(&pkllsppb.CompleteResponse{Items: out}), nil
}

func (s *Service) Hover(ctx context.Context, req *connect.Request[pkllsppb.HoverRequest]) (*connect.Response[pkllsppb.HoverResponse], error) {
	c, err := s.ensureClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	uri, _, err := c.syncDocument(req.Msg.FilePath, req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	raw, err := c.request(ctx, "textDocument/hover", map[string]any{
		"textDocument": map[string]string{"uri": uri},
		"position":     lspPosition{Line: oneToZero(req.Msg.Line), Character: int32ToUint32(req.Msg.Col)},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&pkllsppb.HoverResponse{Markdown: markdownFromHover(raw)}), nil
}

func (s *Service) Definition(ctx context.Context, req *connect.Request[pkllsppb.DefinitionRequest]) (*connect.Response[pkllsppb.DefinitionResponse], error) {
	c, err := s.ensureClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	uri, _, err := c.syncDocument(req.Msg.FilePath, req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	raw, err := c.request(ctx, "textDocument/definition", map[string]any{
		"textDocument": map[string]string{"uri": uri},
		"position":     lspPosition{Line: oneToZero(req.Msg.Line), Character: int32ToUint32(req.Msg.Col)},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	loc, ok := firstLocation(raw)
	if !ok {
		return connect.NewResponse(&pkllsppb.DefinitionResponse{}), nil
	}
	return connect.NewResponse(&pkllsppb.DefinitionResponse{
		FilePath: s.editorPath(pathFromFileURI(loc.URI)),
		Line:     int32(loc.Range.Start.Line + 1),
		Col:      int32(loc.Range.Start.Character),
	}), nil
}

func (s *Service) Diagnose(ctx context.Context, req *connect.Request[pkllsppb.DiagnoseRequest]) (*connect.Response[pkllsppb.DiagnoseResponse], error) {
	c, err := s.ensureClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	uri, version, err := c.syncDocument(req.Msg.FilePath, req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	diags, err := c.waitForDiagnostics(waitCtx, uri, version)
	if err != nil {
		return nil, connect.NewError(connect.CodeDeadlineExceeded, fmt.Errorf("pkl-lsp diagnostics: %w", err))
	}
	out := make([]*pkllsppb.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, &pkllsppb.Diagnostic{
			StartLine: int32(d.Range.Start.Line + 1),
			StartCol:  int32(d.Range.Start.Character),
			EndLine:   int32(d.Range.End.Line + 1),
			EndCol:    int32(d.Range.End.Character),
			Severity:  diagnosticSeverity(d.Severity),
			Message:   d.Message,
			Code:      diagnosticCode(d.Code),
		})
	}
	return connect.NewResponse(&pkllsppb.DiagnoseResponse{Diagnostics: out}), nil
}

func (s *Service) SemanticTokens(ctx context.Context, req *connect.Request[pkllsppb.SemanticTokensRequest]) (*connect.Response[pkllsppb.SemanticTokensResponse], error) {
	c, err := s.ensureClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	uri, _, err := c.syncDocument(req.Msg.FilePath, req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	raw, err := c.request(ctx, "textDocument/semanticTokens/full", map[string]any{
		"textDocument": map[string]string{"uri": uri},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	var res struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pkllsppb.SemanticTokensResponse{Data: res.Data}), nil
}

func (s *Service) ensureClient(ctx context.Context) (*client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && !s.client.isClosed() {
		return s.client, nil
	}
	c, err := startClient(ctx, s.cfg)
	if err != nil {
		return nil, err
	}
	s.client = c
	return c, nil
}

func (s *Service) editorPath(path string) string {
	if s.cfg.ConfigDir == "" || path == "" {
		return path
	}
	absConfig, err := filepath.Abs(s.cfg.ConfigDir)
	if err != nil {
		return path
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(absConfig, absPath)
	if err != nil || rel == "." || rel == ".." || len(rel) >= 3 && rel[:3] == "../" {
		return path
	}
	return filepath.ToSlash(rel)
}

func decodeCompletionItems(raw json.RawMessage) []completionItem {
	if len(raw) == 0 {
		return nil
	}
	var list []completionItem
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var wrapped struct {
		Items []completionItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		return wrapped.Items
	}
	return nil
}

func firstLocation(raw json.RawMessage) (location, bool) {
	var loc location
	if len(raw) == 0 {
		return loc, false
	}
	if err := json.Unmarshal(raw, &loc); err == nil && loc.URI != "" {
		return loc, true
	}
	var list []location
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0], true
	}
	var links []struct {
		TargetURI            string   `json:"targetUri"`
		TargetSelectionRange lspRange `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(raw, &links); err == nil && len(links) > 0 {
		return location{URI: links[0].TargetURI, Range: links[0].TargetSelectionRange}, true
	}
	return loc, false
}

func completionKind(kind int32) string {
	switch kind {
	case 2, 3:
		return "function"
	case 5, 6, 10, 21:
		return "variable"
	case 7, 8, 22, 25:
		return "type"
	case 9, 17, 19:
		return "module"
	case 14:
		return "keyword"
	default:
		return "text"
	}
}

func diagnosticSeverity(sev int32) string {
	switch sev {
	case 1:
		return "error"
	case 2:
		return "warning"
	default:
		return "warning"
	}
}

func diagnosticCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("%d", n)
	}
	return ""
}

func oneToZero(line int32) uint32 {
	if line <= 1 {
		return 0
	}
	return uint32(line - 1)
}

func int32ToUint32(v int32) uint32 {
	if v <= 0 {
		return 0
	}
	return uint32(v)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
