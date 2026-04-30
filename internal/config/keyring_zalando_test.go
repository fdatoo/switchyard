package config_test

import (
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/fdatoo/gohome/internal/config"
)

// TestZalandoKeyring_SatisfiesInterface is a compile-time check that
// ZalandoKeyring implements config.Keyring. Failing this means a runtime
// nil-Keyring branch we no longer guarantee.
func TestZalandoKeyring_SatisfiesInterface(t *testing.T) {
	var _ config.Keyring = config.ZalandoKeyring{}
	var _ config.Keyring = config.NewZalandoKeyring()
}

// TestZalandoKeyring_RoundTrip exercises the real adapter against go-keyring.
// Skips when no OS keyring backend is available (CI without secret service).
func TestZalandoKeyring_RoundTrip(t *testing.T) {
	const service = "gohome-test"
	const user = "round-trip"
	const want = "hunter2"

	if err := keyring.Set(service, user, want); err != nil {
		if strings.Contains(err.Error(), "no usable keyring backend") ||
			strings.Contains(err.Error(), "Specified module could not be found") ||
			strings.Contains(err.Error(), "executable file not found") ||
			strings.Contains(err.Error(), "dbus") ||
			strings.Contains(err.Error(), "org.freedesktop.secrets") ||
			strings.Contains(err.Error(), ".service files") {
			t.Skipf("no keyring backend on this host: %v", err)
		}
		t.Fatalf("seed keyring: %v", err)
	}
	t.Cleanup(func() { _ = keyring.Delete(service, user) })

	got, err := config.NewZalandoKeyring().Get(service, user)
	if err != nil {
		t.Fatalf("ZalandoKeyring.Get: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
