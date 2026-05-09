# F-130: Recovery-mode Admin HTTP Surface — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose five recovery-mode HTTP endpoints on the admin server and a 30-second heartbeat goroutine so an operator can inspect and advance past replay failures without the C13 repair CLI.

**Architecture:** A `RecoveryProvider` interface lives in `internal/observability/`; `*Daemon` implements it so the observability package stays import-cycle-free. Five handlers are registered unconditionally in `ServeMetrics` but each checks `InRecovery()` at request time, returning 404 in normal operation. A `ReplayError` typed error in `internal/eventstore/` carries the failing position from `replayBatch` up to `daemon.Run`, which stores it in `recoveryState` and starts the heartbeat goroutine.

**Tech Stack:** Go 1.25, `net/http` (Go 1.22+ method-prefixed mux routing), `database/sql`, `log/slog`

---

## File Map

| Action | Path | Purpose |
|--------|------|---------|
| Modify | `internal/eventstore/replay.go` | Add `ReplayError` type; return it from `replayBatch` on Apply failure |
| Modify | `internal/eventstore/store.go` | Add `ProjectorNames() []string` |
| Create | `internal/observability/recovery.go` | `RecoveryProvider` interface, row types, five HTTP handlers |
| Create | `internal/observability/recovery_test.go` | Handler unit tests with `fakeRecoveryProvider` |
| Modify | `internal/observability/metrics_server.go` | Accept `RecoveryProvider`, register five routes |
| Modify | `internal/daemon/daemon.go` | Daemon struct, `enterRecovery`, `Run`, implement `RecoveryProvider` |
| Modify | `internal/eventstore/replay_test.go` | Add ReplayError test + skip-then-replay integration test |

---

### Task 1: Add `ReplayError` to eventstore

**Files:**
- Modify: `internal/eventstore/replay.go`
- Modify: `internal/eventstore/replay_test.go`

- [ ] **Step 1: Write a failing test**

Add to `internal/eventstore/replay_test.go`. First add `"errors"` and `"time"` to the import block:

```go
import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)
```

Then append this test function:

```go
func TestReplay_ReturnsReplayError(t *testing.T) {
	ctx := context.Background()

	// Populate the DB with one event using a store that has no failing projector.
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Append(ctx, testutil.StateChanged("light.x", 1)); err != nil {
		t.Fatal(err)
	}
	_ = f.store.Close(ctx)

	// Replay on a fresh store with a projector that fails on the first Apply call.
	f2 := newStoreFixtureOnDB(t, f.db)
	if err := f2.store.RegisterProjector(&countingProjector{name: "boom", failAt: 1}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	err := f2.store.Replay(ctx)
	if err == nil {
		t.Fatal("expected replay to fail")
	}
	var re *eventstore.ReplayError
	if !errors.As(err, &re) {
		t.Fatalf("expected *eventstore.ReplayError, got %T: %v", err, err)
	}
	if re.Position == 0 {
		t.Fatal("ReplayError.Position must be non-zero")
	}
	if re.Projector != "boom" {
		t.Fatalf("ReplayError.Projector = %q, want %q", re.Projector, "boom")
	}
	if re.Err == nil {
		t.Fatal("ReplayError.Err must not be nil")
	}
}
```

Note: `countingProjector`, `newStoreFixture`, and `newStoreFixtureOnDB` are defined in `store_test.go` in the same `package eventstore_test`.

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./internal/eventstore/... -run TestReplay_ReturnsReplayError -v
```
Expected: FAIL with `undefined: eventstore.ReplayError`

- [ ] **Step 3: Add `ReplayError` type to `replay.go`**

Add directly after the `import` block in `internal/eventstore/replay.go`:

```go
// ReplayError is returned by Replay when a sync projector's Apply fails.
// It carries the event position and projector name for recovery introspection.
type ReplayError struct {
	Position  uint64
	Projector string
	Err       error
}

func (e *ReplayError) Error() string {
	return fmt.Sprintf("replay failed at position %d (projector %s): %v",
		e.Position, e.Projector, e.Err)
}

func (e *ReplayError) Unwrap() error { return e.Err }
```

- [ ] **Step 4: Update `replayBatch` to return `*ReplayError`**

In `replayBatch`, find:
```go
			if err := reg.p.Apply(ctx, tx, e); err != nil {
				return 0, fmt.Errorf("replay projector %s at position %d: %w",
					reg.p.Name(), e.Position, err)
			}
