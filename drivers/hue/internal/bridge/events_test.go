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

	got := make([]Event, 0, 3)
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(got), got)
	}
	if got[0].ID != "12345678-90ab-cdef-1234-567890abcdef" || got[0].On == nil || got[0].On.On {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Dimming == nil || got[1].Dimming.Brightness != 75.0 {
		t.Errorf("got[1].Dimming = %+v", got[1].Dimming)
	}
	if got[2].Type != "zigbee_connectivity" {
		t.Errorf("got[2].Type = %q, want zigbee_connectivity", got[2].Type)
	}
	if got[2].Status != "unreachable" {
		t.Errorf("got[2].Status = %q, want unreachable", got[2].Status)
	}
	if got[2].Owner == nil || got[2].Owner.RID != "device-aaa" {
		t.Errorf("got[2].Owner = %+v, want RID=device-aaa", got[2].Owner)
	}
}
