package state

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandToPayloadTurnOnOff(t *testing.T) {
	for _, tc := range []struct {
		cap   string
		state string
	}{
		{"turn_on", "ON"},
		{"turn_off", "OFF"},
	} {
		got, err := CommandToPayload(tc.cap, nil)
		if err != nil {
			t.Fatalf("%s: err = %v", tc.cap, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(got, &decoded); err != nil {
			t.Fatalf("%s: decode = %v", tc.cap, err)
		}
		if decoded["state"] != tc.state {
			t.Errorf("%s: state = %v, want %q", tc.cap, decoded["state"], tc.state)
		}
	}
}

func TestCommandToPayloadSetBrightness(t *testing.T) {
	got, err := CommandToPayload("set_brightness", map[string]string{"brightness": "200"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(got, &decoded)
	if decoded["brightness"].(float64) != 200 {
		t.Errorf("brightness = %v, want 200", decoded["brightness"])
	}
}

func TestCommandToPayloadSetBrightnessOutOfRange(t *testing.T) {
	for _, raw := range []string{"-1", "256", "foo", ""} {
		_, err := CommandToPayload("set_brightness", map[string]string{"brightness": raw})
		if err == nil {
			t.Errorf("brightness=%q: expected error", raw)
		}
	}
}

func TestCommandToPayloadSetBrightnessMissing(t *testing.T) {
	_, err := CommandToPayload("set_brightness", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "brightness") {
		t.Errorf("expected missing-arg error; got %v", err)
	}
}

func TestCommandToPayloadSetColorTemp(t *testing.T) {
	got, err := CommandToPayload("set_color_temp", map[string]string{"color_temp": "300"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(got, &decoded)
	if decoded["color_temp"].(float64) != 300 {
		t.Errorf("color_temp = %v, want 300", decoded["color_temp"])
	}
}

func TestCommandToPayloadSetColorTempRange(t *testing.T) {
	for _, raw := range []string{"50", "1000", "abc"} {
		_, err := CommandToPayload("set_color_temp", map[string]string{"color_temp": raw})
		if err == nil {
			t.Errorf("color_temp=%q: expected error", raw)
		}
	}
}

func TestCommandToPayloadSetColorHex(t *testing.T) {
	got, err := CommandToPayload("set_color", map[string]string{"hex": "#FF8800"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(got, &decoded)
	color, ok := decoded["color"].(map[string]any)
	if !ok {
		t.Fatalf("color block missing or wrong type: %T %v", decoded["color"], decoded["color"])
	}
	if color["hex"] != "#FF8800" {
		t.Errorf("hex = %v, want #FF8800", color["hex"])
	}
}

func TestCommandToPayloadSetColorRGB(t *testing.T) {
	got, err := CommandToPayload("set_color", map[string]string{"r": "255", "g": "136", "b": "0"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(got, &decoded)
	color := decoded["color"].(map[string]any)
	if color["hex"] != "#FF8800" {
		t.Errorf("hex from rgb = %v, want #FF8800", color["hex"])
	}
}

func TestCommandToPayloadSetColorBadInput(t *testing.T) {
	for _, args := range []map[string]string{
		{},
		{"hex": "zz"},
		{"hex": "#FF"},
		{"r": "-1", "g": "0", "b": "0"},
		{"r": "256", "g": "0", "b": "0"},
		{"r": "0", "g": "0"}, // missing b
	} {
		if _, err := CommandToPayload("set_color", args); err == nil {
			t.Errorf("expected error for args %v", args)
		}
	}
}

func TestCommandToPayloadUnknownCapability(t *testing.T) {
	if _, err := CommandToPayload("set_warp_drive", nil); err == nil {
		t.Error("expected error for unknown capability")
	}
}
