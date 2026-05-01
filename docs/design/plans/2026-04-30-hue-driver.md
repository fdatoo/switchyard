# Hue Driver Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `drivers/hue/` — a Carport driver that mirrors a single Philips Hue bridge's lights into gohome as `light.*` entities, using CLIP v2 over HTTPS + server-sent events. White and tunable-white control only.

**Architecture:** Three units in the gohome root module. `internal/bridge` is the HTTPS+SSE client to the Hue CLIP v2 API. `internal/state` is pure Hue↔gohome translation (no I/O). `cmd/hue-driver` is wiring: read env, build the client, enumerate lights, register handlers with the driverkit, run an SSE goroutine that pushes state updates, and start the gRPC server.

**Tech Stack:** Go 1.25, `gohome-driverkit/driver`, `gohome/gen/gohome/entity/v1`, standard library `net/http` + `crypto/tls` + `bufio` (SSE parsing). Tests use `httptest.NewTLSServer` and the driverkit's `drivertest` harness.

---

## File Map

```
drivers/hue/
├── README.md                            # user-facing: pairing curl recipe, config, caveats
├── cmd/hue-driver/
│   ├── main.go                          # config loader, wiring, state cache, SSE goroutine
│   └── main_test.go                     # integration test: drivertest harness + httptest bridge
├── internal/bridge/
│   ├── types.go                         # Light, LightUpdate, Event (no behavior)
│   ├── client.go                        # Client, New, ListLights, SetLight
│   ├── client_test.go
│   ├── events.go                        # Client.Events — SSE reader
│   ├── events_test.go
│   └── testdata/
│       ├── list_lights.json             # fixture from a real bridge
│       └── sse_stream.txt               # canned SSE frames
└── internal/state/
    ├── mapping.go                       # EntityID, LightToAttrs, MergeEvent, CommandToUpdate
    └── mapping_test.go
```

### Files modified
- `docs/docs/drivers/first-party.md` — drop `poll_interval_s` and `use_clip_v2` from the Hue catalog entry.

### Key conventions

- Driver lives in the root `github.com/fdatoo/gohome` module under `drivers/hue/`. Package paths: `github.com/fdatoo/gohome/drivers/hue/internal/bridge`, `.../internal/state`, `.../cmd/hue-driver`.
- Config is read from environment variables passed by `gohomed`: `HUE_BRIDGE_ADDRESS`, `HUE_API_KEY`, `HUE_TLS_SKIP_VERIFY`.
- Brightness mapping: Hue is `float64` 0-100; gohome is `uint32` 0-255. Convert with `uint32(math.Round(hue * 255 / 100))` outbound and `math.Round(float64(gohome) * 100 / 255)` inbound.
- Entity IDs use the first 8 chars of the Hue v2 stable resource UUID: `light.hue_<short>`.
- All Hue API requests carry header `hue-application-key: <HUE_API_KEY>`.
- SSE endpoint requires `Accept: text/event-stream`.

---

## Task 1: Bootstrap directory and stub binary

**Files:**
- Create: `drivers/hue/cmd/hue-driver/main.go`
- Create: `drivers/hue/README.md` (placeholder)

- [ ] **Step 1: Create directories**

```bash
mkdir -p drivers/hue/cmd/hue-driver \
         drivers/hue/internal/bridge/testdata \
         drivers/hue/internal/state
```

- [ ] **Step 2: Write a stub `main.go` that compiles**

`drivers/hue/cmd/hue-driver/main.go`:

```go
// Command hue-driver is a Carport driver for the Philips Hue bridge.
// It mirrors all lights on one bridge into gohome as light.* entities
// over the CLIP v2 API (HTTPS + server-sent events).
package main

func main() {}
```

- [ ] **Step 3: Write a placeholder README**

`drivers/hue/README.md`:

```markdown
# driver-hue

Carport driver for the Philips Hue bridge. CLIP v2 only. White and tunable-white control.

Configuration and pairing instructions land in a later task.
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./drivers/hue/...`
Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/
git commit -m "feat(hue): scaffold driver directory"
```

---

## Task 2: Define bridge wire types

**Files:**
- Create: `drivers/hue/internal/bridge/types.go`

These are the JSON shapes used by the Hue CLIP v2 API. We define them once here so the `state` package and `Client` share the same vocabulary. No behavior — just structs.

- [ ] **Step 1: Write `types.go`**

```go
// Package bridge is the HTTPS + SSE client for the Philips Hue CLIP v2 API.
package bridge

