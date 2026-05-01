package script_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/script"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

// fakeAppender records events for assertion.
type fakeAppender struct {
	mu     sync.Mutex
	events []eventstore.Event
}

func (f *fakeAppender) Append(_ context.Context, e eventstore.Event) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e.Position = uint64(len(f.events) + 1)
	f.events = append(f.events, e)
	return e.Position, nil
}

func (f *fakeAppender) Events() []eventstore.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]eventstore.Event, len(f.events))
	copy(out, f.events)
	return out
}

func newTestEngine(t *testing.T, snap *configpb.ConfigSnapshot) (*script.Engine, *fakeAppender) {
	t.Helper()
	scripts, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatal(err)
	}
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	ap := &fakeAppender{}
	eng := script.NewEngine(scripts, rt, script.Deps{Store: ap})
	return eng, ap
}

func TestEngine_ListEmpty(t *testing.T) {
	e := script.NewEngine(nil, nil, script.Deps{})
	if got := e.List(); len(got) != 0 {
		t.Fatalf("got %d", len(got))
	}
}

func TestEngine_ListAndGet(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{
		{Name: "a", Handler: "def main(params): pass"},
		{Name: "b", Handler: "def main(params): pass"},
	}}
	m, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatal(err)
	}
	e := script.NewEngine(m, nil, script.Deps{})
	names := e.List()
	if len(names) != 2 {
		t.Fatalf("got %v", names)
	}
	s, err := e.Get("a")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "a" {
		t.Fatalf("got %q", s.Name)
	}
	if _, err := e.Get("missing"); err == nil {
		t.Fatal("want ErrScriptNotFound")
	}
}

func TestCall_NotFound(t *testing.T) {
	eng, _ := newTestEngine(t, &configpb.ConfigSnapshot{})
	_, err := eng.Call(context.Background(), "nope", nil, "cli:test", "")
	if err == nil {
		t.Fatal("want ErrScriptNotFound")
	}
}

func TestCall_OK_EmitsEvents(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{
		{Name: "greet", Handler: `log("hi")`},
	}}
	eng, ap := newTestEngine(t, snap)
	res, err := eng.Call(context.Background(), "greet", nil, "cli:test", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.CorrelationID == "" {
		t.Fatal("empty corr id")
	}
	if _, err := uuid.Parse(res.CorrelationID); err != nil {
		t.Fatalf("not a uuid: %v", err)
	}
	evs := ap.Events()
	if len(evs) != 2 {
		t.Fatalf("want 2 events, got %d", len(evs))
	}
	if evs[0].Kind != "script_invoked" || evs[1].Kind != "script_finished" {
		t.Fatalf("kinds = %q, %q", evs[0].Kind, evs[1].Kind)
	}
	fin := evs[1].Payload.GetScriptFinished()
	if fin == nil {
		t.Fatal("not a ScriptFinished payload")
	}
	if fin.GetOutcome() != eventv1.RunOutcome_OUTCOME_OK {
		t.Errorf("outcome = %v", fin.GetOutcome())
	}
	_ = time.Second
}

func TestCall_SharedCorrelation(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{
		{Name: "noop", Handler: "x = 1"},
	}}
	eng, ap := newTestEngine(t, snap)
	shared := uuid.NewString()
	res, err := eng.Call(context.Background(), "noop", nil, "automation:x", shared)
	if err != nil {
		t.Fatal(err)
	}
	if res.CorrelationID != shared {
		t.Fatalf("corr = %q, want %q", res.CorrelationID, shared)
	}
	_ = ap
}

func TestCall_ArgValidation(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{
		{Name: "x", Handler: "x = 1", Params: []*configpb.ScriptParam{
			{Name: "n", Type: configpb.ScriptParam_TYPE_INT, Required: true},
		}},
	}}
	eng, _ := newTestEngine(t, snap)

	if _, err := eng.Call(context.Background(), "x", nil, "cli:t", ""); err == nil {
		t.Fatal("want missing-required")
	}
	if _, err := eng.Call(context.Background(), "x", map[string]string{"n": "abc"}, "cli:t", ""); err == nil {
		t.Fatal("want coerce error")
	}
	if _, err := eng.Call(context.Background(), "x", map[string]string{"n": "1", "bogus": "z"}, "cli:t", ""); err == nil {
		t.Fatal("want unknown-key error")
	}
}
