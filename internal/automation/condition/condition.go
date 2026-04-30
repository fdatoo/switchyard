// Package condition holds automation-engine condition evaluators. Each Pkl
// ConditionConfig variant compiles to one Evaluator; And/Or/Not compose.
// Typed conditions with missing data return (false, nil) with a warn log
// per spec §8 — automations do not abort on condition faults.
package condition

import (
	"context"
	"log/slog"
	"time"

	"github.com/fdatoo/gohome/internal/eventstore"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

type Env struct {
	State   StateReader
	Runtime *ghstarlark.Runtime
	Event   *eventstore.Event
	Now     time.Time
	Loc     *time.Location
	Logger  *slog.Logger
}

type StateReader interface {
	Get(entityID string) (*ghstarlark.EntityState, bool)
}

type Evaluator interface {
	Evaluate(ctx context.Context, env Env) (bool, error)
}
