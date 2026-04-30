package carport_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fdatoo/gohome/internal/carport"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "drivers.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig_EmptyFileIsValid(t *testing.T) {
	p := writeTOML(t, "")
	cfg, err := carport.LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Instances) != 0 {
		t.Fatalf("want 0 instances, got %d", len(cfg.Instances))
	}
}

func TestLoadConfig_HappyPath(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTOML(t, `
[[instance]]
id = "hue_main"
binary = "`+bin+`"
enabled = true
config_json = "{\"x\": 1}"
`)
	cfg, err := carport.LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Instances) != 1 {
		t.Fatalf("want 1 instance, got %d", len(cfg.Instances))
	}
	got := cfg.Instances[0]
	if got.ID != "hue_main" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Binary != bin {
		t.Errorf("Binary = %q", got.Binary)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
	if string(got.ConfigJSON) != `{"x": 1}` {
		t.Errorf("ConfigJSON = %q", got.ConfigJSON)
	}
	if got.Lifecycle.HealthProbeInterval != 15*time.Second {
		t.Errorf("HealthProbeInterval default = %v", got.Lifecycle.HealthProbeInterval)
	}
}

func TestLoadConfig_RejectsDuplicateID(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	p := writeTOML(t, `
[[instance]]
id = "a"
binary = "`+bin+`"

[[instance]]
id = "a"
binary = "`+bin+`"
`)
	_, err := carport.LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for duplicate id")
	}
}

func TestLoadConfig_RejectsInvalidID(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	longID := "a" + string(make([]byte, 80)) // > 64 chars; fails regex
	for _, id := range []string{"", "UPPER", "with space", longID} {
		p := writeTOML(t, `
[[instance]]
id = "`+id+`"
binary = "`+bin+`"
`)
		if _, err := carport.LoadConfig(p); err == nil {
			t.Errorf("expected error for id %q", id)
		}
	}
}

func TestLoadConfig_RejectsMissingBinary(t *testing.T) {
	p := writeTOML(t, `
[[instance]]
id = "x"
binary = "/no/such/file/exists/here"
`)
	_, err := carport.LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestLoadConfig_AcceptsLifecycleOverrides(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	p := writeTOML(t, `
[[instance]]
id = "x"
binary = "`+bin+`"
[instance.lifecycle]
health_probe_interval_ms = 5000
health_failures_to_restart = 5
shutdown_grace_ms = 20000
restart_budget_window_minutes = 30
restart_budget_max = 3
`)
	cfg, err := carport.LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	inst := cfg.Instances[0]
	if inst.Lifecycle.HealthProbeInterval != 5*time.Second {
		t.Errorf("HealthProbeInterval = %v", inst.Lifecycle.HealthProbeInterval)
	}
	if inst.Lifecycle.HealthFailuresToRestart != 5 {
		t.Errorf("HealthFailuresToRestart = %d", inst.Lifecycle.HealthFailuresToRestart)
	}
	if inst.Lifecycle.RestartBudgetWindow != 30*time.Minute {
		t.Errorf("RestartBudgetWindow = %v", inst.Lifecycle.RestartBudgetWindow)
	}
}

func TestLoadConfig_MissingFileReturnsEmptyConfig(t *testing.T) {
	cfg, err := carport.LoadConfig(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("LoadConfig missing: %v", err)
	}
	if len(cfg.Instances) != 0 {
		t.Fatalf("want 0 instances, got %d", len(cfg.Instances))
	}
}
