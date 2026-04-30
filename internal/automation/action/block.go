package action

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type SequenceBlock struct {
	Children  []Executor
	ChildCtrl []ChildCtrl
}

func (b *SequenceBlock) Execute(ctx context.Context, run *Run) error {
	for i, child := range b.Children {
		err := child.Execute(ctx, run)
		if err == nil {
			continue
		}
		if b.continueAt(i) {
			if run.Logger != nil {
				run.Logger.Warn("action continued after error", "index", i, "err", err, "correlation_id", run.CorrelationID)
			}
			continue
		}
		return err
	}
	return nil
}

func (b *SequenceBlock) continueAt(i int) bool {
	if i < len(b.ChildCtrl) {
		return b.ChildCtrl[i].ContinueOnError
	}
	return false
}

type ParallelBlock struct {
	Children  []Executor
	ChildCtrl []ChildCtrl
}

func (b *ParallelBlock) Execute(ctx context.Context, run *Run) error {
	eg, gctx := errgroup.WithContext(ctx)
	for i, child := range b.Children {
		i, child := i, child
		continueOnErr := false
		if i < len(b.ChildCtrl) {
			continueOnErr = b.ChildCtrl[i].ContinueOnError
		}
		eg.Go(func() error {
			err := child.Execute(gctx, run)
			if err == nil {
				return nil
			}
			if continueOnErr {
				if run.Logger != nil {
					run.Logger.Warn("parallel action soft error", "index", i, "err", err, "correlation_id", run.CorrelationID)
				}
				return nil
			}
			return err
		})
	}
	return eg.Wait()
}
