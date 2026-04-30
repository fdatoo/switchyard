package bridge

import (
	"math"
	"testing"
)

func nearXY(t *testing.T, label string, got, want ColorXY, tol float64) {
	t.Helper()
	if math.Abs(got.X-want.X) > tol || math.Abs(got.Y-want.Y) > tol {
		t.Errorf("%s: got (%.4f, %.4f), want (%.4f, %.4f) tol=%v",
			label, got.X, got.Y, want.X, want.Y, tol)
	}
}

func TestRGBToXY_KnownValues(t *testing.T) {
	cases := []struct {
		name    string
		r, g, b uint8
		want    ColorXY
	}{
		// D65 white point. sRGB(255,255,255) → (0.3127, 0.3290).
		{"white", 255, 255, 255, ColorXY{0.3127, 0.3290}},
		// sRGB primaries roughly map to:
		{"red", 255, 0, 0, ColorXY{0.6400, 0.3300}},
		{"green", 0, 255, 0, ColorXY{0.3000, 0.6000}},
		{"blue", 0, 0, 255, ColorXY{0.1500, 0.0600}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RGBToXY(tc.r, tc.g, tc.b)
			nearXY(t, tc.name, got, tc.want, 0.01)
		})
	}
}

func TestXYToRGB_RoundTripsKnownValues(t *testing.T) {
	cases := []struct{ r, g, b uint8 }{
		{255, 255, 255},
		{255, 0, 0},
		{0, 255, 0},
		{0, 0, 255},
		{255, 136, 0}, // an arbitrary orange
	}
	for _, tc := range cases {
		xy := RGBToXY(tc.r, tc.g, tc.b)
		gr, gg, gb := XYToRGB(xy)
		// 2 LSB tolerance; gamma round-trip is lossy.
		check := func(name string, got, want uint8) {
			d := int(got) - int(want)
			if d < -2 || d > 2 {
				t.Errorf("round-trip rgb (%d,%d,%d) → xy → (%d,%d,%d): %s drift=%d",
					tc.r, tc.g, tc.b, gr, gg, gb, name, d)
			}
		}
		check("r", gr, tc.r)
		check("g", gg, tc.g)
		check("b", gb, tc.b)
	}
}

func TestClampToGamut_InsideUnchanged(t *testing.T) {
	gamutC := Gamut{
		Red:   ColorXY{0.6915, 0.3083},
		Green: ColorXY{0.1700, 0.7000},
		Blue:  ColorXY{0.1532, 0.0475},
	}
	inside := ColorXY{0.3, 0.3} // near white, well inside Gamut C
	got := ClampToGamut(inside, gamutC)
	nearXY(t, "inside", got, inside, 1e-9)
}

func TestClampToGamut_OutsideProjects(t *testing.T) {
	gamutC := Gamut{
		Red:   ColorXY{0.6915, 0.3083},
		Green: ColorXY{0.1700, 0.7000},
		Blue:  ColorXY{0.1532, 0.0475},
	}
	// A point well outside (large x, large y).
	outside := ColorXY{0.9, 0.9}
	got := ClampToGamut(outside, gamutC)
	// Result must be inside or on edge.
	if !insideTriangle(got, gamutC) {
		t.Fatalf("clamped point (%.4f, %.4f) not on/inside gamut", got.X, got.Y)
	}
}

func TestPackUnpackRGB(t *testing.T) {
	cases := [][3]uint8{
		{0, 0, 0},
		{255, 255, 255},
		{255, 136, 0},
		{18, 52, 86},
	}
	for _, c := range cases {
		packed := PackRGB(c[0], c[1], c[2])
		gr, gg, gb := UnpackRGB(packed)
		if gr != c[0] || gg != c[1] || gb != c[2] {
			t.Errorf("packed=0x%06X round-trip failed: in=(%d,%d,%d) out=(%d,%d,%d)",
				packed, c[0], c[1], c[2], gr, gg, gb)
		}
	}
	if got := PackRGB(0xFF, 0x88, 0x00); got != 0xFF8800 {
		t.Errorf("PackRGB(0xFF, 0x88, 0x00) = 0x%06X, want 0xFF8800", got)
	}
}

// insideTriangle is a barycentric inside-or-on-edge check used by tests.
func insideTriangle(p ColorXY, g Gamut) bool {
	const eps = 1e-9
	d := func(a, b, c ColorXY) float64 {
		return (a.X-c.X)*(b.Y-c.Y) - (b.X-c.X)*(a.Y-c.Y)
	}
	d1 := d(p, g.Red, g.Green)
	d2 := d(p, g.Green, g.Blue)
	d3 := d(p, g.Blue, g.Red)
	hasNeg := d1 < -eps || d2 < -eps || d3 < -eps
	hasPos := d1 > eps || d2 > eps || d3 > eps
	return !hasNeg || !hasPos
}
