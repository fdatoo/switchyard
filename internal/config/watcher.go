// Package config — file watcher for Pkl configuration files.
// Exposes a Subscribe hook used by the editsession package to push
// ExternalEditDetected events to active sessions when config files change.
package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
	"time"
)

const defaultWatchPollInterval = 500 * time.Millisecond

// WatchSubscriberFn is the callback signature for file-change notifications.
// It is called from the watcher goroutine; implementations must be
// non-blocking or use a buffered channel internally.
type WatchSubscriberFn func(path, hash string, modifiedAt time.Time)

// Watcher polls a set of Pkl files and fires registered subscriber callbacks
// when a file's content changes.
type Watcher struct {
	pollInterval time.Duration

	mu          sync.Mutex
	watched     map[string]watchedEntry // key: path
	subscribers []WatchSubscriberFn
}

type watchedEntry struct {
	hash    string
	modTime time.Time
}

// NewWatcher creates a Watcher with the given poll interval.
func NewWatcher(pollInterval time.Duration) *Watcher {
	if pollInterval <= 0 {
		pollInterval = defaultWatchPollInterval
	}
	return &Watcher{
		pollInterval: pollInterval,
		watched:      make(map[string]watchedEntry),
	}
}

// Watch adds path to the set of monitored files. Safe to call multiple times
// with the same path. Must be called before Start or from the same goroutine.
func (w *Watcher) Watch(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.watched[path]; !ok {
		h, mt, err := watchStatFile(path)
		if err == nil {
			w.watched[path] = watchedEntry{hash: h, modTime: mt}
		} else {
			// File may not exist yet; record an empty entry so we detect creation.
			w.watched[path] = watchedEntry{}
		}
	}
}

// Subscribe registers fn to be invoked whenever any watched file changes.
// Callbacks are fired from the watcher goroutine; fn must be non-blocking.
func (w *Watcher) Subscribe(fn WatchSubscriberFn) {
	w.mu.Lock()
	w.subscribers = append(w.subscribers, fn)
	w.mu.Unlock()
}

// Start begins the polling loop. It stops when ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.poll()
			}
		}
	}()
}

func (w *Watcher) poll() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for path, prev := range w.watched {
		h, mt, err := watchStatFile(path)
		if err != nil {
			continue
		}
		if prev.hash != h {
			w.watched[path] = watchedEntry{hash: h, modTime: mt}
			if prev.hash == "" {
				// First successful read — don't fire (bootstrap).
				continue
			}
			for _, fn := range w.subscribers {
				fn(path, h, mt)
			}
		}
	}
}

func watchStatFile(path string) (hash string, modTime time.Time, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", time.Time{}, err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), info.ModTime(), nil
}
