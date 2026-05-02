package state

import (
	"encoding/json"
	"testing"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
)

func lightAttrs(on bool, brightness, colorTemp, colorRGB uint32) *entityv1.Attributes {
	return &entityv1.Attributes{
		Available: true,
		Kind: &entityv1.Attributes_Light{
			Light: &entityv1.Light{
				On: on, Brightness: brightness, ColorTemp: colorTemp, ColorRgb: colorRGB,
			},
		},
	}
}

func raw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestMergeStateLightOn(t *testing.T) {
	prev := lightAttrs(false, 0, 0, 0)
	got, err := MergeState(prev, "state", raw(t, "ON"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !got.GetLight().GetOn() {
		t.Error("expected on=true")
	}
}

func TestMergeStateLightOff(t *testing.T) {
	prev := lightAttrs(true, 200, 0, 0)
	got, _ := MergeState(prev, "state", raw(t, "OFF"))
	if got.GetLight().GetOn() {
		t.Error("expected on=false")
	}
	// Brightness preserved.
	if got.GetLight().GetBrightness() != 200 {
		t.Error("brightness should be preserved across on/off")
	}
}

func TestMergeStateLightBrightness(t *testing.T) {
	prev := lightAttrs(true, 0, 0, 0)
	got, _ := MergeState(prev, "brightness", raw(t, 128))
	if got.GetLight().GetBrightness() != 128 {
		t.Errorf("brightness = %d, want 128", got.GetLight().GetBrightness())
	}
}

func TestMergeStateLightColorTemp(t *testing.T) {
	prev := lightAttrs(true, 200, 0, 0xFF8800)
	got, _ := MergeState(prev, "color_temp", raw(t, 300))
	if got.GetLight().GetColorTemp() != 300 {
		t.Errorf("color_temp = %d, want 300", got.GetLight().GetColorTemp())
	}
	// Setting color_temp clears color_rgb (mutually exclusive).
	if got.GetLight().GetColorRgb() != 0 {
		t.Errorf("color_rgb should clear when color_temp set; got %#x", got.GetLight().GetColorRgb())
	}
}

func TestMergeStateLightColor(t *testing.T) {
	prev := lightAttrs(true, 200, 250, 0)
	// Z2M color block: {x: ..., y: ...}
	got, err := MergeState(prev, "color", raw(t, map[string]float64{"x": 0.6915, "y": 0.3083}))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// Resulting RGB should be approximately red (high R, low G, low B).
	rgb := got.GetLight().GetColorRgb()
	r := uint8(rgb >> 16)
	if r < 200 {
		t.Errorf("expected red-dominant; got %#x", rgb)
	}
	if got.GetLight().GetColorTemp() != 0 {
		t.Errorf("color_temp should clear when color set; got %d", got.GetLight().GetColorTemp())
	}
}

func TestMergeStateNumericSensor(t *testing.T) {
	prev := &entityv1.Attributes{
		Available: true,
		Kind: &entityv1.Attributes_NumericSensor{
			NumericSensor: &entityv1.NumericSensor{Unit: "°C"},
		},
	}
	got, err := MergeState(prev, "temperature", raw(t, 21.5))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.GetNumericSensor().GetValue() != 21.5 {
		t.Errorf("value = %g, want 21.5", got.GetNumericSensor().GetValue())
	}
	if got.GetNumericSensor().GetUnit() != "°C" {
		t.Errorf("unit dropped: got %q", got.GetNumericSensor().GetUnit())
	}
}

func TestMergeStateBinarySensor(t *testing.T) {
	prev := &entityv1.Attributes{
		Available: true,
		Kind:      &entityv1.Attributes_BinarySensor{BinarySensor: &entityv1.BinarySensor{}},
	}
	for _, tc := range []struct {
		raw  any
		want bool
	}{
		{true, true},
		{false, false},
	} {
		got, err := MergeState(prev, "occupancy", raw(t, tc.raw))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got.GetBinarySensor().GetOn() != tc.want {
			t.Errorf("on = %v, want %v (raw=%v)", got.GetBinarySensor().GetOn(), tc.want, tc.raw)
		}
	}
}

func TestMergeStateNilPrev(t *testing.T) {
	// nil prev should error rather than panic — the caller didn't
	// initialise InitialState, which is a programmer bug.
	if _, err := MergeState(nil, "state", raw(t, "ON")); err == nil {
		t.Error("expected error for nil prev")
	}
}

func TestMergeStateUnknownPropertyForLight(t *testing.T) {
	// Unknown light property → no-op (returns prev unchanged) without
	// error. Caller logs at debug.
	prev := lightAttrs(true, 200, 0, 0)
	got, err := MergeState(prev, "effect", raw(t, "blink"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != prev {
		t.Error("expected prev returned unchanged on unknown property")
	}
}
