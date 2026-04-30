// Package automation is the runtime engine. Compile turns a ConfigSnapshot
// into an in-memory graph of automations; NewEngine+Start subscribes to the
// event store, admits runs through a per-automation mode state machine, and
// emits AutomationTriggered / AutomationFinished under shared correlation IDs.
package automation

import "fmt"

type ItemError struct {
	AutomationID string
	Path         string
	Reason       string
}

func (e *ItemError) Error() string {
	return fmt.Sprintf("automations[%s].%s: %s", e.AutomationID, e.Path, e.Reason)
}

type CompileError struct{ Items []*ItemError }

func (e *CompileError) Error() string {
	if len(e.Items) == 1 {
		return e.Items[0].Error()
	}
	return fmt.Sprintf("%d automation compile errors (first: %s)", len(e.Items), e.Items[0].Error())
}
