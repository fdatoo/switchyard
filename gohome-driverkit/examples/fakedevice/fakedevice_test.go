package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"

	"github.com/fdatoo/gohome-driverkit/driver"
	"github.com/fdatoo/gohome-driverkit/drivertest"
)

// newTestDriver mirrors main()'s setup but is strict about set_brightness args —
// unlike main.go which treats a missing brightness as a no-op (needed to let
// the generic drivertest CLI harness exercise every capability without args),
// this test driver rejects empty/invalid args so unit tests can assert the
// full validation path.
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
			return nil, fmt.Errorf("brightness must be 0-255")
		}
		mu.Lock()
		brightness = uint32(v)
		isOn := on
		mu.Unlock()
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
	_, _ = h.SendCommand(context.Background(), entityID, "turn_on", nil)
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
	_, _ = h1.SendCommand(context.Background(), entityID, "turn_on", nil)
	h1.Close()

	// Brief pause — the gRPC server stays live across host disconnects, so the next
	// NewAtSocket simply opens a new host connection to the same server.
	time.Sleep(200 * time.Millisecond)

	h2 := drivertest.NewAtSocket(t, h1.SocketPath)
	if len(h2.Entities()) != 1 {
		t.Errorf("reconnect: Entities() = %d, want 1", len(h2.Entities()))
	}
}
