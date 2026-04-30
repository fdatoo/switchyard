package starlark

import (
	"fmt"
	"time"
)

// ContextKind identifies the execution context for a Starlark script.
type ContextKind int

const (
	KindAutomation ContextKind = iota
	KindComputedEntity
	KindTriggerCondition
	KindScript
	KindWidgetCompute
	KindMCPEval
)

func (k ContextKind) String() string {
	switch k {
	case KindAutomation:
		return "automation"
	case KindComputedEntity:
		return "computed_entity"
	case KindTriggerCondition:
		return "trigger_condition"
	case KindScript:
		return "script"
	case KindWidgetCompute:
		return "widget_compute"
	case KindMCPEval:
		return "mcp_eval"
	default:
		return "unknown"
	}
}

// KindFromString parses a string into a ContextKind. Accepts both canonical
// names ("automation") and short aliases ("computed", "condition", "widget", "mcp").
func KindFromString(s string) (ContextKind, error) {
	switch s {
	case "automation":
		return KindAutomation, nil
	case "computed", "computed_entity":
		return KindComputedEntity, nil
	case "condition", "trigger_condition":
		return KindTriggerCondition, nil
	case "script":
		return KindScript, nil
	case "widget", "widget_compute":
		return KindWidgetCompute, nil
	case "mcp", "mcp_eval":
		return KindMCPEval, nil
	default:
		return 0, fmt.Errorf("unknown context kind %q", s)
	}
}

// contextLimits holds resource limits and execution mode for one ContextKind.
type contextLimits struct {
	WallClock    time.Duration
	MaxSteps     uint64
	IsExpression bool // true → starlark.Eval; false → starlark.ExecFile
}

var kindLimits = map[ContextKind]contextLimits{
	KindAutomation:       {WallClock: 30 * time.Second, MaxSteps: 10_000_000, IsExpression: false},
	KindComputedEntity:   {WallClock: 100 * time.Millisecond, MaxSteps: 500_000, IsExpression: true},
	KindTriggerCondition: {WallClock: 50 * time.Millisecond, MaxSteps: 100_000, IsExpression: true},
	KindScript:           {WallClock: 30 * time.Second, MaxSteps: 10_000_000, IsExpression: false},
	KindWidgetCompute:    {WallClock: 50 * time.Millisecond, MaxSteps: 100_000, IsExpression: true},
	KindMCPEval:          {WallClock: 30 * time.Second, MaxSteps: 10_000_000, IsExpression: false},
}

func limitsFor(kind ContextKind) contextLimits {
	if cfg, ok := kindLimits[kind]; ok {
		return cfg
	}
	return kindLimits[KindScript]
}
