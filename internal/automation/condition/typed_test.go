package condition_test

import (
	"context"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/automation/condition"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

type fakeState map[string]*ghstarlark.EntityState

func (f fakeState) Get(id string) (*ghstarlark.EntityState, bool) { v, ok := f[id]; return v, ok }

func TestState_Equals(t *testing.T) {
	c := &condition.StateCondition{Entity: "x", Equals: "on"}
	ok, _ := c.Evaluate(context.Background(), condition.Env{State: fakeState{"x": {StateStr: "on"}}})
	if !ok {
		t.Fatal("want true")
	}
}

func TestState_OneOf(t *testing.T) {
	c := &condition.StateCondition{Entity: "x", OneOf: []string{"a", "b"}}
	ok, _ := c.Evaluate(context.Background(), condition.Env{State: fakeState{"x": {StateStr: "b"}}})
	if !ok {
		t.Fatal("want true")
	}
}

func TestState_Not(t *testing.T) {
	c := &condition.StateCondition{Entity: "x", Not: "off"}
	ok, _ := c.Evaluate(context.Background(), condition.Env{State: fakeState{"x": {StateStr: "on"}}})
	if !ok {
		t.Fatal("want true")
	}
}

func TestState_MissingEntity(t *testing.T) {
	c := &condition.StateCondition{Entity: "m", Equals: "on"}
	ok, err := c.Evaluate(context.Background(), condition.Env{State: fakeState{}})
	if ok || err != nil {
		t.Fatalf("want false,nil got %v,%v", ok, err)
	}
}

func TestNumeric_Ops(t *testing.T) {
	cases := []struct {
		op         string
		have, want float64
		expect     bool
	}{
		{"lt", 5, 10, true}, {"lte", 10, 10, true}, {"eq", 10, 10, true},
		{"gte", 10, 10, true}, {"gt", 11, 10, true}, {"gt", 10, 10, false},
	}
	for _, tc := range cases {
		c := &condition.NumericCondition{Entity: "s", Attribute: "v", Op: tc.op, Value: tc.want}
		env := condition.Env{State: fakeState{"s": {Attributes: map[string]any{"v": tc.have}}}}
		got, _ := c.Evaluate(context.Background(), env)
		if got != tc.expect {
			t.Errorf("%s %v %v: got %v want %v", tc.op, tc.have, tc.want, got, tc.expect)
		}
	}
}

func TestTime_AfterBeforeOvernight(t *testing.T) {
	loc := time.FixedZone("t", 0)
	c := &condition.TimeCondition{After: "22:00", Before: "06:00"}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 4, 24, 10, 0, 0, 0, loc), Loc: loc}); ok {
		t.Error("10:00 outside 22-06")
	}
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: time.Date(2026, 4, 24, 2, 0, 0, 0, loc), Loc: loc}); !ok {
		t.Error("02:00 inside 22-06 overnight")
	}
}

func TestTime_Weekdays(t *testing.T) {
	loc := time.FixedZone("t", 0)
	c := &condition.TimeCondition{Weekdays: []string{"mon"}}
	mon := time.Date(2026, 4, 20, 10, 0, 0, 0, loc)
	if ok, _ := c.Evaluate(context.Background(), condition.Env{Now: mon, Loc: loc}); !ok {
		t.Error("monday should match")
	}
}
