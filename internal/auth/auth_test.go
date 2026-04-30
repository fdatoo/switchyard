package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fdatoo/gohome/internal/auth"
)

func TestLocalPeerCred_GrantsSystemLocalOnUDS(t *testing.T) {
	a := auth.LocalPeerCred{}
	p, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:   "uds:peercred",
		PeerCred: &auth.PeerCred{Uid: 1000, Pid: 123},
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if p.ID != "system:local" {
		t.Errorf("ID = %q, want system:local", p.ID)
	}
	if p.Kind != "system" {
		t.Errorf("Kind = %q, want system", p.Kind)
	}
}

func TestLocalPeerCred_NotApplicableOnTCP(t *testing.T) {
	a := auth.LocalPeerCred{}
	_, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:     "bearer",
		RemoteAddr: "1.2.3.4:5678",
	})
	if !errors.Is(err, auth.ErrNotApplicable) {
		t.Fatalf("err = %v, want ErrNotApplicable", err)
	}
}

func TestRejectAll_AlwaysUnauthenticated(t *testing.T) {
	a := auth.RejectAll{}
	_, err := a.Authenticate(context.Background(), auth.Request{})
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestChain_FallsThroughOnNotApplicable(t *testing.T) {
	a := auth.Chain(auth.LocalPeerCred{}, auth.RejectAll{})
	_, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:  "bearer",
		Headers: http.Header{"Authorization": []string{"Bearer x"}},
	})
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated (TCP reject)", err)
	}
}

func TestChain_StopsOnSuccess(t *testing.T) {
	a := auth.Chain(auth.LocalPeerCred{}, auth.RejectAll{})
	p, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:   "uds:peercred",
		PeerCred: &auth.PeerCred{Uid: 1000},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if p.ID != "system:local" {
		t.Errorf("ID = %q, want system:local", p.ID)
	}
}

func TestAllowAll_Allows(t *testing.T) {
	a := auth.AllowAll{}
	err := a.Authorize(context.Background(),
		auth.Principal{ID: "user:x"},
		auth.Action{Service: "EntityService", Method: "List", Verb: "read"},
		auth.Target{Kind: "entity"})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestContextPrincipal_Roundtrip(t *testing.T) {
	p := auth.Principal{ID: "user:a", Kind: "user"}
	ctx := auth.WithPrincipal(context.Background(), p)
	got, ok := auth.PrincipalFromContext(ctx)
	if !ok || got.ID != "user:a" {
		t.Fatalf("PrincipalFromContext = %v, %v", got, ok)
	}
}
