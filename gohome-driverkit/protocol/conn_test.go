package protocol_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	carportv1alpha1 "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"

	"github.com/fdatoo/gohome-driverkit/protocol"
)

// stubHandler is a minimal protocol.Handler for tests.
type stubHandler struct {
	manifest *carportv1alpha1.DriverManifest
	entities []*eventv1.EntityRegistered
	onCmd    func(*carportv1alpha1.Command) *carportv1alpha1.CommandResult
	shutdown bool
}

func (h *stubHandler) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
	mf := h.manifest
	if mf == nil {
		mf = &carportv1alpha1.DriverManifest{Name: "stub", Version: "0.0.0", ProtocolVersion: "v1alpha1"}
	}
	return mf, h.entities, nil
}
func (h *stubHandler) OnRunStart(_ context.Context, _ protocol.Emitter) {}
func (h *stubHandler) OnCommand(_ context.Context, cmd *carportv1alpha1.Command, _ protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
	if h.onCmd != nil {
		return h.onCmd(cmd), nil
	}
	return &carportv1alpha1.CommandResult{CommandId: cmd.GetCommandId(), Ok: true}, nil
}
func (h *stubHandler) OnShutdown(_ context.Context) error { h.shutdown = true; return nil }

// runStartStub closes called when OnRunStart fires.
type runStartStub struct{ called chan struct{} }

func (h *runStartStub) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
	return &carportv1alpha1.DriverManifest{Name: "runstart", ProtocolVersion: "v1alpha1"}, nil, nil
}
func (h *runStartStub) OnRunStart(_ context.Context, _ protocol.Emitter) { close(h.called) }
func (h *runStartStub) OnCommand(_ context.Context, cmd *carportv1alpha1.Command, _ protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
	return &carportv1alpha1.CommandResult{CommandId: cmd.GetCommandId(), Ok: true}, nil
}
func (h *runStartStub) OnShutdown(_ context.Context) error { return nil }

// errorCmdStub's OnCommand always returns an error.
type errorCmdStub struct{}

func (h *errorCmdStub) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
	return &carportv1alpha1.DriverManifest{Name: "err", ProtocolVersion: "v1alpha1"}, nil, nil
}
func (h *errorCmdStub) OnRunStart(_ context.Context, _ protocol.Emitter) {}
func (h *errorCmdStub) OnCommand(_ context.Context, _ *carportv1alpha1.Command, _ protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
	return nil, errors.New("boom")
}
func (h *errorCmdStub) OnShutdown(_ context.Context) error { return nil }

// deadlineShutdownStub captures the context deadline seen by OnShutdown.
type deadlineShutdownStub struct{ gotDeadline chan time.Duration }

func (h *deadlineShutdownStub) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
	return &carportv1alpha1.DriverManifest{Name: "sd", ProtocolVersion: "v1alpha1"}, nil, nil
}
func (h *deadlineShutdownStub) OnRunStart(_ context.Context, _ protocol.Emitter) {}
func (h *deadlineShutdownStub) OnCommand(_ context.Context, cmd *carportv1alpha1.Command, _ protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
	return &carportv1alpha1.CommandResult{CommandId: cmd.GetCommandId(), Ok: true}, nil
}
func (h *deadlineShutdownStub) OnShutdown(ctx context.Context) error {
	if d, ok := ctx.Deadline(); ok {
		h.gotDeadline <- time.Until(d)
	} else {
		h.gotDeadline <- -1
	}
	return nil
}

// startConn starts Conn.Serve in a goroutine and returns the conn + a dial func.
// Uses os.MkdirTemp to keep socket path short (macOS 104-char limit).
func startConn(t *testing.T, secret string, h protocol.Handler) (*protocol.Conn, carportv1alpha1.DriverClient) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ghptest")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")

	c := &protocol.Conn{SocketPath: sock, Secret: secret, InstanceID: "i1", Config: []byte("{}")}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = c.Serve(ctx, h) }()

	// Poll until socket appears (Serve is async).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("server socket never appeared: %v", err)
	}

	cc, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { cc.Close() })
	return c, carportv1alpha1.NewDriverClient(cc)
}

func handshake(t *testing.T, client carportv1alpha1.DriverClient, secret, version string) (*carportv1alpha1.HandshakeResponse, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return client.Handshake(ctx, &carportv1alpha1.HandshakeRequest{
		ProtocolVersion: version,
		HandshakeSecret: secret,
		InstanceConfig:  []byte("{}"),
	})
}

func TestHandshake_Success(t *testing.T) {
	_, client := startConn(t, "secret", &stubHandler{})
	resp, err := handshake(t, client, "secret", "v1alpha1")
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if resp.GetProtocolVersion() != "v1alpha1" {
		t.Errorf("ProtocolVersion = %q", resp.GetProtocolVersion())
	}
	if resp.GetManifest().GetName() != "stub" {
		t.Errorf("Manifest.Name = %q", resp.GetManifest().GetName())
	}
}

