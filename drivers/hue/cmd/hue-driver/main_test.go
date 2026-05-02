package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fdatoo/switchyard-driverkit/drivertest"

	"github.com/fdatoo/switchyard/drivers/hue/internal/bridge"
)

const fakeBridgeListLightsBody = `{
  "errors": [],
  "data": [
    {
      "id": "12345678-90ab-cdef-1234-567890abcdef",
      "type": "light",
      "owner": {"rid": "device-12345678", "rtype": "device"},
      "metadata": { "name": "Kitchen" },
      "on": { "on": false },
      "dimming": { "brightness": 50 },
      "color_temperature": { "mirek": 366, "mirek_valid": true },
      "color": {
        "xy": {"x": 0.3127, "y": 0.3290},
        "gamut": {
          "red":   {"x": 0.6915, "y": 0.3083},
          "green": {"x": 0.1700, "y": 0.7000},
          "blue":  {"x": 0.1532, "y": 0.0475}
        }
      }
    }
  ]
}`

// newTestBridgeClient creates a bridge.Client wired to the given httptest.Server.
func newTestBridgeClient(t *testing.T, srv *httptest.Server) *bridge.Client {
	t.Helper()
	c, err := bridge.New(
		strings.TrimPrefix(srv.URL, "https://"),
		"test-key",
		true,
		bridge.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("bridge.New: %v", err)
	}
	return c
}

