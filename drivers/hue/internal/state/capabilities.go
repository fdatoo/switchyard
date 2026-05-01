package state

import (
	"github.com/fdatoo/switchyard/drivers/hue/internal/bridge"
)

// Capabilities returns the gohome capability strings the bulb supports,
// inferred from the presence of optional fields in the Hue v2 light
// resource. Every Hue light supports turn_on/turn_off; set_brightness
// requires the dimming block; set_color_temp requires the
// color_temperature block; set_color requires the color block.
func Capabilities(l bridge.Light) []string {
	caps := []string{"turn_on", "turn_off"}
	if l.Dimming != nil {
		caps = append(caps, "set_brightness")
	}
	if l.ColorTemperature != nil {
		caps = append(caps, "set_color_temp")
	}
	if l.Color != nil {
		caps = append(caps, "set_color")
	}
	return caps
}