// Light is a single light resource as returned by GET /clip/v2/resource/light.
// Only the fields we use are modeled; the bridge sends more.
type Light struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"` // always "light" for items in the lights collection
	Metadata         LightMetadata     `json:"metadata"`
	On               OnState           `json:"on"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
}

// LightMetadata carries the human-friendly bulb name set in the Hue app.
type LightMetadata struct {
	Name string `json:"name"`
}

// OnState models the bridge's nested {"on": bool} shape.
type OnState struct {
	On bool `json:"on"`
}

// Dimming carries the bulb's brightness in 0-100 float (Hue's native range).
type Dimming struct {
	Brightness float64 `json:"brightness"`
}

// ColorTemperature carries color temp in mireds. Mirek is null on bulbs that
// don't support color temperature.
type ColorTemperature struct {
	Mirek *uint32 `json:"mirek"`
}

// LightUpdate is the JSON body sent to PUT /clip/v2/resource/light/{id}.
// Pointer fields let us send only the keys we want to change.
type LightUpdate struct {
	On               *OnState          `json:"on,omitempty"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
}

// listLightsResponse is the envelope returned by GET /clip/v2/resource/light.
type listLightsResponse struct {
	Errors []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Data []Light `json:"data"`
}

// Event is a single resource-changed payload pulled from the SSE stream.
// Hue v2 events carry only the fields that changed.
type Event struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"` // resource type, e.g. "light"
	On               *OnState          `json:"on,omitempty"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./drivers/hue/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add drivers/hue/internal/bridge/types.go
git commit -m "feat(hue): define CLIP v2 wire types"
```

---

## Task 3: state.EntityID

**Files:**
- Create: `drivers/hue/internal/state/mapping.go`
- Create: `drivers/hue/internal/state/mapping_test.go`

- [ ] **Step 1: Write the failing test**

`drivers/hue/internal/state/mapping_test.go`:

```go
package state

import (
	"testing"

	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
)

func TestEntityID(t *testing.T) {
	cases := []struct {
		name string
		in   bridge.Light
		want string
	}{
		{
			name: "uses first 8 chars of UUID",
			in:   bridge.Light{ID: "12345678-90ab-cdef-1234-567890abcdef"},
			want: "light.hue_12345678",
		},
		{
			name: "stable across name changes",
			in:   bridge.Light{ID: "deadbeef-0000-0000-0000-000000000000", Metadata: bridge.LightMetadata{Name: "Renamed"}},
			want: "light.hue_deadbeef",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EntityID(tc.in)
			if got != tc.want {
				t.Fatalf("EntityID = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test, expect failure**

Run: `go test ./drivers/hue/internal/state/...`
Expected: FAIL — `undefined: EntityID`.

- [ ] **Step 3: Write minimal implementation**

`drivers/hue/internal/state/mapping.go`:

```go
// Package state translates between Philips Hue CLIP v2 resources and
// gohome entityv1 attributes. Pure functions, no I/O.
package state

