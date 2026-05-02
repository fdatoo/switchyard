package state

import (
	"math"
	"testing"

	"github.com/fdatoo/switchyard/drivers/hue/internal/bridge"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
)

func TestEntityID(t *testing.T) {
	cases := []struct {
		name string
		in   bridge.Light
		want string
	}{
		{
			name: "uses first 8 chars of UUID",
			in:   bridge.Light{ID: "12345678-90ab-cdef-1234-567890abcdef"},
			want: "light.hue_12345678",
		},
		{
			name: "stable across name changes",
			in:   bridge.Light{ID: "deadbeef-0000-0000-0000-000000000000", Metadata: bridge.LightMetadata{Name: "Renamed"}},
			want: "light.hue_deadbeef",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EntityID(tc.in)
			if got != tc.want {
				t.Fatalf("EntityID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLightToAttrs(t *testing.T) {
	mirek := uint32(366)
	cases := []struct {
		name string
		in   bridge.Light
		want *entityv1.Light
	}{
		{
			name: "on with brightness and color temp",
			in: bridge.Light{
				On:               bridge.OnState{On: true},
				Dimming:          &bridge.Dimming{Brightness: 50},
				ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek},
			},
			want: &entityv1.Light{On: true, Brightness: 128, ColorTemp: 366},
		},
		{
			name: "off, no dimming or color temp",
			in:   bridge.Light{On: bridge.OnState{On: false}},
			want: &entityv1.Light{On: false},
		},
		{
			name: "rounds brightness up",
			in: bridge.Light{
				On:      bridge.OnState{On: true},
				Dimming: &bridge.Dimming{Brightness: 100},
			},
			want: &entityv1.Light{On: true, Brightness: 255},
		},
		{
			name: "color_temperature with nil mirek (white-only bulb)",
			in: bridge.Light{
				On:               bridge.OnState{On: true},
				ColorTemperature: &bridge.ColorTemperature{Mirek: nil},
			},
			want: &entityv1.Light{On: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LightToAttrs(tc.in, true)
			gotLight := got.GetLight()
			if gotLight.GetOn() != tc.want.GetOn() ||
				gotLight.GetBrightness() != tc.want.GetBrightness() ||
				gotLight.GetColorTemp() != tc.want.GetColorTemp() {
				t.Fatalf("LightToAttrs = %+v, want %+v", gotLight, tc.want)
			}
		})
	}
}

func TestCommandToUpdate(t *testing.T) {
	cases := []struct {
		name      string
		cap       string
		args      map[string]string
		wantOn    *bool
		wantBri   *float64
		wantMirek *uint32
		wantErr   bool
	}{
		{name: "turn_on", cap: "turn_on", wantOn: ptr(true)},
		{name: "turn_off", cap: "turn_off", wantOn: ptr(false)},
		{
			name:    "set_brightness 128 → 50.196",
			cap:     "set_brightness",
			args:    map[string]string{"brightness": "128"},
			wantOn:  ptr(true),
			wantBri: ptrF((128.0 * 100) / 255),
		},
		{
			name:    "set_brightness missing arg",
			cap:     "set_brightness",
			args:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "set_brightness out of range",
			cap:     "set_brightness",
			args:    map[string]string{"brightness": "999"},
			wantErr: true,
		},
		{
			name:    "set_brightness non-integer",
			cap:     "set_brightness",
			args:    map[string]string{"brightness": "bright"},
			wantErr: true,
		},
		{
			name:      "set_color_temp 366",
			cap:       "set_color_temp",
			args:      map[string]string{"color_temp": "366"},
			wantOn:    ptr(true),
			wantMirek: ptrU(366),
		},
		{
			name:    "set_color_temp missing arg",
			cap:     "set_color_temp",
			args:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "unknown capability",
			cap:     "do_a_barrel_roll",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommandToUpdate(tc.cap, tc.args, bridge.Gamut{})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantOn != nil {
				if got.On == nil || got.On.On != *tc.wantOn {
					t.Fatalf("On = %+v, want On.On=%v", got.On, *tc.wantOn)
				}
			}
			if tc.wantBri != nil {
				if got.Dimming == nil || math.Abs(got.Dimming.Brightness-*tc.wantBri) > 0.01 {
					t.Fatalf("Dimming.Brightness = %+v, want %v", got.Dimming, *tc.wantBri)
				}
			}
			if tc.wantMirek != nil {
				if got.ColorTemperature == nil || got.ColorTemperature.Mirek == nil || *got.ColorTemperature.Mirek != *tc.wantMirek {
					t.Fatalf("ColorTemperature = %+v, want mirek=%v", got.ColorTemperature, *tc.wantMirek)
				}
			}
		})
	}
}

func TestMergeEvent(t *testing.T) {
	mirek := uint32(366)
	prev := &entityv1.Light{On: true, Brightness: 200, ColorTemp: 250}

	cases := []struct {
		name string
		ev   bridge.Event
		want *entityv1.Light
	}{
		{
			name: "on flips off, other fields preserved",
			ev:   bridge.Event{On: &bridge.OnState{On: false}},
			want: &entityv1.Light{On: false, Brightness: 200, ColorTemp: 250},
		},
		{
			name: "brightness only",
			ev:   bridge.Event{Dimming: &bridge.Dimming{Brightness: 50}},
			want: &entityv1.Light{On: true, Brightness: 128, ColorTemp: 250},
		},
		{
			name: "color temp only",
			ev:   bridge.Event{ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek}},
			want: &entityv1.Light{On: true, Brightness: 200, ColorTemp: 366},
		},
		{
			name: "no fields → unchanged copy",
			ev:   bridge.Event{},
			want: &entityv1.Light{On: true, Brightness: 200, ColorTemp: 250},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeEvent(prev, tc.ev, true).GetLight()
			if got.GetOn() != tc.want.GetOn() ||
				got.GetBrightness() != tc.want.GetBrightness() ||
				got.GetColorTemp() != tc.want.GetColorTemp() {
				t.Fatalf("MergeEvent = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLightToAttrs_Available(t *testing.T) {
	in := bridge.Light{On: bridge.OnState{On: true}}
	if attrs := LightToAttrs(in, true); !attrs.GetAvailable() {
		t.Errorf("Available = false, want true")
	}
	if attrs := LightToAttrs(in, false); attrs.GetAvailable() {
		t.Errorf("Available = true, want false")
	}
}

func TestMergeEvent_PropagatesAvailable(t *testing.T) {
	prev := &entityv1.Light{On: true, Brightness: 100}
	if merged := MergeEvent(prev, bridge.Event{}, true); !merged.GetAvailable() {
		t.Errorf("Available not propagated when true")
	}
	if merged := MergeEvent(prev, bridge.Event{}, false); merged.GetAvailable() {
		t.Errorf("Available not propagated when false")
	}
}

func TestCommandToUpdate_BrightnessZeroIsOff(t *testing.T) {
	got, err := CommandToUpdate("set_brightness", map[string]string{"brightness": "0"}, bridge.Gamut{})
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
	got, err := CommandToUpdate("set_brightness", map[string]string{"brightness": "128"}, bridge.Gamut{})
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
	got, err := CommandToUpdate("set_color_temp", map[string]string{"color_temp": "366"}, bridge.Gamut{})
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
			got, err := CommandToUpdate("set_brightness", tc.args, bridge.Gamut{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == 0 {
				if got.Dynamics != nil {
					t.Errorf("Dynamics = %+v, want nil for omitted/zero duration", got.Dynamics)
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
			if _, err := CommandToUpdate("turn_on", map[string]string{"duration_ms": raw}, bridge.Gamut{}); err == nil {
				t.Errorf("expected error for duration_ms=%q", raw)
			}
		})
	}
}

func TestCommandToUpdate_TurnOnWithDuration(t *testing.T) {
	got, err := CommandToUpdate("turn_on", map[string]string{"duration_ms": "5000"}, bridge.Gamut{})
	if err != nil {
		t.Fatal(err)
	}
	if got.On == nil || !got.On.On {
		t.Errorf("On = %+v, want On.On=true", got.On)
	}
	if got.Dynamics == nil || got.Dynamics.Duration != 5000 {
		t.Errorf("Dynamics = %+v, want Duration=5000", got.Dynamics)
	}
}

func TestCommandToUpdate_SetColorHex(t *testing.T) {
	gamut := bridge.Gamut{
		Red:   bridge.ColorXY{X: 0.6915, Y: 0.3083},
		Green: bridge.ColorXY{X: 0.1700, Y: 0.7000},
		Blue:  bridge.ColorXY{X: 0.1532, Y: 0.0475},
	}
	got, err := CommandToUpdate("set_color", map[string]string{"hex": "#FF8800"}, gamut)
	if err != nil {
		t.Fatal(err)
	}
	if got.On == nil || !got.On.On {
		t.Errorf("On = %+v, want On.On=true (auto-on)", got.On)
	}
	if got.Color == nil {
		t.Fatal("Color = nil, want set")
	}
	if got.ColorTemperature != nil {
		t.Errorf("ColorTemperature = %+v, want nil (mutually exclusive)", got.ColorTemperature)
	}
	// Sanity: xy is finite and inside gamut.
	if got.Color.XY.X < 0 || got.Color.XY.X > 1 || got.Color.XY.Y < 0 || got.Color.XY.Y > 1 {
		t.Errorf("xy = %+v, want both 0..1", got.Color.XY)
	}
}

func TestCommandToUpdate_SetColorRGB(t *testing.T) {
	got, err := CommandToUpdate("set_color", map[string]string{"r": "255", "g": "136", "b": "0"}, bridge.Gamut{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Color == nil {
		t.Fatal("Color = nil")
	}
}

func TestCommandToUpdate_SetColorBadHex(t *testing.T) {
	if _, err := CommandToUpdate("set_color", map[string]string{"hex": "zz"}, bridge.Gamut{}); err == nil {
		t.Error("expected error for malformed hex")
	}
}

func TestLightToAttrs_ColorRGB(t *testing.T) {
	in := bridge.Light{
		On:    bridge.OnState{On: true},
		Color: &bridge.Color{XY: bridge.ColorXY{X: 0.6400, Y: 0.3300}}, // pure red
	}
	got := LightToAttrs(in, true).GetLight()
	if got.GetColorRgb() == 0 {
		t.Errorf("ColorRgb = 0, want non-zero")
	}
	if got.GetColorTemp() != 0 {
		t.Errorf("ColorTemp = %d, want 0 (mutually exclusive with color)", got.GetColorTemp())
	}
}

func TestLightToAttrs_ColorTempZerosColorRGB(t *testing.T) {
	mirek := uint32(366)
	in := bridge.Light{
		On:               bridge.OnState{On: true},
		ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek},
	}
	got := LightToAttrs(in, true).GetLight()
	if got.GetColorTemp() != 366 {
		t.Errorf("ColorTemp = %d, want 366", got.GetColorTemp())
	}
	if got.GetColorRgb() != 0 {
		t.Errorf("ColorRgb = %d, want 0", got.GetColorRgb())
	}
}

func TestMergeEvent_ColorClearsTemp(t *testing.T) {
	prev := &entityv1.Light{ColorTemp: 366}
	merged := MergeEvent(prev, bridge.Event{
		Color: &bridge.Color{XY: bridge.ColorXY{X: 0.6400, Y: 0.3300}},
	}, true).GetLight()
	if merged.GetColorRgb() == 0 {
		t.Errorf("ColorRgb = 0, want non-zero")
	}
	if merged.GetColorTemp() != 0 {
		t.Errorf("ColorTemp = %d, want 0 after color update", merged.GetColorTemp())
	}
}

func TestMergeEvent_TempClearsColor(t *testing.T) {
	prev := &entityv1.Light{ColorRgb: 0xFF8800}
	mirek := uint32(366)
	merged := MergeEvent(prev, bridge.Event{
		ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek},
	}, true).GetLight()
	if merged.GetColorTemp() != 366 {
		t.Errorf("ColorTemp = %d, want 366", merged.GetColorTemp())
	}
	if merged.GetColorRgb() != 0 {
		t.Errorf("ColorRgb = %d, want 0 after color_temp update", merged.GetColorRgb())
	}
}

func ptr[T any](v T) *T       { return &v }
func ptrF(v float64) *float64 { return &v }
func ptrU(v uint32) *uint32   { return &v }
