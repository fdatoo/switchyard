package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/apple/pkl-go/pkl"
)

// DriverEntry is the resolved view of one driver's manifest.
type DriverEntry struct {
	Name              string
	Version           string
	BinaryPath        string            // absolute, ready to exec
	LifecycleDefaults LifecycleOverride // unset fields fall through to Go-side carport defaults
}

// DriverRegistry indexes the drivers under a single root by name. Built once
// at Manager construction; rebuilt on Manager.Validate so freshly-installed
// drivers are picked up without a daemon restart.
type DriverRegistry struct {
	root    string
	entries map[string]DriverEntry
}

// NewDriverRegistry scans <root>/*/manifest.pkl, evaluates each, and returns
// an indexed registry. Returns an error on the first malformed manifest
// (Pkl eval failure, missing `name`/`version`, name vs. directory mismatch).
//
// Empty or non-existent root is not an error — Names() will return nil.
func NewDriverRegistry(ctx context.Context, root string) (*DriverRegistry, error) {
	entries := map[string]DriverEntry{}
	if root == "" {
		return &DriverRegistry{root: root, entries: entries}, nil
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return &DriverRegistry{root: root, entries: entries}, nil
		}
		return nil, fmt.Errorf("read drivers root %s: %w", root, err)
	}
	if len(dirs) == 0 {
		return &DriverRegistry{root: root, entries: entries}, nil
	}

	ev, err := newPklEvaluator(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("init pkl evaluator for registry scan: %w", err)
	}
	defer func() { _ = ev.Close() }()

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dirName := d.Name()
		if !validDriverName(dirName) {
			continue
		}
		manifestPath := filepath.Join(root, dirName, "manifest.pkl")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		entry, err := evaluateDriverManifest(ctx, ev, root, dirName)
		if err != nil {
			return nil, err
		}
		entries[dirName] = entry
	}
	return &DriverRegistry{root: root, entries: entries}, nil
}

// Names returns the registered driver names, sorted.
func (r *DriverRegistry) Names() []string {
	out := make([]string, 0, len(r.entries))
	for n := range r.entries {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the DriverEntry for name, ok=false if not present.
func (r *DriverRegistry) Lookup(name string) (DriverEntry, bool) {
	e, ok := r.entries[name]
	return e, ok
}

// driverManifestJSON is the wire shape Pkl's JSON renderer produces for a
// driver manifest module (after `extends "switchyard:driver"`).
type driverManifestJSON struct {
	Name              string                `json:"name"`
	Version           string                `json:"version"`
	Description       string                `json:"description"`
	Produces          []string              `json:"produces"`
	DriverEventTypes  []string              `json:"driverEventTypes"`
	Binary            *string               `json:"binary"`
	LifecycleDefaults lifecycleOverrideJSON `json:"lifecycleDefaults"`
}

func evaluateDriverManifest(ctx context.Context, ev *pklEvaluator, root, dirName string) (DriverEntry, error) {
	manifestPath := filepath.Join(root, dirName, "manifest.pkl")
	text, err := ev.ev.EvaluateOutputText(ctx, pkl.FileSource(manifestPath))
	if err != nil {
		return DriverEntry{}, fmt.Errorf("driver %q: evaluate manifest: %w", dirName, err)
	}
	var raw driverManifestJSON
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return DriverEntry{}, fmt.Errorf("driver %q: parse manifest JSON: %w", dirName, err)
	}
	if raw.Name == "" {
		return DriverEntry{}, fmt.Errorf("driver %q: manifest missing required field `name`", dirName)
	}
	if raw.Version == "" {
		return DriverEntry{}, fmt.Errorf("driver %q: manifest missing required field `version`", dirName)
	}
	if raw.Name != dirName {
		return DriverEntry{}, fmt.Errorf("driver %q: manifest declares name=%q, expected %q (directory name is authoritative)", dirName, raw.Name, dirName)
	}
	binary := dirName + "-driver"
	if raw.Binary != nil && *raw.Binary != "" {
		binary = *raw.Binary
	}
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(root, dirName, binary)
	}
	return DriverEntry{
		Name:              raw.Name,
		Version:           raw.Version,
		BinaryPath:        binary,
		LifecycleDefaults: decodeLifecycleOverride(raw.LifecycleDefaults),
	}, nil
}