```
Replace with:
```go
			if err := reg.p.Apply(ctx, tx, e); err != nil {
				return 0, &ReplayError{
					Position:  e.Position,
					Projector: reg.p.Name(),
					Err:       err,
				}
			}
```

- [ ] **Step 5: Run all eventstore tests**

```
go test ./internal/eventstore/... -v
```
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git checkout -b feature/f-130-c1-implement-recovery-mode-admin-http-surface
git add internal/eventstore/replay.go internal/eventstore/replay_test.go
git commit -m "feat(eventstore): add ReplayError type for recovery position tracking"
```

---

### Task 2: Add `ProjectorNames()` to `eventstore.Store`

**Files:**
- Modify: `internal/eventstore/store.go`
- Modify: `internal/eventstore/store_test.go`

- [ ] **Step 1: Write a failing test**

Add to `internal/eventstore/store_test.go`, inside `package eventstore_test`:

```go
func TestStore_ProjectorNames(t *testing.T) {
	f := newStoreFixture(t)
	if err := f.store.RegisterProjector(&countingProjector{name: "alpha"}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(&countingProjector{name: "beta"}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	got := f.store.ProjectorNames()
	want := []string{"alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("ProjectorNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ProjectorNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./internal/eventstore/... -run TestStore_ProjectorNames -v
```
Expected: FAIL with `f.store.ProjectorNames undefined`

- [ ] **Step 3: Implement `ProjectorNames()`**

Add to `internal/eventstore/store.go`, after `RegisterProjector`:

```go
// ProjectorNames returns the names of all registered projectors in registration order.
func (s *Store) ProjectorNames() []string {
	names := make([]string, len(s.projectors))
	for i, reg := range s.projectors {
		names[i] = reg.p.Name()
	}
	return names
}
```

- [ ] **Step 4: Run all eventstore tests**

```
go test ./internal/eventstore/... -v
```
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/eventstore/store.go internal/eventstore/store_test.go
git commit -m "feat(eventstore): add ProjectorNames() for recovery projector validation"
```

---

### Task 3: Recovery interface, row types, and HTTP handlers

**Files:**
- Create: `internal/observability/recovery.go`
- Create: `internal/observability/recovery_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/observability/recovery_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```
go test ./internal/observability/... -run "TestHandle" -v
```
Expected: FAIL with `undefined: observability.HandleRecoveryEvents` etc.

- [ ] **Step 3: Create `internal/observability/recovery.go`**

```go
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RecoveryProvider is implemented by the Daemon to expose recovery-mode state
// to the admin HTTP server. All methods are safe for concurrent use.
type RecoveryProvider interface {
	// InRecovery returns true iff the daemon is in phase -1.
	InRecovery() bool
	// RecoveryInfo returns the failure reason and the event position at which
	// replay halted. failedPos is 0 if the position is unavailable.
	RecoveryInfo() (reason string, failedPos uint64)
	// QueryEvents returns up to limit events centred on position.
	QueryEvents(ctx context.Context, position uint64, limit int) ([]RecoveryEvent, error)
	// QueryProjectionCursors returns all rows from the projection_cursors table.
	QueryProjectionCursors(ctx context.Context) ([]ProjectionCursor, error)
	// QuerySkippedEvents returns all rows from the skipped_events table.
	QuerySkippedEvents(ctx context.Context) ([]SkippedEvent, error)
	// SkipEvent inserts a row into skipped_events. projector must be in ProjectorNames().
	SkipEvent(ctx context.Context, position uint64, projector, reason, skippedBy string) error
	// ProjectorNames returns the names of all registered sync projectors.
	ProjectorNames() []string
	// Shutdown cancels the daemon's root context, triggering a clean exit.
	Shutdown()
}

// RecoveryEvent is a JSON-serialisable projection of an eventstore event.
type RecoveryEvent struct {
	Position  uint64    `json:"position"`
	Timestamp time.Time `json:"timestamp"`
	Kind      string    `json:"kind"`
	Entity    string    `json:"entity,omitempty"`
	Source    string    `json:"source"`
}

// ProjectionCursor is a JSON-serialisable row from projection_cursors.
type ProjectionCursor struct {
	Name      string    `json:"name"`
	Position  uint64    `json:"position"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SkippedEvent is a JSON-serialisable row from skipped_events.
