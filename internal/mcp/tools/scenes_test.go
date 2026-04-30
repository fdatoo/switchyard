package tools_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/mcp"
	"github.com/fdatoo/gohome/internal/mcp/tools"
)

type fakeSceneHandler struct {
	gohomev1alpha1connect.UnimplementedSceneServiceHandler

	applyFn func(context.Context, *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error)
}

func (h *fakeSceneHandler) Apply(ctx context.Context, req *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
	if h.applyFn != nil {
		return h.applyFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("scene service not implemented"))
}

func newScenesTestDeps(t *testing.T, handler *fakeSceneHandler) (tools.Deps, *sdk.Server) {
	t.Helper()
	_, h := gohomev1alpha1connect.NewSceneServiceHandler(handler)
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

func TestApplyScene_Unimplemented(t *testing.T) {
	handler := &fakeSceneHandler{
		applyFn: func(_ context.Context, _ *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("scene service not implemented"))
		},
	}
	d, s := newScenesTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__apply_scene", map[string]any{"slug": "morning"})
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected IsError=true for unimplemented")
}
