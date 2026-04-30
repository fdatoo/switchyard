package condition

import (
	"context"

	starlarkgo "go.starlark.net/starlark"

	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

// StarlarkCondition evaluates a Starlark expression as a boolean condition.
// Any runtime or parse error returns (false, nil) with a warn log per spec §8.
type StarlarkCondition struct{ Expr string }

func (c *StarlarkCondition) Evaluate(ctx context.Context, env Env) (bool, error) {
	if env.Runtime == nil {
		return false, nil
	}
	extra := starlarkgo.StringDict{}
	if env.Event != nil {
		extra["event_kind"] = starlarkgo.String(env.Event.Kind)
		extra["event_entity_id"] = starlarkgo.String(env.Event.Entity)
	}
	res, err := env.Runtime.Execute(ctx, ghstarlark.KindTriggerCondition, c.Expr, extra)
	if err != nil {
		if env.Logger != nil {
			env.Logger.Warn("starlark condition", "err", err)
		}
		return false, nil
	}
	if res == nil || res.Value == nil {
		return false, nil
	}
	if b, ok := res.Value.(starlarkgo.Bool); ok {
		return bool(b), nil
	}
	return res.Value.Truth() == starlarkgo.True, nil
}
