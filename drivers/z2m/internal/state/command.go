package state

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CommandToPayload translates a Carport (capability, args) pair into
// the JSON payload Z2M expects on <base>/<friendly>/set. Returns an
// error for unknown capabilities or out-of-range arguments — caller
// surfaces this as CARPORT_INTERNAL without hitting the network.
func CommandToPayload(capability string, args map[string]string) ([]byte, error) {
	switch capability {
	case "turn_on":
		return json.Marshal(map[string]any{"state": "ON"})
	case "turn_off":
		return json.Marshal(map[string]any{"state": "OFF"})
	case "set_brightness":
		raw, ok := args["brightness"]
		if !ok {
			return nil, fmt.Errorf("set_brightness: missing brightness arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 || v > 255 {
			return nil, fmt.Errorf("set_brightness: brightness must be integer 0-255, got %q", raw)
		}
		return json.Marshal(map[string]any{"brightness": v})
	case "set_color_temp":
		raw, ok := args["color_temp"]
		if !ok {
			return nil, fmt.Errorf("set_color_temp: missing color_temp arg")
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 153 || v > 500 {
			return nil, fmt.Errorf("set_color_temp: color_temp must be integer 153-500 mireds, got %q", raw)
		}
		return json.Marshal(map[string]any{"color_temp": v})
	case "set_color":
		hex, err := parseColorToHex(args)
		if err != nil {
			return nil, fmt.Errorf("set_color: %w", err)
		}
		return json.Marshal(map[string]any{"color": map[string]any{"hex": hex}})
	default:
		return nil, fmt.Errorf("unknown capability %q", capability)
	}
}

// parseColorToHex extracts an RGB triple from args (hex= or r/g/b=)
// and returns it as "#RRGGBB" — the format Z2M understands universally
// across vendors.
func parseColorToHex(args map[string]string) (string, error) {
	if h, ok := args["hex"]; ok {
		s := strings.TrimPrefix(h, "#")
		if len(s) != 6 {
			return "", fmt.Errorf("hex must be 6 chars (with optional leading #), got %q", h)
		}
		if _, err := strconv.ParseUint(s, 16, 32); err != nil {
			return "", fmt.Errorf("hex parse: %w", err)
		}
		return "#" + strings.ToUpper(s), nil
	}
	rs, hasR := args["r"]
	gs, hasG := args["g"]
	bs, hasB := args["b"]
	if !hasR && !hasG && !hasB {
		return "", fmt.Errorf("provide hex=#RRGGBB or r/g/b")
	}
	if !hasR || !hasG || !hasB {
		return "", fmt.Errorf("r, g, and b must all be set")
	}
	r, err := parseByte(rs, "r")
	if err != nil {
		return "", err
	}
	g, err := parseByte(gs, "g")
	if err != nil {
		return "", err
	}
	b, err := parseByte(bs, "b")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b), nil
}

func parseByte(s, name string) (uint8, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > 255 {
		return 0, fmt.Errorf("%s must be integer 0-255, got %q", name, s)
	}
	return uint8(v), nil
}
