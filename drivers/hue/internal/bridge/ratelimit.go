package bridge

import (
	"context"

	"golang.org/x/time/rate"
)

// rateLimiter wraps x/time/rate to keep our internal API tight.
type rateLimiter struct {
	l *rate.Limiter
}

// newRateLimiter constructs a rate-limit config: perSec tokens/second, with
// a burst budget of `burst`. Hue bridges document a limit of ~10 req/sec on
// /light resources; (10, 10) is the right default for that ceiling.
func newRateLimiter(perSec, burst int) *rateLimiter {
	return &rateLimiter{l: rate.NewLimiter(rate.Limit(perSec), burst)}
}

// wait blocks until a token is available or ctx is cancelled.
func (r *rateLimiter) wait(ctx context.Context) error {
	return r.l.Wait(ctx)
}
