package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fdatoo/gohome/internal/cli"
)

func TestDial_TCPEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, base, err := cli.Dial(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if c == nil || base != srv.URL {
		t.Errorf("got client=%v base=%q", c, base)
	}
}

func TestDial_UDSEndpoint(t *testing.T) {
	c, base, err := cli.Dial(context.Background(), "unix:///tmp/nonexistent.sock")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if c == nil || base != "http://unix" {
		t.Errorf("got client=%v base=%q", c, base)
	}
}

func TestResolveEndpoint_DefaultUDS(t *testing.T) {
	got := cli.ResolveEndpoint("", "/data")
	if got != "unix:///data/gohomed.sock" {
		t.Errorf("got %q", got)
	}
}

func TestResolveEndpoint_FlagWins(t *testing.T) {
	t.Setenv("GOHOME_ENDPOINT", "tcp://127.0.0.1:9000")
	got := cli.ResolveEndpoint("tcp://127.0.0.1:8888", "/data")
	if got != "tcp://127.0.0.1:8888" {
		t.Errorf("got %q", got)
	}
}

func TestResolveEndpoint_EnvWins(t *testing.T) {
	t.Setenv("GOHOME_ENDPOINT", "tcp://127.0.0.1:9000")
	got := cli.ResolveEndpoint("", "/data")
	if got != "tcp://127.0.0.1:9000" {
		t.Errorf("got %q", got)
	}
}