import (
	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
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
```

- [ ] **Step 4: Run test, expect pass**

Run: `go test ./drivers/hue/internal/state/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/internal/state/
git commit -m "feat(hue): EntityID from Hue v2 UUID"
```

---

## Task 4: state.LightToAttrs

**Files:**
- Modify: `drivers/hue/internal/state/mapping.go`
- Modify: `drivers/hue/internal/state/mapping_test.go`

- [ ] **Step 1: Add the failing test**

Append to `mapping_test.go`:

```go
import (
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
)

func TestLightToAttrs(t *testing.T) {
	mirek := uint32(366)
	cases := []struct {
		name string
		in   bridge.Light
		want *entityv1.Light
	}{
		{
			name: "on with brightness and color temp",
			in: bridge.Light{
				On:               bridge.OnState{On: true},
				Dimming:          &bridge.Dimming{Brightness: 50},
				ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek},
			},
			want: &entityv1.Light{On: true, Brightness: 128, ColorTemp: 366},
		},
		{
			name: "off, no dimming or color temp",
			in:   bridge.Light{On: bridge.OnState{On: false}},
			want: &entityv1.Light{On: false},
		},
		{
			name: "rounds brightness up",
			in: bridge.Light{
				On:      bridge.OnState{On: true},
				Dimming: &bridge.Dimming{Brightness: 100},
			},
			want: &entityv1.Light{On: true, Brightness: 255},
		},
		{
			name: "color_temperature with nil mirek (white-only bulb)",
			in: bridge.Light{
				On:               bridge.OnState{On: true},
				ColorTemperature: &bridge.ColorTemperature{Mirek: nil},
			},
			want: &entityv1.Light{On: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LightToAttrs(tc.in)
			gotLight := got.GetLight()
			if gotLight.GetOn() != tc.want.GetOn() ||
				gotLight.GetBrightness() != tc.want.GetBrightness() ||
				gotLight.GetColorTemp() != tc.want.GetColorTemp() {
				t.Fatalf("LightToAttrs = %+v, want %+v", gotLight, tc.want)
			}
		})
	}
}
```

Combine the imports at the top of the file into a single `import (...)` block.

- [ ] **Step 2: Run test, expect failure**

Run: `go test ./drivers/hue/internal/state/...`
Expected: FAIL — `undefined: LightToAttrs`.

- [ ] **Step 3: Implement**

Append to `mapping.go`:

```go
import (
	"math"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
)

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
```

Combine the imports at the top of the file into a single `import (...)` block.

- [ ] **Step 4: Run test, expect pass**

Run: `go test ./drivers/hue/internal/state/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/internal/state/
git commit -m "feat(hue): map Hue Light to entityv1 attributes"
```

---

## Task 5: state.CommandToUpdate

Translates a Carport `(capability, args)` pair into a `bridge.LightUpdate`. Validates ranges. Returns errors for unknown capabilities or bad arguments.

**Files:**
- Modify: `drivers/hue/internal/state/mapping.go`
- Modify: `drivers/hue/internal/state/mapping_test.go`

- [ ] **Step 1: Add the failing test**

Append to `mapping_test.go`:

```go
func TestCommandToUpdate(t *testing.T) {
	cases := []struct {
		name      string
		cap       string
		args      map[string]string
		wantOn    *bool
		wantBri   *float64
		wantMirek *uint32
		wantErr   bool
	}{
		{name: "turn_on", cap: "turn_on", wantOn: ptr(true)},
		{name: "turn_off", cap: "turn_off", wantOn: ptr(false)},
		{
			name:    "set_brightness 128 → 50.196",
			cap:     "set_brightness",
			args:    map[string]string{"brightness": "128"},
			wantBri: ptrF((128.0 * 100) / 255),
		},
		{
			name:    "set_brightness missing arg",
			cap:     "set_brightness",
			args:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "set_brightness out of range",
			cap:     "set_brightness",
			args:    map[string]string{"brightness": "999"},
			wantErr: true,
		},
		{
			name:    "set_brightness non-integer",
			cap:     "set_brightness",
			args:    map[string]string{"brightness": "bright"},
			wantErr: true,
		},
		{
			name:      "set_color_temp 366",
			cap:       "set_color_temp",
			args:      map[string]string{"color_temp": "366"},
			wantMirek: ptrU(366),
		},
		{
			name:    "set_color_temp missing arg",
			cap:     "set_color_temp",
			args:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "unknown capability",
			cap:     "do_a_barrel_roll",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommandToUpdate(tc.cap, tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantOn != nil {
				if got.On == nil || got.On.On != *tc.wantOn {
					t.Fatalf("On = %+v, want On.On=%v", got.On, *tc.wantOn)
				}
			}
			if tc.wantBri != nil {
				if got.Dimming == nil || math.Abs(got.Dimming.Brightness-*tc.wantBri) > 0.01 {
					t.Fatalf("Dimming.Brightness = %+v, want %v", got.Dimming, *tc.wantBri)
				}
			}
			if tc.wantMirek != nil {
				if got.ColorTemperature == nil || got.ColorTemperature.Mirek == nil || *got.ColorTemperature.Mirek != *tc.wantMirek {
					t.Fatalf("ColorTemperature = %+v, want mirek=%v", got.ColorTemperature, *tc.wantMirek)
				}
			}
		})
	}
}

