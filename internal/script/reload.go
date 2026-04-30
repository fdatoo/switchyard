package script

import (
	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
)

// Reload re-compiles the snapshot and atomically swaps the script registry.
// In-flight Calls that captured their *Script pointer under RLock continue
// against the old definition.
func (e *Engine) Reload(snapshot *configpb.ConfigSnapshot) error {
	m, err := CompileScripts(snapshot)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.scripts = m
	n := len(m)
	e.mu.Unlock()
	if e.deps.Metrics != nil {
		e.deps.Metrics.ScriptRegistered.Set(float64(n))
	}
	return nil
}
