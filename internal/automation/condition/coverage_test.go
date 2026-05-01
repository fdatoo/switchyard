package condition_test

import (
	"context"
	"errors"
	"testing"
	"time"

	starlarkgo "go.starlark.net/starlark"

	"github.com/fdatoo/switchyard/internal/automation/condition"
	"github.com/fdatoo/switchyard/internal/eventstore"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

// --- StateCondition error / default branch ---

func TestState_NoOperatorReturnsError(t *testing.T) {
	c := &condition.StateCondition{Entity: "x"} // no Equals/OneOf/Not
	ok, err := c.Evaluate(context.Background(), condition.Env{State: fakeState{"x": {StateStr: "on"}}})
	if err == nil || ok {
		t.Fatalf("want false,err got %v,%v", ok, err)
	}
}

func TestState_OneOfNoMatch(t *testing.T) {
	c := &condition.StateCondition{Entity: "x", OneOf: []string{"a", "b"}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{State: fakeState{"x": {StateStr: "z"}}})
	if ok {
		t.Fatal("want false")
	}
}

func TestState_NotMatchReturnsFalse(t *testing.T) {
	c := &condition.StateCondition{Entity: "x", Not: "on"}
	ok, _ := c.Evaluate(context.Background(), condition.Env{State: fakeState{"x": {StateStr: "on"}}})
	if ok {
		t.Fatal("want false: Not=on, current=on")
	}
}

// --- NumericCondition: missing attribute, default attribute, unknown op, all toFloat input shapes ---

func TestNumeric_MissingEntityReturnsFalse(t *testing.T) {
	c := &condition.NumericCondition{Entity: "missing", Op: "eq", Value: 1}
	ok, err := c.Evaluate(context.Background(), condition.Env{State: fakeState{}})
	if ok || err != nil {
		t.Fatalf("want false,nil got %v,%v", ok, err)
	}
}

func TestNumeric_MissingAttributeReturnsFalse(t *testing.T) {
	c := &condition.NumericCondition{Entity: "s", Attribute: "missing", Op: "eq", Value: 1}
	env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{}}}}
	ok, _ := c.Evaluate(context.Background(), env)
	if ok {
		t.Fatal("want false")
	}
}

func TestNumeric_DefaultAttributeNameIsValue(t *testing.T) {
	c := &condition.NumericCondition{Entity: "s", Op: "eq", Value: 42}
	env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{"value": 42.0}}}}
	ok, err := c.Evaluate(context.Background(), env)
	if !ok || err != nil {
		t.Fatalf("want true,nil got %v,%v", ok, err)
	}
}

func TestNumeric_UnknownOpReturnsError(t *testing.T) {
	c := &condition.NumericCondition{Entity: "s", Attribute: "v", Op: "wat", Value: 1}
	env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{"v": 1.0}}}}
	if ok, err := c.Evaluate(context.Background(), env); ok || err == nil {
		t.Fatalf("want false,err got %v,%v", ok, err)
	}
}

func TestNumeric_NonNumericAttribute(t *testing.T) {
	c := &condition.NumericCondition{Entity: "s", Attribute: "v", Op: "eq", Value: 1}
	env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{"v": []byte("nope")}}}}
	if ok, _ := c.Evaluate(context.Background(), env); ok {
		t.Fatal("want false: byte slice is not numeric")
	}
}

func TestNumeric_AcceptsAllNumericKinds(t *testing.T) {
	cases := []any{float32(7), int(7), int64(7), "7"}
	for _, v := range cases {
		c := &condition.NumericCondition{Entity: "s", Attribute: "v", Op: "eq", Value: 7}
		env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{"v": v}}}}
		ok, _ := c.Evaluate(context.Background(), env)
		if !ok {
			t.Errorf("input %T(%v) should compare equal to 7", v, v)
		}
	}
}

func TestNumeric_StringNotParseable(t *testing.T) {
	c := &condition.NumericCondition{Entity: "s", Attribute: "v", Op: "eq", Value: 1}
	env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{"v": "not a number"}}}}
	if ok, _ := c.Evaluate(context.Background(), env); ok {
		t.Fatal("want false: 'not a number' should not parse")
	}
}

// --- TimeCondition: branch coverage ---

func TestTime_NilLocDefaultsLocal(t *testing.T) {
	c := &condition.TimeCondition{} // no constraints → always true
	ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Now()})
	if !ok {
		t.Fatal("want true with no constraints")
	}
}

func TestTime_AfterOnly(t *testing.T) {
	loc := time.FixedZone("t", 0)
	c := &condition.TimeCondition{After: "08:00"}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 1, 1, 9, 0, 0, 0, loc), Loc: loc}); !ok {
		t.Error("9:00 should be after 8:00")
	}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 1, 1, 7, 0, 0, 0, loc), Loc: loc}); ok {
		t.Error("7:00 should not be after 8:00")
	}
}

func TestTime_BeforeOnly(t *testing.T) {
	loc := time.FixedZone("t", 0)
	c := &condition.TimeCondition{Before: "20:00"}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 1, 1, 19, 0, 0, 0, loc), Loc: loc}); !ok {
		t.Error("19:00 should be before 20:00")
	}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 1, 1, 21, 0, 0, 0, loc), Loc: loc}); ok {
		t.Error("21:00 should not be before 20:00")
	}
}

func TestTime_AfterBeforeSameDay(t *testing.T) {
	loc := time.FixedZone("t", 0)
	c := &condition.TimeCondition{After: "08:00", Before: "20:00"}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 1, 1, 12, 0, 0, 0, loc), Loc: loc}); !ok {
		t.Error("noon should match 8-20")
	}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 1, 1, 21, 0, 0, 0, loc), Loc: loc}); ok {
		t.Error("21:00 should not match 8-20")
	}
}

