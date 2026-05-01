package automation

import (
	"context"

	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/observability"
)

// metricExecutor wraps a leaf action.Executor to record
// switchyard_automation_actions_total{automation_id, action_kind, result}.
// Block executors (SequenceBlock, ParallelBlock) are NOT wrapped here —
// their children each get their own wrapper so nested actions are counted
// individually. The continueOnError flag is the same flag the parent block
// uses: when set and Execute returns an error the result label is
// "skipped_continue" (the parent block continues past the error).
type metricExecutor struct {
	inner           action.Executor
	kind            string
	automationID    string
	continueOnError bool
	metrics         *observability.Metrics
}

func (w *metricExecutor) Execute(ctx context.Context, run *action.Run) error {
	err := w.inner.Execute(ctx, run)
	if w.metrics != nil {
		var result string
		switch {
		case err == nil:
			result = "ok"
		case w.continueOnError:
			result = "skipped_continue"
		default:
			result = "error"
		}
		w.metrics.AutomationActionsTotal.WithLabelValues(w.automationID, w.kind, result).Inc()
	}
	return err
}

// actionKindOf returns the string kind name for a known leaf executor type.
// Returns "" for block types that should not be individually wrapped.
func actionKindOf(ex action.Executor) string {
	switch ex.(type) {
	case *action.CallServiceAction:
		return "call_service"
	case *action.SceneAction:
		return "scene"
	case *action.ScriptAction:
		return "script"
	case *action.StarlarkAction:
		return "starlark"
	case *action.WaitAction:
		return "wait"
	default:
		return ""
	}
}

// wrapActionsWithMetrics walks an Automation's action slice and wraps each
// leaf executor with a metricExecutor. Used by NewEngine to instrument
// pre-compiled automations that came from CompileAutomations (which doesn't
// inject metrics).
func wrapActionsWithMetrics(a *Automation, metrics *observability.Metrics) {
	if metrics == nil {
		return
	}
	for i, ex := range a.Actions {
		continueOnError := false
		if i < len(a.ActionCtrl) {
			continueOnError = a.ActionCtrl[i].ContinueOnError
		}
		a.Actions[i] = wrapExec(ex, a.ID, continueOnError, metrics)
	}
}

// wrapExec wraps a single executor: leaf types get a metricExecutor, blocks
// have their children wrapped recursively.
func wrapExec(ex action.Executor, automationID string, continueOnError bool, metrics *observability.Metrics) action.Executor {
	// If already wrapped (e.g. from an instrumented compile), skip.
	if _, ok := ex.(*metricExecutor); ok {
		return ex
	}
	kind := actionKindOf(ex)
	if kind != "" {
		return &metricExecutor{
			inner:           ex,
			kind:            kind,
			automationID:    automationID,
			continueOnError: continueOnError,
			metrics:         metrics,
		}
	}
	// Recurse into blocks.
	switch b := ex.(type) {
	case *action.SequenceBlock:
		for i, child := range b.Children {
			coe := false
			if i < len(b.ChildCtrl) {
				coe = b.ChildCtrl[i].ContinueOnError
			}
			b.Children[i] = wrapExec(child, automationID, coe, metrics)
		}
	case *action.ParallelBlock:
		for i, child := range b.Children {
			coe := false
			if i < len(b.ChildCtrl) {
				coe = b.ChildCtrl[i].ContinueOnError
			}
			b.Children[i] = wrapExec(child, automationID, coe, metrics)
		}
	}
	return ex
}
