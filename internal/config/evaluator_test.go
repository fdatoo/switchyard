package config

import (
	"context"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

type fakeEval struct {
	snap *configpb.ConfigSnapshot
	err  error
}

func (f *fakeEval) Evaluate(_ context.Context, _ string) (*configpb.ConfigSnapshot, error) {
	return f.snap, f.err
}

func TestFakeEvalSatisfiesInterface(t *testing.T) {
	var _ configEvaluator = &fakeEval{}
}