func TestDriver_AllCapabilities(t *testing.T) {
	var mu sync.Mutex
	var seenPUTs []string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			mu.Lock()
			seenPUTs = append(seenPUTs, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/eventstream/clip/v2":
			// Hold the connection open until the harness closes the test.
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client := newTestBridgeClient(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	const entityID = "light.hue_12345678"

	if _, err := h.SendCommand(ctx, entityID, "turn_on", nil); err != nil {
		t.Fatalf("turn_on: %v", err)
	}
	if _, err := h.SendCommand(ctx, entityID, "turn_off", nil); err != nil {
		t.Fatalf("turn_off: %v", err)
	}
	if _, err := h.SendCommand(ctx, entityID, "set_brightness", map[string]string{"brightness": "128"}); err != nil {
		t.Fatalf("set_brightness: %v", err)
	}
	if _, err := h.SendCommand(ctx, entityID, "set_color_temp", map[string]string{"color_temp": "300"}); err != nil {
		t.Fatalf("set_color_temp: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seenPUTs) != 4 {
		t.Fatalf("expected 4 PUT calls to bridge, got %d: %v", len(seenPUTs), seenPUTs)
	}
	const wantSuffix = "/clip/v2/resource/light/12345678-90ab-cdef-1234-567890abcdef"
	for _, p := range seenPUTs {
		if p != wantSuffix {
			t.Errorf("unexpected PUT path %q, want %q", p, wantSuffix)
		}
	}
}

func TestDriver_BridgeError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			http.Error(w, "bridge error", http.StatusInternalServerError)
		case r.URL.Path == "/eventstream/clip/v2":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client := newTestBridgeClient(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	const entityID = "light.hue_12345678"
	res, err := h.SendCommand(ctx, entityID, "turn_on", nil)
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if res.GetOk() {
		t.Fatal("expected Ok=false when bridge returns HTTP 500")
	}
}

func TestDriver_BrightnessZeroTurnsOff(t *testing.T) {
	var (
		mu   sync.Mutex
		puts []string
	)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			puts = append(puts, string(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/clip/v2/resource/light":
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/eventstream/clip/v2":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true, bridge.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("bridge.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	const entityID = "light.hue_12345678"
	if _, err := h.SendCommand(ctx, entityID, "set_brightness", map[string]string{"brightness": "0"}); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(puts))
	}
	if !strings.Contains(puts[0], `"on":{"on":false}`) {
		t.Errorf("PUT body missing on:false: %s", puts[0])
	}
	if strings.Contains(puts[0], `"dimming"`) {
		t.Errorf("PUT body should not include dimming for brightness=0: %s", puts[0])
	}
}

func TestDriver_DurationPassthrough(t *testing.T) {
	var (
		mu   sync.Mutex
		puts []string
	)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			puts = append(puts, string(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/clip/v2/resource/light":
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/eventstream/clip/v2":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true, bridge.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("bridge.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	const entityID = "light.hue_12345678"
	if _, err := h.SendCommand(ctx, entityID, "set_brightness", map[string]string{"brightness": "128", "duration_ms": "5000"}); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(puts))
	}
	if !strings.Contains(puts[0], `"dynamics":{"duration":5000}`) {
		t.Errorf("PUT body missing dynamics.duration=5000: %s", puts[0])
	}
}

func TestDriver_HotAddRemove(t *testing.T) {
	const initialLights = `{
		"errors": [],
		"data": [
			{
				"id": "12345678-90ab-cdef-1234-567890abcdef",
				"type": "light",
				"owner": {"rid": "device-12345678", "rtype": "device"},
				"metadata": {"name": "Original"},
				"on": {"on": true},
				"dimming": {"brightness": 50}
			}
		]
	}`
	const swappedLights = `{
		"errors": [],
		"data": [
			{
				"id": "abcdef00-0000-0000-0000-000000000001",
				"type": "light",
				"owner": {"rid": "device-abcdef00", "rtype": "device"},
				"metadata": {"name": "Added"},
				"on": {"on": true},
				"dimming": {"brightness": 50}
			}
		]
	}`

	var lightsBody atomic.Value
	lightsBody.Store(initialLights)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/clip/v2/resource/light":
			_, _ = w.Write([]byte(lightsBody.Load().(string)))
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/eventstream/clip/v2":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true, bridge.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("bridge.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	d, cache, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	if got := len(h.Entities()); got != 1 {
		t.Fatalf("initial entities = %d, want 1", got)
	}

	// Swap fake bridge: original gone, "Added" appears.
	lightsBody.Store(swappedLights)

	if err := resync(ctx, client, d, cache); err != nil {
		t.Fatalf("resync: %v", err)
	}

	// Send commands: original should be unknown, added should work.
	res, _ := h.SendCommand(ctx, "light.hue_12345678", "turn_on", nil)
	if res.GetOk() {
		t.Errorf("command to removed entity returned ok=true")
	}
	res, _ = h.SendCommand(ctx, "light.hue_abcdef00", "turn_on", nil)
	if !res.GetOk() {
		t.Errorf("command to added entity returned ok=false: %s", res.GetErrorMessage())
	}
}

func TestDriver_SetColorHex(t *testing.T) {
	var (
		mu   sync.Mutex
		puts []string
	)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			puts = append(puts, string(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/clip/v2/resource/light":
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case r.URL.Path == "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case r.URL.Path == "/eventstream/clip/v2":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true, bridge.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("bridge.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	const entityID = "light.hue_12345678"
	if _, err := h.SendCommand(ctx, entityID, "set_color", map[string]string{"hex": "#FF8800"}); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(puts))
	}
	if !strings.Contains(puts[0], `"color":{"xy":{"x":`) {
		t.Errorf("PUT body missing color.xy: %s", puts[0])
	}
	if strings.Contains(puts[0], `"color_temperature"`) {
		t.Errorf("PUT body should not include color_temperature alongside color: %s", puts[0])
	}
	if !strings.Contains(puts[0], `"on":{"on":true}`) {
		t.Errorf("PUT body missing on:true (auto-on): %s", puts[0])
	}
}

func TestDriver_SetColorBadHex(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/light":
			_, _ = w.Write([]byte(fakeBridgeListLightsBody))
		case "/clip/v2/resource/device":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
		case "/eventstream/clip/v2":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			flusher.Flush()
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)

	client, _ := bridge.New(strings.TrimPrefix(srv.URL, "https://"), "k", true, bridge.WithHTTPClient(srv.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, _, err := buildDriver(ctx, client)
	if err != nil {
		t.Fatalf("buildDriver: %v", err)
	}

	h := drivertest.New(t, d)
	defer h.Close()

	res, err := h.SendCommand(ctx, "light.hue_12345678", "set_color", map[string]string{"hex": "zz"})
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if res.GetOk() {
		t.Error("expected ok=false for malformed hex")
	}
}
