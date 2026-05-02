package testutil_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/script"
	scripttestutil "github.com/fdatoo/switchyard/internal/script/testutil"
)

func TestSyntheticScript(t *testing.T) {
	s := scripttestutil.SyntheticScript("noop", "return None", scripttestutil.StringParam("x", false))
	if s.Name != "noop" || s.Handler != "return None" {
		t.Fatalf("bad synthetic: %+v", s)
	}
	if len(s.Params) != 1 || s.Params[0].Name != "x" {
		t.Fatalf("params: %+v", s.Params)
	}
}

func TestNewEngineSmoke(t *testing.T) {
	eng := scripttestutil.NewEngine(t, map[string]*script.Script{
		"hi": scripttestutil.SyntheticScript("hi", `x = 1`),
	})
	if names := eng.List(); len(names) != 1 || names[0] != "hi" {
		t.Fatalf("List = %v, want [hi]", names)
	}
}

func TestNewEngineWithStore_RecordsInvocations(t *testing.T) {
	eng, app := scripttestutil.NewEngineWithStore(t, map[string]*script.Script{
		"hi": scripttestutil.SyntheticScript("hi", `x = 1`),
	})
	res, err := eng.Call(context.Background(), "hi", nil, "test:user", "")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res == nil || res.CorrelationID == "" {
		t.Fatalf("missing result: %+v", res)
	}
	if got := app.CountByKind("script_invoked"); got != 1 {
		t.Errorf("script_invoked count = %d, want 1", got)
	}
	if got := app.CountByKind("script_finished"); got != 1 {
		t.Errorf("script_finished count = %d, want 1", got)
	}
}
