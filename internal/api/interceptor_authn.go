package api

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/authn"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/policy"
)

// NewAuthenticate returns the C9 authenticate interceptor. Wraps the supplied
// authenticator chain, attaches Principal + (if applicable) compiled token
// scope to the request context.
func NewAuthenticate(chain auth.Authenticator, bearer *authn.Bearer, tokens *credentials.Tokens) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			authReq := auth.Request{
				Headers:    req.Header(),
				RemoteAddr: req.Peer().Addr,
				Method:     req.Spec().Procedure,
			}

			p, err := chain.Authenticate(ctx, authReq)
			if errors.Is(err, auth.ErrUnauthenticated) {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			ctx = auth.WithPrincipal(ctx, p)
			ctx = withRemoteAddr(ctx, authReq.RemoteAddr)
			ctx = withUserAgent(ctx, authReq.Headers.Get("User-Agent"))

			if bearer != nil && tokens != nil {
				if enc, ok := p.Metadata["token_scope"]; ok {
					if blob, decErr := base64.StdEncoding.DecodeString(enc); decErr == nil {
						ctx = policy.WithTokenScope(ctx, decodeTokenScope(ctx, blob))
					}
				}
			}
			return next(ctx, req)
		}
	}
}

// decodeTokenScope is a stub until TokenScope proto is defined (TODO: Task 21 follow-up).
// Logs a warning when a non-empty scope is presented so callers know narrowing is skipped.
func decodeTokenScope(ctx context.Context, blob []byte) policy.CompiledTokenScope {
	if len(blob) > 0 {
		slog.WarnContext(ctx, "api: token scope narrowing not yet implemented; scope ignored")
	}
	return policy.CompiledTokenScope{}
}