func ptr[T any](v T) *T   { return &v }
func ptrF(v float64) *float64 { return &v }
func ptrU(v uint32) *uint32   { return &v }
```

Add `"math"` to the test file's imports.

- [ ] **Step 2: Run test, expect failure**

Run: `go test ./drivers/hue/internal/state/...`
Expected: FAIL — `undefined: CommandToUpdate`.

- [ ] **Step 3: Implement**

Append to `mapping.go`:

```go
import (
	"fmt"
	"strconv"
)

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
```

Combine imports.

- [ ] **Step 4: Run test, expect pass**

Run: `go test ./drivers/hue/internal/state/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/internal/state/
git commit -m "feat(hue): translate Carport commands to LightUpdate"
```

---

## Task 6: state.MergeEvent

Hue v2 SSE events carry only the fields that changed. We merge each event into the cached previous state to produce the full new attributes.

**Files:**
- Modify: `drivers/hue/internal/state/mapping.go`
- Modify: `drivers/hue/internal/state/mapping_test.go`

- [ ] **Step 1: Add the failing test**

Append to `mapping_test.go`:

```go
func TestMergeEvent(t *testing.T) {
	mirek := uint32(366)
	prev := &entityv1.Light{On: true, Brightness: 200, ColorTemp: 250}

	cases := []struct {
		name string
		ev   bridge.Event
		want *entityv1.Light
	}{
		{
			name: "on flips off, other fields preserved",
			ev:   bridge.Event{On: &bridge.OnState{On: false}},
			want: &entityv1.Light{On: false, Brightness: 200, ColorTemp: 250},
		},
		{
			name: "brightness only",
			ev:   bridge.Event{Dimming: &bridge.Dimming{Brightness: 50}},
			want: &entityv1.Light{On: true, Brightness: 128, ColorTemp: 250},
		},
		{
			name: "color temp only",
			ev:   bridge.Event{ColorTemperature: &bridge.ColorTemperature{Mirek: &mirek}},
			want: &entityv1.Light{On: true, Brightness: 200, ColorTemp: 366},
		},
		{
			name: "no fields → unchanged copy",
			ev:   bridge.Event{},
			want: &entityv1.Light{On: true, Brightness: 200, ColorTemp: 250},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeEvent(prev, tc.ev).GetLight()
			if got.GetOn() != tc.want.GetOn() ||
				got.GetBrightness() != tc.want.GetBrightness() ||
				got.GetColorTemp() != tc.want.GetColorTemp() {
				t.Fatalf("MergeEvent = %+v, want %+v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test, expect failure**

Run: `go test ./drivers/hue/internal/state/...`
Expected: FAIL — `undefined: MergeEvent`.

- [ ] **Step 3: Implement**

Append to `mapping.go`:

```go
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
```

- [ ] **Step 4: Run test, expect pass**

Run: `go test ./drivers/hue/internal/state/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/internal/state/
git commit -m "feat(hue): merge partial SSE events into cached state"
```

---

## Task 7: bridge.Client and ListLights

**Files:**
- Create: `drivers/hue/internal/bridge/client.go`
- Create: `drivers/hue/internal/bridge/client_test.go`
- Create: `drivers/hue/internal/bridge/testdata/list_lights.json`

- [ ] **Step 1: Add the canned-response fixture**

`drivers/hue/internal/bridge/testdata/list_lights.json`:

```json
{
  "errors": [],
  "data": [
    {
      "id": "12345678-90ab-cdef-1234-567890abcdef",
      "type": "light",
      "metadata": { "name": "Kitchen" },
      "on": { "on": true },
      "dimming": { "brightness": 50.0 },
      "color_temperature": { "mirek": 366, "mirek_valid": true }
    },
    {
      "id": "deadbeef-0000-0000-0000-000000000000",
      "type": "light",
      "metadata": { "name": "Lamp" },
      "on": { "on": false }
    }
  ]
}
```

- [ ] **Step 2: Write the failing test**

`drivers/hue/internal/bridge/client_test.go`:

```go
package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	c, err := New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.httpClient = srv.Client()
	return c
}

func TestListLights(t *testing.T) {
	body, err := os.ReadFile("testdata/list_lights.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var (
		gotPath, gotKey string
	)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("hue-application-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))

	lights, err := c.ListLights(context.Background())
	if err != nil {
		t.Fatalf("ListLights: %v", err)
	}
	if gotPath != "/clip/v2/resource/light" {
		t.Errorf("path = %q, want /clip/v2/resource/light", gotPath)
	}
	if gotKey != "test-key" {
		t.Errorf("hue-application-key = %q, want test-key", gotKey)
	}
	if len(lights) != 2 {
		t.Fatalf("got %d lights, want 2", len(lights))
	}
	if lights[0].Metadata.Name != "Kitchen" {
		t.Errorf("lights[0].Metadata.Name = %q, want Kitchen", lights[0].Metadata.Name)
	}
	if lights[0].Dimming == nil || lights[0].Dimming.Brightness != 50.0 {
		t.Errorf("lights[0].Dimming = %+v, want brightness=50", lights[0].Dimming)
	}
}

func TestListLights_HTTPError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	if _, err := c.ListLights(context.Background()); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
```

- [ ] **Step 3: Run test, expect failure**

Run: `go test ./drivers/hue/internal/bridge/...`
Expected: FAIL — `undefined: New`, `undefined: Client.ListLights`.

- [ ] **Step 4: Implement**

`drivers/hue/internal/bridge/client.go`:

```go
package bridge

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client talks to a single Philips Hue bridge over CLIP v2. Safe for
// concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New constructs a Client. address is "<host>" or "<host>:<port>" — the
// CLIP v2 API is always HTTPS, so no scheme. apiKey is the bridge
// application key. tlsSkipVerify defaults to true in production because the
// bridge ships a self-signed cert.
func New(address, apiKey string, tlsSkipVerify bool) (*Client, error) {
	if address == "" {
		return nil, fmt.Errorf("bridge address is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	return &Client{
		baseURL: "https://" + address,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: tlsSkipVerify}, //nolint:gosec // bridge ships self-signed cert
			},
		},
	}, nil
}

// ListLights returns every light resource on the bridge.
func (c *Client) ListLights(ctx context.Context) ([]Light, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/clip/v2/resource/light", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hue: list lights: status %d: %s", resp.StatusCode, body)
	}
	var out listLightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("hue: decode list lights: %w", err)
	}
	return out.Data, nil
}
```

- [ ] **Step 5: Run test, expect pass**

Run: `go test ./drivers/hue/internal/bridge/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add drivers/hue/internal/bridge/
git commit -m "feat(hue): bridge.Client.ListLights"
```

---

## Task 8: bridge.Client.SetLight

**Files:**
- Modify: `drivers/hue/internal/bridge/client.go`
- Modify: `drivers/hue/internal/bridge/client_test.go`

- [ ] **Step 1: Add the failing test**

Append to `client_test.go`:

```go
func TestSetLight(t *testing.T) {
	type captured struct {
		path string
		body string
	}
	var got captured
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		got.body = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":[],"data":[{"rid":"12345678-90ab-cdef-1234-567890abcdef","rtype":"light"}]}`))
	}))

	on := OnState{On: true}
	dim := Dimming{Brightness: 50}
	err := c.SetLight(context.Background(), "12345678-90ab-cdef-1234-567890abcdef", LightUpdate{On: &on, Dimming: &dim})
	if err != nil {
		t.Fatalf("SetLight: %v", err)
	}
	if got.path != "/clip/v2/resource/light/12345678-90ab-cdef-1234-567890abcdef" {
		t.Errorf("path = %q", got.path)
	}
	if !strings.Contains(got.body, `"on":{"on":true}`) || !strings.Contains(got.body, `"brightness":50`) {
		t.Errorf("body = %s", got.body)
	}
}

