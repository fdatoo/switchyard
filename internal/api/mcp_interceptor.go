package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	"github.com/fdatoo/gohome/internal/observability"
)

type mcpInterceptor struct{ m *observability.Metrics }

// MCPInterceptor emits gohome_mcp_* metrics for MCP-sourced requests.
func MCPInterceptor(m *observability.Metrics) connect.Interceptor { return &mcpInterceptor{m: m} }

func (i *mcpInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		source, _ := SourceFromContext(ctx)
		if source != "mcp" || i.m == nil {
			return next(ctx, req)
		}
		tool := req.Header().Get("x-gohome-mcp-tool")
		started := time.Now()
		resp, err := next(ctx, req)
		elapsed := time.Since(started).Seconds()
		if tool != "" && i.m.MCPToolCallsTotal != nil {
			result := classifyErr(err)
			i.m.MCPToolCallsTotal.WithLabelValues(tool, result).Inc()
			i.m.MCPToolCallDuration.WithLabelValues(tool, result).Observe(elapsed)
		}
		return resp, err
	}
}

func (i *mcpInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *mcpInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func classifyErr(err error) string {
	if err == nil {
		return "ok"
	}
	var ce *connect.Error
	if errors.As(err, &ce) && ce.Code() == connect.CodeUnimplemented {
		return "unimplemented"
	}
	return "error"
}
