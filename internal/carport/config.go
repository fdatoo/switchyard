package carport

import (
	"time"
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

// Instance is a single driver instance declaration.
type Instance struct {
	ID         string
	Binary     string
	Enabled    bool
	ConfigJSON []byte
	Lifecycle  LifecycleConfig
}

// defaultLifecycleConfig returns the defaults used for dynamically registered
// instances (those coming from main.pkl rather than drivers.toml).
func defaultLifecycleConfig() LifecycleConfig {
	return LifecycleConfig{
		HandshakeDeadline:       5 * time.Second,
		HealthProbeInterval:     15 * time.Second,
		HealthProbeTimeout:      3 * time.Second,
		HealthFailuresToRestart: 3,
		ShutdownGrace:           10 * time.Second,
		RestartBackoffInitial:   time.Second,
		RestartBackoffMax:       60 * time.Second,
		RestartBudgetWindow:     10 * time.Minute,
		RestartBudgetMax:        10,
	}
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
