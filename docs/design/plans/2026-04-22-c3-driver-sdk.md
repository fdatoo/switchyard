# C3 Driver SDK (gohome-driverkit) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `github.com/fynn-labs/gohome-driverkit` — a standalone Go SDK for writing gohome Carport drivers, with a low-level `protocol` package, a high-level `driver` package, a `drivertest` test harness, a CLI harness binary, and a `fakedevice` example driver.

**Architecture:** A layered SDK in a new repo. `protocol.Conn` owns the gRPC server, UDS, handshake, heartbeat, and shutdown. `driver.Driver` implements `protocol.Handler` and adds entity registration, state tracking, command routing, and a reconnect loop. `drivertest.Harness` acts as a fake Carport host (gRPC client) for in-process unit tests. A `drivertest` CLI binary exercises compiled driver binaries end-to-end.

**Tech Stack:** Go 1.25, `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/google/uuid`, `github.com/fynn-labs/gohome` (for generated proto types only).

---

## File Map

```
gohome-driverkit/
├── go.mod
├── go.sum
├── .golangci.yml
├── .github/workflows/ci.yml
├── doc.go                            # top-level package orientation
├── protocol/
│   ├── doc.go
│   └── conn.go                       # Conn, FromEnv, Emitter, Handler, Serve, driverServer
├── driver/
│   ├── doc.go
│   ├── entity.go                     # EntitySpec, CapabilityHandler
│   └── driver.go                     # Driver, New, AddEntity, OnCapability, EmitState, Run, RunConn
├── drivertest/
│   ├── doc.go
│   ├── harness.go                    # Harness, New, NewAtSocket, SendCommand, StateChanges, AssertState, Entities, Close
│   └── cmd/
│       └── drivertest/
│           └── main.go               # CLI: run --scenario happy-path|reconnect
├── examples/
│   └── fakedevice/
│       ├── main.go                   # dimmable light: turn_on, turn_off, set_brightness
│       └── fakedevice_test.go        # uses drivertest.New
└── docs/
    ├── getting-started.md
    ├── testing.md
    └── protocol.md
```

### Key design decisions locked in here

**`driver.RunConn(ctx, *protocol.Conn)`** is exported. `driver.Run(ctx)` calls `FromEnv()` then `RunConn`. `drivertest.New` calls `RunConn` directly — no env var manipulation, no race between parallel tests.

**StateChanged sent before CommandResult.** When a `CapabilityHandler` returns attrs, the SDK sends `StateChanged` first, then `CommandResult`. This deviates from the C2 spec's recommended ordering but guarantees that when `drivertest.SendCommand` returns, the harness reader has already processed the state change. Noted in a comment in `driver.go`.

**Entity IDs are not in the Carport `StateChanged` proto.** `Harness.AssertState(t, entityID, want)` uses `entityID` only in the failure message; it checks the most recently received `StateChanged` attributes against `want`.

---

## Task 1: Repo bootstrap

**Files:**
- Create: `go.mod`
- Create: `.golangci.yml`
- Create: `.github/workflows/ci.yml`
- Create: `doc.go`, `protocol/doc.go`, `driver/doc.go`, `drivertest/doc.go`

- [ ] **Step 1: Init git repo and module**

```bash
mkdir -p ~/dev/gohome-driverkit
cd ~/dev/gohome-driverkit
git init
go mod init github.com/fynn-labs/gohome-driverkit
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/fynn-labs/gohome@latest
go get github.com/google/uuid@latest
go get google.golang.org/grpc@v1.79.1
go get google.golang.org/protobuf@v1.36.11
go mod tidy
```

Expected: `go.mod` lists all four dependencies, `go.sum` populated.

- [ ] **Step 3: Create `.golangci.yml`**

```yaml
linters:
  enable:
    - gofmt
    - govet
    - staticcheck
    - errcheck
    - unused
    - misspell

linters-settings:
  gofmt:
    simplify: true

issues:
  exclude-rules:
    - path: _test\.go
      linters: [errcheck]
```

- [ ] **Step 4: Create `.github/workflows/ci.yml`**

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go build ./...
      - run: go test ./...
      - run: go test -race ./...
      - run: go test -tags integration ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

- [ ] **Step 5: Create package doc files**

`doc.go`:
```go
// Package driverkit is the Go SDK for writing gohome Carport drivers.
// Most authors start with the driver package.
package driverkit
```

`protocol/doc.go`:
```go
// Package protocol is the low-level Carport driver implementation.
// Most authors use the driver package instead.
package protocol
```

`driver/doc.go`:
```go
// Package driver is the high-level SDK for writing gohome Carport drivers.
// Start with New.
package driver
```

`drivertest/doc.go`:
```go
// Package drivertest provides test helpers for Carport drivers.
// Use New for in-process unit tests and the drivertest CLI for binary integration tests.
package drivertest
```

- [ ] **Step 6: Verify build**

```bash
go build ./...
```

Expected: no output, exit code 0.

- [ ] **Step 7: Commit**

```bash
git add .
git commit -m "chore: bootstrap gohome-driverkit module and CI"
```

---

## Task 2: `protocol/conn.go`

**Files:**
- Create: `protocol/conn.go`

- [ ] **Step 1: Write `protocol/conn.go`**

