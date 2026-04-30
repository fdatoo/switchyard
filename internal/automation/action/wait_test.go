package action_test

import (
	"context"
	"testing"
	"time"

	"github.com/fdatoo/gohome/internal/automation/action"
)

func TestWait_Elapses(t *testing.T) {
	a := &action.WaitAction{Duration: 30 * time.Millisecond}
	start := time.Now()
	if err := a.Execute(context.Background(), &action.Run{}); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 25*time.Millisecond {
		t.Error("too short")
	}
}

func TestWait_ContextCancels(t *testing.T) {
	a := &action.WaitAction{Duration: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	if err := a.Execute(ctx, &action.Run{}); err == nil {
		t.Fatal("want err")
	}
}
