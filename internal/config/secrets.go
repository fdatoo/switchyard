package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

// Keyring is satisfied by go-keyring and by test doubles.
type Keyring interface {
	Get(service, user string) (string, error)
}

// ResolveSecrets resolves tagged secret strings in driver instance params in-place.
// Secrets are NEVER written to the event log — call this after Compile, before Apply side-effects.
// If kr is nil, keyring: secrets return an error.
func ResolveSecrets(_ context.Context, snap *configpb.ConfigSnapshot, kr Keyring) error {
	for _, di := range snap.GetDriverInstances() {
		resolved, err := resolveJSONSecrets(di.GetParams(), kr)
		if err != nil {
			return fmt.Errorf("driver instance %q: %w", di.GetId(), err)
		}
		di.Params = resolved
	}
	return nil
}

func resolveJSONSecrets(data []byte, kr Keyring) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return data, nil
	}
	changed := false
	if err := walkMap(obj, kr, &changed); err != nil {
		return nil, err
	}
	if !changed {
		return data, nil
	}
	return json.Marshal(obj)
}

func walkMap(m map[string]interface{}, kr Keyring, changed *bool) error {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			resolved, err := resolveSecret(val, kr)
			if err != nil {
				return fmt.Errorf("field %q: %w", k, err)
			}
			if resolved != val {
				m[k] = resolved
				*changed = true
			}
		case map[string]interface{}:
			if err := walkMap(val, kr, changed); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSecret(s string, kr Keyring) (string, error) {
	switch {
	case strings.HasPrefix(s, "env:"):
		varName := s[4:]
		val := os.Getenv(varName)
		if val == "" {
			return "", fmt.Errorf("env var %q is not set", varName)
		}
		return val, nil
	case strings.HasPrefix(s, "file:"):
		path := s[5:]
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read secret file %q: %w", path, err)
		}
		return strings.TrimSpace(string(data)), nil
	case strings.HasPrefix(s, "keyring:"):
		if kr == nil {
			return "", fmt.Errorf("keyring not available (secret: %q)", s)
		}
		rest := s[8:]
		idx := strings.LastIndex(rest, "/")
		if idx < 0 {
			return "", fmt.Errorf("invalid keyring secret %q: want keyring:service/user", s)
		}
		service, user := rest[:idx], rest[idx+1:]
		return kr.Get(service, user)
	default:
		return s, nil
	}
}
