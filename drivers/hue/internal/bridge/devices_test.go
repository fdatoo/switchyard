package bridge

import (
	"context"
	"net/http"
	"os"
	"testing"
)

func TestListDevices(t *testing.T) {
	devicesBody, err := os.ReadFile("testdata/devices.json")
	if err != nil {
		t.Fatal(err)
	}
	zcBody, err := os.ReadFile("testdata/zigbee_connectivity.json")
	if err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/clip/v2/resource/device":
			_, _ = w.Write(devicesBody)
		case "/clip/v2/resource/zigbee_connectivity":
			_, _ = w.Write(zcBody)
		default:
			http.NotFound(w, r)
		}
	}))

	got, err := c.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	want := map[string]string{
		"light-aaa": "connected",
		"light-bbb": "unreachable",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for lightID, wantStatus := range want {
		if got[lightID] != wantStatus {
			t.Errorf("light %s status = %q, want %q", lightID, got[lightID], wantStatus)
		}
	}
}

func TestListDevices_HTTPError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	if _, err := c.ListDevices(context.Background()); err == nil {
		t.Fatal("expected error on 500")
	}
}