```go
package protocol

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	carportv1alpha1 "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
)

// Emitter sends messages on the active Run stream.
// Safe to call from any goroutine while the stream is open.
type Emitter interface {
	Send(msg *carportv1alpha1.DriverToHost) error
}

// Handler is implemented by the driver layer or directly by power users.
type Handler interface {
	// OnHandshake is called once during Handshake, after secret verification.
	// config is the raw instance config bytes. Returns the manifest and initial entity list.
	OnHandshake(config []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error)

	// OnRunStart is called when the Run stream opens. emit is valid for the stream's lifetime.
	// Store emit for use in background goroutines.
	OnRunStart(ctx context.Context, emit Emitter)

	// OnCommand is called for each Command on the Run stream.
	// Return (nil, err) to map to CARPORT_INTERNAL.
	OnCommand(ctx context.Context, cmd *carportv1alpha1.Command, emit Emitter) (*carportv1alpha1.CommandResult, error)

	// OnShutdown is called on graceful Shutdown RPC. Flush in-flight work.
	OnShutdown(ctx context.Context) error
}

// Conn holds parameters for a single Carport connection.
type Conn struct {
	SocketPath string
	Secret     string
	InstanceID string
	Config     []byte

	// HealthChecker, if non-nil, is called during Health RPCs.
	// Defaults to always returning ok=true.
	HealthChecker func(ctx context.Context) (ok bool, detail string)
}

// FromEnv constructs a Conn from Carport environment variables.
// Returns an error if GOHOME_CARPORT_SOCKET is unset.
func FromEnv() (*Conn, error) {
	sock := os.Getenv("GOHOME_CARPORT_SOCKET")
	if sock == "" {
		return nil, errors.New("GOHOME_CARPORT_SOCKET not set")
	}
	return &Conn{
		SocketPath: sock,
		Secret:     os.Getenv("GOHOME_CARPORT_SECRET"),
		InstanceID: os.Getenv("GOHOME_CARPORT_INSTANCE_ID"),
		Config:     []byte(os.Getenv("GOHOME_CARPORT_INSTANCE_CONFIG")),
	}, nil
}

// Serve starts the gRPC server on SocketPath and drives the Carport protocol
// for one connection lifetime. Returns when the Run stream closes, Shutdown is
// called, or ctx is cancelled. Does not reconnect — caller manages reconnect loops.
func (c *Conn) Serve(ctx context.Context, h Handler) error {
	// Remove stale socket from a previous Serve call.
	_ = os.Remove(c.SocketPath)

	ln, err := net.Listen("unix", c.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", c.SocketPath, err)
	}

	done := make(chan struct{})
	ds := &driverServer{conn: c, handler: h, done: done}
	srv := grpc.NewServer()
	carportv1alpha1.RegisterDriverServer(srv, ds)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		srv.GracefulStop()
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
		srv.GracefulStop()
		return nil
	}
}

// driverServer is the internal gRPC server implementation.
type driverServer struct {
	carportv1alpha1.UnimplementedDriverServer
	conn    *Conn
	handler Handler
	done    chan struct{}
	once    sync.Once
}

func (s *driverServer) signalDone() {
	s.once.Do(func() { close(s.done) })
}

func (s *driverServer) Handshake(_ context.Context, req *carportv1alpha1.HandshakeRequest) (*carportv1alpha1.HandshakeResponse, error) {
	if req.GetHandshakeSecret() != s.conn.Secret {
		return nil, status.Error(codes.Unauthenticated, "bad handshake secret")
	}
	if req.GetProtocolVersion() != "v1alpha1" {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported protocol version %q", req.GetProtocolVersion())
	}
	manifest, entities, err := s.handler.OnHandshake(s.conn.Config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "handshake: %v", err)
	}
	return &carportv1alpha1.HandshakeResponse{
		ProtocolVersion: "v1alpha1",
		Manifest:        manifest,
		InitialEntities: entities,
	}, nil
}

func (s *driverServer) Health(ctx context.Context, _ *carportv1alpha1.HealthRequest) (*carportv1alpha1.HealthResponse, error) {
	if s.conn.HealthChecker != nil {
		ok, detail := s.conn.HealthChecker(ctx)
		return &carportv1alpha1.HealthResponse{Ok: ok, Detail: detail}, nil
	}
	return &carportv1alpha1.HealthResponse{Ok: true}, nil
}

func (s *driverServer) Shutdown(ctx context.Context, _ *carportv1alpha1.ShutdownRequest) (*carportv1alpha1.ShutdownResponse, error) {
	_ = s.handler.OnShutdown(ctx)
	s.signalDone()
	return &carportv1alpha1.ShutdownResponse{Acknowledged: true}, nil
}

// streamEmitter serialises Send calls — gRPC allows one Send at a time.
type streamEmitter struct {
	mu  sync.Mutex
	srv carportv1alpha1.Driver_RunServer
}

func (e *streamEmitter) Send(msg *carportv1alpha1.DriverToHost) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.srv.Send(msg)
}

func (s *driverServer) Run(srv carportv1alpha1.Driver_RunServer) error {
	emit := &streamEmitter{srv: srv}
	s.handler.OnRunStart(srv.Context(), emit)

	for {
		in, err := srv.Recv()
		if err != nil {
			return err
		}
		switch k := in.GetKind().(type) {
		case *carportv1alpha1.HostToDriver_Command:
			result, handlerErr := s.handler.OnCommand(srv.Context(), k.Command, emit)
			if handlerErr != nil {
				result = &carportv1alpha1.CommandResult{
					CommandId:    k.Command.GetCommandId(),
					Ok:           false,
					Code:         carportv1alpha1.CarportErrorCode_CARPORT_INTERNAL,
					ErrorMessage: handlerErr.Error(),
				}
			}
			if sendErr := emit.Send(&carportv1alpha1.DriverToHost{
				Kind: &carportv1alpha1.DriverToHost_Result{Result: result},
			}); sendErr != nil {
				return sendErr
			}
		case *carportv1alpha1.HostToDriver_Ping:
			_ = emit.Send(&carportv1alpha1.DriverToHost{
				Kind: &carportv1alpha1.DriverToHost_Pong{Pong: &carportv1alpha1.Heartbeat{
					TsUnixMs: k.Ping.GetTsUnixMs(),
				}},
			})
		}
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./protocol/...
```

Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add protocol/
git commit -m "feat(protocol): Conn, FromEnv, Handler, Serve — low-level Carport implementation"
```

---

## Task 3: `protocol` unit tests

**Files:**
- Create: `protocol/conn_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package protocol_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	carportv1alpha1 "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"

	"github.com/fynn-labs/gohome-driverkit/protocol"
)

// stubHandler is a minimal protocol.Handler for tests.
type stubHandler struct {
	manifest *carportv1alpha1.DriverManifest
	entities []*eventv1.EntityRegistered
	onCmd    func(*carportv1alpha1.Command) *carportv1alpha1.CommandResult
	onHealth func(context.Context) (bool, string)
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
	dir, _ := os.MkdirTemp("", "ghptest")
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
	cc, _ := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	if c.SocketPath != "/tmp/test.sock" || c.Secret != "mysecret" {
		t.Errorf("unexpected Conn: %+v", c)
	}
}
```

- [ ] **Step 2: Run tests — verify they pass**

```bash
go test ./protocol/... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 3: Run with race detector**

```bash
go test -race ./protocol/... -count=1
```

Expected: PASS with no race conditions.

- [ ] **Step 4: Commit**

```bash
git add protocol/conn_test.go
git commit -m "test(protocol): full unit test coverage for Conn.Serve"
```

---

## Task 4: `driver` package

**Files:**
- Create: `driver/entity.go`
- Create: `driver/driver.go`

- [ ] **Step 1: Write `driver/entity.go`**

```go
package driver

// EntitySpec describes a driver-owned entity.
type EntitySpec struct {
	EntityType   string   // "light", "sensor", "switch", etc.
	FriendlyName string
	Capabilities []string // advertised in the manifest
}
```

- [ ] **Step 2: Write `driver/driver.go`**

