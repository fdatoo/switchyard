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

`Conn.Serve` starts the gRPC server and blocks until `Shutdown` is called or ctx is cancelled. The server remains live across multiple host connections — if the host disconnects mid-session, it can reconnect without the driver exiting. `Conn.Serve` only returns on transport errors (e.g., bind failure) or intentional shutdown, so a minimal reconnect loop is:

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

The `driver.Driver` type provides this loop automatically — use `driver.RunConn` to get a reconnect-capable supervisor with entity registry.

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
