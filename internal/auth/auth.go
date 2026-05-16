package auth

import (
	"context"
	"errors"
	"net/http"
)

var (
	// ErrNotApplicable tells an auth chain to try the next authenticator.
	ErrNotApplicable = errors.New("auth: not applicable to this request")
	// ErrUnauthenticated means the request presented no valid identity.
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	// ErrForbidden means the principal is known but not allowed to act.
	ErrForbidden = errors.New("auth: forbidden")
)

// Principal is the authenticated actor attached to a request.
type Principal struct {
	ID          string
	DisplayName string
	Kind        string
	Metadata    map[string]string
}

// Request is the transport-neutral authentication input.
type Request struct {
	Scheme     string
	Headers    http.Header
	PeerCred   *PeerCred
	RemoteAddr string
	Method     string
	HTTP       *http.Request
}

// Action is the operation an authenticated principal wants to perform.
type Action struct {
	Service string
	Method  string
	Verb    string
}

// Target is the optional resource-level authorization subject.
type Target struct {
	Kind  string
	ID    string
	Area  string
	Class string
	Attr  map[string]string
}

// Authenticator resolves a request into a principal or a stable auth error.
type Authenticator interface {
	Authenticate(ctx context.Context, req Request) (Principal, error)
}

// Authorizer decides whether a principal may perform an action on a target.
type Authorizer interface {
	Authorize(ctx context.Context, p Principal, a Action, t Target) error
}

type principalCtxKey struct{}

// WithPrincipal returns a child context carrying the authenticated principal.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext returns the principal attached by authentication middleware.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}
