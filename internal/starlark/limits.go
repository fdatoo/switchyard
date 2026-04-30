package starlark

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	starlarkgo "go.starlark.net/starlark"
)

// LimitKind identifies which resource limit was breached.
type LimitKind int

const (
	LimitSteps LimitKind = iota
	LimitWallClock
)

// LimitError is returned by Execute when a resource limit is exceeded.
type LimitError struct {
	Kind    LimitKind
	Context ContextKind
	Detail  string
}

func (e *LimitError) Error() string {
	switch e.Kind {
	case LimitSteps:
		return fmt.Sprintf("step limit exceeded in %s: %s", e.Context, e.Detail)
	case LimitWallClock:
		return fmt.Sprintf("wall-clock limit exceeded in %s: %s", e.Context, e.Detail)
	}
	return e.Detail
}

// startWatchdog starts a goroutine that cancels thread after timeout or when
// ctx is done. Returns a stop function (must be deferred) and a timedOut flag
// that is set true only when the wall-clock deadline elapsed.
func startWatchdog(ctx context.Context, timeout time.Duration, thread *starlarkgo.Thread) (stop func(), timedOut *atomic.Bool) {
	wdCtx, cancel := context.WithTimeout(ctx, timeout)
	timedOut = &atomic.Bool{}
	go func() {
		<-wdCtx.Done()
		if errors.Is(wdCtx.Err(), context.DeadlineExceeded) {
			timedOut.Store(true)
			thread.Cancel("timeout")
			return
		}
		// Caller ctx cancelled — propagate cancellation but not as a wall-clock breach.
		thread.Cancel("caller_cancelled")
	}()
	return cancel, timedOut
}
