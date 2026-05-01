package api_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/api"
)

func TestHeartbeatTicker_FiresOnIdle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tk := api.NewHeartbeatTicker(ctx, 50*time.Millisecond)
	defer tk.Stop()

	select {
	case <-tk.C():
		// ok
	case <-time.After(time.Second):
		t.Fatal("no heartbeat fired in time")
	}
}

func TestHeartbeatTicker_ResetOnSent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tk := api.NewHeartbeatTicker(ctx, 100*time.Millisecond)
	defer tk.Stop()

	time.Sleep(80 * time.Millisecond)
	tk.NotePayloadSent()

	select {
	case <-tk.C():
		t.Fatal("heartbeat fired despite recent payload")
	case <-time.After(60 * time.Millisecond):
		// pass
	}
}

func TestBoundedFanOut_DropsOnOverflow(t *testing.T) {
	in := make(chan int, 1)
	out, done := api.BoundedFanOut(context.Background(), in, 4)

	for i := 0; i < 100; i++ {
		select {
		case in <- i:
		default:
		}
		time.Sleep(time.Microsecond)
	}
	close(in)

	overflowed := false
	for {
		select {
		case <-out:
		case err := <-done:
			if errors.Is(err, api.ErrSubscriptionOverflow) {
				overflowed = true
			}
			if !overflowed {
				t.Fatalf("expected overflow, done err = %v", err)
			}
			return
		case <-time.After(2 * time.Second):
			t.Fatal("timed out")
		}
	}
}
