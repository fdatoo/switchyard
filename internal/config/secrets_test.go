package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

type fakeKeyring struct {
	data map[string]string
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
	v, ok := f.data[service+"/"+user]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return v, nil
}

func TestResolveSecrets_Env(t *testing.T) {
	t.Setenv("TEST_API_KEY", "secret-value")
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", Params: []byte(`{"id":"hue-main","apiKey":"env:TEST_API_KEY"}`)},
		},
	}
	if err := ResolveSecrets(context.Background(), snap, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(snap.DriverInstances[0].Params, []byte("secret-value")) {
		t.Fatalf("secret not resolved: %s", snap.DriverInstances[0].Params)
	}
}

func TestResolveSecrets_File(t *testing.T) {
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "api_key")
	_ = os.WriteFile(secretFile, []byte("  file-secret-value\n"), 0o600)

	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "test", Params: []byte(`{"id":"test","token":"file:` + secretFile + `"}`)},
		},
	}
	if err := ResolveSecrets(context.Background(), snap, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(snap.DriverInstances[0].Params, []byte("file-secret-value")) {
		t.Fatalf("file secret not resolved: %s", snap.DriverInstances[0].Params)
	}
}

func TestResolveSecrets_Keyring(t *testing.T) {
	kr := &fakeKeyring{data: map[string]string{"gohome/hue_key": "kr-secret"}}
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue", Params: []byte(`{"id":"hue","apiKey":"keyring:gohome/hue_key"}`)},
		},
	}
	if err := ResolveSecrets(context.Background(), snap, kr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(snap.DriverInstances[0].Params, []byte("kr-secret")) {
		t.Fatalf("keyring secret not resolved: %s", snap.DriverInstances[0].Params)
	}
}
