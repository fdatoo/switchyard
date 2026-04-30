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
