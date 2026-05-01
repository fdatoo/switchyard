package z2m

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeBridgeDevices(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "bridge_devices.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var devices []Device
	if err := json.Unmarshal(raw, &devices); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, want := len(devices), 5; got != want {
		t.Fatalf("device count: got %d, want %d", got, want)
	}

	// Spot-check the colour light: it should expose a "light" composite
	// with four feature children.
	light := findDevice(devices, "kitchen_light")
	if light == nil {
		t.Fatal("kitchen_light not found")
	}
	if len(light.Definition.Exposes) != 2 {
		t.Errorf("kitchen_light top-level exposes: got %d, want 2", len(light.Definition.Exposes))
	}
	lightExpose := light.Definition.Exposes[0]
	if lightExpose.Type != "light" {
		t.Errorf("first expose type: got %q, want %q", lightExpose.Type, "light")
	}
	if len(lightExpose.Features) != 4 {
		t.Errorf("light features count: got %d, want 4", len(lightExpose.Features))
	}

	// Spot-check the multi-sensor.
	motion := findDevice(devices, "hallway_motion")
	if motion == nil {
		t.Fatal("hallway_motion not found")
	}
	occ := findExpose(motion.Definition.Exposes, "occupancy")
	if occ == nil {
		t.Fatal("occupancy expose not found")
	}
	if occ.Type != "binary" {
		t.Errorf("occupancy type: got %q, want %q", occ.Type, "binary")
	}
	if !occ.AccessPublished() {
		t.Error("occupancy AccessPublished=false")
	}
	if occ.AccessSettable() {
		t.Error("occupancy AccessSettable=true; expected read-only")
	}
}

func TestAccessBits(t *testing.T) {
	cases := []struct {
		access    uint8
		published bool
		settable  bool
	}{
		{0, false, false},
		{1, true, false},
		{2, false, true},
		{3, true, true},
		{7, true, true},
	}
	for _, tc := range cases {
		e := Expose{Access: tc.access}
		if got := e.AccessPublished(); got != tc.published {
			t.Errorf("Access=%d: AccessPublished=%v, want %v", tc.access, got, tc.published)
		}
		if got := e.AccessSettable(); got != tc.settable {
			t.Errorf("Access=%d: AccessSettable=%v, want %v", tc.access, got, tc.settable)
		}
	}
}

func TestDecodeBridgeState(t *testing.T) {
	var s BridgeStatePayload
	if err := json.Unmarshal([]byte(`{"state":"online"}`), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.State != "online" {
		t.Errorf("State = %q, want %q", s.State, "online")
	}
}

func TestDecodeStatePayload(t *testing.T) {
	raw := []byte(`{"state":"ON","brightness":128,"color_temp":250}`)
	var p StatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := len(p); got != 3 {
		t.Errorf("payload size: got %d, want 3", got)
	}
	if string(p["brightness"]) != "128" {
		t.Errorf("brightness raw: %q", string(p["brightness"]))
	}
}

func findDevice(devices []Device, name string) *Device {
	for i, d := range devices {
		if d.FriendlyName == name {
			return &devices[i]
		}
	}
	return nil
}

func findExpose(exposes []Expose, property string) *Expose {
	for i, e := range exposes {
		if e.Property == property {
			return &exposes[i]
		}
		if found := findExpose(e.Features, property); found != nil {
			return found
		}
	}
	return nil
}
