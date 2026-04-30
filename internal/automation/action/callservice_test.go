package action_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fdatoo/gohome/internal/automation/action"
	"github.com/fdatoo/gohome/internal/cause"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

type fakeDisp struct {
	lastEntity, lastCap string
	lastArgs            map[string]string
	lastCtx             context.Context
	result              *ghstarlark.DispatchResult
	err                 error
}

func (f *fakeDisp) Dispatch(ctx context.Context, e, c string, a map[string]string) (*ghstarlark.DispatchResult, error) {
	f.lastCtx = ctx
	f.lastEntity = e
	f.lastCap = c
	f.lastArgs = a
	return f.result, f.err
}

func TestCallService_OK(t *testing.T) {
	d := &fakeDisp{result: &ghstarlark.DispatchResult{Ok: true}}
	a := &action.CallServiceAction{Entity: "light.a", Capability: "turn_on", Args: map[string]string{"level": "80"}}
	if err := a.Execute(context.Background(), &action.Run{Dispatcher: d}); err != nil {
		t.Fatal(err)
	}
	if d.lastEntity != "light.a" || d.lastCap != "turn_on" {
		t.Fatalf("bad: %q %q", d.lastEntity, d.lastCap)
	}
}

func TestCallService_Error(t *testing.T) {
	d := &fakeDisp{err: errors.New("boom")}
	a := &action.CallServiceAction{Entity: "x", Capability: "y"}
	if err := a.Execute(context.Background(), &action.Run{Dispatcher: d}); err == nil {
		t.Fatal("want err")
	}
}

func TestCallService_NotOk(t *testing.T) {
	d := &fakeDisp{result: &ghstarlark.DispatchResult{Ok: false, Error: "nope"}}
	a := &action.CallServiceAction{Entity: "x", Capability: "y"}
	err := a.Execute(context.Background(), &action.Run{Dispatcher: d})
	var de *action.DispatchError
	if !errors.As(err, &de) {
		t.Fatalf("want DispatchError, got %T", err)
	}
}

// TestCallService_CorrelationThreaded verifies that CallServiceAction stamps
// automation lineage onto the context passed to Dispatcher.Dispatch. The
// carport layer reads this via cause.FromCorrelation to set CommandIssued.Source.
func TestCallService_CorrelationThreaded(t *testing.T) {
	d := &fakeDisp{result: &ghstarlark.DispatchResult{Ok: true}}
	a := &action.CallServiceAction{Entity: "light.a", Capability: "turn_on"}
	run := &action.Run{
		Dispatcher:    d,
		AutomationID:  "auto1",
		CorrelationID: "corr-abc",
	}
	if err := a.Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	corr, ok := cause.FromCorrelation(d.lastCtx)
	if !ok {
		t.Fatal("no correlation in dispatched ctx")
	}
	if corr.AutomationID != "auto1" {
		t.Errorf("AutomationID = %q, want %q", corr.AutomationID, "auto1")
	}
	if corr.CorrelationID != "corr-abc" {
		t.Errorf("CorrelationID = %q, want %q", corr.CorrelationID, "corr-abc")
	}
}
