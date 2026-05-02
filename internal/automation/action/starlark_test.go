package action_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fdatoo/switchyard/internal/automation/action"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

func TestStarlark_Executes(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	a := &action.StarlarkAction{Body: `log("hi")`}
	run := &action.Run{Runtime: rt, CorrelationID: "c"}
	if err := a.Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	steps, logs := run.Snapshot()
	if steps == 0 {
		t.Error("expected steps")
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "hi") {
		t.Fatalf("logs %v", logs)
	}
}

func TestStarlark_Error(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	a := &action.StarlarkAction{Body: "1/0"}
	if err := a.Execute(context.Background(), &action.Run{Runtime: rt}); err == nil {
		t.Fatal("want err")
	}
}