```go
package driver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	carportv1alpha1 "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"

	"github.com/fynn-labs/gohome-driverkit/protocol"
)

// Sentinel errors returned by Driver methods.
var (
	ErrEntityAlreadyRegistered = errors.New("entity already registered")
	ErrEntityUnknown           = errors.New("entity unknown")
	ErrCapabilityUnknown       = errors.New("capability unknown")
	ErrNotConnected            = errors.New("no active run stream")
)

// CapabilityHandler handles a single capability invocation.
// The returned *entityv1.Attributes becomes the entity's tracked state and is
// emitted as a StateChanged event before the CommandResult is sent. Return
// (nil, nil) to acknowledge success without updating state.
type CapabilityHandler func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error)

type entityEntry struct {
	spec     EntitySpec
	attrs    *entityv1.Attributes
	handlers map[string]CapabilityHandler
}

// Driver is the high-level SDK entry point for writing Carport drivers.
type Driver struct {
	name    string
	version string

	mu       sync.RWMutex
	entities map[string]*entityEntry

	emitMu  sync.RWMutex
	emitter protocol.Emitter
}

// New creates a Driver with the given name and version.
func New(name, version string) *Driver {
	return &Driver{
		name:     name,
		version:  version,
		entities: make(map[string]*entityEntry),
	}
}

// AddEntity registers an entity. Call before Run.
// entityID format: "<type>.<name>" e.g. "light.kitchen".
func (d *Driver) AddEntity(entityID string, spec EntitySpec) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.entities[entityID]; ok {
		return fmt.Errorf("%w: %s", ErrEntityAlreadyRegistered, entityID)
	}
	d.entities[entityID] = &entityEntry{
		spec:     spec,
		handlers: make(map[string]CapabilityHandler),
	}
	return nil
}

// OnCapability registers a handler for a specific entity+capability pair.
// Panics if entityID was not registered via AddEntity.
func (d *Driver) OnCapability(entityID, capability string, h CapabilityHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entities[entityID]
	if !ok {
		panic(fmt.Sprintf("driver.OnCapability: entity %q not registered via AddEntity", entityID))
	}
	e.handlers[capability] = h
}

// EmitState updates tracked state for entityID and sends a StateChanged event
// on the current Run stream. Safe to call from any goroutine.
// Returns ErrNotConnected if no stream is active.
func (d *Driver) EmitState(entityID string, attrs *entityv1.Attributes) error {
	d.mu.Lock()
	e, ok := d.entities[entityID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrEntityUnknown, entityID)
	}
	e.attrs = attrs
	d.mu.Unlock()

	d.emitMu.RLock()
	emit := d.emitter
	d.emitMu.RUnlock()
	if emit == nil {
		return ErrNotConnected
	}
	return emit.Send(&carportv1alpha1.DriverToHost{
		Kind: &carportv1alpha1.DriverToHost_StateChanged{
			StateChanged: &eventv1.StateChanged{Attributes: attrs},
		},
	})
}

// Run reads Conn from env vars and calls RunConn in a reconnect loop until ctx
// is cancelled.
func (d *Driver) Run(ctx context.Context) error {
	conn, err := protocol.FromEnv()
	if err != nil {
		return err
	}
	return d.RunConn(ctx, conn)
}

// RunConn serves in a reconnect loop using the provided Conn until ctx is
// cancelled. Exported for testability — drivertest.New uses this directly
// to avoid env var manipulation.
//
// Backoff: 1s initial, 30s max, exponential. Resets to 1s after a session
// longer than 5s (distinguishes transient failures from crash loops).
func (d *Driver) RunConn(ctx context.Context, conn *protocol.Conn) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		start := time.Now()
		if err := conn.Serve(ctx, d); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Since(start) > 5*time.Second {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

// --- protocol.Handler implementation ---

// OnHandshake implements protocol.Handler. Returns the manifest and initial
// entity list using the current tracked state for each entity's capabilities.
func (d *Driver) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	seen := make(map[string]bool)
	var caps []string
	for _, e := range d.entities {
		for _, c := range e.spec.Capabilities {
			if !seen[c] {
				caps = append(caps, c)
				seen[c] = true
			}
		}
	}

	manifest := &carportv1alpha1.DriverManifest{
		Name:                  d.name,
		Version:               d.version,
		ProtocolVersion:       "v1alpha1",
		SupportedCapabilities: caps,
	}

	var entities []*eventv1.EntityRegistered
	for _, e := range d.entities {
		entities = append(entities, &eventv1.EntityRegistered{
			EntityType:   e.spec.EntityType,
			FriendlyName: e.spec.FriendlyName,
			Capabilities: &entityv1.Attributes{}, // typed capabilities are a C4 concern
		})
	}

	return manifest, entities, nil
}

// OnRunStart implements protocol.Handler. Stores the emitter for EmitState calls.
func (d *Driver) OnRunStart(_ context.Context, emit protocol.Emitter) {
	d.emitMu.Lock()
	d.emitter = emit
	d.emitMu.Unlock()
}

// OnCommand implements protocol.Handler. Routes to the registered CapabilityHandler.
// StateChanged is sent before CommandResult so that drivertest.AssertState works
// immediately after SendCommand returns (StateChanged arrives before CommandResult
// on the stream; the harness reader processes it first).
func (d *Driver) OnCommand(_ context.Context, cmd *carportv1alpha1.Command, emit protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
	entityID := cmd.GetEntityId()

	d.mu.RLock()
	e, ok := d.entities[entityID]
	var handler CapabilityHandler
	if ok {
		handler = e.handlers[cmd.GetCapability()]
	}
	d.mu.RUnlock()

	if !ok {
		return &carportv1alpha1.CommandResult{
			CommandId:    cmd.GetCommandId(),
			Ok:           false,
			Code:         carportv1alpha1.CarportErrorCode_CARPORT_UNSUPPORTED_CAPABILITY,
			ErrorMessage: fmt.Sprintf("entity %q unknown", entityID),
		}, nil
	}
	if handler == nil {
		return &carportv1alpha1.CommandResult{
			CommandId:    cmd.GetCommandId(),
			Ok:           false,
			Code:         carportv1alpha1.CarportErrorCode_CARPORT_UNSUPPORTED_CAPABILITY,
			ErrorMessage: fmt.Sprintf("capability %q not registered for %q", cmd.GetCapability(), entityID),
		}, nil
	}

	attrs, err := handler(context.Background(), entityID, cmd.GetArgs())
	if err != nil {
		return &carportv1alpha1.CommandResult{
			CommandId:    cmd.GetCommandId(),
			Ok:           false,
			Code:         carportv1alpha1.CarportErrorCode_CARPORT_INTERNAL,
			ErrorMessage: err.Error(),
		}, nil
	}

	if attrs != nil {
		d.mu.Lock()
		if ent, exists := d.entities[entityID]; exists {
			ent.attrs = attrs
		}
		d.mu.Unlock()

		// Send StateChanged before CommandResult (see package comment in doc.go).
		_ = emit.Send(&carportv1alpha1.DriverToHost{
			Kind: &carportv1alpha1.DriverToHost_StateChanged{
				StateChanged: &eventv1.StateChanged{Attributes: attrs},
			},
		})
	}

	return &carportv1alpha1.CommandResult{
		CommandId: cmd.GetCommandId(),
		Ok:        true,
	}, nil
}

// OnShutdown implements protocol.Handler. Clears the emitter.
func (d *Driver) OnShutdown(_ context.Context) error {
	d.emitMu.Lock()
	d.emitter = nil
	d.emitMu.Unlock()
	return nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 3: Build**

```bash
go build ./driver/...
```

Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add driver/
git commit -m "feat(driver): entity registry, capability routing, state tracking, reconnect loop"
```

---

## Task 5: `drivertest/harness.go`

**Files:**
- Create: `drivertest/harness.go`

- [ ] **Step 1: Write `drivertest/harness.go`**

```go
package drivertest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"
	carportv1alpha1 "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"

	"github.com/fynn-labs/gohome-driverkit/driver"
	"github.com/fynn-labs/gohome-driverkit/protocol"
)

// Harness simulates a Carport host (gRPC client) for in-process driver testing.
type Harness struct {
	// SocketPath is exposed so tests can call NewAtSocket to simulate reconnect.
	SocketPath string

	t        testing.TB
	conn     *grpc.ClientConn
	client   carportv1alpha1.DriverClient
	stream   carportv1alpha1.Driver_RunClient
	entities []*eventv1.EntityRegistered

	sendMu    sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan *carportv1alpha1.CommandResult

	stateChanges chan *eventv1.StateChanged
	lastStateMu  sync.RWMutex
	lastState    *entityv1.Attributes

	doneOnce sync.Once
	done     chan struct{}
}

// New creates a Harness: starts d.RunConn in the background, waits for the
// socket, dials as a gRPC client, and performs the Carport handshake.
// The harness is cleaned up via t.Cleanup automatically.
func New(t testing.TB, d *driver.Driver) *Harness {
	t.Helper()
	dir, err := os.MkdirTemp("", "ghdt")
	if err != nil {
		t.Fatalf("drivertest.New: MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")

	conn := &protocol.Conn{
		SocketPath: sock,
		Secret:     "test-secret",
		InstanceID: "test-instance",
		Config:     []byte("{}"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = d.RunConn(ctx, conn) }()

	return connect(t, sock, "test-secret")
}

// NewAtSocket connects to a driver already listening at socketPath.
// Use after h.Close() to simulate a host reconnect.
func NewAtSocket(t testing.TB, socketPath string) *Harness {
	t.Helper()
	return connect(t, socketPath, "test-secret")
}

// connect dials the socket and performs the handshake.
func connect(t testing.TB, sock, secret string) *Harness {
	t.Helper()

	// Poll until socket appears.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("drivertest: socket %q never appeared: %v", sock, err)
	}

	cc, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("drivertest: dial: %v", err)
	}

	client := carportv1alpha1.NewDriverClient(cc)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := client.Handshake(ctx, &carportv1alpha1.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		HandshakeSecret: secret,
		InstanceConfig:  []byte("{}"),
	})
	if err != nil {
		cc.Close()
		t.Fatalf("drivertest: Handshake: %v", err)
	}

	streamCtx, streamCancel := context.WithCancel(context.Background())
	stream, err := client.Run(streamCtx)
	if err != nil {
		streamCancel()
		cc.Close()
		t.Fatalf("drivertest: Run: %v", err)
	}

	h := &Harness{
		SocketPath:   sock,
		t:            t,
		conn:         cc,
		client:       client,
		stream:       stream,
		entities:     resp.GetInitialEntities(),
		pending:      make(map[string]chan *carportv1alpha1.CommandResult),
		stateChanges: make(chan *eventv1.StateChanged, 64),
		done:         make(chan struct{}),
	}
	go h.reader()

	t.Cleanup(func() {
		streamCancel()
		h.closeOnce()
	})

	return h
}

// Entities returns the initial entity list from the handshake response.
func (h *Harness) Entities() []*eventv1.EntityRegistered {
	return h.entities
}

// SendCommand delivers a command to the driver and blocks for the CommandResult.
func (h *Harness) SendCommand(ctx context.Context, entityID, capability string, args map[string]string) (*carportv1alpha1.CommandResult, error) {
	id := uuid.NewString()
	ch := make(chan *carportv1alpha1.CommandResult, 1)

	h.pendingMu.Lock()
	h.pending[id] = ch
	h.pendingMu.Unlock()
	defer func() {
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
	}()

	h.sendMu.Lock()
	err := h.stream.Send(&carportv1alpha1.HostToDriver{
		Kind: &carportv1alpha1.HostToDriver_Command{
			Command: &carportv1alpha1.Command{
				CommandId:  id,
				EntityId:   entityID,
				Capability: capability,
				Args:       args,
			},
		},
	})
	h.sendMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-h.done:
		return nil, errors.New("drivertest: stream closed")
	}
}

// StateChanges returns a channel that receives every StateChanged event emitted
// by the driver. The channel is buffered (64); slow consumers miss events.
func (h *Harness) StateChanges() <-chan *eventv1.StateChanged {
	return h.stateChanges
}

// AssertState fails the test if the most recently received StateChanged attributes
// do not match want within 1s. entityID is used only in the failure message —
// the Carport StateChanged proto does not carry entity IDs.
func (h *Harness) AssertState(t testing.TB, entityID string, want *entityv1.Attributes) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		h.lastStateMu.RLock()
		got := h.lastState
		h.lastStateMu.RUnlock()
		if proto.Equal(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("AssertState(%q): got %v, want %v", entityID, got, want)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Close closes the gRPC connection without stopping the driver.
// The driver's RunConn reconnect loop will re-listen on SocketPath.
func (h *Harness) Close() {
	h.closeOnce()
}

func (h *Harness) closeOnce() {
	h.doneOnce.Do(func() {
		_ = h.conn.Close()
		close(h.done)
	})
}

// reader pumps DriverToHost messages from the stream.
func (h *Harness) reader() {
	for {
		msg, err := h.stream.Recv()
		if err != nil {
			return
		}
		switch k := msg.GetKind().(type) {
		case *carportv1alpha1.DriverToHost_Result:
			h.pendingMu.Lock()
			ch, ok := h.pending[k.Result.GetCommandId()]
			h.pendingMu.Unlock()
			if ok {
				ch <- k.Result
			}
		case *carportv1alpha1.DriverToHost_StateChanged:
			h.lastStateMu.Lock()
			h.lastState = k.StateChanged.GetAttributes()
			h.lastStateMu.Unlock()
			select {
			case h.stateChanges <- k.StateChanged:
			default:
			}
		}
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./drivertest/...
```

Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add drivertest/harness.go drivertest/doc.go
git commit -m "feat(drivertest): in-process Harness acting as fake Carport host"
```

---

## Task 6: `driver` unit tests

**Files:**
- Create: `driver/driver_test.go`

- [ ] **Step 1: Write failing tests**

```go
package driver_test

