package bridge

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Paces(t *testing.T) {
	l := newRateLimiter(10, 10)
	ctx := context.Background()
	// Burn the burst.
	for i := 0; i < 10; i++ {
		if err := l.wait(ctx); err != nil {
			t.Fatalf("burst call %d: %v", i, err)
		}
	}
	// 11th must wait for a refill (~100ms at 10/sec).
	start := time.Now()
	if err := l.wait(ctx); err != nil {
		t.Fatalf("11th call: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("11th call elapsed %v, want >= 50ms (refill)", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("11th call elapsed %v, want <= 200ms (one token = ~100ms)", elapsed)
	}
}

func TestRateLimiter_RespectsContext(t *testing.T) {
	l := newRateLimiter(1, 1)
	ctx := context.Background()
	if err := l.wait(ctx); err != nil {
		t.Fatal(err)
	}
	// Burn the only token. Next wait would block for ~1s; with a tight
	// deadline it should return ctx.DeadlineExceeded quickly.
	tight, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := l.wait(tight); err == nil {
		t.Fatal("expected deadline-exceeded error")
	}
}