type SkippedEvent struct {
	Position  uint64    `json:"position"`
	Projector string    `json:"projector"`
	SkippedAt time.Time `json:"skipped_at"`
	SkippedBy string    `json:"skipped_by"`
	Reason    string    `json:"reason"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func recoveryGuard(w http.ResponseWriter, r *http.Request, p RecoveryProvider) bool {
	if !p.InRecovery() {
		http.NotFound(w, r)
		return false
	}
	return true
}

// HandleRecoveryEvents returns a handler for GET /events?position=N&limit=K.
// Returns up to limit events starting from around position. Defaults: limit=50, max 200.
func HandleRecoveryEvents(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		const defaultLimit = 50
		const maxLimit = 200
		limit := defaultLimit
		if s := r.URL.Query().Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			if n > maxLimit {
				n = maxLimit
			}
			limit = n
		}
		var position uint64
		if s := r.URL.Query().Get("position"); s != "" {
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "position must be a non-negative integer")
				return
			}
			position = n
		}
		events, err := p.QueryEvents(r.Context(), position, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if events == nil {
			events = []RecoveryEvent{}
		}
		writeJSON(w, http.StatusOK, events)
	}
}

// HandleProjectionCursors returns a handler for GET /projection-cursors.
func HandleProjectionCursors(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		cursors, err := p.QueryProjectionCursors(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if cursors == nil {
			cursors = []ProjectionCursor{}
		}
		writeJSON(w, http.StatusOK, cursors)
	}
}

// HandleSkippedEvents returns a handler for GET /skipped-events.
func HandleSkippedEvents(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		skipped, err := p.QuerySkippedEvents(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if skipped == nil {
			skipped = []SkippedEvent{}
		}
		writeJSON(w, http.StatusOK, skipped)
	}
}

// HandleSkipEvent returns a handler for POST /events/{position}/skip.
// Body: {"projector":"...","reason":"..."}.
func HandleSkipEvent(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		position, err := strconv.ParseUint(r.PathValue("position"), 10, 64)
		if err != nil || position == 0 {
			writeError(w, http.StatusBadRequest, "position must be a positive integer")
			return
		}
		var body struct {
			Projector string `json:"projector"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if body.Projector == "" {
			writeError(w, http.StatusUnprocessableEntity, "projector is required")
			return
		}
		if body.Reason == "" {
			writeError(w, http.StatusUnprocessableEntity, "reason is required")
			return
		}
		known := false
		for _, name := range p.ProjectorNames() {
			if name == body.Projector {
				known = true
				break
			}
		}
		if !known {
			writeError(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("unknown projector %q; known projectors: %v", body.Projector, p.ProjectorNames()))
			return
		}
		if err := p.SkipEvent(r.Context(), position, body.Projector, body.Reason, r.RemoteAddr); err != nil {
			writeError(w, http.StatusInternalServerError, "skip failed: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleShutdown returns a handler for POST /shutdown.
func HandleShutdown(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		p.Shutdown()
		w.WriteHeader(http.StatusAccepted)
	}
}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/observability/... -run "TestHandle" -v
```
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/observability/recovery.go internal/observability/recovery_test.go
git commit -m "feat(observability): add RecoveryProvider interface and recovery HTTP handlers"
```

---

### Task 4: Wire recovery handlers into `ServeMetrics`

**Files:**
- Modify: `internal/observability/metrics_server.go`

- [ ] **Step 1: Update `ServeMetrics` signature and register recovery routes**

Replace the full content of `internal/observability/metrics_server.go` with:

```go
package observability

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (m *Metrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// ServeMetrics runs an HTTP server exposing /metrics, /health, and (while in
// recovery mode) the five recovery endpoints. Blocks until ctx is cancelled.
func (m *Metrics) ServeMetrics(ctx context.Context, addr string, healthFn func() (string, int), recovery RecoveryProvider) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.HTTPHandler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		status := "ok"
		code := http.StatusOK
		if healthFn != nil {
			status, code = healthFn()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"status":"` + status + `"}`))
	})

	mux.HandleFunc("GET /events", HandleRecoveryEvents(recovery))
	mux.HandleFunc("GET /projection-cursors", HandleProjectionCursors(recovery))
	mux.HandleFunc("GET /skipped-events", HandleSkippedEvents(recovery))
	mux.HandleFunc("POST /events/{position}/skip", HandleSkipEvent(recovery))
	mux.HandleFunc("POST /shutdown", HandleShutdown(recovery))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		// Fresh context: parent is already cancelled, but shutdown needs time to drain.
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx) //nolint:contextcheck
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
```

- [ ] **Step 2: Update the call site in `internal/daemon/daemon.go`**

Find (around line 117):
```go
	go func() {
		_ = d.metrics.ServeMetrics(ctx, fmt.Sprintf(":%d", d.cfg.AdminPort), d.healthStatus)
	}()
