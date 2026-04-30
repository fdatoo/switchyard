package carport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
)

// instanceConn is the per-instance live runtime: the gRPC client, the open
// Run bidi stream, and the pending-waiter map correlating Commands with Results.
type instanceConn struct {
	conn   *grpc.ClientConn
	client carportpb.DriverClient

	streamCtx    context.Context
	streamCancel context.CancelFunc
	stream       carportpb.Driver_RunClient

	mu      sync.Mutex
	pending map[string]chan *carportpb.CommandResult
	closed  bool

	// sendMu serializes stream.Send calls — gRPC allows one Send at a time.
	sendMu sync.Mutex

	// hookMu guards ingestHook and onStreamError against data races between the
	// reader goroutine and the supervisor goroutine that sets these fields.
	hookMu sync.RWMutex
	// ingestHook fires on every non-result DriverToHost. Set by supervisor.
	ingestHook func(*carportpb.DriverToHost)
	// onStreamError fires exactly once when the reader goroutine exits.
	onStreamError func(error)
	streamErrOnce sync.Once
}

// DialInstance connects to a driver over its Unix-domain socket and opens the
// Run bidi stream. Returns an instanceConn; the caller must Close it.
//
// Transport is always UDS insecure for C2 — authentication is via the handshake
// secret in HandshakeRequest, not TLS.
func DialInstance(_ context.Context, socketPath string) (*instanceConn, error) {
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	client := carportpb.NewDriverClient(conn)

	streamCtx, cancel := context.WithCancel(context.Background())
	stream, err := client.Run(streamCtx) //nolint:contextcheck
	if err != nil {
		cancel()
		_ = conn.Close()
		return nil, fmt.Errorf("open run stream: %w", err)
	}
	ic := &instanceConn{
		conn:         conn,
		client:       client,
		streamCtx:    streamCtx,
		streamCancel: cancel,
		stream:       stream,
		pending:      map[string]chan *carportpb.CommandResult{},
	}
	go ic.reader()
	return ic, nil
}

// reader pumps DriverToHost messages. Results go to pending waiters;
// other messages go through ingestHook.
func (ic *instanceConn) reader() {
	for {
		msg, err := ic.stream.Recv()
		if err != nil {
			ic.failAll(err)
			ic.streamErrOnce.Do(func() {
				ic.hookMu.RLock()
				fn := ic.onStreamError
				ic.hookMu.RUnlock()
				if fn != nil {
					fn(err)
				}
			})
			return
		}
		switch k := msg.GetKind().(type) {
		case *carportpb.DriverToHost_Result:
			ic.deliver(k.Result)
		default:
			ic.hookMu.RLock()
			fn := ic.ingestHook
			ic.hookMu.RUnlock()
			if fn != nil {
				fn(msg)
			}
		}
	}
}

func (ic *instanceConn) deliver(r *carportpb.CommandResult) {
	ic.mu.Lock()
	ch, ok := ic.pending[r.CommandId]
	if ok {
		delete(ic.pending, r.CommandId)
	}
	ic.mu.Unlock()
	if ok {
		select {
		case ch <- r:
		default:
		}
	}
}

func (ic *instanceConn) failAll(_ error) {
	ic.mu.Lock()
	if ic.closed {
		ic.mu.Unlock()
		return
	}
	ic.closed = true
	pending := ic.pending
	ic.pending = nil
	ic.mu.Unlock()
	for _, ch := range pending {
		close(ch) // receivers treat closed chan as ErrStreamClosed
	}
}

// SendCommand queues a Command and blocks on its matching CommandResult.
// Respects ctx deadline. Returns ErrStreamClosed if the stream has died.
func (ic *instanceConn) SendCommand(ctx context.Context, c *carportpb.Command) (*carportpb.CommandResult, error) {
	ic.mu.Lock()
	if ic.closed {
		ic.mu.Unlock()
		return nil, ErrStreamClosed
	}
	ch := make(chan *carportpb.CommandResult, 1)
	ic.pending[c.CommandId] = ch
	ic.mu.Unlock()

	ic.sendMu.Lock()
	sendErr := ic.stream.Send(&carportpb.HostToDriver{Kind: &carportpb.HostToDriver_Command{Command: c}})
	ic.sendMu.Unlock()

	if sendErr != nil {
		ic.mu.Lock()
		delete(ic.pending, c.CommandId)
		ic.mu.Unlock()
		return nil, fmt.Errorf("send command: %w", sendErr)
	}

	// Honor an explicit per-Command deadline if set.
	var tmr *time.Timer
	var deadlineC <-chan time.Time
	if c.DeadlineUnixMs > 0 {
		d := time.Until(time.UnixMilli(c.DeadlineUnixMs))
		if d <= 0 {
			ic.mu.Lock()
			delete(ic.pending, c.CommandId)
			ic.mu.Unlock()
			return nil, ErrDispatchTimeout
		}
		tmr = time.NewTimer(d)
		defer tmr.Stop()
		deadlineC = tmr.C
	}

	select {
	case res, ok := <-ch:
		if !ok {
			return nil, ErrStreamClosed
		}
		return res, nil
	case <-ctx.Done():
		ic.mu.Lock()
		delete(ic.pending, c.CommandId)
		ic.mu.Unlock()
		return nil, ctx.Err()
	case <-deadlineC:
		ic.mu.Lock()
		delete(ic.pending, c.CommandId)
		ic.mu.Unlock()
		return nil, ErrDispatchTimeout
	}
}

// Close terminates the stream and the underlying gRPC client connection.
func (ic *instanceConn) Close() error {
	ic.streamCancel()
	ic.failAll(nil)
	return ic.conn.Close()
}

// setIngestHook registers the supervisor's ingest callback.
func (ic *instanceConn) setIngestHook(f func(*carportpb.DriverToHost)) {
	ic.hookMu.Lock()
	ic.ingestHook = f
	ic.hookMu.Unlock()
}

// setStreamErrorHook registers the supervisor's stream-error callback.
func (ic *instanceConn) setStreamErrorHook(f func(error)) {
	ic.hookMu.Lock()
	ic.onStreamError = f
	ic.hookMu.Unlock()
}

// Handshake performs the Handshake RPC on the already-dialed connection.
func (ic *instanceConn) Handshake(ctx context.Context, req *carportpb.HandshakeRequest) (*carportpb.HandshakeResponse, error) {
	return ic.client.Handshake(ctx, req)
}

// Health calls Health RPC (out-of-band from the Run stream).
func (ic *instanceConn) Health(ctx context.Context) (*carportpb.HealthResponse, error) {
	return ic.client.Health(ctx, &carportpb.HealthRequest{})
}

// Shutdown sends Shutdown RPC; caller is responsible for waiting grace.
func (ic *instanceConn) Shutdown(ctx context.Context, graceMs int64) (*carportpb.ShutdownResponse, error) {
	return ic.client.Shutdown(ctx, &carportpb.ShutdownRequest{GraceMs: graceMs})
}
