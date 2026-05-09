package observability_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/observability"
)

type fakeRecoveryProvider struct {
	inRecovery     bool
	reason         string
	failedPos      uint64
	events         []observability.RecoveryEvent
	cursors        []observability.ProjectionCursor
	skipped        []observability.SkippedEvent
	projectorNames []string
	skipErr        error
	skipCalls      []skipCall
	shutdownCalled bool
}

type skipCall struct {
	position  uint64
	projector string
	reason    string
	skippedBy string
}

func (f *fakeRecoveryProvider) InRecovery() bool { return f.inRecovery }
func (f *fakeRecoveryProvider) RecoveryInfo() (string, uint64) {
	return f.reason, f.failedPos
}
func (f *fakeRecoveryProvider) QueryEvents(_ context.Context, _ uint64, _ int) ([]observability.RecoveryEvent, error) {
	return f.events, nil
}
func (f *fakeRecoveryProvider) QueryProjectionCursors(_ context.Context) ([]observability.ProjectionCursor, error) {
	return f.cursors, nil
}
func (f *fakeRecoveryProvider) QuerySkippedEvents(_ context.Context) ([]observability.SkippedEvent, error) {
	return f.skipped, nil
}
func (f *fakeRecoveryProvider) SkipEvent(_ context.Context, position uint64, projector, reason, skippedBy string) error {
	f.skipCalls = append(f.skipCalls, skipCall{position, projector, reason, skippedBy})
	return f.skipErr
}
func (f *fakeRecoveryProvider) ProjectorNames() []string { return f.projectorNames }
func (f *fakeRecoveryProvider) Shutdown()                { f.shutdownCalled = true }

// --- GET /events ---

func TestHandleRecoveryEvents_NotInRecovery(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: false}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events?position=1&limit=10", nil)
	observability.HandleRecoveryEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandleRecoveryEvents_Success(t *testing.T) {
	ts := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	p := &fakeRecoveryProvider{
		inRecovery: true,
		events:     []observability.RecoveryEvent{{Position: 7, Timestamp: ts, Kind: "test.event", Source: "src"}},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events?position=7&limit=10", nil)
	observability.HandleRecoveryEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []observability.RecoveryEvent
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Position != 7 {
		t.Fatalf("unexpected events: %+v", got)
	}
}

func TestHandleRecoveryEvents_EmptyReturnsArray(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true, events: nil}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	observability.HandleRecoveryEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "[]") {
		t.Fatalf("body should be empty JSON array, got: %s", w.Body.String())
	}
}

func TestHandleRecoveryEvents_BadLimit(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events?limit=notanumber", nil)
	observability.HandleRecoveryEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandleRecoveryEvents_BadPosition(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events?position=notanumber", nil)
	observability.HandleRecoveryEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// --- GET /projection-cursors ---

func TestHandleProjectionCursors_NotInRecovery(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: false}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projection-cursors", nil)
	observability.HandleProjectionCursors(p).ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandleProjectionCursors_Success(t *testing.T) {
	p := &fakeRecoveryProvider{
		inRecovery: true,
		cursors:    []observability.ProjectionCursor{{Name: "cache", Position: 42, UpdatedAt: time.Now()}},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projection-cursors", nil)
	observability.HandleProjectionCursors(p).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []observability.ProjectionCursor
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "cache" || got[0].Position != 42 {
		t.Fatalf("unexpected cursors: %+v", got)
	}
}

// --- GET /skipped-events ---

func TestHandleSkippedEvents_NotInRecovery(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: false}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/skipped-events", nil)
	observability.HandleSkippedEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandleSkippedEvents_Success(t *testing.T) {
	p := &fakeRecoveryProvider{
		inRecovery: true,
		skipped: []observability.SkippedEvent{
			{Position: 5, Projector: "cache", SkippedBy: "192.0.2.1:9999", Reason: "bad payload"},
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/skipped-events", nil)
	observability.HandleSkippedEvents(p).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []observability.SkippedEvent
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Position != 5 {
		t.Fatalf("unexpected skipped: %+v", got)
	}
}

// --- POST /events/{position}/skip ---

func skipMux(p observability.RecoveryProvider) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events/{position}/skip", observability.HandleSkipEvent(p))
	return mux
}

func TestHandleSkipEvent_NotInRecovery(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: false}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/1/skip", strings.NewReader(`{"projector":"cache","reason":"x"}`))
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandleSkipEvent_Success(t *testing.T) {
	p := &fakeRecoveryProvider{
		inRecovery:     true,
		projectorNames: []string{"cache", "registry"},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/42/skip",
		strings.NewReader(`{"projector":"cache","reason":"bad payload"}`))
	r.Header.Set("Content-Type", "application/json")
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: body=%s", w.Code, w.Body.String())
	}
	if len(p.skipCalls) != 1 {
		t.Fatal("SkipEvent not called")
	}
	if p.skipCalls[0].position != 42 {
		t.Fatalf("position = %d, want 42", p.skipCalls[0].position)
	}
	if p.skipCalls[0].projector != "cache" {
		t.Fatalf("projector = %q, want cache", p.skipCalls[0].projector)
	}
}

func TestHandleSkipEvent_UnknownProjector(t *testing.T) {
	p := &fakeRecoveryProvider{
		inRecovery:     true,
		projectorNames: []string{"cache"},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/1/skip",
		strings.NewReader(`{"projector":"unknown","reason":"x"}`))
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandleSkipEvent_MissingProjector(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true, projectorNames: []string{"cache"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/1/skip",
		strings.NewReader(`{"reason":"x"}`))
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandleSkipEvent_MissingReason(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true, projectorNames: []string{"cache"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/1/skip",
		strings.NewReader(`{"projector":"cache"}`))
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandleSkipEvent_BadJSON(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true, projectorNames: []string{"cache"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/1/skip",
		strings.NewReader(`not json`))
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandleSkipEvent_ZeroPosition(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true, projectorNames: []string{"cache"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/events/0/skip",
		strings.NewReader(`{"projector":"cache","reason":"x"}`))
	skipMux(p).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// --- POST /shutdown ---

func TestHandleShutdown_NotInRecovery(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: false}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/shutdown", nil)
	observability.HandleShutdown(p).ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if p.shutdownCalled {
		t.Fatal("Shutdown must not be called when not in recovery")
	}
}

func TestHandleShutdown_Success(t *testing.T) {
	p := &fakeRecoveryProvider{inRecovery: true}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/shutdown", nil)
	observability.HandleShutdown(p).ServeHTTP(w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	if !p.shutdownCalled {
		t.Fatal("Shutdown not called")
	}
}
