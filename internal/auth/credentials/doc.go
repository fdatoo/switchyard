// Package credentials stores and verifies user-facing authentication material.
//
// It owns password hashes, API tokens, WebAuthn passkeys, and one-time
// enrollment tokens. Callers receive stable sentinel errors for invalid,
// expired, revoked, replayed, or otherwise unusable credentials so API adapters
// can map failures without parsing error strings.
package credentials
