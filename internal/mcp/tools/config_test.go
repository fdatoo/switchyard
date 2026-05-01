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

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/mcp/tools"
)

type fakeConfigHandler struct {
	switchyardv1alpha1connect.UnimplementedConfigServiceHandler

	validateFn func(context.Context, *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error)
	applyFn    func(context.Context, *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error)
}

func (h *fakeConfigHandler) Validate(ctx context.Context, req *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error) {
	if h.validateFn != nil {
		return h.validateFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeConfigHandler) Apply(ctx context.Context, req *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error) {
	if h.applyFn != nil {
		return h.applyFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func newConfigTestDeps(t *testing.T, handler *fakeConfigHandler) (tools.Deps, *sdk.Server) {
	t.Helper()
	_, h := switchyardv1alpha1connect.NewConfigServiceHandler(handler)
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

func TestValidateConfig_HappyPath(t *testing.T) {
	handler := &fakeConfigHandler{
		validateFn: func(_ context.Context, req *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error) {
			assert.Equal(t, []byte("bundle"), req.Msg.PklBundle)
			return connect.NewResponse(&v1.ValidateConfigResponse{
				Valid:  true,
				Errors: nil,
			}), nil
		},
	}
	d, s := newConfigTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__validate_config", map[string]any{
		"pkl_bundle": "bundle",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, true, out["valid"])
}

func TestApplyConfig_DryRunForwarded(t *testing.T) {
	var capturedDryRun bool
	handler := &fakeConfigHandler{
		applyFn: func(_ context.Context, req *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error) {
			capturedDryRun = req.Msg.DryRun
			return connect.NewResponse(&v1.ApplyConfigResponse{
				Applied: false,
			}), nil
		},
	}
	d, s := newConfigTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__apply_config", map[string]any{
		"pkl_bundle": "bundle",
		"dry_run":    true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.True(t, capturedDryRun, "expected dry_run=true to be forwarded")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, false, out["applied"])
}
