// Package colorconv provides pure colour-space conversions for driver
// authors: CIE 1931 xy ↔ sRGB, HSV ↔ sRGB, gamut clamping, and packed
// RGB helpers. No I/O, no allocations beyond return values, no logging.
//
// Both first-party drivers (Hue, Z2M) consume this package; third-party
// drivers are encouraged to as well.
package colorconv

import "math"

// XY is a CIE 1931 chromaticity point. Both dimensions are 0..1.
// JSON-tagged so drivers can decode wire payloads directly.
type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Gamut is the triangle of representable colours for one bulb. Drivers
// without per-bulb gamut info pass the zero value, which disables
// clamping.
type Gamut struct {
	Red   XY `json:"red"`
	Green XY `json:"green"`
	Blue  XY `json:"blue"`
}

// PackRGB packs three bytes into a 0xRRGGBB uint32.
func PackRGB(r, g, b uint8) uint32 {
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// UnpackRGB unpacks a 0xRRGGBB uint32 into three bytes.
func UnpackRGB(packed uint32) (uint8, uint8, uint8) {
	return uint8(packed >> 16), uint8(packed >> 8), uint8(packed)
}

// RGBToXY converts an 8-bit-per-channel sRGB triple to a CIE 1931 xy
// chromaticity point. Standard sRGB → linear → CIE conversion with the
// D65 white reference. The returned point may fall outside any specific
// bulb's gamut; use ClampToGamut to project it back.
func RGBToXY(r, g, b uint8) XY {
	rf := gammaInverse(float64(r) / 255.0)
	gf := gammaInverse(float64(g) / 255.0)
	bf := gammaInverse(float64(b) / 255.0)

	X := rf*0.4124564 + gf*0.3575761 + bf*0.1804375
	Y := rf*0.2126729 + gf*0.7151522 + bf*0.0721750
	Z := rf*0.0193339 + gf*0.1191920 + bf*0.9503041

	sum := X + Y + Z
	if sum == 0 {
		return XY{0, 0}
	}
	return XY{X: X / sum, Y: Y / sum}
}

// XYToRGB converts a CIE 1931 xy point back to 8-bit sRGB, clamped to
// [0, 255]. Brightness is normalised so the brightest channel reaches
// 255 — callers control intensity separately via dimming.
func XYToRGB(xy XY) (uint8, uint8, uint8) {
	if xy.Y < 1e-9 {
		return 0, 0, 0
	}
	X := xy.X / xy.Y
	Y := 1.0
	Z := (1.0 - xy.X - xy.Y) / xy.Y

	rl := X*3.2404542 + Y*-1.5371385 + Z*-0.4985314
	gl := X*-0.9692660 + Y*1.8760108 + Z*0.0415560
	bl := X*0.0556434 + Y*-0.2040259 + Z*1.0572252

	maxC := math.Max(rl, math.Max(gl, bl))
	if maxC > 1.0 {
		rl, gl, bl = rl/maxC, gl/maxC, bl/maxC
	}
	return floatToByte(gammaForward(rl)), floatToByte(gammaForward(gl)), floatToByte(gammaForward(bl))
}

// ClampToGamut projects xy onto the gamut triangle if outside.
// Inside-or-on returns xy unchanged. A zero Gamut (all corners at 0,0)
// disables clamping — returns xy unchanged.
func ClampToGamut(xy XY, g Gamut) XY {
	if g.Red == (XY{}) && g.Green == (XY{}) && g.Blue == (XY{}) {
		return xy
	}
	if pointInTriangle(xy, g.Red, g.Green, g.Blue) {
		return xy
	}
	a := closestOnSegment(xy, g.Red, g.Green)
	b := closestOnSegment(xy, g.Green, g.Blue)
	c := closestOnSegment(xy, g.Blue, g.Red)
	best, bestD := a, distSq(xy, a)
	if d := distSq(xy, b); d < bestD {
		best, bestD = b, d
	}
	if d := distSq(xy, c); d < bestD {
		best = c
	}
	return best
}

// RGBToHSV converts 8-bit sRGB to HSV with hue in [0, 360), saturation
// and value in [0, 1]. Used by drivers whose target accepts {hue, sat}
// natively (some Z2M devices).
func RGBToHSV(r, g, b uint8) (h, s, v float64) {
	rf, gf, bf := float64(r)/255.0, float64(g)/255.0, float64(b)/255.0
	maxC := math.Max(rf, math.Max(gf, bf))
	minC := math.Min(rf, math.Min(gf, bf))
	v = maxC
	d := maxC - minC
	if maxC == 0 {
		return 0, 0, 0
	}
	s = d / maxC
	if d == 0 {
		return 0, s, v
	}
	switch maxC {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h *= 60
	return h, s, v
}

// HSVToRGB converts HSV (hue [0,360), saturation/value [0,1]) to 8-bit
// sRGB. Inputs outside their domains are clamped.
func HSVToRGB(h, s, v float64) (uint8, uint8, uint8) {
	if s < 0 {
		s = 0
	}
	if s > 1 {
		s = 1
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}
	return floatToByte(rf + m), floatToByte(gf + m), floatToByte(bf + m)
}

// --- internal helpers ---

func gammaInverse(c float64) float64 {
	if c > 0.04045 {
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return c / 12.92
}

func gammaForward(c float64) float64 {
	if c <= 0 {
		return 0
	}
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1.0/2.4) - 0.055
}

func floatToByte(f float64) uint8 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 255
	}
	return uint8(math.Round(f * 255))
}

func crossSign(a, b, c XY) float64 {
	return (a.X-c.X)*(b.Y-c.Y) - (b.X-c.X)*(a.Y-c.Y)
}

func pointInTriangle(p, a, b, c XY) bool {
	const eps = 1e-9
	d1 := crossSign(p, a, b)
	d2 := crossSign(p, b, c)
	d3 := crossSign(p, c, a)
	hasNeg := d1 < -eps || d2 < -eps || d3 < -eps
	hasPos := d1 > eps || d2 > eps || d3 > eps
	return !hasNeg || !hasPos
}

func closestOnSegment(p, a, b XY) XY {
	dx := b.X - a.X
	dy := b.Y - a.Y
	denom := dx*dx + dy*dy
	if denom == 0 {
		return a
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / denom
	switch {
	case t < 0:
		return a
	case t > 1:
		return b
	}
	return XY{a.X + t*dx, a.Y + t*dy}
}

func distSq(a, b XY) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}
