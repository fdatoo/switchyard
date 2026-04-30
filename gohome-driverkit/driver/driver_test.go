package driver_test

import (
	"context"
	"testing"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"

	"github.com/fdatoo/gohome-driverkit/driver"
	"github.com/fdatoo/gohome-driverkit/drivertest"
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
	ents := h.Entities()
	if got := len(ents); got != 2 {
		t.Fatalf("Entities() len = %d, want 2", got)
	}

	// Each EntityRegistered must carry its DeviceId — the daemon uses this to
	// bind events to a specific entity. Empty DeviceId collapses every entity
	// into a single key, breaking multi-entity drivers.
	got := map[string]bool{}
	for _, e := range ents {
		got[e.GetDeviceId()] = true
	}
	for _, want := range []string{"light.a", "switch.b"} {
		if !got[want] {
			t.Errorf("EntityRegistered.DeviceId %q missing; got %v", want, got)
		}
	}
}

// TestDriver_OnRunStartEmitsInitialState guards the driverkit's contract that
// every entity with tracked attrs gets a StateChanged at the start of each Run
// stream. Without this, a daemon replaying old EntityRegistered events would
// keep a stale (or empty) state cache forever — the daemon treats registration
// as schema-level (idempotent) and trusts StateChanged for current truth.
func TestDriver_OnRunStartEmitsInitialState(t *testing.T) {
	d := driver.New("test", "0.0.1")
	want := lightAttrs(true, 200)
	if err := d.AddEntity("light.a", driver.EntitySpec{
		EntityType: "light", FriendlyName: "A",
		Capabilities: []string{"turn_on"},
		InitialState: want,
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEntity("light.b", driver.EntitySpec{
		EntityType: "light", FriendlyName: "B",
		Capabilities: []string{"turn_on"},
		// Note: no InitialState — should not produce a StateChanged.
	}); err != nil {
		t.Fatal(err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	deadline := time.After(time.Second)
	seen := map[string]bool{}
	for len(seen) < 1 {
		select {
		case sc := <-h.StateChanges():
			seen[sc.GetEntityId()] = true
			if sc.GetEntityId() == "light.b" {
				t.Errorf("got StateChanged for light.b which had no InitialState")
			}
		case <-deadline:
			t.Fatalf("timed out waiting for OnRunStart-driven StateChanges; saw %v", seen)
		}
	}
	if !seen["light.a"] {
		t.Errorf("expected StateChanged for light.a; saw %v", seen)
	}
}

// TestDriver_StateChangedCarriesEntityID guards against regressing the
// entity-id-propagation bug: every StateChanged emitted by the driver must
// carry its EntityId so the carport host can route it.
func TestDriver_StateChangedCarriesEntityID(t *testing.T) {
	d := driver.New("test", "0.0.1")
	for _, id := range []string{"light.a", "light.b"} {
		if err := d.AddEntity(id, lightSpec("turn_on")); err != nil {
			t.Fatal(err)
		}
		d.OnCapability(id, "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
			return lightAttrs(true, 100), nil
		})
	}

	h := drivertest.New(t, d)
	for _, id := range []string{"light.a", "light.b"} {
		res, err := h.SendCommand(context.Background(), id, "turn_on", nil)
		if err != nil || !res.GetOk() {
			t.Fatalf("turn_on %s: %v %v", id, res, err)
		}
	}

	// Drain the channel and assert each StateChanged carries the right EntityId.
	deadline := time.After(time.Second)
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case sc := <-h.StateChanges():
			if sc.GetEntityId() == "" {
				t.Fatalf("StateChanged with empty EntityId: %v", sc)
			}
			seen[sc.GetEntityId()] = true
		case <-deadline:
			t.Fatalf("timed out waiting for StateChanges; saw %v", seen)
		}
	}
	for _, want := range []string{"light.a", "light.b"} {
		if !seen[want] {
			t.Errorf("no StateChanged for %q; saw %v", want, seen)
		}
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
	// Brief pause — the gRPC server stays live across host disconnects, so the next
	// NewAtSocket simply opens a new host connection to the same server.
	time.Sleep(200 * time.Millisecond)

	// Connect h2 to the same socket.
	h2 := drivertest.NewAtSocket(t, h1.SocketPath)
	// Entity count should be the same after reconnect.
	if got := len(h2.Entities()); got != 1 {
		t.Errorf("after reconnect Entities() len = %d, want 1", got)
	}
}
