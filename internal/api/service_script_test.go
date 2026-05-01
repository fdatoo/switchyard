package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

type fakeScripts struct {
	scripts    []api.Script
	runResult  api.ScriptRunResult
	runErr     error
	cancelErr  error
	evalResult *structpb.Value
	evalStdout string
	evalErr    error
}

func (f *fakeScripts) List(_ context.Context, _ api.PageReq) ([]api.Script, api.Cursor, error) {
	return f.scripts, api.Cursor{}, nil
}

func (f *fakeScripts) Run(_ context.Context, name string, _ map[string]any, _ string) (api.ScriptRunResult, error) {
	if f.runErr != nil {
		return api.ScriptRunResult{}, f.runErr
	}
	// Find script by name
	for _, sc := range f.scripts {
		if sc.Name == name {
			return f.runResult, nil
		}
	}
	return api.ScriptRunResult{}, api.ErrScriptNotFound
}

func (f *fakeScripts) Cancel(_ context.Context, runID string) error {
	if f.cancelErr != nil {
		return f.cancelErr
	}
	return nil
}

func (f *fakeScripts) Eval(_ context.Context, _ string, _ string) (*structpb.Value, string, error) {
	if f.evalErr != nil {
		return nil, "", f.evalErr
	}
	return f.evalResult, f.evalStdout, nil
}

func (f *fakeScripts) RunTests(_ context.Context, _ string) (<-chan api.StarlarkTestEvent, func(), error) {
	ch := make(chan api.StarlarkTestEvent)
	return ch, func() { close(ch) }, nil
}

var _ api.ScriptRunner = (*fakeScripts)(nil)

func TestScriptService_Run(t *testing.T) {
	fs := &fakeScripts{
		scripts:   []api.Script{{Name: "greet", Description: "say hello"}},
		runResult: api.ScriptRunResult{RunID: "run-42"},
	}
	s := api.NewScriptService(fs, nil, nil)
	resp, err := s.Run(context.Background(), connect.NewRequest(&v1.RunScriptRequest{Name: "greet"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Msg.RunId != "run-42" {
		t.Errorf("unexpected run_id: %s", resp.Msg.RunId)
	}
}

func TestScriptService_Run_NotFound(t *testing.T) {
	fs := &fakeScripts{}
	s := api.NewScriptService(fs, nil, nil)
	_, err := s.Run(context.Background(), connect.NewRequest(&v1.RunScriptRequest{Name: "unknown"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got: %v", err)
	}
}

func TestScriptService_Cancel_AlreadyFinished(t *testing.T) {
	fs := &fakeScripts{cancelErr: api.ErrRunAlreadyFinished}
	s := api.NewScriptService(fs, nil, nil)
	_, err := s.Cancel(context.Background(), connect.NewRequest(&v1.CancelScriptRequest{RunId: "run-done"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("expected CodeFailedPrecondition, got: %v", err)
	}
}

func TestScriptService_Eval(t *testing.T) {
	val, _ := structpb.NewValue(42.0)
	fs := &fakeScripts{
		evalResult: val,
		evalStdout: "hello\n",
	}
	s := api.NewScriptService(fs, nil, nil)
	resp, err := s.Eval(context.Background(), connect.NewRequest(&v1.EvalScriptRequest{Expr: "21 + 21"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Msg.Stdout != "hello\n" {
		t.Errorf("unexpected stdout: %q", resp.Msg.Stdout)
	}
	if resp.Msg.Result == nil {
		t.Error("expected non-nil result")
	}
}

func TestScriptService_List(t *testing.T) {
	fs := &fakeScripts{
		scripts: []api.Script{
			{Name: "greet", Description: "say hello"},
			{Name: "notify", Description: "send notification"},
		},
	}
	s := api.NewScriptService(fs, nil, nil)
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListScriptsRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Msg.Scripts) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(resp.Msg.Scripts))
	}
}

// TestScriptService_Eval_MCP_Truncated verifies that stdout exceeding the cap
// is truncated and an MCPEvalRequested audit event is appended.
func TestScriptService_Eval_MCP_Truncated(t *testing.T) {
	// 100 KB of output, cap is 100 bytes
	bigOutput := strings.Repeat("x", 100*1024)
	fs := &fakeScripts{evalStdout: bigOutput}
	evts := &fakeEventAppender{}
	caps := fixedMCPCaps{evalCap: 100}
	s := api.NewScriptService(fs, evts, caps)

	ctx := api.WithSource(context.Background(), "mcp")
	req := connect.NewRequest(&v1.EvalScriptRequest{Expr: "x"})
	resp, err := s.Eval(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Msg.Truncated {
		t.Error("expected truncated=true")
	}
	if uint32(len(resp.Msg.Stdout)) > 100 {
		t.Errorf("stdout length %d exceeds cap 100", len(resp.Msg.Stdout))
	}
	if !strings.Contains(resp.Msg.Stdout, "truncated") {
		t.Errorf("expected truncation marker in stdout, got: %q", resp.Msg.Stdout)
	}
	if len(evts.appended) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(evts.appended))
	}
	audit := evts.appended[0].GetMcpEvalRequested()
	if audit == nil {
		t.Fatal("expected MCPEvalRequested payload")
	}
	if !audit.Truncated {
		t.Error("audit.Truncated should be true")
	}
	if audit.ResultBytes != uint32(len(bigOutput)) {
		t.Errorf("audit.ResultBytes = %d, want %d", audit.ResultBytes, len(bigOutput))
	}
}

// TestScriptService_Eval_CLI_NoAudit verifies that CLI requests are not
// truncated and produce no audit event.
func TestScriptService_Eval_CLI_NoAudit(t *testing.T) {
	bigOutput := strings.Repeat("x", 100*1024)
	fs := &fakeScripts{evalStdout: bigOutput}
	evts := &fakeEventAppender{}
	caps := fixedMCPCaps{evalCap: 100}
	s := api.NewScriptService(fs, evts, caps)

	// Default context has no source set → treated as cli
	resp, err := s.Eval(context.Background(), connect.NewRequest(&v1.EvalScriptRequest{Expr: "x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Msg.Truncated {
		t.Error("expected truncated=false for cli source")
	}
	if resp.Msg.Stdout != bigOutput {
		t.Error("stdout should not be truncated for cli source")
	}
	if len(evts.appended) != 0 {
		t.Errorf("expected no audit events for cli, got %d", len(evts.appended))
	}
}

// TestScriptService_Eval_MCP_Error verifies that an eval error with mcp source
// still produces an audit event with the Error field set.
func TestScriptService_Eval_MCP_Error(t *testing.T) {
	evalErr := errors.New("starlark execution failed")
	fs := &fakeScripts{evalErr: evalErr}
	evts := &fakeEventAppender{}
	caps := fixedMCPCaps{evalCap: 65536}
	s := api.NewScriptService(fs, evts, caps)

	ctx := api.WithSource(context.Background(), "mcp")
	req := connect.NewRequest(&v1.EvalScriptRequest{Expr: "bad"})
	_, err := s.Eval(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(evts.appended) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(evts.appended))
	}
	audit := evts.appended[0].GetMcpEvalRequested()
	if audit == nil {
		t.Fatal("expected MCPEvalRequested payload")
	}
	if audit.Error == "" {
		t.Error("audit.Error should be set when eval fails")
	}
}
