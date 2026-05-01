// Package state translates between Philips Hue CLIP v2 resources and
// gohome entityv1 attributes. Pure functions, no I/O.
package state

import (
	"fmt"
	"math"
	"strconv"

	"github.com/fdatoo/gohome-driverkit/colorconv"
	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
)

// colorToRgb is the bridge xy → packed gohome RGB conversion. Used by
// both LightToAttrs and MergeEvent to populate Light.ColorRgb.
func colorToRgb(xy bridge.ColorXY) uint32 {
	r, g, b := colorconv.XYToRGB(xy)
	return colorconv.PackRGB(r, g, b)
}

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
func LightToAttrs(l bridge.Light, available bool) *entityv1.Attributes {
	light := &entityv1.Light{On: l.On.On}
	if l.Dimming != nil {
		light.Brightness = brightnessHueToGohome(l.Dimming.Brightness)
	}
	switch {
	case l.Color != nil:
		light.ColorRgb = colorToRgb(l.Color.XY)
		// ColorTemp left zero (mutually exclusive).
	case l.ColorTemperature != nil && l.ColorTemperature.Mirek != nil:
		light.ColorTemp = *l.ColorTemperature.Mirek
	}
	return &entityv1.Attributes{
		Available: available,
		Kind:      &entityv1.Attributes_Light{Light: light},
	}
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

// parseDuration extracts and validates the optional "duration_ms" arg.
// Returns 0 if the arg is absent. Returns an error if the value is
// non-integer, negative, or exceeds Hue's 6,000,000 ms cap.
func parseDuration(args map[string]string) (uint32, error) {
	raw, ok := args["duration_ms"]
	if !ok {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 || v > 6_000_000 {
		return 0, fmt.Errorf("duration_ms must be integer 0-6000000, got %q", raw)
	}
	return uint32(v), nil
}

// CommandToUpdate translates a Carport (capability, args) pair into a
// bridge.LightUpdate. Validates argument ranges; returns an error for
// unknown capabilities or malformed arguments. gamut is used for the
// set_color case to clamp the computed xy point into the bulb's representable
// triangle; pass bridge.Gamut{} when no gamut is available.
func CommandToUpdate(capability string, args map[string]string, gamut bridge.Gamut) (bridge.LightUpdate, error) {
	dur, err := parseDuration(args)
	if err != nil {
		return bridge.LightUpdate{}, err
	}
	var u bridge.LightUpdate
	if dur > 0 {
		u.Dynamics = &bridge.Dynamics{Duration: dur}
	}

	switch capability {
	case "turn_on":
		u.On = &bridge.OnState{On: true}
	case "turn_off":
		u.On = &bridge.OnState{On: false}
	case "set_brightness":
		raw, ok := args["brightness"]
		if !ok {
			return bridge.LightUpdate{}, fmt.Errorf("set_brightness: missing brightness arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 || v > 255 {
			return bridge.LightUpdate{}, fmt.Errorf("set_brightness: brightness must be integer 0-255, got %q", raw)
		}
		if v == 0 {
			u.On = &bridge.OnState{On: false}
		} else {
			u.On = &bridge.OnState{On: true}
			hue := float64(v) * 100 / 255
			u.Dimming = &bridge.Dimming{Brightness: hue}
		}
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
		u.On = &bridge.OnState{On: true}
		u.ColorTemperature = &bridge.ColorTemperature{Mirek: &mirek}
	case "set_color":
		r, g, b, err := ParseColor(args)
		if err != nil {
			return bridge.LightUpdate{}, fmt.Errorf("set_color: %w", err)
		}
		raw := colorconv.RGBToXY(r, g, b)
		clamped := colorconv.ClampToGamut(raw, gamut)
		u.On = &bridge.OnState{On: true}
		u.Color = &bridge.ColorUpdate{XY: clamped}
	default:
		return bridge.LightUpdate{}, fmt.Errorf("unknown capability %q", capability)
	}
	return u, nil
}

// MergeEvent applies a partial SSE event to the cached previous state and
// returns the full new attributes. The returned value is a fresh allocation;
// the prev pointer is not mutated.
func MergeEvent(prev *entityv1.Light, ev bridge.Event, available bool) *entityv1.Attributes {
	next := &entityv1.Light{
		On:         prev.GetOn(),
		Brightness: prev.GetBrightness(),
		ColorTemp:  prev.GetColorTemp(),
		ColorRgb:   prev.GetColorRgb(),
	}
	if ev.On != nil {
		next.On = ev.On.On
	}
	if ev.Dimming != nil {
		next.Brightness = brightnessHueToGohome(ev.Dimming.Brightness)
	}
	switch {
	case ev.Color != nil:
		next.ColorRgb = colorToRgb(ev.Color.XY)
		next.ColorTemp = 0
	case ev.ColorTemperature != nil && ev.ColorTemperature.Mirek != nil:
		next.ColorTemp = *ev.ColorTemperature.Mirek
		next.ColorRgb = 0
	}
	return &entityv1.Attributes{
		Available: available,
		Kind:      &entityv1.Attributes_Light{Light: next},
	}
}