func TestSetLight_HTTPError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	if err := c.SetLight(context.Background(), "id", LightUpdate{}); err == nil {
		t.Fatal("expected error on 400")
	}
}
```

Add `"io"` to the test file's imports if not already present.

- [ ] **Step 2: Run test, expect failure**

Run: `go test ./drivers/hue/internal/bridge/...`
Expected: FAIL — `undefined: Client.SetLight`.

- [ ] **Step 3: Implement**

Append to `client.go`:

```go
import (
	"bytes"
)

// SetLight applies an update to one light resource. Returns nil on 2xx,
// an error otherwise.
func (c *Client) SetLight(ctx context.Context, id string, update LightUpdate) error {
	body, err := json.Marshal(update)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/clip/v2/resource/light/"+id, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hue: set light %s: status %d: %s", id, resp.StatusCode, respBody)
	}
	return nil
}
```

Combine imports.

- [ ] **Step 4: Run test, expect pass**

Run: `go test ./drivers/hue/internal/bridge/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add drivers/hue/internal/bridge/
git commit -m "feat(hue): bridge.Client.SetLight"
```

---

## Task 9: bridge.Client.Events (SSE reader)

The reader opens `GET /eventstream/clip/v2`, parses `data:` frames, decodes each frame as a JSON array of "update" envelopes, and emits each inner resource update as a `bridge.Event` on the returned channel. The channel closes when the stream disconnects or `ctx.Done()` fires.

**Files:**
- Create: `drivers/hue/internal/bridge/events.go`
- Create: `drivers/hue/internal/bridge/events_test.go`
- Create: `drivers/hue/internal/bridge/testdata/sse_stream.txt`

- [ ] **Step 1: Add the SSE fixture**

`drivers/hue/internal/bridge/testdata/sse_stream.txt` (use `\n` line endings; one blank line between events):

```
id: 1:0
data: [{"creationtime":"2026-04-30T01:00:00Z","data":[{"id":"12345678-90ab-cdef-1234-567890abcdef","type":"light","on":{"on":false}}],"id":"evt1","type":"update"}]

id: 1:1
data: [{"creationtime":"2026-04-30T01:00:01Z","data":[{"id":"12345678-90ab-cdef-1234-567890abcdef","type":"light","dimming":{"brightness":75.0}}],"id":"evt2","type":"update"}]

```

- [ ] **Step 2: Write the failing test**

`drivers/hue/internal/bridge/events_test.go`:

```go
package bridge

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestEvents(t *testing.T) {
	body, err := os.ReadFile("testdata/sse_stream.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eventstream/clip/v2" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			http.Error(w, "missing Accept", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(body)
		flusher.Flush()
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := c.Events(ctx)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}

	got := make([]Event, 0, 2)
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(got), got)
	}
	if got[0].ID != "12345678-90ab-cdef-1234-567890abcdef" || got[0].On == nil || got[0].On.On {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Dimming == nil || got[1].Dimming.Brightness != 75.0 {
		t.Errorf("got[1].Dimming = %+v", got[1].Dimming)
	}
}
```

- [ ] **Step 3: Run test, expect failure**

Run: `go test ./drivers/hue/internal/bridge/...`
Expected: FAIL — `undefined: Client.Events`.

- [ ] **Step 4: Implement**

`drivers/hue/internal/bridge/events.go`:

```go
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// sseEnvelope is the outer JSON object emitted by the bridge in each
// "data:" frame. The inner Data slice carries the partial resource updates
// we care about.
type sseEnvelope struct {
	Type string  `json:"type"`
	Data []Event `json:"data"`
}

