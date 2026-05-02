package condition_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/automation/condition"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

func TestStarlarkCond_True(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	c := &condition.StarlarkCondition{Expr: "1+1 == 2"}
	ok, err := c.Evaluate(context.Background(), condition.Env{Runtime: rt})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestStarlarkCond_RuntimeErrIsFalse(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	c := &condition.StarlarkCondition{Expr: "1/0"}
	ok, err := c.Evaluate(context.Background(), condition.Env{Runtime: rt})
	if ok || err != nil {
		t.Fatalf("want false,nil got %v,%v", ok, err)
	}
}
