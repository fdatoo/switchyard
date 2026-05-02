package colorconv

import (
	"math"
	"testing"
)

func TestPackUnpackRGB(t *testing.T) {
	for _, tc := range []struct {
		r, g, b uint8
		packed  uint32
	}{
		{0, 0, 0, 0x000000},
		{255, 255, 255, 0xFFFFFF},
		{0xFF, 0x88, 0x00, 0xFF8800},
		{0x12, 0x34, 0x56, 0x123456},
	} {
		got := PackRGB(tc.r, tc.g, tc.b)
		if got != tc.packed {
			t.Errorf("PackRGB(%d,%d,%d) = %#x, want %#x", tc.r, tc.g, tc.b, got, tc.packed)
		}
		r, g, b := UnpackRGB(tc.packed)
		if r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("UnpackRGB(%#x) = (%d,%d,%d), want (%d,%d,%d)", tc.packed, r, g, b, tc.r, tc.g, tc.b)
		}
	}
}

func TestRGBXYRoundTrip(t *testing.T) {
	// Round-tripping primary colours must stay close. The conversion is
	// lossy because XY drops luminance, but pure primaries should
	// reconstruct to within 8 LSB after re-normalisation.
	for _, tc := range []struct {
		name    string
		r, g, b uint8
	}{
		{"red", 255, 0, 0},
		{"green", 0, 255, 0},
		{"blue", 0, 0, 255},
		{"orange", 0xFF, 0x88, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xy := RGBToXY(tc.r, tc.g, tc.b)
			r2, g2, b2 := XYToRGB(xy)
			if absDiff(r2, tc.r) > 8 || absDiff(g2, tc.g) > 8 || absDiff(b2, tc.b) > 16 {
				t.Errorf("round-trip %s: got (%d,%d,%d), want (%d,%d,%d)", tc.name, r2, g2, b2, tc.r, tc.g, tc.b)
			}
		})
	}
}

func TestRGBToXYBlack(t *testing.T) {
	xy := RGBToXY(0, 0, 0)
	if xy.X != 0 || xy.Y != 0 {
		t.Errorf("black → %v, want zero", xy)
	}
}

func TestClampToGamutInside(t *testing.T) {
	g := Gamut{
		Red:   XY{0.7, 0.3},
		Green: XY{0.2, 0.7},
		Blue:  XY{0.15, 0.05},
	}
	p := XY{0.4, 0.4}
	got := ClampToGamut(p, g)
	if got != p {
		t.Errorf("inside point modified: got %v, want %v", got, p)
	}
}

func TestClampToGamutOutside(t *testing.T) {
	g := Gamut{
		Red:   XY{0.7, 0.3},
		Green: XY{0.2, 0.7},
		Blue:  XY{0.15, 0.05},
	}
	// Far outside the triangle.
	got := ClampToGamut(XY{0.9, 0.9}, g)
	if !pointInTriangle(got, g.Red, g.Green, g.Blue) {
		t.Errorf("outside point not clamped into triangle: got %v", got)
	}
}

func TestClampToGamutZeroGamut(t *testing.T) {
	p := XY{0.9, 0.9}
	got := ClampToGamut(p, Gamut{})
	if got != p {
		t.Errorf("zero gamut should pass through: got %v, want %v", got, p)
	}
}

func TestHSVRGBRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		r, g, b uint8
	}{
		{"red", 255, 0, 0},
		{"green", 0, 255, 0},
		{"blue", 0, 0, 255},
		{"yellow", 255, 255, 0},
		{"grey", 128, 128, 128},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, s, v := RGBToHSV(tc.r, tc.g, tc.b)
			r2, g2, b2 := HSVToRGB(h, s, v)
			if absDiff(r2, tc.r) > 1 || absDiff(g2, tc.g) > 1 || absDiff(b2, tc.b) > 1 {
				t.Errorf("round-trip %s via HSV: got (%d,%d,%d), want (%d,%d,%d)", tc.name, r2, g2, b2, tc.r, tc.g, tc.b)
			}
		})
	}
}

func TestHSVToRGBKnownPoints(t *testing.T) {
	cases := []struct {
		h, s, v float64
		r, g, b uint8
	}{
		{0, 0, 0, 0, 0, 0},       // black
		{0, 0, 1, 255, 255, 255}, // white
		{0, 1, 1, 255, 0, 0},     // red
		{120, 1, 1, 0, 255, 0},   // green
		{240, 1, 1, 0, 0, 255},   // blue
	}
	for _, tc := range cases {
		r, g, b := HSVToRGB(tc.h, tc.s, tc.v)
		if r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("HSV(%g,%g,%g) → (%d,%d,%d), want (%d,%d,%d)", tc.h, tc.s, tc.v, r, g, b, tc.r, tc.g, tc.b)
		}
	}
}

func TestHSVOutOfDomainClamped(t *testing.T) {
	r, g, b := HSVToRGB(720, 2, 5) // hue wraps; sat/val clamp to 1
	r2, g2, b2 := HSVToRGB(0, 1, 1)
	if r != r2 || g != g2 || b != b2 {
		t.Errorf("clamped HSV mismatch: got (%d,%d,%d), want (%d,%d,%d)", r, g, b, r2, g2, b2)
	}
	// Negative inputs clamp to zero.
	r, g, b = HSVToRGB(0, -1, -1)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("negative HSV → (%d,%d,%d), want (0,0,0)", r, g, b)
	}
}

func absDiff(a, b uint8) int {
	d := int(a) - int(b)
	return int(math.Abs(float64(d)))
}
