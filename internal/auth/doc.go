// Package auth defines the daemon's authentication and authorization boundary.
//
// The package contains transport-neutral request, principal, action, and target
// types plus the interfaces implemented by concrete authenticators and
// authorizers. Subpackages handle credential storage, identity projection,
// session cookies, throttling, and request-specific authentication chains.
package auth