import (
	"context"
	"testing"

	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"

	"github.com/fynn-labs/gohome-driverkit/driver"
	"github.com/fynn-labs/gohome-driverkit/drivertest"
)

func lightSpec(caps ...string) driver.EntitySpec {
	return driver.EntitySpec{EntityType: "light", FriendlyName: "Test Light", Capabilities: caps}
}

func lightAttrs(on bool, brightness uint32) *entityv1.Attributes {
	return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
		Light: &entityv1.Light{On: on, Brightness: brightness},
	}}
}

func TestDriver_AddEntity_Duplicate(t *testing.T) {
	d := driver.New("t", "0")
	if err := d.AddEntity("light.a", lightSpec("turn_on")); err != nil {
		t.Fatalf("first AddEntity: %v", err)
	}
	err := d.AddEntity("light.a", lightSpec("turn_on"))
	if err == nil {
		t.Fatal("expected error on duplicate AddEntity")
	}
}

func TestDriver_OnCapability_PanicsOnUnknownEntity(t *testing.T) {
	d := driver.New("t", "0")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unknown entity")
		}
	}()
	d.OnCapability("light.unknown", "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		return nil, nil
	})
}

func TestDriver_EmitState_NotConnected(t *testing.T) {
	d := driver.New("t", "0")
	if err := d.AddEntity("light.a", lightSpec()); err != nil {
		t.Fatal(err)
	}
	err := d.EmitState("light.a", lightAttrs(true, 100))
	if err != driver.ErrNotConnected {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestDriver_EmitState_UnknownEntity(t *testing.T) {
	d := driver.New("t", "0")
	err := d.EmitState("light.unknown", lightAttrs(true, 100))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDriver_HappyPath(t *testing.T) {
	d := driver.New("test", "0.0.1")
	if err := d.AddEntity("light.a", lightSpec("turn_on", "turn_off")); err != nil {
		t.Fatal(err)
	}
	d.OnCapability("light.a", "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		return lightAttrs(true, 200), nil
	})
	d.OnCapability("light.a", "turn_off", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		return lightAttrs(false, 0), nil
	})

	h := drivertest.New(t, d)

	// Turn on.
	res, err := h.SendCommand(context.Background(), "light.a", "turn_on", nil)
	if err != nil {
		t.Fatalf("SendCommand turn_on: %v", err)
	}
	if !res.GetOk() {
		t.Errorf("turn_on result: ok=false, msg=%s", res.GetErrorMessage())
	}
	h.AssertState(t, "light.a", lightAttrs(true, 200))

	// Turn off.
	res, err = h.SendCommand(context.Background(), "light.a", "turn_off", nil)
	if err != nil {
		t.Fatalf("SendCommand turn_off: %v", err)
	}
	if !res.GetOk() {
		t.Errorf("turn_off result: ok=false, msg=%s", res.GetErrorMessage())
	}
	h.AssertState(t, "light.a", lightAttrs(false, 0))
}

func TestDriver_UnknownCapability(t *testing.T) {
	d := driver.New("test", "0.0.1")
	if err := d.AddEntity("light.a", lightSpec("turn_on")); err != nil {
		t.Fatal(err)
	}
	h := drivertest.New(t, d)
	res, err := h.SendCommand(context.Background(), "light.a", "set_brightness", map[string]string{"brightness": "50"})
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if res.GetOk() {
		t.Error("expected ok=false for unknown capability")
	}
}

func TestDriver_InitialEntities(t *testing.T) {
	d := driver.New("test", "0.0.1")
	if err := d.AddEntity("light.a", lightSpec("turn_on")); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEntity("switch.b", driver.EntitySpec{EntityType: "switch", FriendlyName: "B", Capabilities: []string{"toggle"}}); err != nil {
		t.Fatal(err)
	}
	h := drivertest.New(t, d)
	if got := len(h.Entities()); got != 2 {
		t.Errorf("Entities() len = %d, want 2", got)
	}
}

func TestDriver_StatePreservedOnReconnect(t *testing.T) {
	d := driver.New("test", "0.0.1")
	if err := d.AddEntity("light.a", lightSpec("turn_on")); err != nil {
		t.Fatal(err)
	}
	want := lightAttrs(true, 255)
	d.OnCapability("light.a", "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		return want, nil
	})

	h1 := drivertest.New(t, d)
	res, err := h1.SendCommand(context.Background(), "light.a", "turn_on", nil)
	if err != nil || !res.GetOk() {
		t.Fatalf("turn_on failed: %v %v", res, err)
	}
	h1.AssertState(t, "light.a", want)

	// Close h1 — driver's RunConn loop will reconnect.
	h1.Close()
	// Wait briefly for the driver to re-listen.
	time.Sleep(200 * time.Millisecond)

	// Connect h2 to the same socket.
	h2 := drivertest.NewAtSocket(t, h1.SocketPath)
	// Entity count should be the same after reconnect.
	if got := len(h2.Entities()); got != 1 {
		t.Errorf("after reconnect Entities() len = %d, want 1", got)
	}
}
```

- [ ] **Step 2: Run tests — verify they all pass**

```bash
go test ./driver/... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 3: Run with race detector**

