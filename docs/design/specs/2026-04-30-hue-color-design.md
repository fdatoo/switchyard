# Hue Color Support — Design

**Status:** draft
**Date:** 2026-04-30
**Branch:** `feat/hue-driver` (continues from the robustness pass)

## Summary

Color control for the Hue driver, end-to-end. After this lands, every
colour bulb on the bridge (Play, Lightstrip Plus, Bloom, Gradient, etc.)
accepts a new `set_color` capability with either a hex string or RGB
component arguments. The cached state surfaces colour as a packed RGB
field on `entityv1.Light`. One additive proto change benefits every
future colour-capable driver.

## Goals

- A single `set_color` capability accepts `hex=#FF8800` or `r=255 g=136 b=0`.
- Per-bulb gamut clamping at command-time, so the cached state matches
  what the bulb actually renders.
- xy → RGB conversion on inbound state (`ListLights`, SSE) so consumers
  see RGB without needing colour-science knowledge.
- Mutual exclusion with `set_color_temp`: whichever was last commanded
  is the active mode; the cached state zeroes the other dimension.
- Per-bulb capability filtering: only colour-capable bulbs advertise
  `set_color`.

## Non-goals

- HSV/HSL command surface (`set_color_hsv`). Additive later if needed.
- Per-bulb colour calibration / cross-vendor uniformity.
- Hue's effects/dynamics/signal blocks beyond the existing `duration_ms`
  passthrough.
- White-balance correction (`white_x` / `white_y` reference).
- A web UI colour picker. Out of scope; CLI is enough for v0.1.

## Architecture

### Cross-cutting proto change

In `proto/gohome/entity/v1/attributes.proto`, extend `Light`:

```proto
message Light {
  bool   on         = 1;
  uint32 brightness = 2;   // 0-255
  uint32 color_temp = 3;   // mireds; 0 if unsupported / not active
  uint32 color_rgb  = 4;   // 0xRRGGBB; 0 if unsupported / not active
}
```

Field 4 is additive. Packed `0xRRGGBB` keeps it in a single uint32 —
matches `color_temp`'s shape, JSON-serialises cleanly, and avoids a
nested `Color` message for one tiny payload.

`color_rgb` and `color_temp` are mutually exclusive at the *cached
state* level: only one is non-zero. Drivers enforce this when applying
commands and SSE events. There's no proto-level constraint; it's a
runtime invariant.

### Driver-internal layout

```
proto/gohome/entity/v1/attributes.proto    # +color_rgb on Light
gen/...                                     # regenerated
drivers/hue/
├── cmd/hue-driver/main.go                  # gamut cache, set_color routing
├── cmd/hue-driver/main_test.go             # set_color integration test
└── internal/
    ├── bridge/
    │   ├── types.go                        # Color, Gamut, ColorXY wire types
    │   ├── colormath.go                    # NEW — RGB↔xy + gamut clamp
    │   ├── colormath_test.go
    │   └── testdata/list_lights.json       # fixture gains color blocks
    └── state/
        ├── color.go                        # NEW — ParseColor (hex/r/g/b)
        ├── color_test.go
        ├── capabilities.go                 # +set_color for color-capable bulbs
        ├── capabilities_test.go
        ├── mapping.go                      # CommandToUpdate handles set_color
        └── mapping_test.go
```

## Components

### 1. Proto change

```proto
message Light {
  bool   on         = 1;
  uint32 brightness = 2;
  uint32 color_temp = 3;
  uint32 color_rgb  = 4;   // 0xRRGGBB
}
```

Additive. No changes to the daemon's state cache code (it already
round-trips `Attributes` via `proto.Clone`). Existing readers see
`color_rgb=0` for any entity whose driver doesn't populate it — which
is the same as "no color".

### 2. `bridge` wire types

In `drivers/hue/internal/bridge/types.go`:

```go
// Color is the Hue v2 color block. Carries an xy chromaticity point and
// the bulb's representable gamut. Both are present on every color-
// capable Light response.
type Color struct {
    XY    ColorXY  `json:"xy"`
    Gamut Gamut    `json:"gamut"`
    // GamutType ("A"|"B"|"C") is also returned by Hue but unused — the
    // explicit Gamut triangle is what matters.
}

// ColorXY is a CIE 1931 chromaticity point. Both dimensions are 0..1.
type ColorXY struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

// Gamut is the triangle of representable colors for one bulb model.
// Hue v2 returns three corners — red, green, blue.
type Gamut struct {
    Red   ColorXY `json:"red"`
    Green ColorXY `json:"green"`
    Blue  ColorXY `json:"blue"`
}
```

`bridge.Light` gains:

```go
type Light struct {
    // ... existing
    Color *Color `json:"color,omitempty"`
}
```

`bridge.LightUpdate` gains:

