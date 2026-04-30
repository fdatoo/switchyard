package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/api"
	"github.com/fdatoo/gohome/internal/observability"
)

func TestMCPInterceptor_TagsToolCall(t *testing.T) {
	m := observability.NewMetrics()

	var called bool
	next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return nil, nil
	})

	h := api.MCPInterceptor(m).WrapUnary(next)

	ctx := api.WithSource(context.Background(), "mcp")
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("x-gohome-mcp-tool", "eval_starlark")

	_, _ = h(ctx, req)
	require.True(t, called)

	// Verify the counter was incremented
	mfs, err := m.Registry.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "gohome_mcp_tool_calls_total" {
			for _, metric := range mf.GetMetric() {
				for _, lp := range metric.GetLabel() {
					if lp.GetName() == "tool" && lp.GetValue() == "eval_starlark" {
						found = true
					}
				}
			}
		}
	}
	require.True(t, found, "expected gohome_mcp_tool_calls_total{tool=eval_starlark} to be present")
}

func TestMCPInterceptor_NoMetricForCLISource(t *testing.T) {
	m := observability.NewMetrics()

	var called bool
	next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return nil, nil
	})

	h := api.MCPInterceptor(m).WrapUnary(next)

	// No source header → treated as cli by SourceInterceptor, but here we test
	// the MCPInterceptor directly with a plain context (no source set = "cli" default).
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("x-gohome-mcp-tool", "eval_starlark")

	_, _ = h(context.Background(), req)
	require.True(t, called)

	mfs, err := m.Registry.Gather()
	require.NoError(t, err)

	for _, mf := range mfs {
		if mf.GetName() == "gohome_mcp_tool_calls_total" {
			require.Empty(t, mf.GetMetric(), "expected no MCP tool call metrics for cli source")
		}
	}
}
