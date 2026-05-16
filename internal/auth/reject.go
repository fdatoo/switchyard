package auth

import "context"

// RejectAll denies every authentication attempt.
type RejectAll struct{}

// Authenticate always returns ErrUnauthenticated.
func (RejectAll) Authenticate(_ context.Context, _ Request) (Principal, error) {
	return Principal{}, ErrUnauthenticated
}
