// Package testutil provides a pre-wired Runtime with injectable fakes
// for use in Go tests (primarily C6 automation/condition unit tests).
package testutil

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	starlarkgo "go.starlark.net/starlark"

	"github.com/fdatoo/switchyard/internal/eventstore"
	ghs "github.com/fdatoo/switchyard/internal/starlark"
)

// FakeState maps entity ID → EntityState for test injection.
type FakeState map[string]*ghs.EntityState

func (f FakeState) Get(id string) (*ghs.EntityState, bool) {
	v, ok := f[id]
	return v, ok
}

// DispatchCall records one call to FakeDispatcher.Dispatch.
type DispatchCall struct {
	EntityID   string
	Capability string
	Args       map[string]string
}

// FakeDispatcher records all Dispatch calls for assertion.
type FakeDispatcher struct {
	mu    sync.Mutex
	Calls []DispatchCall
}

func (f *FakeDispatcher) Dispatch(_ context.Context, entityID, capability string, args map[string]string) (*ghs.DispatchResult, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, DispatchCall{EntityID: entityID, Capability: capability, Args: args})
	f.mu.Unlock()
	return &ghs.DispatchResult{Ok: true}, nil
}

// nopAppender satisfies EventAppender with no-op behaviour.
type nopAppender struct{}

func (nopAppender) Append(_ context.Context, _ eventstore.Event) (uint64, error) { return 1, nil }

// NewTestRuntime returns a Runtime with fake dependencies.
// seed controls random() output (0 = random seed).
func NewTestRuntime(state FakeState, dispatcher *FakeDispatcher, seed int64) *ghs.Runtime {
	rt := ghs.NewRuntime(state, dispatcher, nopAppender{}, slog.Default(), "", nil)
	if seed != 0 {
		rt = rt.WithRandSeed(seed)
	}
	return rt
}

// ScriptResult wraps ghs.Result for assertion helpers.
type ScriptResult struct {
	Value   starlarkgo.Value
	Logs    []string
	Steps   uint64
	Elapsed interface{}
}

// RunScript executes script in the given context kind. Calls t.Fatal on error.
func RunScript(t testing.TB, rt *ghs.Runtime, kind ghs.ContextKind, script string) *ScriptResult {
	t.Helper()
	res, err := rt.Execute(context.Background(), kind, script, nil)
	if err != nil {
		t.Fatalf("RunScript(%s): %v", kind, err)
	}
	return &ScriptResult{
		Value:   res.Value,
		Logs:    res.Logs,
		Steps:   res.Steps,
		Elapsed: res.Elapsed,
	}
}

// AssertValue asserts result.Value equals expected.
func AssertValue(t testing.TB, result *ScriptResult, expected starlarkgo.Value) {
	t.Helper()
	if result.Value != expected {
		if ok, _ := starlarkgo.Equal(result.Value, expected); !ok {
			t.Errorf("AssertValue: got %v, want %v", result.Value, expected)
		}
	}
}

// AssertLog asserts at least one log entry contains the given substring.
func AssertLog(t testing.TB, result *ScriptResult, contains string) {
	t.Helper()
	for _, l := range result.Logs {
		if strings.Contains(l, contains) {
			return
		}
	}
	t.Errorf("AssertLog: no log entry contains %q; logs: %v", contains, result.Logs)
}

// AssertCallService asserts FakeDispatcher received a call to entityID.capability.
func AssertCallService(t testing.TB, d *FakeDispatcher, entityID, capability string) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.Calls {
		if c.EntityID == entityID && c.Capability == capability {
			return
		}
	}
	t.Errorf("AssertCallService: no call to %s.%s; calls: %v", entityID, capability, d.Calls)
}

// AssertNoCallService asserts FakeDispatcher received no Dispatch calls.
func AssertNoCallService(t testing.TB, d *FakeDispatcher) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.Calls) > 0 {
		t.Errorf("AssertNoCallService: unexpected calls: %v", d.Calls)
	}
}

// AssertError asserts err is non-nil and its message contains the given substring.
func AssertError(t testing.TB, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Errorf("AssertError: expected error containing %q, got nil", contains)
		return
	}
	if !strings.Contains(err.Error(), contains) {
		t.Errorf("AssertError: error %q does not contain %q", err.Error(), contains)
	}
}
