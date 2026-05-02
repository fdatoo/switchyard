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

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/fdatoo/switchyard-driverkit/driver"
	"github.com/fdatoo/switchyard-driverkit/protocol"
	carportv1alpha1 "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
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
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
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
		InstanceId:      "test-instance",
		InstanceConfig:  []byte("{}"),
	})
	if err != nil {
		_ = cc.Close()
		t.Fatalf("drivertest: Handshake: %v", err)
	}

	streamCtx, streamCancel := context.WithCancel(context.Background())
	stream, err := client.Run(streamCtx)
	if err != nil {
		streamCancel()
		_ = cc.Close()
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
