package bridge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	c, err := New(strings.TrimPrefix(srv.URL, "https://"), "test-key", true, WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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

func TestListLights_BridgeError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"description":"unauthorized"}],"data":[]}`))
	}))
	_, err := c.ListLights(context.Background())
	if err == nil {
		t.Fatal("expected error when envelope.errors is non-empty")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error should mention bridge description, got %q", err.Error())
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

func TestClient_AuthFailureCounting(t *testing.T) {
	var count atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	// Two 401s — not yet revoked.
	for i := 0; i < 2; i++ {
		if _, err := c.ListLights(context.Background()); err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}
	_, err := c.ListLights(context.Background())
	if err == nil {
		t.Fatal("expected error on third call")
	}
	// The third 401 trips the threshold; subsequent calls return the sentinel
	// without hitting the network.
	startCount := count.Load()
	_, err = c.ListLights(context.Background())
	if !errors.Is(err, ErrAuthRevoked) {
		t.Fatalf("expected ErrAuthRevoked after 3 401s, got %v", err)
	}
	if count.Load() != startCount {
		t.Errorf("post-revocation call hit the network %d times, want 0", count.Load()-startCount)
	}
}

func TestClient_AuthFailures_OutsideWindow(t *testing.T) {
	// Two stale 401s should not contribute to the live count.
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	// Inject two old failures via the public knob below.
	c.recordAuthFailureAt(time.Now().Add(-2 * time.Minute))
	c.recordAuthFailureAt(time.Now().Add(-90 * time.Second))
	// One fresh 401 brings the live count to 1, not 3.
	if _, err := c.ListLights(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.ListLights(context.Background()); errors.Is(err, ErrAuthRevoked) {
		t.Fatalf("ErrAuthRevoked too early — stale failures should age out, got %v", err)
	}
}

func TestClient_SetLightTimeout(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	start := time.Now()
	err := c.SetLight(context.Background(), "id", LightUpdate{})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 7*time.Second {
		t.Errorf("took %v, want < 7s (timeout is 5s)", elapsed)
	}
	if elapsed < 4*time.Second {
		t.Errorf("returned in %v — earlier than the 5s budget; timeout may be too aggressive", elapsed)
	}
}
