package state

import (
	"math"
	"testing"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
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
			got := LightToAttrs(tc.in)
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
			got, err := CommandToUpdate(tc.cap, tc.args)
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

func ptr[T any](v T) *T   { return &v }
func ptrF(v float64) *float64 { return &v }
func ptrU(v uint32) *uint32   { return &v }