func TestHandshake_BadSecret(t *testing.T) {
	_, client := startConn(t, "correct", &stubHandler{})
	_, err := handshake(t, client, "wrong", "v1alpha1")
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestHandshake_BadVersion(t *testing.T) {
	_, client := startConn(t, "s", &stubHandler{})
	_, err := handshake(t, client, "s", "v99")
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestHealth_Default(t *testing.T) {
	_, client := startConn(t, "s", &stubHandler{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.Health(ctx, &carportv1alpha1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !resp.GetOk() {
		t.Errorf("expected ok=true")
	}
}

func TestHealth_CustomChecker(t *testing.T) {
	dir, err := os.MkdirTemp("", "ghptest")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")
	c := &protocol.Conn{
		SocketPath:    sock,
		Secret:        "s",
		HealthChecker: func(_ context.Context) (bool, string) { return false, "degraded" },
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = c.Serve(ctx, &stubHandler{}) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("server socket never appeared: %v", err)
	}
	cc, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { cc.Close() })
	client := carportv1alpha1.NewDriverClient(cc)

	rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rcancel()
	resp, err := client.Health(rctx, &carportv1alpha1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if resp.GetOk() {
		t.Error("expected ok=false")
	}
	if resp.GetDetail() != "degraded" {
		t.Errorf("Detail = %q", resp.GetDetail())
	}
}

func TestShutdown(t *testing.T) {
	h := &stubHandler{}
	_, client := startConn(t, "s", h)
	_, err := handshake(t, client, "s", "v1alpha1")
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.Shutdown(ctx, &carportv1alpha1.ShutdownRequest{GraceMs: 1000})
	if err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if !resp.GetAcknowledged() {
		t.Error("expected acknowledged=true")
	}
	if !h.shutdown {
		t.Error("expected OnShutdown to be called")
	}
}

func TestRun_CommandDispatch(t *testing.T) {
	h := &stubHandler{
		onCmd: func(cmd *carportv1alpha1.Command) *carportv1alpha1.CommandResult {
			return &carportv1alpha1.CommandResult{CommandId: cmd.GetCommandId(), Ok: true}
		},
	}
	_, client := startConn(t, "s", h)
	_, err := handshake(t, client, "s", "v1alpha1")
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := stream.Send(&carportv1alpha1.HostToDriver{
		Kind: &carportv1alpha1.HostToDriver_Command{
			Command: &carportv1alpha1.Command{CommandId: "cmd-1", EntityId: "light.a", Capability: "turn_on"},
		},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	result := msg.GetResult()
	if result == nil {
		t.Fatalf("expected CommandResult, got %T", msg.GetKind())
	}
	if result.GetCommandId() != "cmd-1" || !result.GetOk() {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestRun_HeartbeatPong(t *testing.T) {
	_, client := startConn(t, "s", &stubHandler{})
	_, err := handshake(t, client, "s", "v1alpha1")
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := stream.Send(&carportv1alpha1.HostToDriver{
		Kind: &carportv1alpha1.HostToDriver_Ping{Ping: &carportv1alpha1.Heartbeat{TsUnixMs: 12345}},
	}); err != nil {
		t.Fatalf("Send ping: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	pong := msg.GetPong()
	if pong == nil {
		t.Fatalf("expected Pong, got %T", msg.GetKind())
	}
	if pong.GetTsUnixMs() != 12345 {
		t.Errorf("pong TsUnixMs = %d, want 12345", pong.GetTsUnixMs())
	}
}

func TestFromEnv_MissingSocket(t *testing.T) {
	t.Setenv("GOHOME_CARPORT_SOCKET", "")
	_, err := protocol.FromEnv()
	if err == nil {
		t.Fatal("expected error when GOHOME_CARPORT_SOCKET is unset")
	}
}

func TestFromEnv_Success(t *testing.T) {
	t.Setenv("GOHOME_CARPORT_SOCKET", "/tmp/test.sock")
	t.Setenv("GOHOME_CARPORT_SECRET", "mysecret")
	t.Setenv("GOHOME_CARPORT_INSTANCE_ID", "inst1")
	t.Setenv("GOHOME_CARPORT_INSTANCE_CONFIG", `{"key":"val"}`)
	c, err := protocol.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if c.SocketPath != "/tmp/test.sock" || c.Secret != "mysecret" ||
		c.InstanceID != "inst1" || string(c.Config) != `{"key":"val"}` {
		t.Errorf("unexpected Conn: %+v", c)
	}
}

func TestRun_OnRunStart_Invoked(t *testing.T) {
	called := make(chan struct{})
	h := &runStartStub{called: called}
	_, client := startConn(t, "s", h)
	if _, err := handshake(t, client, "s", "v1alpha1"); err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("OnRunStart was not invoked within 2s of opening Run stream")
	}
}

func TestRun_CommandHandlerError(t *testing.T) {
	h := &errorCmdStub{}
	_, client := startConn(t, "s", h)
	if _, err := handshake(t, client, "s", "v1alpha1"); err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := stream.Send(&carportv1alpha1.HostToDriver{
		Kind: &carportv1alpha1.HostToDriver_Command{
			Command: &carportv1alpha1.Command{CommandId: "cmd-e", EntityId: "light.a", Capability: "turn_on"},
		},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	result := msg.GetResult()
	if result == nil {
		t.Fatalf("expected CommandResult, got %T", msg.GetKind())
	}
	if result.GetOk() {
		t.Error("expected ok=false when handler returns error")
	}
	if result.GetCode() != carportv1alpha1.CarportErrorCode_CARPORT_INTERNAL {
		t.Errorf("expected CARPORT_INTERNAL, got %v", result.GetCode())
	}
	if result.GetErrorMessage() != "boom" {
		t.Errorf("ErrorMessage = %q, want %q", result.GetErrorMessage(), "boom")
	}
}

func TestShutdown_GraceMsDeadline(t *testing.T) {
	h := &deadlineShutdownStub{gotDeadline: make(chan time.Duration, 1)}
	_, client := startConn(t, "s", h)
	if _, err := handshake(t, client, "s", "v1alpha1"); err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Shutdown(ctx, &carportv1alpha1.ShutdownRequest{GraceMs: 500}); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case d := <-h.gotDeadline:
		// Deadline should be in the future but <= 500ms from call time. Allow wide slack.
		if d <= 0 || d > 600*time.Millisecond {
			t.Errorf("expected deadline ~500ms, got %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnShutdown was not called")
	}
}
