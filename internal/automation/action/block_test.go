package action_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fdatoo/gohome/internal/automation/action"
)

type recExec struct {
	ran       int32
	returnErr error
	delay     time.Duration
}

func (r *recExec) Execute(ctx context.Context, _ *action.Run) error {
	if r.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.delay):
		}
	}
	atomic.StoreInt32(&r.ran, 1)
	return r.returnErr
}

func TestSeq_Abort(t *testing.T) {
	a := &recExec{returnErr: errors.New("boom")}
	b := &recExec{}
	seq := &action.SequenceBlock{Children: []action.Executor{a, b}, ChildCtrl: []action.ChildCtrl{{}, {}}}
	if err := seq.Execute(context.Background(), &action.Run{}); err == nil {
		t.Fatal("want err")
	}
	if atomic.LoadInt32(&b.ran) != 0 {
		t.Fatal("b should not run")
	}
}

func TestSeq_Continue(t *testing.T) {
	a := &recExec{returnErr: errors.New("boom")}
	b := &recExec{}
	seq := &action.SequenceBlock{
		Children:  []action.Executor{a, b},
		ChildCtrl: []action.ChildCtrl{{ContinueOnError: true}, {}},
	}
	if err := seq.Execute(context.Background(), &action.Run{}); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&b.ran) == 0 {
		t.Fatal("b should run")
	}
}

func TestPar_AllRun(t *testing.T) {
	a := &recExec{delay: 20 * time.Millisecond}
	b := &recExec{delay: 20 * time.Millisecond}
	blk := &action.ParallelBlock{Children: []action.Executor{a, b}, ChildCtrl: []action.ChildCtrl{{}, {}}}
	start := time.Now()
	if err := blk.Execute(context.Background(), &action.Run{}); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("sequential")
	}
	if a.ran == 0 || b.ran == 0 {
		t.Fatal("both should run")
	}
}

func TestPar_HardErrCancels(t *testing.T) {
	a := &recExec{returnErr: errors.New("boom"), delay: 5 * time.Millisecond}
	b := &recExec{delay: time.Hour}
	blk := &action.ParallelBlock{Children: []action.Executor{a, b}, ChildCtrl: []action.ChildCtrl{{}, {}}}
	start := time.Now()
	if err := blk.Execute(context.Background(), &action.Run{}); err == nil {
		t.Fatal("want err")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("sibling should have cancelled")
	}
}