func TestTime_UnknownWeekdayReturnsError(t *testing.T) {
	c := &condition.TimeCondition{Weekdays: []string{"someday"}}
	if _, err := c.Evaluate(context.Background(), condition.Env{Now: time.Now()}); err == nil {
		t.Fatal("want error on unknown weekday")
	}
}

func TestTime_WeekdayMissReturnsFalse(t *testing.T) {
	loc := time.FixedZone("t", 0)
	c := &condition.TimeCondition{Weekdays: []string{"mon"}}
	tue := time.Date(2026, 4, 21, 10, 0, 0, 0, loc) // Tuesday
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: tue, Loc: loc}); ok {
		t.Fatal("want false: tuesday should not match mon constraint")
	}
}

// --- StarlarkCondition: nil runtime, error swallowed, non-bool truthy ---

func TestStarlark_NilRuntimeReturnsFalse(t *testing.T) {
	c := &condition.StarlarkCondition{Expr: "True"}
	ok, err := c.Evaluate(context.Background(), condition.Env{Runtime: nil})
	if ok || err != nil {
		t.Fatalf("want false,nil got %v,%v", ok, err)
	}
}

func TestStarlark_RuntimeErrorSwallowed(t *testing.T) {
	rt := newStarlarkRuntime(t)
	c := &condition.StarlarkCondition{Expr: "1 / 0"}
	ok, err := c.Evaluate(context.Background(), condition.Env{Runtime: rt})
	if err != nil {
		t.Fatalf("err should be swallowed, got %v", err)
	}
	if ok {
		t.Fatal("want false on runtime error")
	}
}

func TestStarlark_TrueExpr(t *testing.T) {
	rt := newStarlarkRuntime(t)
	c := &condition.StarlarkCondition{Expr: "1 + 1 == 2"}
	ok, _ := c.Evaluate(context.Background(), condition.Env{Runtime: rt})
	if !ok {
		t.Fatal("want true")
	}
}

func TestStarlark_NonBoolTruthy(t *testing.T) {
	rt := newStarlarkRuntime(t)
	c := &condition.StarlarkCondition{Expr: "[1, 2]"} // non-empty list is truthy
	ok, _ := c.Evaluate(context.Background(), condition.Env{Runtime: rt})
	if !ok {
		t.Fatal("want true: non-empty list is truthy")
	}
}

func TestStarlark_EventInjected(t *testing.T) {
	rt := newStarlarkRuntime(t)
	// In KindTriggerCondition, event globals are exposed via the `event` struct.
	c := &condition.StarlarkCondition{Expr: `event.kind == "alarm"`}
	env := condition.Env{
		Runtime: rt,
		Event:   &eventstore.Event{Kind: "alarm", Entity: "sensor.x"},
	}
	ok, err := c.Evaluate(context.Background(), env)
	if err != nil {
		t.Fatalf("eval err: %v", err)
	}
	if !ok {
		t.Fatal("want true: event.kind should be 'alarm'")
	}
}

// --- AndCondition / OrCondition / NotCondition: error propagation paths ---

type errEvaluator struct{}

func (errEvaluator) Evaluate(_ context.Context, _ condition.Env) (bool, error) {
	return false, errors.New("boom")
}

func TestAnd_ErrPropagates(t *testing.T) {
	c := &condition.AndCondition{All: []condition.Evaluator{errEvaluator{}}}
	if _, err := c.Evaluate(context.Background(), condition.Env{}); err == nil {
		t.Fatal("want err")
	}
}

func TestAnd_ShortCircuitOnFalse(t *testing.T) {
	called := false
	tracker := condition.Evaluator(funcEval(func() (bool, error) { called = true; return true, nil }))
	c := &condition.AndCondition{All: []condition.Evaluator{
		funcEval(func() (bool, error) { return false, nil }),
		tracker,
	}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{})
	if ok {
		t.Error("want false")
	}
	if called {
		t.Error("second evaluator should not have been called after first false")
	}
}

func TestOr_ErrPropagates(t *testing.T) {
	c := &condition.OrCondition{Any: []condition.Evaluator{errEvaluator{}}}
	if _, err := c.Evaluate(context.Background(), condition.Env{}); err == nil {
		t.Fatal("want err")
	}
}

func TestOr_ShortCircuitOnTrue(t *testing.T) {
	called := false
	tracker := funcEval(func() (bool, error) { called = true; return false, nil })
	c := &condition.OrCondition{Any: []condition.Evaluator{
		funcEval(func() (bool, error) { return true, nil }),
		tracker,
	}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{})
	if !ok {
		t.Error("want true")
	}
	if called {
		t.Error("second evaluator should not have been called after first true")
	}
}

func TestOr_AllFalse(t *testing.T) {
	c := &condition.OrCondition{Any: []condition.Evaluator{
		funcEval(func() (bool, error) { return false, nil }),
		funcEval(func() (bool, error) { return false, nil }),
	}}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{}); ok {
		t.Fatal("want false when all false")
	}
}

func TestNot_ErrPropagates(t *testing.T) {
	c := &condition.NotCondition{Inner: errEvaluator{}}
	if _, err := c.Evaluate(context.Background(), condition.Env{}); err == nil {
		t.Fatal("want err")
	}
}

// helpers

type funcEval func() (bool, error)

func (f funcEval) Evaluate(_ context.Context, _ condition.Env) (bool, error) { return f() }

func newStarlarkRuntime(t *testing.T) *ghstarlark.Runtime {
	t.Helper()
	return ghstarlark.NewRuntime(
		fakeState{},
		nil, nil, nil,
		t.TempDir(),
		nil,
	)
}

// silence unused import lint when starlarkgo is not directly referenced
var _ = starlarkgo.True
