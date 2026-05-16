// Package authn adapts HTTP requests into auth.Authenticator implementations.
//
// It contains the bearer-token and session-cookie authenticators used by the
// Connect listener, plus helpers that preserve Unix-domain peer credentials for
// trusted local daemon traffic.
package authn
