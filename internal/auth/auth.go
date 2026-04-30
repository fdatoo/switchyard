package auth

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrNotApplicable   = errors.New("auth: not applicable to this request")
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	ErrForbidden       = errors.New("auth: forbidden")
)

type Principal struct {
	ID          string
	DisplayName string
	Kind        string
	Metadata    map[string]string
}

type Request struct {
	Scheme     string
	Headers    http.Header
	PeerCred   *PeerCred
	RemoteAddr string
	Method     string
	HTTP       *http.Request
}

type Action struct {
	Service string
	Method  string
	Verb    string
}

type Target struct {
	Kind  string
	ID    string
	Area  string
	Class string
	Attr  map[string]string
}

type Authenticator interface {
	Authenticate(ctx context.Context, req Request) (Principal, error)
}

type Authorizer interface {
	Authorize(ctx context.Context, p Principal, a Action, t Target) error
}

type principalCtxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}