```bash
go test -race ./driver/... -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add driver/driver_test.go
git commit -m "test(driver): unit tests for entity registration, command routing, state tracking, reconnect"
```

---

## Task 7: `examples/fakedevice/main.go`

**Files:**
- Create: `examples/fakedevice/main.go`

- [ ] **Step 1: Write the example driver**

```go
// Package main is a realistic example Carport driver: a dimmable light that
// tracks on/off and brightness state. Use it as a starting point for real drivers.
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"

	"github.com/fynn-labs/gohome-driverkit/driver"
)

const entityID = "light.fake_light"

func main() {
	d := driver.New("fakedevice", "0.1.0")

	var mu sync.Mutex
	var on bool
	var brightness uint32 = 100

	if err := d.AddEntity(entityID, driver.EntitySpec{
		EntityType:   "light",
		FriendlyName: "Fake Light",
		Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
	}); err != nil {
		log.Fatalf("AddEntity: %v", err)
	}

	d.OnCapability(entityID, "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		mu.Lock()
		on = true
		b := brightness
		mu.Unlock()
		return lightAttrs(true, b), nil
	})

	d.OnCapability(entityID, "turn_off", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		mu.Lock()
		on = false
		mu.Unlock()
		return lightAttrs(false, 0), nil
	})

	d.OnCapability(entityID, "set_brightness", func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
		v, err := strconv.Atoi(args["brightness"])
		if err != nil || v < 0 || v > 255 {
			return nil, fmt.Errorf("brightness must be an integer 0-255, got %q", args["brightness"])
		}
		mu.Lock()
		brightness = uint32(v)
		isOn := on
		mu.Unlock()
		return lightAttrs(isOn, uint32(v)), nil
	})

	log.Fatal(d.Run(context.Background()))
}

func lightAttrs(isOn bool, b uint32) *entityv1.Attributes {
	return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
		Light: &entityv1.Light{On: isOn, Brightness: b},
	}}
}
```

- [ ] **Step 2: Build**

```bash
go build ./examples/fakedevice/...
```

Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add examples/fakedevice/main.go
git commit -m "feat(examples): fakedevice dimmable-light example driver"
```

---

## Task 8: `examples/fakedevice` tests

**Files:**
- Create: `examples/fakedevice/fakedevice_test.go`

- [ ] **Step 1: Write tests**

```go
package main

import (
	"context"
	"sync"
	"testing"

	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"

	"github.com/fynn-labs/gohome-driverkit/driver"
	"github.com/fynn-labs/gohome-driverkit/drivertest"
)

// newTestDriver mirrors the setup in main() so tests exercise the real driver logic.
func newTestDriver() *driver.Driver {
	d := driver.New("fakedevice", "0.1.0")

	var mu sync.Mutex
	var on bool
	var brightness uint32 = 100

	_ = d.AddEntity(entityID, driver.EntitySpec{
		EntityType:   "light",
		FriendlyName: "Fake Light",
		Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
	})

	d.OnCapability(entityID, "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		mu.Lock(); on = true; b := brightness; mu.Unlock()
		return lightAttrs(true, b), nil
	})
	d.OnCapability(entityID, "turn_off", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		mu.Lock(); on = false; mu.Unlock()
		return lightAttrs(false, 0), nil
	})
	d.OnCapability(entityID, "set_brightness", func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
		v, err := strconv.Atoi(args["brightness"])
		if err != nil || v < 0 || v > 255 {
			return nil, fmt.Errorf("brightness must be 0-255")
		}
		mu.Lock(); brightness = uint32(v); isOn := on; mu.Unlock()
		return lightAttrs(isOn, uint32(v)), nil
	})

	return d
}

func TestFakedevice_TurnOn(t *testing.T) {
	h := drivertest.New(t, newTestDriver())
	res, err := h.SendCommand(context.Background(), entityID, "turn_on", nil)
	if err != nil || !res.GetOk() {
		t.Fatalf("turn_on: %v %v", err, res.GetErrorMessage())
	}
	h.AssertState(t, entityID, lightAttrs(true, 100))
}

func TestFakedevice_TurnOff(t *testing.T) {
	h := drivertest.New(t, newTestDriver())
	h.SendCommand(context.Background(), entityID, "turn_on", nil)  //nolint:errcheck
	res, err := h.SendCommand(context.Background(), entityID, "turn_off", nil)
	if err != nil || !res.GetOk() {
		t.Fatalf("turn_off: %v %v", err, res.GetErrorMessage())
	}
	h.AssertState(t, entityID, lightAttrs(false, 0))
}

func TestFakedevice_SetBrightness(t *testing.T) {
	h := drivertest.New(t, newTestDriver())
	res, err := h.SendCommand(context.Background(), entityID, "set_brightness", map[string]string{"brightness": "42"})
	if err != nil || !res.GetOk() {
		t.Fatalf("set_brightness: %v %v", err, res.GetErrorMessage())
	}
	h.AssertState(t, entityID, lightAttrs(false, 42))
}

func TestFakedevice_SetBrightness_InvalidArg(t *testing.T) {
	h := drivertest.New(t, newTestDriver())
	res, err := h.SendCommand(context.Background(), entityID, "set_brightness", map[string]string{"brightness": "bad"})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.GetOk() {
		t.Error("expected ok=false for invalid brightness")
	}
}

func TestFakedevice_Reconnect(t *testing.T) {
	d := newTestDriver()
	h1 := drivertest.New(t, d)
	h1.SendCommand(context.Background(), entityID, "turn_on", nil) //nolint:errcheck
	h1.Close()

	time.Sleep(200 * time.Millisecond)

	h2 := drivertest.NewAtSocket(t, h1.SocketPath)
	if len(h2.Entities()) != 1 {
		t.Errorf("reconnect: Entities() = %d, want 1", len(h2.Entities()))
	}
}
```

Note: `fakedevice_test.go` is in `package main` to access unexported helpers from `main.go` (`lightAttrs`, `entityID`). Add the missing imports to the test file: `"fmt"`, `"strconv"`, `"time"` — these are already used in `main.go` but must be re-imported in test files.

- [ ] **Step 2: Add missing imports to test file**

The test file needs these imports at the top:
```go
import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
	"github.com/fynn-labs/gohome-driverkit/driver"
	"github.com/fynn-labs/gohome-driverkit/drivertest"
)
```

- [ ] **Step 3: Run tests**

```bash
go test ./examples/fakedevice/... -v -count=1
```

Expected: all 5 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add examples/fakedevice/fakedevice_test.go
git commit -m "test(examples): fakedevice capability and reconnect tests"
```

---

## Task 9: `drivertest` CLI binary

**Files:**
- Create: `drivertest/cmd/drivertest/main.go`

- [ ] **Step 1: Write the CLI**

```go
// Command drivertest exercises a compiled Carport driver binary end-to-end.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	carportv1alpha1 "github.com/fynn-labs/gohome/gen/gohome/carport/v1alpha1"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "drivertest: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	binary     string
	instanceID string
	cfgJSON    string
	scenario   string
	timeout    time.Duration
	jsonOut    bool
}

func run(args []string) error {
	cfg := config{
		instanceID: "test-instance",
		cfgJSON:    "{}",
		scenario:   "happy-path",
		timeout:    30 * time.Second,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "run":
			if i+1 >= len(args) {
				return fmt.Errorf("run requires a binary path")
			}
			i++
			cfg.binary = args[i]
		case "--instance-id":
			i++; cfg.instanceID = args[i]
		case "--config":
			i++; cfg.cfgJSON = args[i]
		case "--scenario":
			i++; cfg.scenario = args[i]
		case "--timeout":
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			cfg.timeout = d
		case "--json":
			cfg.jsonOut = true
		}
	}

	if cfg.binary == "" {
		return fmt.Errorf("usage: drivertest run <binary> [--scenario happy-path|reconnect] [--json]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	switch cfg.scenario {
	case "happy-path":
		return runHappyPath(ctx, cfg)
	case "reconnect":
		return runReconnect(ctx, cfg)
	default:
		return fmt.Errorf("unknown scenario %q; valid: happy-path, reconnect", cfg.scenario)
	}
}

type result struct {
	Scenario string `json:"scenario"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

