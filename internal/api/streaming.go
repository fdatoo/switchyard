package api

import (
	"context"
	"sync"
	"time"
)

// HeartbeatTicker emits stream heartbeats only after an idle interval.
type HeartbeatTicker struct {
	interval time.Duration
	c        chan time.Time
	resetCh  chan struct{}
	done     chan struct{}
	once     sync.Once
}

// NewHeartbeatTicker starts a heartbeat ticker tied to ctx.
func NewHeartbeatTicker(ctx context.Context, interval time.Duration) *HeartbeatTicker {
	t := &HeartbeatTicker{
		interval: interval,
		c:        make(chan time.Time, 1),
		resetCh:  make(chan struct{}, 16),
		done:     make(chan struct{}),
	}
	go t.run(ctx)
	return t
}

func (t *HeartbeatTicker) run(ctx context.Context) {
	timer := time.NewTimer(t.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.done:
			return
		case <-t.resetCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(t.interval)
		case now := <-timer.C:
			select {
			case t.c <- now:
			default:
			}
			timer.Reset(t.interval)
		}
	}
}

// C returns the channel that receives idle heartbeat ticks.
func (t *HeartbeatTicker) C() <-chan time.Time { return t.c }

// NotePayloadSent resets the idle timer after a real stream payload is sent.
func (t *HeartbeatTicker) NotePayloadSent() {
	select {
	case t.resetCh <- struct{}{}:
	default:
	}
}

// Stop terminates the ticker goroutine.
func (t *HeartbeatTicker) Stop() {
	t.once.Do(func() { close(t.done) })
}

// BoundedFanOut reads from in and forwards to an output channel of size bufSize.
// If the output channel is full, it closes the output channel and sends
// ErrSubscriptionOverflow on the done channel. When in is closed or ctx is
// cancelled, done receives nil (or ctx.Err()).
func BoundedFanOut[T any](ctx context.Context, in <-chan T, bufSize int) (<-chan T, <-chan error) {
	out := make(chan T, bufSize)
	done := make(chan error, 1)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case v, ok := <-in:
				if !ok {
					done <- nil
					return
				}
				select {
				case out <- v:
				default:
					done <- ErrSubscriptionOverflow
					return
				}
			}
		}
	}()
	return out, done
}

// StreamConfig holds tunables for streaming RPCs.
type StreamConfig struct {
	HeartbeatInterval time.Duration
	BufSize           int
}

// DefaultStreamConfig returns the production defaults.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{HeartbeatInterval: 30 * time.Second, BufSize: 10000}
}

var (
	streamCfgMu sync.RWMutex
	streamCfg   = DefaultStreamConfig()
)

// SetStreamConfig replaces the global stream config. Intended for tests.
func SetStreamConfig(c StreamConfig) {
	streamCfgMu.Lock()
	defer streamCfgMu.Unlock()
	streamCfg = c
}

func currentStreamConfig() StreamConfig {
	streamCfgMu.RLock()
	defer streamCfgMu.RUnlock()
	return streamCfg
}
