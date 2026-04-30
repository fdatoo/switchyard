// Package state translates between Philips Hue CLIP v2 resources and
// gohome entityv1 attributes. Pure functions, no I/O.
package state

import (
	"fmt"
	"math"
	"strconv"

	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
)

// EntityID returns the gohome entity ID for a Hue light. The first 8 chars
// of the Hue v2 stable resource UUID are deterministic across renames and
// short enough to read in logs.
func EntityID(l bridge.Light) string {
	id := l.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return "light.hue_" + id
}

// LightToAttrs builds a full entityv1.Attributes from a Hue light. Used at
// startup enumeration and when resyncing after an SSE drop.
func LightToAttrs(l bridge.Light) *entityv1.Attributes {
	light := &entityv1.Light{On: l.On.On}
	if l.Dimming != nil {
		light.Brightness = brightnessHueToGohome(l.Dimming.Brightness)
	}
	if l.ColorTemperature != nil && l.ColorTemperature.Mirek != nil {
		light.ColorTemp = *l.ColorTemperature.Mirek
	}
	return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: light}}
}

// brightnessHueToGohome converts Hue's 0-100 float to gohome's 0-255 uint32.
func brightnessHueToGohome(h float64) uint32 {
	if h < 0 {
		h = 0
	}
	if h > 100 {
		h = 100
	}
	return uint32(math.Round(h * 255 / 100))
}

// CommandToUpdate translates a Carport (capability, args) pair into a
// bridge.LightUpdate. Validates argument ranges; returns an error for
// unknown capabilities or malformed arguments.
func CommandToUpdate(capability string, args map[string]string) (bridge.LightUpdate, error) {
	switch capability {
	case "turn_on":
		on := bridge.OnState{On: true}
		return bridge.LightUpdate{On: &on}, nil
	case "turn_off":
		on := bridge.OnState{On: false}
		return bridge.LightUpdate{On: &on}, nil
	case "set_brightness":
		raw, ok := args["brightness"]
		if !ok {
			return bridge.LightUpdate{}, fmt.Errorf("set_brightness: missing brightness arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 || v > 255 {
			return bridge.LightUpdate{}, fmt.Errorf("set_brightness: brightness must be integer 0-255, got %q", raw)
		}
		hue := float64(v) * 100 / 255
		return bridge.LightUpdate{Dimming: &bridge.Dimming{Brightness: hue}}, nil
	case "set_color_temp":
		raw, ok := args["color_temp"]
		if !ok {
			return bridge.LightUpdate{}, fmt.Errorf("set_color_temp: missing color_temp arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 153 || v > 500 {
			return bridge.LightUpdate{}, fmt.Errorf("set_color_temp: color_temp must be integer 153-500 mireds, got %q", raw)
		}
		mirek := uint32(v)
		return bridge.LightUpdate{ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek}}, nil
	default:
		return bridge.LightUpdate{}, fmt.Errorf("unknown capability %q", capability)
	}
}

// MergeEvent applies a partial SSE event to the cached previous state and
// returns the full new attributes. The returned value is a fresh allocation;
// the prev pointer is not mutated.
func MergeEvent(prev *entityv1.Light, ev bridge.Event) *entityv1.Attributes {
	next := &entityv1.Light{
		On:         prev.GetOn(),
		Brightness: prev.GetBrightness(),
		ColorTemp:  prev.GetColorTemp(),
	}
	if ev.On != nil {
		next.On = ev.On.On
	}
	if ev.Dimming != nil {
		next.Brightness = brightnessHueToGohome(ev.Dimming.Brightness)
	}
	if ev.ColorTemperature != nil && ev.ColorTemperature.Mirek != nil {
		next.ColorTemp = *ev.ColorTemperature.Mirek
	}
	return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: next}}
}