func printResult(cfg config, r result) {
	if cfg.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(r) //nolint:errcheck
	} else {
		if r.OK {
			fmt.Printf("PASS scenario=%s\n", r.Scenario)
		} else {
			fmt.Printf("FAIL scenario=%s error=%s\n", r.Scenario, r.Error)
		}
	}
}

func runHappyPath(ctx context.Context, cfg config) error {
	dir, err := os.MkdirTemp("", "ghdt-cli")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s")
	secret := "cli-secret"

	cmd := exec.CommandContext(ctx, cfg.binary)
	cmd.Env = append(os.Environ(),
		"GOHOME_CARPORT_SOCKET="+sock,
		"GOHOME_CARPORT_SECRET="+secret,
		"GOHOME_CARPORT_INSTANCE_ID="+cfg.instanceID,
		"GOHOME_CARPORT_INSTANCE_CONFIG="+cfg.cfgJSON,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start driver: %w", err)
	}
	defer cmd.Process.Kill() //nolint:errcheck

	client, cc, err := dialAndHandshake(ctx, sock, secret)
	if err != nil {
		printResult(cfg, result{Scenario: "happy-path", Error: err.Error()})
		return err
	}
	defer cc.Close()

	// Verify Handshake returned a manifest.
	hsResp, err := doHandshake(ctx, client, secret)
	if err != nil {
		printResult(cfg, result{Scenario: "happy-path", Error: "handshake: " + err.Error()})
		return err
	}
	if hsResp.GetManifest().GetName() == "" {
		err := fmt.Errorf("manifest name is empty")
		printResult(cfg, result{Scenario: "happy-path", Error: err.Error()})
		return err
	}

	// Send one command per declared capability (to "test.entity") and assert ok.
	stream, err := client.Run(ctx)
	if err != nil {
		printResult(cfg, result{Scenario: "happy-path", Error: "open Run stream: " + err.Error()})
		return err
	}

	for i, cap := range hsResp.GetManifest().GetSupportedCapabilities() {
		cmdID := fmt.Sprintf("cli-cmd-%d", i)
		if err := stream.Send(&carportv1alpha1.HostToDriver{
			Kind: &carportv1alpha1.HostToDriver_Command{
				Command: &carportv1alpha1.Command{
					CommandId:  cmdID,
					EntityId:   "test.entity",
					Capability: cap,
				},
			},
		}); err != nil {
			printResult(cfg, result{Scenario: "happy-path", Error: "send command: " + err.Error()})
			return err
		}

		// Drain until we get the CommandResult for this command.
		if err := drainUntilResult(stream, cmdID, 5*time.Second); err != nil {
			printResult(cfg, result{Scenario: "happy-path", Error: fmt.Sprintf("cap %q: %v", cap, err)})
			return err
		}
	}

	// Graceful shutdown.
	sCtx, sCancel := context.WithTimeout(ctx, 5*time.Second)
	defer sCancel()
	resp, err := client.Shutdown(sCtx, &carportv1alpha1.ShutdownRequest{GraceMs: 3000})
	if err != nil || !resp.GetAcknowledged() {
		err = fmt.Errorf("shutdown: %v acknowledged=%v", err, resp.GetAcknowledged())
		printResult(cfg, result{Scenario: "happy-path", Error: err.Error()})
		return err
	}

	printResult(cfg, result{Scenario: "happy-path", OK: true, Detail: fmt.Sprintf("entities=%d caps=%d", len(hsResp.GetInitialEntities()), len(hsResp.GetManifest().GetSupportedCapabilities()))})
	return nil
}

func runReconnect(ctx context.Context, cfg config) error {
	dir, err := os.MkdirTemp("", "ghdt-cli")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s")
	secret := "cli-secret"

	cmd := exec.CommandContext(ctx, cfg.binary)
	cmd.Env = append(os.Environ(),
		"GOHOME_CARPORT_SOCKET="+sock,
		"GOHOME_CARPORT_SECRET="+secret,
		"GOHOME_CARPORT_INSTANCE_ID="+cfg.instanceID,
		"GOHOME_CARPORT_INSTANCE_CONFIG="+cfg.cfgJSON,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start driver: %w", err)
	}
	defer cmd.Process.Kill() //nolint:errcheck

	// First handshake.
	_, cc1, err := dialAndHandshake(ctx, sock, secret)
	if err != nil {
		printResult(cfg, result{Scenario: "reconnect", Error: "first handshake: " + err.Error()})
		return err
	}
	resp1, err := doHandshake(ctx, carportv1alpha1.NewDriverClient(cc1), secret)
	if err != nil {
		cc1.Close()
		printResult(cfg, result{Scenario: "reconnect", Error: err.Error()})
		return err
	}
	entityCount := len(resp1.GetInitialEntities())
	cc1.Close()

	// Wait for driver to reconnect.
	time.Sleep(300 * time.Millisecond)

	// Second handshake.
	_, cc2, err := dialAndHandshake(ctx, sock, secret)
	if err != nil {
		printResult(cfg, result{Scenario: "reconnect", Error: "reconnect handshake: " + err.Error()})
		return err
	}
	resp2, err := doHandshake(ctx, carportv1alpha1.NewDriverClient(cc2), secret)
	cc2.Close()
	if err != nil {
		printResult(cfg, result{Scenario: "reconnect", Error: err.Error()})
		return err
	}

	if len(resp2.GetInitialEntities()) != entityCount {
		err := fmt.Errorf("entity count after reconnect: got %d, want %d", len(resp2.GetInitialEntities()), entityCount)
		printResult(cfg, result{Scenario: "reconnect", Error: err.Error()})
		return err
	}

	printResult(cfg, result{Scenario: "reconnect", OK: true, Detail: fmt.Sprintf("entities=%d", entityCount)})
	return nil
}

func dialAndHandshake(ctx context.Context, sock, secret string) (carportv1alpha1.DriverClient, *grpc.ClientConn, error) {
	// Poll for socket.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cc, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	return carportv1alpha1.NewDriverClient(cc), cc, nil
}

func doHandshake(ctx context.Context, client carportv1alpha1.DriverClient, secret string) (*carportv1alpha1.HandshakeResponse, error) {
	hCtx, hCancel := context.WithTimeout(ctx, 5*time.Second)
	defer hCancel()
	return client.Handshake(hCtx, &carportv1alpha1.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		HandshakeSecret: secret,
		InstanceConfig:  []byte("{}"),
	})
}

func drainUntilResult(stream carportv1alpha1.Driver_RunClient, commandID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		if r := msg.GetResult(); r != nil && r.GetCommandId() == commandID {
			if !r.GetOk() {
				return fmt.Errorf("command failed: %s", r.GetErrorMessage())
			}
			return nil
		}
		// StateChanged or other messages — continue draining.
	}
	return fmt.Errorf("timeout waiting for result of command %q", commandID)
}
```

- [ ] **Step 2: Build the CLI binary**

```bash
go build ./drivertest/cmd/drivertest/...
```

Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add drivertest/cmd/
git commit -m "feat(drivertest): CLI binary harness with happy-path and reconnect scenarios"
```

---

## Task 10: CLI integration test

**Files:**
- Create: `drivertest/cmd/drivertest/main_test.go`

- [ ] **Step 1: Write the integration test**

