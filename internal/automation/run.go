package automation

import (
	"context"
	stderr "errors"
	"strings"
	"time"

	"github.com/google/uuid"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/automation/action"
	"github.com/fdatoo/gohome/internal/automation/condition"
	"github.com/fdatoo/gohome/internal/automation/trigger"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

func (e *Engine) executeRun(ctx context.Context, auto *Automation, m trigger.Match, invokedBy string) eventv1.RunOutcome {
	corrID := uuid.New()
	corrStr := corrID.String()

	run := &action.Run{
		CorrelationID: corrStr,
		AutomationID:  auto.ID,
		State:         e.deps.State,
		Dispatcher:    e.deps.Dispatcher,
		Store:         e.deps.Store,
		Scenes:        e.deps.Scenes,
		Scripts:       e.scriptCaller,
		Runtime:       e.runtime,
		Logger:        e.deps.Logger,
		Metrics:       e.deps.Metrics,
		TriggerEvent:  m.Event,
	}

	triggerPos := uint64(0)
	if m.Event != nil {
		triggerPos = m.Event.Position
	}
	_, _ = e.deps.Store.Append(ctx, eventstore.Event{
		Kind:          "automation_triggered",
		Source:        "automation:" + auto.ID,
		Timestamp:     time.Now(),
		CorrelationID: corrID,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationTriggered{
			AutomationTriggered: &eventv1.AutomationTriggered{
				AutomationId:         auto.ID,
				CorrelationId:        corrStr,
				TriggerEventPosition: triggerPos,
				TriggerKind:          m.TriggerKind,
				InvokedBy:            invokedBy,
			}}},
	})

	start := time.Now()
	if !evalConditions(ctx, auto.Conditions, run, m, e.runtime, e.deps.Metrics) {
		elapsed := time.Since(start)
		outcome := eventv1.RunOutcome_OUTCOME_CONDITION_FAIL
		if e.deps.Metrics != nil {
			e.deps.Metrics.AutomationRunsTotal.WithLabelValues(auto.ID, runOutcomeLabel(outcome)).Inc()
			e.deps.Metrics.AutomationRunDurationSeconds.WithLabelValues(auto.ID).Observe(elapsed.Seconds())
			steps, _ := run.Snapshot()
			e.deps.Metrics.AutomationStarlarkSteps.WithLabelValues(auto.ID).Observe(float64(steps))
		}
		e.appendFinished(ctx, auto, run, corrID, outcome, "", start)
		return outcome
	}
	top := &action.SequenceBlock{Children: auto.Actions, ChildCtrl: auto.ActionCtrl}
	err := top.Execute(ctx, run)
	outcome := classify(err, ctx)
	elapsed := time.Since(start)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	if e.deps.Metrics != nil {
		e.deps.Metrics.AutomationRunsTotal.WithLabelValues(auto.ID, runOutcomeLabel(outcome)).Inc()
		e.deps.Metrics.AutomationRunDurationSeconds.WithLabelValues(auto.ID).Observe(elapsed.Seconds())
		steps, _ := run.Snapshot()
		e.deps.Metrics.AutomationStarlarkSteps.WithLabelValues(auto.ID).Observe(float64(steps))
	}
	e.appendFinished(ctx, auto, run, corrID, outcome, errMsg, start)
	return outcome
}

// runOutcomeLabel converts a RunOutcome proto enum to the lower-snake label
// used in gohome_automation_runs_total{outcome}. Strips the "OUTCOME_" prefix
// and lowercases the remainder (e.g. OUTCOME_OK → "ok").
func runOutcomeLabel(o eventv1.RunOutcome) string {
	s := o.String() // e.g. "OUTCOME_OK"
	const prefix = "OUTCOME_"
	if len(s) > len(prefix) {
		tail := s[len(prefix):]
		return strings.ToLower(tail)
	}
	return strings.ToLower(s)
}

func evalConditions(ctx context.Context, evs []condition.Evaluator, run *action.Run, m trigger.Match, rt *ghstarlark.Runtime, metrics *observability.Metrics) bool {
	env := condition.Env{
		State:   run.State,
		Runtime: rt,
		Event:   m.Event,
		Now:     time.Now(),
		Loc:     time.Local,
		Logger:  run.Logger,
	}
	for _, c := range evs {
		ok, err := c.Evaluate(ctx, env)
		if metrics != nil {
			result := "pass"
			if err != nil {
				result = "error"
			} else if !ok {
				result = "fail"
			}
			metrics.AutomationConditionsTotal.WithLabelValues(run.AutomationID, result).Inc()
		}
		if !ok {
			return false
		}
	}
	return true
}

func classify(err error, ctx context.Context) eventv1.RunOutcome {
	if err == nil {
		return eventv1.RunOutcome_OUTCOME_OK
	}
	var le *ghstarlark.LimitError
	if stderr.As(err, &le) {
		return eventv1.RunOutcome_OUTCOME_LIMIT_EXCEEDED
	}
	if ctx.Err() == context.Canceled || stderr.Is(err, context.Canceled) {
		return eventv1.RunOutcome_OUTCOME_CANCELLED
	}
	return eventv1.RunOutcome_OUTCOME_ACTION_ERROR
}

func (e *Engine) appendFinished(ctx context.Context, auto *Automation, run *action.Run, corrID uuid.UUID, outcome eventv1.RunOutcome, errStr string, start time.Time) {
	steps, logs := run.Snapshot()
	_, _ = e.deps.Store.Append(ctx, eventstore.Event{
		Kind:          "automation_finished",
		Source:        "automation:" + auto.ID,
		Timestamp:     time.Now(),
		CorrelationID: corrID,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationFinished{
			AutomationFinished: &eventv1.AutomationFinished{
				AutomationId:  auto.ID,
				CorrelationId: corrID.String(),
				Outcome:       outcome,
				Error:         errStr,
				ElapsedMs:     time.Since(start).Milliseconds(),
				StarlarkSteps: steps,
				LogLines:      logs,
			}}},
	})
}
