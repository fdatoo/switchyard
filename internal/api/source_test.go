package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/api"
)

func TestSourceFromContext_Default(t *testing.T) {
	src, ok := api.SourceFromContext(context.Background())
	require.False(t, ok)
	require.Equal(t, "cli", src)
}

func TestSourceFromContext_Explicit(t *testing.T) {
	ctx := api.WithSource(context.Background(), "mcp")
	src, ok := api.SourceFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "mcp", src)
}

func TestSourceInterceptor_ReadsHeader(t *testing.T) {
	var observed string
	next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		observed, _ = api.SourceFromContext(ctx)
		return nil, nil
	})
	h := api.SourceInterceptor().WrapUnary(next)
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("x-gohome-source", "mcp")
	_, _ = h(context.Background(), req)
	require.Equal(t, "mcp", observed)
}

func TestSourceInterceptor_DefaultCLI(t *testing.T) {
	var observed string
	next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		observed, _ = api.SourceFromContext(ctx)
		return nil, nil
	})
	h := api.SourceInterceptor().WrapUnary(next)
	req := connect.NewRequest(&struct{}{})
	_, _ = h(context.Background(), req) // no header
	require.Equal(t, "cli", observed)
}