```go
//go:build integration

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCLI_HappyPath compiles the fakedevice binary and runs drivertest against it.
func TestCLI_HappyPath(t *testing.T) {
	tmp := t.TempDir()

	fakedeviceBin := filepath.Join(tmp, "fakedevice")
	if out, err := exec.Command("go", "build", "-o", fakedeviceBin,
		"github.com/fynn-labs/gohome-driverkit/examples/fakedevice").CombinedOutput(); err != nil {
		t.Fatalf("build fakedevice: %v\n%s", err, out)
	}

	driverTestBin := filepath.Join(tmp, "drivertest")
	if out, err := exec.Command("go", "build", "-o", driverTestBin,
		"github.com/fynn-labs/gohome-driverkit/drivertest/cmd/drivertest").CombinedOutput(); err != nil {
		t.Fatalf("build drivertest: %v\n%s", err, out)
	}

	cmd := exec.Command(driverTestBin, "run", fakedeviceBin, "--scenario", "happy-path", "--json")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("drivertest happy-path: %v", err)
	}
}

func TestCLI_Reconnect(t *testing.T) {
	tmp := t.TempDir()

	fakedeviceBin := filepath.Join(tmp, "fakedevice")
	if out, err := exec.Command("go", "build", "-o", fakedeviceBin,
		"github.com/fynn-labs/gohome-driverkit/examples/fakedevice").CombinedOutput(); err != nil {
		t.Fatalf("build fakedevice: %v\n%s", err, out)
	}

	driverTestBin := filepath.Join(tmp, "drivertest")
	if out, err := exec.Command("go", "build", "-o", driverTestBin,
		"github.com/fynn-labs/gohome-driverkit/drivertest/cmd/drivertest").CombinedOutput(); err != nil {
		t.Fatalf("build drivertest: %v\n%s", err, out)
	}

	cmd := exec.Command(driverTestBin, "run", fakedeviceBin, "--scenario", "reconnect", "--json")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("drivertest reconnect: %v", err)
	}
}
```

- [ ] **Step 2: Run integration tests**

```bash
go test -tags integration ./drivertest/cmd/drivertest/... -v -count=1
```

Expected: both tests PASS.

- [ ] **Step 3: Commit**

```bash
git add drivertest/cmd/drivertest/main_test.go
git commit -m "test(drivertest): integration tests compile fakedevice and run CLI scenarios"
```

---

## Task 11: Documentation

**Files:**
- Create: `README.md`
- Create: `docs/getting-started.md`
- Create: `docs/testing.md`
- Create: `docs/protocol.md`

- [ ] **Step 1: Write `README.md`**

```markdown
# gohome-driverkit

Go SDK for writing [gohome](https://github.com/fynn-labs/gohome) Carport drivers.

## Quick start

```go
package main

import (
    "context"
    "log"

    entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
    "github.com/fynn-labs/gohome-driverkit/driver"
)

func main() {
    d := driver.New("my-driver", "0.1.0")

    d.AddEntity("light.main", driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Main Light",
        Capabilities: []string{"turn_on", "turn_off"},
    })

    d.OnCapability("light.main", "turn_on", func(ctx context.Context, id string, args map[string]string) (*entityv1.Attributes, error) {
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{On: true}}}, nil
    })

    log.Fatal(d.Run(context.Background()))
}
```

Point `gohomed` at the compiled binary via `drivers.toml`, and it will spawn and supervise your driver automatically.

## Install

```bash
go get github.com/fynn-labs/gohome-driverkit@latest
```

## Packages

| Package | Purpose |
|---|---|
| [`driver`](./driver/) | High-level SDK — start here |
| [`protocol`](./protocol/) | Low-level Carport protocol — for power users |
| [`drivertest`](./drivertest/) | Test helpers: in-process harness and CLI |

## Docs

- [Getting started](./docs/getting-started.md)
- [Testing your driver](./docs/testing.md)
- [Low-level protocol](./docs/protocol.md)

## CLI harness

```bash
go install github.com/fynn-labs/gohome-driverkit/drivertest/cmd/drivertest@latest
drivertest run ./my-driver --scenario happy-path
```

## Carport protocol compatibility

| driverkit | Carport protocol | gohome |
|---|---|---|
| v0.x | v1alpha1 | C2+ |
```

- [ ] **Step 2: Write `docs/getting-started.md`**

```markdown
# Getting started

This guide walks you from zero to a working Carport driver connected to `gohomed`.

## Prerequisites

- Go 1.25+
- A running `gohomed` instance (C2 or later)

## 1. Create your module

```bash
mkdir my-driver && cd my-driver
go mod init github.com/you/my-driver
go get github.com/fynn-labs/gohome-driverkit@latest
```

## 2. Declare entities

Create `main.go`:

```go
package main

import (
    "context"
    "log"

    entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
    "github.com/fynn-labs/gohome-driverkit/driver"
)

const entityID = "light.ceiling"

func main() {
    d := driver.New("my-driver", "0.1.0")

    if err := d.AddEntity(entityID, driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Ceiling Light",
        Capabilities: []string{"turn_on", "turn_off"},
    }); err != nil {
        log.Fatal(err)
    }
```

Entity IDs follow the format `<type>.<name>`, matching gohome's domain conventions.

## 3. Register capability handlers

```go
    d.OnCapability(entityID, "turn_on", func(ctx context.Context, id string, args map[string]string) (*entityv1.Attributes, error) {
        // Talk to your device here.
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: true, Brightness: 255},
        }}, nil
    })

    d.OnCapability(entityID, "turn_off", func(ctx context.Context, id string, args map[string]string) (*entityv1.Attributes, error) {
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: false},
        }}, nil
    })
```

The returned `*entityv1.Attributes` becomes the new tracked state and is automatically emitted as a `StateChanged` event. Return `(nil, nil)` to acknowledge success without updating state.

## 4. Handle arg validation

```go
    d.OnCapability(entityID, "set_brightness", func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
        v, err := strconv.Atoi(args["brightness"])
        if err != nil || v < 0 || v > 255 {
            return nil, fmt.Errorf("brightness must be 0-255, got %q", args["brightness"])
        }
        // ... apply to device
        return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: true, Brightness: uint32(v)},
        }}, nil
    })
```

## 5. Start the driver

```go
    log.Fatal(d.Run(context.Background()))
}
```

`d.Run` reads env vars set by `gohomed` (`GOHOME_CARPORT_SOCKET`, `GOHOME_CARPORT_SECRET`, etc.) and serves in a reconnect loop until the process exits.

## 6. Background state emission

For drivers that poll a device independently of commands:

```go
go func() {
    for range time.Tick(30 * time.Second) {
        state := pollMyDevice()
        if err := d.EmitState(entityID, state); err != nil && err != driver.ErrNotConnected {
            log.Printf("EmitState: %v", err)
        }
    }
}()
```

`ErrNotConnected` is expected when `gohomed` hasn't connected yet; log other errors.

## 7. Build and register

```bash
go build -o my-driver .
```

Add to `$GOHOME_CONFIG_DIR/drivers.toml`:

```toml
[[instance]]
id      = "my_driver"
binary  = "/path/to/my-driver"
enabled = true
```

Restart `gohomed`. Check health:

```bash
gohome driver status my_driver
```

## 8. Write tests

See [testing.md](./testing.md).
```

- [ ] **Step 3: Write `docs/testing.md`**

```markdown
# Testing your driver

## In-process harness

`drivertest.New(t, d)` starts your driver in-process and connects to it as a fake Carport host. No subprocess needed — tests run fast.

```go
func TestMyDriver_TurnOn(t *testing.T) {
    d := driver.New("my-driver", "0.1.0")
    d.AddEntity("light.ceiling", driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Ceiling",
        Capabilities: []string{"turn_on"},
    })
    d.OnCapability("light.ceiling", "turn_on", myTurnOnHandler)

    h := drivertest.New(t, d)

    res, err := h.SendCommand(context.Background(), "light.ceiling", "turn_on", nil)
    if err != nil || !res.GetOk() {
        t.Fatalf("turn_on failed: %v %s", err, res.GetErrorMessage())
    }
    h.AssertState(t, "light.ceiling", wantAttrs)
}
```

The harness is torn down automatically via `t.Cleanup`.

### Available methods

| Method | Purpose |
|---|---|
| `h.Entities()` | Initial entity list from the handshake |
| `h.SendCommand(ctx, entityID, capability, args)` | Deliver a command, wait for result |
| `h.StateChanges()` | Channel of all `StateChanged` events |
| `h.AssertState(t, entityID, want)` | Assert the last state change matches `want` (polls up to 1s) |
| `h.Close()` | Close the host connection; driver reconnects automatically |

### Reconnect testing

```go
h1 := drivertest.New(t, d)
h1.SendCommand(ctx, "light.ceiling", "turn_on", nil)
h1.Close()
time.Sleep(200 * time.Millisecond) // wait for driver to re-listen

