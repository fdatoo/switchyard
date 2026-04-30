package api

import (
	"context"

	"connectrpc.com/connect"
)

type sourceCtxKey struct{}

// WithSource attaches the request source to ctx.
func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, sourceCtxKey{}, source)
}

// SourceFromContext returns the source. Returns ("cli", false) if not set.
func SourceFromContext(ctx context.Context) (string, bool) {
	if v, ok := ctx.Value(sourceCtxKey{}).(string); ok {
		return v, true
	}
	return "cli", false
}

type sourceInterceptor struct{}

// SourceInterceptor reads x-gohome-source header and puts it on context.
func SourceInterceptor() connect.Interceptor { return &sourceInterceptor{} }

func (i *sourceInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		source := req.Header().Get("x-gohome-source")
		if source == "" {
			source = "cli"
		}
		return next(WithSource(ctx, source), req)
	}
}

func (i *sourceInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *sourceInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		source := conn.RequestHeader().Get("x-gohome-source")
		if source == "" {
			source = "cli"
		}
		return next(WithSource(ctx, source), conn)
	}
}
