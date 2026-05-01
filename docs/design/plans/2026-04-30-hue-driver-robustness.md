# Hue Driver Robustness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the ten-item robustness pass on the Hue driver per `docs/design/specs/2026-04-30-hue-driver-robustness-design.md`, plus the foundational `entityv1.Attributes.available` proto field that benefits every driver.

**Architecture:** Foundational change first (proto + driverkit additions), then bottom-up driver work — pure functions in `state` and `bridge` packages, then the wiring in `cmd/hue-driver`. Each task is TDD where it has behaviour to verify; mechanical tasks (proto regen, signature changes) skip the red-green dance.

**Tech Stack:** Go 1.25, `golang.org/x/time/rate` for token-bucket, `log/slog` for structured logging, `gohome-driverkit/driver` for SDK additions. Tests use `httptest.NewTLSServer` and the driverkit's `drivertest` harness.

---

## File Map

```
proto/gohome/entity/v1/attributes.proto   # add bool available = 90
gen/...                                   # regenerated
gohome-driverkit/driver/
├── driver.go                             # UnregisterEntity, EmitDriverEvent methods
└── driver_test.go                        # tests for new methods
internal/cli/
└── state.go                              # add AVAIL column to formatter
drivers/hue/
├── cmd/hue-driver/
│   ├── main.go                           # slog setup, hot-add/remove diff, periodic resync,
│   │                                     # DriverEvent emissions, ListDevices wiring
│   └── main_test.go                      # hot-add/remove, brightness 0, duration, auth exit
└── internal/
    ├── bridge/
    │   ├── client.go                     # auth tracking, timeouts, rate limit integration
    │   ├── client_test.go                # tighter auth + timeout tests
    │   ├── ratelimit.go                  # NEW — limiter wrapper
    │   ├── ratelimit_test.go
    │   ├── devices.go                    # NEW — ListDevices + zigbee_connectivity correlation
    │   ├── devices_test.go
    │   ├── types.go                      # add Dynamics; add Device/ZigbeeConnectivity types
    │   ├── events.go                     # accept zigbee_connectivity events
    │   └── events_test.go
    └── state/
        ├── mapping.go                    # CommandToUpdate updates, LightToAttrs gains available
        ├── mapping_test.go
        ├── capabilities.go               # NEW — Capabilities(bridge.Light) []string
        └── capabilities_test.go
```

### Key conventions

- Module path stays `github.com/fdatoo/gohome` for the driver, `github.com/fdatoo/gohome-driverkit` for the SDK.
- Rate limit: 10 events/sec, burst 10. Lives on `*Client`, wraps both `SetLight` and the read paths.
- Auth-failure window: 3 consecutive 401s within 60s → `ErrAuthRevoked`.
- Command timeout: 5s on `SetLight`; 10s on `ListLights` and `ListDevices`.
- All log lines use `slog` with structured fields: `instance_id`, `bridge_address`, `entity_id`, `error`.

---

## Task 1: Add `available` to `entityv1.Attributes`

**Files:**
- Modify: `proto/gohome/entity/v1/attributes.proto`
- Modify: `gen/gohome/entity/v1/attributes.pb.go` (regenerated)

- [ ] **Step 1: Edit the proto**

```proto
message Attributes {
  oneof kind {
    // 10-19: actuator/sensor domains
    Light  light         = 10;
    Switch switch_device = 11;
    Sensor sensor        = 12;
  }
  // 90-99: cross-cutting metadata
  // available is the driver's claim that the entity is currently reachable.
  // Default false (zero) means unknown / unreachable; drivers explicitly set
  // true after confirming connectivity. The state cache surfaces this.
  bool available = 90;
}
```

- [ ] **Step 2: Regenerate**

```bash
PATH="$PATH:/Users/fdatoo/go/bin" task proto
```

- [ ] **Step 3: Verify the field made it in**

```bash
grep -A1 "Available" gen/gohome/entity/v1/attributes.pb.go | head
```

Expected: a `Available bool` field with json tag.

- [ ] **Step 4: Build everything to confirm no breakage**

```bash
go build ./...
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add proto/ gen/
git commit -m "proto: add Attributes.available for cross-driver reachability"
```

---

## Task 2: driverkit `UnregisterEntity`

Drivers need a way to remove an entity at runtime — currently `AddEntity` is one-way. `UnregisterEntity` removes from the internal map and emits an `EntityUnregistered` message.

**Files:**
- Modify: `gohome-driverkit/driver/driver.go`
- Modify: `gohome-driverkit/driver/driver_test.go`

- [ ] **Step 1: Write the failing test**

Append to `driver_test.go`:

```go
func TestDriver_UnregisterEntity(t *testing.T) {
	d := driver.New("test", "0.0.1")
	if err := d.AddEntity("light.a", lightSpec("turn_on")); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEntity("light.b", lightSpec("turn_on")); err != nil {
		t.Fatal(err)
	}
	h := drivertest.New(t, d)
	defer h.Close()

	if err := d.UnregisterEntity("light.a"); err != nil {
		t.Fatalf("UnregisterEntity: %v", err)
	}

	// Subsequent commands to the unregistered entity must fail at the driver.
	res, err := h.SendCommand(context.Background(), "light.a", "turn_on", nil)
	if err != nil {
		t.Fatalf("SendCommand after unregister: %v", err)
	}
	if res.GetOk() {
		t.Errorf("expected ok=false on unregistered entity")
	}

	// Re-registration must succeed (entity ID is now free).
	if err := d.AddEntity("light.a", lightSpec("turn_on")); err != nil {
		t.Errorf("re-registration after unregister failed: %v", err)
	}
}

func TestDriver_UnregisterEntity_Unknown(t *testing.T) {
	d := driver.New("t", "0")
	if err := d.UnregisterEntity("light.unknown"); err == nil {
		t.Fatal("expected error unregistering unknown entity")
	}
}
```

- [ ] **Step 2: Run, expect failure**

```bash
go test ./gohome-driverkit/driver/... -run TestDriver_UnregisterEntity
```

Expected: FAIL — `undefined: UnregisterEntity`.

- [ ] **Step 3: Implement**

In `driver.go`, after `OnCapability`:

