package editsession

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileWatcher_ExternalEditDetected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watch.pkl")
	if err := os.WriteFile(path, []byte("id = \"v1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	watcher := NewFileWatcher(30 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)

	// Subscribe before the external edit
	ch, unsubscribe := watcher.Subscribe(path)
	defer unsubscribe()

	// Write from outside
	time.Sleep(50 * time.Millisecond) // let watcher record initial state
	if err := os.WriteFile(path, []byte("id = \"v2\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-ch:
		if evt.Path != path {
			t.Errorf("event path: got %q want %q", evt.Path, path)
		}
		if evt.Hash == "" {
			t.Error("expected non-empty hash")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for ExternalEditDetected event")
	}
}

func TestFileWatcher_NoSpuriousEvents_WhenFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.pkl")
	if err := os.WriteFile(path, []byte("id = \"stable\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	watcher := NewFileWatcher(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)

	ch, unsubscribe := watcher.Subscribe(path)
	defer unsubscribe()

	// Wait several poll cycles
	time.Sleep(100 * time.Millisecond)

	select {
	case evt := <-ch:
		t.Errorf("unexpected event: %+v", evt)
	default:
		// Good: no spurious events
	}
}

func TestFileWatcher_Unsubscribe_ClosesChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unsub.pkl")
	_ = os.WriteFile(path, []byte("x\n"), 0o644)

	watcher := NewFileWatcher(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)

	ch, unsubscribe := watcher.Subscribe(path)
	unsubscribe()

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected closed channel after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel not closed after unsubscribe")
	}
}
