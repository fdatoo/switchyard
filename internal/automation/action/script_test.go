package action_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fdatoo/switchyard/internal/automation/action"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

type fakeScriptRes struct {
	ok    bool
	err   string
	steps uint64
	logs  []string
}

func (f fakeScriptRes) Succeeded() bool   { return f.ok }
func (f fakeScriptRes) GetError() string  { return f.err }
func (f fakeScriptRes) GetSteps() uint64  { return f.steps }
func (f fakeScriptRes) GetLogs() []string { return f.logs }

type fakeCaller struct {
	lastName, lastBy, lastCorr string
	lastArgs                   map[string]string
	res                        fakeScriptRes
	err                        error
}

func (f *fakeCaller) Call(_ context.Context, n string, a map[string]string, by, s string) (action.ScriptCallResult, error) {
	f.lastName, f.lastArgs, f.lastBy, f.lastCorr = n, a, by, s
	return f.res, f.err
}

func TestScriptAction_PassesCorr(t *testing.T) {
	c := &fakeCaller{res: fakeScriptRes{ok: true}}
	a := &action.ScriptAction{Name: "g"}
	run := &action.Run{Scripts: c, CorrelationID: "corrX", AutomationID: "autA"}
	if err := a.Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if c.lastCorr != "corrX" || c.lastBy != "automation:autA" {
		t.Fatalf("bad %+v", c)
	}
}

func TestScriptAction_Error(t *testing.T) {
	c := &fakeCaller{err: errors.New("boom")}
	a := &action.ScriptAction{Name: "x"}
	if err := a.Execute(context.Background(), &action.Run{Scripts: c}); err == nil {
		t.Fatal("want err")
	}
}

// TestScriptAction_LimitError_Wraps verifies that a *ghstarlark.LimitError
// returned by the ScriptCaller is preserved through errors.As after Execute
// wraps it. This is critical: classify() in run.go uses errors.As to detect
// LimitError and assign OUTCOME_LIMIT_EXCEEDED rather than OUTCOME_ACTION_ERROR.
func TestScriptAction_LimitError_Wraps(t *testing.T) {
	limErr := &ghstarlark.LimitError{Kind: ghstarlark.LimitSteps, Context: ghstarlark.KindScript, Detail: "100 steps"}
	c := &fakeCaller{
		res: fakeScriptRes{ok: false, err: limErr.Error()},
		err: limErr,
	}
	a := &action.ScriptAction{Name: "x"}
	err := a.Execute(context.Background(), &action.Run{Scripts: c})
	if err == nil {
		t.Fatal("want err")
	}
	var le *ghstarlark.LimitError
	if !errors.As(err, &le) {
		t.Fatalf("errors.As(*LimitError) = false; got %T: %v", err, err)
	}
}
