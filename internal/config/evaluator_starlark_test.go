//go:build integration

package config

import (
	"context"
	"errors"
	"testing"
)

func TestPklValidator_RejectsInvalidStarlark(t *testing.T) {
	ctx := context.Background()
	ev, err := newPklEvaluator(ctx)
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}
	_, err = ev.Evaluate(ctx, testdataDir(t, "bad-starlark"))
	if err == nil {
		t.Fatal("expected validation error for bad Starlark expression")
	}
	var ee *EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
}