```go
// UnregisterEntity removes the entity from the driver and emits an
// EntityUnregistered message on the current Run stream. Returns
// ErrEntityUnknown if the entity wasn't registered, or ErrNotConnected
// if no stream is active.
func (d *Driver) UnregisterEntity(entityID string) error {
	d.mu.Lock()
	if _, ok := d.entities[entityID]; !ok {
		d.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrEntityUnknown, entityID)
	}
	delete(d.entities, entityID)
	d.mu.Unlock()

	d.emitMu.RLock()
	emit := d.emitter
	d.emitMu.RUnlock()
	if emit == nil {
		return ErrNotConnected
	}
	return emit.Send(&carportv1alpha1.DriverToHost{
		Kind: &carportv1alpha1.DriverToHost_EntityUnregistered{
			EntityUnregistered: &eventv1.EntityUnregistered{Reason: "removed_by_driver"},
		},
	})
}
```

Important: the existing `EntityUnregistered` proto has only a `Reason` field — no entity ID. The carport supervisor's ingest path needs to know which entity. That's a separate concern; we'll handle on the daemon side in Task 3.

- [ ] **Step 4: Run test, expect pass**

```bash
go test ./gohome-driverkit/driver/... -run TestDriver_UnregisterEntity
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add gohome-driverkit/driver/
git commit -m "feat(driverkit): UnregisterEntity for runtime entity removal"
```

---

## Task 3: Add `entity_id` to `EntityUnregistered` proto + carport binding

The current `EntityUnregistered{reason}` proto can't tell the daemon which entity is being removed. Add `entity_id` field and have the carport host bind `Event.Entity` from it.

**Files:**
- Modify: `proto/gohome/event/v1/event.proto`
- Modify: `gen/...` (regenerated)
- Modify: `internal/carport/ingest.go`
- Modify: `internal/carport/ingest_test.go`
- Modify: `gohome-driverkit/driver/driver.go` — populate the new field

- [ ] **Step 1: Edit the proto**

```proto
message EntityUnregistered {
  // 1-9: identity
  string entity_id = 1;
  // 10-19: payload
  string reason    = 10;
}
```

(Field number 1 → `entity_id` is breaking for any existing serialized
EntityUnregistered events; none exist in our event log so this is fine.
Old `reason = 1` becomes `reason = 10`.)

- [ ] **Step 2: Regenerate**

```bash
PATH="$PATH:/Users/fdatoo/go/bin" task proto
```

- [ ] **Step 3: Update driverkit's `UnregisterEntity` to populate `EntityId`**

In `gohome-driverkit/driver/driver.go`:

```go
return emit.Send(&carportv1alpha1.DriverToHost{
    Kind: &carportv1alpha1.DriverToHost_EntityUnregistered{
        EntityUnregistered: &eventv1.EntityUnregistered{
            EntityId: entityID,
            Reason:   "removed_by_driver",
        },
    },
})
```

- [ ] **Step 4: Add the carport ingest test**

Append to `internal/carport/ingest_test.go`:

```go
func TestIngestMessage_EntityUnregisteredBindsEntityID(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_EntityUnregistered{
			EntityUnregistered: &eventpb.EntityUnregistered{EntityId: "light.kitchen", Reason: "removed_by_driver"},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue", msg); err != nil {
		t.Fatal(err)
	}
	evs, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(evs) != 1 || evs[0].Entity != "light.kitchen" {
		t.Fatalf("got %+v, want Entity=light.kitchen", evs)
	}
}
```

- [ ] **Step 5: Update ingest.go to read `entity_id`**

In `internal/carport/ingest.go`, the `EntityUnregistered` case:

```go
case *carportpb.DriverToHost_EntityUnregistered:
    _, err := store.Append(ctx, eventstore.Event{
        Timestamp: now,
        Kind:      "entity_unregistered",
        Entity:    k.EntityUnregistered.GetEntityId(),
        Source:    source,
        Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityUnregistered{
            EntityUnregistered: k.EntityUnregistered,
        }},
    })
    return err
```

- [ ] **Step 6: Run the new test + existing carport tests**

```bash
go test ./internal/carport/... ./gohome-driverkit/driver/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add proto/ gen/ internal/carport/ gohome-driverkit/
git commit -m "fix(carport,driverkit): bind Entity on EntityUnregistered

Adds entity_id field to EntityUnregistered proto. The driverkit's
UnregisterEntity populates it; the carport ingest path reads it into
Event.Entity so the state cache deletes the right entry."
```

---

## Task 4: driverkit `EmitDriverEvent`

Drivers need a way to emit operator-visible `DriverEvent` messages (e.g. "bridge unreachable"). Currently only the carport host emits these.

**Files:**
- Modify: `gohome-driverkit/driver/driver.go`
- Modify: `gohome-driverkit/driver/driver_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestDriver_EmitDriverEvent(t *testing.T) {
	d := driver.New("test", "0.0.1")
	if err := d.AddEntity("light.a", lightSpec()); err != nil {
		t.Fatal(err)
	}
	h := drivertest.New(t, d)
	defer h.Close()

	if err := d.EmitDriverEvent("test_kind", "test_detail"); err != nil {
		t.Fatalf("EmitDriverEvent: %v", err)
	}
	// drivertest doesn't expose a DriverEvent channel; this test verifies
	// the call doesn't panic and doesn't return an error when connected.
}

func TestDriver_EmitDriverEvent_NotConnected(t *testing.T) {
	d := driver.New("t", "0")
	err := d.EmitDriverEvent("k", "d")
	if !errors.Is(err, driver.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}
```

- [ ] **Step 2: Run, expect failure**

Expected: FAIL — `undefined: EmitDriverEvent`.

- [ ] **Step 3: Implement**

```go
// EmitDriverEvent sends a typed driver event on the current Run stream.
// Returns ErrNotConnected if no stream is active. The kind/detail
// strings are free-form; conventionally kind is snake_case (e.g.
// "bridge_unreachable") and detail is human-readable text.
func (d *Driver) EmitDriverEvent(kind, detail string) error {
	d.emitMu.RLock()
	emit := d.emitter
	d.emitMu.RUnlock()
	if emit == nil {
		return ErrNotConnected
	}
	return emit.Send(&carportv1alpha1.DriverToHost{
		Kind: &carportv1alpha1.DriverToHost_DriverEvent{
			DriverEvent: &eventv1.DriverEvent{Kind: kind, Detail: detail},
		},
	})
}
```

