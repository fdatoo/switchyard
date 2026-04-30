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
go install github.com/fdatoo/gohome-driverkit/drivertest/cmd/drivertest@latest
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
  run: go install github.com/fdatoo/gohome-driverkit/drivertest/cmd/drivertest@latest

- name: Build driver
  run: go build -o my-driver .

- name: Run drivertest
  run: drivertest run ./my-driver --scenario happy-path --json
```
