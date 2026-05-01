# Testing Drivers

!!! status-alpha "Alpha — shipped, interface evolving"

The `drivertest` package provides two complementary test harnesses:

- **In-process `Harness`** (`drivertest.New`) — fast unit and integration tests that run inside `go test`, no subprocess required.
- **CLI binary harness** (`drivertest run`) — acceptance tests against a compiled driver binary, suitable for CI.

Both exercise the real Carport gRPC path. Neither mocks the protocol layer.

---

## In-process harness

`drivertest.New` starts your driver's `RunConn` in a background goroutine, creates a temporary Unix socket, dials in as a gRPC client, and performs the Carport handshake — exactly what `switchyardd` does, but in-process and without subprocess overhead. The harness is cleaned up automatically via `t.Cleanup`.

```go
import (
    "context"
    "testing"

    entityv1 "github.com/fynn-labs/switchyard/gen/switchyard/entity/v1"
    "github.com/fynn-labs/switchyard-driverkit/driver"
    "github.com/fynn-labs/switchyard-driverkit/drivertest"
)

func TestTurnOn(t *testing.T) {
    d := driver.New("fakedevice", "0.1.0")
    _ = d.AddEntity("light.fake", driver.EntitySpec{
        EntityType:   "light",
        FriendlyName: "Fake Light",
        Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
    })
    d.OnCapability("light.fake", "turn_on", handleTurnOn)

    h := drivertest.New(t, d)

    res, err := h.SendCommand(context.Background(), "light.fake", "turn_on", nil)
    if err != nil {
        t.Fatalf("SendCommand: %v", err)
    }
    if !res.Ok {
        t.Fatalf("expected ok, got: %s", res.ErrorMessage)
    }

    h.AssertState(t, "light.fake", &entityv1.Attributes{
        Kind: &entityv1.Attributes_Light{
            Light: &entityv1.Light{On: true, Brightness: 100},
        },
    })
}
```

### Harness API

```go
// New starts d.RunConn in the background, dials as a gRPC client, and
// performs the Carport handshake. Cleaned up via t.Cleanup.
func New(t testing.TB, d *driver.Driver) *Harness

// Entities returns the initial entity list from the HandshakeResponse.
func (h *Harness) Entities() []*eventv1.EntityRegistered

// SendCommand delivers a command to the driver and blocks for the
// CommandResult. Returns an error if the stream closes or ctx expires.
func (h *Harness) SendCommand(
    ctx context.Context,
    entityID, capability string,
    args map[string]string,
) (*carportv1alpha1.CommandResult, error)

// StateChanges returns a channel that receives every StateChanged event.
// The channel is buffered (64). **Not closed on shutdown** — use a context or timeout to avoid blocking indefinitely.
func (h *Harness) StateChanges() <-chan *eventv1.StateChanged

// AssertState fails the test if the last received StateChanged attributes
// do not match want within 1s. entityID is used only in the failure message.
func (h *Harness) AssertState(t testing.TB, entityID string, want *entityv1.Attributes)

// Close closes the gRPC connection without stopping the driver.
// Use with NewAtSocket to simulate a host reconnect.
func (h *Harness) Close()
```

### Consuming background state changes

For drivers that emit state changes outside of command handlers (e.g. polling goroutines), use the `StateChanges()` channel:

```go
func TestBackgroundPoll(t *testing.T) {
    d := buildPollingDriver()
    h := drivertest.New(t, d)

    // Wait for up to 2s for a background state change.
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    select {
    case sc := <-h.StateChanges():
        // Inspect sc.Attributes
        _ = sc
    case <-ctx.Done():
        t.Fatal("no StateChanged received within 2s")
    }
}
```

The channel is buffered (64 slots). If your test is slow to consume, events are dropped from the channel (not from the test — `AssertState` uses a separate `lastState` tracking variable that is always up to date).

### Testing reconnect behaviour

Use `h.Close()` to simulate a host disconnect and `drivertest.NewAtSocket` to reconnect:

```go
func TestReconnect(t *testing.T) {
    d := driver.New("my-driver", "0.1.0")
    _ = d.AddEntity("switch.test", driver.EntitySpec{
        EntityType:   "switch",
        FriendlyName: "Test Switch",
        Capabilities: []string{"turn_on", "turn_off"},
    })
    d.OnCapability("switch.test", "turn_on", handleTurnOn)

    h := drivertest.New(t, d)
    sock := h.SocketPath

    // Turn on, then disconnect.
    _, _ = h.SendCommand(context.Background(), "switch.test", "turn_on", nil)
    h.Close()

    // Reconnect — driver should present current state in initial_entities.
    h2 := drivertest.NewAtSocket(t, sock)

    entities := h2.Entities()
    if len(entities) == 0 {
        t.Fatal("expected entity list on reconnect")
    }
    // On reconnect, the entity list structure is preserved but initial attributes are empty — the daemon restores state from its event log.
}
```

---

## `fakedevice` reference implementation

The `examples/fakedevice` driver in the driverkit repo is the canonical reference implementation. It exercises:

- Multi-capability routing (`turn_on`, `turn_off`, `set_brightness`)
- Shared mutable state protected by a mutex
- Argument validation with typed errors
- Background `EmitState` from a goroutine (in the `fakedevice_test.go` variant)

Read `examples/fakedevice/main.go` when you're unsure about Go idioms for a particular pattern.

---

## Integration test patterns

Structure driver tests as a table of scenarios:

```go
func TestCapabilities(t *testing.T) {
    tests := []struct {
        name       string
        capability string
        args       map[string]string
        wantOk     bool
        wantAttrs  *entityv1.Attributes
    }{
        {
            name:       "turn on",
            capability: "turn_on",
            wantOk:     true,
            wantAttrs:  lightAttrs(true, 100),
        },
        {
            name:       "set brightness valid",
            capability: "set_brightness",
            args:       map[string]string{"brightness": "50"},
            wantOk:     true,
            wantAttrs:  lightAttrs(true, 50),
        },
        {
            name:       "set brightness out of range",
            capability: "set_brightness",
            args:       map[string]string{"brightness": "999"},
            wantOk:     false,
        },
    }

    d := buildDriver()
    h := drivertest.New(t, d)

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            res, err := h.SendCommand(context.Background(), "light.fake", tc.capability, tc.args)
            if err != nil {
                t.Fatalf("SendCommand: %v", err)
            }
            if res.Ok != tc.wantOk {
                t.Errorf("ok = %v, want %v: %s", res.Ok, tc.wantOk, res.ErrorMessage)
            }
            if tc.wantAttrs != nil {
                h.AssertState(t, "light.fake", tc.wantAttrs)
            }
        })
    }
}
```

---

## CLI binary harness

The `drivertest` CLI binary exercises a compiled driver end-to-end. Install it:

```
go install github.com/fynn-labs/switchyard-driverkit/drivertest/cmd/drivertest@latest
```

Run built-in scenarios against your binary:

```
drivertest run ./bin/my-driver \
    --instance-id my_instance \
    --config '{"bridgeAddress":"10.0.0.1","apiKeyEnv":"HUE_KEY"}' \
    --scenario happy-path \
    --timeout 30s \
    [--json]
```

**Built-in scenarios:**

| Scenario | What it asserts |
|---|---|
| `happy-path` | Handshake succeeds; at least one entity registered; one `SendCommand` per declared capability returns `ok: true`; clean `Shutdown`. |
| `reconnect` | Handshake, close stream, reconnect, assert entity list matches first handshake exactly (state tracking on reconnect). |

**Output:**

- Human-readable by default.
- `--json` produces structured output for CI reporting.
- Exit code 0 = all assertions passed; non-zero = failure with reason.

### Running `switchyard test` against a driver

```
switchyard driver test ./bin/my-driver --instance hue_main
```

This is a wrapper around `drivertest run` that reads instance config from your local `drivers.toml`, so you don't need to pass `--config` manually.

### Wiring into GitHub Actions

```yaml
- name: Build driver
  run: go build -o ./bin/my-driver ./cmd/my-driver

- name: Run drivertest
  run: |
    drivertest run ./bin/my-driver \
      --scenario happy-path \
      --scenario reconnect \
      --config '{}' \
      --timeout 60s \
      --json | tee drivertest.json

- name: Upload results
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: drivertest-results
    path: drivertest.json
```

Exit code propagation means the step fails automatically if any scenario fails.
