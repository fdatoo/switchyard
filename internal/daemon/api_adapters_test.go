package daemon

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/api"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
	"github.com/fdatoo/gohome/internal/testutil"
)

// ----- traceEventFromStoreEvent -----

func TestTraceEventFromStoreEvent_Triggered(t *testing.T) {
	cid := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ts := time.Unix(1700000000, 0).UTC()
	ev := eventstore.Event{
		Position:      42,
		Timestamp:     ts,
		Kind:          "automation_triggered",
		Source:        "automation:morning_routine",
		CorrelationID: cid,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationTriggered{
			AutomationTriggered: &eventv1.AutomationTriggered{
				AutomationId:         "morning_routine",
				CorrelationId:        cid.String(),
				TriggerEventPosition: 17,
				TriggerKind:          "state_changed",
				InvokedBy:            "cli:alice",
			},
		}},
	}

	te := traceEventFromStoreEvent(ev)
	if te.Cursor != 42 {
		t.Errorf("Cursor = %d, want 42", te.Cursor)
	}
	if !te.At.Equal(ts) {
		t.Errorf("At = %v, want %v", te.At, ts)
	}
	if te.AutomationID != "morning_routine" {
		t.Errorf("AutomationID = %q, want morning_routine", te.AutomationID)
	}
	if te.RunID != cid.String() {
		t.Errorf("RunID = %q, want %q", te.RunID, cid.String())
	}
	if te.Kind != "automation_triggered" {
		t.Errorf("Kind = %q, want automation_triggered", te.Kind)
	}
	if te.Detail != "state_changed" {
		t.Errorf("Detail = %q, want state_changed", te.Detail)
	}
	if got := te.Metadata["invoked_by"]; got != "cli:alice" {
		t.Errorf("Metadata[invoked_by] = %q, want cli:alice", got)
	}
	if got := te.Metadata["trigger_event_position"]; got != "17" {
		t.Errorf("Metadata[trigger_event_position] = %q, want 17", got)
	}
}

func TestTraceEventFromStoreEvent_Finished(t *testing.T) {
	cid := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	ev := eventstore.Event{
		Position:      99,
		Timestamp:     time.Unix(1700000100, 0).UTC(),
		Kind:          "automation_finished",
		Source:        "automation:bedtime",
		CorrelationID: cid,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationFinished{
			AutomationFinished: &eventv1.AutomationFinished{
				AutomationId:  "bedtime",
				CorrelationId: cid.String(),
				Outcome:       eventv1.RunOutcome_OUTCOME_OK,
				ElapsedMs:     2345,
				StarlarkSteps: 678,
			},
		}},
	}

	te := traceEventFromStoreEvent(ev)
	if te.AutomationID != "bedtime" {
		t.Errorf("AutomationID = %q, want bedtime", te.AutomationID)
	}
	if te.Detail != "OUTCOME_OK" {
		t.Errorf("Detail = %q, want OUTCOME_OK", te.Detail)
	}
	if got := te.Metadata["elapsed_ms"]; got != "2345" {
		t.Errorf("Metadata[elapsed_ms] = %q, want 2345", got)
	}
	if got := te.Metadata["starlark_steps"]; got != "678" {
		t.Errorf("Metadata[starlark_steps] = %q, want 678", got)
	}
	if _, hasErr := te.Metadata["error"]; hasErr {
		t.Errorf("Metadata[error] should not be set when error is empty")
	}
}

func TestTraceEventFromStoreEvent_FinishedWithError(t *testing.T) {
	cid := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	ev := eventstore.Event{
		Position:      100,
		Timestamp:     time.Now(),
		Kind:          "automation_finished",
		Source:        "automation:foo",
		CorrelationID: cid,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationFinished{
			AutomationFinished: &eventv1.AutomationFinished{
				AutomationId:  "foo",
				CorrelationId: cid.String(),
				Outcome:       eventv1.RunOutcome_OUTCOME_ACTION_ERROR,
				Error:         "service call failed",
			},
		}},
	}
	te := traceEventFromStoreEvent(ev)
	if got := te.Metadata["error"]; got != "service call failed" {
		t.Errorf("Metadata[error] = %q, want 'service call failed'", got)
	}
}

func TestTraceEventFromStoreEvent_NilPayload(t *testing.T) {
	ev := eventstore.Event{
		Position:  1,
		Timestamp: time.Now(),
		Kind:      "automation_triggered",
		Source:    "automation:bar",
	}
	te := traceEventFromStoreEvent(ev)
	if te.AutomationID != "bar" {
		t.Errorf("AutomationID = %q, want bar (from source fallback)", te.AutomationID)
	}
	if te.Kind != "automation_triggered" {
		t.Errorf("Kind = %q, want automation_triggered", te.Kind)
	}
}

// ----- automationControlAdapter.Trace -----

