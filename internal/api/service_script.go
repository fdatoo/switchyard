package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
)

// ScriptService implements Starlark script listing, execution, eval, and tests.
type ScriptService struct {
	be      ScriptRunner
	events  EventAppender
	mcpCaps MCPCapsProvider
}

// NewScriptService returns a script service backed by runner, event audit, and MCP caps.
func NewScriptService(be ScriptRunner, events EventAppender, caps MCPCapsProvider) *ScriptService {
	return &ScriptService{be: be, events: events, mcpCaps: caps}
}

var _ switchyardv1alpha1connect.ScriptServiceHandler = (*ScriptService)(nil)

// List returns invocable scripts with API pagination.
func (s *ScriptService) List(ctx context.Context, req *connect.Request[v1.ListScriptsRequest]) (*connect.Response[v1.ListScriptsResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	scripts, next, err := s.be.List(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListScriptsResponse{Page: &v1.PageResponse{}}
	if tok, _ := EncodeCursor(next); tok != "" {
		out.Page.NextPageToken = tok
	}
	for _, sc := range scripts {
		out.Scripts = append(out.Scripts, &v1.Script{Name: sc.Name, Description: sc.Description})
	}
	return connect.NewResponse(out), nil
}

// Run invokes a named script with structured arguments.
func (s *ScriptService) Run(ctx context.Context, req *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error) {
	var args map[string]any
	if req.Msg.Args != nil {
		args = req.Msg.Args.AsMap()
	}
	result, err := s.be.Run(ctx, req.Msg.Name, args, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "run_failed")
	}
	return connect.NewResponse(&v1.RunScriptResponse{
		RunId:  result.RunID,
		Result: result.Result,
	}), nil
}

// Cancel requests cancellation for a running script.
func (s *ScriptService) Cancel(ctx context.Context, req *connect.Request[v1.CancelScriptRequest]) (*connect.Response[v1.CancelScriptResponse], error) {
	if err := s.be.Cancel(ctx, req.Msg.RunId); err != nil {
		return nil, ToConnect(ctx, err, "cancel_failed")
	}
	return connect.NewResponse(&v1.CancelScriptResponse{}), nil
}

// Eval executes ad hoc Starlark and audits MCP-originated evaluations.
func (s *ScriptService) Eval(ctx context.Context, req *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error) {
	source, _ := SourceFromContext(ctx)
	fromMCP := source == "mcp"

	var cap uint32
	var sessionID string
	if fromMCP && s.mcpCaps != nil {
		cfg, err := s.mcpCaps.MCPConfig(ctx)
		if err == nil {
			cap = cfg.EvalResultMaxBytes
		}
		sessionID = req.Header().Get("x-switchyard-mcp-session")
	}

	started := time.Now()
	result, stdout, runErr := s.be.Eval(ctx, req.Msg.Expr, principalID(ctx))
	durationMs := uint32(time.Since(started).Milliseconds())

	fullBytes := uint32(len(stdout))
	truncated := false
	if fromMCP && cap > 0 && fullBytes > cap {
		marker := fmt.Sprintf("...[truncated; result was %d bytes]", fullBytes)
		keep := int(cap) - len(marker)
		if keep < 0 {
			keep = 0
		}
		stdout = stdout[:keep] + marker
		truncated = true
	}

	if fromMCP && s.events != nil {
		p, _ := auth.PrincipalFromContext(ctx)
		sum := sha256.Sum256([]byte(stdout))
		payload := &eventv1.Payload{Kind: &eventv1.Payload_McpEvalRequested{
			McpEvalRequested: &eventv1.MCPEvalRequested{
				PrincipalId:     p.ID,
				SessionId:       sessionID,
				Source:          req.Msg.Expr,
				ResultSha256Hex: hex.EncodeToString(sum[:]),
				Truncated:       truncated,
				ResultBytes:     fullBytes,
				DurationMs:      durationMs,
				Error:           errString(runErr),
			},
		}}
		if _, appendErr := s.events.Append(ctx, payload); appendErr != nil {
			slog.WarnContext(ctx, "mcp audit append failed", "error", appendErr)
		}
	}

	if runErr != nil {
		return nil, ToConnect(ctx, runErr, "eval_failed")
	}
	return connect.NewResponse(&v1.EvalScriptResponse{
		Result:     result,
		Stdout:     stdout,
		Truncated:  truncated,
		DurationMs: durationMs,
	}), nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// RunTests streams Starlark test results and idle heartbeats.
func (s *ScriptService) RunTests(ctx context.Context, req *connect.Request[v1.RunTestsRequest], stream *connect.ServerStream[v1.RunTestsResponse]) error {
	if req.Msg.Path == "" {
		return ToConnect(ctx, ErrValidationFailed, "missing_path")
	}
	cfg := currentStreamConfig()
	src, cancel, err := s.be.RunTests(ctx, req.Msg.Path)
	if err != nil {
		return ToConnect(ctx, err, "runtests_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case te, ok := <-buffered:
			if !ok {
				return nil
			}
			if err := stream.Send(&v1.RunTestsResponse{
				Kind: &v1.RunTestsResponse_Event{Event: &v1.StarlarkTestEvent{
					Name:    te.Name,
					Outcome: te.Outcome,
					Detail:  te.Detail,
					At:      ProtoTime(te.At),
				}},
			}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.RunTestsResponse{
				Kind: &v1.RunTestsResponse_Heartbeat{Heartbeat: &v1.Heartbeat{
					ServerTime: ProtoTime(t),
				}},
			}); err != nil {
				return err
			}
		}
	}
}
