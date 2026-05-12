package config

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeApplier counts Apply calls; returns the configured error.
type fakeApplier struct {
	mu        sync.Mutex
	calls     int32
	err       error
	lastSrcs  []string
	applyDone chan struct{}
}

func (f *fakeApplier) Apply(_ context.Context, source string) error {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	f.lastSrcs = append(f.lastSrcs, source)
	err := f.err
	done := f.applyDone
	f.mu.Unlock()
	if done != nil {
		close(done)
	}
	return err
}

func TestReloader_DebouncesBurst(t *testing.T) {
	app := &fakeApplier{}
	r := NewReloader(app, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	for i := 0; i < 5; i++ {
		r.Trigger("watcher")
	}
	time.Sleep(200 * time.Millisecond)

	got := atomic.LoadInt32(&app.calls)
	if got != 1 {
		t.Errorf("want 1 Apply call (debounced), got %d", got)
	}
}

func TestReloader_SeparateBurstsEachApply(t *testing.T) {
	app := &fakeApplier{}
	r := NewReloader(app, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	r.Trigger("form")
	r.Trigger("form")
	time.Sleep(200 * time.Millisecond)
	r.Trigger("rpc")
	time.Sleep(200 * time.Millisecond)

	got := atomic.LoadInt32(&app.calls)
	if got != 2 {
		t.Errorf("want 2 Apply calls, got %d", got)
	}
}

func TestReloader_TracksLastError(t *testing.T) {
	app := &fakeApplier{err: errors.New("apply failed")}
	r := NewReloader(app, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	r.Trigger("rpc")
	time.Sleep(100 * time.Millisecond)

	if got := r.LastError(); got == "" || got != "apply failed" {
		t.Errorf("LastError = %q, want %q", got, "apply failed")
	}

	app.mu.Lock()
	app.err = nil
	app.mu.Unlock()

	r.Trigger("rpc")
	time.Sleep(100 * time.Millisecond)

	if got := r.LastError(); got != "" {
		t.Errorf("LastError after success = %q, want empty", got)
	}
}

func TestReloader_StopHaltsDispatch(t *testing.T) {
	app := &fakeApplier{}
	r := NewReloader(app, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	r.Trigger("watcher")
	cancel()
	time.Sleep(100 * time.Millisecond)
	r.Trigger("watcher")

	got := atomic.LoadInt32(&app.calls)
	if got > 1 {
		t.Errorf("post-stop trigger fired Apply (calls=%d)", got)
	}
}
