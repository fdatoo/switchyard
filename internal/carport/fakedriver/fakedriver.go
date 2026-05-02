// Package fakedriver provides an in-process implementation of the Carport Driver
// gRPC service used by unit tests. Separate package so test files can import
// both carport and fakedriver without import cycles.
package fakedriver

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	carportpb "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	eventpb "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
)

// Double implements carportpb.DriverServer for tests.
// Behavior is pluggable via OnCommand and other hooks.
type Double struct {
	carportpb.UnimplementedDriverServer

	// Hooks — callers set these before Serve.
	OnCommand          func(ctx context.Context, c *carportpb.Command) *carportpb.CommandResult
	InitialEntities    []*eventpb.EntityRegistered
	Manifest           *carportpb.DriverManifest
	WantHandshakeError error
	// EventsToEmit: pushed down the Run stream right after it opens.
	EventsToEmit []*carportpb.DriverToHost
	// ExpectedSecret: if non-empty, Handshake rejects mismatches with codes.Unauthenticated.
	ExpectedSecret string

	mu         sync.Mutex
	handshaken int
	closed     bool
}

// TB is the subset of testing.TB we need. Defined here so the fakedriver pkg
// stays testing-free in non-test builds.
type TB interface {
	Helper()
	TempDir() string
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// Serve starts a grpc server on a fresh Unix-domain socket and returns the
// socket path plus a stop func. The server and listener are cleaned up via
// t.Cleanup AND the returned stop func — calling stop is idempotent.
func (d *Double) Serve(t TB) (socketPath string, stop func()) {
	t.Helper()
	// os.MkdirTemp keeps paths short; t.TempDir() embeds the test name and can
	// exceed macOS's 104-char Unix socket path limit.
	dir, err := os.MkdirTemp("", "ghfd")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath = filepath.Join(dir, "sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	s := grpc.NewServer()
	carportpb.RegisterDriverServer(s, d)
	go func() { _ = s.Serve(ln) }()

	var once sync.Once
	stop = func() {
		once.Do(func() {
			s.GracefulStop()
			_ = ln.Close()
		})
	}
	t.Cleanup(stop)
	return socketPath, stop
}

func (d *Double) Handshake(_ context.Context, req *carportpb.HandshakeRequest) (*carportpb.HandshakeResponse, error) {
	if d.WantHandshakeError != nil {
		return nil, d.WantHandshakeError
	}
	if d.ExpectedSecret != "" && req.GetHandshakeSecret() != d.ExpectedSecret {
		return nil, status.Error(codes.Unauthenticated, "bad handshake secret")
	}
	d.mu.Lock()
	d.handshaken++
	d.mu.Unlock()

	mf := d.Manifest
	if mf == nil {
		mf = &carportpb.DriverManifest{
			Name:            "fake",
			Version:         "0.0.0",
			ProtocolVersion: "v1alpha1",
		}
	}
	return &carportpb.HandshakeResponse{
		ProtocolVersion: "v1alpha1",
		Manifest:        mf,
		InitialEntities: d.InitialEntities,
	}, nil
}

func (d *Double) Run(srv carportpb.Driver_RunServer) error {
	// Emit any pre-programmed events immediately.
	for _, m := range d.EventsToEmit {
		if err := srv.Send(m); err != nil {
			return err
		}
	}
	for {
		in, err := srv.Recv()
		if err != nil {
			return err
		}
		switch k := in.GetKind().(type) {
		case *carportpb.HostToDriver_Command:
			if d.OnCommand == nil {
				continue
			}
			res := d.OnCommand(srv.Context(), k.Command)
			if res == nil {
				continue
			}
			if err := srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Result{Result: res},
			}); err != nil {
				return err
			}
		case *carportpb.HostToDriver_Ping:
			_ = srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{TsUnixMs: time.Now().UnixMilli()}},
			})
		}
	}
}

func (d *Double) Health(_ context.Context, _ *carportpb.HealthRequest) (*carportpb.HealthResponse, error) {
	return &carportpb.HealthResponse{Ok: true}, nil
}

func (d *Double) Shutdown(_ context.Context, _ *carportpb.ShutdownRequest) (*carportpb.ShutdownResponse, error) {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	return &carportpb.ShutdownResponse{Acknowledged: true}, nil
}

// HandshakeCount returns the number of successful Handshakes (for assertions).
func (d *Double) HandshakeCount() int { d.mu.Lock(); defer d.mu.Unlock(); return d.handshaken }

// Closed returns true if Shutdown was called.
func (d *Double) Closed() bool { d.mu.Lock(); defer d.mu.Unlock(); return d.closed }
