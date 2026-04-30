package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/mcp"
	"github.com/fdatoo/gohome/internal/mcp/tools"
)

type fakeScriptHandler struct {
	gohomev1alpha1connect.UnimplementedScriptServiceHandler

	runFn  func(context.Context, *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error)
	evalFn func(context.Context, *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error)
}

func (h *fakeScriptHandler) Run(ctx context.Context, req *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error) {
	if h.runFn != nil {
		return h.runFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeScriptHandler) Eval(ctx context.Context, req *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error) {
	if h.evalFn != nil {
		return h.evalFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func newScriptsTestDeps(t *testing.T, handler *fakeScriptHandler) (tools.Deps, *sdk.Server) {
	t.Helper()
	_, h := gohomev1alpha1connect.NewScriptServiceHandler(handler)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	s := sdk.NewServer(testImpl, nil)
	d := tools.Deps{
		Server: s,
		Client: c,
		Auth:   auth.AllowAll{},
	}
	return d, s
}

func TestRunScript_HappyPath(t *testing.T) {
	handler := &fakeScriptHandler{
		runFn: func(_ context.Context, req *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error) {
			assert.Equal(t, "my_script", req.Msg.Name)
			val, _ := structpb.NewValue(42.0)
			return connect.NewResponse(&v1.RunScriptResponse{
				RunId:  "run-1",
				Result: val,
			}), nil
		},
	}
	d, s := newScriptsTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__run_script", map[string]any{
		"name": "my_script",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, "run-1", out["run_id"])
}

func TestEvalStarlark_TruncatedPropagated(t *testing.T) {
	handler := &fakeScriptHandler{
		evalFn: func(_ context.Context, req *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error) {
			assert.Equal(t, "1+1", req.Msg.Expr)
			val, _ := structpb.NewValue(2.0)
			return connect.NewResponse(&v1.EvalScriptResponse{
				Result:     val,
				Stdout:     "hello\n",
				DurationMs: 5,
				Truncated:  true,
			}), nil
		},
	}
	d, s := newScriptsTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__eval_starlark", map[string]any{
		"source": "1+1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, true, out["truncated"])
	assert.Equal(t, "hello\n", out["stdout"])
	assert.Equal(t, float64(5), out["duration_ms"])
}
