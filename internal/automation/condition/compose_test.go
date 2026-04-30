package condition_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fdatoo/gohome/internal/automation/condition"
)

type fakeEv struct {
	result bool
	err    error
	calls  int
}

func (f *fakeEv) Evaluate(_ context.Context, _ condition.Env) (bool, error) {
	f.calls++
	return f.result, f.err
}

func TestAnd_ShortCircuit(t *testing.T) {
	a := &fakeEv{result: false}
	b := &fakeEv{result: true}
	c := &condition.AndCondition{All: []condition.Evaluator{a, b}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{})
	if ok || b.calls != 0 {
		t.Fatalf("ok=%v b.calls=%d", ok, b.calls)
	}
}

func TestOr_ShortCircuit(t *testing.T) {
	a := &fakeEv{result: true}
	b := &fakeEv{result: false}
	c := &condition.OrCondition{Any: []condition.Evaluator{a, b}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{})
	if !ok || b.calls != 0 {
		t.Fatal("bad")
	}
}

func TestNot_Inverts(t *testing.T) {
	c := &condition.NotCondition{Inner: &fakeEv{result: false}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{})
	if !ok {
		t.Fatal("want true")
	}
}

func TestNot_PreservesError(t *testing.T) {
	c := &condition.NotCondition{Inner: &fakeEv{err: errors.New("x")}}
	if _, err := c.Evaluate(context.Background(), condition.Env{}); err == nil {
		t.Fatal("want err")
	}
}
