package automation

import (
	"fmt"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/automation/condition"
	"github.com/fdatoo/switchyard/internal/automation/trigger"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/script"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

// metricsIface is a thin optional wrapper so that compile functions can thread
// the metrics pointer without requiring a non-nil value.
type metricsIface = *observability.Metrics

// CompileAutomations walks AutomationConfig entries, producing a name-keyed
// map of compiled runtime Automations. Errors are aggregated; callers get
// the full list from one `config validate`.
func CompileAutomations(snap *configpb.ConfigSnapshot, se *script.Engine, rt *ghstarlark.Runtime) (map[string]*Automation, error) {
	out := map[string]*Automation{}
	var errs []*ItemError
	scriptNames := map[string]bool{}
	if se != nil {
		for _, n := range se.List() {
			scriptNames[n] = true
		}
	}

	for _, ac := range snap.GetAutomations() {
		if ac.GetId() == "" {
			errs = append(errs, &ItemError{AutomationID: "<unset>", Path: "id", Reason: "empty"})
			continue
		}
		a, itemErrs := compileOne(ac, scriptNames, rt)
		if len(itemErrs) > 0 {
			errs = append(errs, itemErrs...)
			continue
		}
		if _, dup := out[a.ID]; dup {
			errs = append(errs, &ItemError{AutomationID: a.ID, Path: "id", Reason: "duplicate"})
			continue
		}
		out[a.ID] = a
	}
	if len(errs) > 0 {
		return nil, &CompileError{Items: errs}
	}
	return out, nil
}

func compileOne(ac *configpb.AutomationConfig, scripts map[string]bool, rt *ghstarlark.Runtime) (*Automation, []*ItemError) {
	return compileOneWithMetrics(ac, scripts, rt, nil)
}

func compileOneWithMetrics(ac *configpb.AutomationConfig, scripts map[string]bool, rt *ghstarlark.Runtime, metrics metricsIface) (*Automation, []*ItemError) {
	var errs []*ItemError
	a := &Automation{
		ID:        ac.GetId(),
		Enabled:   ac.GetEnabled(),
		Mode:      modeFromProto(ac.GetMode()),
		MaxQueued: int(ac.GetMaxQueued()),
		Source:    ac,
	}
	if a.MaxQueued <= 0 {
		a.MaxQueued = 10
	}

	for i, tc := range ac.GetTriggers() {
		m, err := compileTrigger(a.ID, tc)
		if err != nil {
			errs = append(errs, &ItemError{AutomationID: a.ID, Path: fmt.Sprintf("triggers[%d]", i), Reason: err.Error()})
			continue
		}
		a.Triggers = append(a.Triggers, m)
	}
	for i, cc := range ac.GetConditions() {
		ev, err := compileCondition(cc)
		if err != nil {
			errs = append(errs, &ItemError{AutomationID: a.ID, Path: fmt.Sprintf("conditions[%d]", i), Reason: err.Error()})
			continue
		}
		a.Conditions = append(a.Conditions, ev)
	}
	for i, acfg := range ac.GetActions() {
		ex, err := compileActionInstrumented(acfg, scripts, rt, a.ID, metrics)
		if err != nil {
			errs = append(errs, &ItemError{AutomationID: a.ID, Path: fmt.Sprintf("actions[%d]", i), Reason: err.Error()})
			continue
		}
		a.Actions = append(a.Actions, ex)
		a.ActionCtrl = append(a.ActionCtrl, action.ChildCtrl{ContinueOnError: acfg.GetContinueOnError()})
	}
	return a, errs
}

func modeFromProto(m configpb.AutomationConfig_Mode) Mode {
	switch m {
	case configpb.AutomationConfig_MODE_QUEUED:
		return ModeQueued
	case configpb.AutomationConfig_MODE_RESTART:
		return ModeRestart
	case configpb.AutomationConfig_MODE_PARALLEL:
		return ModeParallel
	default:
		return ModeSingle
	}
}

func compileTrigger(automationID string, tc *configpb.TriggerConfig) (trigger.Matcher, error) {
	switch k := tc.GetKind().(type) {
	case *configpb.TriggerConfig_StateChange:
		sc := k.StateChange
		if len(sc.GetEntities()) == 0 {
			return nil, fmt.Errorf("state_change: entities empty")
		}
		return trigger.NewStateChangeMatcher(automationID, sc.GetEntities(), sc.GetFrom(), sc.GetTo(), time.Duration(sc.GetForDurNs()), nil), nil
	case *configpb.TriggerConfig_Event:
		if k.Event.GetKind() == "" {
			return nil, fmt.Errorf("event: kind empty")
		}
		if err := trigger.ValidateEventDataKeys(k.Event.GetData()); err != nil {
			return nil, err
		}
		return trigger.NewEventMatcher(automationID, k.Event.GetKind(), k.Event.GetData()), nil
	case *configpb.TriggerConfig_Time:
		tt := k.Time
		set := 0
		if tt.GetAt() != "" {
			set++
		}
		if tt.GetCron() != "" {
			set++
		}
		if tt.GetEveryNs() != 0 {
			set++
		}
		if set != 1 {
			return nil, fmt.Errorf("time: exactly one of at/cron/every, got %d", set)
		}
		return &trigger.TimeMatcher{AutomationIDVal: automationID, At: tt.GetAt(), Cron: tt.GetCron(), Every: time.Duration(tt.GetEveryNs())}, nil
	case *configpb.TriggerConfig_Webhook:
		return &trigger.WebhookMatcher{AutomationIDVal: automationID, Path: k.Webhook.GetPath()}, nil
	default:
		return nil, fmt.Errorf("unknown trigger variant")
	}
}

func compileCondition(cc *configpb.ConditionConfig) (condition.Evaluator, error) {
	switch k := cc.GetKind().(type) {
	case *configpb.ConditionConfig_State:
		s := k.State
		nOp := 0
		if s.GetEquals() != "" {
			nOp++
		}
		if len(s.GetOneOf()) > 0 {
			nOp++
		}
		if s.GetNot() != "" {
			nOp++
		}
		if nOp != 1 {
			return nil, fmt.Errorf("state: exactly one operator, got %d", nOp)
		}
		return &condition.StateCondition{Entity: s.GetEntity(), Equals: s.GetEquals(), OneOf: s.GetOneOf(), Not: s.GetNot()}, nil
	case *configpb.ConditionConfig_Numeric:
		n := k.Numeric
		switch n.GetOp() {
		case "lt", "lte", "eq", "gte", "gt":
		default:
			return nil, fmt.Errorf("numeric op %q", n.GetOp())
		}
		return &condition.NumericCondition{Entity: n.GetEntity(), Attribute: n.GetAttribute(), Op: n.GetOp(), Value: n.GetValue()}, nil
	case *configpb.ConditionConfig_Time:
		t := k.Time
		return &condition.TimeCondition{After: t.GetAfter(), Before: t.GetBefore(), Weekdays: t.GetWeekdays()}, nil
	case *configpb.ConditionConfig_Starlark:
		s := k.Starlark
		if err := ghstarlark.ParseOnly(s.GetExpr(), true); err != nil {
			return nil, fmt.Errorf("starlark cond: %w", err)
		}
		return &condition.StarlarkCondition{Expr: s.GetExpr()}, nil
	case *configpb.ConditionConfig_And:
		if len(k.And.GetAll()) == 0 {
			return nil, fmt.Errorf("and: empty")
		}
		and := &condition.AndCondition{}
		for _, c := range k.And.GetAll() {
			ev, err := compileCondition(c)
			if err != nil {
				return nil, err
			}
			and.All = append(and.All, ev)
		}
		return and, nil
	case *configpb.ConditionConfig_Or:
		if len(k.Or.GetAny()) == 0 {
			return nil, fmt.Errorf("or: empty")
		}
		or := &condition.OrCondition{}
		for _, c := range k.Or.GetAny() {
			ev, err := compileCondition(c)
			if err != nil {
				return nil, err
			}
			or.Any = append(or.Any, ev)
		}
		return or, nil
	case *configpb.ConditionConfig_Not:
		inner, err := compileCondition(k.Not.GetNot())
		if err != nil {
			return nil, err
		}
		return &condition.NotCondition{Inner: inner}, nil
	default:
		return nil, fmt.Errorf("unknown condition variant")
	}
}

// compileActionInstrumented wraps leaf executors in
// metricExecutor so that gohome_automation_actions_total is recorded per
// action. automationID and metrics may be empty/nil for tests that don't need
// metrics (the wrapper is a no-op when metrics == nil).
func compileActionInstrumented(acfg *configpb.ActionConfig, scripts map[string]bool, rt *ghstarlark.Runtime, automationID string, metrics metricsIface) (action.Executor, error) {
	wrap := func(ex action.Executor, kind string) action.Executor {
		if metrics == nil {
			return ex
		}
		return &metricExecutor{
			inner:           ex,
			kind:            kind,
			automationID:    automationID,
			continueOnError: acfg.GetContinueOnError(),
			metrics:         metrics,
		}
	}

	switch k := acfg.GetKind().(type) {
	case *configpb.ActionConfig_CallService:
		cs := k.CallService
		if cs.GetEntity() == "" || cs.GetCapability() == "" {
			return nil, fmt.Errorf("call_service: entity+capability required")
		}
		return wrap(&action.CallServiceAction{Entity: cs.GetEntity(), Capability: cs.GetCapability(), Args: cs.GetArgs()}, "call_service"), nil
	case *configpb.ActionConfig_Scene:
		return wrap(&action.SceneAction{Slug: k.Scene.GetSlug()}, "scene"), nil
	case *configpb.ActionConfig_Script:
		sc := k.Script
		if !scripts[sc.GetName()] {
			return nil, fmt.Errorf("script: unknown name %q", sc.GetName())
		}
		return wrap(&action.ScriptAction{Name: sc.GetName(), Args: sc.GetArgs()}, "script"), nil
	case *configpb.ActionConfig_Starlark:
		if err := ghstarlark.ParseOnly(k.Starlark.GetBody(), false); err != nil {
			return nil, fmt.Errorf("starlark: %w", err)
		}
		return wrap(&action.StarlarkAction{Body: k.Starlark.GetBody()}, "starlark"), nil
	case *configpb.ActionConfig_Wait:
		return wrap(&action.WaitAction{Duration: time.Duration(k.Wait.GetDurationNs())}, "wait"), nil
	case *configpb.ActionConfig_Sequence:
		blk := &action.SequenceBlock{}
		for _, c := range k.Sequence.GetActions() {
			ex, err := compileActionInstrumented(c, scripts, rt, automationID, metrics)
			if err != nil {
				return nil, err
			}
			blk.Children = append(blk.Children, ex)
			blk.ChildCtrl = append(blk.ChildCtrl, action.ChildCtrl{ContinueOnError: c.GetContinueOnError()})
		}
		return blk, nil
	case *configpb.ActionConfig_Parallel:
		blk := &action.ParallelBlock{}
		for _, c := range k.Parallel.GetActions() {
			ex, err := compileActionInstrumented(c, scripts, rt, automationID, metrics)
			if err != nil {
				return nil, err
			}
			blk.Children = append(blk.Children, ex)
			blk.ChildCtrl = append(blk.ChildCtrl, action.ChildCtrl{ContinueOnError: c.GetContinueOnError()})
		}
		return blk, nil
	default:
		return nil, fmt.Errorf("unknown action variant")
	}
}
