package state

import (
	"reflect"
	"sort"
	"testing"

	"github.com/fdatoo/switchyard/drivers/hue/internal/bridge"
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
		{
			name: "color-capable",
			in: bridge.Light{
				Dimming:          &bridge.Dimming{},
				ColorTemperature: &bridge.ColorTemperature{},
				Color:            &bridge.Color{},
			},
			want: []string{"set_brightness", "set_color", "set_color_temp", "turn_off", "turn_on"},
		},
		{
			name: "color-only no temp",
			in: bridge.Light{
				Dimming: &bridge.Dimming{},
				Color:   &bridge.Color{},
			},
			want: []string{"set_brightness", "set_color", "turn_off", "turn_on"},
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
