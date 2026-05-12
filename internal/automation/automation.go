package automation

import (
	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/automation/condition"
	"github.com/fdatoo/switchyard/internal/automation/trigger"
)

type Mode int

const (
	ModeSingle Mode = iota
	ModeQueued
	ModeRestart
	ModeParallel
)

func (m Mode) String() string {
	switch m {
	case ModeQueued:
		return "queued"
	case ModeRestart:
		return "restart"
	case ModeParallel:
		return "parallel"
	default:
		return "single"
	}
}

type Automation struct {
	ID         string
	Triggers   []trigger.Matcher
	Conditions []condition.Evaluator
	Actions    []action.Executor
	ActionCtrl []action.ChildCtrl
	Mode       Mode
	MaxQueued  int
	Enabled    bool
	// Areas — rooms this automation operates on. Declared via Pkl;
	// surfaced on the wire as Automation.area_ids. The engine doesn't
	// otherwise consume this; it's purely informational for the UI's
	// per-room scoping.
	Areas []string

	// Source is the original proto that compiled into this Automation. It is
	// used by Reload to detect structural equality so unchanged automations
	// skip trigger re-registration (avoiding hold-timer and matcher state loss).
	Source *configpb.AutomationConfig
}
