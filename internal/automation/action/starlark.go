// Package-level note on event globals (spec §10.1 deviation):
//
// Spec §10.1 describes a structured "event" global (event.kind, event.entity_id)
// for KindAutomation scripts. This implementation instead injects flat globals
// event_kind and event_entity_id to remain consistent with the C5
// KindTriggerCondition contract where the same flat names were established.
//
// Migration path: a future v1.x release will add the structured "event" dict
// alongside the flat names (with a deprecation period) before removing the
// flat form in v2. Until then, automation Starlark bodies must use event_kind
// and event_entity_id rather than event.kind / event.entity_id.
package action

import (
	"context"
	"fmt"

	starlarkgo "go.starlark.net/starlark"

	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

type StarlarkAction struct{ Body string }

func (a *StarlarkAction) Execute(ctx context.Context, run *Run) error {
	if run.Runtime == nil {
		return fmt.Errorf("starlark: no runtime")
	}
	extra := starlarkgo.StringDict{"correlation_id": starlarkgo.String(run.CorrelationID)}
	if run.TriggerEvent != nil {
		extra["event_kind"] = starlarkgo.String(run.TriggerEvent.Kind)
		extra["event_entity_id"] = starlarkgo.String(run.TriggerEvent.Entity)
	}
	res, err := run.Runtime.Execute(ctx, ghstarlark.KindAutomation, a.Body, extra)
	if res != nil {
		run.AddSteps(res.Steps)
		for _, l := range res.Logs {
			run.AddLog(l)
		}
	}
	return err
}
