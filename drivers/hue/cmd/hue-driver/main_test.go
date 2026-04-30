package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
