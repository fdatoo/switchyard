// Package identity projects configured users and roles into the auth database.
//
// Pkl config remains the source of truth. Store applies full snapshots
// atomically, then exposes read APIs used by authenticators, policy evaluation,
// and administrative surfaces.
package identity
