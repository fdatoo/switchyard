package action

import (
	"context"
	"fmt"

	"github.com/fdatoo/switchyard/internal/cause"
)

type CallServiceAction struct {
	Entity, Capability string
	Args               map[string]string
}

func (a *CallServiceAction) Execute(ctx context.Context, run *Run) error {
	if run.Dispatcher == nil {
		return fmt.Errorf("call_service: no dispatcher")
	}
	// Stamp automation lineage on ctx so carport can record it on CommandIssued.
	ctx = cause.WithCorrelation(ctx, cause.Correlation{
		AutomationID:  run.AutomationID,
		CorrelationID: run.CorrelationID,
	})
	res, err := run.Dispatcher.Dispatch(ctx, a.Entity, a.Capability, a.Args)
	if err != nil {
		return fmt.Errorf("call_service %s.%s: %w", a.Entity, a.Capability, err)
	}
	if !res.Ok {
		return &DispatchError{Entity: a.Entity, Capability: a.Capability, Msg: res.Error}
	}
	return nil
}

type DispatchError struct {
	Entity, Capability, Msg string
}

func (e *DispatchError) Error() string {
	return fmt.Sprintf("dispatch %s.%s: %s", e.Entity, e.Capability, e.Msg)
}