```
Replace with:
```go
	go func() {
		_ = d.metrics.ServeMetrics(ctx, fmt.Sprintf(":%d", d.cfg.AdminPort), d.healthStatus, d)
	}()
```

- [ ] **Step 3: Attempt to build (expect compile error)**

```
go build ./...
```
Expected: FAIL — `*Daemon` does not implement `observability.RecoveryProvider` (methods not added yet). This confirms the interface wiring is correct.

- [ ] **Step 4: Commit the metrics_server change**

> **Note:** After this commit the branch will not compile until Task 5 adds the `RecoveryProvider` methods to `*Daemon`. This is expected — proceed directly to Task 5.

```bash
git add internal/observability/metrics_server.go internal/daemon/daemon.go
git commit -m "feat(observability): wire RecoveryProvider into ServeMetrics"
```

---

### Task 5: Daemon structural changes and `RecoveryProvider` implementation

**Files:**
- Modify: `internal/daemon/daemon.go`

- [ ] **Step 1: Update the `Daemon` struct and `recoveryState`**

In `daemon.go`, replace:
```go
type Daemon struct {
	cfg              Config
	logger           *slog.Logger
	metrics          *observability.Metrics
	lockfile         *storage.Lockfile
	db               *sql.DB
	store            *eventstore.Store
	cache            *state.Cache
	registry         *registry.Registry
	carport          *carport.Host
	configMgr        *config.Manager
	starlarkRuntime  *starlark.Runtime
	scriptEngine     *script.Engine
	automationEngine *automation.Engine
	configDir        string

	phase        atomic.Int32
	recoveryInfo atomic.Pointer[recoveryState]
}

type recoveryState struct {
	reason string
}
```
with:
```go
type Daemon struct {
	cfg              Config
	logger           *slog.Logger
	metrics          *observability.Metrics
	lockfile         *storage.Lockfile
	db               *sql.DB
	store            *eventstore.Store
	cache            *state.Cache
	registry         *registry.Registry
	carport          *carport.Host
	configMgr        *config.Manager
	starlarkRuntime  *starlark.Runtime
	scriptEngine     *script.Engine
	automationEngine *automation.Engine
	configDir        string

	phase          atomic.Int32
	recoveryInfo   atomic.Pointer[recoveryState]
	shutdownCancel atomic.Pointer[context.CancelFunc]
}

type recoveryState struct {
	reason    string
	failedPos uint64
}

