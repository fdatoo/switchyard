package listener

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"

	errorv1 "github.com/fdatoo/gohome/gen/gohome/error/v1alpha1"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/observability"
)

// SchemeClassifier classifies a request into an auth scheme and whether it
// arrived on the Unix-domain socket.
type SchemeClassifier interface {
	Classify(req connect.AnyRequest) (scheme string, isUDS bool)
}

type peerCredKey struct{}

// WithPeerCred attaches the Unix peer credentials to ctx. Called by the
// UDS connection handler before the request is dispatched.
func WithPeerCred(ctx context.Context, c *auth.PeerCred) context.Context {
	return context.WithValue(ctx, peerCredKey{}, c)
}

func peerCredFromContext(ctx context.Context) *auth.PeerCred {
	c, _ := ctx.Value(peerCredKey{}).(*auth.PeerCred)
	return c
}

// RequestIDInterceptor mints or echoes the X-Request-Id header and stores the
// value in the request context via observability.WithRequestID.
func RequestIDInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			id := req.Header().Get("X-Request-Id")
			if id == "" {
				id = ulid.Make().String()
			}
			ctx = observability.WithRequestID(ctx, id)
			resp, err := next(ctx, req)
			if resp != nil {
				resp.Header().Set("X-Request-Id", id)
			}
			return resp, err
		}
	})
}

// SlogInterceptor logs each completed RPC with method, code, duration, and
// request-id.
func SlogInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			id, _ := observability.RequestIDFromContext(ctx)
			code := connect.CodeOf(err)
			slog.InfoContext(ctx, "api request",
				slog.String("request_id", id),
				slog.String("method", req.Spec().Procedure),
				slog.String("code", code.String()),
				slog.Duration("duration", time.Since(start)))
			return resp, err
		}
	})
}

// MetricsInterceptor records per-procedure request count and latency via the
// gohome_api_* Prometheus metrics.
func MetricsInterceptor(m *observability.Metrics) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			if m != nil && m.APIRequestsTotal != nil {
				proc := req.Spec().Procedure
				code := connect.CodeOf(err).String()
				m.APIRequestsTotal.WithLabelValues(proc, code).Inc()
				m.APIRequestDurationSeconds.WithLabelValues(proc, code).Observe(time.Since(start).Seconds())
			}
			return resp, err
		}
	})
}

// RecoverInterceptor catches panics from downstream handlers and converts them
// to connect.CodeInternal errors, logging the stack trace.
func RecoverInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					id, _ := observability.RequestIDFromContext(ctx)
					slog.ErrorContext(ctx, "api: panic",
						slog.String("request_id", id),
						slog.Any("panic", r),
						slog.String("stack", string(stack)))
					ce := connect.NewError(connect.CodeInternal, errors.New("internal error"))
					detail := &errorv1.ErrorDetail{Reason: "panic", RequestId: id}
					if d, derr := connect.NewErrorDetail(detail); derr == nil {
						ce.AddDetail(d)
					}
					err = ce
				}
			}()
			return next(ctx, req)
		}
	})
}

// AuthenticateInterceptor runs the Authenticator against every request and
// attaches the resulting Principal to the context. Returns CodeUnauthenticated
// if authentication fails.
func AuthenticateInterceptor(a auth.Authenticator, cls SchemeClassifier) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			scheme, _ := cls.Classify(req)
			ar := auth.Request{
				Scheme:     scheme,
				Headers:    req.Header(),
				RemoteAddr: req.Peer().Addr,
				Method:     req.Spec().Procedure,
				PeerCred:   peerCredFromContext(ctx),
			}
			p, err := a.Authenticate(ctx, ar)
			if err != nil {
				id, _ := observability.RequestIDFromContext(ctx)
				ce := connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
				detail := &errorv1.ErrorDetail{Reason: "unauthenticated", RequestId: id}
				if d, derr := connect.NewErrorDetail(detail); derr == nil {
					ce.AddDetail(d)
				}
				return nil, ce
			}
			ctx = auth.WithPrincipal(ctx, p)
			return next(ctx, req)
		}
	})
}

// AuthorizeInterceptor checks the principal in ctx against the action map.
// Procedures not in the map are allowed through unconditionally.
func AuthorizeInterceptor(az auth.Authorizer, actions map[string]auth.Action) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			a, ok := actions[req.Spec().Procedure]
			if !ok {
				return next(ctx, req)
			}
			p, _ := auth.PrincipalFromContext(ctx)
			if err := az.Authorize(ctx, p, a, auth.Target{}); err != nil {
				id, _ := observability.RequestIDFromContext(ctx)
				ce := connect.NewError(connect.CodePermissionDenied, fmt.Errorf("forbidden"))
				detail := &errorv1.ErrorDetail{Reason: "forbidden", RequestId: id}
				if d, derr := connect.NewErrorDetail(detail); derr == nil {
					ce.AddDetail(d)
				}
				return nil, ce
			}
			return next(ctx, req)
		}
	})
}
