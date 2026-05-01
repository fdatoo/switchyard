package authn

import (
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
)

// Build assembles the C9 authenticator chain in priority order:
//  1. LocalPeerCred (Unix-domain socket peers)
//  2. Bearer token
//  3. Session cookie
//  4. RejectAll (catch-all)
func Build(tokens *credentials.Tokens, sess *sessions.Store) auth.Authenticator {
	return auth.Chain(
		auth.LocalPeerCred{},
		NewBearer(tokens),
		NewSessionCookie(sess),
		auth.RejectAll{},
	)
}