// Compile-time assertion: *Daemon must satisfy RecoveryProvider.
var _ observability.RecoveryProvider = (*Daemon)(nil)
```

- [ ] **Step 2: Update `Run` to store the cancel func and extract `failedPos`**

At the very top of `Run`, add `context.WithCancel`:

Replace:
```go
func (d *Daemon) Run(ctx context.Context) error {
	d.metrics.SetBuildInfo(Version, Commit, GoVersion)
	start := time.Now()
```
with:
```go
func (d *Daemon) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	d.shutdownCancel.Store(&cancel)
	defer cancel()

	d.metrics.SetBuildInfo(Version, Commit, GoVersion)
	start := time.Now()
```

Replace the replay error handling block:
```go
	if err := store.Replay(ctx); err != nil {
		d.enterRecovery(err.Error())
		<-ctx.Done()
		return nil
	}
```
with:
```go
	if err := store.Replay(ctx); err != nil {
		var replayErr *eventstore.ReplayError
		var failedPos uint64
		if errors.As(err, &replayErr) {
			failedPos = replayErr.Position
		}
		d.enterRecovery(ctx, err.Error(), failedPos)
		<-ctx.Done()
		return nil
	}
```

Add `"errors"` to the import block in `daemon.go` (it is not currently imported).

- [ ] **Step 3: Update `enterRecovery` with new signature and heartbeat**

Replace:
```go
func (d *Daemon) enterRecovery(reason string) {
	d.metrics.RecoveryModeEntered.Inc()
	d.phase.Store(-1)
	d.metrics.StartupPhase.Set(-1)
	d.recoveryInfo.Store(&recoveryState{reason: reason})
	d.logger.Error("entering recovery mode", "reason", reason)
}
```
with:
```go
func (d *Daemon) enterRecovery(ctx context.Context, reason string, failedPos uint64) {
	d.metrics.RecoveryModeEntered.Inc()
	d.phase.Store(-1)
	d.metrics.StartupPhase.Set(-1)
	d.recoveryInfo.Store(&recoveryState{reason: reason, failedPos: failedPos})
	d.logger.Error("entering recovery mode", "reason", reason, "failed_position", failedPos)
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				d.logger.Error("daemon in recovery mode — operator action required",
					"reason", reason,
					"failed_position", failedPos,
				)
			case <-ctx.Done():
				return
			}
		}
	}()
}
```

- [ ] **Step 4: Add `RecoveryProvider` methods on `*Daemon`**

Add the following methods to `daemon.go`, after `enterRecovery`:

```go
func (d *Daemon) InRecovery() bool {
	return d.phase.Load() == -1
}

func (d *Daemon) RecoveryInfo() (string, uint64) {
	if info := d.recoveryInfo.Load(); info != nil {
		return info.reason, info.failedPos
	}
	return "", 0
}

// QueryEvents returns up to limit events starting from around position.
// Uses position - limit as the exclusive lower bound so the failing event is included.
func (d *Daemon) QueryEvents(ctx context.Context, position uint64, limit int) ([]observability.RecoveryEvent, error) {
	var from uint64
	if position > uint64(limit) {
		from = position - uint64(limit) - 1
	}
	events, err := d.store.Query(ctx, eventstore.QueryOptions{
		FromPosition: from,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]observability.RecoveryEvent, len(events))
	for i, e := range events {
		out[i] = observability.RecoveryEvent{
			Position:  e.Position,
			Timestamp: e.Timestamp,
			Kind:      e.Kind,
			Entity:    e.Entity,
			Source:    e.Source,
		}
	}
	return out, nil
}

func (d *Daemon) QueryProjectionCursors(ctx context.Context) ([]observability.ProjectionCursor, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT name, position, updated_at FROM projection_cursors ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []observability.ProjectionCursor
	for rows.Next() {
		var c observability.ProjectionCursor
		var updatedAtNano int64
		if err := rows.Scan(&c.Name, &c.Position, &updatedAtNano); err != nil {
			return nil, err
		}
		c.UpdatedAt = time.Unix(0, updatedAtNano)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *Daemon) QuerySkippedEvents(ctx context.Context) ([]observability.SkippedEvent, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT position, projector, skipped_at, skipped_by, reason FROM skipped_events ORDER BY position, projector`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []observability.SkippedEvent
	for rows.Next() {
		var e observability.SkippedEvent
		var skippedAtNano int64
		if err := rows.Scan(&e.Position, &e.Projector, &skippedAtNano, &e.SkippedBy, &e.Reason); err != nil {
			return nil, err
		}
		e.SkippedAt = time.Unix(0, skippedAtNano)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *Daemon) SkipEvent(ctx context.Context, position uint64, projector, reason, skippedBy string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO skipped_events (position, projector, skipped_at, skipped_by, reason)
		VALUES (?, ?, ?, ?, ?)`,
		position, projector, time.Now().UnixNano(), skippedBy, reason,
	)
	return err
}

func (d *Daemon) ProjectorNames() []string {
	return d.store.ProjectorNames()
}

