//go:build integration

package config

import (
	"context"
	"reflect"
	"testing"
)

func TestCompileIntegration_InvalidXref(t *testing.T) {
	ctx := context.Background()
	ev, err := newPklEvaluator(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}

	snap, err := ev.Evaluate(ctx, testdataDir(t, "invalid-xref"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	errs := Compile(snap, nil)
	want := []ValidationError{{
		Field:   "entities[invalid_no_dot].id",
		Message: `entity id must be "<type>.<name>" e.g. "light.living_room"`,
	}}
	if !reflect.DeepEqual(errs, want) {
		t.Fatalf("Compile errors = %#v, want %#v", errs, want)
	}
}
