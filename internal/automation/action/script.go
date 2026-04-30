package action

import (
	"context"
	"fmt"
)

type ScriptAction struct {
	Name string
	Args map[string]string
}

func (a *ScriptAction) Execute(ctx context.Context, run *Run) error {
	if run.Scripts == nil {
		return fmt.Errorf("script: no caller")
	}
	invokedBy := "automation:" + run.AutomationID
	res, callErr := run.Scripts.Call(ctx, a.Name, a.Args, invokedBy, run.CorrelationID)
	if res != nil {
		run.AddSteps(res.GetSteps())
		for _, l := range res.GetLogs() {
			run.AddLog(l)
		}
	}
	if callErr != nil {
		// Preserve typed errors (e.g. *starlark.LimitError) so that classify()
		// in run.go can match them via errors.As — wrapping with %w keeps the chain.
		return fmt.Errorf("script %q: %w", a.Name, callErr)
	}
	if res != nil && !res.Succeeded() {
		return fmt.Errorf("script %q: %s", a.Name, res.GetError())
	}
	return nil
}
