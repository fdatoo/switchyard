package config

import (
	"github.com/zalando/go-keyring"
)

// ZalandoKeyring is the production Keyring backed by the OS-native secret store
// (Keychain on macOS, Secret Service on Linux, Credential Manager on Windows).
// Use NewZalandoKeyring to construct.
type ZalandoKeyring struct{}

// NewZalandoKeyring returns a Keyring that delegates to github.com/zalando/go-keyring.
func NewZalandoKeyring() *ZalandoKeyring { return &ZalandoKeyring{} }

// Get fetches the secret stored under (service, user) from the OS keyring.
func (ZalandoKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}
