// Package testutil provides fakes for automation engine tests.
package testutil

import (
	"context"
	"sync"
)

type FakeSceneApplier struct {
	mu      sync.Mutex
	Applied []string
	Err     error
}

func (f *FakeSceneApplier) Apply(_ context.Context, slug, _ string) error {
	f.mu.Lock()
	f.Applied = append(f.Applied, slug)
	f.mu.Unlock()
	return f.Err
}

func (f *FakeSceneApplier) Slugs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.Applied))
	copy(out, f.Applied)
	return out
}
