package auth

import "context"

type RejectAll struct{}

func (RejectAll) Authenticate(_ context.Context, _ Request) (Principal, error) {
	return Principal{}, ErrUnauthenticated
}
