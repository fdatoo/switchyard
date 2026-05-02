package config

import (
	"time"

	"github.com/fdatoo/switchyard/internal/carport"
)

// MergeLifecycle computes the effective LifecycleConfig for one instance:
//
//	carport.DefaultLifecycleConfig()  (Go-side concrete defaults)
//	  ← manifest defaults             (LifecycleOverride: only non-nil fields override)
//	  ← per-instance override         (LifecycleOverride: only non-nil fields override)
//
// Both override layers use the same shape; both layers' wire format omits
// unset fields (Pkl JsonRenderer drops nulls), so each pointer being nil
// genuinely means "the author didn't set this."
func MergeLifecycle(manifest, override LifecycleOverride) carport.LifecycleConfig {
	lc := carport.DefaultLifecycleConfig()
	applyDur(&lc.HandshakeDeadline, manifest.HandshakeDeadline, override.HandshakeDeadline)
	applyDur(&lc.HealthProbeInterval, manifest.HealthProbeInterval, override.HealthProbeInterval)
	applyDur(&lc.HealthProbeTimeout, manifest.HealthProbeTimeout, override.HealthProbeTimeout)
	applyInt(&lc.HealthFailuresToRestart, manifest.HealthFailuresToRestart, override.HealthFailuresToRestart)
	applyDur(&lc.ShutdownGrace, manifest.ShutdownGrace, override.ShutdownGrace)
	applyDur(&lc.RestartBackoffInitial, manifest.RestartBackoffInitial, override.RestartBackoffInitial)
	applyDur(&lc.RestartBackoffMax, manifest.RestartBackoffMax, override.RestartBackoffMax)
	applyDur(&lc.RestartBudgetWindow, manifest.RestartBudgetWindow, override.RestartBudgetWindow)
	applyInt(&lc.RestartBudgetMax, manifest.RestartBudgetMax, override.RestartBudgetMax)
	return lc
}

func applyDur(dst *time.Duration, fromManifest, fromOverride *time.Duration) {
	if fromManifest != nil {
		*dst = *fromManifest
	}
	if fromOverride != nil {
		*dst = *fromOverride
	}
}

func applyInt(dst *int, fromManifest, fromOverride *int) {
	if fromManifest != nil {
		*dst = *fromManifest
	}
	if fromOverride != nil {
		*dst = *fromOverride
	}
}
