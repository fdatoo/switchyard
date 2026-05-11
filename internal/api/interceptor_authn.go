package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
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
			peerCred := auth.PeerCredFromContext(ctx)
			scheme := "bearer"
			if peerCred != nil {
				scheme = "uds:peercred"
			}
			authReq := auth.Request{
				Scheme:     scheme,
				Headers:    req.Header(),
				RemoteAddr: req.Peer().Addr,
				Method:     req.Spec().Procedure,
				HTTP:       httpRequestFromCtx(ctx),
				PeerCred:   peerCred,
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
					blob, decErr := base64.StdEncoding.DecodeString(enc)
					if decErr != nil {
						return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token scope"))
					}
					scope, decErr := decodeTokenScope(ctx, p.Metadata["token_id"], blob)
					if decErr != nil {
						return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token scope"))
					}
					ctx = policy.WithTokenScope(ctx, scope)
				}
			}
			return next(ctx, req)
		}
	}
}

var tokenScopeCache sync.Map

func decodeTokenScope(_ context.Context, tokenID string, blob []byte) (policy.CompiledTokenScope, error) {
	if len(blob) == 0 {
		return policy.CompiledTokenScope{}, nil
	}
	cacheKey := tokenID
	if cacheKey == "" {
		sum := sha256.Sum256(blob)
		cacheKey = hex.EncodeToString(sum[:])
	}
	if cached, ok := tokenScopeCache.Load(cacheKey); ok {
		return cached.(policy.CompiledTokenScope), nil
	}
	var scopePB authpb.TokenScope
	if err := proto.Unmarshal(blob, &scopePB); err != nil {
		return policy.CompiledTokenScope{}, err
	}
	scope, err := compileTokenScopePB(&scopePB)
	if err != nil {
		return policy.CompiledTokenScope{}, err
	}
	tokenScopeCache.Store(cacheKey, scope)
	return scope, nil
}
