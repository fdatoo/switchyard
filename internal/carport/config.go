package carport

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
)

// LifecycleConfig tunes per-instance timing and restart policy.
// Zero values get replaced by defaults during load.
type LifecycleConfig struct {
	HandshakeDeadline       time.Duration
	HealthProbeInterval     time.Duration
	HealthProbeTimeout      time.Duration
	HealthFailuresToRestart int
	ShutdownGrace           time.Duration
	RestartBackoffInitial   time.Duration
	RestartBackoffMax       time.Duration
	RestartBudgetWindow     time.Duration
	RestartBudgetMax        int
}

// Instance is one entry in drivers.toml.
type Instance struct {
	ID         string
	Binary     string
	Enabled    bool
	ConfigJSON []byte
	Lifecycle  LifecycleConfig
}

// Config is the parsed drivers.toml.
type Config struct {
	Instances []Instance
}

var idRE = regexp.MustCompile(`^[a-z0-9_\-]{1,64}$`)

// LoadConfig reads drivers.toml at path. A missing file yields an empty Config.
// An unreadable/invalid file is an error. Every entry is validated per §5.2 of
// the C2 design doc.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read drivers.toml: %w", err)
	}

	var raw struct {
		Instance []struct {
			ID         string `toml:"id"`
			Binary     string `toml:"binary"`
			Enabled    *bool  `toml:"enabled"`
			ConfigJSON string `toml:"config_json"`
			Lifecycle  struct {
				HandshakeDeadlineMs     int `toml:"handshake_deadline_ms"`
				HealthProbeIntervalMs   int `toml:"health_probe_interval_ms"`
				HealthProbeTimeoutMs    int `toml:"health_probe_timeout_ms"`
				HealthFailuresToRestart int `toml:"health_failures_to_restart"`
				ShutdownGraceMs         int `toml:"shutdown_grace_ms"`
				RestartBackoffInitialMs int `toml:"restart_backoff_initial_ms"`
				RestartBackoffMaxMs     int `toml:"restart_backoff_max_ms"`
				RestartBudgetWindowMin  int `toml:"restart_budget_window_minutes"`
				RestartBudgetMax        int `toml:"restart_budget_max"`
			} `toml:"lifecycle"`
		} `toml:"instance"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse drivers.toml: %w", err)
	}

	seen := map[string]bool{}
	cfg := &Config{}
	for i, r := range raw.Instance {
		if !idRE.MatchString(r.ID) {
			return nil, fmt.Errorf("instance[%d]: invalid id %q (want [a-z0-9_-]{1,64})", i, r.ID)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("duplicate instance id %q", r.ID)
		}
		seen[r.ID] = true
		if r.Binary == "" {
			return nil, fmt.Errorf("instance %q: binary is required", r.ID)
		}
		info, err := os.Stat(r.Binary)
		if err != nil {
			return nil, fmt.Errorf("instance %q: binary %q: %w", r.ID, r.Binary, err)
		}
		if info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("instance %q: binary %q is not executable", r.ID, r.Binary)
		}
		enabled := true
		if r.Enabled != nil {
			enabled = *r.Enabled
		}
		lc := LifecycleConfig{
			HandshakeDeadline:       dur(r.Lifecycle.HandshakeDeadlineMs, 5*time.Second),
			HealthProbeInterval:     dur(r.Lifecycle.HealthProbeIntervalMs, 15*time.Second),
			HealthProbeTimeout:      dur(r.Lifecycle.HealthProbeTimeoutMs, 3*time.Second),
			HealthFailuresToRestart: intd(r.Lifecycle.HealthFailuresToRestart, 3),
			ShutdownGrace:           dur(r.Lifecycle.ShutdownGraceMs, 10*time.Second),
			RestartBackoffInitial:   dur(r.Lifecycle.RestartBackoffInitialMs, 1*time.Second),
			RestartBackoffMax:       dur(r.Lifecycle.RestartBackoffMaxMs, 60*time.Second),
			RestartBudgetWindow:     time.Duration(intd(r.Lifecycle.RestartBudgetWindowMin, 10)) * time.Minute,
			RestartBudgetMax:        intd(r.Lifecycle.RestartBudgetMax, 10),
		}
		cfg.Instances = append(cfg.Instances, Instance{
			ID:         r.ID,
			Binary:     r.Binary,
			Enabled:    enabled,
			ConfigJSON: []byte(r.ConfigJSON),
			Lifecycle:  lc,
		})
	}
	return cfg, nil
}

func dur(ms int, def time.Duration) time.Duration {
	if ms <= 0 {
		return def
	}
	return time.Duration(ms) * time.Millisecond
}

func intd(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
