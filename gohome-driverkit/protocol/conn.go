package protocol

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	carportv1alpha1 "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
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

	// OnRunStart is called when the Run stream opens. emit is valid for the
	// stream's lifetime; ctx is cancelled when the stream ends. Background
	// goroutines that store emit should select on ctx.Done() to stop sending
	// before the stream closes.
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

// Serve starts the gRPC server on SocketPath and blocks until Shutdown is
// called or ctx is cancelled. The server remains live across multiple Run
// streams; a host that disconnects mid-session can reconnect without the
// driver process exiting. Caller manages reconnect loops only for transport
// errors (bind failures, socket loss).
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
		// GracefulStop waits for the Run stream to close. The host must close
		// its send-side after receiving ShutdownResponse; if it does not, the
		// host will SIGKILL the driver process after the grace window.
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

func (s *driverServer) Shutdown(ctx context.Context, req *carportv1alpha1.ShutdownRequest) (*carportv1alpha1.ShutdownResponse, error) {
	if ms := req.GetGraceMs(); ms > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
		defer cancel()
	}
	// OnShutdown error is intentionally ignored: ShutdownResponse has no error
	// field, and the host SIGKILLs the driver if it does not exit within grace_ms.
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