func (d *Daemon) Shutdown() {
	if fn := d.shutdownCancel.Load(); fn != nil {
		(*fn)()
	}
}
```

- [ ] **Step 5: Build to verify compilation**

```
go build ./...
```
Expected: SUCCESS — all packages compile

- [ ] **Step 6: Run full test suite**

```
go test ./...
```
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/daemon/daemon.go
git commit -m "feat(daemon): implement RecoveryProvider, heartbeat, and shutdown wiring"
```

---

### Task 6: Integration test — skip event + replay

**Files:**
- Modify: `internal/eventstore/replay_test.go`

This test verifies the full skip-then-replay path: replay fails, operator inserts a skip row (simulating the HTTP handler), next replay succeeds.

- [ ] **Step 1: Write the integration test**

Add to `internal/eventstore/replay_test.go` (the `"time"` import was added in Task 1):

```go
func TestReplay_SkipEventAllowsReplayToProceed(t *testing.T) {
	ctx := context.Background()

	// Phase A: populate the DB with one event using a store with no failing projector.
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	pos, err := f.store.Append(ctx, testutil.StateChanged("light.z", 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("appended event at position %d", pos)
	_ = f.store.Close(ctx)

	// Phase B: replay with a projector that fails on the first Apply → ReplayError.
	f2 := newStoreFixtureOnDB(t, f.db)
	if err := f2.store.RegisterProjector(&countingProjector{name: "boom", failAt: 1}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	replayErr := f2.store.Replay(ctx)
	if replayErr == nil {
		t.Fatal("expected replay to fail")
	}
	var re *eventstore.ReplayError
	if !errors.As(replayErr, &re) {
		t.Fatalf("expected *eventstore.ReplayError, got %T", replayErr)
	}

	// Phase C: insert a skip row for (position, "boom") — simulates POST /events/{pos}/skip.
	_, err = f.db.ExecContext(ctx, `
		INSERT INTO skipped_events (position, projector, skipped_at, skipped_by, reason)
		VALUES (?, ?, ?, ?, ?)`,
		re.Position, "boom", time.Now().UnixNano(), "integration-test", "skip to unblock replay",
	)
	if err != nil {
		t.Fatalf("insert skipped_events: %v", err)
	}

	// Phase D: replay again — must succeed now that the event is skipped.
	f3 := newStoreFixtureOnDB(t, f.db)
	if err := f3.store.RegisterProjector(&countingProjector{name: "boom", failAt: 1}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f3.store.Replay(ctx); err != nil {
		t.Fatalf("replay after skip failed: %v", err)
	}
}
```

- [ ] **Step 2: Run the integration test**

```
go test ./internal/eventstore/... -run TestReplay_SkipEventAllowsReplayToProceed -v
```
Expected: PASS

- [ ] **Step 3: Run full test suite one final time**

```
go test ./...
```
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/eventstore/replay_test.go
git commit -m "test(eventstore): add skip-event integration test for recovery flow"
```

---

### Task 7: Open pull request

- [ ] **Step 1: Push branch**

```bash
git push -u origin feature/f-130-c1-implement-recovery-mode-admin-http-surface
```

- [ ] **Step 2: Create PR**

```bash
gh pr create \
  --title "F-130: C1 recovery-mode admin HTTP surface" \
  --body "$(cat <<'EOF'
## Summary
- Adds `ReplayError` typed error to eventstore so replay failures carry the failing position and projector name
- Adds `RecoveryProvider` interface in `internal/observability/` with five HTTP handlers (`GET /events`, `GET /projection-cursors`, `GET /skipped-events`, `POST /events/{position}/skip`, `POST /shutdown`)
- Implements `RecoveryProvider` on `*Daemon`; all five endpoints guarded by `InRecovery()` (return 404 when phase != -1)
- Adds 30-second heartbeat goroutine in `enterRecovery` that logs failure reason + position until daemon shuts down
- `POST /shutdown` cancels the daemon's root context via a stored `CancelFunc`

## Test plan
- [ ] `go test ./internal/eventstore/...` — ReplayError, ProjectorNames, skip+replay integration test
- [ ] `go test ./internal/observability/...` — 17 handler unit tests (success, 404-when-not-in-recovery, malformed input per endpoint)
- [ ] `go test ./...` — full suite green
- [ ] Manual: start `switchyardd`, poison an event in the DB, confirm 503 on `/health`, confirm recovery endpoints respond, POST a skip, restart daemon, confirm clean boot

Closes F-130

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