```go
type LightUpdate struct {
    // ... existing
    Color *ColorUpdate `json:"color,omitempty"`
}

// ColorUpdate is the body shape for a PUT — only xy, no gamut.
type ColorUpdate struct {
    XY ColorXY `json:"xy"`
}
```

### 3. `bridge.colormath`

`drivers/hue/internal/bridge/colormath.go` carries four pure functions:

```go
// RGBToXY converts an 8-bit-per-channel RGB triple to a CIE 1931 xy
// chromaticity point. Uses sRGB → CIE conversion (D65 white).
func RGBToXY(r, g, b uint8) ColorXY

// XYToRGB converts an xy point back to 8-bit RGB, clamped to [0,255].
// Brightness is preserved at full (1.0) — caller scales by the bulb's
// dimming separately if needed.
func XYToRGB(xy ColorXY) (r, g, b uint8)

// ClampToGamut projects xy onto the gamut triangle. If xy is already
// inside, returns it unchanged. If outside, returns the closest point
// on the triangle's edge.
func ClampToGamut(xy ColorXY, g Gamut) ColorXY

// Pack/Unpack helpers for the proto's 0xRRGGBB form.
func PackRGB(r, g, b uint8) uint32          // 0xRRGGBB
func UnpackRGB(packed uint32) (r, g, b uint8)
```

The math is well-documented online (Philips publishes the formulae);
roughly 80 lines including the gamma correction step. Tests use known
sRGB → xy values from the spec ("magenta = #FF00FF → xy ≈ (0.385, 0.155)").

### 4. `state.ParseColor`

`drivers/hue/internal/state/color.go`:

```go
// ParseColor extracts an RGB triple from command args. Accepts either
// hex=#RRGGBB (with or without leading '#') or r=N g=N b=N (each 0-255).
// Hex takes precedence if both forms are present. Returns (0,0,0,error)
// if neither form parses cleanly or values are out of range.
func ParseColor(args map[string]string) (r, g, b uint8, err error)
```

Hex parser accepts 6-char form only (no shortened `#F80` v0.1).

### 5. `state.Capabilities` extension

`Capabilities` adds `set_color` when `l.Color != nil`:

```go
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
```

### 6. `state.CommandToUpdate` extension

The `set_color` case lives inside `CommandToUpdate`, but it needs the
bulb's gamut to clamp xy. The function signature gains a gamut argument:

```go
func CommandToUpdate(capability string, args map[string]string, gamut bridge.Gamut) (bridge.LightUpdate, error)
```

