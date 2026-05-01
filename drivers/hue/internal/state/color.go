package state

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseColor extracts an RGB triple from command args. Accepts either
// hex=#RRGGBB (with or without leading '#') or r=N g=N b=N (each 0-255).
// Hex takes precedence if both forms are present.
func ParseColor(args map[string]string) (uint8, uint8, uint8, error) {
	if hex, ok := args["hex"]; ok {
		return parseHex(hex)
	}
	rs, hasR := args["r"]
	gs, hasG := args["g"]
	bs, hasB := args["b"]
	if !hasR && !hasG && !hasB {
		return 0, 0, 0, fmt.Errorf("set_color: provide hex=#RRGGBB or r/g/b")
	}
	if !hasR || !hasG || !hasB {
		return 0, 0, 0, fmt.Errorf("set_color: r, g, and b must all be set")
	}
	r, err := parseByte(rs, "r")
	if err != nil {
		return 0, 0, 0, err
	}
	g, err := parseByte(gs, "g")
	if err != nil {
		return 0, 0, 0, err
	}
	b, err := parseByte(bs, "b")
	if err != nil {
		return 0, 0, 0, err
	}
	return r, g, b, nil
}

func parseHex(s string) (uint8, uint8, uint8, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("hex must be 6 chars (with optional leading #), got %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("hex parse: %w", err)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}

func parseByte(s, name string) (uint8, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > 255 {
		return 0, fmt.Errorf("%s must be integer 0-255, got %q", name, s)
	}
	return uint8(v), nil
}
