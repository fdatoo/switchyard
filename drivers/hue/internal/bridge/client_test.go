package bridge

import (
	"context"
	"io"
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