func newTraceTestStore(t *testing.T) *eventstore.Store {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("eventstore.Open: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("eventstore.Start: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s
}

func mustAppendAutomationTriggered(t *testing.T, s *eventstore.Store, automationID string, corrID uuid.UUID) {
	t.Helper()
	_, err := s.Append(context.Background(), eventstore.Event{
		Kind:          "automation_triggered",
		Source:        "automation:" + automationID,
		Timestamp:     time.Now(),
		CorrelationID: corrID,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationTriggered{
			AutomationTriggered: &eventv1.AutomationTriggered{
				AutomationId:  automationID,
				CorrelationId: corrID.String(),
				TriggerKind:   "manual",
			},
		}},
	})
	if err != nil {
		t.Fatalf("append triggered: %v", err)
	}
}

func mustAppendAutomationFinished(t *testing.T, s *eventstore.Store, automationID string, corrID uuid.UUID) {
	t.Helper()
	_, err := s.Append(context.Background(), eventstore.Event{
		Kind:          "automation_finished",
		Source:        "automation:" + automationID,
		Timestamp:     time.Now(),
		CorrelationID: corrID,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationFinished{
			AutomationFinished: &eventv1.AutomationFinished{
				AutomationId:  automationID,
				CorrelationId: corrID.String(),
				Outcome:       eventv1.RunOutcome_OUTCOME_OK,
				ElapsedMs:     10,
				StarlarkSteps: 5,
			},
		}},
	})
	if err != nil {
		t.Fatalf("append finished: %v", err)
	}
}

func drainTraceN(t *testing.T, ch <-chan api.TraceEvent, n int, timeout time.Duration) []api.TraceEvent {
	t.Helper()
	out := make([]api.TraceEvent, 0, n)
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case te, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, te)
		case <-deadline:
			t.Fatalf("timed out after %d/%d events", len(out), n)
		}
	}
	return out
}

func TestTraceAdapter_StoreNil_Errors(t *testing.T) {
	a := &automationControlAdapter{store: nil}
	_, _, err := a.Trace(context.Background(), "any", "", 0)
	if err == nil {
		t.Fatal("expected error when store is nil")
	}
}

func TestTraceAdapter_StreamsEvents(t *testing.T) {
	store := newTraceTestStore(t)
	a := &automationControlAdapter{store: store}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, traceCancel, err := a.Trace(ctx, "foo", "", 0)
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	defer traceCancel()

	corrID := uuid.New()
	mustAppendAutomationTriggered(t, store, "foo", corrID)
	mustAppendAutomationFinished(t, store, "foo", corrID)

	events := drainTraceN(t, ch, 2, 2*time.Second)
	if events[0].Kind != "automation_triggered" {
		t.Errorf("events[0].Kind = %q, want automation_triggered", events[0].Kind)
	}
	if events[0].AutomationID != "foo" {
		t.Errorf("events[0].AutomationID = %q, want foo", events[0].AutomationID)
	}
	if events[1].Kind != "automation_finished" {
		t.Errorf("events[1].Kind = %q, want automation_finished", events[1].Kind)
	}
	if events[0].RunID != corrID.String() {
		t.Errorf("events[0].RunID = %q, want %q", events[0].RunID, corrID.String())
	}
}

