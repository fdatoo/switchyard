package automation

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fdatoo/gohome/internal/automation/trigger"
)

type runState struct {
	auto *Automation

	running   atomic.Bool // ModeSingle gate
	restartMu sync.Mutex
	// activeCancel/restartGen are guarded by restartMu.
	// restartGen increments on each new restart; a goroutine only nils
	// activeCancel if its generation still matches.
	activeCancel context.CancelFunc
	restartGen   uint64

	queueMu sync.Mutex
	queue   chan pending
}

type pending struct {
	match      trigger.Match
	enqueuedAt time.Time
}

func newRunState(a *Automation) *runState { return &runState{auto: a} }

func (rs *runState) ensureQueue(size int) chan pending {
	rs.queueMu.Lock()
	defer rs.queueMu.Unlock()
	if rs.queue == nil {
		rs.queue = make(chan pending, size)
	}
	return rs.queue
}

func (rs *runState) swapAuto(a *Automation) {
	rs.queueMu.Lock()
	rs.auto = a
	rs.queueMu.Unlock()
}