// Events opens the bridge's SSE stream and returns a channel of resource
// update events. The channel is closed when the stream disconnects, the
// HTTP body returns EOF, or ctx is cancelled. Callers should range over the
// channel until it closes; reconnect is the caller's responsibility.
func (c *Client) Events(ctx context.Context) (<-chan Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/eventstream/clip/v2", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("hue: events: status %d: %s", resp.StatusCode, body)
	}

	out := make(chan Event, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		// SSE frames can be larger than the default 64 KiB buffer.
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var envelopes []sseEnvelope
			if err := json.Unmarshal([]byte(payload), &envelopes); err != nil {
				// Malformed frame — skip and keep reading.
				continue
			}
			for _, env := range envelopes {
				if env.Type != "update" {
					continue
				}
				for _, ev := range env.Data {
					if ev.Type != "light" {
						continue
					}
					select {
					case out <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return out, nil
}
```

- [ ] **Step 5: Run test, expect pass**

Run: `go test ./drivers/hue/internal/bridge/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add drivers/hue/internal/bridge/
git commit -m "feat(hue): bridge.Client.Events SSE reader"
```

---

## Task 10: cmd/hue-driver wiring

Read env vars, build the bridge client, enumerate lights, register handlers + state cache, run the SSE goroutine, call `driver.Run`. No new public API; this is the binary's `main`.

**Files:**
- Modify: `drivers/hue/cmd/hue-driver/main.go`

- [ ] **Step 1: Replace the stub `main.go`**

```go
// Command hue-driver is a Carport driver for the Philips Hue bridge.
// It mirrors all lights on one bridge into gohome as light.* entities
// over the CLIP v2 API (HTTPS + server-sent events).
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	"github.com/fdatoo/gohome-driverkit/driver"

	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
	"github.com/fdatoo/gohome/drivers/hue/internal/state"
)

const driverName, driverVersion = "driver.hue", "0.1.0"

var capabilities = []string{"turn_on", "turn_off", "set_brightness", "set_color_temp"}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("hue-driver: config: %v", err)
	}

	client, err := bridge.New(cfg.Address, cfg.APIKey, cfg.TLSSkipVerify)
	if err != nil {
		log.Fatalf("hue-driver: bridge: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	d, cache, err := buildDriver(ctx, client)
	if err != nil {
		log.Fatalf("hue-driver: build: %v", err)
	}

	go runEventLoop(ctx, client, d, cache)

	if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("hue-driver: run: %v", err)
	}
}

// config holds parsed environment variables.
type config struct {
	Address       string
	APIKey        string
	TLSSkipVerify bool
}

func loadConfig() (config, error) {
	addr := os.Getenv("HUE_BRIDGE_ADDRESS")
	if addr == "" {
		return config{}, errors.New("HUE_BRIDGE_ADDRESS is required")
	}
	key := os.Getenv("HUE_API_KEY")
	if key == "" {
		return config{}, errors.New("HUE_API_KEY is required")
	}
	skip := true
	if v := os.Getenv("HUE_TLS_SKIP_VERIFY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return config{}, errors.New("HUE_TLS_SKIP_VERIFY must be a boolean")
		}
		skip = b
	}
	return config{Address: addr, APIKey: key, TLSSkipVerify: skip}, nil
}

// stateCache is the in-memory map of last-known full state per entity ID.
// Guarded by a single mutex; both command handlers and the SSE goroutine
// read+write it.
type stateCache struct {
	mu      sync.Mutex
	byEntID map[string]*entityv1.Light    // last known state per gohome entity ID
	hueToID map[string]string             // Hue resource UUID → gohome entity ID
}

func newStateCache() *stateCache {
	return &stateCache{
		byEntID: map[string]*entityv1.Light{},
		hueToID: map[string]string{},
	}
}

// buildDriver enumerates lights, registers each with the driverkit, and
// seeds the state cache. Returns the driver and cache; main wires them into
// the SSE goroutine.
func buildDriver(ctx context.Context, client *bridge.Client) (*driver.Driver, *stateCache, error) {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return nil, nil, err
	}

	d := driver.New(driverName, driverVersion)
	cache := newStateCache()

	for _, l := range lights {
		entityID := state.EntityID(l)
		if err := d.AddEntity(entityID, driver.EntitySpec{
			EntityType:   "light",
			FriendlyName: l.Metadata.Name,
			Capabilities: capabilities,
		}); err != nil {
			return nil, nil, err
		}
		cache.byEntID[entityID] = state.LightToAttrs(l).GetLight()
		cache.hueToID[l.ID] = entityID

		hueID := l.ID // pin loop variable
		for _, cap := range capabilities {
			cap := cap
			d.OnCapability(entityID, cap, func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error) {
				return handleCommand(ctx, client, cache, hueID, entityID, cap, args)
			})
		}
	}
	return d, cache, nil
}