func TestTraceAdapter_FiltersByAutomationID(t *testing.T) {
	store := newTraceTestStore(t)
	a := &automationControlAdapter{store: store}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, traceCancel, err := a.Trace(ctx, "wanted", "", 0)
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	defer traceCancel()

	// Append events for a different automation — these must NOT come through.
	mustAppendAutomationTriggered(t, store, "other", uuid.New())
	mustAppendAutomationFinished(t, store, "other", uuid.New())
	// And one for the wanted automation — this MUST come through.
	wantedCorrID := uuid.New()
	mustAppendAutomationTriggered(t, store, "wanted", wantedCorrID)

	select {
	case te := <-ch:
		if te.AutomationID != "wanted" {
			t.Errorf("expected only wanted events, got AutomationID=%q", te.AutomationID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for wanted event")
	}

	// Confirm no spurious extras within a small window.
	select {
	case te := <-ch:
		if te.AutomationID != "wanted" {
			t.Fatalf("leaked event from other automation: %+v", te)
		}
	case <-time.After(150 * time.Millisecond):
		// good — no leakage
	}
}

func TestTraceAdapter_FiltersByRunID(t *testing.T) {
	store := newTraceTestStore(t)
	a := &automationControlAdapter{store: store}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wantedCorrID := uuid.New()
	otherCorrID := uuid.New()

	ch, traceCancel, err := a.Trace(ctx, "auto", wantedCorrID.String(), 0)
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	defer traceCancel()

	// Both correlation IDs share the same automation, but only the wanted one should reach the channel.
	mustAppendAutomationTriggered(t, store, "auto", otherCorrID)
	mustAppendAutomationTriggered(t, store, "auto", wantedCorrID)

	select {
	case te := <-ch:
		if te.RunID != wantedCorrID.String() {
			t.Errorf("RunID = %q, want %q", te.RunID, wantedCorrID.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for wanted event")
	}

	// Confirm no spurious extras for otherCorrID.
	select {
	case te := <-ch:
		if te.RunID != wantedCorrID.String() {
			t.Fatalf("leaked event with RunID=%q (want only %q)", te.RunID, wantedCorrID.String())
		}
	case <-time.After(150 * time.Millisecond):
		// good — no leakage
	}
}

func TestTraceAdapter_InvalidRunID_Errors(t *testing.T) {
	store := newTraceTestStore(t)
	a := &automationControlAdapter{store: store}
	_, _, err := a.Trace(context.Background(), "auto", "not-a-uuid", 0)
	if err == nil {
		t.Fatal("expected error for invalid run_id")
	}
	if !strings.Contains(err.Error(), "invalid run_id") {
		t.Errorf("err = %v, want it to contain 'invalid run_id'", err)
	}
}

// ----- scriptRunnerAdapter.RunTests -----

func newTestStarlarkRuntime(t *testing.T) *ghstarlark.Runtime {
	t.Helper()
	return ghstarlark.NewRuntime(nil, nil, nil, nil, t.TempDir(), nil)
}

func writeTempStarlarkFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestRunTestsAdapter_NilRuntime(t *testing.T) {
	a := &scriptRunnerAdapter{rt: nil}
	_, _, err := a.RunTests(context.Background(), "any.star")
	if err == nil {
		t.Fatal("expected error when runtime is nil")
	}
	if !strings.Contains(err.Error(), "starlark runtime not available") {
		t.Errorf("err = %v, want 'starlark runtime not available'", err)
	}
}

func TestRunTestsAdapter_FileNotFound(t *testing.T) {
	a := &scriptRunnerAdapter{rt: newTestStarlarkRuntime(t), configDir: t.TempDir()}
	_, _, err := a.RunTests(context.Background(), "does_not_exist_test.star")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRunTestsAdapter_EmptyPath(t *testing.T) {
	a := &scriptRunnerAdapter{rt: newTestStarlarkRuntime(t)}
	_, _, err := a.RunTests(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestRunTestsAdapter_StreamsResults(t *testing.T) {
	dir := t.TempDir()
	src := `
def test_passes():
    assert(1 + 1 == 2)

def test_fails():
    assert(False, "boom")
`
	path := writeTempStarlarkFile(t, dir, "thing_test.star", src)

	a := &scriptRunnerAdapter{rt: newTestStarlarkRuntime(t)}
	ch, cancel, err := a.RunTests(context.Background(), path)
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	defer cancel()

	var events []api.StarlarkTestEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
	}
	// Results are sorted alphabetically: test_fails, test_passes.
	if events[0].Name != "test_fails" || events[0].Outcome != "fail" {
		t.Errorf("events[0] = %+v, want test_fails/fail", events[0])
	}
	if !strings.Contains(events[0].Detail, "boom") {
		t.Errorf("events[0].Detail = %q, want it to contain 'boom'", events[0].Detail)
	}
	if events[1].Name != "test_passes" || events[1].Outcome != "ok" {
		t.Errorf("events[1] = %+v, want test_passes/ok", events[1])
	}
}

func TestRunTestsAdapter_ResolvesRelativeAgainstConfigDir(t *testing.T) {
	cfgDir := t.TempDir()
	src := `
def test_a():
    assert(True)
`
	writeTempStarlarkFile(t, cfgDir, "rel_test.star", src)

	// Run from a working dir that does NOT contain the file, so the as-is open fails
	// and the adapter falls back to configDir.
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	a := &scriptRunnerAdapter{rt: newTestStarlarkRuntime(t), configDir: cfgDir}
	ch, cancel, err := a.RunTests(context.Background(), "rel_test.star")
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	defer cancel()
	var events []api.StarlarkTestEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 1 || events[0].Name != "test_a" || events[0].Outcome != "ok" {
		t.Errorf("got %+v, want one ok test_a event", events)
	}
}

func TestRunTestsAdapter_FileLevelErrorIsReportedAsEvent(t *testing.T) {
	dir := t.TempDir()
	path := writeTempStarlarkFile(t, dir, "broken_test.star", `def broken(:`)

	a := &scriptRunnerAdapter{rt: newTestStarlarkRuntime(t)}
	ch, cancel, err := a.RunTests(context.Background(), path)
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	defer cancel()

	var events []api.StarlarkTestEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 synthetic fail event, got %d", len(events))
	}
	if events[0].Outcome != "fail" {
		t.Errorf("expected fail outcome, got %q", events[0].Outcome)
	}
}
