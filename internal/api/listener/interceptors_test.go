package listener_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"

	commonv1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api/listener"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/observability"
)

// newReq returns a real connect.AnyRequest (backed by connect.Request[T]) so
// that interceptors can be unit-tested without a running HTTP server.
func newReq() *connect.Request[commonv1.PageRequest] {
	return connect.NewRequest(&commonv1.PageRequest{})
}

func TestRequestID_MintsIfAbsent(t *testing.T) {
	var seen string
	ic := listener.RequestIDInterceptor()
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		id, _ := observability.RequestIDFromContext(ctx)
		seen = id
		return nil, nil
	})
	_, _ = handler(context.Background(), newReq())
	if seen == "" {
		t.Fatal("expected a minted request id")
	}
}

func TestRequestID_EchoesInbound(t *testing.T) {
	ic := listener.RequestIDInterceptor()
	var seen string
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		seen, _ = observability.RequestIDFromContext(ctx)
		return nil, nil
	})
	req := newReq()
	req.Header().Set("X-Request-Id", "abc123")
	_, _ = handler(context.Background(), req)
	if seen != "abc123" {
		t.Errorf("seen = %q, want abc123", seen)
	}
}

func TestAuthenticate_AttachesPrincipal(t *testing.T) {
	ic := listener.AuthenticateInterceptor(auth.LocalPeerCred{}, udsRequestMarker{})
	var seen auth.Principal
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		p, _ := auth.PrincipalFromContext(ctx)
		seen = p
		return nil, nil
	})
	ctx := listener.WithPeerCred(context.Background(), &auth.PeerCred{Uid: 1000})
	_, _ = handler(ctx, newReq())
	if seen.ID != "system:local" {
		t.Errorf("principal = %+v, want system:local", seen)
	}
}

func TestAuthenticate_Rejects(t *testing.T) {
	ic := listener.AuthenticateInterceptor(auth.RejectAll{}, tcpRequestMarker{})
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})
	_, err := handler(context.Background(), newReq())
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnauthenticated {
		t.Fatalf("err = %v, want Unauthenticated", err)
	}
}

func TestRecover_TurnsPanicIntoInternal(t *testing.T) {
	ic := listener.RecoverInterceptor()
	handler := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		panic("boom")
	})
	_, err := handler(context.Background(), newReq())
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInternal {
		t.Fatalf("err = %v, want Internal", err)
	}
}

// X-Request-Id echoing requires a response that has headers; use a real
// connect.Response so that the interceptor can call resp.Header().Set(...).

type udsRequestMarker struct{}
type tcpRequestMarker struct{}

func (udsRequestMarker) Classify(connect.AnyRequest) (scheme string, isUDS bool) {
	return "uds:peercred", true
}

func (tcpRequestMarker) Classify(connect.AnyRequest) (scheme string, isUDS bool) {
	return "bearer", false
}

// Ensure the test file compiles even though http is transitively referenced.
var _ = http.MethodPost