- [ ] **Step 4: Run test, expect pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(driverkit): EmitDriverEvent for operator-visible events"
```

---

## Task 5: `state.Capabilities` per-bulb filter

**Files:**
- Create: `drivers/hue/internal/state/capabilities.go`
- Create: `drivers/hue/internal/state/capabilities_test.go`

- [ ] **Step 1: Write the failing test**

```go
package state

import (
	"reflect"
	"sort"
	"testing"

	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
)

func TestCapabilities(t *testing.T) {
	cases := []struct {
		name string
		in   bridge.Light
		want []string
	}{
		{
			name: "white-only bulb (no dimming)",
			in:   bridge.Light{},
			want: []string{"turn_off", "turn_on"},
		},
		{
			name: "dimmable white",
			in:   bridge.Light{Dimming: &bridge.Dimming{}},
			want: []string{"set_brightness", "turn_off", "turn_on"},
		},
		{
			name: "dimmable + color temp",
			in: bridge.Light{
				Dimming:          &bridge.Dimming{},
				ColorTemperature: &bridge.ColorTemperature{},
			},
			want: []string{"set_brightness", "set_color_temp", "turn_off", "turn_on"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Capabilities(tc.in)
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Capabilities = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run, expect failure**

Expected: FAIL — `undefined: Capabilities`.

- [ ] **Step 3: Implement**

```go
package state

import (
	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
)

// Capabilities returns the gohome capability strings the bulb supports,
// inferred from the presence of optional fields in the Hue v2 light
// resource. Every Hue light supports turn_on/turn_off; set_brightness
// requires the dimming block; set_color_temp requires the
// color_temperature block.
func Capabilities(l bridge.Light) []string {
	caps := []string{"turn_on", "turn_off"}
	if l.Dimming != nil {
		caps = append(caps, "set_brightness")
	}
	if l.ColorTemperature != nil {
		caps = append(caps, "set_color_temp")
	}
	return caps
}
```

- [ ] **Step 4: Run test, expect pass**

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/internal/state/
git commit -m "feat(hue): per-bulb capability filtering"
```

---

## Task 6: `state.LightToAttrs` and `MergeEvent` accept `available`

**Files:**
- Modify: `drivers/hue/internal/state/mapping.go`
- Modify: `drivers/hue/internal/state/mapping_test.go`

- [ ] **Step 1: Add the failing test**

Append to `mapping_test.go`:

```go
func TestLightToAttrs_Available(t *testing.T) {
	in := bridge.Light{On: bridge.OnState{On: true}}
	attrs := LightToAttrs(in, true)
	if !attrs.GetAvailable() {
		t.Errorf("Available = false, want true")
	}
	attrs2 := LightToAttrs(in, false)
	if attrs2.GetAvailable() {
		t.Errorf("Available = true, want false")
	}
}

func TestMergeEvent_PreservesAvailable(t *testing.T) {
	prev := &entityv1.Light{On: true, Brightness: 100}
	merged := MergeEvent(prev, bridge.Event{}, true)
	if !merged.GetAvailable() {
		t.Errorf("Available not propagated")
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

Expected: signature mismatch.

- [ ] **Step 3: Update implementations**

In `mapping.go`, change the signatures:

```go
func LightToAttrs(l bridge.Light, available bool) *entityv1.Attributes {
	light := &entityv1.Light{On: l.On.On}
	if l.Dimming != nil {
		light.Brightness = brightnessHueToGohome(l.Dimming.Brightness)
	}
	if l.ColorTemperature != nil && l.ColorTemperature.Mirek != nil {
		light.ColorTemp = *l.ColorTemperature.Mirek
	}
	return &entityv1.Attributes{
		Available: available,
		Kind:      &entityv1.Attributes_Light{Light: light},
	}
}

func MergeEvent(prev *entityv1.Light, ev bridge.Event, available bool) *entityv1.Attributes {
	next := &entityv1.Light{
		On:         prev.GetOn(),
		Brightness: prev.GetBrightness(),
		ColorTemp:  prev.GetColorTemp(),
	}
	if ev.On != nil {
		next.On = ev.On.On
	}
	if ev.Dimming != nil {
		next.Brightness = brightnessHueToGohome(ev.Dimming.Brightness)
	}
	if ev.ColorTemperature != nil && ev.ColorTemperature.Mirek != nil {
		next.ColorTemp = *ev.ColorTemperature.Mirek
	}
	return &entityv1.Attributes{
		Available: available,
		Kind:      &entityv1.Attributes_Light{Light: next},
	}
}
```

Update existing callers in `mapping_test.go` and the not-yet-updated callers in `cmd/hue-driver/main.go`. For the existing tests that don't care about availability, pass `true`.

- [ ] **Step 4: Run all state tests, expect pass**

```bash
go test ./drivers/hue/internal/state/...
```

- [ ] **Step 5: Build to find broken callers, fix**

```bash
go build ./drivers/hue/...
```

Likely callers in `cmd/hue-driver/main.go` need an `available` arg. Pass
`true` everywhere for now — Task 11 wires real reachability.

- [ ] **Step 6: Commit**

```bash
git add drivers/hue/
git commit -m "feat(hue): plumb available bool through state mapping"
```

---

## Task 7: `state.CommandToUpdate` — duration_ms, brightness=0, auto-on

**Files:**
- Modify: `drivers/hue/internal/state/mapping.go`
- Modify: `drivers/hue/internal/state/mapping_test.go`
- Modify: `drivers/hue/internal/bridge/types.go` (add `Dynamics`)

- [ ] **Step 1: Add `Dynamics` to bridge types**

In `types.go`:

```go
// Dynamics carries optional transition timing for a LightUpdate. Hue v2
// caps duration at ~6,000,000 ms (100 minutes).
type Dynamics struct {
	Duration uint32 `json:"duration"`
}
```

And add field to `LightUpdate`:

```go
type LightUpdate struct {
	On               *OnState          `json:"on,omitempty"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
	Dynamics         *Dynamics         `json:"dynamics,omitempty"`
}
```

- [ ] **Step 2: Add the failing tests**

Append to `mapping_test.go`:

```go
func TestCommandToUpdate_BrightnessZeroIsOff(t *testing.T) {
	got, err := CommandToUpdate("set_brightness", map[string]string{"brightness": "0"})
	if err != nil {
		t.Fatal(err)
	}
	if got.On == nil || got.On.On {
		t.Errorf("On = %+v, want On.On=false", got.On)
	}
	if got.Dimming != nil {
		t.Errorf("Dimming = %+v, want nil for brightness=0", got.Dimming)
	}
}

func TestCommandToUpdate_BrightnessNonZeroAutoOn(t *testing.T) {
	got, err := CommandToUpdate("set_brightness", map[string]string{"brightness": "128"})
	if err != nil {
		t.Fatal(err)
	}
	if got.On == nil || !got.On.On {
		t.Errorf("On = %+v, want On.On=true (auto-on for brightness>0)", got.On)
	}
	if got.Dimming == nil {
		t.Errorf("Dimming = nil, want set")
	}
}

func TestCommandToUpdate_ColorTempAutoOn(t *testing.T) {
	got, err := CommandToUpdate("set_color_temp", map[string]string{"color_temp": "366"})
	if err != nil {
		t.Fatal(err)
	}
	if got.On == nil || !got.On.On {
		t.Errorf("On = %+v, want On.On=true", got.On)
	}
}

func TestCommandToUpdate_DurationMs(t *testing.T) {
	cases := []struct {
		name string
		args map[string]string
		want uint32
	}{
		{"omitted", map[string]string{"brightness": "100"}, 0},
		{"explicit", map[string]string{"brightness": "100", "duration_ms": "5000"}, 5000},
		{"zero", map[string]string{"brightness": "100", "duration_ms": "0"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommandToUpdate("set_brightness", tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == 0 {
				if got.Dynamics != nil {
					t.Errorf("Dynamics = %+v, want nil for omitted duration", got.Dynamics)
				}
				return
			}
			if got.Dynamics == nil || got.Dynamics.Duration != tc.want {
				t.Errorf("Dynamics = %+v, want Duration=%d", got.Dynamics, tc.want)
			}
		})
	}
}

func TestCommandToUpdate_DurationMs_OutOfRange(t *testing.T) {
	for _, raw := range []string{"-1", "9000000", "abc"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := CommandToUpdate("turn_on", map[string]string{"duration_ms": raw}); err == nil {
				t.Errorf("expected error for duration_ms=%q", raw)
			}
		})
	}
}
```

- [ ] **Step 3: Run, expect failure**

- [ ] **Step 4: Implement**

Replace the `CommandToUpdate` switch body. Add a `parseDuration` helper:

```go
func parseDuration(args map[string]string) (uint32, error) {
	raw, ok := args["duration_ms"]
	if !ok {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 || v > 6_000_000 {
		return 0, fmt.Errorf("duration_ms must be integer 0-6000000, got %q", raw)
	}
	return uint32(v), nil
}

func CommandToUpdate(capability string, args map[string]string) (bridge.LightUpdate, error) {
	dur, err := parseDuration(args)
	if err != nil {
		return bridge.LightUpdate{}, err
	}
	var u bridge.LightUpdate
	if dur > 0 {
		u.Dynamics = &bridge.Dynamics{Duration: dur}
	}

	switch capability {
	case "turn_on":
		u.On = &bridge.OnState{On: true}
	case "turn_off":
		u.On = &bridge.OnState{On: false}
	case "set_brightness":
		raw, ok := args["brightness"]
		if !ok {
			return bridge.LightUpdate{}, fmt.Errorf("set_brightness: missing brightness arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 || v > 255 {
			return bridge.LightUpdate{}, fmt.Errorf("set_brightness: brightness must be integer 0-255, got %q", raw)
		}
		if v == 0 {
			u.On = &bridge.OnState{On: false}
		} else {
			u.On = &bridge.OnState{On: true}
			hue := float64(v) * 100 / 255
			u.Dimming = &bridge.Dimming{Brightness: hue}
		}
	case "set_color_temp":
		raw, ok := args["color_temp"]
		if !ok {
			return bridge.LightUpdate{}, fmt.Errorf("set_color_temp: missing color_temp arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 153 || v > 500 {
			return bridge.LightUpdate{}, fmt.Errorf("set_color_temp: color_temp must be integer 153-500 mireds, got %q", raw)
		}
		mirek := uint32(v)
		u.On = &bridge.OnState{On: true}
		u.ColorTemperature = &bridge.ColorTemperature{Mirek: &mirek}
	default:
		return bridge.LightUpdate{}, fmt.Errorf("unknown capability %q", capability)
	}
	return u, nil
}
```

- [ ] **Step 5: Run all state tests, expect pass**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(hue): duration_ms, brightness=0→off, auto-on on changes"
```

---

## Task 8: bridge auth-failure detection

**Files:**
- Modify: `drivers/hue/internal/bridge/client.go`
- Modify: `drivers/hue/internal/bridge/client_test.go`

- [ ] **Step 1: Write the failing test**

Append to `client_test.go`:

```go
func TestClient_AuthFailureCounting(t *testing.T) {
	var count atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	for i := 0; i < 2; i++ {
		if _, err := c.ListLights(context.Background()); err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}
	// Two strikes — not yet revoked.
	_, err := c.ListLights(context.Background())
	if errors.Is(err, bridge.ErrAuthRevoked) {
		t.Fatal("ErrAuthRevoked too early after 3 calls")
	}
	// The 3rd 401 trips the revocation. Subsequent call returns sentinel.
	_, err = c.ListLights(context.Background())
	if !errors.Is(err, bridge.ErrAuthRevoked) {
		t.Fatalf("expected ErrAuthRevoked after 3 401s, got %v", err)
	}
}
```

(`atomic`, `errors`, `bridge` may need to be added to test imports.)

- [ ] **Step 2: Run, expect failure**

- [ ] **Step 3: Implement**

In `client.go`, add the sentinel and tracker:

```go
// ErrAuthRevoked indicates the bridge has rejected the API key three or
// more times within 60s. The driver treats this as fatal.
var ErrAuthRevoked = errors.New("hue: api key rejected by bridge (re-pair required)")

type authFailureTracker struct {
	mu          sync.Mutex
	timestamps  []time.Time
}

func (t *authFailureTracker) record() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-60 * time.Second)
	pruned := t.timestamps[:0]
	for _, ts := range t.timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	t.timestamps = append(pruned, now)
}

func (t *authFailureTracker) revoked() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-60 * time.Second)
	count := 0
	for _, ts := range t.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}
	return count >= 3
}
```

Add `authTracker authFailureTracker` to `Client`. After every HTTP call,
if response is 401: `c.authTracker.record()`. Then before every call,
check `if c.authTracker.revoked() { return ErrAuthRevoked }`.

Refactor `ListLights` and `SetLight` to share a `do(req)` helper that
encapsulates the check + record dance.

- [ ] **Step 4: Run test, expect pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(hue): detect repeated auth failures with ErrAuthRevoked"
```

---

## Task 9: bridge command timeouts

**Files:**
- Modify: `drivers/hue/internal/bridge/client.go`
- Modify: `drivers/hue/internal/bridge/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestClient_SetLightTimeout(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	start := time.Now()
	err := c.SetLight(context.Background(), "id", LightUpdate{})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 7*time.Second {
		t.Errorf("took %v, want < 7s (timeout is 5s)", elapsed)
	}
}
```

- [ ] **Step 2: Implement**

In `SetLight`:

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

In `ListLights`:

```go
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()
```

- [ ] **Step 3: Test passes**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(hue): per-call timeouts on SetLight (5s) and ListLights (10s)"
```

---

## Task 10: bridge rate limiter

**Files:**
- Create: `drivers/hue/internal/bridge/ratelimit.go`
- Create: `drivers/hue/internal/bridge/ratelimit_test.go`
- Modify: `drivers/hue/internal/bridge/client.go` — call into limiter

- [ ] **Step 1: Add `golang.org/x/time/rate` dep**

```bash
go get golang.org/x/time/rate
```

- [ ] **Step 2: Write the failing test**

`drivers/hue/internal/bridge/ratelimit_test.go`:

```go
package bridge

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Paces(t *testing.T) {
	l := newRateLimiter(10, 10)
	ctx := context.Background()
	// Burn the burst.
	for i := 0; i < 10; i++ {
		if err := l.wait(ctx); err != nil {
			t.Fatalf("burst call %d: %v", i, err)
		}
	}
	// 11th must wait for a refill.
	start := time.Now()
	if err := l.wait(ctx); err != nil {
		t.Fatalf("11th call: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("11th call elapsed %v, want >= 50ms (refill)", elapsed)
	}
}
```

- [ ] **Step 3: Implement**

`drivers/hue/internal/bridge/ratelimit.go`:

```go
package bridge

import (
	"context"

	"golang.org/x/time/rate"
)

type rateLimiter struct {
	l *rate.Limiter
}

func newRateLimiter(perSec, burst int) *rateLimiter {
	return &rateLimiter{l: rate.NewLimiter(rate.Limit(perSec), burst)}
}

func (r *rateLimiter) wait(ctx context.Context) error {
	return r.l.Wait(ctx)
}
```

- [ ] **Step 4: Wire into Client**

In `client.go`, add `limiter *rateLimiter` to `Client`, initialise in
`New(...)` with `newRateLimiter(10, 10)`, and call `c.limiter.wait(ctx)`
at the top of the shared HTTP-do helper from Task 8.

- [ ] **Step 5: Run all bridge tests**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(hue): token-bucket rate limiter (10/sec, burst 10)"
```

---

## Task 11: bridge `ListDevices` + zigbee_connectivity correlation

**Files:**
- Create: `drivers/hue/internal/bridge/devices.go`
- Create: `drivers/hue/internal/bridge/devices_test.go`
- Create: `drivers/hue/internal/bridge/testdata/devices.json`
- Create: `drivers/hue/internal/bridge/testdata/zigbee_connectivity.json`
- Modify: `drivers/hue/internal/bridge/types.go` — add wire types

- [ ] **Step 1: Add the wire types**

In `types.go`:

```go
// Device is a single physical device on the bridge. Each device carries
// a list of services (light, button, zigbee_connectivity, etc.).
type Device struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Services []ResourceRef   `json:"services"`
}

// ResourceRef is the {rid, rtype} pointer used pervasively in CLIP v2.
type ResourceRef struct {
	RID   string `json:"rid"`
	RType string `json:"rtype"`
}

// ZigbeeConnectivity carries the bulb's reachability status.
type ZigbeeConnectivity struct {
	ID     string `json:"id"`
	Owner  ResourceRef `json:"owner"`
	Status string `json:"status"` // "connected" | "connectivity_issue" | "unreachable"
}
```

- [ ] **Step 2: Add fixture files**

`drivers/hue/internal/bridge/testdata/devices.json`:

```json
{
  "errors": [],
  "data": [
    {
      "id": "device-aaa",
      "type": "device",
      "services": [
        {"rid": "light-aaa", "rtype": "light"},
        {"rid": "zc-aaa", "rtype": "zigbee_connectivity"}
      ]
    },
    {
      "id": "device-bbb",
      "type": "device",
      "services": [
        {"rid": "light-bbb", "rtype": "light"},
        {"rid": "zc-bbb", "rtype": "zigbee_connectivity"}
      ]
    }
  ]
}
```

`drivers/hue/internal/bridge/testdata/zigbee_connectivity.json`:

```json
{
  "errors": [],
  "data": [
    {"id": "zc-aaa", "owner": {"rid": "device-aaa", "rtype": "device"}, "status": "connected"},
    {"id": "zc-bbb", "owner": {"rid": "device-bbb", "rtype": "device"}, "status": "unreachable"}
  ]
}
```

- [ ] **Step 3: Write the failing test**

`devices_test.go`:

```go
package bridge

import (
	"context"
	"net/http"
	"os"
	"testing"
)

func TestListDevices(t *testing.T) {
	devicesBody, _ := os.ReadFile("testdata/devices.json")
	zcBody, _ := os.ReadFile("testdata/zigbee_connectivity.json")
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/clip/v2/resource/device":
			_, _ = w.Write(devicesBody)
		case "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write(zcBody)
		default:
			http.NotFound(w, r)
		}
	}))
	got, err := c.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	want := map[string]string{
		"light-aaa": "connected",
		"light-bbb": "unreachable",
	}
	for lightID, wantStatus := range want {
		if got[lightID] != wantStatus {
			t.Errorf("light %s status = %q, want %q", lightID, got[lightID], wantStatus)
		}
	}
}
```

- [ ] **Step 4: Implement**

`devices.go`:

```go
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListDevices returns a map of light-resource-id → reachability status.
// Walks /clip/v2/resource/device and /clip/v2/resource/zigbee_connectivity,
// joining via the device's services list.
func (c *Client) ListDevices(ctx context.Context) (map[string]string, error) {
	var devicesEnv struct {
		Errors []struct{ Description string } `json:"errors"`
		Data   []Device                       `json:"data"`
	}
	if err := c.getJSON(ctx, "/clip/v2/resource/device", &devicesEnv); err != nil {
		return nil, err
	}
	if len(devicesEnv.Errors) > 0 {
		return nil, fmt.Errorf("hue: list devices: %s", devicesEnv.Errors[0].Description)
	}

	var zcEnv struct {
		Errors []struct{ Description string } `json:"errors"`
		Data   []ZigbeeConnectivity           `json:"data"`
	}
	if err := c.getJSON(ctx, "/clip/v2/resource/zigbee_connectivity", &zcEnv); err != nil {
		return nil, err
	}
	if len(zcEnv.Errors) > 0 {
		return nil, fmt.Errorf("hue: list zigbee_connectivity: %s", zcEnv.Errors[0].Description)
	}

	// Build: device_id → status.
	deviceStatus := map[string]string{}
	for _, zc := range zcEnv.Data {
		deviceStatus[zc.Owner.RID] = zc.Status
	}

	// Map device → first light service. Result keyed by light_id.
	out := map[string]string{}
	for _, d := range devicesEnv.Data {
		var lightID string
		for _, s := range d.Services {
			if s.RType == "light" {
				lightID = s.RID
				break
			}
		}
		if lightID == "" {
			continue
		}
		out[lightID] = deviceStatus[d.ID]
	}
	return out, nil
}

// getJSON is a small helper that issues a GET, applies the rate limiter,
// auth tracking, and timeout, then decodes JSON into v.
func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.limiter.wait(ctx); err != nil {
		return err
	}
	if c.authTracker.revoked() {
		return ErrAuthRevoked
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode == http.StatusUnauthorized {
		c.authTracker.record()
		return fmt.Errorf("hue: %s: 401", path)
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hue: %s: status %d: %s", path, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
```

Refactor `ListLights` to use `getJSON`. Add `time` and `io` imports as needed.

- [ ] **Step 5: Run all bridge tests**

- [ ] **Step 6: Commit**

```bash
git add drivers/hue/internal/bridge/
git commit -m "feat(hue): ListDevices for per-bulb reachability via zigbee_connectivity"
```

---

## Task 12: SSE handles `zigbee_connectivity` events

**Files:**
- Modify: `drivers/hue/internal/bridge/types.go` — `Event` accepts connectivity payload
- Modify: `drivers/hue/internal/bridge/events.go` — filter widens
- Modify: `drivers/hue/internal/bridge/events_test.go`
- Modify: `drivers/hue/internal/bridge/testdata/sse_stream.txt`

- [ ] **Step 1: Extend `Event` to carry connectivity status**

In `types.go`:

```go
type Event struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"` // "light" | "zigbee_connectivity"
	On               *OnState          `json:"on,omitempty"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
	Status           string            `json:"status,omitempty"` // for zigbee_connectivity
	Owner            *ResourceRef      `json:"owner,omitempty"`  // for zigbee_connectivity
}
```

- [ ] **Step 2: Update SSE filter**

In `events.go`, change:

```go
if ev.Type != "light" {
    continue
}
```

to:

```go
switch ev.Type {
case "light", "zigbee_connectivity":
default:
    continue
}
```

- [ ] **Step 3: Add SSE fixture frame**

Append a third event to `testdata/sse_stream.txt`:

```
id: 1:2
data: [{"creationtime":"2026-04-30T01:00:02Z","data":[{"id":"zc-aaa","type":"zigbee_connectivity","owner":{"rid":"device-aaa","rtype":"device"},"status":"unreachable"}],"id":"evt3","type":"update"}]

```

- [ ] **Step 4: Update test**

In `events_test.go` extend `TestEvents` to expect 3 events; the third has Type=zigbee_connectivity and Status=unreachable.

- [ ] **Step 5: Run tests, commit**

```bash
git commit -am "feat(hue): accept zigbee_connectivity events from SSE"
```

---

## Task 13: cmd/hue-driver — slog setup

**Files:**
- Modify: `drivers/hue/cmd/hue-driver/main.go`

- [ ] **Step 1: Replace `log.Printf` with slog**

At the top of `main`, after `loadConfig`:

```go
logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
	Level: parseLogLevel(os.Getenv("HUE_LOG_LEVEL")),
})).With(
	"instance_id", os.Getenv("GOHOME_CARPORT_INSTANCE_ID"),
	"bridge_address", cfg.Address,
)
slog.SetDefault(logger)
```

Helper:

```go
func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

Replace every `log.Printf("hue-driver: ...", ...)` with `slog.Info("...", "key", val, ...)`. Common substitutions:

| Before | After |
|---|---|
| `log.Printf("hue-driver: events: %v", err)` | `slog.Warn("sse stream error", "error", err)` |
| `log.Printf("hue-driver: emit %s: %v", id, err)` | `slog.Warn("emit state failed", "entity_id", id, "error", err)` |
| `log.Fatalf("hue-driver: config: %v", err)` | `slog.Error("config invalid", "error", err); os.Exit(1)` |

- [ ] **Step 2: Build and run tests**

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(hue): structured logging via slog"
```

---

## Task 14: cmd/hue-driver — wire reachability + capability filtering + DriverEvents

**Files:**
- Modify: `drivers/hue/cmd/hue-driver/main.go`
- Modify: `drivers/hue/cmd/hue-driver/main_test.go`

This is the integration task — connects everything together.

- [ ] **Step 1: Update `buildDriver` for per-bulb capabilities + reachability**

```go
func buildDriver(ctx context.Context, client *bridge.Client) (*driver.Driver, *stateCache, error) {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return nil, nil, err
	}
	statuses, err := client.ListDevices(ctx)
	if err != nil {
		return nil, nil, err
	}

	d := driver.New(driverName, driverVersion)
	cache := newStateCache()

	for _, l := range lights {
		registerBulb(d, cache, client, l, statuses[l.ID] == "connected")
	}
	return d, cache, nil
}

func registerBulb(d *driver.Driver, cache *stateCache, client *bridge.Client, l bridge.Light, available bool) error {
	entityID := state.EntityID(l)
	caps := state.Capabilities(l)
	attrs := state.LightToAttrs(l, available)
	if err := d.AddEntity(entityID, driver.EntitySpec{
		EntityType:   "light",
		FriendlyName: l.Metadata.Name,
		Capabilities: caps,
		InitialState: attrs,
	}); err != nil {
		return err
	}
	cache.byEntID[entityID] = attrs.GetLight()
	cache.byEntID[entityID].... // extend stateCache to track available bool too
	cache.hueToID[l.ID] = entityID

	hueID := l.ID
	for _, cap := range caps {
		cap := cap
		d.OnCapability(entityID, cap, func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error) {
			return handleCommand(ctx, client, cache, hueID, entityID, cap, args)
		})
	}
	return nil
}
```

The `stateCache` struct gains a per-entity `available bool` map:

```go
type stateCache struct {
	mu        sync.Mutex
	byEntID   map[string]*entityv1.Light
	available map[string]bool
	hueToID   map[string]string
}
```

`handleCommand` now reads available from the cache when emitting StateChanged via `state.MergeEvent`.

- [ ] **Step 2: Add diff to `resync`**

```go
func resync(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return err
	}
	statuses, _ := client.ListDevices(ctx) // best-effort

	seenHueIDs := map[string]bool{}
	for _, l := range lights {
		seenHueIDs[l.ID] = true
		available := statuses[l.ID] == "connected"

		cache.mu.Lock()
		entityID, known := cache.hueToID[l.ID]
		cache.mu.Unlock()

		if !known {
			// Hot-add.
			if err := registerBulb(d, cache, client, l, available); err != nil {
				slog.Warn("register new bulb failed", "hue_id", l.ID, "error", err)
				continue
			}
			_ = d.EmitDriverEvent("bulb_added", state.EntityID(l))
		} else {
			cache.mu.Lock()
			attrs := state.LightToAttrs(l, available)
			cache.byEntID[entityID] = attrs.GetLight()
			cache.available[entityID] = available
			cache.mu.Unlock()
			_ = d.EmitState(entityID, attrs)
		}
	}

	// Hot-remove.
	cache.mu.Lock()
	var removed []string
	for hueID, entityID := range cache.hueToID {
		if !seenHueIDs[hueID] {
			removed = append(removed, entityID)
			delete(cache.hueToID, hueID)
			delete(cache.byEntID, entityID)
			delete(cache.available, entityID)
		}
	}
	cache.mu.Unlock()
	for _, entityID := range removed {
		if err := d.UnregisterEntity(entityID); err != nil {
			slog.Warn("unregister failed", "entity_id", entityID, "error", err)
		}
		_ = d.EmitDriverEvent("bulb_removed", entityID)
	}

	return nil
}
```

- [ ] **Step 3: Periodic resync + DriverEvent emissions in event loop**

In `runEventLoop`, add a 5-minute periodic resync alongside the normal SSE loop:

```go
func runEventLoop(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) {
	go periodicResync(ctx, client, d, cache)
	backoff := time.Second
	for {
		start := time.Now()
		err := streamOnce(ctx, client, d, cache)
		if err != nil {
			slog.Warn("sse stream error", "error", err)
			if errors.Is(err, bridge.ErrAuthRevoked) {
				slog.Error("api key rejected — re-pair via button press")
				_ = d.EmitDriverEvent("auth_failed", "")
				os.Exit(2)
			}
		}
		if ctx.Err() != nil {
			return
		}
		downtime := time.Since(start)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if downtime > 5*time.Second {
			backoff = time.Second
		} else if backoff < 30*time.Second {
			backoff = backoff * 2
		}
		_ = d.EmitDriverEvent("sse_reconnected", fmt.Sprintf("%dms", downtime.Milliseconds()))
		if err := resync(ctx, client, d, cache); err != nil {
			slog.Warn("resync failed", "error", err)
		}
	}
}

func periodicResync(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := resync(ctx, client, d, cache); err != nil {
				slog.Warn("periodic resync failed", "error", err)
			}
		}
	}
}
```

- [ ] **Step 4: Handle zigbee_connectivity events in streamOnce**

`streamOnce` currently only handles light events. Extend:

```go
for ev := range ch {
	switch ev.Type {
	case "light":
		// existing path: merge into cache, emit StateChanged
	case "zigbee_connectivity":
		// look up which light this device owns, update cache.available,
		// emit StateChanged with refreshed attrs
		applyConnectivityEvent(d, cache, ev)
	}
}
```

`applyConnectivityEvent` walks `cache.deviceToLight` (a new map populated in registerBulb) to find which entityID the device owns, updates `cache.available[entityID]`, then emits a StateChanged with the cached light merged with new availability via `state.LightToAttrs`/`MergeEvent`.

For this to work, registerBulb needs to populate `cache.deviceToLight[device.ID] = entityID`. We get `device.ID` from `light.Owner.RID` in the bridge response, which means adding `Owner ResourceRef` to `bridge.Light` types.

- [ ] **Step 5: Add startup auth-fast-fail**

In `main`, after `bridge.New`:

```go
if _, err := client.ListLights(ctx); errors.Is(err, bridge.ErrAuthRevoked) || isHTTPStatus(err, 401) {
	slog.Error("api key rejected on startup", "error", err)
	_ = d.EmitDriverEvent("auth_failed", cfg.Address)
	os.Exit(2)
}
```

(Need a small `isHTTPStatus` helper since the first 401 won't trigger the 3-strike sentinel.)

- [ ] **Step 6: Add `bridge_unreachable` / `bridge_recovered` emits**

Add a counter of consecutive HTTP-network failures in `streamOnce` and command handlers. After 3, emit `bridge_unreachable`. On the next success, emit `bridge_recovered`. Track via a small struct on `*Driver`'s context.

- [ ] **Step 7: Build and run integration tests**

```bash
go test ./drivers/hue/... -race -count=1
```

Existing integration tests should still pass; new behaviour is exercised by Task 15.

- [ ] **Step 8: Commit**

```bash
git commit -am "feat(hue): wire reachability, hot-add/remove, periodic resync, DriverEvents"
```

---

## Task 15: integration tests for hot-add/remove and brightness/duration

**Files:**
- Modify: `drivers/hue/cmd/hue-driver/main_test.go`

- [ ] **Step 1: Add hot-add/remove test**

```go
func TestDriver_HotAddRemove(t *testing.T) {
	var lightsBody atomic.Value
	lightsBody.Store(`{"errors":[],"data":[{"id":"light-aaa","type":"light","metadata":{"name":"Original"},"on":{"on":true}}]}`)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/light":
			_, _ = w.Write([]byte(lightsBody.Load().(string)))
		case "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case "/eventstream/clip/v2":
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client, _ := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "k", true, bridge.WithHTTPClient(srv.Client()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, cache, _ := buildDriver(ctx, client)
	h := drivertest.New(t, d)
	defer h.Close()

	if got := len(h.Entities()); got != 1 {
		t.Fatalf("initial entities = %d, want 1", got)
	}

	// Swap fake bridge: add one bulb, remove the original.
	lightsBody.Store(`{"errors":[],"data":[{"id":"light-bbb","type":"light","metadata":{"name":"Added"},"on":{"on":true}}]}`)

	if err := resync(ctx, client, d, cache); err != nil {
		t.Fatalf("resync: %v", err)
	}

	// We can't easily query d.entities directly, but we can SendCommand:
	// turn_on for the original should now fail (entity unknown), and
	// turn_on for the new should succeed.
	res, _ := h.SendCommand(ctx, "light.hue_light-aa", "turn_on", nil)
	if res.GetOk() {
		t.Errorf("command to removed entity returned ok=true")
	}
	res, _ = h.SendCommand(ctx, "light.hue_light-bb", "turn_on", nil)
	if !res.GetOk() {
		t.Errorf("command to added entity returned ok=false")
	}
}
```

- [ ] **Step 2: Add brightness=0 test**

```go
func TestDriver_BrightnessZeroTurnsOff(t *testing.T) {
	var puts []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			puts = append(puts, string(body))
			w.WriteHeader(200)
		case r.URL.Path == "/clip/v2/resource/light":
			_, _ = w.Write([]byte(fakeBridgeListLightsBody)) // existing fixture
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/eventstream/clip/v2":
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)

	client, _ := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "k", true, bridge.WithHTTPClient(srv.Client()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, _ := buildDriver(ctx, client)
	h := drivertest.New(t, d)
	defer h.Close()

	if _, err := h.SendCommand(ctx, "light.hue_12345678", "set_brightness", map[string]string{"brightness": "0"}); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	if len(puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(puts))
	}
	body := puts[0]
	if !strings.Contains(body, `"on":{"on":false}`) {
		t.Errorf("body missing on:false: %s", body)
	}
	if strings.Contains(body, `"dimming"`) {
		t.Errorf("body should not include dimming for brightness=0: %s", body)
	}
}
```

- [ ] **Step 3: Add duration_ms passthrough test**

Similar shape; assert PUT body includes `"dynamics":{"duration":5000}` when `duration_ms=5000` arg is passed.

- [ ] **Step 4: Run tests**

```bash
go test ./drivers/hue/cmd/hue-driver/... -race -count=1
```

- [ ] **Step 5: Commit**

```bash
git commit -am "test(hue): integration tests for hot-add/remove, brightness=0, duration"
```

---

## Task 16: CLI shows availability column

**Files:**
- Modify: `internal/cli/state.go` (or wherever the state list/get formatter lives)

- [ ] **Step 1: Find the formatter and add an AVAIL column**

Locate the function that renders a row in `gohome state list`. Add a column that reads `attributes.GetAvailable()` and renders `✓` / `✗` (or `yes`/`no` for non-color terminals).

- [ ] **Step 2: Update any matching test fixtures**

If `internal/cli/state_test.go` has golden-output assertions, update.

- [ ] **Step 3: Build and smoke-test**

```bash
go build -o dist/gohome ./cmd/gohome
./dist/gohome state list
```

(Requires daemon with new driver running.)

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(cli): show availability column in state list"
```

---

## Task 17: Final verification

- [ ] **Step 1: Full hue test suite with race detector**

```bash
go test ./drivers/hue/... -race -count=1
```

- [ ] **Step 2: Workspace test**

```bash
go test ./... -count=1
```

- [ ] **Step 3: Lint**

```bash
PATH="$PATH:/Users/fdatoo/go/bin" golangci-lint run ./drivers/hue/... ./internal/carport/... ./gohome-driverkit/...
```

- [ ] **Step 4: Build all binaries**

```bash
go build -o dist/gohomed ./cmd/gohomed
go build -o dist/gohome ./cmd/gohome
go build -o ~/.local/share/gohome/bin/hue-driver ./drivers/hue/cmd/hue-driver
```

- [ ] **Step 5: Smoke against real bridge**

Stop running daemon, wipe `~/.local/share/gohome/{*.db*,gohomed.lock,gohomed.sock,carport/}`, restart `~/.config/gohome/run.sh`. Verify:

- `gohome state list` shows availability column populated.
- `gohome events tail` shows the new DriverEvents (`sse_reconnected`, `bulb_added`/`removed` if applicable).
- `gohome command send light.hue_<id> set_brightness --arg brightness=0` turns the bulb off.
- `gohome command send light.hue_<id> set_brightness --arg brightness=128 --arg duration_ms=5000` fades over 5s.

- [ ] **Step 6: Working tree clean**

```bash
git status
```

Expected: clean.
