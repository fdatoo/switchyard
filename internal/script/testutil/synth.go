// Package testutil provides synthesized helpers for testing the script engine
// without going through Pkl compile or a full daemon. Mirrors the shape of
// internal/automation/testutil for consistency.
package testutil

import (
	"context"
	"sync"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/script"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

// SyntheticScript builds an in-memory *script.Script bypassing the proto/Pkl
// compile pipeline. Useful for engine-level tests that just need a callable
// script without exercising parameter validation logic.
func SyntheticScript(name, handler string, params ...script.Param) *script.Script {
	return &script.Script{Name: name, Handler: handler, Params: params}
}

// NewEngine constructs a script.Engine over the provided scripts using a
// freshly-built Starlark runtime from the starlark testutil. Pass nil scripts
// to build an empty engine.
func NewEngine(t *testing.T, scripts map[string]*script.Script) *script.Engine {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	if scripts == nil {
		scripts = map[string]*script.Script{}
	}
	return script.NewEngine(scripts, rt, script.Deps{})
}

// NewEngineWithStore is NewEngine but threads a synthetic event store so
// Call() can emit script_invoked / script_finished events for assertions.
func NewEngineWithStore(t *testing.T, scripts map[string]*script.Script) (*script.Engine, *FakeAppender) {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	if scripts == nil {
		scripts = map[string]*script.Script{}
	}
	app := &FakeAppender{}
	eng := script.NewEngine(scripts, rt, script.Deps{Store: app})
	return eng, app
}

// FakeAppender satisfies script.EventAppender by recording every Append call.
type FakeAppender struct {
	mu     sync.Mutex
	Events []eventstore.Event
}

// Append implements script.EventAppender.
func (f *FakeAppender) Append(_ context.Context, e eventstore.Event) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Events = append(f.Events, e)
	return uint64(len(f.Events)), nil
}

// Snapshot returns a defensive copy of recorded events.
func (f *FakeAppender) Snapshot() []eventstore.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]eventstore.Event, len(f.Events))
	copy(out, f.Events)
	return out
}

// CountByKind returns how many events of the given kind have been appended.
func (f *FakeAppender) CountByKind(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.Events {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// IntParam is a convenience constructor for the common "required int" param.
func IntParam(name string, required bool) script.Param {
	return script.Param{Name: name, Type: configpb.ScriptParam_TYPE_INT, Required: required}
}

// StringParam is a convenience constructor for the common "required string" param.
func StringParam(name string, required bool) script.Param {
	return script.Param{Name: name, Type: configpb.ScriptParam_TYPE_STRING, Required: required}
}

// Runtime exposes the underlying *ghstarlark.Runtime for tests that need to
// drive it directly (e.g. RunTestsInFile assertions). It returns the runtime
// the engine was constructed with.
func Runtime(eng *script.Engine) *ghstarlark.Runtime {
	if eng == nil {
		return nil
	}
	return eng.Runtime()
}