Existing callers (which don't deal with colour) pass a zero `bridge.Gamut{}`
— functions like `RGBToXY` then `ClampToGamut(xy, zeroGamut)` collapse the
xy to (0,0) which is harmless because non-color paths don't read
`u.Color`.

Inside the switch, the new case:

```go
case "set_color":
    r, g, b, err := ParseColor(args)
    if err != nil {
        return bridge.LightUpdate{}, fmt.Errorf("set_color: %w", err)
    }
    raw := bridge.RGBToXY(r, g, b)
    clamped := bridge.ClampToGamut(raw, gamut)
    u.On = &bridge.OnState{On: true}                  // auto-on, like other setters
    u.Color = &bridge.ColorUpdate{XY: clamped}
```

Note: `u.ColorTemperature` is intentionally left nil so the bridge
doesn't get conflicting fields in one PUT (Hue rejects that).

### 7. `state.LightToAttrs` and `MergeEvent`

Both functions gain an inbound xy → RGB conversion when the bridge's
`Color` field is set:

```go
if l.Color != nil {
    rgb := bridge.PackRGB(bridge.XYToRGB(l.Color.XY))
    light.ColorRgb = rgb
    light.ColorTemp = 0  // mutual exclusion: a real color win zeros temp
}
```

Symmetric: when `l.ColorTemperature.Mirek != nil`, set `ColorTemp` and zero
`ColorRgb`. The two fields are never both non-zero in the cached state.

`MergeEvent` does the same for partial events: a `Color` update zeros
`ColorTemp`, and a `ColorTemperature` update zeros `ColorRgb`.

### 8. Hue driver wiring (`cmd/hue-driver/main.go`)

`stateCache` gains a per-entity gamut map:

```go
type stateCache struct {
    // ... existing
    gamuts map[string]bridge.Gamut  // entity_id → bulb's gamut
}
```

Populated in `registerBulb` from `l.Color.Gamut` (zero-value when the
bulb has no color block — `set_color` won't be advertised in that case
anyway).

`handleCommand` pulls the gamut from cache and passes it to `state.CommandToUpdate`:

```go
cache.mu.Lock()
gamut := cache.gamuts[entityID]
cache.mu.Unlock()
update, err := state.CommandToUpdate(capability, args, gamut)
```

`applyLightEvent` already uses `MergeEvent`; the xy→RGB conversion is
inside `MergeEvent` so no further wiring needed.

`resync` updates `cache.gamuts[entityID]` whenever it refreshes a bulb
(in case the bulb's gamut changes — vanishingly unlikely but cheap).

### 9. CLI surface

No CLI changes required. `state get` already shows `Light.color_rgb` via
`EmitUnpopulated` (R-Task 16's protojson change). `command send` takes
arbitrary `--arg` values; `set_color --arg hex=#FF8800` works through
the existing dispatch path.

If we ever want a friendlier CLI display, we'd format `color_rgb=0xff8800`
as `#ff8800` — but that's polish for later.

## Data flow

**Outbound (command):**
1. CLI: `gohome command send light.hue_xxx set_color --arg hex=#FF8800`
2. Daemon dispatches to driver; `OnCapability("set_color")` handler runs.
3. `handleCommand` looks up `cache.gamuts[entityID]`.
4. `state.CommandToUpdate("set_color", args, gamut)` → `state.ParseColor` → `bridge.RGBToXY` → `bridge.ClampToGamut` → `bridge.LightUpdate{On: ..., Color: ...}`.
5. `bridge.Client.SetLight` PUTs to `/clip/v2/resource/light/<id>`.
6. Optimistic cache merge: `state.MergeEvent` with the new color, `ColorRgb` set, `ColorTemp` zeroed.
7. SSE event arrives shortly with the bridge's confirmed xy (might differ slightly due to gamut clamping); `applyLightEvent` reconciles.

**Inbound (external change):**
- Wall switch / Hue app changes color → bridge emits SSE event with `color.xy`.
- `applyLightEvent` calls `MergeEvent` with the partial event.
- `MergeEvent` runs `bridge.XYToRGB` → packs into `color_rgb`, zeros `color_temp`.
- Driver emits `StateChanged` with the new attributes; daemon state cache updates.

## Error handling

**Bad colour input.** `ParseColor` returns errors for malformed hex, out-of-range component values, missing args. `CommandToUpdate` returns the error; the driverkit translates it into `CommandResult{Ok: false}`.

**Unsupported capability.** Bulbs without a `Color` block don't advertise `set_color`, so commands never arrive. If they did somehow (registry race), `OnCapability` returns `unsupported_capability`.

**Gamut clamping.** Always succeeds — even a zero `Gamut{}` produces a (degenerate) clamped result. `set_color` on a bulb without a Color block is gated by the capability list.

**SSE color events for non-color bulbs.** Shouldn't happen (bridge only emits color events for color bulbs), but if one arrives, `MergeEvent` writes `color_rgb` to the cached `entityv1.Light` regardless. The daemon doesn't care; the field is just a uint32.

## Lifecycle

No lifecycle changes. Color flows through the same enumeration / SSE / command paths as on/brightness/color_temp.

## Testing

**Unit (colormath).**
- `TestRGBToXY` — table of canonical values: white `(255,255,255)` → near `(0.3127, 0.3290)` (D65); pure red, green, blue at primaries.
- `TestXYToRGB` — round-trip the table; assert ≤1 LSB drift per channel.
- `TestClampToGamut_InsideUnchanged` — point inside Gamut C unchanged.
- `TestClampToGamut_OutsideProjects` — pure magenta `xy ≈ (0.385, 0.155)` clamped to nearest edge of Gamut C; result is on the edge (verify via cross-product test).
- `TestPackUnpackRGB` — round-trip preservation.

**Unit (state.ParseColor).**
- Hex with and without `#`.
- RGB triple.
- Out-of-range component (`r=300`).
- Both forms → hex wins.
- Neither form → error.

**Unit (state.Capabilities).**
- New case: `Light{Color: ...}` advertises `set_color`.

**Unit (state.CommandToUpdate).**
- `set_color hex=#FF8800` produces `LightUpdate{On: true, Color: {XY: ...}}`, no `color_temperature` field.
- `set_color` on a zero-gamut input still produces a non-error result (defensive).
- `set_color_temp` with an existing color cached: caller is responsible for zeroing `ColorRgb` via MergeEvent — covered in MergeEvent tests.

**Unit (state.LightToAttrs / MergeEvent).**
- A `bridge.Light{Color: ...}` produces `entityv1.Light{ColorRgb != 0, ColorTemp == 0}`.
- A `bridge.Light{ColorTemperature: ...}` produces `ColorRgb == 0, ColorTemp != 0`.
- A bridge.Event with Color clears the cached ColorTemp.
- A bridge.Event with ColorTemperature clears the cached ColorRgb.

**Integration (cmd/hue-driver).**
- `TestDriver_SetColorHex` — fake bridge serves a Light with a Color block + Gamut C; driver registers `set_color`; `SendCommand("set_color", "hex=#FF8800")` results in a PUT body containing `color.xy` near the expected value (within 0.01 tolerance, accounting for gamut clamping).
- `TestDriver_SetColorRGB` — same but with `r=255 g=136 b=0`.
- `TestDriver_SetColor_BadHex` — `hex=zz` returns `Ok=false`.

## Open items

None blocking.