func handleCommand(ctx context.Context, client *bridge.Client, cache *stateCache, hueID, entityID, capability string, args map[string]string) (*entityv1.Attributes, error) {
	update, err := state.CommandToUpdate(capability, args)
	if err != nil {
		return nil, err
	}
	if err := client.SetLight(ctx, hueID, update); err != nil {
		return nil, err
	}
	// Optimistically merge the command into cache. The bridge will also
	// emit an SSE event that confirms it; both paths produce the same
	// state, so this just reduces UI lag.
	cache.mu.Lock()
	prev := cache.byEntID[entityID]
	if prev == nil {
		prev = &entityv1.Light{}
	}
	merged := state.MergeEvent(prev, bridge.Event{
		On:               update.On,
		Dimming:          update.Dimming,
		ColorTemperature: update.ColorTemperature,
	})
	cache.byEntID[entityID] = merged.GetLight()
	cache.mu.Unlock()
	return merged, nil
}

// runEventLoop opens the SSE stream, applies events to the cache, and
// pushes StateChanged events into the driverkit. On disconnect it backs
// off (1s → 30s exponential), resyncs via ListLights, and reopens.
// Exits only on ctx.Done().
func runEventLoop(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) {
	backoff := time.Second
	for {
		if err := streamOnce(ctx, client, d, cache); err != nil {
			log.Printf("hue-driver: events: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
		// Resync state before reopening the stream.
		if err := resync(ctx, client, d, cache); err != nil {
			log.Printf("hue-driver: resync: %v", err)
		}
	}
}

func streamOnce(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	ch, err := client.Events(ctx)
	if err != nil {
		return err
	}
	for ev := range ch {
		cache.mu.Lock()
		entityID, ok := cache.hueToID[ev.ID]
		if !ok {
			cache.mu.Unlock()
			continue // unknown bulb (paired after startup) — out of scope for v0.1
		}
		prev := cache.byEntID[entityID]
		if prev == nil {
			prev = &entityv1.Light{}
		}
		merged := state.MergeEvent(prev, ev)
		cache.byEntID[entityID] = merged.GetLight()
		cache.mu.Unlock()

		if err := d.EmitState(entityID, merged); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			log.Printf("hue-driver: emit %s: %v", entityID, err)
		}
	}
	return nil
}

func resync(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return err
	}
	for _, l := range lights {
		cache.mu.Lock()
		entityID, ok := cache.hueToID[l.ID]
		if !ok {
			cache.mu.Unlock()
			continue
		}
		attrs := state.LightToAttrs(l)
		cache.byEntID[entityID] = attrs.GetLight()
		cache.mu.Unlock()
		if err := d.EmitState(entityID, attrs); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			log.Printf("hue-driver: emit resync %s: %v", entityID, err)
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify it builds and `go vet` is clean**

Run: `go build ./drivers/hue/... && go vet ./drivers/hue/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add drivers/hue/cmd/hue-driver/
git commit -m "feat(hue): wire driver, state cache, and SSE event loop"
```

---

## Task 11: Integration test against fake bridge

Drives the binary's `buildDriver` + command path end-to-end through the driverkit's `drivertest` harness.

**Files:**
- Create: `drivers/hue/cmd/hue-driver/main_test.go`

- [ ] **Step 1: Write the test**

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	carportv1alpha1 "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	"github.com/fdatoo/gohome-driverkit/drivertest"

	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
)

const fakeBridgeListLightsBody = `{
  "errors": [],
  "data": [
    {
      "id": "12345678-90ab-cdef-1234-567890abcdef",
      "type": "light",
      "metadata": { "name": "Kitchen" },
      "on": { "on": false },
      "dimming": { "brightness": 50 }
    }
  ]
}`

func TestDriver_TurnOnAndSetBrightness(t *testing.T) {
	var seenSetLight string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			seenSetLight = r.URL.Path
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/eventstream/clip/v2":
			// Hold the connection open until the harness closes the test.
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true)
	if err != nil {
		t.Fatalf("bridge.New: %v", err)
	}
	client.SetHTTPClientForTest(srv.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	const entityID = "light.hue_12345678"

	if _, err := h.SendCommand(ctx, &carportv1alpha1.Command{
		EntityId:   entityID,
		Capability: "turn_on",
	}); err != nil {
		t.Fatalf("turn_on: %v", err)
	}
	if _, err := h.SendCommand(ctx, &carportv1alpha1.Command{
		EntityId:   entityID,
		Capability: "set_brightness",
		Args:       map[string]string{"brightness": "128"},
	}); err != nil {
		t.Fatalf("set_brightness: %v", err)
	}

	if !strings.HasSuffix(seenSetLight, "/12345678-90ab-cdef-1234-567890abcdef") {
		t.Fatalf("expected SetLight call to bridge, got %q", seenSetLight)
	}
}
```

- [ ] **Step 2: Add the test seam to `bridge.Client`**

Add to `drivers/hue/internal/bridge/client.go`:

```go
// SetHTTPClientForTest swaps the underlying *http.Client. Tests use this
// to inject httptest.NewTLSServer's pre-trusted client so calls to the
// fake bridge don't fail TLS verification regardless of skip-verify.
func (c *Client) SetHTTPClientForTest(h *http.Client) {
	c.httpClient = h
}
```

- [ ] **Step 3: Run test**

Run: `go test ./drivers/hue/cmd/hue-driver/... -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add drivers/hue/
git commit -m "test(hue): integration test via drivertest + fake bridge"
```

---

## Task 12: User-facing README

**Files:**
- Modify: `drivers/hue/README.md`

- [ ] **Step 1: Write the README**

Replace the placeholder content with:

````markdown
# driver-hue

GoHome Carport driver for the [Philips Hue](https://www.philips-hue.com/) bridge.

- CLIP v2 only (Hue bridge firmware 1.48+, shipped 2021).
- One driver instance = one bridge.
- White and tunable-white control: `turn_on`, `turn_off`, `set_brightness`, `set_color_temp`.

## Quick start

### 1. Find your bridge

If you have one bridge on your network, the simplest path is the Philips discovery cloud endpoint:

```bash
curl -s https://discovery.meethue.com | jq
# [{"id":"...","internalipaddress":"192.168.1.10","port":443}]
```

Or browse to your router's DHCP table and look for a device named `Philips-hue`.

### 2. Get an API key

Press the round button on top of the bridge, then within 30 seconds:

```bash
curl -k -X POST https://192.168.1.10/api \
  -H 'Content-Type: application/json' \
  -d '{"devicetype":"gohome#hue-driver","generateclientkey":true}'
# [{"success":{"username":"<your-api-key>","clientkey":"..."}}]
```

The `username` field is your API key. Store it in your secret manager and reference it in the driver config.

### 3. Configure the driver

The driver reads three environment variables, set by `gohomed`:

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `HUE_BRIDGE_ADDRESS` | yes | — | IP or hostname of the bridge. |
| `HUE_API_KEY` | yes | — | Application key from step 2. |
| `HUE_TLS_SKIP_VERIFY` | no | `true` | The bridge ships a self-signed cert. |

## Known caveats

- New bulbs paired after the driver starts are not picked up until the driver restarts.
- Color bulbs (RGB/XY) are listed and can be turned on/off, but color isn't controllable yet — the gohome `Light` proto doesn't carry color fields.
- Groups, scenes, motion sensors, and dimmer switches are out of scope for v0.1.
````

- [ ] **Step 2: Commit**

```bash
git add drivers/hue/README.md
git commit -m "docs(hue): user-facing README with pairing recipe"
```

---

## Task 13: Update first-party catalog page

The published catalog entry advertises `poll_interval_s` and `use_clip_v2`, neither of which the driver implements. Trim it to match.

**Files:**
- Modify: `docs/docs/drivers/first-party.md`

- [ ] **Step 1: Replace the Hue config table**

Find the `### Hue (\`driver.hue\`)` section and replace its config-fields table and the bullet that mentions CLIP v1 fallback. The table should become:

```markdown
| Field | Type | Required | Description |
|---|---|---|---|
| `bridge_address` | `string` | yes | IP address or hostname of the Hue bridge |
| `api_key_env` | `string` | yes | Env var containing the Hue API key (obtained via the curl recipe in the driver README) |
| `tls_skip_verify` | `bool` | no | Skip TLS verification for the CLIP v2 HTTPS endpoint. Defaults to `true` (bridge uses self-signed cert) |
```

Replace the first "Known caveats" bullet ("CLIP v2 (default) supports server-sent events…") with:

```markdown
- CLIP v2 only — bridges on firmware older than 1.48 (pre-2021) are not supported.
- Server-sent events deliver state changes from wall switches and the Hue app to gohome with sub-second latency.
```

- [ ] **Step 2: Verify the docs site still builds (if you have Zensical locally)**

Run from `docs/`: `zensical build` (skip if not installed).
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add docs/docs/drivers/first-party.md
git commit -m "docs: trim Hue catalog entry to v0.1 config surface"
```

---

## Task 14: Final verification

- [ ] **Step 1: Full test suite**

Run from repo root: `go test ./drivers/hue/... -race`
Expected: PASS.

- [ ] **Step 2: Lint**

Run: `golangci-lint run ./drivers/hue/...`
Expected: clean. If `bodyclose` or `gosec` complain about TLS skip-verify, the inline `//nolint:gosec` in `client.go` should cover it; otherwise fix the actual issue rather than adding more nolints.

- [ ] **Step 3: Workspace-wide check**

Run: `task test`
Expected: PASS.

- [ ] **Step 4: Confirm nothing is uncommitted**

Run: `git status`
Expected: working tree clean.
