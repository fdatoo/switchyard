package api

import (
	"context"
	"net/http"

	"github.com/fdatoo/switchyard/internal/observability"
)

type remoteAddrKey struct{}
type userAgentKey struct{}

func withRemoteAddr(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, remoteAddrKey{}, addr)
}

func remoteAddrFromCtx(ctx context.Context) string {
	s, _ := ctx.Value(remoteAddrKey{}).(string)
	return s
}

func withUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, userAgentKey{}, ua)
}

func userAgentFromCtx(ctx context.Context) string {
	s, _ := ctx.Value(userAgentKey{}).(string)
	return s
}

func requestIDFromCtx(ctx context.Context) string {
	id, _ := observability.RequestIDFromContext(ctx)
	return id
}

type rwContextKey struct{}

// WithResponseWriter stores an http.ResponseWriter on the context so Connect
// handlers can write cookies.
func WithResponseWriter(ctx context.Context, w http.ResponseWriter) context.Context {
	return context.WithValue(ctx, rwContextKey{}, w)
}

func responseWriterFromCtx(ctx context.Context) http.ResponseWriter {
	w, _ := ctx.Value(rwContextKey{}).(http.ResponseWriter)
	return w
}

type httpReqContextKey struct{}

// WithHTTPRequest stores an *http.Request on the context so Connect handlers
// can read incoming cookies (e.g. for session refresh).
func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpReqContextKey{}, r)
}

func httpRequestFromCtx(ctx context.Context) *http.Request {
	r, _ := ctx.Value(httpReqContextKey{}).(*http.Request)
	return r
}