h2 := drivertest.NewAtSocket(t, h1.SocketPath)
// h2.Entities() should reflect reconnected state
```

### Background EmitState

Use `h.StateChanges()` to observe background state emissions:

```go
select {
case sc := <-h.StateChanges():
    // inspect sc.GetAttributes()
case <-time.After(2 * time.Second):
    t.Fatal("no state change received")
}
```

## CLI harness

Install:

```bash
go install github.com/fynn-labs/gohome-driverkit/drivertest/cmd/drivertest@latest
```

Run against a compiled binary:

```bash
drivertest run ./my-driver --scenario happy-path --timeout 30s
drivertest run ./my-driver --scenario reconnect
drivertest run ./my-driver --scenario happy-path --json   # structured output
```

### Scenarios

| Scenario | What it checks |
|---|---|
| `happy-path` | Handshake, one command per capability, clean shutdown |
| `reconnect` | Handshake, drop connection, reconnect, entity count matches |

### Exit codes

- `0` — all assertions passed
- `1` — assertion failure or transport error (reason printed to stderr or JSON)

### CI example (GitHub Actions)

```yaml
- name: Install drivertest
  run: go install github.com/fynn-labs/gohome-driverkit/drivertest/cmd/drivertest@latest

- name: Build driver
  run: go build -o my-driver .

- name: Run drivertest
  run: drivertest run ./my-driver --scenario happy-path --json
```
```

- [ ] **Step 4: Write `docs/protocol.md`**

```markdown
# Low-level protocol package

Most driver authors should use the `driver` package. This doc is for power users who need `protocol.Conn` directly — for example, if you are wrapping an existing gRPC server or need custom `Health` behaviour.

## When to use `protocol` directly

- Your driver already manages a gRPC server and you only want the Carport protocol machinery.
- You need a custom `HealthChecker` (e.g., checking connectivity to the downstream device).
- You are building a non-standard transport layer.

## `Conn` and `FromEnv`

```go
// Reads GOHOME_CARPORT_SOCKET, GOHOME_CARPORT_SECRET,
// GOHOME_CARPORT_INSTANCE_ID, GOHOME_CARPORT_INSTANCE_CONFIG.
conn, err := protocol.FromEnv()

// Or construct directly:
conn := &protocol.Conn{
    SocketPath:    "/run/gohome/my-driver.sock",
    Secret:        "...",
    InstanceID:    "my-driver",
    Config:        []byte(`{"key":"value"}`),
    HealthChecker: func(ctx context.Context) (bool, string) {
        if err := pingDownstreamDevice(); err != nil {
            return false, err.Error()
        }
        return true, ""
    },
}
```

## `Handler` interface

Implement all four methods:

```go
type Handler interface {
    OnHandshake(config []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error)
    OnRunStart(ctx context.Context, emit protocol.Emitter)
    OnCommand(ctx context.Context, cmd *carportv1alpha1.Command, emit protocol.Emitter) (*carportv1alpha1.CommandResult, error)
    OnShutdown(ctx context.Context) error
}
```

`OnRunStart` receives an `Emitter` valid for the stream's lifetime. Store it:

```go
type MyHandler struct {
    mu    sync.Mutex
    emit  protocol.Emitter
}

func (h *MyHandler) OnRunStart(_ context.Context, emit protocol.Emitter) {
    h.mu.Lock()
    h.emit = emit
    h.mu.Unlock()
}
```

Then use it from any goroutine:

```go
h.mu.Lock()
e := h.emit
h.mu.Unlock()
if e != nil {
    e.Send(&carportv1alpha1.DriverToHost{
        Kind: &carportv1alpha1.DriverToHost_StateChanged{
            StateChanged: &eventv1.StateChanged{Attributes: attrs},
        },
    })
}
```

## Reconnect responsibility

`Conn.Serve` handles **one connection lifetime**. It returns when the Run stream closes, `Shutdown` is called, or ctx is cancelled. You must call it again to reconnect:

```go
backoff := time.Second
for {
    if err := conn.Serve(ctx, handler); ctx.Err() != nil {
        return
    }
    time.Sleep(backoff)
    backoff = min(backoff*2, 30*time.Second)
}
```

The `driver.Driver` type does this for you — use `driver.RunConn` if you want the reconnect loop without the entity registry.

## Minimal working example

```go
type myHandler struct{}

func (h *myHandler) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
    return &carportv1alpha1.DriverManifest{
        Name: "minimal", Version: "0.0.1", ProtocolVersion: "v1alpha1",
    }, nil, nil
}
func (h *myHandler) OnRunStart(_ context.Context, _ protocol.Emitter) {}
func (h *myHandler) OnCommand(_ context.Context, cmd *carportv1alpha1.Command, _ protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
    return &carportv1alpha1.CommandResult{CommandId: cmd.GetCommandId(), Ok: true}, nil
}
func (h *myHandler) OnShutdown(_ context.Context) error { return nil }

func main() {
    conn, _ := protocol.FromEnv()
    log.Fatal(conn.Serve(context.Background(), &myHandler{}))
}
```
```

- [ ] **Step 5: Verify docs exist and build is clean**

```bash
go build ./...
go test ./...
```

Expected: both pass.

- [ ] **Step 6: Commit**

```bash
git add README.md docs/
git commit -m "docs: README, getting-started, testing, protocol reference"
```

---

## Task 12: Full CI verification and git tag

- [ ] **Step 1: Run the full test suite**

```bash
go build ./...
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
go test -tags integration ./... -count=1
```

Expected: all pass with exit code 0 and no race conditions.

- [ ] **Step 2: Run linter**

```bash
golangci-lint run ./...
```

Expected: no issues. Fix any reported before continuing.

- [ ] **Step 3: Tidy**

```bash
go mod tidy
git diff go.mod go.sum
```

Expected: no diff (already tidy). If there is a diff, commit it:

```bash
git add go.mod go.sum
git commit -m "chore: go mod tidy"
```

- [ ] **Step 4: Final commit and tag**

```bash
git add -A
git status  # should be clean
git tag c3-complete
```

---

## Self-review against spec

| Spec requirement | Covered by |
|---|---|
| `protocol.Conn`, `FromEnv`, `Handler`, `Emitter`, `Serve` | Task 2 |
| Secret verification, version check, heartbeat pong, Health, Shutdown | Task 2 |
| `protocol` unit tests (handshake, health, shutdown, command, ping, env) | Task 3 |
| `driver.EntitySpec`, `CapabilityHandler` | Task 4 |
| `driver.New`, `AddEntity`, `OnCapability`, `EmitState` | Task 4 |
| Entity state tracking, reconnect preserves state | Task 4, Task 6 |
| `driver.Run` (env → RunConn), `driver.RunConn` (exported for testability) | Task 4 |
| `drivertest.Harness` — in-process fake Carport host | Task 5 |
| `New`, `NewAtSocket`, `SendCommand`, `StateChanges`, `AssertState`, `Entities`, `Close` | Task 5 |
| `driver` unit tests incl. reconnect | Task 6 |
| `fakedevice` example — dimmable light with 3 capabilities | Task 7 |
| `fakedevice` tests — all capabilities + reconnect | Task 8 |
| `drivertest` CLI binary — `run` command, `happy-path`, `reconnect` scenarios | Task 9 |
| CLI integration tests against compiled binary | Task 10 |
| README, getting-started, testing, protocol docs + Go doc comments | Task 11 |
| CI matrix (linux/amd64, linux/arm64, darwin/arm64), race detector, integration gate | Task 1+12 |
| Git tag `c3-complete` | Task 12 |

**Plan complete and saved to `docs/superpowers/plans/2026-04-22-c3-driver-sdk.md`.**

---

## Plan Amendments

### 2026-04-29 — golangci-lint v2 schema

Linter config migrated to golangci-lint v2 schema — same linters enabled (govet, staticcheck, errcheck, unused, misspell, gofmt via formatters block). Non-blocking deviation.
