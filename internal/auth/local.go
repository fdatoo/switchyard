package auth

import (
	"context"
	"errors"
	"strconv"
)

// LocalPeerCred authenticates trusted Unix-domain socket clients by peer credentials.
type LocalPeerCred struct{}

// Authenticate accepts requests classified as local peer-credential traffic.
func (LocalPeerCred) Authenticate(_ context.Context, req Request) (Principal, error) {
	if req.Scheme != "uds:peercred" || req.PeerCred == nil {
		return Principal{}, ErrNotApplicable
	}
	return Principal{
		ID:          "system:local",
		DisplayName: "local",
		Kind:        "system",
		Metadata: map[string]string{
			"uid": strconv.FormatUint(uint64(req.PeerCred.Uid), 10),
			"gid": strconv.FormatUint(uint64(req.PeerCred.Gid), 10),
			"pid": strconv.FormatInt(int64(req.PeerCred.Pid), 10),
		},
	}, nil
}

// AllowAll authorizes every action and is used when policy enforcement is disabled.
type AllowAll struct{}

// Authorize always permits the request.
func (AllowAll) Authorize(_ context.Context, _ Principal, _ Action, _ Target) error {
	return nil
}

// Chain tries authenticators in order until one succeeds or returns a terminal error.
func Chain(as ...Authenticator) Authenticator {
	return chain(as)
}

type chain []Authenticator

func (c chain) Authenticate(ctx context.Context, req Request) (Principal, error) {
	for _, a := range c {
		p, err := a.Authenticate(ctx, req)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrNotApplicable) {
			return Principal{}, err
		}
	}
	return Principal{}, ErrUnauthenticated
}
