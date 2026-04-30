package action

import (
	"context"
	"time"
)

type WaitAction struct{ Duration time.Duration }

func (a *WaitAction) Execute(ctx context.Context, _ *Run) error {
	if a.Duration <= 0 {
		return nil
	}
	t := time.NewTimer(a.Duration)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
