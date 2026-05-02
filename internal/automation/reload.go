package automation

import (
	"google.golang.org/protobuf/proto"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation/trigger"
)

// Reload replaces the live map with a freshly compiled one. In-flight runs
// whose automation is unchanged continue under their original *Automation
// pointer (runState.auto is swapped in place for new runs only). Removed
// automations have active runs cancelled; new automations are registered.
func (e *Engine) Reload(snap *configpb.ConfigSnapshot) error {
	var scriptNames map[string]bool
	if e.scriptEngine != nil {
		scriptNames = map[string]bool{}
		for _, n := range e.scriptEngine.List() {
			scriptNames[n] = true
		}
	}

	newAutos := map[string]*Automation{}
	var errs []*ItemError
	for _, ac := range snap.GetAutomations() {
		a, itemErrs := compileOneWithMetrics(ac, scriptNames, e.runtime, e.deps.Metrics)
		if len(itemErrs) > 0 {
			errs = append(errs, itemErrs...)
			continue
		}
		newAutos[a.ID] = a
	}
	if len(errs) > 0 {
		if e.deps.Metrics != nil {
			e.deps.Metrics.AutomationReloadFailuresTotal.Inc()
		}
		return &CompileError{Items: errs}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for id, oldA := range e.automations {
		newA, ok := newAutos[id]
		switch {
		case !ok:
			// Automation removed — cancel active runs and close queue.
			e.unregisterTriggers(oldA)
			if rs := e.runStates[id]; rs != nil {
				rs.restartMu.Lock()
				if rs.activeCancel != nil {
					rs.activeCancel()
					rs.activeCancel = nil
				}
				rs.restartMu.Unlock()
				rs.queueMu.Lock()
				if rs.queue != nil {
					close(rs.queue)
					rs.queue = nil
				}
				rs.queueMu.Unlock()
				delete(e.runStates, id)
			}
		default:
			if structurallyEqual(oldA, newA) {
				// Nothing changed — keep existing triggers and matchers intact
				// so StateChangeMatcher.last state and hold timers are preserved.
				continue
			}
			// Automation updated — swap triggers and auto pointer.
			e.unregisterTriggers(oldA)
			e.registerTriggers(newA)
			if rs := e.runStates[id]; rs != nil {
				rs.swapAuto(newA)
			}
		}
	}
	for id, newA := range newAutos {
		if _, existed := e.automations[id]; existed {
			continue
		}
		e.runStates[id] = newRunState(newA)
		e.registerTriggers(newA)
	}
	e.automations = newAutos
	if e.deps.Metrics != nil {
		e.deps.Metrics.AutomationRegistered.Set(float64(len(e.automations)))
	}

	// Rebuild scheduler wholesale.
	e.scheduler.Reset()
	for _, a := range e.automations {
		for _, m := range a.Triggers {
			if t, ok := m.(*trigger.TimeMatcher); ok {
				switch {
				case t.At != "":
					_ = e.scheduler.AddAt(a.ID, t.At)
				case t.Cron != "":
					_ = e.scheduler.AddCron(a.ID, t.Cron)
				case t.Every > 0:
					_ = e.scheduler.AddEvery(a.ID, t.Every)
				}
			}
		}
	}
	return nil
}

// structurallyEqual reports whether two Automation values are structurally
// identical by comparing their source proto definitions. When true, reload
// skips trigger re-registration so StateChangeMatcher.last state and active
// hold timers survive a no-op config push.
func structurallyEqual(a, b *Automation) bool {
	if a.Source == nil || b.Source == nil {
		return false
	}
	return proto.Equal(a.Source, b.Source)
}
